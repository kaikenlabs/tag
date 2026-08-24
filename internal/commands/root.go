package commands

import (
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
)

// RootCommands returns the top-level command tree for the tag CLI. main.go
// builds its App.Commands from this so the binary and the test harness walk
// the exact same tree — see GlobalFlags for the same reasoning applied to the
// App's global flags.
func RootCommands(cfg *config.Config, version string, docs SkillDocs) []*cli.Command {
	return []*cli.Command{
		GenerateCommand(cfg),
		ScaffoldCommand(version),
		TemplateCommand(cfg, version),
		LibCommand(),
		DialectCommand(),
		CacheCommand(),
		ConvertCommand(),
		ExtractCommand(),
		UndoCommand(),
		CheckCommand(),
		DiffCommand(),
		UpdateTemplateCommand(),
		VersionCommand(version),
		UpgradeCommand(version),
		TestCommand(),
		DoctorCommand(version),
		SkillCommand(version, docs),
	}
}
