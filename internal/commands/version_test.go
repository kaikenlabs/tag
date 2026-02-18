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

func TestUT_FetchLatestVersion_RedirectSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/kaikenlabs/tag/releases/tag/v1.2.3", http.StatusFound)
	}))
	defer srv.Close()

	version, err := fetchLatestVersion(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", version)
}

func TestUT_FetchLatestVersion_MovedPermanently(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/kaikenlabs/tag/releases/tag/v0.9.0", http.StatusMovedPermanently)
	}))
	defer srv.Close()

	version, err := fetchLatestVersion(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "v0.9.0", version)
}

func TestUT_FetchLatestVersion_NonRedirectResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := fetchLatestVersion(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status 200")
}

func TestUT_FetchLatestVersion_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchLatestVersion(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status 404")
}

func TestUT_FetchLatestVersion_NetworkError(t *testing.T) {
	_, err := fetchLatestVersion(context.Background(), "http://127.0.0.1:1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network request failed")
}

func TestUT_FetchLatestVersion_MalformedRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	_, err := fetchLatestVersion(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not extract version")
}

func TestUT_FetchLatestVersion_MissingLocationHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	_, err := fetchLatestVersion(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing Location header")
}

func TestUT_VersionCheckAction_UpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/kaikenlabs/tag/releases/tag/v1.0.0", http.StatusFound)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	err := versionCheckAction(context.Background(), &buf, "v1.0.0", srv.URL)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "You are up to date!")
}

func TestUT_VersionCheckAction_UpdateAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/kaikenlabs/tag/releases/tag/v2.0.0", http.StatusFound)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	err := versionCheckAction(context.Background(), &buf, "v1.0.0", srv.URL)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Update available: v1.0.0 → v2.0.0")
	assert.Contains(t, buf.String(), "go install")
}

func TestUT_VersionCheckAction_DevBuild(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"empty", ""},
		{"dev", "dev"},
		{"dev-commit", "dev-abc1234"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := versionCheckAction(context.Background(), &buf, tt.version, "http://should-not-be-called")
			require.NoError(t, err)
			assert.Contains(t, buf.String(), "Development build, version check skipped.")
		})
	}
}

func TestUT_VersionAction_PrintsVersionWithoutCheck(t *testing.T) {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name: "version",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "check"},
				},
				Action: func(c *cli.Context) error {
					var buf bytes.Buffer
					err := versionAction(c, &buf, "v1.2.3", "http://should-not-be-called")
					require.NoError(t, err)
					assert.Contains(t, buf.String(), "tag version v1.2.3")
					assert.NotContains(t, buf.String(), "up to date")
					assert.NotContains(t, buf.String(), "Update available")
					return nil
				},
			},
		},
	}
	require.NoError(t, app.Run([]string{"app", "version"}))
}

func TestUT_IsDevBuild(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"", true},
		{"dev", true},
		{"dev-abc1234", true},
		{"v1.0.0", false},
		{"1.0.0", false},
		{"0.6.4", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			assert.Equal(t, tt.want, isDevBuild(tt.version))
		})
	}
}
