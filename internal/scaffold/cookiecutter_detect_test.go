package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_IsCookiecutterTemplate(t *testing.T) {
	tests := []struct {
		name               string
		setupFunc          func(dir string) error
		wantIsCookiecutter bool
		wantPathContains   string
	}{
		{
			name: "directory with cookiecutter.json",
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "cookiecutter.json"), []byte(`{"name": "test"}`), 0o644)
			},
			wantIsCookiecutter: true,
			wantPathContains:   "cookiecutter.json",
		},
		{
			name: "directory without cookiecutter.json",
			setupFunc: func(dir string) error {
				return nil // Empty directory
			},
			wantIsCookiecutter: false,
			wantPathContains:   "",
		},
		{
			name: "directory with tag.template.json only",
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(`{"name": "test"}`), 0o644)
			},
			wantIsCookiecutter: false,
			wantPathContains:   "",
		},
		{
			name: "directory with both files",
			setupFunc: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "cookiecutter.json"), []byte(`{"name": "test"}`), 0o644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(`{"name": "test"}`), 0o644)
			},
			wantIsCookiecutter: true, // Still detects cookiecutter.json
			wantPathContains:   "cookiecutter.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, tt.setupFunc(dir))

			gotPath, gotIsCookiecutter := IsCookiecutterTemplate(dir)

			assert.Equal(t, tt.wantIsCookiecutter, gotIsCookiecutter)
			if tt.wantIsCookiecutter {
				assert.Contains(t, gotPath, tt.wantPathContains)
			} else {
				assert.Empty(t, gotPath)
			}
		})
	}
}

func TestUT_CookiecutterDetectedError(t *testing.T) {
	t.Run("error message formatting", func(t *testing.T) {
		err := &CookiecutterDetectedError{CookiecutterPath: "/path/to/cookiecutter.json"}
		assert.Equal(t, "cookiecutter template detected: /path/to/cookiecutter.json", err.Error())
	})

	t.Run("errors.As matching", func(t *testing.T) {
		originalErr := &CookiecutterDetectedError{CookiecutterPath: "/test/path"}

		var ccErr *CookiecutterDetectedError
		assert.True(t, errors.As(originalErr, &ccErr))
		assert.Equal(t, "/test/path", ccErr.CookiecutterPath)
	})

	t.Run("errors.As not matching non-cookiecutter error", func(t *testing.T) {
		originalErr := errors.New("some other error")

		var ccErr *CookiecutterDetectedError
		assert.False(t, errors.As(originalErr, &ccErr))
	})
}
