package commands

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/dialect"
	"github.com/kaikenlabs/tag/pkg/app"
)

// DialectCommand returns the top-level dialect command definition.
func DialectCommand() *cli.Command {
	return &cli.Command{
		Name:    "dialect",
		Aliases: []string{"dialects"},
		Usage:   "Inspect available type-mapping dialects",
		Description: `Dialects map canonical type names (string, uuid, datetime, etc.) to
language-specific types. Use the to() filter in templates:

  {{ field.type | to("go") }}       → time.Time (for datetime)
  {{ field.type | to("postgres") }} → UUID (for uuid)

Dialects are loaded from three tiers (later overrides earlier):
  1. Built-in (6 dialects: go, postgres, mysql, typescript, openapi, protobuf)
  2. User-global (~/.local/share/tag/dialects/)
  3. Template-local (_dialects/ directory within a template)

EXAMPLES
  tag dialect list
  tag dialect show postgres`,
		Subcommands: []*cli.Command{
			dialectListCommand(),
			dialectShowCommand(),
		},
	}
}

func dialectListCommand() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Usage:   "List available dialects",
		Aliases: []string{"ls"},
		Action: func(c *cli.Context) error {
			reg, err := loadDialectRegistryGlobal()
			if err != nil {
				return app.Errorf("failed to load dialects: %w", err)
			}
			printDialectList(c.App.Writer, reg)
			return nil
		},
	}
}

func dialectShowCommand() *cli.Command {
	return &cli.Command{
		Name:      "show",
		Usage:     "Show type mappings for a dialect",
		ArgsUsage: "<dialect>",
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return app.Errorf("missing dialect name. Usage: tag dialect show <dialect>")
			}

			dialectName := c.Args().First()

			reg, err := loadDialectRegistryGlobal()
			if err != nil {
				return app.Errorf("failed to load dialects: %w", err)
			}

			d := reg.Get(dialectName)
			if d == nil {
				return app.Errorf("unknown dialect %q. Run 'tag dialect list' to see available dialects.", dialectName)
			}

			printDialectShow(c.App.Writer, d)
			return nil
		},
	}
}

func printDialectList(w io.Writer, reg *dialect.Registry) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSOURCE\tDESCRIPTION")

	for _, name := range reg.Names() {
		d := reg.Get(name)
		source := "built-in"
		description := d.Description
		fmt.Fprintf(tw, "%s\t%s\t%s\n", name, source, description)
	}

	tw.Flush()
}

func printDialectShow(w io.Writer, d *dialect.Dialect) {
	fmt.Fprintf(w, "Dialect: %s (built-in)\n", d.Name)
	if d.Description != "" {
		fmt.Fprintln(w, d.Description)
	}
	fmt.Fprintln(w)

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "CANONICAL\tTARGET")

	// Sort canonical types alphabetically.
	types := make([]string, 0, len(d.Types))
	for t := range d.Types {
		types = append(types, t)
	}
	sort.Strings(types)

	for _, t := range types {
		fmt.Fprintf(tw, "%s\t%s\n", t, d.Types[t])
	}

	tw.Flush()
}
