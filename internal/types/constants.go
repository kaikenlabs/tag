package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// Template and configuration file/directory constants used across the codebase.
const (
	// TemplateConfigFile is the name of the TAG template configuration file.
	TemplateConfigFile = "tag.template.json"

	// CookiecutterConfigFile is the name of the Cookiecutter configuration file.
	CookiecutterConfigFile = "cookiecutter.json"

	// GeneratorsDir is the directory within a template that contains generators.
	GeneratorsDir = "_generators"

	// DialectsDir is the directory within a template that contains dialect override files.
	DialectsDir = "_dialects"

	// TemplatesDir is the directory within a scaffolded project that holds generators.
	TemplatesDir = ".tag"

	// SharedDir is the default subdirectory for shared templates.
	SharedDir = "_shared"

	// BundlesDir is the default subdirectory for bundles.
	BundlesDir = "_bundles"

	// BundleExtension is the file extension for bundle definition files.
	BundleExtension = ".json"

	// CacheMetaFile is the remote cache metadata file written by the resolver.
	CacheMetaFile = "_meta.json"

	// TemplateReadme is the template documentation file displayed after scaffolding.
	TemplateReadme = "README.md"

	// TemplateHowto is the optional how-to documentation file included in templates.
	TemplateHowto = "HOWTO.md"

	// TagIgnoreFile is the name of the gitignore-style exclusion file for templates.
	TagIgnoreFile = ".tagignore"

	// HistoryFile is the filename for the generation history manifest.
	HistoryFile = "history.json"

	// HistoryDir is the TAG-internal subdirectory inside TemplatesDir that holds
	// generation history and backups. It is not a generator.
	HistoryDir = "history"

	// HistoryBackupsDir is the subdirectory inside TemplatesDir where backups are stored.
	HistoryBackupsDir = "history/backups"

	// TagConfigFile is the name of the project-level scaffold configuration file.
	TagConfigFile = ".tagconfig.json"

	// TagConfigSchemaVersion is the current schema version for .tagconfig.json.
	TagConfigSchemaVersion = 1
)

// TemplateType indicates the source type of a scaffolded template.
type TemplateType string

const (
	// TemplateTypeLocal indicates a template sourced from a local directory.
	TemplateTypeLocal TemplateType = "local"

	// TemplateTypeRemote indicates a template sourced from a remote git/zip origin.
	TemplateTypeRemote TemplateType = "remote"
)

// InjectClause represents where to inject content relative to a marker.
type InjectClause string

const (
	InjectBefore InjectClause = "Before"
	InjectAfter  InjectClause = "After"
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

func dropSkipped(cmds []string, skip []bool) []string {
	if skip == nil {
		return cmds
	}
	kept := make([]string, 0, len(cmds))
	for i, c := range cmds {
		if !skip[i] {
			kept = append(kept, c)
		}
	}
	return kept
}

type hookObject struct {
	Run        *string `json:"run"`
	SkipInTest bool    `json:"skip_in_test"`
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
		return "", false, errors.New("hook must be a command string or an object with \"run\"")
	}
}
