package commands

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/scaffold"
)

// reparseTrailingFlags rescans c.Args() for flag-like tokens that Go's flag
// parser silently dropped (it stops at the first non-flag argument).
// Recognised flags are applied via c.Set(); the remaining positional arguments
// are returned.
func reparseTrailingFlags(c *cli.Context, flags []cli.Flag) ([]string, error) {
	args := c.Args().Slice()

	// Build lookup tables:
	// - boolFlags: names/aliases that are boolean (set to "true", no value consumed)
	// - canonical: maps every name/alias → primary (first) name
	//
	// We always c.Set the canonical name because urfave/cli v2 registers
	// separate flag.Value instances per name when Destination is nil,
	// so setting an alias doesn't update the canonical name's value.
	boolFlags := make(map[string]bool)
	canonical := make(map[string]string)

	for _, f := range flags {
		names := f.Names()
		if len(names) == 0 {
			continue
		}
		primary := names[0]
		for _, n := range names {
			canonical[n] = primary
		}
		if _, ok := f.(*cli.BoolFlag); ok {
			for _, n := range names {
				boolFlags[n] = true
			}
		}
	}

	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// "--" terminates flag parsing; everything after is positional.
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}

		// Not a flag → positional argument.
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}

		// Strip leading dashes to get the flag name.
		flagName := strings.TrimLeft(arg, "-")

		// Handle --flag=value / -flag=value.
		if name, val, ok := strings.Cut(flagName, "="); ok {
			setName := canonical[name]
			if setName == "" {
				setName = name
			}
			if err := c.Set(setName, val); err != nil {
				return nil, fmt.Errorf("setting flag %s: %w", name, err)
			}
			continue
		}

		// Resolve to canonical name for c.Set.
		setName := canonical[flagName]
		if setName == "" {
			setName = flagName
		}

		// Boolean flag → set to "true".
		if boolFlags[flagName] {
			if err := c.Set(setName, "true"); err != nil {
				return nil, fmt.Errorf("setting flag %s: %w", flagName, err)
			}
			continue
		}

		// Value flag → consume the next argument.
		if i+1 >= len(args) {
			return nil, fmt.Errorf("flag -%s requires a value", flagName)
		}
		i++
		if err := c.Set(setName, args[i]); err != nil {
			return nil, fmt.Errorf("setting flag %s: %w", flagName, err)
		}
	}
	return positional, nil
}

// commonScaffoldFlags returns flags shared between the scaffold and run commands.
func commonScaffoldFlags() []cli.Flag {
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
			Usage: "Accept and run pre/post scaffold hooks without prompting for confirmation",
		},
		&cli.BoolFlag{
			Name:  "allow-recursive-render",
			Usage: "Allow template syntax in variable values to be rendered (SECURITY: enables recursive template rendering)",
		},
	}
}

// buildScaffoldOpts reads common flags from the CLI context and returns a scaffold.Options
// with all shared fields populated. Callers set only the differing fields (TemplateRef, IsRemote).
func buildScaffoldOpts(c *cli.Context, templateDir, projectName string, meta map[string]string) scaffold.Options {
	return scaffold.Options{
		TemplateDir:          templateDir,
		OutputDir:            c.String("output"),
		ProjectName:          projectName,
		ValuesFile:           c.String("values"),
		Meta:                 meta,
		NoInput:              c.Bool("no-input"),
		Force:                c.Bool("force"),
		Replay:               c.Bool("replay"),
		NoSave:               c.Bool("no-save"),
		AcceptHooks:          c.Bool("accept-hooks"),
		AllowRecursiveRender: c.Bool("allow-recursive-render"),
		IsTTY:                scaffold.IsTTY(),
	}
}
