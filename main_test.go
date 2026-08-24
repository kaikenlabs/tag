package main

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/commands"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/pkg/app"
)

func TestUT_SetLogger_DefaultLevel(t *testing.T) {
	// Ensure ENV is not set so we get the default info level.
	t.Setenv("ENV", "")

	setLogger()

	// After setLogger, the default logger should be set.
	// We can verify by checking that debug messages are NOT enabled.
	assert.False(t, slog.Default().Enabled(context.Background(), slog.LevelDebug))
}

func TestUT_SetLogger_DevLevel(t *testing.T) {
	t.Setenv("ENV", "DEV")

	setLogger()

	// In DEV mode, debug should be enabled.
	assert.True(t, slog.Default().Enabled(context.Background(), slog.LevelDebug))
}

func TestUT_SetLogger_NonDevEnv(t *testing.T) {
	// Set ENV to something other than DEV — should behave like default.
	t.Setenv("ENV", "PRODUCTION")

	setLogger()

	assert.False(t, slog.Default().Enabled(context.Background(), slog.LevelDebug))
}

func TestUT_HandleExitError_IsNoop(t *testing.T) {
	t.Parallel()

	// handleExitError should not panic or do anything.
	handleExitError(nil, nil)
	handleExitError(nil, assert.AnError)
}

func TestUT_AppBuilds_HasExpectedCommands(t *testing.T) {
	cfg := &config.Config{
		Env: config.Env{
			Path:       ".tag",
			SharedPath: "_shared",
			BundlePath: "_bundles",
		},
	}

	tag := &cli.App{
		Version:              "test",
		Name:                 AppName,
		Usage:                "Template-driven code generation and project scaffolding",
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			commands.GenerateCommand(cfg),
			commands.ScaffoldCommand("test"),
			commands.TemplateCommand(cfg, "test"),
			commands.LibCommand(),
			commands.DialectCommand(),
			commands.CacheCommand(),
			commands.ConvertCommand(),
			commands.ExtractCommand(),
			commands.UndoCommand(),
			commands.CheckCommand(),
			commands.DiffCommand(),
			commands.UpdateTemplateCommand(),
			commands.VersionCommand("test"),
			commands.UpgradeCommand("test"),
			commands.TestCommand(),
			commands.DoctorCommand("test"),
			commands.SkillCommand("test", commands.SkillDocs{
				Guide:     skillDoc,
				Reference: referenceDoc,
				Recipes:   recipesDoc,
			}),
		},
		ExitErrHandler: handleExitError,
	}

	require.NotNil(t, tag)
	assert.Equal(t, AppName, tag.Name)
	assert.Equal(t, "test", tag.Version)

	// Check that all expected commands are registered.
	cmdNames := make(map[string]bool, len(tag.Commands))
	for _, cmd := range tag.Commands {
		cmdNames[cmd.Name] = true
	}

	expectedCommands := []string{
		"generate", "scaffold", "template", "lib", "dialect",
		"cache", "convert", "extract", "undo", "check", "diff",
		"update", "version", "upgrade", "test", "doctor", "skills",
	}
	for _, name := range expectedCommands {
		assert.True(t, cmdNames[name], "expected command %q to be registered", name)
	}
}

func TestUT_AppName_IsTag(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "tag", AppName)
}

func TestUT_VersionDefault(t *testing.T) {
	t.Parallel()
	// Version is set by ldflags, so in tests it should be empty.
	assert.Empty(t, Version)
}

func TestUT_ExitCode_FromCommandError(t *testing.T) {
	t.Parallel()

	cmdErr := &app.CommandError{
		Message: "test error",
		Code:    app.ExitUsage,
	}
	assert.Equal(t, app.ExitUsage, cmdErr.ExitCode())
}

func TestUT_ExitCode_DefaultGeneral(t *testing.T) {
	t.Parallel()

	cmdErr := &app.CommandError{
		Message: "general error",
	}
	assert.Equal(t, app.ExitGeneral, cmdErr.ExitCode())
}

func TestUT_EmbeddedSkillDocs_NotEmpty(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, skillDoc, "embedded SKILL.md should not be empty")
	assert.NotEmpty(t, referenceDoc, "embedded reference.md should not be empty")
	assert.NotEmpty(t, recipesDoc, "embedded recipes.md should not be empty")
}

func TestUT_SetLogger_EnvNotSet(t *testing.T) {
	// Unset ENV entirely to exercise the ok==false branch.
	os.Unsetenv("ENV")

	setLogger()

	assert.False(t, slog.Default().Enabled(context.Background(), slog.LevelDebug))
}
