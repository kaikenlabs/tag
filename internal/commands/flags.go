package commands

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

// Output format vocabulary shared by every command that accepts --format.
const (
	formatText = "text"
	formatJSON = "json"
	formatDOT  = "dot"
)

// formatFlag builds the --format flag. The usage string is derived from the
// allowed values so a command cannot advertise a vocabulary it does not accept.
func formatFlag(allowed ...string) *cli.StringFlag {
	return &cli.StringFlag{
		Name:  "format",
		Usage: "Output format: " + humanList(allowed),
		Value: formatText,
	}
}

// resolveFormat validates the --format value against the command's allowed set.
//
// It must be called AFTER reparseTrailingFlags: urfave/cli stops parsing at the
// first positional, so a trailing --format is not yet visible on the context.
func resolveFormat(c *cli.Context, allowed ...string) (string, error) {
	format := c.String("format")

	// An unregistered flag and an explicitly empty one both read back as "".
	// Only the former is benign — it is what hand-built test contexts produce.
	// `--format=""` is a real invocation and must be rejected, or a consumer
	// whose $FORMAT expanded to nothing silently receives text.
	if format == "" && !c.IsSet("format") {
		return formatText, nil
	}

	if slices.Contains(allowed, format) {
		return format, nil
	}

	return "", app.UsageErrorf("unsupported format %q (use %s)", format, humanList(allowed))
}

// humanList renders {"a"} as "a", {"a","b"} as "a or b", and longer lists as
// "a, b, or c".
func humanList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " or " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", or " + items[len(items)-1]
	}
}

// cmdOut returns the sink a command should print to. It falls back to os.Stdout
// for contexts built without an App writer, which is what hand-rolled test
// contexts in this package produce.
func cmdOut(c *cli.Context) io.Writer {
	if c != nil && c.App != nil && c.App.Writer != nil {
		return c.App.Writer
	}
	return os.Stdout
}

// cmdErr returns the sink a command should send human-facing text to when
// stdout is reserved for a JSON document. It is the ErrWriter counterpart of
// cmdOut, and exists for the same reason: urfave/cli only fills ErrWriter in
// during App.Setup, so a hand-rolled context — which this package's tests are
// full of — leaves it nil, and writing to a nil io.Writer panics rather than
// failing politely.
func cmdErr(c *cli.Context) io.Writer {
	if c != nil && c.App != nil && c.App.ErrWriter != nil {
		return c.App.ErrWriter
	}
	return os.Stderr
}

// reparseTrailingFlags rescans c.Args() for flag-like tokens that Go's flag
// parser silently dropped (it stops at the first non-flag argument).
// Recognised flags are applied via c.Set(); the remaining positional arguments
// are returned.
//
// An unrecognised dash-prefixed token is an error, not a positional: a typo'd
// flag must not be silently accepted as an argument. See unknownFlagError for
// how to pass a literal dash-prefixed value.
func reparseTrailingFlags(c *cli.Context, cliFlags []cli.Flag) ([]string, error) {
	args := c.Args().Slice()

	// Build lookup tables:
	// - valueless: names/aliases that consume no value (set to "true")
	// - canonical: maps every name/alias → primary (first) name
	//
	// We always c.Set the canonical name because urfave/cli v2 registers
	// separate flag.Value instances per name when Destination is nil,
	// so setting an alias doesn't update the canonical name's value.
	//
	// Whether a flag takes a value comes from urfave/cli's own TakesValue(),
	// not from a *cli.BoolFlag type assertion: any other valueless flag type
	// would otherwise be misread as taking a value and would swallow the next
	// token.
	valueless := make(map[string]bool)
	canonical := make(map[string]string)

	register := func(flags []cli.Flag) {
		for _, f := range flags {
			names := f.Names()
			if len(names) == 0 {
				continue
			}
			primary := names[0]
			for _, n := range names {
				canonical[n] = primary
			}
			if df, ok := f.(cli.DocGenerationFlag); ok && !df.TakesValue() {
				for _, n := range names {
					valueless[n] = true
				}
			}
		}
	}

	register(cliFlags)

	// The App's own flags (--dry-run, --path, --shared-path, --bundle-path) are
	// declared in main.go, not on any command, so a command that relies on them
	// would otherwise see a trailing --dry-run as an unknown flag. Registering
	// them here is enough on its own: cli.Context.Set resolves a name through
	// the whole context lineage (see lookupFlagSet), so the value lands on
	// whichever context actually owns the flag. A command-local flag of the same
	// name still wins, because Lineage() runs child-to-parent.
	//
	// Before this, `tag generate model User --dry-run` silently DROPPED
	// --dry-run: nothing rescanned the tail. Registering only the command's own
	// flags would have converted that silent drop into a hard error instead of
	// making it work.
	if c != nil && c.App != nil {
		register(c.App.Flags)
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
			setName, known := canonical[name]
			if !known {
				return nil, unknownFlagError(name)
			}
			if err := c.Set(setName, val); err != nil {
				return nil, fmt.Errorf("setting flag %s: %w", name, err)
			}
			continue
		}

		// Resolve to canonical name for c.Set.
		setName, known := canonical[flagName]
		if !known {
			return nil, unknownFlagError(flagName)
		}

		// Valueless (boolean-like) flag → set to "true".
		if valueless[flagName] {
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

// unknownFlagError reports a dash-prefixed token that matches no flag on the
// command. Without this the token would fall through to the value-flag branch
// and be reported as "requires a value", which sends the reader looking for a
// missing argument rather than a misspelled flag.
//
// The "--" escape only survives when it follows a positional argument: given
// `cmd -- -x`, urfave/cli consumes the "--" itself and never hands it to us,
// whereas in `cmd query -- -x` parsing has already stopped and the "--" reaches
// c.Args(). The hint says "after another argument" rather than a bare "use --"
// so it does not send the reader down a path that cannot work.
func unknownFlagError(name string) error {
	return fmt.Errorf("unknown flag -%s (to pass it as a literal argument, put it after another argument and a \"--\" separator)", name)
}

// GlobalFlags returns the flags declared on the cli.App itself rather than on
// any single command: they apply to several commands and are read through the
// context lineage (e.g. generate reads --dry-run, --path, --shared-path and
// --bundle-path without declaring any of them).
//
// This lives here, rather than inline in main.go, so the test harness builds
// its App from the same list the binary does. The two had drifted: tests
// constructed an App with no flags at all, so no test could invoke
// `generate --dry-run` — it failed to parse — and the whole dry-run surface was
// unreachable from the command-level suite.
func GlobalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:    flags.DryRunFlag,
			Aliases: []string{"d"},
			Usage:   "Dry run mode (applies to: generate, convert)",
		},
		&cli.StringFlag{
			Name:    flags.PathFlag,
			Value:   ".tag",
			Usage:   "Creates the templates directory path at the root of the project.",
			EnvVars: []string{"TAG_PATH"},
		},
		&cli.StringFlag{
			Name:    flags.SharedPathFlag,
			Value:   "_shared",
			Usage:   "Shared template directory name",
			EnvVars: []string{"TAG_SHARED_PATH"},
		},
		&cli.StringFlag{
			Name:    flags.BundlePathFlag,
			Value:   "_bundles",
			Usage:   "Bundles directory name",
			EnvVars: []string{"TAG_BUNDLE_PATH"},
		},
	}
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
		&cli.BoolFlag{
			Name:  flags.UpdateLockFlag,
			Usage: "Refresh the lockfile entry for the template (accepts new version/checksum)",
		},
		&cli.BoolFlag{
			Name:  flags.IgnoreLockFlag,
			Usage: "Skip lockfile verification for this run (a warning will be printed)",
		},
		&cli.BoolFlag{
			Name:    flags.DryRunFlag,
			Aliases: []string{"d"},
			Usage:   "Preview what would be written without creating any files",
		},
		&cli.BoolFlag{
			Name:  flags.AddToLibFlag,
			Usage: "Add the template to the library after scaffolding (enables generator resolution from library)",
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
		UpdateLock:           c.Bool(flags.UpdateLockFlag),
		IgnoreLock:           c.Bool(flags.IgnoreLockFlag),
		DryRun:               c.Bool(flags.DryRunFlag),
	}
}
