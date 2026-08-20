package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/fileaction"
)

func TestUT_OnExistingPolicy_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		policy OnExistingPolicy
		want   bool
	}{
		{name: "empty default", policy: "", want: true},
		{name: "fail", policy: "fail", want: true},
		{name: "skip", policy: "skip", want: true},
		{name: "overwrite", policy: "overwrite", want: true},
		{name: "invalid", policy: "invalid", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.policy.IsValid())
		})
	}
}

func TestUT_OnExistingPolicy_isFail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		policy OnExistingPolicy
		want   bool
	}{
		{name: "empty default is fail", policy: "", want: true},
		{name: "fail is fail", policy: "fail", want: true},
		{name: "skip is not fail", policy: "skip", want: false},
		{name: "overwrite is not fail", policy: "overwrite", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.policy.isFail())
		})
	}
}

func TestUT_GenerateResult_Add(t *testing.T) {
	t.Parallel()

	r := &GenerateResult{
		Created:     1,
		Skipped:     2,
		Overwritten: 0,
		Modified:    1,
		Details: []FileOpDetail{
			{Path: "a.go", Action: fileaction.ActionAppend},
		},
	}

	other := GenerateResult{
		Created:     3,
		Skipped:     1,
		Overwritten: 2,
		Modified:    0,
		Details: []FileOpDetail{
			{Path: "b.go", Action: fileaction.ActionCreate},
			{Path: "c.go", Action: fileaction.ActionInject},
		},
	}

	r.Add(other)

	assert.Equal(t, 4, r.Created)
	assert.Equal(t, 3, r.Skipped)
	assert.Equal(t, 2, r.Overwritten)
	assert.Equal(t, 1, r.Modified)
	require.Len(t, r.Details, 3)
	assert.Equal(t, "a.go", r.Details[0].Path)
	assert.Equal(t, "b.go", r.Details[1].Path)
	assert.Equal(t, "c.go", r.Details[2].Path)
	// The aggregated Details must preserve the distinct append/inject
	// actions from each operand, not just their paths.
	assert.Equal(t, fileaction.ActionAppend, r.Details[0].Action)
	assert.Equal(t, fileaction.ActionCreate, r.Details[1].Action)
	assert.Equal(t, fileaction.ActionInject, r.Details[2].Action)
}

func TestUT_FileOpDetail_DisplayOp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		action fileaction.Action
		want   string
	}{
		{name: "create", action: fileaction.ActionCreate, want: "created"},
		{name: "skip", action: fileaction.ActionSkip, want: "skipped"},
		{name: "overwrite", action: fileaction.ActionOverwrite, want: "overwritten"},
		{name: "append", action: fileaction.ActionAppend, want: "modified"},
		{name: "inject", action: fileaction.ActionInject, want: "modified"},
		{name: "openapi-merge", action: fileaction.ActionOpenAPIMerge, want: "merged"},
		{name: "zero value", action: fileaction.Action(""), want: ""},
		{name: "unknown value", action: fileaction.Action("future-op"), want: "future-op"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := FileOpDetail{Path: "x.go", Action: tt.action}
			assert.Equal(t, tt.want, d.DisplayOp())
		})
	}
}

func TestUT_GenerateResult_Add_Empty(t *testing.T) {
	t.Parallel()

	r := &GenerateResult{}
	r.Add(GenerateResult{})

	assert.Equal(t, 0, r.Created)
	assert.Equal(t, 0, r.Skipped)
	assert.Equal(t, 0, r.Overwritten)
	assert.Equal(t, 0, r.Modified)
	assert.Empty(t, r.Details)
}

func TestUT_ConflictError_Error(t *testing.T) {
	t.Parallel()

	ce := &ConflictError{
		Files: []string{"main.go", "config.yaml", "readme.md"},
	}

	got := ce.Error()
	assert.Contains(t, got, "main.go")
	assert.Contains(t, got, "config.yaml")
	assert.Contains(t, got, "readme.md")
	assert.Contains(t, got, "--on-existing=overwrite")
	assert.Contains(t, got, "--on-existing=skip")
	assert.Contains(t, got, "  - main.go")
	assert.Contains(t, got, "  - config.yaml")
	assert.Contains(t, got, "  - readme.md")
}

func TestUT_ConflictError_Error_SingleFile(t *testing.T) {
	t.Parallel()

	ce := &ConflictError{
		Files: []string{"only.go"},
	}

	got := ce.Error()
	assert.Contains(t, got, "only.go")
	assert.Contains(t, got, "  - only.go")
}
