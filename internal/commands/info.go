package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/charmbracelet/glamour"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/pkg/app"
)

func templateInfoFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "Force refresh of cached remote templates",
		},
	}
}

// InfoCommand returns the info command definition.
func templateInfoCommand() *cli.Command {
	return &cli.Command{
		Name:      "info",
		Usage:     "Show information about a template without scaffolding",
		ArgsUsage: "<template>",
		Description: `Display metadata, variables, hooks, and documentation for a template.

Accepts all template reference formats: local paths, library names,
and remote references (gh:, gl:, bb:, Git URLs, zip URLs).

Library templates are resolved first by name, then as remote/local references.

Examples:
  # Info for a local template
  tag template info ./my-template

  # Info for an installed library template
  tag template info go-api

  # Info for a remote template
  tag template info gh:user/awesome-template

  # Force refresh of cached remote template
  tag template info gh:user/awesome-template --update`,
		Flags:        templateInfoFlags(),
		Action:       infoAction,
		BashComplete: completeLibraryTemplateNames,
	}
}

func infoAction(c *cli.Context) error {
	args, err := reparseTrailingFlags(c, templateInfoFlags())
	if err != nil {
		return app.UsageErrorf("%s", err)
	}
	if len(args) < 1 {
		return app.UsageErrorf("template reference is required\n\nUsage: tag template info <template>")
	}

	ref := args[0]

	templateDir, err := resolveTemplateDir(c, ref)
	if err != nil {
		return err
	}

	return displayTemplateInfo(os.Stdout, templateDir)
}

// resolveTemplateDir resolves a template reference to a directory path.
// It tries library lookup first, then falls back to local/remote resolution.
func resolveTemplateDir(c *cli.Context, ref string) (string, error) {
	// Try library first
	lib, err := newLocalLibrary()
	if err == nil {
		if templateDir, libErr := lib.TemplatePath(ref); libErr == nil {
			return templateDir, nil
		}
	}

	// Fall back to local/remote resolution
	resolver, err := remote.NewResolver()
	if err != nil {
		return "", app.Errorf("failed to create resolver: %w", err)
	}

	resolveResult, err := resolver.Resolve(c.Context, ref, remote.ResolveOptions{
		ForceUpdate: c.Bool("update"),
	})
	if err != nil {
		return "", app.Errorf("failed to resolve template %q: %w", ref, err)
	}

	return resolveResult.Path, nil
}

// displayTemplateInfo orchestrates the display of all template information sections.
func displayTemplateInfo(w io.Writer, templateDir string) error {
	configPath := filepath.Join(templateDir, types.TemplateConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return app.Errorf("not a TAG template: %s not found in %s", types.TemplateConfigFile, templateDir)
		}
		return app.Errorf("failed to read %s: %w", types.TemplateConfigFile, err)
	}

	config, err := scaffold.ParseTemplateConfig(data)
	if err != nil {
		return app.Errorf("failed to parse %s: %w", types.TemplateConfigFile, err)
	}

	displayMetadata(w, config)
	displayVariables(w, config)
	displayHooks(w, config)
	renderDocFile(w, templateDir, types.TemplateReadme, "README")
	renderDocFile(w, templateDir, types.TemplateHowto, "HOWTO")

	return nil
}

// displayMetadata prints the template header: name, version, description.
func displayMetadata(w io.Writer, config *scaffold.TemplateConfig) {
	if config.Name != "" {
		fmt.Fprintf(w, "Name:         %s\n", config.Name)
	}
	if config.Version != "" {
		fmt.Fprintf(w, "Version:      %s\n", config.Version)
	}
	if config.Description != "" {
		fmt.Fprintf(w, "Description:  %s\n", config.Description)
	}
}

// displayVariables prints the sorted list of template variables with types and defaults.
func displayVariables(w io.Writer, config *scaffold.TemplateConfig) {
	if len(config.Vars) == 0 {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Variables:")

	names := make([]string, 0, len(config.Vars))
	for name := range config.Vars {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		v := config.Vars[name]
		switch {
		case v.Type == scaffold.VarTypeChoice:
			fmt.Fprintf(w, "  %-20s (choice: %s)\n", name, joinOptions(v.Options))
		case v.Type != "" && v.Type != scaffold.VarTypeString:
			fmt.Fprintf(w, "  %-20s (%s)\n", name, v.Type)
		case v.Default != nil:
			fmt.Fprintf(w, "  %-20s = %v\n", name, v.Default)
		default:
			fmt.Fprintf(w, "  %-20s (string)\n", name)
		}
	}
}

// displayHooks prints pre/post scaffold hooks if present.
func displayHooks(w io.Writer, config *scaffold.TemplateConfig) {
	if config.Hooks == nil {
		return
	}

	hasHooks := len(config.Hooks.PreScaffold) > 0 || len(config.Hooks.PostScaffold) > 0
	if !hasHooks {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Hooks:")
	if len(config.Hooks.PreScaffold) > 0 {
		fmt.Fprintln(w, "  pre_scaffold:")
		for _, cmd := range config.Hooks.PreScaffold {
			fmt.Fprintf(w, "    - %s\n", cmd)
		}
	}
	if len(config.Hooks.PostScaffold) > 0 {
		fmt.Fprintln(w, "  post_scaffold:")
		for _, cmd := range config.Hooks.PostScaffold {
			fmt.Fprintf(w, "    - %s\n", cmd)
		}
	}
}

// renderDocFile renders a markdown documentation file (README.md, HOWTO.md) with glamour.
// Silently skips if the file does not exist.
func renderDocFile(w io.Writer, templateDir, filename, label string) {
	path := filepath.Join(templateDir, filename)
	content, err := os.ReadFile(path)
	if err != nil || len(content) == 0 {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "--- %s ---\n", label)

	rendered, err := glamour.Render(string(content), "auto")
	if err != nil {
		// Fallback: print raw markdown
		fmt.Fprintln(w, string(content))
		return
	}
	fmt.Fprint(w, rendered)
}

// joinOptions formats choice options for display.
func joinOptions(opts []string) string {
	if len(opts) <= 3 {
		return fmt.Sprintf("%v", opts)
	}
	return fmt.Sprintf("%v +%d more", opts[:3], len(opts)-3)
}
