package testrunner_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/testrunner"
	"github.com/kaikenlabs/tag/internal/tmplconfig"
)

func TestUT_ExtractBooleanVars(t *testing.T) {
	t.Parallel()

	cfg := &tmplconfig.TemplateConfig{
		Vars: map[string]tmplconfig.VariableDef{
			"module_path":    {Type: tmplconfig.VarTypeString},
			"use_postgres":   {Type: tmplconfig.VarTypeBoolean},
			"use_amqp":       {Type: tmplconfig.VarTypeBoolean},
			"use_clickhouse": {Type: tmplconfig.VarTypeBoolean},
			"db_type":        {Type: tmplconfig.VarTypeChoice},
		},
	}

	tests := []struct {
		name     string
		skipVars []string
		want     []string
	}{
		{
			name:     "extracts all booleans sorted",
			skipVars: nil,
			want:     []string{"use_amqp", "use_clickhouse", "use_postgres"},
		},
		{
			name:     "skips specified vars",
			skipVars: []string{"use_clickhouse"},
			want:     []string{"use_amqp", "use_postgres"},
		},
		{
			name:     "skips all booleans",
			skipVars: []string{"use_amqp", "use_clickhouse", "use_postgres"},
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := testrunner.ExtractBooleanVars(cfg, tt.skipVars)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUT_CombinationCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		boolVars []string
		pinVars  map[string]string
		want     int
	}{
		{name: "2 vars", boolVars: []string{"a", "b"}, want: 4},
		{name: "3 vars", boolVars: []string{"a", "b", "c"}, want: 8},
		{name: "0 vars", boolVars: nil, want: 1},
		{name: "1 pinned of 3", boolVars: []string{"a", "b", "c"}, pinVars: map[string]string{"b": "true"}, want: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := testrunner.CombinationCount(tt.boolVars, tt.pinVars)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUT_GenerateCombinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		boolVars []string
		pinVars  map[string]string
		wantLen  int
	}{
		{
			name:     "2 vars = 4 combinations",
			boolVars: []string{"a", "b"},
			pinVars:  nil,
			wantLen:  4,
		},
		{
			name:     "3 vars = 8 combinations",
			boolVars: []string{"a", "b", "c"},
			pinVars:  nil,
			wantLen:  8,
		},
		{
			name:     "0 vars = 1 combination",
			boolVars: nil,
			pinVars:  nil,
			wantLen:  1,
		},
		{
			name:     "pinned var reduces combinations",
			boolVars: []string{"a", "b", "c"},
			pinVars:  map[string]string{"b": "true"},
			wantLen:  4, // 2^2 since b is pinned
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			combos := testrunner.GenerateCombinations(tt.boolVars, tt.pinVars)
			assert.Len(t, combos, tt.wantLen)
		})
	}
}

func TestUT_GenerateCombinations_PinnedValue(t *testing.T) {
	t.Parallel()

	combos := testrunner.GenerateCombinations(
		[]string{"a", "b"},
		map[string]string{"b": "true"},
	)

	// All combos should have b=true.
	for _, c := range combos {
		assert.Equal(t, "true", c.Vars["b"], "pinned var should be fixed")
	}

	// a should have both true and false.
	aValues := map[string]bool{}
	for _, c := range combos {
		aValues[c.Vars["a"]] = true
	}
	assert.True(t, aValues["true"])
	assert.True(t, aValues["false"])
}

func TestUT_FilterCombinations(t *testing.T) {
	t.Parallel()

	combos := testrunner.GenerateCombinations([]string{"a", "b"}, nil)
	require.Len(t, combos, 4)

	tests := []struct {
		name    string
		filter  string
		wantLen int
		wantErr bool
	}{
		{name: "empty filter returns all", filter: "", wantLen: 4},
		{name: "numeric filter returns one", filter: "0", wantLen: 1},
		{name: "kv filter narrows", filter: "a=true", wantLen: 2},
		{name: "multi kv filter", filter: "a=true,b=true", wantLen: 1},
		{name: "no match returns nil", filter: "999", wantLen: 0},
		{name: "malformed filter errors", filter: "noequals", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			filtered, err := testrunner.FilterCombinations(combos, tt.filter)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, filtered, tt.wantLen)
		})
	}
}

func TestUT_ComboLabel(t *testing.T) {
	t.Parallel()

	combo := testrunner.Combination{
		Index: 0,
		Vars:  map[string]string{"a": "true", "b": "false"},
	}
	label := testrunner.ComboLabel(combo, []string{"a", "b"})
	assert.Equal(t, "a=true b=false", label)
}
