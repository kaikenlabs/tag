package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
)

func SkillCommand(version string) *cli.Command {
	return &cli.Command{
		Name:  "skills",
		Usage: "Print AI coding assistant reference for TAG",
		Description: `Prints a markdown-formatted guide for AI coding assistants.

Includes links to TAG's skill, reference, and recipes documentation
so that agents can fetch and load them as context.

Example:
  tag skills`,
		Action: func(c *cli.Context) error {
			return skillAction(c, os.Stdout, version)
		},
	}
}

func skillAction(_ *cli.Context, w io.Writer, version string) error {
	ref := "refs/heads/main"
	if version != "" && version != "dev" && !strings.HasPrefix(version, "dev-") {
		v := version
		if !strings.HasPrefix(v, "v") {
			v = "v" + v
		}
		ref = "refs/tags/" + v
	}

	base := fmt.Sprintf("https://raw.githubusercontent.com/kaikenlabs/tag/%s/.skill", ref)

	msg := fmt.Sprintf(`---

## TAG - Reference for AI Coding Assistants

TAG is a CLI for template-driven code generation and project scaffolding.

### TAG Skills for AI Coding Assistants

Learn how to use TAG by loading the skills in these URLs:

| File | Description |
|------|-------------|
| [SKILL.md](%[1]s/SKILL.md) | Decision tree, generator/bundle anatomy, CLI quick reference, pitfalls |
| [reference.md](%[1]s/reference.md) | Full syntax, filters, variable system, hooks, remote templates |
| [recipes.md](%[1]s/recipes.md) | Real-world patterns: CRUD bundles, inject patterns, scaffolds |

### Quick Start

`+"```"+`
tag scaffold [template] [project-name]   # Create a new project
tag generate <name> <entity>             # Run a generator or bundle
tag template list                        # List available templates
tag template info <template>             # Show template metadata
`+"```"+`

Run `+"`tag --help`"+` for the full command reference.
`, base)

	_, err := fmt.Fprint(w, msg)

	return err
}
