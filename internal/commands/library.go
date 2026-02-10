package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/xdg"
	"github.com/kaikenlabs/tag/pkg/app"
)

// LibCommand returns the lib command definition with subcommands.
func LibCommand() *cli.Command {
	return &cli.Command{
		Name:    "lib",
		Aliases: []string{"library"},
		Usage:   "Manage the template library",
		Description: `Install, list, and manage project templates in a persistent library.

Templates added to the library are stored locally and can be used with 'tag run'.
Cookiecutter templates are auto-detected and converted to TAG format on add.

EXAMPLES:
  # Add a template from GitHub
  tag lib add gh:user/go-api-template

  # Add with a custom name
  tag lib add gh:user/cookiecutter-django --as django

  # List installed templates
  tag lib ls

  # Update a template from its original source
  tag lib update go-api

  # Remove a template
  tag lib rm go-api`,
		Subcommands: []*cli.Command{
			libAddCommand(),
			libListCommand(),
			libRemoveCommand(),
			libUpdateCommand(),
			libInspectCommand(),
			libEditCommand(),
		},
	}
}

func newLibrary() (*library.Library, error) {
	dataDir, err := xdg.DataHome()
	if err != nil {
		return nil, fmt.Errorf("failed to determine data directory: %w", err)
	}
	return library.New(dataDir)
}

// newLocalLibrary creates a library without a resolver (for ls, inspect, edit, rm).
// It is a package-level variable to allow test substitution.
var newLocalLibrary = defaultNewLocalLibrary

func defaultNewLocalLibrary() (*library.Library, error) {
	dataDir, err := xdg.DataHome()
	if err != nil {
		return nil, fmt.Errorf("failed to determine data directory: %w", err)
	}
	return library.NewLocal(dataDir), nil
}

func libAddCommand() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Add a template to the library",
		ArgsUsage: "<ref>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "as",
				Usage: "Override the template name in the library",
			},
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Overwrite existing template with same name",
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return app.Errorf("template reference is required\n\nUsage: tag lib add <ref>")
			}

			lib, err := newLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			result, err := lib.Add(c.Context, library.AddOptions{
				Ref:   c.Args().Get(0),
				Name:  c.String("as"),
				Force: c.Bool("force"),
			})
			if err != nil {
				return asAppError(err)
			}

			printAddResult(result)
			return nil
		},
	}
}

func printAddResult(result *library.AddResult) {
	action := "Added"
	if result.IsUpdate {
		action = "Updated"
	}

	fmt.Printf("%s template %q\n", action, result.Name)
	fmt.Printf("  Source: %s\n", result.Source)
	fmt.Printf("  Path:   %s\n", result.TemplateDir)

	if result.ConvertedFrom != "" {
		fmt.Printf("  Converted from: %s\n", result.ConvertedFrom)
	}

	if len(result.Warnings) > 0 {
		fmt.Println()
		fmt.Println("Warnings:")
		for _, w := range result.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}

	fmt.Println()
	fmt.Printf("Run with: tag run %s\n", result.Name)
}

func libListCommand() *cli.Command {
	return &cli.Command{
		Name:    "ls",
		Aliases: []string{"list"},
		Usage:   "List installed templates",
		Action: func(c *cli.Context) error {
			lib, err := newLocalLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			entries, err := lib.List()
			if err != nil {
				return asAppError(err)
			}

			if len(entries) == 0 {
				fmt.Println("No templates installed.")
				fmt.Println()
				fmt.Println("Add one with: tag lib add <ref>")
				return nil
			}

			// Print table header
			fmt.Printf("%-20s %-30s %-10s %s\n", "NAME", "SOURCE", "VERSION", "DESCRIPTION")
			fmt.Printf("%-20s %-30s %-10s %s\n", "----", "------", "-------", "-----------")

			for _, entry := range entries {
				version := entry.Version
				if version == "" {
					version = "-"
				}
				desc := truncate(entry.Description, 40)
				fmt.Printf("%-20s %-30s %-10s %s\n",
					truncate(entry.Name, 20),
					truncate(entry.Source, 30),
					truncate(version, 10),
					desc,
				)
			}

			return nil
		},
	}
}

func libRemoveCommand() *cli.Command {
	return &cli.Command{
		Name:         "rm",
		Aliases:      []string{"remove"},
		Usage:        "Remove a template from the library",
		ArgsUsage:    "<name>",
		BashComplete: completeLibraryTemplateNames,
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return app.Errorf("template name is required\n\nUsage: tag lib rm <name>")
			}

			lib, err := newLocalLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			name := c.Args().Get(0)
			if err := lib.Remove(name); err != nil {
				return asAppError(err)
			}

			fmt.Printf("Removed template %q\n", name)
			return nil
		},
	}
}

func libUpdateCommand() *cli.Command {
	return &cli.Command{
		Name:         "update",
		Usage:        "Update a template (or all templates) from the original source",
		ArgsUsage:    "[name]",
		BashComplete: completeLibraryTemplateNames,
		Action: func(c *cli.Context) error {
			lib, err := newLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			if c.NArg() >= 1 {
				return updateSingleTemplate(c, lib)
			}

			return updateAllTemplates(c, lib)
		},
	}
}

func updateSingleTemplate(c *cli.Context, lib *library.Library) error {
	name := c.Args().Get(0)
	result, err := lib.Update(c.Context, name)
	if err != nil {
		return asAppError(err)
	}
	printAddResult(result)
	return nil
}

func updateAllTemplates(c *cli.Context, lib *library.Library) error {
	results, err := lib.UpdateAll(c.Context)

	// Print successfully updated templates even on partial failure
	if len(results) > 0 {
		fmt.Printf("Updated %d template(s)\n", len(results))
		for _, r := range results {
			fmt.Printf("  - %s\n", r.Name)
		}
	}

	if err != nil {
		return asAppError(err)
	}

	return nil
}

func libInspectCommand() *cli.Command {
	return &cli.Command{
		Name:         "inspect",
		Usage:        "Show detailed information about a template",
		ArgsUsage:    "<name>",
		BashComplete: completeLibraryTemplateNames,
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return app.Errorf("template name is required\n\nUsage: tag lib inspect <name>")
			}

			lib, err := newLocalLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			name := c.Args().Get(0)
			entry, err := lib.Get(name)
			if err != nil {
				return asAppError(err)
			}

			templateDir, err := lib.TemplatePath(name)
			if err != nil {
				return asAppError(err)
			}

			printInspect(entry, templateDir)
			return nil
		},
	}
}

func printInspect(entry *library.Entry, templateDir string) {
	fmt.Printf("Name:        %s\n", entry.Name)
	fmt.Printf("Source:       %s\n", entry.Source)
	fmt.Printf("Path:         %s\n", templateDir)
	fmt.Printf("Added:        %s\n", entry.AddedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:      %s\n", entry.UpdatedAt.Format("2006-01-02 15:04:05"))

	if entry.Version != "" {
		fmt.Printf("Version:      %s\n", entry.Version)
	}
	if entry.Description != "" {
		fmt.Printf("Description:  %s\n", entry.Description)
	}
	if entry.ConvertedFrom != "" {
		fmt.Printf("Converted:    from %s\n", entry.ConvertedFrom)
	}

	// Read and display template config using typed parser
	configPath := filepath.Join(templateDir, types.TemplateConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: could not read %s: %v\n", types.TemplateConfigFile, err)
		}
		return
	}

	config, err := scaffold.ParseTemplateConfig(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not parse %s: %v\n", types.TemplateConfigFile, err)
		return
	}

	if len(config.Vars) > 0 {
		fmt.Println()
		fmt.Println("Variables:")
		names := make([]string, 0, len(config.Vars))
		for name := range config.Vars {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			v := config.Vars[name]
			switch {
			case v.Type == scaffold.VarTypeChoice:
				fmt.Printf("  %-20s (choice: %s)\n", name, joinOptions(v.Options))
			case v.Type != "" && v.Type != scaffold.VarTypeString:
				fmt.Printf("  %-20s (%s)\n", name, v.Type)
			case v.Default != nil:
				fmt.Printf("  %-20s = %v\n", name, v.Default)
			default:
				fmt.Printf("  %-20s (string)\n", name)
			}
		}
	}

	if config.Hooks != nil {
		hasHooks := len(config.Hooks.PreScaffold) > 0 || len(config.Hooks.PostScaffold) > 0
		if hasHooks {
			fmt.Println()
			fmt.Println("Hooks:")
			if len(config.Hooks.PreScaffold) > 0 {
				fmt.Println("  pre_scaffold:")
				for _, cmd := range config.Hooks.PreScaffold {
					fmt.Printf("    - %s\n", cmd)
				}
			}
			if len(config.Hooks.PostScaffold) > 0 {
				fmt.Println("  post_scaffold:")
				for _, cmd := range config.Hooks.PostScaffold {
					fmt.Printf("    - %s\n", cmd)
				}
			}
		}
	}
}

// joinOptions formats choice options for display.
func joinOptions(opts []string) string {
	if len(opts) <= 3 {
		return fmt.Sprintf("%v", opts)
	}
	return fmt.Sprintf("%v +%d more", opts[:3], len(opts)-3)
}

// asAppError wraps library errors as CommandErrors without adding redundant context.
// LibraryError already contains operation and name context, so we pass its message through directly.
func asAppError(err error) error {
	var libErr *library.LibraryError
	if errors.As(err, &libErr) {
		return app.Errorf("%s", libErr.Error())
	}
	return app.Errorf("%w", err)
}

func libEditCommand() *cli.Command {
	return &cli.Command{
		Name:         "edit",
		Usage:        "Print the template path (for editing with your preferred tools)",
		ArgsUsage:    "<name>",
		BashComplete: completeLibraryTemplateNames,
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return app.Errorf("template name is required\n\nUsage: tag lib edit <name>")
			}

			lib, err := newLocalLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			name := c.Args().Get(0)
			path, err := lib.TemplatePath(name)
			if err != nil {
				return asAppError(err)
			}

			fmt.Println(path)
			return nil
		},
	}
}
