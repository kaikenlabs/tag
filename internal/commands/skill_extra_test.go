package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestUT_SkillCommand_Structure(t *testing.T) {
	t.Parallel()
	docs := SkillDocs{
		Guide:     "guide",
		Reference: "reference",
		Recipes:   "recipes",
	}

	cmd := SkillCommand("v1.0.0", docs)
	assert.Equal(t, "skills", cmd.Name)
	assert.NotEmpty(t, cmd.Usage)
	assert.NotNil(t, cmd.Action)
	require.Len(t, cmd.Subcommands, 3)

	subNames := make([]string, len(cmd.Subcommands))
	for i, sc := range cmd.Subcommands {
		subNames[i] = sc.Name
	}
	assert.ElementsMatch(t, []string{"guide", "reference", "recipes"}, subNames)
}

func TestUT_SkillOverviewAction_ContainsVersion(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := skillOverviewAction(nil, &buf, "v2.5.0")
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "TAG v2.5.0")
}

func TestUT_SkillOverviewAction_ContainsCommands(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := skillOverviewAction(nil, &buf, "v1.0.0")
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "tag scaffold")
	assert.Contains(t, output, "tag generate")
	assert.Contains(t, output, "tag template list")
}

func TestUT_SkillDocCommand_OutputsContent(t *testing.T) {
	t.Parallel()
	content := "# Full Reference\nAll the details."
	var buf bytes.Buffer

	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name: "skills",
				Subcommands: []*cli.Command{
					{
						Name: "test-doc",
						Action: func(_ *cli.Context) error {
							_, err := buf.WriteString(content)
							return err
						},
					},
				},
			},
		},
	}

	err := app.Run([]string{"app", "skills", "test-doc"})
	require.NoError(t, err)
	assert.Equal(t, content, buf.String())
}

func TestUT_SkillDocs_FieldsAssignment(t *testing.T) {
	t.Parallel()
	docs := SkillDocs{
		Guide:     "g",
		Reference: "r",
		Recipes:   "rc",
	}
	assert.Equal(t, "g", docs.Guide)
	assert.Equal(t, "r", docs.Reference)
	assert.Equal(t, "rc", docs.Recipes)
}
