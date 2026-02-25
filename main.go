package main

import (
	"errors"
	"log/slog"
	"os"

	"github.com/kaikenlabs/tag/pkg/prettylog"

	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types/flags"
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
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			commands.GenerateCommand(cfg),
			commands.ScaffoldCommand(),
			commands.TemplateCommand(cfg),
			commands.LibCommand(),
			commands.CacheCommand(),
			commands.ConvertCommand(),
			commands.VersionCommand(Version),
		},
		Flags: []cli.Flag{
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
		},
		// Custom error handler to avoid urfave/cli printing errors (we use slog).
		ExitErrHandler: handleExitError,
	}
	tag.Commands = append(tag.Commands, commands.CompletionCommand(tag))

	if err := tag.Run(os.Args); err != nil {
		exitCode := app.ExitGeneral

		// Check for prompt cancellation (Ctrl+C) → exit 130
		if errors.Is(err, scaffold.ErrPromptCancelled) {
			os.Exit(app.ExitInterrupted)
		}

		// Extract exit code from CommandError
		var cmdErr *app.CommandError
		if errors.As(err, &cmdErr) {
			exitCode = cmdErr.ExitCode()
		}

		slog.Error(err.Error())
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
