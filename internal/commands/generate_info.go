package commands

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/tmplconfig"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/pkg/app"
)

const sourceLocal = "local"

// generatorInfoJSON is the top-level JSON output for `tag generate info`.
type generatorInfoJSON struct {
	Name          string                     `json:"name"`
	Type          string                     `json:"type"`
	Description   string                     `json:"description,omitempty"`
	Source        string                     `json:"source,omitempty"`
	Requires      []string                   `json:"requires,omitempty"`
	Variables     map[string]variableDefJSON `json:"variables,omitempty"`
	Hooks         *hooksJSON                 `json:"hooks,omitempty"`
	Files         []fileInfoJSON             `json:"files,omitempty"`
	SelfContained *bool                      `json:"self_contained,omitempty"`
	Generators    []string                   `json:"generators,omitempty"`
	Usage         string                     `json:"usage"`
}

type variableDefJSON struct {
	Type     string   `json:"type"`
	Prompt   string   `json:"prompt,omitempty"`
	Default  any      `json:"default,omitempty"`
	Required bool     `json:"required,omitempty"`
	Options  []string `json:"options,omitempty"`
	Secret   bool     `json:"secret,omitempty"`
}

type hooksJSON struct {
	PreScaffold  []string `json:"pre_scaffold,omitempty"`
	PostScaffold []string `json:"post_scaffold,omitempty"`
}

type fileInfoJSON struct {
	To     string `json:"to"`
	Action string `json:"action"`
	After  string `json:"after,omitempty"`
	Before string `json:"before,omitempty"`
}

func generateInfoCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:      "info",
		Usage:     "Show JSON metadata for a generator or bundle",
		ArgsUsage: "<name>",
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return app.UsageErrorf("please provide the generator or bundle name")
			}
			return generateInfo(cfg, c.Args().First(), os.Stdout)
		},
		BashComplete: func(c *cli.Context) {
			if c.NArg() > 0 {
				return
			}
			completeGeneratorNames(cfg)
		},
	}
}

func generateInfo(cfg *config.Config, name string, w io.Writer) error {
	if err := config.CheckConfig(cfg); err != nil {
		return err
	}

	target, err := resolveGenerateTarget(cfg, name, cfg.Env.BundlePath)
	if err != nil {
		return err
	}

	var info generatorInfoJSON
	if target.IsBundle {
		info, err = buildBundleInfo(name, target.BundlePath)
	} else {
		info, err = buildGeneratorInfo(cfg, name, target.GenDir)
	}
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return app.Errorf("cannot marshal info: %w", err)
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

func buildBundleInfo(name, bundlePath string) (generatorInfoJSON, error) {
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		return generatorInfoJSON{}, app.Errorf("cannot read bundle file: %w", err)
	}

	var bundle engine.Bundle
	if err = json.Unmarshal(data, &bundle); err != nil {
		return generatorInfoJSON{}, app.Errorf("cannot decode bundle file: %w", err)
	}

	var selfContained *bool
	if bundle.SelfContained {
		selfContained = &bundle.SelfContained
	}

	generators := make([]string, len(bundle.Generators))
	for i, g := range bundle.Generators {
		generators[i] = g.Name
	}

	return generatorInfoJSON{
		Name:          name,
		Type:          "bundle",
		Description:   bundle.Description,
		Requires:      bundle.Requires,
		SelfContained: selfContained,
		Generators:    generators,
		Usage:         "tag generate " + name + " <name> [--meta key=value ...]",
	}, nil
}

func buildGeneratorInfo(cfg *config.Config, name, genDir string) (generatorInfoJSON, error) {
	info := generatorInfoJSON{
		Name:  name,
		Type:  "generator",
		Usage: "tag generate " + name + " <name> [--meta key=value ...]",
	}

	// Read tag.template.json if present.
	configPath := filepath.Join(genDir, types.TemplateConfigFile)
	if data, readErr := os.ReadFile(configPath); readErr == nil {
		tc, parseErr := tmplconfig.ParseTemplateConfig(data)
		if parseErr != nil {
			return generatorInfoJSON{}, app.Errorf("cannot parse template config: %w", parseErr)
		}
		info.Description = tc.Description
		info.Requires = tc.Requires
		info.Variables = convertVariables(tc.Vars)
		if tc.Hooks != nil {
			info.Hooks = &hooksJSON{
				PreScaffold:  tc.Hooks.PreScaffold,
				PostScaffold: tc.Hooks.PostScaffold,
			}
		}
	}

	// Fallback description from frontmatter.
	if info.Description == "" {
		info.Description = readFrontmatterDesc(genDir)
	}

	// Extract file info from templates.
	files, err := extractFileInfos(genDir)
	if err != nil {
		return generatorInfoJSON{}, err
	}
	info.Files = files

	info.Source = determineSource(cfg, genDir)

	return info, nil
}

func convertVariables(vars map[string]tmplconfig.VariableDef) map[string]variableDefJSON {
	if len(vars) == 0 {
		return nil
	}
	result := make(map[string]variableDefJSON, len(vars))
	for name, v := range vars {
		result[name] = variableDefJSON{
			Type:     string(v.Type),
			Prompt:   v.Prompt,
			Default:  v.Default,
			Required: v.Required,
			Options:  v.Options,
			Secret:   v.Secret,
		}
	}
	return result
}

// extractFileInfos reads template files from genDir and parses their frontmatter
// to extract output path and action information. It parses raw key:value lines
// rather than using ParseMetadata, since template expressions are unrendered.
func extractFileInfos(genDir string) ([]fileInfoJSON, error) {
	templates, err := engine.LoadTemplateFiles(genDir)
	if err != nil {
		return nil, app.Errorf("cannot load template files: %w", err)
	}

	var files []fileInfoJSON
	for _, content := range templates {
		metaRaw, _, extractErr := template.ExtractMetadata(content)
		if extractErr != nil || metaRaw == "" {
			continue
		}

		fi := parseRawFrontmatter(metaRaw)
		if fi.To == "" {
			continue
		}
		files = append(files, fi)
	}
	return files, nil
}

// parseRawFrontmatter parses raw frontmatter key:value lines to extract
// file info without rendering template expressions.
func parseRawFrontmatter(metaRaw string) fileInfoJSON {
	fi := fileInfoJSON{Action: "create"}
	for line := range strings.SplitSeq(metaRaw, "\n") {
		rawKey, rawValue, found := strings.Cut(strings.TrimSpace(line), ":")
		if !found {
			continue
		}
		key := strings.TrimSpace(rawKey)
		value := strings.TrimSpace(rawValue)
		switch key {
		case "to":
			fi.To = value
		case "append":
			if strings.EqualFold(value, "true") {
				fi.Action = "append"
			}
		case "inject":
			if strings.EqualFold(value, "true") {
				fi.Action = "inject"
			}
		case "after":
			fi.After = value
		case "before":
			fi.Before = value
		}
	}
	return fi
}

func determineSource(cfg *config.Config, genDir string) string {
	if !cfg.HasTemplateOrigin() {
		return sourceLocal
	}
	lib, err := newLocalLibrary()
	if err != nil {
		return sourceLocal
	}
	templateDir, err := lib.TemplatePath(cfg.Template.Name)
	if err != nil {
		return sourceLocal
	}
	if strings.HasPrefix(genDir, templateDir) {
		return "template"
	}
	return sourceLocal
}
