package commands

import (
	"errors"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/parse"
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
		Flags:  commonScaffoldFlags(),
		Action: runAction,
	}
}

func runAction(c *cli.Context) error {
	lib, err := newLocalLibrary()
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

	// Get entry first — provides both path and source in one lookup
	entry, err := lib.Get(templateName)
	if err != nil {
		return asAppError(err)
	}

	templateDir, err := lib.TemplatePath(templateName)
	if err != nil {
		return asAppError(err)
	}

	meta, err := parse.ParseKeyValues(c.StringSlice("meta"), true)
	if err != nil {
		return app.Errorf("invalid meta flag: %w", err)
	}

	opts := buildScaffoldOpts(c, templateDir, projectName, meta)
	opts.TemplateRef = entry.Source
	opts.TemplateName = templateName
	opts.IsRemote = false // Library templates are local

	s, err := scaffold.NewScaffold(opts)
	if err != nil {
		return app.Errorf("failed to initialize scaffold: %w", err)
	}

	if err := s.Run(opts); err != nil {
		// If a library template becomes an unconverted Cookiecutter (e.g., via `tag lib edit`),
		// give a more helpful error message.
		var ccErr *scaffold.CookiecutterDetectedError
		if errors.As(err, &ccErr) {
			return app.Errorf("template %q is a Cookiecutter template; run 'tag lib update %s' to convert it", templateName, templateName)
		}
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
		return "", app.Errorf("no templates installed; add one with: tag lib add <ref>")
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
			Active:   "> {{ .Display }}",
			Inactive: "  {{ .Display }}",
			Selected: "* {{ .Name }}",
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
