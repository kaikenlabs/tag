package templateupdate

import (
	"context"
	"fmt"

	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
)

// DiffOptions configures the diff operation.
type DiffOptions struct {
	ProjectDir string // Project directory (default: ".")
	Ref        string // Override template ref
}

// DiffResult contains the outcome of a template diff operation.
type DiffResult struct {
	OldSHA  string        // commit SHA from .tagconfig.json
	NewSHA  string        // latest commit SHA from remote
	Source  string        // template source string
	Results []MergeResult // merge results from dry-run
	Skipped []string      // paths skipped by ignore rules
}

// Differ performs a dry-run 3-way merge to show what would change on update.
type Differ struct {
	renderer *HistoricalRenderer
	resolver remote.LatestCommitResolver
}

// NewDiffer creates a Differ with the given renderer and resolver.
func NewDiffer(renderer *HistoricalRenderer, resolver remote.LatestCommitResolver) *Differ {
	return &Differ{renderer: renderer, resolver: resolver}
}

// Diff computes the differences between the current project and the latest
// template version. It performs a full 3-way merge in dry-run mode.
func (d *Differ) Diff(ctx context.Context, opts DiffOptions) (*DiffResult, error) {
	projectDir := opts.ProjectDir
	if projectDir == "" {
		projectDir = "."
	}

	cfg, err := scaffold.LoadTagConfig(projectDir)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	if valErr := validateTagConfig(cfg); valErr != nil {
		return nil, valErr
	}

	// Resolve latest commit.
	ref, err := remote.Parse(cfg.Template.Source)
	if err != nil {
		return nil, fmt.Errorf("parse template source: %w", err)
	}

	if opts.Ref != "" {
		ref.Version = opts.Ref
	} else if cfg.Template.Ref != "" {
		ref.Version = cfg.Template.Ref
	}

	latestSHA, err := d.resolver.ResolveLatestCommit(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("resolve latest commit: %w", err)
	}

	if cfg.Template.CommitSHA == latestSHA {
		return &DiffResult{
			OldSHA: cfg.Template.CommitSHA,
			NewSHA: latestSHA,
			Source: cfg.Template.Source,
		}, nil
	}

	// Build template URL for rendering.
	templateURL := ref.URL

	// Render base (old commit) and theirs (new commit).
	base, theirs, err := d.renderer.RenderPair(ctx, templateURL, cfg.Template.CommitSHA, latestSHA, cfg.Variables)
	if err != nil {
		return nil, fmt.Errorf("render templates: %w", err)
	}

	// Read user's project files as "ours".
	ours, err := ReadProjectFiles(projectDir)
	if err != nil {
		return nil, fmt.Errorf("read project files: %w", err)
	}

	// Build ignore matcher.
	ignoreMatcher, err := NewIgnoreMatcher(IgnoreMatcherOptions{
		ProjectRoot:       projectDir,
		TagconfigPatterns: cfg.SkipPatterns,
	})
	if err != nil {
		return nil, fmt.Errorf("build ignore matcher: %w", err)
	}

	// Run 3-way merge. Adapt IgnoreMatcher to MergeEngine's signature
	// (files in the merge tree are never directories).
	ignoreFn := func(path string) bool {
		return ignoreMatcher.ShouldSkip(path, false)
	}
	merger := NewMergeEngine(&GitMerger{}, ignoreFn)
	results, skipped, err := merger.MergeTrees(ctx, base, ours, theirs)
	if err != nil {
		return nil, fmt.Errorf("merge trees: %w", err)
	}

	return &DiffResult{
		OldSHA:  cfg.Template.CommitSHA,
		NewSHA:  latestSHA,
		Source:  cfg.Template.Source,
		Results: results,
		Skipped: skipped,
	}, nil
}
