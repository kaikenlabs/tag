package commands

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/config"
)

func newTestEditorSource(cfg *config.GlobalConfig, env map[string]string, tty bool) *editorSource {
	if cfg == nil {
		cfg = &config.GlobalConfig{}
	}
	var saved *config.GlobalConfig
	return &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) { return cfg, nil },
		saveConfig: func(c *config.GlobalConfig) error { saved = c; _ = saved; return nil },
		getenv: func(key string) string {
			if env == nil {
				return ""
			}
			return env[key]
		},
		isTTY:  func() bool { return tty },
		prompt: func() (string, error) { return "", nil },
	}
}

func TestUT_ResolveEditor_FlagOverridesAll(t *testing.T) {
	s := newTestEditorSource(
		&config.GlobalConfig{Editor: "saved-editor"},
		map[string]string{"VISUAL": "visual-editor", "EDITOR": "env-editor"},
		true,
	)
	editor, err := s.resolve("flag-editor")
	require.NoError(t, err)
	assert.Equal(t, "flag-editor", editor)
}

func TestUT_ResolveEditor_GlobalConfig(t *testing.T) {
	s := newTestEditorSource(
		&config.GlobalConfig{Editor: "code --wait"},
		map[string]string{"VISUAL": "visual", "EDITOR": "env"},
		true,
	)
	editor, err := s.resolve("")
	require.NoError(t, err)
	assert.Equal(t, "code --wait", editor)
}

func TestUT_ResolveEditor_VisualEnv(t *testing.T) {
	s := newTestEditorSource(
		&config.GlobalConfig{},
		map[string]string{"VISUAL": "visual-editor", "EDITOR": "env-editor"},
		true,
	)
	editor, err := s.resolve("")
	require.NoError(t, err)
	assert.Equal(t, "visual-editor", editor)
}

func TestUT_ResolveEditor_EditorEnv(t *testing.T) {
	s := newTestEditorSource(
		&config.GlobalConfig{},
		map[string]string{"EDITOR": "nano"},
		true,
	)
	editor, err := s.resolve("")
	require.NoError(t, err)
	assert.Equal(t, "nano", editor)
}

func TestUT_ResolveEditor_PromptSavesConfig(t *testing.T) {
	var savedCfg *config.GlobalConfig
	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) { return &config.GlobalConfig{}, nil },
		saveConfig: func(c *config.GlobalConfig) error { savedCfg = c; return nil },
		getenv:     func(string) string { return "" },
		isTTY:      func() bool { return true },
		prompt:     func() (string, error) { return "vim", nil },
	}

	editor, err := s.resolve("")
	require.NoError(t, err)
	assert.Equal(t, "vim", editor)
	require.NotNil(t, savedCfg)
	assert.Equal(t, "vim", savedCfg.Editor)
}

func TestUT_ResolveEditor_NoTTY_NoEditor_Error(t *testing.T) {
	s := newTestEditorSource(&config.GlobalConfig{}, nil, false)
	_, err := s.resolve("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no editor configured")
}

func TestUT_ResolveEditor_ConfigLoadError_FallsThrough(t *testing.T) {
	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) {
			return nil, errors.New("corrupt config")
		},
		saveConfig: func(*config.GlobalConfig) error { return nil },
		getenv: func(key string) string {
			if key == "VISUAL" {
				return "visual-fallback"
			}
			return ""
		},
		isTTY:  func() bool { return false },
		prompt: func() (string, error) { return "", nil },
	}

	editor, err := s.resolve("")
	require.NoError(t, err)
	assert.Equal(t, "visual-fallback", editor)
}

func TestUT_ResolveEditor_EmptyPrompt_Error(t *testing.T) {
	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) { return &config.GlobalConfig{}, nil },
		saveConfig: func(*config.GlobalConfig) error { return nil },
		getenv:     func(string) string { return "" },
		isTTY:      func() bool { return true },
		prompt:     func() (string, error) { return "", nil },
	}

	_, err := s.resolve("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no editor configured")
}

func TestUT_SplitEditorArgs(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"vim", []string{"vim"}},
		{"code --wait", []string{"code", "--wait"}},
		{"  nano  ", []string{"nano"}},
		{"subl -w --new-window", []string{"subl", "-w", "--new-window"}},
		{`code --goto "my file.txt"`, []string{"code", "--goto", "my file.txt"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := splitEditorArgs(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUT_SplitEditorArgs_InvalidQuoting(t *testing.T) {
	_, err := splitEditorArgs(`code "unterminated`)
	assert.Error(t, err)
}
