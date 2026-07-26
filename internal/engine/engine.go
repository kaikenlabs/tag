package engine

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"

	"github.com/mattn/go-isatty"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/history"
	"github.com/kaikenlabs/tag/internal/parse"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/writer"
)

// NewGenerator creates a Generator with the standard pipeline.
// It creates a new template engine, loads templates, and wires up the parser and writer.
func NewGenerator(dryRun bool, dirPath, sharedPath string, out io.Writer) (Generator, error) {
	tmplEngine, err := template.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("cannot create template engine: %w", err)
	}
	return NewGeneratorWithEngine(tmplEngine, dryRun, dirPath, sharedPath, out)
}

// NewGeneratorWithEngine creates a Generator using an existing template engine.
// This allows sharing a template engine (and its cache) across multiple generators,
// such as when running a bundle of generators.
func NewGeneratorWithEngine(tmplEngine *template.Engine, dryRun bool, dirPath, sharedPath string, out io.Writer) (Generator, error) {
	if dryRun {
		slog.Info(chalk.Cyan("DRYRUN MODE"))
	}

	// Load primary templates
	templates, err := LoadTemplateFiles(dirPath)
	if err != nil {
		return nil, fmt.Errorf("cannot load templates: %w", err)
	}

	// Load shared templates (non-fatal)
	sharedTemplates, sharedErr := LoadTemplateFiles(sharedPath)
	if sharedErr != nil {
		slog.Debug("shared templates not loaded", "path", sharedPath, "error", sharedErr)
	}

	// Wire shared templates into loader if any were loaded
	if len(sharedTemplates) > 0 {
		loader := template.CreateMemoryLoaderFromMap(sharedTemplates)
		tmplEngine.SetLoader(loader)
		tmplEngine.SetSharedContent(sharedTemplates)
		slog.Debug("loaded shared templates", "count", len(sharedTemplates))
	}

	// Create parser and writer with injected dependencies.
	// In dry-run mode, pass the output writer so diffs are shown alongside engine messages.
	parser := NewParserWithExecutor(tmplEngine, templates, sharedTemplates)
	writerOpts := dryRunWriterOpts(dryRun, out)
	w, err := writer.NewFileWriter(dryRun, writerOpts...)
	if err != nil {
		return nil, fmt.Errorf("cannot create writer: %w", err)
	}

	core := NewCore(parser, w, out)
	return &core, nil
}

// NewGeneratorWithRecorder creates a Generator that records file operations
// into rec. It is identical to NewGeneratorWithEngine but wraps the FileWriter
// with a history.RecordingFileWriter.
func NewGeneratorWithRecorder(tmplEngine *template.Engine, dryRun bool, dirPath, sharedPath string, rec *history.Recorder, out io.Writer) (Generator, error) {
	if dryRun {
		slog.Info(chalk.Cyan("DRYRUN MODE"))
	}

	templates, err := LoadTemplateFiles(dirPath)
	if err != nil {
		return nil, fmt.Errorf("cannot load templates: %w", err)
	}

	sharedTemplates, sharedErr := LoadTemplateFiles(sharedPath)
	if sharedErr != nil {
		slog.Debug("shared templates not loaded", "path", sharedPath, "error", sharedErr)
	}

	if len(sharedTemplates) > 0 {
		loader := template.CreateMemoryLoaderFromMap(sharedTemplates)
		tmplEngine.SetLoader(loader)
		tmplEngine.SetSharedContent(sharedTemplates)
		slog.Debug("loaded shared templates", "count", len(sharedTemplates))
	}

	parser := NewParserWithExecutor(tmplEngine, templates, sharedTemplates)
	writerOpts := dryRunWriterOpts(dryRun, out)
	w, err := writer.NewFileWriter(dryRun, writerOpts...)
	if err != nil {
		return nil, fmt.Errorf("cannot create writer: %w", err)
	}

	fw := w
	// In dry-run mode nothing is written, so recording (which hashes files
	// after write) would fail and abort the run — skip it. History is only
	// persisted for real runs anyway (see generate.go).
	if rec != nil && !dryRun {
		fw = history.NewRecordingFileWriter(w, rec)
	}

	core := NewCore(parser, fw, out)
	return &core, nil
}

// NewCore creates a Core engine with injected dependencies.
// This is the preferred constructor, enabling dependency injection for testing.
func NewCore(parser TemplateParser, fwr writer.FileWriter, out io.Writer) Core {
	return Core{
		parser: parser,
		fwr:    fwr,
		out:    out,
	}
}

// dryRunWriterOpts returns WriterOption slice for dry-run mode.
// When dryRun is true, diff output is directed to out and TTY is auto-detected.
func dryRunWriterOpts(dryRun bool, out io.Writer) []writer.WriterOption {
	if !dryRun {
		return nil
	}
	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	return []writer.WriterOption{writer.WithDiffOutput(out, os.Stdin, isTTY)}
}

// Generate generates code from templates using the Gonja-based engine.
func (c *Core) Generate(data Data) (GenerateResult, error) {
	var result GenerateResult

	// Build input data for the parser
	meta, _ := parse.ParseKeyValues(data.RawMeta, false)
	input := InputData{
		Name:         data.Name,
		Meta:         meta,
		ScaffoldVars: data.ScaffoldVars,
	}

	// Parse all templates
	parsedOutput, err := c.parser.Parse(input)
	if err != nil {
		slog.Error("cannot parse input data", "error", err)
		return result, err
	}

	// Phase 1: pre-scan for conflicts when policy is fail (or default).
	// This ensures the command is atomic — we never write anything if a conflict exists.
	if data.OnExisting.isFail() {
		if err := c.checkConflicts(parsedOutput); err != nil {
			return result, err
		}
	}

	// Phase 2: write files, applying policy for create actions.
	for _, item := range parsedOutput {
		switch item.Action {
		case template.ActionOpenAPI:
			mergeResult, err := c.fwr.MergeOpenAPIFile(item.To, item.Output, writer.OpenAPIMergeOptions{
				ValidateResult: item.Validate,
			})
			if err != nil {
				slog.Error("cannot merge openapi spec", "file", item.To, "error", err)
				return result, err
			}
			if mergeResult.Changed {
				slog.Info(chalk.Yellow("merged"), "file", item.To,
					"paths", len(mergeResult.AddedPaths),
					"schemas", len(mergeResult.AddedSchemas))
				result.Modified++
				result.Details = append(result.Details, FileOpDetail{Path: item.To, Op: "merged"})
			} else {
				slog.Info(chalk.Yellow("unchanged"), "file", item.To)
				result.Skipped++
				result.Details = append(result.Details, FileOpDetail{Path: item.To, Op: "skipped"})
			}

		case template.ActionAppend:
			if err := c.fwr.AppendFile(item.To, item.Output); err != nil {
				slog.Error("cannot append to file", "file", item.To, "error", err)
				return result, err
			}
			slog.Info(chalk.Yellow("modified"), "file", item.To)
			result.Modified++
			result.Details = append(result.Details, FileOpDetail{Path: item.To, Op: "modified"})

		case template.ActionInject:
			if err := c.fwr.InjectIntoFile(item.To, item.Output, writer.Inject{
				Matcher: item.InjectMatcher,
				Clause:  item.InjectClause,
			}); err != nil {
				slog.Error("cannot inject to file", "file", item.To, "error", err)
				return result, err
			}
			slog.Info(chalk.Yellow("modified"), "file", item.To)
			result.Modified++
			result.Details = append(result.Details, FileOpDetail{Path: item.To, Op: "modified"})

		default: // create
			if err := c.applyCreatePolicy(item, data.OnExisting, &result); err != nil {
				return result, err
			}
		}
	}

	return result, nil
}

// checkConflicts scans all create-action items for existing files and returns
// a ConflictError if any are found. Used for atomic pre-scan in fail mode.
func (c *Core) checkConflicts(items []TemplateData) error {
	var conflicts []string
	for _, item := range items {
		if item.Action == template.ActionAppend || item.Action == template.ActionInject || item.Action == template.ActionOpenAPI {
			continue
		}
		if _, statErr := os.Stat(item.To); statErr == nil {
			conflicts = append(conflicts, item.To)
		}
	}
	if len(conflicts) > 0 {
		slices.Sort(conflicts)
		return &ConflictError{Files: conflicts}
	}
	return nil
}

// applyCreatePolicy handles the create action with the configured OnExistingPolicy.
// Returns (true, nil) when the item was handled (skip/overwrite) and the caller
// should continue to the next item. Returns (false, nil) when the file does not
// exist and the caller should proceed with a normal WriteFile call.
func (c *Core) applyCreatePolicy(item TemplateData, policy OnExistingPolicy, result *GenerateResult) error {
	_, statErr := os.Stat(item.To)
	fileExists := statErr == nil

	if !fileExists {
		if writeErr := c.fwr.WriteFile(item.To, item.Output, types.FileMode); writeErr != nil {
			slog.Error("cannot write to file", "file", item.To, "error", writeErr)
			return writeErr
		}
		slog.Info(chalk.Blue("created"), "file", item.To)
		result.Created++
		result.Details = append(result.Details, FileOpDetail{Path: item.To, Op: "created"})
		if item.Notes != "" {
			fmt.Fprintf(c.out, "%s\n%s: %s\n", chalk.Red("IMPORTANT"), chalk.Yellow(item.To), chalk.Green(item.Notes))
		}
		return nil
	}

	switch policy {
	case OnExistingSkip:
		slog.Info(chalk.Yellow("skipped"), "file", item.To)
		result.Skipped++
		result.Details = append(result.Details, FileOpDetail{Path: item.To, Op: "skipped"})
		return nil
	case OnExistingOverwrite:
		if writeErr := c.fwr.WriteFile(item.To, item.Output, types.FileMode); writeErr != nil {
			slog.Error("cannot overwrite file", "file", item.To, "error", writeErr)
			return writeErr
		}
		slog.Info(chalk.Yellow("overwritten"), "file", item.To)
		result.Overwritten++
		result.Details = append(result.Details, FileOpDetail{Path: item.To, Op: "overwritten"})
		return nil
	default:
		// isFail() pre-scan should have caught this; guard for safety.
		return &ConflictError{Files: []string{item.To}}
	}
}
