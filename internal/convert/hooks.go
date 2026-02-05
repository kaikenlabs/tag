package convert

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// HookKind identifies the type of hook script.
type HookKind string

const (
	HookKindPython HookKind = "python"
	HookKindShell  HookKind = "shell"
	HookKindBatch  HookKind = "batch"
)

// Standard Cookiecutter hook filenames.
// This map is kept for documentation/reference of recognized hook patterns.
var _ = map[string]struct{}{
	"pre_gen_project.py":   {},
	"post_gen_project.py":  {},
	"pre_gen_project.sh":   {},
	"post_gen_project.sh":  {},
	"pre_gen_project.bat":  {},
	"post_gen_project.bat": {},
}

// HooksProcessor handles detection and copying of Cookiecutter hooks.
type HooksProcessor struct {
	sourceDir string
	destDir   string
	dryRun    bool
}

// NewHooksProcessor creates a new hooks processor.
func NewHooksProcessor(sourceDir, destDir string, dryRun bool) *HooksProcessor {
	return &HooksProcessor{
		sourceDir: sourceDir,
		destDir:   destDir,
		dryRun:    dryRun,
	}
}

// DetectHooks finds all hook files in the source directory.
func (p *HooksProcessor) DetectHooks() ([]HookFinding, error) {
	hooksDir := filepath.Join(p.sourceDir, "hooks")

	info, err := os.Stat(hooksDir)
	if os.IsNotExist(err) {
		return nil, nil // No hooks directory
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	var findings []HookFinding

	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		finding := HookFinding{
			Path: filepath.Join("hooks", name),
		}

		// Determine hook kind
		switch {
		case strings.HasSuffix(name, ".py"):
			finding.Kind = string(HookKindPython)
			finding.Message = "Python hook detected; TAG does not execute Python hooks. " +
				"Manual conversion to shell script or Go may be required."
		case strings.HasSuffix(name, ".sh"):
			finding.Kind = string(HookKindShell)
			finding.Message = "Shell hook detected; can be referenced in tag.template.json hooks section."
		case strings.HasSuffix(name, ".bat"):
			finding.Kind = string(HookKindBatch)
			finding.Message = "Batch hook detected; Windows-only, may need conversion for cross-platform support."
		default:
			finding.Kind = "unknown"
			finding.Message = "Unknown hook type; review and convert manually if needed."
		}

		findings = append(findings, finding)
	}

	return findings, nil
}

// CopyHooks copies hook files from source to destination.
func (p *HooksProcessor) CopyHooks() ([]HookFinding, error) {
	findings, err := p.DetectHooks()
	if err != nil {
		return nil, err
	}

	if len(findings) == 0 {
		return nil, nil
	}

	// Create hooks directory in destination
	destHooksDir := filepath.Join(p.destDir, "hooks")
	if !p.dryRun {
		if err := os.MkdirAll(destHooksDir, 0o755); err != nil {
			return nil, err
		}
	}

	for i := range findings {
		srcPath := filepath.Join(p.sourceDir, findings[i].Path)
		destPath := filepath.Join(p.destDir, findings[i].Path)

		if p.dryRun {
			findings[i].IsCopied = false // Would be copied
			continue
		}

		if err := copyFileWithMode(srcPath, destPath); err != nil {
			return findings, err
		}
		findings[i].IsCopied = true
	}

	return findings, nil
}

// copyFileWithMode copies a file preserving its permissions.
func copyFileWithMode(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// IsHooksDir checks if a path is the hooks directory.
func IsHooksDir(path string) bool {
	return path == "hooks" || strings.HasPrefix(path, "hooks/") || strings.HasPrefix(path, "hooks\\")
}

// SuggestTagHooksConfig generates suggested hooks configuration for tag.template.json.
func SuggestTagHooksConfig(findings []HookFinding) (preHooks, postHooks []string) {
	for _, f := range findings {
		name := filepath.Base(f.Path)

		// Only suggest shell hooks for TAG config
		if f.Kind != string(HookKindShell) {
			continue
		}

		if strings.HasPrefix(name, "pre_") {
			preHooks = append(preHooks, "sh hooks/"+name)
		} else if strings.HasPrefix(name, "post_") {
			postHooks = append(postHooks, "sh hooks/"+name)
		}
	}
	return
}
