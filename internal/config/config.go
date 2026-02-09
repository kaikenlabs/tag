package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
	"github.com/urfave/cli/v2"
)

const File = ".tagconfig.json"

type Config struct {
	Env   Env   `json:"env"`
	Hooks Hooks `json:"hooks"`
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

// LoadConfigFile loads the configuration from the config file.
// Returns nil config if the file doesn't exist, or an error if parsing fails.
func LoadConfigFile() (*Config, error) {
	if _, err := os.Stat(File); err != nil {
		return nil, nil
	}
	data, err := os.ReadFile(File)
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
