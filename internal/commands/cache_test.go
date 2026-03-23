package commands

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
