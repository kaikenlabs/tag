package commands

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/search"
)

// --- printAddResult ---

func TestUT_PrintAddResult_New(t *testing.T) {
	t.Parallel()
	result := &library.AddResult{
		Name:        "mytemplate",
		Source:      "gh:acme/mytemplate",
		TemplateDir: "/home/user/.tag/templates/mytemplate",
	}

	var buf bytes.Buffer
	printAddResult(&buf, result)

	out := buf.String()
	assert.Contains(t, out, "Added")
	assert.Contains(t, out, "mytemplate")
	assert.Contains(t, out, "gh:acme/mytemplate")
	assert.Contains(t, out, "tag scaffold mytemplate")
}

func TestUT_PrintAddResult_Update(t *testing.T) {
	t.Parallel()
	result := &library.AddResult{
		Name:        "mytemplate",
		Source:      "gh:acme/mytemplate",
		TemplateDir: "/path",
		IsUpdate:    true,
	}

	var buf bytes.Buffer
	printAddResult(&buf, result)

	assert.Contains(t, buf.String(), "Updated")
}

func TestUT_PrintAddResult_Converted(t *testing.T) {
	t.Parallel()
	result := &library.AddResult{
		Name:          "cookiecutter-go",
		Source:        "gh:acme/cookiecutter-go",
		TemplateDir:   "/path",
		ConvertedFrom: "cookiecutter",
	}

	var buf bytes.Buffer
	printAddResult(&buf, result)

	assert.Contains(t, buf.String(), "Converted from: cookiecutter")
}

func TestUT_PrintAddResult_WithWarnings(t *testing.T) {
	t.Parallel()
	result := &library.AddResult{
		Name:        "mytemplate",
		Source:      "gh:acme/mytemplate",
		TemplateDir: "/path",
		Warnings:    []string{"unsupported extension: .j2", "missing description"},
	}

	var buf bytes.Buffer
	printAddResult(&buf, result)

	out := buf.String()
	assert.Contains(t, out, "Warnings:")
	assert.Contains(t, out, "unsupported extension: .j2")
	assert.Contains(t, out, "missing description")
}

// --- printSearchResults ---

func TestUT_PrintSearchResults_FormattedTable(t *testing.T) {
	t.Parallel()
	results := []search.SearchResult{
		{
			FullName:    "acme/tag-template",
			Description: "A Go microservice template",
			Stars:       42,
			UpdatedAt:   time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			FullName:    "corp/react-starter",
			Description: "React starter template",
			Stars:       100,
			UpdatedAt:   time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
		},
	}

	var buf bytes.Buffer
	printSearchResults(&buf, results)

	out := buf.String()
	assert.Contains(t, out, "Found 2 template(s)")
	assert.Contains(t, out, "REPOSITORY")
	assert.Contains(t, out, "STARS")
	assert.Contains(t, out, "acme/tag-template")
	assert.Contains(t, out, "42")
	assert.Contains(t, out, "2025-03-15")
	assert.Contains(t, out, "tag lib add gh:<owner>/<repo>")
}

func TestUT_PrintSearchResults_Empty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	printSearchResults(&buf, nil)

	out := buf.String()
	assert.Contains(t, out, "Found 0 template(s)")
}
