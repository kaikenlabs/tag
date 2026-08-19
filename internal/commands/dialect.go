package commands

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/dialect"
	"github.com/kaikenlabs/tag/internal/jsonout"
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

// dialectListEntry is the JSON shape for one row of `dialect list`. It
// deliberately omits provenance: the registry does not track where a dialect
// came from, and the text path's hardcoded "built-in" would mislead scripts.
type dialectListEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func dialectListFlags() []cli.Flag {
	return []cli.Flag{formatFlag(formatText, formatJSON)}
}

func dialectListCommand() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Usage:   "List available dialects",
		Aliases: []string{"ls"},
		Flags:   dialectListFlags(),
		Action: func(c *cli.Context) error {
			format, err := resolveFormat(c, formatText, formatJSON)
			if err != nil {
				return err
			}

			reg, err := loadDialectRegistryGlobal()
			if err != nil {
				return app.Errorf("failed to load dialects: %w", err)
			}

			out := cmdOut(c)
			if format == formatJSON {
				return jsonout.Write(out, map[string]any{"dialects": dialectListEntries(reg)})
			}

			printDialectList(out, reg)
			return nil
		},
	}
}

// dialectListEntries materialises the registry's dialects as JSON-ready rows.
func dialectListEntries(reg *dialect.Registry) []dialectListEntry {
	names := reg.Names()
	entries := make([]dialectListEntry, 0, len(names))
	for _, name := range names {
		d := reg.Get(name)
		entries = append(entries, dialectListEntry{Name: name, Description: d.Description})
	}
	return entries
}

func dialectShowFlags() []cli.Flag {
	return []cli.Flag{formatFlag(formatText, formatJSON)}
}

func dialectShowCommand() *cli.Command {
	return &cli.Command{
		Name:      "show",
		Usage:     "Show type mappings for a dialect",
		ArgsUsage: "<dialect>",
		Flags:     dialectShowFlags(),
		Action: func(c *cli.Context) error {
			args, err := reparseTrailingFlags(c, dialectShowFlags())
			if err != nil {
				return app.UsageErrorf("%s", err)
			}
			if len(args) == 0 {
				return app.Errorf("missing dialect name. Usage: tag dialect show <dialect>")
			}

			format, err := resolveFormat(c, formatText, formatJSON)
			if err != nil {
				return err
			}

			dialectName := args[0]

			reg, err := loadDialectRegistryGlobal()
			if err != nil {
				return app.Errorf("failed to load dialects: %w", err)
			}

			d := reg.Get(dialectName)
			if d == nil {
				return app.Errorf("unknown dialect %q. Run 'tag dialect list' to see available dialects.", dialectName)
			}

			out := cmdOut(c)
			if format == formatJSON {
				return writeDialectJSON(out, d)
			}

			printDialectShow(out, d)
			return nil
		},
	}
}

// writeDialectJSON serialises a dialect as a bare JSON object. A dialect with
// no declared types has a nil Types map; it is copied out to an empty map so
// the wire shape is {} rather than null.
func writeDialectJSON(w io.Writer, d *dialect.Dialect) error {
	payload := *d
	if payload.Types == nil {
		payload.Types = map[string]string{}
	}
	return jsonout.Write(w, payload)
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
