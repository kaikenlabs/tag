package commands

import (
	"errors"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/pkg/app"
)

// RunCommand returns the run command definition.
func RunCommand() *cli.Command {
	return &cli.Command{
		Name:      "run",
		Usage:     "Scaffold a project from a library template",
		ArgsUsage: "[template-name] [project-name]",
		Description: `Scaffold a new project using a template from the library.

If no template name is given and the terminal is interactive,
a fuzzy picker is shown to select from installed templates.

EXAMPLES:
  # Pick a template interactively
  tag run

  # Scaffold from a specific library template
  tag run go-api my-service

  # With variable overrides
  tag run go-api my-service -m author="Jane Doe"`,
		Flags:  scaffoldFlags(),
		Action: runAction,
	}
}

// scaffoldFlags returns the shared scaffold flags (minus --update, which is irrelevant for library templates).
func scaffoldFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output directory (default: ./<project_name>)",
		},
		&cli.StringFlag{
			Name:  "values",
			Usage: "JSON file with variable values",
		},
		&cli.StringSliceFlag{
			Name:    "meta",
			Aliases: []string{"m"},
			Usage:   "Variable override in key=value format (can be repeated)",
		},
		&cli.BoolFlag{
			Name:  "no-input",
			Usage: "Skip interactive prompts, use defaults only",
		},
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "Overwrite output directory if it exists",
		},
		&cli.BoolFlag{
			Name:  "replay",
			Usage: "Reuse saved variable values from a previous scaffold of this template",
		},
		&cli.BoolFlag{
			Name:  "no-save",
			Usage: "Don't save variable values for future replay",
		},
		&cli.BoolFlag{
			Name:  "accept-hooks",
			Usage: "Accept and run pre/post scaffold hooks without prompting",
		},
		&cli.BoolFlag{
			Name:  "allow-recursive-render",
			Usage: "Allow template syntax in variable values to be rendered",
		},
	}
}

func runAction(c *cli.Context) error {
	lib, err := newLibrary()
	if err != nil {
		return app.Errorf("failed to initialize library: %w", err)
	}

	templateName, err := resolveTemplateName(c, lib)
	if err != nil {
		return err
	}

	projectName := ""
	if c.NArg() >= 2 {
		projectName = c.Args().Get(1)
	}

	templateDir, err := lib.TemplatePath(templateName)
	if err != nil {
		return app.Errorf("failed to find template: %w", err)
	}

	entry, err := lib.Get(templateName)
	if err != nil {
		return app.Errorf("failed to get template info: %w", err)
	}

	meta, err := scaffold.ParseMetaFlags(c.StringSlice("meta"))
	if err != nil {
		return app.Errorf("invalid meta flag: %w", err)
	}

	opts := scaffold.Options{
		TemplateDir:          templateDir,
		OutputDir:            c.String("output"),
		ProjectName:          projectName,
		ValuesFile:           c.String("values"),
		Meta:                 meta,
		NoInput:              c.Bool("no-input"),
		Force:                c.Bool("force"),
		Replay:               c.Bool("replay"),
		NoSave:               c.Bool("no-save"),
		TemplateRef:          entry.Source,
		AcceptHooks:          c.Bool("accept-hooks"),
		IsRemote:             false, // Library templates are local
		AllowRecursiveRender: c.Bool("allow-recursive-render"),
		IsTTY:                scaffold.IsTTY(),
	}

	s, err := scaffold.NewScaffold(opts)
	if err != nil {
		return app.Errorf("failed to initialize scaffold: %w", err)
	}

	if err := s.Run(opts); err != nil {
		return app.Errorf("scaffolding failed: %w", err)
	}

	return nil
}

// resolveTemplateName determines the template name from args or interactive picker.
func resolveTemplateName(c *cli.Context, lib *library.Library) (string, error) {
	switch {
	case c.NArg() >= 1:
		return c.Args().Get(0), nil
	case scaffold.IsTTY() && !c.Bool("no-input"):
		return pickTemplate(lib)
	default:
		return "", app.Errorf("template name is required\n\nUsage: tag run <template-name> [project-name]")
	}
}

// pickTemplate shows an interactive template picker.
func pickTemplate(lib *library.Library) (string, error) {
	entries, err := lib.List()
	if err != nil {
		return "", err
	}

	if len(entries) == 0 {
		return "", errors.New("no templates installed; add one with: tag lib add <ref>")
	}

	// Build display items
	type displayEntry struct {
		Name    string
		Display string
	}

	items := make([]displayEntry, len(entries))
	for i, e := range entries {
		display := e.Name
		if e.Description != "" {
			display += " - " + e.Description
		}
		display += " (" + e.Source + ")"
		items[i] = displayEntry{Name: e.Name, Display: display}
	}

	prompt := promptui.Select{
		Label: "Select a template",
		Items: items,
		Size:  10,
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "\U0001F449 {{ .Display }}",
			Inactive: "  {{ .Display }}",
			Selected: "\u2705 {{ .Name }}",
		},
		Searcher: func(input string, index int) bool {
			return strings.Contains(
				strings.ToLower(items[index].Display),
				strings.ToLower(input),
			)
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		if errors.Is(err, promptui.ErrInterrupt) {
			return "", errors.New("selection cancelled")
		}
		return "", err
	}

	return items[idx].Name, nil
}
