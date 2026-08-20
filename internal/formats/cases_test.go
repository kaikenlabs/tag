package formats

import (
	"sync"
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

func TestUT_CaseSnake_SymbolEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		{
			name: "multiple spaces",
			str:  "hello world foo",
			want: "hello_world_foo",
		},
		{
			name: "mixed hyphens and spaces",
			str:  "hello-world foo",
			want: "hello_world_foo",
		},
		{
			name: "mixed underscores and hyphens",
			str:  "hello_world-foo",
			want: "hello_world_foo",
		},
		{
			name: "consecutive hyphens",
			str:  "hello--world",
			want: "hello__world",
		},
		{
			name: "leading space",
			str:  " hello",
			want: "_hello",
		},
		{
			name: "trailing space",
			str:  "hello ",
			want: "hello_",
		},
		{
			name: "single word no symbols",
			str:  "hello",
			want: "hello",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CaseSnake(tt.str))
		})
	}
}

func TestUT_CaseTitle(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		{
			name: "sentence",
			str:  "blood orange tree",
			want: "Blood Orange Tree",
		},
		{
			name: "all-caps token is flattened",
			str:  "HTTP server",
			want: "Http Server",
		},
		{
			name: "empty",
			str:  "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CaseTitle(tt.str))
		})
	}
}

func TestUT_CasePascal_SymbolEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		{
			name: "multiple underscores",
			str:  "hello_world_foo",
			want: "HelloWorldFoo",
		},
		{
			name: "mixed hyphens and spaces",
			str:  "hello-world foo",
			want: "HelloWorldFoo",
		},
		{
			name: "all-caps token is flattened by cases.Title",
			str:  "HTTP server",
			want: "HttpServer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CasePascal(tt.str))
		})
	}
}

func TestUT_CaseCamel_SymbolEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		{
			name: "multiple underscores",
			str:  "hello_world_foo",
			want: "helloWorldFoo",
		},
		{
			name: "with hyphens",
			str:  "hello-world-foo",
			want: "helloWorldFoo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CaseCamel(tt.str))
		})
	}
}

func TestUT_CasePast(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		// PascalCase
		{name: "PascalCase regular", str: "OrderCancel", want: "OrderCancelled"},
		{name: "PascalCase ends in e", str: "OrderCreate", want: "OrderCreated"},
		{name: "PascalCase default ed", str: "OrderProcess", want: "OrderProcessed"},
		{name: "PascalCase irregular", str: "OrderSend", want: "OrderSent"},
		{name: "PascalCase consonant+y", str: "FileCopy", want: "FileCopied"},
		{name: "PascalCase ic ending", str: "JobPanic", want: "JobPanicked"},
		{name: "PascalCase unchanged", str: "ValueSet", want: "ValueSet"},
		{name: "PascalCase with acronym", str: "HTTPServerStart", want: "HTTPServerStarted"},
		// camelCase
		{name: "camelCase regular", str: "orderCancel", want: "orderCancelled"},
		{name: "camelCase irregular", str: "orderRun", want: "orderRan"},
		{name: "camelCase ends in e", str: "orderCreate", want: "orderCreated"},
		// snake_case
		{name: "snake_case regular", str: "order_cancel", want: "order_cancelled"},
		{name: "snake_case irregular", str: "order_send", want: "order_sent"},
		{name: "snake_case ends in e", str: "order_create", want: "order_created"},
		// kebab-case
		{name: "kebab-case regular", str: "order-cancel", want: "order-cancelled"},
		{name: "kebab-case irregular", str: "order-build", want: "order-built"},
		// Single word
		{name: "single word regular", str: "cancel", want: "cancelled"},
		{name: "single word irregular", str: "run", want: "ran"},
		{name: "single word ends in e", str: "create", want: "created"},
		{name: "single word consonant+y", str: "copy", want: "copied"},
		{name: "single word vowel+y", str: "play", want: "played"},
		{name: "single word default", str: "process", want: "processed"},
		{name: "single word already past", str: "created", want: "created"},
		// Edge cases
		{name: "empty string", str: "", want: ""},
		// Double consonant whitelist
		{name: "stop", str: "stop", want: "stopped"},
		{name: "commit", str: "commit", want: "committed"},
		{name: "submit", str: "submit", want: "submitted"},
		{name: "drop", str: "drop", want: "dropped"},
		{name: "ship", str: "ship", want: "shipped"},
		{name: "plan", str: "plan", want: "planned"},
		{name: "log", str: "log", want: "logged"},
		{name: "wrap", str: "wrap", want: "wrapped"},
		{name: "occur", str: "occur", want: "occurred"},
		// Should NOT double
		{name: "open", str: "open", want: "opened"},
		{name: "render", str: "render", want: "rendered"},
		{name: "filter", str: "filter", want: "filtered"},
		{name: "target", str: "target", want: "targeted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CasePast(tt.str))
		})
	}
}

func TestUT_CaseKebab_SymbolEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		{
			name: "from spaces",
			str:  "hello world foo",
			want: "hello-world-foo",
		},
		{
			name: "from underscores",
			str:  "hello_world_foo",
			want: "hello-world-foo",
		},
		{
			name: "consecutive spaces",
			str:  "hello  world",
			want: "hello--world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CaseKebab(tt.str))
		})
	}
}

func TestUT_CasePascalCamel_ConcurrentUse(t *testing.T) {
	t.Parallel()

	const (
		goroutines = 64
		iterations = 500
	)

	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range iterations {
				if got := CasePascal("blood orange tree"); got != "BloodOrangeTree" {
					t.Errorf("CasePascal = %q", got)
					return
				}
				if got := CaseCamel("user account handler"); got != "userAccountHandler" {
					t.Errorf("CaseCamel = %q", got)
					return
				}
			}
		})
	}
	wg.Wait()
}
