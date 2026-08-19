package commands

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/jsonout"
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

func cacheListFlags() []cli.Flag {
	return []cli.Flag{formatFlag(formatText, formatJSON)}
}

func cacheListCommand() *cli.Command {
	return &cli.Command{
		Name:    "ls",
		Aliases: []string{"list"},
		Usage:   "List cached templates",
		Flags:   cacheListFlags(),
		Action: func(c *cli.Context) error {
			format, err := resolveFormat(c, formatText, formatJSON)
			if err != nil {
				return err
			}

			cache, err := remote.NewFSCache("")
			if err != nil {
				return app.Errorf("failed to open cache: %w", err)
			}

			entries, err := cache.List()
			if err != nil {
				return app.Errorf("failed to list cache: %w", err)
			}

			out := cmdOut(c)
			if format == formatJSON {
				return jsonout.Write(out, map[string]any{"entries": redactCacheEntries(entries)})
			}

			if len(entries) == 0 {
				fmt.Fprintln(out, "No cached templates.")
				return nil
			}

			printCacheEntries(out, entries)
			return nil
		},
	}
}

// redactCacheEntries strips query strings from the URLs held in cache metadata
// before they are serialised.
//
// A template can be scaffolded from a presigned URL, whose credential lives in
// the query string, and remote.Resolve stores that URL verbatim in the entry's
// metadata. The text listing never showed either URL column, so adding JSON
// output would otherwise newly print the credential — straight into CI logs for
// anyone running `tag cache ls --format json`. The path is kept because it is
// what makes the entry identifiable; only the secret-bearing part is dropped.
//
// Entries are copied rather than mutated: the originals are the caller's, and
// the on-disk metadata must keep the working URL.
func redactCacheEntries(entries []remote.CacheEntry) []remote.CacheEntry {
	out := make([]remote.CacheEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Meta != nil {
			meta := *entry.Meta
			meta.OriginalRef = redactQuery(meta.OriginalRef)
			meta.ResolvedURL = redactQuery(meta.ResolvedURL)
			entry.Meta = &meta
		}
		out = append(out, entry)
	}
	return out
}

// redactQuery replaces a URL's query string with a marker, leaving everything
// else intact. Non-URL refs such as "gh:acme/tmpl" contain no "?" and pass
// through untouched.
func redactQuery(ref string) string {
	base, query, found := strings.Cut(ref, "?")
	if !found || query == "" {
		return ref
	}
	return base + "?" + redactedMarker
}

const redactedMarker = "[redacted]"

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
