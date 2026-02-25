package commands

import (
	"fmt"
	"io"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/pkg/app"
)

// CacheCommand returns the cache command definition with subcommands.
func CacheCommand() *cli.Command {
	return &cli.Command{
		Name:  "cache",
		Usage: "Manage the template cache",
		Subcommands: []*cli.Command{
			cacheClearCommand(),
			cacheListCommand(),
		},
	}
}

func cacheClearCommand() *cli.Command {
	return &cli.Command{
		Name:  "clear",
		Usage: "Clear cached templates (expired only by default)",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "all",
				Usage: "Remove all cached templates, not just expired ones",
			},
		},
		Action: func(c *cli.Context) error {
			cache, err := remote.NewFSCache("")
			if err != nil {
				return app.Errorf("failed to open cache: %w", err)
			}

			var removed int
			if c.Bool("all") {
				removed, err = cache.ClearAll()
			} else {
				removed, err = cache.ClearExpired()
			}
			if err != nil {
				return app.Errorf("cache clear failed: %w", err)
			}

			fmt.Fprintf(c.App.Writer, "Removed %d cached template(s)\n", removed)
			return nil
		},
	}
}

func cacheListCommand() *cli.Command {
	return &cli.Command{
		Name:    "ls",
		Aliases: []string{"list"},
		Usage:   "List cached templates",
		Action: func(c *cli.Context) error {
			cache, err := remote.NewFSCache("")
			if err != nil {
				return app.Errorf("failed to open cache: %w", err)
			}

			entries, err := cache.List()
			if err != nil {
				return app.Errorf("failed to list cache: %w", err)
			}

			if len(entries) == 0 {
				fmt.Fprintln(c.App.Writer, "No cached templates.")
				return nil
			}

			printCacheEntries(c.App.Writer, entries)
			return nil
		},
	}
}

func printCacheEntries(w io.Writer, entries []remote.CacheEntry) {
	fmt.Fprintf(w, "%-40s %-12s %-20s %s\n", "KEY", "VERSION", "FETCHED", "EXPIRES")
	fmt.Fprintf(w, "%-40s %-12s %-20s %s\n", "---", "-------", "-------", "-------")

	for _, entry := range entries {
		version := "-"
		fetched := "unknown"
		expires := "never"

		if entry.Meta != nil {
			if entry.Meta.Version != "" {
				version = entry.Meta.Version
			}
			fetched = entry.Meta.FetchedAt.Format(time.RFC3339)
			if entry.Meta.ExpiresAt != nil {
				if time.Now().After(*entry.Meta.ExpiresAt) {
					expires = "expired"
				} else {
					expires = entry.Meta.ExpiresAt.Format(time.RFC3339)
				}
			}
		}

		fmt.Fprintf(w, "%-40s %-12s %-20s %s\n",
			truncate(entry.Key, 40),
			truncate(version, 12),
			truncate(fetched, 20),
			expires,
		)
	}
}
