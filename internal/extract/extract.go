package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/pkg/app"
)

// Run extracts a generator template from the given source file.
// It reads the file, detects occurrences of the entity name in various cases,
// replaces them with template expressions, adds frontmatter, and either writes
// the result to the .tag/ directory or prints a preview (dry-run mode).
func Run(opts Options, sourcePath string) (*Result, error) {
	// Read source file.
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, app.Errorf("cannot read source file: %w", err)
	}

	// Reject binary files.
	if !fileutil.IsTextContent(content) {
		return nil, app.Errorf("source file appears to be binary: %s", sourcePath)
	}

	// Build replacement rules from the entity name.
	rules := BuildRules(opts.Name)

	// Find all occurrences.
	occurrences := FindOccurrences(content, rules)

	// Interactive filtering if requested.
	if opts.Interactive && opts.Prompter != nil {
		occurrences, err = filterInteractive(occurrences, opts.Prompter)
		if err != nil {
			return nil, err
		}
	}

	// Apply replacements.
	templated := Apply(content, occurrences)

	// Build parameterized output path.
	toPath := BuildToPath(sourcePath, rules)

	// Build frontmatter + content.
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "to: %s\n", toPath)
	sb.WriteString("---\n")
	sb.Write(templated)

	templateContent := sb.String()

	// Determine output path: .tag/<as>/<filename>
	filename := filepath.Base(sourcePath)
	templatePath := filepath.Join(opts.TagDir, opts.As, filename)

	result := &Result{
		TemplatePath: templatePath,
		ToPath:       toPath,
		Replacements: len(occurrences),
		Content:      templateContent,
	}

	// Dry-run: print preview and return.
	if opts.DryRun {
		if opts.Writer != nil {
			fmt.Fprintf(opts.Writer, "=== Dry Run — Template Preview ===\n\n")
			fmt.Fprintf(opts.Writer, "Would write to: %s\n\n", templatePath)
			fmt.Fprintln(opts.Writer, templateContent)
		}
		return result, nil
	}

	// Write the template file.
	if err := os.MkdirAll(filepath.Dir(templatePath), 0o750); err != nil {
		return nil, app.Errorf("cannot create output directory: %w", err)
	}

	// #nosec G306 -- extracted template is a source artifact meant to be readable, not executable
	if err := os.WriteFile(templatePath, []byte(templateContent), 0o644); err != nil {
		return nil, app.Errorf("cannot write template: %w", err)
	}

	return result, nil
}

// filterInteractive prompts the user for each occurrence and returns the
// accepted subset.
func filterInteractive(occurrences []Occurrence, prompter Confirmer) ([]Occurrence, error) {
	var accepted []Occurrence
	acceptAll := false

	for _, occ := range occurrences {
		if acceptAll {
			accepted = append(accepted, occ)
			continue
		}

		decision, err := prompter.Confirm(occ)
		if err != nil {
			return nil, app.Errorf("interactive prompt failed: %w", err)
		}

		switch decision {
		case DecisionYes:
			accepted = append(accepted, occ)
		case DecisionNo:
			continue
		case DecisionAll:
			acceptAll = true
			accepted = append(accepted, occ)
		case DecisionQuit:
			return accepted, nil
		}
	}

	return accepted, nil
}
