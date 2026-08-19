package commands

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestUT_VersionCommand_Structure(t *testing.T) {
	t.Parallel()
	cmd := VersionCommand("v1.0.0")

	assert.Equal(t, "version", cmd.Name)
	assert.NotEmpty(t, cmd.Usage)
	assert.NotEmpty(t, cmd.Description)
	assert.NotNil(t, cmd.Action)

	// Check the --check and --format flags exist
	require.Len(t, cmd.Flags, 2)
	assert.Equal(t, "check", cmd.Flags[0].Names()[0])
	assert.Equal(t, "format", cmd.Flags[1].Names()[0])
}

func TestUT_VersionCheckAction_NetworkError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := versionCheckAction(context.Background(), &buf, "v1.0.0", "http://127.0.0.1:1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check for updates")
}

func TestUT_VersionCheckAction_WithoutVPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/kaikenlabs/tag/releases/tag/v1.0.0", http.StatusFound)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	err := versionCheckAction(context.Background(), &buf, "1.0.0", srv.URL)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "You are up to date!")
}

func TestUT_VersionAction_FullFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/kaikenlabs/tag/releases/tag/v3.0.0", http.StatusFound)
	}))
	defer srv.Close()

	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name: "version",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "check"},
				},
				Action: func(c *cli.Context) error {
					var buf bytes.Buffer
					err := versionAction(c, &buf, "v1.0.0", srv.URL)
					require.NoError(t, err)
					output := buf.String()
					assert.Contains(t, output, "tag version v1.0.0")
					assert.Contains(t, output, "Update available: v1.0.0 → v3.0.0")
					return nil
				},
			},
		},
	}
	require.NoError(t, app.Run([]string{"app", "version", "--check"}))
}

func TestUT_FetchLatestVersion_LocationDot(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", ".")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	_, err := fetchLatestVersion(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not extract version")
}
