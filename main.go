package main

import (
	_ "embed"
	"errors"
	"log/slog"
	"os"

	"github.com/kaikenlabs/tag/pkg/prettylog"

	"github.com/kaikenlabs/tag/pkg/app"

	"github.com/kaikenlabs/tag/internal/commands"
	"github.com/kaikenlabs/tag/internal/config"

	"github.com/urfave/cli/v2"
)

const AppName = "tag"

//nolint:gochecknoglobals // version/commit/date are set by ldflags at build time
var (
	Version    string
	CommitHash string
	BuildDate  string
)

//go:embed .skills/SKILL.md
var skillDoc string

//go:embed .skills/reference.md
var referenceDoc string

//go:embed .skills/recipes.md
var recipesDoc string

func main() {
	if Version == "" {
		Version = "dev"
	}

	setLogger()

	cfg, err := config.LoadConfigFile(".")
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	tag := &cli.App{
		Version:              Version,
		Name:                 AppName,
		Usage:                "Template-driven code generation and project scaffolding",
		EnableBashCompletion: true,
		Commands: commands.RootCommands(cfg, Version, commands.SkillDocs{
			Guide:     skillDoc,
			Reference: referenceDoc,
			Recipes:   recipesDoc,
		}),
		Flags: commands.GlobalFlags(),
		// Custom error handler to avoid urfave/cli printing errors (we use slog).
		ExitErrHandler: handleExitError,
	}
	// CompletionCommand needs the *cli.App itself to generate its completion
	// script, so it cannot be part of RootCommands and is appended here instead.
	tag.Commands = append(tag.Commands, commands.CompletionCommand(tag))

	if err := tag.Run(os.Args); err != nil {
		exitCode := app.ExitGeneral

		// Extract exit code from CommandError
		var cmdErr *app.CommandError
		if errors.As(err, &cmdErr) {
			exitCode = cmdErr.ExitCode()
		}

		// Don't log user-initiated cancellation (Ctrl+C), and don't re-log an
		// error the JSON error seam already reported to stderr.
		if exitCode != app.ExitInterrupted && !commands.ErrorAlreadyReported(err) {
			slog.Error(err.Error()) //nolint:gosec // G706: slog structured logging; log injection not a concern in a CLI tool
		}
		os.Exit(exitCode)
	}
}

// handleExitError is the ExitErrHandler for urfave/cli. It prevents
// the framework from printing errors or calling os.Exit, letting main()
// handle error reporting and exit codes consistently.
func handleExitError(_ *cli.Context, _ error) {
	// Intentionally empty: errors are handled in main() after tag.Run() returns.
}

func setLogger() {
	logLevel := slog.LevelInfo
	addSource := false
	if val, ok := os.LookupEnv("ENV"); ok {
		if val == "DEV" {
			addSource = true
			logLevel = slog.LevelDebug
		}
	}
	slog.SetDefault(
		slog.New(
			prettylog.NewHandler(
				"tag",
				&slog.HandlerOptions{
					Level:     logLevel,
					AddSource: addSource,
				},
			),
		),
	)
}
