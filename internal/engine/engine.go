package engine

import (
	"fmt"
	"log/slog"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/parse"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/writer"
)

// NewGenerator creates a Generator with the standard pipeline.
// It creates a new template engine, loads templates, and wires up the parser and writer.
func NewGenerator(dryRun bool, dirPath, sharedPath string) (Generator, error) {
	tmplEngine, err := template.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("cannot create template engine: %w", err)
	}
	return NewGeneratorWithEngine(tmplEngine, dryRun, dirPath, sharedPath)
}

// NewGeneratorWithEngine creates a Generator using an existing template engine.
// This allows sharing a template engine (and its cache) across multiple generators,
// such as when running a bundle of generators.
func NewGeneratorWithEngine(tmplEngine *template.Engine, dryRun bool, dirPath, sharedPath string) (Generator, error) {
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

	// Create parser and writer with injected dependencies
	parser := NewParserWithExecutor(tmplEngine, templates, sharedTemplates)
	w, err := writer.NewFileWriter(dryRun)
	if err != nil {
		return nil, fmt.Errorf("cannot create writer: %w", err)
	}

	core := NewCore(parser, w)
	return &core, nil
}

// NewCore creates a Core engine with injected dependencies.
// This is the preferred constructor, enabling dependency injection for testing.
func NewCore(parser TemplateParser, fwr writer.FileWriter) Core {
	return Core{
		parser: parser,
		fwr:    fwr,
	}
}

// Generate generates code from templates using the Gonja-based engine.
func (c *Core) Generate(data Data) error {
	// Build input data for the parser
	input := InputData{
		Name:         data.Name,
		Args:         data.Args,
		Meta:         parseMeta(data.RawMeta),
		ScaffoldVars: data.ScaffoldVars,
	}

	// Parse all templates
	parsedOutput, err := c.parser.Parse(input)
	if err != nil {
		slog.Error("cannot parse input data", "error", err)
		return err
	}

	// Write files
	for _, item := range parsedOutput {
		var action string
		switch item.Action {
		case template.ActionAppend:
			if err := c.fwr.AppendFile(item.To, item.Output); err != nil {
				slog.Error("cannot append to file", "file", item.To, "error", err)
				return err
			}
			action = chalk.Yellow("modified")
		case template.ActionInject:
			if err := c.fwr.InjectIntoFile(item.To, item.Output, writer.Inject{
				Matcher: item.InjectMatcher,
				Clause:  item.InjectClause,
			}); err != nil {
				slog.Error("cannot inject to file", "file", item.To, "error", err)
				return err
			}
			action = chalk.Yellow("modified")
		default:
			if err := c.fwr.WriteFile(item.To, item.Output, 0o750); err != nil {
				slog.Error("cannot write to file", "file", item.To, "error", err)
				return err
			}
			action = chalk.Blue("created")
			if item.Notes != "" {
				message := fmt.Sprintf("%s\n%s: %s", chalk.Red("IMPORTANT"), chalk.Yellow(item.To), chalk.Green(item.Notes))
				fmt.Println(message)
			}
		}
		slog.Info(action, "file", item.To)
	}

	return nil
}

// parseMeta is a lenient parser: malformed entries are skipped with a warning.
func parseMeta(meta []string) map[string]string {
	result, _ := parse.ParseKeyValues(meta, false)
	return result
}
