package templateupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
)

// UpdateMode defines the operational mode for the update command.
type UpdateMode int

const (
	// UpdateModeApply is the default mode: compute and apply changes.
	UpdateModeApply UpdateMode = iota
	// UpdateModeContinue resumes after manual conflict resolution.
	UpdateModeContinue
	// UpdateModeAbort restores from backup and cancels in-progress update.
	UpdateModeAbort
)

// UpdateOptions configures the update operation.
type UpdateOptions struct {
	ProjectDir   string
	Ref          string
	VarOverrides map[string]string
	ResolveMode  ResolveMode
	SkipPatterns []string
	DryRun       bool
	Backup       bool
	Mode         UpdateMode
}

// UpdateResult contains the outcome of an update operation.
type UpdateResult struct {
	OldSHA       string
	NewSHA       string
	Applied      []MergeResult
	Conflicts    *ConflictReport
	NewFiles     int
	UpdatedFiles int
	DeletedFiles int
}

// Updater applies upstream template changes to the user's project.
type Updater struct {
	renderer *HistoricalRenderer
	resolver remote.LatestCommitResolver
}

// NewUpdater creates an Updater with the given renderer and resolver.
func NewUpdater(renderer *HistoricalRenderer, resolver remote.LatestCommitResolver) *Updater {
	return &Updater{renderer: renderer, resolver: resolver}
}

// Update performs the template update operation.
func (u *Updater) Update(ctx context.Context, opts UpdateOptions) (*UpdateResult, error) {
	projectDir := opts.ProjectDir
	if projectDir == "" {
		projectDir = "."
	}

	switch opts.Mode {
	case UpdateModeContinue:
		return u.continueUpdate(projectDir)
	case UpdateModeAbort:
		return u.abortUpdate(projectDir)
	default:
		return u.applyUpdate(ctx, projectDir, opts)
	}
}

// resolveUpdateContext holds the resolved state needed to perform a merge.
type resolveUpdateContext struct {
	cfg       *scaffold.TagConfig
	ref       *remote.Reference
	latestSHA string
	vars      map[string]any
}

// resolveUpdate performs pre-flight checks and resolves the latest upstream commit.
func (u *Updater) resolveUpdate(ctx context.Context, projectDir string, opts UpdateOptions) (*resolveUpdateContext, *UpdateResult, error) {
	cfg, err := scaffold.LoadTagConfig(projectDir)
	if err != nil {
		return nil, nil, fmt.Errorf("load project config: %w", err)
	}

	if valErr := validateTagConfig(cfg); valErr != nil {
		return nil, nil, valErr
	}

	// Check for pending conflicts.
	status, statusErr := ReadConflictStatus(projectDir)
	if statusErr != nil {
		return nil, nil, fmt.Errorf("read conflict status: %w", statusErr)
	}
	if status != nil {
		return nil, nil, errors.New("pending conflicts from a previous update — run 'tag update --continue' or 'tag update --abort'")
	}

	ref, err := remote.Parse(cfg.Template.Source)
	if err != nil {
		return nil, nil, fmt.Errorf("parse template source: %w", err)
	}

	if opts.Ref != "" {
		ref.Version = opts.Ref
	} else if cfg.Template.Ref != "" {
		ref.Version = cfg.Template.Ref
	}

	latestSHA, err := u.resolver.ResolveLatestCommit(ctx, ref)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve latest commit: %w", err)
	}

	if cfg.Template.CommitSHA == latestSHA {
		return nil, &UpdateResult{
			OldSHA: cfg.Template.CommitSHA,
			NewSHA: latestSHA,
		}, nil
	}

	return &resolveUpdateContext{
		cfg:       cfg,
		ref:       ref,
		latestSHA: latestSHA,
		vars:      mergeVars(cfg.Variables, opts.VarOverrides),
	}, nil, nil
}

// performMerge renders both template versions and performs the 3-way merge.
func (u *Updater) performMerge(ctx context.Context, projectDir string, rctx *resolveUpdateContext, opts UpdateOptions) ([]MergeResult, *ConflictReport, error) {
	base, theirs, err := u.renderer.RenderPair(ctx, rctx.ref.URL, rctx.cfg.Template.CommitSHA, rctx.latestSHA, rctx.vars)
	if err != nil {
		return nil, nil, fmt.Errorf("render templates: %w", err)
	}

	ours, err := ReadProjectFiles(projectDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read project files: %w", err)
	}

	allSkipPatterns := make([]string, 0, len(rctx.cfg.SkipPatterns)+len(opts.SkipPatterns))
	allSkipPatterns = append(allSkipPatterns, rctx.cfg.SkipPatterns...)
	allSkipPatterns = append(allSkipPatterns, opts.SkipPatterns...)
	ignoreMatcher, err := NewIgnoreMatcher(IgnoreMatcherOptions{
		ProjectRoot:       projectDir,
		TagconfigPatterns: allSkipPatterns,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("build ignore matcher: %w", err)
	}

	ignoreFn := func(path string) bool {
		return ignoreMatcher.ShouldSkip(path, false)
	}
	merger := NewMergeEngine(&GitMerger{}, ignoreFn)
	results, skipped, err := merger.MergeTrees(ctx, base, ours, theirs)
	if err != nil {
		return nil, nil, fmt.Errorf("merge trees: %w", err)
	}

	report := NewConflictReport(results, skipped)

	if opts.ResolveMode != ResolveNone && report.HasConflicts() {
		resolved := ResolveConflicts(report, opts.ResolveMode)
		results = replaceConflicts(results, resolved)
		report = NewConflictReport(results, skipped)
	}

	return results, report, nil
}

// finalizeUpdate applies merge results and updates the project config.
func finalizeUpdate(projectDir string, result *UpdateResult, report *ConflictReport, cfg *scaffold.TagConfig, latestSHA, refVersion string, vars map[string]any) error {
	if applyErr := applyResults(projectDir, result.Applied); applyErr != nil {
		return fmt.Errorf("apply changes: %w", applyErr)
	}

	if report.HasConflicts() {
		conflictStatus := NewConflictStatus(report, latestSHA)
		if writeErr := WriteConflictStatus(projectDir, conflictStatus); writeErr != nil {
			return fmt.Errorf("write conflict status: %w", writeErr)
		}
		return nil
	}

	if updateErr := updateTagConfig(projectDir, cfg, latestSHA, refVersion, vars); updateErr != nil {
		return fmt.Errorf("update tagconfig: %w", updateErr)
	}
	return nil
}

// applyUpdate runs the full update workflow.
func (u *Updater) applyUpdate(ctx context.Context, projectDir string, opts UpdateOptions) (*UpdateResult, error) {
	rctx, earlyResult, err := u.resolveUpdate(ctx, projectDir, opts)
	if err != nil {
		return nil, err
	}
	if earlyResult != nil {
		return earlyResult, nil
	}

	results, report, err := u.performMerge(ctx, projectDir, rctx, opts)
	if err != nil {
		return nil, err
	}

	// Backup affected files.
	if opts.Backup && !opts.DryRun {
		affectedPaths := collectAffectedPaths(results)
		if len(affectedPaths) > 0 {
			if _, backupErr := CreateBackup(projectDir, affectedPaths); backupErr != nil {
				return nil, fmt.Errorf("create backup: %w", backupErr)
			}
		}
	}

	result := buildUpdateResult(rctx.cfg.Template.CommitSHA, rctx.latestSHA, results, report)

	if opts.DryRun {
		return result, nil
	}

	if err := finalizeUpdate(projectDir, result, report, rctx.cfg, rctx.latestSHA, rctx.ref.Version, rctx.vars); err != nil {
		return nil, err
	}

	return result, nil
}

// buildUpdateResult constructs an UpdateResult with operation counts.
func buildUpdateResult(oldSHA, newSHA string, results []MergeResult, report *ConflictReport) *UpdateResult {
	result := &UpdateResult{
		OldSHA:    oldSHA,
		NewSHA:    newSHA,
		Applied:   results,
		Conflicts: report,
	}
	for _, r := range results {
		switch r.Op {
		case MergeAdd:
			result.NewFiles++
		case MergeUpdate:
			result.UpdatedFiles++
		case MergeDelete:
			result.DeletedFiles++
		}
	}
	return result
}

// continueUpdate resumes after manual conflict resolution.
func (u *Updater) continueUpdate(projectDir string) (*UpdateResult, error) {
	status, err := ReadConflictStatus(projectDir)
	if err != nil {
		return nil, fmt.Errorf("read conflict status: %w", err)
	}
	if status == nil {
		return nil, errors.New("no pending update to continue")
	}

	// Check all conflicted files are resolved (no more conflict markers).
	for _, cf := range status.ConflictedFiles {
		filePath := filepath.Join(projectDir, cf)
		content, readErr := os.ReadFile(filePath)
		if readErr != nil {
			return nil, fmt.Errorf("read conflicted file %s: %w", cf, readErr)
		}

		if strings.Contains(string(content), "<<<<<<<") {
			return nil, fmt.Errorf("unresolved conflict markers in %s — please resolve before continuing", cf)
		}
	}

	// Clear conflict status.
	if clearErr := ClearConflictStatus(projectDir); clearErr != nil {
		return nil, fmt.Errorf("clear conflict status: %w", clearErr)
	}

	// Update tagconfig with the new commit.
	cfg, loadErr := scaffold.LoadTagConfig(projectDir)
	if loadErr != nil {
		return nil, fmt.Errorf("load project config: %w", loadErr)
	}

	if updateErr := updateTagConfig(projectDir, cfg, status.UpdateCommit, "", cfg.Variables); updateErr != nil {
		return nil, fmt.Errorf("update tagconfig: %w", updateErr)
	}

	return &UpdateResult{
		OldSHA: cfg.Template.CommitSHA,
		NewSHA: status.UpdateCommit,
	}, nil
}

// abortUpdate restores from backup and clears conflict state.
func (u *Updater) abortUpdate(projectDir string) (*UpdateResult, error) {
	backupPath, err := FindLatestBackup(projectDir)
	if err != nil {
		return nil, fmt.Errorf("find backup: %w", err)
	}
	if backupPath == "" {
		return nil, errors.New("no backup found — cannot abort")
	}

	if err := RestoreBackup(projectDir, backupPath); err != nil {
		return nil, fmt.Errorf("restore backup: %w", err)
	}

	// Clean up.
	_ = ClearConflictStatus(projectDir)
	_ = RemoveBackup(backupPath)

	return &UpdateResult{}, nil
}

// applyResults writes merge results to the project directory.
func applyResults(projectDir string, results []MergeResult) error {
	for _, r := range results {
		filePath := filepath.Join(projectDir, r.Path)

		switch r.Op {
		case MergeAdd, MergeUpdate, MergeConflict:
			if err := os.MkdirAll(filepath.Dir(filePath), types.DirMode); err != nil {
				return fmt.Errorf("create dir for %s: %w", r.Path, err)
			}
			if err := os.WriteFile(filePath, r.Content, r.Mode); err != nil {
				return fmt.Errorf("write %s: %w", r.Path, err)
			}
		case MergeDelete:
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("delete %s: %w", r.Path, err)
			}
		case MergeKeep, MergeUserAdded:
			// No action needed.
		}
	}
	return nil
}

// updateTagConfig updates .tagconfig.json with the new commit SHA and variables.
func updateTagConfig(projectDir string, cfg *scaffold.TagConfig, newSHA, newRef string, vars map[string]any) error {
	cfg.Template.CommitSHA = newSHA
	if newRef != "" {
		cfg.Template.Ref = newRef
	}
	cfg.Variables = vars

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tagconfig: %w", err)
	}

	configPath := filepath.Join(projectDir, types.TagConfigFile)
	return os.WriteFile(configPath, append(data, '\n'), types.FileMode)
}

// mergeVars merges stored variables with command-line overrides.
func mergeVars(stored map[string]any, overrides map[string]string) map[string]any {
	if len(overrides) == 0 {
		return stored
	}

	merged := make(map[string]any, len(stored))
	maps.Copy(merged, stored)
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}

// replaceConflicts replaces MergeConflict entries in results with resolved entries.
func replaceConflicts(results, resolved []MergeResult) []MergeResult {
	resolvedMap := make(map[string]MergeResult, len(resolved))
	for _, r := range resolved {
		resolvedMap[r.Path] = r
	}

	out := make([]MergeResult, 0, len(results))
	for _, r := range results {
		if r.Op == MergeConflict {
			if replacement, ok := resolvedMap[r.Path]; ok {
				out = append(out, replacement)
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

// collectAffectedPaths returns paths that will be modified by the merge.
func collectAffectedPaths(results []MergeResult) []string {
	var paths []string
	for _, r := range results {
		switch r.Op {
		case MergeAdd, MergeUpdate, MergeDelete, MergeConflict:
			paths = append(paths, r.Path)
		}
	}
	return paths
}
