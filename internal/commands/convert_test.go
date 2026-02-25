package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kaikenlabs/tag/internal/convert"
)

func TestUT_Truncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string unchanged",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length unchanged",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long string truncated with ellipsis",
			input:    "hello world this is a long string",
			maxLen:   15,
			expected: "hello world ...",
		},
		{
			name:     "newlines replaced with spaces",
			input:    "hello\nworld",
			maxLen:   20,
			expected: "hello world",
		},
		{
			name:     "whitespace trimmed",
			input:    "  hello  ",
			maxLen:   20,
			expected: "hello",
		},
		{
			name:     "truncation after newline replacement",
			input:    "line1\nline2\nline3",
			maxLen:   10,
			expected: "line1 l...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_PrintConversionResult(t *testing.T) {
	tests := []struct {
		name     string
		result   *convert.Result
		contains []string
	}{
		{
			name: "basic result",
			result: &convert.Result{
				Destination:        "/tmp/output",
				VariablesConverted: 5,
				DirsRenamed:        2,
				FilesRenamed:       3,
				FilesProcessed:     10,
			},
			contains: []string{
				"Converted template: /tmp/output",
				"Variables: 5 converted",
				"Directories renamed: 2",
				"Files renamed: 3",
				"Files processed: 10",
			},
		},
		{
			name: "dry run result",
			result: &convert.Result{
				Destination:        "/tmp/output",
				DryRun:             true,
				VariablesConverted: 3,
				FilesProcessed:     5,
			},
			contains: []string{
				"Dry Run",
				"Run without --dry-run",
			},
		},
		{
			name: "result with hooks",
			result: &convert.Result{
				Destination:        "/tmp/output",
				VariablesConverted: 2,
				FilesProcessed:     4,
				HooksCopied:        3,
			},
			contains: []string{
				"Hooks: 3 files copied",
			},
		},
		{
			name: "result with warnings",
			result: &convert.Result{
				Destination:        "/tmp/output",
				VariablesConverted: 1,
				FilesProcessed:     2,
				Warnings:           []string{"some variable uses Jinja2-only feature"},
			},
			contains: []string{
				"Warnings:",
				"some variable uses Jinja2-only feature",
			},
		},
		{
			name: "result with incompatibilities",
			result: &convert.Result{
				Destination:        "/tmp/output",
				VariablesConverted: 1,
				FilesProcessed:     2,
				Incompatibilities: []convert.Incompatibility{
					{
						Path:       "template.py",
						Line:       42,
						Kind:       "jinja2_filter",
						Original:   "{{ value|dictsort }}",
						Suggestion: "use custom filter",
					},
				},
			},
			contains: []string{
				"Content incompatibilities found: 1",
				"template.py:42",
				"jinja2_filter",
				"{{ value|dictsort }}",
				"use custom filter",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printConversionResult(&buf, tt.result)
			output := buf.String()

			for _, s := range tt.contains {
				assert.Contains(t, output, s)
			}
		})
	}
}

func TestUT_ConvertCookiecutterAction_MissingArguments(t *testing.T) {
	ctx := createTestCLIContext(t, []string{}, nil)

	err := convertCookiecutterAction(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source template is required")
}
