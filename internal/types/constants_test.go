package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_HooksConfig_UnmarshalJSON(t *testing.T) {
	valid := []struct {
		name     string
		in       string
		wantPre  []string
		wantPost []string
		keptPre  []string
		keptPost []string
	}{
		{
			name:     "plain strings",
			in:       `{"pre_scaffold":["a"],"post_scaffold":["b","c"]}`,
			wantPre:  []string{"a"},
			wantPost: []string{"b", "c"},
			keptPre:  []string{"a"},
			keptPost: []string{"b", "c"},
		},
		{
			name:     "object without skip",
			in:       `{"post_scaffold":[{"run":"b"}]}`,
			wantPost: []string{"b"},
			keptPost: []string{"b"},
		},
		{
			name:     "object skip true",
			in:       `{"post_scaffold":[{"run":"b","skip_in_test":true}]}`,
			wantPost: []string{"b"},
			keptPost: []string{},
		},
		{
			name:     "object skip false",
			in:       `{"post_scaffold":[{"run":"b","skip_in_test":false}]}`,
			wantPost: []string{"b"},
			keptPost: []string{"b"},
		},
		{
			name:     "empty run",
			in:       `{"post_scaffold":[{"run":""}]}`,
			wantPost: []string{""},
			keptPost: []string{""},
		},
		{
			name:     "mixed in both phases",
			in:       `{"pre_scaffold":["a",{"run":"b","skip_in_test":true},{"run":"c"}],"post_scaffold":[{"run":"d","skip_in_test":true},"e"]}`,
			wantPre:  []string{"a", "b", "c"},
			wantPost: []string{"d", "e"},
			keptPre:  []string{"a", "c"},
			keptPost: []string{"e"},
		},
		{
			name:    "null phase",
			in:      `{"pre_scaffold":["a"],"post_scaffold":null}`,
			wantPre: []string{"a"},
			keptPre: []string{"a"},
		},
		{
			name: "unknown phase key ignored",
			in:   `{"other":["x"]}`,
		},
	}
	for _, tt := range valid {
		t.Run("accepts "+tt.name, func(t *testing.T) {
			var h HooksConfig
			require.NoError(t, json.Unmarshal([]byte(tt.in), &h))
			assert.Equal(t, tt.wantPre, h.PreScaffold)
			assert.Equal(t, tt.wantPost, h.PostScaffold)
			got := h.WithoutTestSkips()
			require.NotNil(t, got)
			assert.ElementsMatch(t, tt.keptPre, got.PreScaffold)
			assert.ElementsMatch(t, tt.keptPost, got.PostScaffold)
			assert.Equal(t, len(tt.keptPre), len(got.PreScaffold))
			assert.Equal(t, len(tt.keptPost), len(got.PostScaffold))
		})
	}

	invalid := []struct {
		name string
		in   string
		want []string
	}{
		{"object missing run", `{"post_scaffold":["a",{"skip_in_test":true}]}`, []string{"post_scaffold[1]", `"run"`}},
		{"run null", `{"pre_scaffold":[{"run":null}]}`, []string{"pre_scaffold[0]", `"run"`}},
		{"run number", `{"pre_scaffold":[{"run":5}]}`, []string{"pre_scaffold[0]"}},
		{"skip_in_test string", `{"post_scaffold":[{"run":"a","skip_in_test":"yes"}]}`, []string{"post_scaffold[0]"}},
		{"unknown key", `{"post_scaffold":[{"run":"a","extra":1}]}`, []string{"post_scaffold[0]", "extra"}},
		{"null entry", `{"post_scaffold":["a",null]}`, []string{"post_scaffold[1]"}},
		{"number entry", `{"pre_scaffold":[1]}`, []string{"pre_scaffold[0]"}},
		{"bool entry", `{"pre_scaffold":["a","b",true]}`, []string{"pre_scaffold[2]"}},
		{"array entry", `{"post_scaffold":[["a"]]}`, []string{"post_scaffold[0]"}},
	}
	for _, tt := range invalid {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			var h HooksConfig
			err := json.Unmarshal([]byte(tt.in), &h)
			require.Error(t, err)
			for _, w := range tt.want {
				assert.Contains(t, err.Error(), w)
			}
		})
	}
}

func TestUT_HooksConfig_WithoutTestSkips(t *testing.T) {
	parse := func(t *testing.T, in string) *HooksConfig {
		t.Helper()
		var h HooksConfig
		require.NoError(t, json.Unmarshal([]byte(in), &h))
		return &h
	}

	t.Run("order preserved across phases", func(t *testing.T) {
		h := parse(t, `{"pre_scaffold":["p1",{"run":"p2","skip_in_test":true},"p3"],"post_scaffold":["q1",{"run":"q2","skip_in_test":true},{"run":"q3"},"q4"]}`)
		got := h.WithoutTestSkips()
		assert.Equal(t, []string{"p1", "p3"}, got.PreScaffold)
		assert.Equal(t, []string{"q1", "q3", "q4"}, got.PostScaffold)
	})

	t.Run("original not mutated", func(t *testing.T) {
		h := parse(t, `{"post_scaffold":["a",{"run":"b","skip_in_test":true}]}`)
		_ = h.WithoutTestSkips()
		assert.Equal(t, []string{"a", "b"}, h.PostScaffold)
		again := h.WithoutTestSkips()
		assert.Equal(t, []string{"a"}, again.PostScaffold)
	})

	t.Run("duplicate command keeps the unflagged one", func(t *testing.T) {
		h := parse(t, `{"post_scaffold":["x",{"run":"x","skip_in_test":true}]}`)
		assert.Equal(t, []string{"x"}, h.WithoutTestSkips().PostScaffold)
	})

	t.Run("all skipped phase is empty", func(t *testing.T) {
		h := parse(t, `{"post_scaffold":[{"run":"a","skip_in_test":true}],"pre_scaffold":["k"]}`)
		got := h.WithoutTestSkips()
		assert.Empty(t, got.PostScaffold)
		assert.Equal(t, []string{"k"}, got.PreScaffold)
	})

	t.Run("skip false keeps everything", func(t *testing.T) {
		h := parse(t, `{"post_scaffold":[{"run":"a","skip_in_test":false},"b"]}`)
		assert.Equal(t, []string{"a", "b"}, h.WithoutTestSkips().PostScaffold)
	})

	t.Run("nil receiver", func(t *testing.T) {
		var h *HooksConfig
		assert.Nil(t, h.WithoutTestSkips())
	})

	t.Run("reusing a value leaves no stale flags", func(t *testing.T) {
		var h HooksConfig
		require.NoError(t, json.Unmarshal([]byte(`{"post_scaffold":[{"run":"a","skip_in_test":true}]}`), &h))
		require.NoError(t, json.Unmarshal([]byte(`{"post_scaffold":["a"]}`), &h))
		assert.Equal(t, []string{"a"}, h.WithoutTestSkips().PostScaffold)
	})

	t.Run("result carries no flags", func(t *testing.T) {
		h := parse(t, `{"post_scaffold":["a",{"run":"b","skip_in_test":true}]}`)
		got := h.WithoutTestSkips()
		assert.Equal(t, []string{"a"}, got.WithoutTestSkips().PostScaffold)
	})
}

// TestUT_HooksConfig_MarshalStringOnlyUnchanged is a no-change guard: string-only configs marshal as string arrays.
func TestUT_HooksConfig_MarshalStringOnlyUnchanged(t *testing.T) {
	in := `{"pre_scaffold":["a b"],"post_scaffold":["c","d"]}`
	var h HooksConfig
	require.NoError(t, json.Unmarshal([]byte(in), &h))
	out, err := json.Marshal(h)
	require.NoError(t, err)
	assert.JSONEq(t, in, string(out))
	assert.Equal(t, in, string(out))
}
