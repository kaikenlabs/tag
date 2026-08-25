package vars

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/types"
)

// varNamePattern is the set of names Gonja can reach through dot access, and so
// the set this command is able to rename.
var varNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Change is a single line-level edit within a file.
type Change struct {
	// Line is the 1-based line number, or 0 for a whole-path rename.
	Line   int    `json:"line"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// FileChange collects every edit for one file.
type FileChange struct {
	// Path is the file's location relative to the template root.
	Path string `json:"path"`
	// NewPath is set when the file or one of its parent directories carries a
	// {{ vars.x }} placeholder and therefore moves on disk.
	NewPath string `json:"new_path,omitempty"`
	// Changes lists the content edits; empty for a pure path rename.
	Changes []Change `json:"changes"`
	// Replacements counts individual substitutions, which exceeds len(Changes)
	// when a single line mentions the variable more than once.
	Replacements int `json:"replacements"`

	content  []byte
	original []byte
}

// RenamePlan is the complete set of edits for one variable rename. Planning is
// read-only, so a plan can be printed for --dry-run and discarded.
type RenamePlan struct {
	Root    string       `json:"root"`
	OldName string       `json:"old_name"`
	NewName string       `json:"new_name"`
	Files   []FileChange `json:"files"`
}

// FileCount returns the number of files the plan touches.
func (p *RenamePlan) FileCount() int {
	return len(p.Files)
}

// ReplacementCount returns the total number of individual replacements, counting
// a moved path as one.
func (p *RenamePlan) ReplacementCount() int {
	total := 0
	for _, f := range p.Files {
		total += f.Replacements
		if f.NewPath != "" {
			total++
		}
	}
	return total
}

// PathRenameCount returns the number of files whose on-disk path changes.
func (p *RenamePlan) PathRenameCount() int {
	n := 0
	for _, f := range p.Files {
		if f.NewPath != "" {
			n++
		}
	}
	return n
}

// PlanRename computes every edit needed to rename oldName to newName across a
// TAG template, without touching the filesystem.
//
// The rename covers the whole template tree, including _generators/ (generators
// inherit root variables) and .tag/ (its bundle manifests reference scaffold
// variables by name). Files excluded from scaffold output by .tagignore are left
// alone, as are _dialects/, .git/ and binary files.
func PlanRename(root, oldName, newName string) (*RenamePlan, error) {
	absRoot, err := validateRenameRoot(root, oldName, newName)
	if err != nil {
		return nil, err
	}

	plan := &RenamePlan{Root: absRoot, OldName: oldName, NewName: newName}

	ignoreMatcher, err := loadIgnorePatterns(absRoot)
	if err != nil {
		return nil, fmt.Errorf("load ignore patterns: %w", err)
	}

	walkErr := filepath.WalkDir(absRoot, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, relErr := filepath.Rel(absRoot, srcPath)
		if relErr != nil {
			return fmt.Errorf("relative path: %w", relErr)
		}
		if skip, skipDir := shouldSkipRenameEntry(relPath, d, ignoreMatcher); skip {
			if skipDir {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		change, planErr := planFile(absRoot, relPath, oldName, newName)
		if planErr != nil {
			return planErr
		}
		if change != nil {
			plan.Files = append(plan.Files, *change)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	slices.SortFunc(plan.Files, func(a, b FileChange) int {
		return strings.Compare(a.Path, b.Path)
	})

	if err := checkPathCollisions(absRoot, plan); err != nil {
		return nil, err
	}

	return plan, nil
}

// validateRenameRoot checks the arguments and the template root, returning the
// absolute root path.
func validateRenameRoot(root, oldName, newName string) (string, error) {
	if oldName == newName {
		return "", fmt.Errorf("old and new names are identical: %q", oldName)
	}
	for _, name := range []string{oldName, newName} {
		if !varNamePattern.MatchString(name) {
			return "", fmt.Errorf("%q is not a valid variable name "+
				"(letters, digits and underscores, not starting with a digit)", name)
		}
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("path does not exist: %s", root)
		}
		return "", fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", root)
	}

	absRoot, err = fileutil.ResolveSymlinkedRoot(absRoot)
	if err != nil {
		return "", err
	}

	config, err := loadConfig(absRoot)
	if err != nil {
		return "", err
	}
	if _, ok := config.Vars[oldName]; !ok {
		return "", fmt.Errorf("variable %q is not declared in %s",
			oldName, types.TemplateConfigFile)
	}
	if _, ok := config.Vars[newName]; ok {
		return "", fmt.Errorf("variable %q is already declared in %s",
			newName, types.TemplateConfigFile)
	}

	return absRoot, nil
}

// shouldSkipRenameEntry decides whether an entry is out of scope for the rename
// and whether its whole subtree can be pruned.
func shouldSkipRenameEntry(
	relPath string, d fs.DirEntry, ignoreMatcher gitignore.Matcher,
) (skip, skipDir bool) {
	if relPath == "." {
		return true, false
	}
	// Rewriting through a symlink would edit a file outside the template.
	if d.Type()&os.ModeSymlink != 0 {
		return true, false
	}

	name := d.Name()
	if name == types.TagIgnoreFile || name == types.CacheMetaFile {
		return true, false
	}
	// Dialects define type mappings, not variable references; .git is not
	// template content at all.
	if d.IsDir() && (name == types.DialectsDir || name == ".git") {
		return true, true
	}

	if ignoreMatcher != nil {
		components := strings.Split(relPath, string(filepath.Separator))
		if ignoreMatcher.Match(components, d.IsDir()) {
			return true, d.IsDir()
		}
	}

	return false, false
}

// planFile computes the edits for a single file, returning nil when nothing
// changes.
func planFile(absRoot, relPath, oldName, newName string) (*FileChange, error) {
	newRelPath, _ := renameInExpressions(relPath, oldName, newName)

	// #nosec G304 -- relPath comes from walking absRoot, the template the user named
	original, err := os.ReadFile(filepath.Join(absRoot, relPath))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", relPath, err)
	}

	change := FileChange{Path: relPath, original: original}
	if newRelPath != relPath {
		change.NewPath = newRelPath
	}

	if fileutil.IsTextContent(original) {
		updated, changes, count, cErr := planContent(relPath, original, oldName, newName)
		if cErr != nil {
			return nil, cErr
		}
		if len(changes) > 0 {
			change.Changes = changes
			change.Replacements = count
			change.content = updated
		}
	}

	if change.NewPath == "" && len(change.Changes) == 0 {
		return nil, nil //nolint:nilnil // nil change means "file unaffected"
	}
	return &change, nil
}

// planContent rewrites one file's bytes and derives the per-line diff. Because
// no rewrite step adds or removes newlines, old and new content always have the
// same number of lines and can be compared index-wise.
func planContent(
	relPath string, original []byte, oldName, newName string,
) (updated []byte, changes []Change, count int, err error) {
	updated = original

	// Config and bundle manifests record variable names as data (a `vars` key,
	// a `requires` entry) as well as inside expressions.
	if isDeclarationFile(relPath) {
		spliced, n, splErr := renameDeclarations(relPath, updated, oldName, newName)
		if splErr != nil {
			return nil, nil, 0, splErr
		}
		updated, count = spliced, n
	}

	rewritten, exprCount := renameInExpressions(string(updated), oldName, newName)
	updated = []byte(rewritten)
	count += exprCount

	return updated, diffLines(string(original), string(updated)), count, nil
}

// renameDeclarations rewrites the JSON declarations in one config or bundle
// manifest, refusing to proceed when the file already declares the new name.
// Splicing over an existing declaration would produce a duplicate JSON key, and
// a map-based parse silently keeps only one of them.
func renameDeclarations(relPath string, data []byte, oldName, newName string) ([]byte, int, error) {
	oldSpans, err := findDeclarationSpans(data, oldName)
	if err != nil {
		return nil, 0, fmt.Errorf("rewrite %s: %w", relPath, err)
	}
	if len(oldSpans) == 0 {
		return data, 0, nil
	}

	newSpans, err := findDeclarationSpans(data, newName)
	if err != nil {
		return nil, 0, fmt.Errorf("rewrite %s: %w", relPath, err)
	}
	if len(newSpans) > 0 {
		return nil, 0, fmt.Errorf("%s: %q is already declared there", relPath, newName)
	}

	spliced, count, err := renameJSONDeclarations(data, oldName, newName)
	if err != nil {
		return nil, 0, fmt.Errorf("rewrite %s: %w", relPath, err)
	}
	return spliced, count, nil
}

// diffLines pairs up differing lines of two same-line-count strings.
func diffLines(before, after string) []Change {
	if before == after {
		return nil
	}
	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(after, "\n")
	if len(beforeLines) != len(afterLines) {
		// Defensive: a rewrite that changed the line count would make line
		// numbers meaningless, so report the file as a single change.
		return []Change{{Line: 0, Before: before, After: after}}
	}

	var changes []Change
	for i := range beforeLines {
		if beforeLines[i] != afterLines[i] {
			changes = append(changes, Change{
				Line:   i + 1,
				Before: beforeLines[i],
				After:  afterLines[i],
			})
		}
	}
	return changes
}

// isDeclarationFile reports whether a file declares variables as JSON data:
// any tag.template.json, or any JSON file inside a _bundles/ directory.
func isDeclarationFile(relPath string) bool {
	base := filepath.Base(relPath)
	if base == types.TemplateConfigFile {
		return true
	}
	if filepath.Ext(base) != ".json" {
		return false
	}
	return slices.Contains(strings.Split(relPath, string(filepath.Separator)), types.BundlesDir)
}

// checkPathCollisions fails the plan when a renamed path would land on an
// existing entry that the rename does not itself move away.
func checkPathCollisions(absRoot string, plan *RenamePlan) error {
	moving := make(map[string]struct{}, len(plan.Files))
	for _, f := range plan.Files {
		if f.NewPath != "" {
			moving[f.Path] = struct{}{}
		}
	}

	claimed := make(map[string]string, len(plan.Files))
	for _, f := range plan.Files {
		if f.NewPath == "" {
			continue
		}
		// Two sources renaming onto one destination would silently overwrite
		// each other during Apply, so refuse the whole plan.
		if other, ok := claimed[f.NewPath]; ok {
			return fmt.Errorf("cannot rename %s and %s to the same path %s",
				other, f.Path, f.NewPath)
		}
		claimed[f.NewPath] = f.Path

		if _, ok := moving[f.NewPath]; ok {
			continue
		}
		if err := checkExistingTarget(absRoot, f.Path, f.NewPath); err != nil {
			return err
		}
	}
	return nil
}

// checkExistingTarget reports a collision when the destination is already taken.
// A destination that resolves to the source itself is not a collision: that is
// what a case-only rename looks like on a case-insensitive filesystem.
func checkExistingTarget(absRoot, from, to string) error {
	dstInfo, err := os.Lstat(filepath.Join(absRoot, to))
	if err != nil {
		return nil //nolint:nilerr // no readable entry at the destination means nothing to collide with
	}
	if srcInfo, srcErr := os.Lstat(filepath.Join(absRoot, from)); srcErr == nil &&
		os.SameFile(srcInfo, dstInfo) {
		return nil
	}
	return fmt.Errorf("cannot rename %s to %s: target already exists", from, to)
}

// Apply writes the planned edits. Content is written first, then paths are moved
// deepest-first so parent directories stay valid while their children move.
//
// If any step fails, everything already done is rolled back from the originals
// captured at plan time, so a partial rename never survives the call.
func (p *RenamePlan) Apply() error {
	var undo []func() error

	fail := func(err error) error {
		var rollbackErrs []error
		for i := len(undo) - 1; i >= 0; i-- {
			if undoErr := undo[i](); undoErr != nil {
				rollbackErrs = append(rollbackErrs, undoErr)
			}
		}
		if len(rollbackErrs) > 0 {
			// The tree is now half-applied; make that loud rather than silent.
			return errors.Join(
				fmt.Errorf("%w (WARNING: rollback also failed; the template may be "+
					"partially renamed — restore it from version control)", err),
				errors.Join(rollbackErrs...))
		}
		return err
	}

	for i := range p.Files {
		f := &p.Files[i]
		if len(f.Changes) == 0 || f.content == nil {
			continue
		}
		abs := filepath.Join(p.Root, f.Path)
		info, err := os.Stat(abs)
		if err != nil {
			return fail(fmt.Errorf("stat %s: %w", f.Path, err))
		}
		if err := os.WriteFile(abs, f.content, info.Mode().Perm()); err != nil {
			return fail(fmt.Errorf("write %s: %w", f.Path, err))
		}
		original, mode := f.original, info.Mode().Perm()
		undo = append(undo, func() error { return os.WriteFile(abs, original, mode) })
	}

	for _, move := range p.pathMoves() {
		from := filepath.Join(p.Root, move.from)
		to := filepath.Join(p.Root, move.to)
		// A renamed directory placeholder means the target's parent may not
		// exist yet; recreate it with the permissions the original carried.
		created, err := mirrorParentDir(from, to)
		if err != nil {
			return fail(fmt.Errorf("create directory for %s: %w", move.to, err))
		}
		if len(created) > 0 {
			undo = append(undo, func() error {
				// created is deepest-first, so each removal sees an empty dir.
				for _, dir := range created {
					_ = os.Remove(dir)
				}
				return nil
			})
		}
		if err := os.Rename(from, to); err != nil {
			return fail(fmt.Errorf("rename %s to %s: %w", move.from, move.to, err))
		}
		undo = append(undo, func() error { return os.Rename(to, from) })
	}

	p.pruneEmptyRenamedDirs()

	return nil
}

// mirrorParentDir ensures the destination's parent directory exists, giving it
// the same permissions as the source's parent rather than a fixed mode. It
// returns the directories it created, deepest-first, so a failed Apply can
// remove them again.
func mirrorParentDir(from, to string) ([]string, error) {
	dstParent := filepath.Dir(to)
	if _, err := os.Stat(dstParent); err == nil {
		return nil, nil
	}

	var created []string
	for dir := dstParent; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(dir); err == nil {
			break
		}
		created = append(created, dir)
		if parent := filepath.Dir(dir); parent == dir {
			break
		}
	}

	mode := fs.FileMode(0o750)
	if info, err := os.Stat(filepath.Dir(from)); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(dstParent, mode); err != nil {
		return nil, fmt.Errorf("create %s: %w", dstParent, err)
	}
	return created, nil
}

type pathMove struct{ from, to string }

// pathMoves returns the file moves ordered deepest-first, so that renaming a
// file never invalidates a path still queued behind it.
func (p *RenamePlan) pathMoves() []pathMove {
	var moves []pathMove
	for _, f := range p.Files {
		if f.NewPath != "" {
			moves = append(moves, pathMove{from: f.Path, to: f.NewPath})
		}
	}
	slices.SortFunc(moves, func(a, b pathMove) int {
		return strings.Count(b.from, string(filepath.Separator)) -
			strings.Count(a.from, string(filepath.Separator))
	})
	return moves
}

// pruneEmptyRenamedDirs removes directories left behind once their contents
// moved to the renamed path. Only now-empty directories whose own name carried
// the old placeholder are removed, so unrelated empty directories survive.
func (p *RenamePlan) pruneEmptyRenamedDirs() {
	seen := map[string]struct{}{}
	var dirs []string
	for _, f := range p.Files {
		if f.NewPath == "" {
			continue
		}
		for dir := filepath.Dir(f.Path); dir != "."; dir = filepath.Dir(dir) {
			if _, ok := seen[dir]; ok {
				continue
			}
			seen[dir] = struct{}{}
			dirs = append(dirs, dir)
		}
	}
	// Deepest first, so nested leftovers clear before their parents.
	slices.SortFunc(dirs, func(a, b string) int { return len(b) - len(a) })

	for _, dir := range dirs {
		renamed, _ := renameInExpressions(dir, p.OldName, p.NewName)
		if renamed == dir {
			continue
		}
		_ = os.Remove(filepath.Join(p.Root, dir))
	}
}
