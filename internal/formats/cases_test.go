package formats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCaseSnake(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		{
			name: "should return title_case",
			str:  "Title Case",
			want: "title_case",
		},
		{
			name: "should return sentence_case",
			str:  "sentence case",
			want: "sentence_case",
		},
		{
			name: "should return capital_case",
			str:  "CAPITAL_CASE",
			want: "capital_case",
		},
		{
			name: "should return snake_case",
			str:  "snakeCase",
			want: "snake_case",
		},
		{
			name: "pascal should return pascal_case",
			str:  "PascalCase",
			want: "pascal_case",
		},
		{
			name: "kebab should return kebab_case",
			str:  "kebab-case",
			want: "kebab_case",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CaseSnake(tt.str))
		})
	}
}

func TestCasePascal(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		{
			name: "should return SnakeCase",
			str:  "snake_case",
			want: "SnakeCase",
		},
		{
			name: "should return CamelCase",
			str:  "camelCase",
			want: "CamelCase",
		},
		{
			name: "should return KebabCase",
			str:  "kebab-case",
			want: "KebabCase",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CasePascal(tt.str))
		})
	}
}

func TestCaseCamel(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		{
			name: "should return titleCase",
			str:  "Title Case",
			want: "titleCase",
		},
		{
			name: "should return sentenceCase",
			str:  "sentence case",
			want: "sentenceCase",
		},
		{
			name: "should return snakeCase",
			str:  "snake_case",
			want: "snakeCase",
		},
		{
			name: "should return pascalCase",
			str:  "PascalCase",
			want: "pascalCase",
		},
		{
			name: "should return kebabCase",
			str:  "kebab-case",
			want: "kebabCase",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CaseCamel(tt.str))
		})
	}
}

func TestCaseKebab(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		{
			name: "should return title-case",
			str:  "Title Case",
			want: "title-case",
		},
		{
			name: "should return sentence-case",
			str:  "sentence case",
			want: "sentence-case",
		},
		{
			name: "should return snake-case",
			str:  "snake_case",
			want: "snake-case",
		},
		{
			name: "should return pascal-case",
			str:  "PascalCase",
			want: "pascal-case",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CaseKebab(tt.str))
		})
	}
}
