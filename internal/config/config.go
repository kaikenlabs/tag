package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"gitlab.com/Vitrifi/tag/pkg/app"

	"gitlab.com/Vitrifi/tag/internal/types/flags"
)

const File = ".tagconfig.json"

type Config struct {
	Env   Env   `json:"env"`
	Hooks Hooks `json:"hooks"`
}
type Env struct {
	Path       string `json:"TAG_PATH"`
	Extension  string `json:"TAG_EXTENSION"`
	SharedPath string `json:"TAG_SHARED_PATH"`
	BundlePath string `json:"TAG_BUNDLE_PATH"`
}
type Hooks struct {
	Pre  [][]string `json:"pre"`
	Post [][]string `json:"post"`
}

func CheckConfig(cfg *Config) {
	var emptyConfig *Config
	if cfg == emptyConfig {
		app.Terminate("please run the 'init' command first or run this command from where the '%s' file is located", File)
	}
}

func LoadConfigFile() *Config {
	if _, err := os.Stat(File); err != nil {
		return &Config{}
	}
	data, err := os.ReadFile(File)
	if err != nil {
		app.Terminate("cannot load config file: %s", err.Error())
	}
	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		app.Terminate("cannot parse config file: %s", err.Error())
	}
	return &config
}

func CreateConfigFile(c *cli.Context) error {
	file, err := os.Create(File)
	if err != nil {
		return err
	}
	defer file.Close() // Remember to close the file

	content := `
{
  "env":{
    "TAG_PATH": "%s",
    "TAG_EXTENSION": "%s",
    "TAG_SHARED_PATH": "%s",
    "TAG_BUNDLE_PATH": "%s"
  },
  "hooks": {
    "pre": [],
    "post":[]
  }
}
`
	_, err = file.WriteString(fmt.Sprintf(content, c.String(flags.PathFlag), c.String(flags.ExtensionFlag), c.String(flags.SharedPathFlag), c.String(flags.BundlePathFlag)))
	if err != nil {
		return err
	}
	return nil
}
