package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/library"
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
				return app.Errorf("failed to add template: %w", err)
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
			lib, err := newLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			entries, err := lib.List()
			if err != nil {
				return app.Errorf("failed to list templates: %w", err)
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
				desc := entry.Description
				if len(desc) > 40 {
					desc = desc[:37] + "..."
				}
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
		Name:      "rm",
		Aliases:   []string{"remove"},
		Usage:     "Remove a template from the library",
		ArgsUsage: "<name>",
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return app.Errorf("template name is required\n\nUsage: tag lib rm <name>")
			}

			lib, err := newLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			name := c.Args().Get(0)
			if err := lib.Remove(name); err != nil {
				return app.Errorf("failed to remove template: %w", err)
			}

			fmt.Printf("Removed template %q\n", name)
			return nil
		},
	}
}

func libUpdateCommand() *cli.Command {
	return &cli.Command{
		Name:      "update",
		Usage:     "Update a template (or all templates) from the original source",
		ArgsUsage: "[name]",
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
		return app.Errorf("failed to update template: %w", err)
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
		return app.Errorf("failed to update templates: %w", err)
	}

	return nil
}

func libInspectCommand() *cli.Command {
	return &cli.Command{
		Name:      "inspect",
		Usage:     "Show detailed information about a template",
		ArgsUsage: "<name>",
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return app.Errorf("template name is required\n\nUsage: tag lib inspect <name>")
			}

			lib, err := newLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			name := c.Args().Get(0)
			entry, err := lib.Get(name)
			if err != nil {
				return app.Errorf("failed to get template: %w", err)
			}

			templateDir, err := lib.TemplatePath(name)
			if err != nil {
				return app.Errorf("failed to get template path: %w", err)
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

	// Read and display template config
	configPath := filepath.Join(templateDir, types.TemplateConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return
	}

	if vars, ok := config["vars"].(map[string]any); ok && len(vars) > 0 {
		fmt.Println()
		fmt.Println("Variables:")
		names := sortedKeys(vars)
		for _, name := range names {
			val := vars[name]
			switch v := val.(type) {
			case map[string]any:
				if t, ok := v["type"].(string); ok {
					fmt.Printf("  %-20s (%s)\n", name, t)
				} else {
					fmt.Printf("  %-20s (object)\n", name)
				}
			default:
				fmt.Printf("  %-20s = %v\n", name, val)
			}
		}
	}

	if hooks, ok := config["hooks"].(map[string]any); ok && len(hooks) > 0 {
		fmt.Println()
		fmt.Println("Hooks:")
		phases := sortedKeys(hooks)
		for _, phase := range phases {
			cmds := hooks[phase]
			fmt.Printf("  %s:\n", phase)
			if cmdList, ok := cmds.([]any); ok {
				for _, cmd := range cmdList {
					fmt.Printf("    - %v\n", cmd)
				}
			}
		}
	}
}

// sortedKeys returns the keys of a map in sorted order.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func libEditCommand() *cli.Command {
	return &cli.Command{
		Name:      "edit",
		Usage:     "Print the template path (for editing with your preferred tools)",
		ArgsUsage: "<name>",
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return app.Errorf("template name is required\n\nUsage: tag lib edit <name>")
			}

			lib, err := newLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			name := c.Args().Get(0)
			path, err := lib.TemplatePath(name)
			if err != nil {
				return app.Errorf("failed to get template path: %w", err)
			}

			fmt.Println(path)
			return nil
		},
	}
}
