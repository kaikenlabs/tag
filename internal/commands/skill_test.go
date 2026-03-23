package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestUT_SkillOverviewAction_PrintsOverview(t *testing.T) {
	var buf bytes.Buffer
	err := skillOverviewAction(nil, &buf, "v1.0.0")
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "TAG v1.0.0")
	assert.Contains(t, output, "tag skills guide")
	assert.Contains(t, output, "tag skills reference")
	assert.Contains(t, output, "tag skills recipes")
	assert.Contains(t, output, "Quick Start")
}

func TestUT_SkillSubcommand_Guide(t *testing.T) {
	docs := SkillDocs{
		Guide:     "# Guide\nThis is the guide doc.",
		Reference: "ref content",
		Recipes:   "recipes content",
	}

	var buf bytes.Buffer
	app := &cli.App{
		Commands: []*cli.Command{
			skillCommandWithWriter("v1.0.0", docs, &buf),
		},
	}

	err := app.Run([]string{"app", "skills", "guide"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "# Guide")
}

func TestUT_SkillSubcommand_Reference(t *testing.T) {
	docs := SkillDocs{
		Guide:     "guide content",
		Reference: "# Reference\nFull reference here.",
		Recipes:   "recipes content",
	}

	var buf bytes.Buffer
	app := &cli.App{
		Commands: []*cli.Command{
			skillCommandWithWriter("v1.0.0", docs, &buf),
		},
	}

	err := app.Run([]string{"app", "skills", "reference"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "# Reference")
}

func TestUT_SkillSubcommand_Recipes(t *testing.T) {
	docs := SkillDocs{
		Guide:     "guide content",
		Reference: "ref content",
		Recipes:   "# Recipes\nCRUD bundle pattern.",
	}

	var buf bytes.Buffer
	app := &cli.App{
		Commands: []*cli.Command{
			skillCommandWithWriter("v1.0.0", docs, &buf),
		},
	}

	err := app.Run([]string{"app", "skills", "recipes"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "# Recipes")
}

// skillCommandWithWriter creates a SkillCommand that writes subcommand output
// to the provided buffer instead of os.Stdout, for testing.
func skillCommandWithWriter(version string, docs SkillDocs, buf *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:  "skills",
		Usage: "Print AI coding assistant reference for TAG",
		Action: func(c *cli.Context) error {
			return skillOverviewAction(c, buf, version)
		},
		Subcommands: []*cli.Command{
			skillDocCommandWithWriter("guide", "", docs.Guide, buf),
			skillDocCommandWithWriter("reference", "", docs.Reference, buf),
			skillDocCommandWithWriter("recipes", "", docs.Recipes, buf),
		},
	}
}

func skillDocCommandWithWriter(name, usage, content string, buf *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:  name,
		Usage: usage,
		Action: func(_ *cli.Context) error {
			_, err := buf.WriteString(content)
			return err
		},
	}
}
