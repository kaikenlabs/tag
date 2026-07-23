package commands

import (
	"io"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/internal/vars"
	"github.com/kaikenlabs/tag/pkg/app"
)

func templateRenameVarCommand() *cli.Command {
	return &cli.Command{
		Name:      "rename-var",
		Usage:     "Rename a template variable across the config and all template files",
		ArgsUsage: "[--dry-run] <old-name> <new-name> [path]",
		Description: `Renames a variable everywhere a TAG template refers to it:
the declaration in tag.template.json, derived variable defaults, hook commands,
bundle "requires" entries, template bodies, conditionals and loops, and file or
directory name placeholders (which are renamed on disk).

Only Gonja expressions are rewritten. Prose that merely mentions the variable
name is left alone, as are comments, {% raw %} blocks and string literals.

Files excluded by .tagignore, the _dialects/ tree and binary files are skipped.
Generator and bundle definitions under _generators/ and .tag/ are included,
because they reference root-level variables.

If [path] is omitted, the current directory is used.

Flags must precede the positional arguments.

Examples:
  tag template rename-var --dry-run project_name service_name
  tag template rename-var project_name service_name
  tag template rename-var old_flag new_flag ./my-template`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  flags.DryRunFlag,
				Usage: "Preview all changes without modifying anything",
			},
		},
		Action: func(c *cli.Context) error {
			return renameVarAction(c, os.Stdout)
		},
	}
}

func renameVarAction(c *cli.Context, out io.Writer) error {
	if c.NArg() < 2 || c.NArg() > 3 {
		return app.UsageErrorf(
			"expected <old-name> <new-name> [path], got %d argument(s)", c.NArg())
	}

	oldName := c.Args().Get(0)
	newName := c.Args().Get(1)
	root := "."
	if c.NArg() == 3 {
		root = c.Args().Get(2)
	}

	plan, err := vars.PlanRename(root, oldName, newName)
	if err != nil {
		return app.Errorf("cannot rename variable: %w", err)
	}

	dryRun := c.Bool(flags.DryRunFlag)
	if !dryRun {
		if applyErr := plan.Apply(); applyErr != nil {
			return app.Errorf("rename failed (template left unchanged): %w", applyErr)
		}
	}

	vars.WriteRenamePlan(out, plan, dryRun)

	return nil
}
