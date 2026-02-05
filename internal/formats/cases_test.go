package formats

import (
	"testing"
)

func TestCaseSnake(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "should return title_case",
			args: args{
				str: "Title Case",
			},
			want: "title_case",
		},
		{
			name: "should return sentence_case",
			args: args{
				str: "sentence case",
			},
			want: "sentence_case",
		},
		{
			name: "should return capital_case",
			args: args{
				str: "CAPITAL_CASE",
			},
			want: "capital_case",
		},
		{
			name: "should return snake_case",
			args: args{
				str: "snakeCase",
			},
			want: "snake_case",
		},
		{
			name: "pascal should return pascal_case",
			args: args{
				str: "PascalCase",
			},
			want: "pascal_case",
		},
		{
			name: "kebab should return kebab_case",
			args: args{
				str: "kebab-case",
			},
			want: "kebab_case",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CaseSnake(tt.args.str); got != tt.want {
				t.Errorf("CaseSnake() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCasePascal(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "should return SnakeCase",
			args: args{
				str: "snake_case",
			},
			want: "SnakeCase",
		},
		{
			name: "should return CamelCase",
			args: args{
				str: "camelCase",
			},
			want: "CamelCase",
		},
		{
			name: "should return KebabCase",
			args: args{
				str: "kebab-case",
			},
			want: "KebabCase",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CasePascal(tt.args.str); got != tt.want {
				t.Errorf("CasePascal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCaseCamel(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "should return titleCase",
			args: args{
				str: "Title Case",
			},
			want: "titleCase",
		},
		{
			name: "should return sentenceCase",
			args: args{
				str: "sentence case",
			},
			want: "sentenceCase",
		},
		{
			name: "should return snakeCase",
			args: args{
				str: "snake_case",
			},
			want: "snakeCase",
		},
		{
			name: "should return pascalCase",
			args: args{
				str: "PascalCase",
			},
			want: "pascalCase",
		},
		{
			name: "should return kebabCase",
			args: args{
				str: "kebab-case",
			},
			want: "kebabCase",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CaseCamel(tt.args.str); got != tt.want {
				t.Errorf("CaseCamel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCaseKebab(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "should return title-case",
			args: args{
				str: "Title Case",
			},
			want: "title-case",
		},
		{
			name: "should return sentence-case",
			args: args{
				str: "sentence case",
			},
			want: "sentence-case",
		},
		{
			name: "should return snake-case",
			args: args{
				str: "snake_case",
			},
			want: "snake-case",
		},
		{
			name: "should return pascal-case",
			args: args{
				str: "PascalCase",
			},
			want: "pascal-case",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CaseKebab(tt.args.str); got != tt.want {
				t.Errorf("CaseKebab() = %v, want %v", got, tt.want)
			}
		})
	}
}
