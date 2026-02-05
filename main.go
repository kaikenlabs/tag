package main

import (
	"log/slog"
	"os"

	"github.com/kaikenlabs/tag/pkg/prettylog"

	"github.com/kaikenlabs/tag/internal/types/flags"

	"github.com/kaikenlabs/tag/internal/commands"
	"github.com/kaikenlabs/tag/internal/config"

	"github.com/urfave/cli/v2"
)

const AppName = "tag"

//nolint:gochecknoglobals
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

	cfg, err := config.LoadConfigFile()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	tag := &cli.App{
		Version: Version,
		Name:    AppName,
		Commands: []*cli.Command{
			commands.InitCommand(),
			commands.NewCommand(cfg),
			commands.BundleCommand(cfg),
			commands.GenerateCommand(cfg),
			commands.ScaffoldCommand(),
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    flags.DryRunFlag,
				Aliases: []string{"d"},
				Usage:   "dry run, only displays output",
			},
			&cli.StringFlag{
				Name:    flags.PathFlag,
				Value:   "_templates",
				Usage:   "Creates the templates directory path at the root of the project.",
				Aliases: []string{"tp"},
				EnvVars: []string{"TAG_PATH"},
			},
			&cli.StringFlag{
				Name:    flags.SharedPathFlag,
				Value:   "_shared",
				Usage:   "Shared template directory name",
				Aliases: []string{"sp"},
				EnvVars: []string{"TAG_SHARED_PATH"},
			}, &cli.StringFlag{
				Name:    flags.BundlePathFlag,
				Value:   "_bundles",
				Usage:   "Bundles directory name",
				Aliases: []string{"bp"},
				EnvVars: []string{"TAG_BUNDLE_PATH"},
			},
			&cli.StringFlag{
				Name:    flags.ExtensionFlag,
				Value:   ".tmpl",
				Usage:   "Template file extension.",
				Aliases: []string{"x"},
				EnvVars: []string{"TAG_EXTENSION"},
			},
		},
	}
	if err := tag.Run(os.Args); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
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
