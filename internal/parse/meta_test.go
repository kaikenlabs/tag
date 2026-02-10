package parse

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ParseKeyValues_Lenient(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]string
	}{
		{
			name:  "simple key-value pair",
			input: []string{"key=value"},
			expected: map[string]string{
				"key": "value",
			},
		},
		{
			name:  "multiple key-value pairs",
			input: []string{"key1=value1", "key2=value2"},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name:  "quoted values with commas",
			input: []string{`field="name->string,age->int"`, `enabled=true`},
			expected: map[string]string{
				"field":   "name->string,age->int",
				"enabled": "true",
			},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: map[string]string{},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: map[string]string{},
		},
		{
			name:  "spaces around equals",
			input: strings.Split("key = value, space = needed", ","),
			expected: map[string]string{
				"key":   "value",
				"space": "needed",
			},
		},
		{
			name:  "quoted strings with spaces",
			input: strings.Split(`name="John Doe",age=30`, ","),
			expected: map[string]string{
				"name": "John Doe",
				"age":  "30",
			},
		},
		{
			name:  "complex nested structure",
			input: []string{`fields="name->string,age->int"`, `config="host=localhost,port=8080"`},
			expected: map[string]string{
				"fields": "name->string,age->int",
				"config": "host=localhost,port=8080",
			},
		},
		{
			name:     "malformed entries skipped",
			input:    []string{"keyvalue", "key=value"},
			expected: map[string]string{"key": "value"},
		},
		{
			name:     "empty value skipped",
			input:    []string{"key="},
			expected: map[string]string{},
		},
		{
			name:  "single quoted value",
			input: []string{`single='value'`},
			expected: map[string]string{
				"single": "value",
			},
		},
		{
			name:  "double quoted value",
			input: []string{`single="value"`},
			expected: map[string]string{
				"single": "value",
			},
		},
		{
			name:  "unmatched quotes",
			input: []string{`key="value`, `other=test`},
			expected: map[string]string{
				"key": "value", "other": "test",
			},
		},
		{
			name:  "empty quoted string",
			input: []string{`empty=""`},
			expected: map[string]string{
				"empty": "",
			},
		},
		{
			name:  "value with equals sign",
			input: []string{"key=value=with=equals"},
			expected: map[string]string{
				"key": "value=with=equals",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseKeyValues(tt.input, false)
			require.NoError(t, err, "lenient mode should never return error")
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_ParseKeyValues_Strict(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]string
		wantErr  bool
	}{
		{
			name:     "valid flags",
			input:    []string{"key1=value1", "key2=value2"},
			expected: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name:     "value with equals sign",
			input:    []string{"key=value=with=equals"},
			expected: map[string]string{"key": "value=with=equals"},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: map[string]string{},
		},
		{
			name:    "invalid flag format",
			input:   []string{"invalid"},
			wantErr: true,
		},
		{
			name:    "empty string entry",
			input:   []string{""},
			wantErr: true,
		},
		{
			name:     "quoted values are stripped in strict mode too",
			input:    []string{`key="value"`},
			expected: map[string]string{"key": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseKeyValues(tt.input, true)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid meta flag format")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestUT_StripQuotes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{`"hello`, "hello"},
		{`'hello`, "hello"},
		{`hello"`, `hello"`},
		{`hello`, "hello"},
		{`""`, ""},
		{`''`, ""},
		{`"`, `"`},
		{``, ``},
		{`  "hello"  `, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, stripQuotes(tt.input))
		})
	}
}
