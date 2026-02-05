package engine

import (
	"strings"
	"testing"

	"github.com/code-gorilla-au/odize"
)

var emptyMap = map[string]string{}

func Test_generateMeta(t *testing.T) {
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
			name:  "complex nested structure, different separators",
			input: []string{`fields="name:string;age:int"`, `config="host=localhost,port=8080"`},
			expected: map[string]string{
				"fields": "name:string;age:int",
				"config": "host=localhost,port=8080",
			},
		},
		{
			name:     "invalid format missing value",
			input:    []string{"key="},
			expected: emptyMap,
		},
		{
			name:     "invalid format missing equals",
			input:    []string{"keyvalue"},
			expected: emptyMap,
		},
		{
			name:  "single quoted value",
			input: []string{`single='value'`},
			expected: map[string]string{
				"single": "value",
			},
		},
		{
			name:  "single double quoted value",
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
			name:  "multiple single quoted sections",
			input: []string{`first='quoted value'`, `second='another value'`},
			expected: map[string]string{
				"first":  "quoted value",
				"second": "another value",
			},
		},
		{
			name:  "multiple double quoted sections",
			input: []string{`first="quoted value"`, `second="another value"`},
			expected: map[string]string{
				"first":  "quoted value",
				"second": "another value",
			},
		},
		{
			name:  "empty quoted string",
			input: []string{`empty=""`},
			expected: map[string]string{
				"empty": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateMeta(tt.input)
			odize.AssertEqual(t, tt.expected, result)
		})
	}
}
