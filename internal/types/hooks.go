package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// HooksConfig defines pre and post scaffold hooks.
type HooksConfig struct {
	PreScaffold  []string `json:"pre_scaffold,omitempty"`
	PostScaffold []string `json:"post_scaffold,omitempty"`

	preSkipInTest  []bool
	postSkipInTest []bool
}

// UnmarshalJSON decodes hooks given as command strings or as {"run", "skip_in_test"} objects.
func (h *HooksConfig) UnmarshalJSON(data []byte) error {
	var raw struct {
		PreScaffold  []json.RawMessage `json:"pre_scaffold"`
		PostScaffold []json.RawMessage `json:"post_scaffold"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	pre, preSkip, err := decodeHookPhase("pre_scaffold", raw.PreScaffold)
	if err != nil {
		return err
	}
	post, postSkip, err := decodeHookPhase("post_scaffold", raw.PostScaffold)
	if err != nil {
		return err
	}
	*h = HooksConfig{PreScaffold: pre, PostScaffold: post, preSkipInTest: preSkip, postSkipInTest: postSkip}
	return nil
}

// MarshalJSON writes each hook as a command string, or as an object when it is marked skip_in_test.
func (h HooksConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		PreScaffold  []any `json:"pre_scaffold,omitempty"`
		PostScaffold []any `json:"post_scaffold,omitempty"`
	}{
		PreScaffold:  encodeHookPhase(h.PreScaffold, h.preSkipInTest),
		PostScaffold: encodeHookPhase(h.PostScaffold, h.postSkipInTest),
	})
}

// WithoutTestSkips returns a copy of the config without the hooks marked skip_in_test.
func (h *HooksConfig) WithoutTestSkips() *HooksConfig {
	if h == nil {
		return nil
	}
	return &HooksConfig{
		PreScaffold:  dropSkipped(h.PreScaffold, h.preSkipInTest),
		PostScaffold: dropSkipped(h.PostScaffold, h.postSkipInTest),
	}
}

type hookObject struct {
	Run        *string `json:"run"`
	SkipInTest bool    `json:"skip_in_test"`
}

func isSkipped(skip []bool, i int) bool {
	return i < len(skip) && skip[i]
}

func dropSkipped(cmds []string, skip []bool) []string {
	if skip == nil {
		return cmds
	}
	kept := make([]string, 0, len(cmds))
	for i, c := range cmds {
		if !isSkipped(skip, i) {
			kept = append(kept, c)
		}
	}
	return kept
}

func encodeHookPhase(cmds []string, skip []bool) []any {
	if cmds == nil {
		return nil
	}
	out := make([]any, len(cmds))
	for i, c := range cmds {
		if isSkipped(skip, i) {
			out[i] = hookObject{Run: &c, SkipInTest: true}
		} else {
			out[i] = c
		}
	}
	return out
}

func decodeHookPhase(phase string, entries []json.RawMessage) ([]string, []bool, error) {
	if entries == nil {
		return nil, nil, nil
	}
	cmds := make([]string, len(entries))
	skip := make([]bool, len(entries))
	anySkip := false
	for i, entry := range entries {
		cmd, skipped, err := decodeHook(entry)
		if err != nil {
			return nil, nil, fmt.Errorf("%s[%d]: %w", phase, i, err)
		}
		cmds[i], skip[i] = cmd, skipped
		anySkip = anySkip || skipped
	}
	if !anySkip {
		skip = nil
	}
	return cmds, skip, nil
}

func decodeHook(entry json.RawMessage) (string, bool, error) {
	trimmed := bytes.TrimSpace(entry)
	switch {
	case bytes.Equal(trimmed, []byte("null")):
		return "", false, nil
	case len(trimmed) > 0 && trimmed[0] == '"':
		var cmd string
		err := json.Unmarshal(trimmed, &cmd)
		return cmd, false, err
	case len(trimmed) > 0 && trimmed[0] == '{':
		var obj hookObject
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&obj); err != nil {
			return "", false, err
		}
		if obj.Run == nil {
			return "", false, errors.New(`hook object requires "run"`)
		}
		return *obj.Run, obj.SkipInTest, nil
	default:
		return "", false, errors.New(`hook must be a command string or an object with "run"`)
	}
}
