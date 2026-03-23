package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"
)

// SkillDocs holds embedded skill documentation content.
type SkillDocs struct {
	Guide     string
	Reference string
	Recipes   string
}

func SkillCommand(version string, docs SkillDocs) *cli.Command {
	return &cli.Command{
		Name:  "skills",
		Usage: "Print AI coding assistant reference for TAG",
		Description: `Prints embedded skill documentation for AI coding assistants.

Use subcommands to view specific documentation:
  tag skills           Overview and quick start
  tag skills guide     Full guide (decision tree, CLI reference, pitfalls)
  tag skills reference Complete syntax, filters, variable system, hooks
  tag skills recipes   Real-world patterns and examples`,
		Action: func(c *cli.Context) error {
			return skillOverviewAction(c, os.Stdout, version)
		},
		Subcommands: []*cli.Command{
			skillDocCommand("guide", "Print the guide (decision tree, CLI reference, pitfalls)", docs.Guide),
			skillDocCommand("reference", "Print the full reference (syntax, filters, variables, hooks)", docs.Reference),
			skillDocCommand("recipes", "Print recipes (real-world patterns and examples)", docs.Recipes),
		},
	}
}

func skillDocCommand(name, usage, content string) *cli.Command {
	return &cli.Command{
		Name:  name,
		Usage: usage,
		Action: func(_ *cli.Context) error {
			_, err := fmt.Fprint(os.Stdout, content)
			return err
		},
	}
}

func skillOverviewAction(_ *cli.Context, w io.Writer, version string) error {
	msg := fmt.Sprintf(`---

## TAG %s - Reference for AI Coding Assistants

TAG is a CLI for template-driven code generation and project scaffolding.

### Available Documentation

| Subcommand          | Description                                              |
|---------------------|----------------------------------------------------------|
| tag skills guide    | Decision tree, generator/bundle anatomy, CLI quick ref   |
| tag skills reference| Full syntax, filters, variable system, hooks, remotes    |
| tag skills recipes  | Real-world patterns: CRUD bundles, inject, scaffolds     |

### Quick Start

`+"```"+`
tag scaffold [template] [project-name]   # Create a new project
tag generate <name> <entity>             # Run a generator or bundle
tag template list                        # List available templates
tag template info <template>             # Show template metadata
`+"```"+`

Run `+"`tag --help`"+` for the full command reference.
`, version)

	_, err := fmt.Fprint(w, msg)
	return err
}
