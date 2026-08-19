package commands

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/remote"
)

func TestUT_PrintCacheEntries_Header(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	printCacheEntries(&buf, nil)

	out := buf.String()
	assert.Contains(t, out, "KEY")
	assert.Contains(t, out, "VERSION")
	assert.Contains(t, out, "FETCHED")
	assert.Contains(t, out, "EXPIRES")
}

func TestUT_PrintCacheEntries_FullMetadata(t *testing.T) {
	t.Parallel()
	fetched := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	expires := time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)
	entries := []remote.CacheEntry{
		{
			Key: "gh:acme/template",
			Meta: &remote.CacheMeta{
				Version:   "v1.2.3",
				FetchedAt: fetched,
				ExpiresAt: &expires,
			},
		},
	}

	var buf bytes.Buffer
	printCacheEntries(&buf, entries)

	out := buf.String()
	assert.Contains(t, out, "gh:acme/template")
	assert.Contains(t, out, "v1.2.3")
	assert.Contains(t, out, fetched.Format(time.RFC3339))
	assert.Contains(t, out, expires.Format(time.RFC3339))
}

func TestUT_PrintCacheEntries_NilMeta(t *testing.T) {
	t.Parallel()
	entries := []remote.CacheEntry{
		{Key: "gh:orphan/entry", Meta: nil},
	}

	var buf bytes.Buffer
	printCacheEntries(&buf, entries)

	out := buf.String()
	assert.Contains(t, out, "gh:orphan/entry")
	assert.Contains(t, out, "-")       // version placeholder
	assert.Contains(t, out, "unknown") // fetched placeholder
	assert.Contains(t, out, "never")   // expires placeholder
}

func TestUT_PrintCacheEntries_Expired(t *testing.T) {
	t.Parallel()
	fetched := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	expired := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC) // in the past
	entries := []remote.CacheEntry{
		{
			Key: "gh:old/template",
			Meta: &remote.CacheMeta{
				FetchedAt: fetched,
				ExpiresAt: &expired,
			},
		},
	}

	var buf bytes.Buffer
	printCacheEntries(&buf, entries)

	assert.Contains(t, buf.String(), "expired")
}

func TestUT_PrintCacheEntries_TruncatesLongKey(t *testing.T) {
	t.Parallel()
	longKey := "gh:organization/very-long-template-name-that-exceeds-the-40-char-limit"
	entries := []remote.CacheEntry{
		{Key: longKey, Meta: nil},
	}

	var buf bytes.Buffer
	printCacheEntries(&buf, entries)

	out := buf.String()
	// Key should be truncated to 40 chars (37 + "...")
	assert.NotContains(t, out, longKey)
	assert.Contains(t, out, "...")
}

// TestUT_RedactCacheEntries_StripsQueryFromURLs proves redactCacheEntries
// removes a presigned-URL credential rather than merely reformatting it: if
// the query-string strip were removed, both assertions on the redacted value
// would fail (it would still equal the un-redacted original, which contains
// "SECRET").
func TestUT_RedactCacheEntries_StripsQueryFromURLs(t *testing.T) {
	t.Parallel()

	entries := []remote.CacheEntry{
		{
			Key: "gh-secret-template",
			Meta: &remote.CacheMeta{
				OriginalRef: "https://host/tmpl.zip?token=SECRET",
				ResolvedURL: "https://host/tmpl.zip?token=SECRET",
				Version:     "v1.0.0",
				FetchedAt:   goldenTime,
			},
		},
	}

	redacted := redactCacheEntries(entries)
	require.Len(t, redacted, 1)
	require.NotNil(t, redacted[0].Meta)

	assert.Equal(t, "https://host/tmpl.zip?[redacted]", redacted[0].Meta.OriginalRef)
	assert.Equal(t, "https://host/tmpl.zip?[redacted]", redacted[0].Meta.ResolvedURL)
	assert.NotContains(t, redacted[0].Meta.OriginalRef, "SECRET")
	assert.NotContains(t, redacted[0].Meta.ResolvedURL, "SECRET")

	// The original entries must be untouched — the on-disk metadata still
	// needs the working, credentialed URL.
	assert.Equal(t, "https://host/tmpl.zip?token=SECRET", entries[0].Meta.OriginalRef)
	assert.Equal(t, "https://host/tmpl.zip?token=SECRET", entries[0].Meta.ResolvedURL)
}

// TestUT_RedactCacheEntries_PlainRefPassesThrough proves a ref with no query
// string (the common case: "gh:acme/tmpl") is left byte-for-byte unchanged.
func TestUT_RedactCacheEntries_PlainRefPassesThrough(t *testing.T) {
	t.Parallel()

	entries := []remote.CacheEntry{
		{
			Key: "go-api",
			Meta: &remote.CacheMeta{
				OriginalRef: "gh:acme/tmpl",
				ResolvedURL: "gh:acme/tmpl",
				FetchedAt:   goldenTime,
			},
		},
	}

	redacted := redactCacheEntries(entries)
	require.Len(t, redacted, 1)
	require.NotNil(t, redacted[0].Meta)
	assert.Equal(t, "gh:acme/tmpl", redacted[0].Meta.OriginalRef)
	assert.Equal(t, "gh:acme/tmpl", redacted[0].Meta.ResolvedURL)
}

// TestUT_CacheLs_JSON_RedactsPresignedURLCredential is the end-to-end
// counterpart of TestUT_RedactCacheEntries_StripsQueryFromURLs: it drives the
// real `cache ls --format json` action so the assertion also proves
// cacheListCommand actually calls redactCacheEntries, not just that the
// helper works in isolation.
func TestUT_CacheLs_JSON_RedactsPresignedURLCredential(t *testing.T) {
	// Uses seedHome/seedCacheEntry, which call t.Setenv — do NOT use t.Parallel.
	home := seedHome(t)
	seedCacheEntry(t, home, "go-api", "v1.2.0", nil, func(m *remote.CacheMeta) {
		m.ResolvedURL = "https://host/tmpl.zip?token=SECRET"
	})

	run := runCLI(t, CacheCommand(), "cache", "ls", "--format", "json")
	require.NoError(t, run.Err)
	assert.Contains(t, run.Writer, "[redacted]")
	assert.NotContains(t, run.Writer, "SECRET")
}

// TestUT_CacheLs_Text_UnaffectedByRedaction proves the text listing path,
// which never prints the URL column, is unchanged by the redaction added for
// JSON: no credential leaks into it, but redactCacheEntries also has no
// visible effect on it (it never reaches the text printer).
func TestUT_CacheLs_Text_UnaffectedByRedaction(t *testing.T) {
	// Uses seedHome/seedCacheEntry, which call t.Setenv — do NOT use t.Parallel.
	home := seedHome(t)
	seedCacheEntry(t, home, "go-api", "v1.2.0", nil, func(m *remote.CacheMeta) {
		m.ResolvedURL = "https://host/tmpl.zip?token=SECRET"
	})

	run := runCLI(t, CacheCommand(), "cache", "ls")
	require.NoError(t, run.Err)
	assert.NotContains(t, run.All(), "SECRET")
	assert.NotContains(t, run.All(), "[redacted]")
}
