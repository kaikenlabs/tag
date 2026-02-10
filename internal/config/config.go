package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

const File = ".tagconfig.json"

// TemplateOrigin records which template was used to scaffold a project.
type TemplateOrigin struct {
	Source  string `json:"source"`            // Original ref (e.g., "gh:acme/nextjs-starter")
	Name    string `json:"name"`              // Library name (e.g., "nextjs-starter")
	Version string `json:"version,omitempty"` // From tag.template.json at scaffold time
}

type Config struct {
	Template  *TemplateOrigin `json:"template,omitempty"`
	Variables map[string]any  `json:"variables,omitempty"`
	Env       Env             `json:"env"`
	Hooks     Hooks           `json:"hooks"`
}

// HasTemplateOrigin reports whether the config references a template from the library.
func (c *Config) HasTemplateOrigin() bool {
	return c != nil && c.Template != nil && c.Template.Name != ""
}

type Env struct {
	Path       string `json:"TAG_PATH"`
	SharedPath string `json:"TAG_SHARED_PATH"`
	BundlePath string `json:"TAG_BUNDLE_PATH"`
}
type Hooks struct {
	Pre  [][]string `json:"pre"`
	Post [][]string `json:"post"`
}

// CheckConfig validates that a config exists and returns an error if not.
func CheckConfig(cfg *Config) error {
	if cfg == nil {
		return app.Errorf("please run the 'init' command first or run this command from where the '%s' file is located", File)
	}
	return nil
}

// LoadConfigFile loads the configuration from the config file in the given directory.
// Returns nil config if the file doesn't exist, or an error if parsing fails.
func LoadConfigFile(dir string) (*Config, error) {
	path := filepath.Join(dir, File)
	if _, err := os.Stat(path); err != nil {
		return nil, nil //nolint:nilerr,nilnil // missing config file is not an error, returns nil config
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, app.Errorf("cannot load config file: %w", err)
	}
	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, app.Errorf("cannot parse config file: %w", err)
	}
	return &config, nil
}

func CreateConfigFile(c *cli.Context) error {
	cfg := Config{
		Env: Env{
			Path:       c.String(flags.PathFlag),
			SharedPath: c.String(flags.SharedPathFlag),
			BundlePath: c.String(flags.BundlePathFlag),
		},
		Hooks: Hooks{
			Pre:  [][]string{},
			Post: [][]string{},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(File, data, types.FileMode)
}
