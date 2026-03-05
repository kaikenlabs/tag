package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/pkg/app"
)

var agentFileDefaults = map[string]string{
	"claude":   "CLAUDE.md",
	"cursor":   ".cursorrules",
	"windsurf": ".windsurfrules",
	"copilot":  ".github/copilot-instructions.md",
}

const (
	agentMarkerStart = "<!-- tag:generators:start -->"
	agentMarkerEnd   = "<!-- tag:generators:end -->"
)

func generateAgentFileCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:      "agent-file",
		Usage:     "Generate an AI agent reference file listing available generators",
		ArgsUsage: "<format>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output file path (overrides format default)",
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return app.UsageErrorf("please provide the format (%s)",
					strings.Join(agentFileFormatNames(), ", "))
			}
			return generateAgentFile(cfg, c.Args().First(), c.String("output"), c.App.Writer)
		},
		BashComplete: func(c *cli.Context) {
			if c.NArg() > 0 {
				return
			}
			for _, name := range agentFileFormatNames() {
				fmt.Println(name)
			}
		},
	}
}

func agentFileFormatNames() []string {
	return []string{"claude", "cursor", "windsurf", "copilot"}
}

func generateAgentFile(cfg *config.Config, format, outputPath string, w io.Writer) error {
	if err := config.CheckConfig(cfg); err != nil {
		return err
	}

	defaultPath, ok := agentFileDefaults[format]
	if !ok {
		return app.UsageErrorf("unknown format %q: must be one of %s",
			format, strings.Join(agentFileFormatNames(), ", "))
	}

	if outputPath == "" {
		outputPath = defaultPath
	}

	lists := collectGeneratorLists(cfg)
	content := buildAgentContent(lists)

	if err := writeAgentFile(outputPath, content); err != nil {
		return err
	}

	fmt.Fprintf(w, "Wrote agent file: %s\n", outputPath)
	return nil
}

func buildAgentContent(lists generatorLists) string {
	var b strings.Builder

	b.WriteString(agentMarkerStart)
	b.WriteString("\nThis project uses [tag](https://github.com/kaikenlabs/tag) for code generation.\n")
	b.WriteString("Use `tag skills` to learn how to use `tag` efficiently.\n\n")
	b.WriteString("Use `tag generate info <name>` to retrieve full metadata (variables, files, hooks) ")
	b.WriteString("for any generator or bundle as structured JSON — ")
	b.WriteString("useful for understanding inputs before running a generator.\n\n")
	b.WriteString("## Code Generators\n\n")

	b.WriteString("| Name | Type | Description |\n")
	b.WriteString("|------|------|-------------|\n")

	writeRows := func(items []generatorInfo, itemType string) {
		for _, g := range items {
			b.WriteString("| ")
			b.WriteString(g.Name)
			b.WriteString(" | ")
			b.WriteString(itemType)
			b.WriteString(" | ")
			b.WriteString(g.Description)
			b.WriteString(" |\n")
		}
	}

	writeRows(lists.templateGens, "generator")
	writeRows(lists.localGens, "generator")
	writeRows(lists.templateBundles, "bundle")
	writeRows(lists.localBundles, "bundle")

	b.WriteString("\n### Usage\n\n")
	b.WriteString("```\ntag generate <generator-or-bundle> <name> [--meta key=value ...]\n```\n\n")
	b.WriteString(agentMarkerEnd)
	b.WriteString("\n")

	return b.String()
}

func writeAgentFile(path, content string) error {
	// Create parent directories if needed.
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return app.Errorf("cannot create directory %q: %w", dir, err)
		}
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist — create it.
		return os.WriteFile(path, []byte(content), 0o600)
	}

	existingStr := string(existing)

	// If markers exist, replace the section.
	if strings.Contains(existingStr, agentMarkerStart) {
		replaced := replaceMarkerSection(existingStr, content)
		return os.WriteFile(path, []byte(replaced), 0o600)
	}

	// No markers — append with preceding newline.
	if !strings.HasSuffix(existingStr, "\n") {
		existingStr += "\n"
	}
	existingStr += "\n" + content
	return os.WriteFile(path, []byte(existingStr), 0o600)
}

func replaceMarkerSection(existing, newContent string) string {
	before, afterStart, foundStart := strings.Cut(existing, agentMarkerStart)
	if !foundStart {
		return existing + newContent
	}

	_, afterEnd, foundEnd := strings.Cut(afterStart, agentMarkerEnd)
	if !foundEnd {
		return existing + newContent
	}

	// Skip the trailing newline after the end marker if present.
	afterEnd = strings.TrimPrefix(afterEnd, "\n")

	return before + newContent + afterEnd
}
