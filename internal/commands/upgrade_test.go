package commands

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestUT_UpgradeAction_AlreadyUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/kaikenlabs/tag/releases/tag/v1.0.0", http.StatusFound)
	}))
	defer srv.Close()

	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name: "upgrade",
				Action: func(c *cli.Context) error {
					var buf bytes.Buffer
					err := upgradeAction(c, &buf, "v1.0.0", srv.URL)
					require.NoError(t, err)
					assert.Contains(t, buf.String(), "Already up to date!")
					return nil
				},
			},
		},
	}
	require.NoError(t, app.Run([]string{"app", "upgrade"}))
}

func TestUT_UpgradeAction_DevBuildProceeds(t *testing.T) {
	// For dev builds, upgradeAction should NOT say "already up to date" — it should
	// proceed to the upgrade. We test that it at least reaches the "Development build"
	// message and attempts to upgrade (which will fail since we don't serve the archive).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/releases/latest" {
			http.Redirect(w, r, "https://github.com/kaikenlabs/tag/releases/tag/v2.0.0", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name: "upgrade",
				Action: func(c *cli.Context) error {
					var buf bytes.Buffer
					err := upgradeAction(c, &buf, "dev", srv.URL)
					// Will error because download fails, but should show dev build message.
					assert.Contains(t, buf.String(), "Development build detected")
					assert.Error(t, err)
					return nil
				},
			},
		},
	}
	require.NoError(t, app.Run([]string{"app", "upgrade"}))
}

func TestUT_UpgradeAction_UpgradeAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/releases/latest" {
			http.Redirect(w, r, "https://github.com/kaikenlabs/tag/releases/tag/v2.0.0", http.StatusFound)
			return
		}
		// Return 404 for archive/checksums — we only test the messaging here.
		http.NotFound(w, r)
	}))
	defer srv.Close()

	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name: "upgrade",
				Action: func(c *cli.Context) error {
					var buf bytes.Buffer
					err := upgradeAction(c, &buf, "v1.0.0", srv.URL)
					// Will error because download fails, but should show upgrade message.
					assert.Contains(t, buf.String(), "Upgrading tag v1.0.0 → v2.0.0")
					assert.Error(t, err)
					return nil
				},
			},
		},
	}
	require.NoError(t, app.Run([]string{"app", "upgrade"}))
}

func TestUT_UpgradeAction_FetchVersionError(t *testing.T) {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name: "upgrade",
				Action: func(c *cli.Context) error {
					var buf bytes.Buffer
					err := upgradeAction(c, &buf, "v1.0.0", "http://127.0.0.1:1")
					require.Error(t, err)
					assert.Contains(t, err.Error(), "failed to check for updates")
					return nil
				},
			},
		},
	}
	require.NoError(t, app.Run([]string{"app", "upgrade"}))
}
