package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
)

// createTemplateConfig writes a tag.template.json file in the given directory.
func createTemplateConfig(t *testing.T, dir string, config map[string]any) {
	t.Helper()
	data, err := json.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, types.TemplateConfigFile), data, 0o644))
}

func TestUT_DisplayTemplateInfo_FullTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	createTemplateConfig(t, dir, map[string]any{
		"name":        "my-template",
		"version":     "1.2.0",
		"description": "A test template",
		"vars": map[string]any{
			"project_name": "default-name",
			"use_docker": map[string]any{
				"type":    "boolean",
				"default": true,
			},
			"license": map[string]any{
				"type":    "choice",
				"options": []string{"MIT", "Apache-2.0", "GPL-3.0"},
			},
		},
		"hooks": map[string]any{
			"pre_scaffold":  []string{"echo pre"},
			"post_scaffold": []string{"go mod tidy", "git init"},
		},
	})

	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# My Template\nThis is great."), 0o644)
	os.WriteFile(filepath.Join(dir, "HOWTO.md"), []byte("# How To\nStep 1: do things."), 0o644)

	var buf bytes.Buffer
	err := displayTemplateInfo(&buf, dir)
	require.NoError(t, err)

	out := buf.String()

	// Metadata
	assert.Contains(t, out, "Name:         my-template")
	assert.Contains(t, out, "Version:      1.2.0")
	assert.Contains(t, out, "Description:  A test template")

	// Variables
	assert.Contains(t, out, "Variables:")
	assert.Contains(t, out, "project_name")
	assert.Contains(t, out, "use_docker")
	assert.Contains(t, out, "(boolean)")
	assert.Contains(t, out, "license")
	assert.Contains(t, out, "(choice:")

	// Hooks
	assert.Contains(t, out, "Hooks:")
	assert.Contains(t, out, "pre_scaffold:")
	assert.Contains(t, out, "echo pre")
	assert.Contains(t, out, "post_scaffold:")
	assert.Contains(t, out, "go mod tidy")

	// README
	assert.Contains(t, out, "--- README ---")
	assert.Contains(t, out, "My Template")

	// HOWTO
	assert.Contains(t, out, "--- HOWTO ---")
	assert.Contains(t, out, "How To")
}

func TestUT_DisplayTemplateInfo_MinimalTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	createTemplateConfig(t, dir, map[string]any{
		"name": "minimal",
		"vars": map[string]any{},
	})

	var buf bytes.Buffer
	err := displayTemplateInfo(&buf, dir)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Name:         minimal")
	assert.NotContains(t, out, "Version:")
	assert.NotContains(t, out, "Variables:")
	assert.NotContains(t, out, "Hooks:")
	assert.NotContains(t, out, "--- README ---")
	assert.NotContains(t, out, "--- HOWTO ---")
}

func TestUT_DisplayTemplateInfo_MissingConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	var buf bytes.Buffer
	err := displayTemplateInfo(&buf, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a TAG template")
}

func TestUT_DisplayTemplateInfo_NoReadmeNoHowto(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	createTemplateConfig(t, dir, map[string]any{
		"name": "no-docs",
		"vars": map[string]any{
			"name": "test",
		},
	})

	var buf bytes.Buffer
	err := displayTemplateInfo(&buf, dir)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Name:         no-docs")
	assert.Contains(t, out, "Variables:")
	assert.NotContains(t, out, "--- README ---")
	assert.NotContains(t, out, "--- HOWTO ---")
}

func TestUT_DisplayTemplateInfo_OnlyHowto(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	createTemplateConfig(t, dir, map[string]any{
		"name": "howto-only",
		"vars": map[string]any{},
	})

	os.WriteFile(filepath.Join(dir, "HOWTO.md"), []byte("# Steps\n1. Do this"), 0o644)

	var buf bytes.Buffer
	err := displayTemplateInfo(&buf, dir)
	require.NoError(t, err)

	out := buf.String()
	assert.NotContains(t, out, "--- README ---")
	assert.Contains(t, out, "--- HOWTO ---")
	assert.Contains(t, out, "Steps")
}

func TestUT_DisplayTemplateInfo_EmptyReadme(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	createTemplateConfig(t, dir, map[string]any{
		"name": "empty-readme",
		"vars": map[string]any{},
	})

	os.WriteFile(filepath.Join(dir, "README.md"), []byte(""), 0o644)

	var buf bytes.Buffer
	err := displayTemplateInfo(&buf, dir)
	require.NoError(t, err)

	out := buf.String()
	assert.NotContains(t, out, "--- README ---")
}

// Tests that library resolution is tried first.
// WARNING: This test mutates package-level newLocalLibrary; do NOT use t.Parallel().
func TestUT_ResolveTemplateDir_LibraryFirst(t *testing.T) {
	templateDir := setupFakeLibrary(t, "my-lib-template")

	// Write a minimal config so the template is valid
	createTemplateConfig(t, templateDir, map[string]any{
		"name": "my-lib-template",
		"vars": map[string]any{},
	})

	c := createTestCLIContext(t, []string{"my-lib-template"}, nil)
	resolved, err := resolveTemplateDir(c, "my-lib-template")
	require.NoError(t, err)
	assert.Equal(t, templateDir, resolved)
}

func TestUT_ResolveTemplateDir_LocalPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	createTemplateConfig(t, dir, map[string]any{
		"name": "local-test",
		"vars": map[string]any{},
	})

	c := createTestCLIContext(t, []string{dir}, nil)
	resolved, err := resolveTemplateDir(c, dir)
	require.NoError(t, err)
	assert.Equal(t, dir, resolved)
}

func TestUT_DisplayMetadata_AllFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	displayMetadata(&buf, &scaffold.TemplateConfig{
		Name:        "test",
		Version:     "2.0.0",
		Description: "A great template",
	})

	out := buf.String()
	assert.Contains(t, out, "Name:         test")
	assert.Contains(t, out, "Version:      2.0.0")
	assert.Contains(t, out, "Description:  A great template")
}

func TestUT_DisplayMetadata_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	displayMetadata(&buf, &scaffold.TemplateConfig{})

	assert.Empty(t, buf.String())
}

func TestUT_DisplayVariables_Sorted(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	displayVariables(&buf, &scaffold.TemplateConfig{
		Vars: map[string]scaffold.VariableDef{
			"zebra":   {Type: scaffold.VarTypeString},
			"alpha":   {Type: scaffold.VarTypeString, Default: "hello"},
			"beta":    {Type: scaffold.VarTypeBoolean},
			"charlie": {Type: scaffold.VarTypeChoice, Options: []string{"a", "b", "c"}},
		},
	})

	out := buf.String()
	// Check order: alpha before beta before charlie before zebra
	alphaIdx := strings.Index(out, "alpha")
	betaIdx := strings.Index(out, "beta")
	charlieIdx := strings.Index(out, "charlie")
	zebraIdx := strings.Index(out, "zebra")

	assert.Greater(t, betaIdx, alphaIdx)
	assert.Greater(t, charlieIdx, betaIdx)
	assert.Greater(t, zebraIdx, charlieIdx)

	assert.Contains(t, out, "alpha")
	assert.Contains(t, out, "= hello")
	assert.Contains(t, out, "(boolean)")
	assert.Contains(t, out, "(choice: [a b c])")
}

func TestUT_DisplayHooks_NoHooks(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	displayHooks(&buf, &scaffold.TemplateConfig{})
	assert.Empty(t, buf.String())
}

func TestUT_DisplayHooks_EmptyHooksConfig(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	displayHooks(&buf, &scaffold.TemplateConfig{
		Hooks: &types.HooksConfig{},
	})
	assert.Empty(t, buf.String())
}

// --- #350: `tag template info --format json` --------------------------------

func TestUT_TemplateInfo_FormatFlagBothPositions_ProducesJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTemplateConfig(t, dir, map[string]any{
		"name": "both-orderings",
		"vars": map[string]any{},
	})

	trailing := runCLI(t, templateInfoCommand(), "info", dir, "--format", formatJSON)
	require.NoError(t, trailing.Err)

	leading := runCLI(t, templateInfoCommand(), "info", "--format", formatJSON, dir)
	require.NoError(t, leading.Err)

	var trailingParsed, leadingParsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(trailing.Writer), &trailingParsed), "trailing: %s", trailing.Writer)
	require.NoError(t, json.Unmarshal([]byte(leading.Writer), &leadingParsed), "leading: %s", leading.Writer)
	assert.Equal(t, trailingParsed, leadingParsed)
	assert.Equal(t, "both-orderings", trailingParsed["name"])
}

func TestUT_TemplateInfo_EmptyFormat_IsUsageError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTemplateConfig(t, dir, map[string]any{"name": "x", "vars": map[string]any{}})

	run := runCLI(t, templateInfoCommand(), "info", dir, "--format=")
	require.Error(t, run.Err)
	assert.Contains(t, run.Err.Error(), `unsupported format ""`)
}

func TestUT_TemplateInfo_RejectsSecondPositional(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTemplateConfig(t, dir, map[string]any{"name": "x", "vars": map[string]any{}})

	run := runCLI(t, templateInfoCommand(), "info", dir, "extra-arg")
	require.Error(t, run.Err)
	assert.Contains(t, run.Err.Error(), "expected exactly one template argument, got 2")
}

func TestUT_TemplateInfo_FormatTextEqualsDefault(t *testing.T) {
	t.Parallel()

	dir := seedInfoTemplate(t)

	defaultRun := runCLI(t, templateInfoCommand(), "info", dir)
	require.NoError(t, defaultRun.Err)

	explicitRun := runCLI(t, templateInfoCommand(), "info", dir, "--format", formatText)
	require.NoError(t, explicitRun.Err)

	assert.Equal(t, defaultRun.Writer, explicitRun.Writer)
}

func TestUT_TemplateInfoJSON_LeavesStdoutClean(t *testing.T) {
	dir := seedInfoTemplate(t)

	run := runCLICapturingStdout(t, templateInfoCommand(), "info", dir, "--format", formatJSON)
	require.NoError(t, run.Err)
	assert.Empty(t, run.Stdout, "a JSON command must not bypass c.App.Writer")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed), "writer did not hold the JSON: %s", run.Writer)
	assert.Equal(t, "go-api", parsed["name"])
}

func TestUT_TemplateInfoJSON_MissingConfigErrorsBeforeWriting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	run := runCLI(t, templateInfoCommand(), "info", dir, "--format", formatJSON)
	require.Error(t, run.Err)
	assert.Contains(t, run.Err.Error(), "not a TAG template")
	assert.Empty(t, run.Writer, "no partial document may be written before the load error")
}

func TestUT_TemplateInfoJSON_EmptyCollectionsAreArrays(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTemplateConfig(t, dir, map[string]any{
		"name": "no-vars-no-hooks",
		"vars": map[string]any{},
	})

	run := runCLI(t, templateInfoCommand(), "info", dir, "--format", formatJSON)
	require.NoError(t, run.Err)

	// Asserting on raw encoded bytes matters: `null` and `[]` both unmarshal
	// to a nil Go slice, so a struct-level assertion would be vacuous here.
	assert.Contains(t, run.Writer, `"variables": []`)
	assert.Contains(t, run.Writer, `"pre_scaffold": []`)
	assert.Contains(t, run.Writer, `"post_scaffold": []`)
	assert.NotContains(t, run.Writer, "null")
}

func TestUT_TemplateInfoJSON_NeverInvokesGlamour(t *testing.T) {
	dir := seedInfoTemplate(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hi"), 0o600))

	calls := 0
	orig := renderMarkdown
	renderMarkdown = func(s string) (string, error) {
		calls++
		return orig(s)
	}
	t.Cleanup(func() { renderMarkdown = orig })

	run := runCLI(t, templateInfoCommand(), "info", dir, "--format", formatJSON)
	require.NoError(t, run.Err)
	assert.Zero(t, calls, "the JSON path must never render markdown through glamour")
}

func TestUT_TemplateInfo_TextRendersDocsThroughGlamour(t *testing.T) {
	dir := seedInfoTemplate(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hi"), 0o600))

	calls := 0
	orig := renderMarkdown
	renderMarkdown = func(s string) (string, error) {
		calls++
		return orig(s)
	}
	t.Cleanup(func() { renderMarkdown = orig })

	run := runCLI(t, templateInfoCommand(), "info", dir)
	require.NoError(t, run.Err)
	assert.Equal(t, 1, calls, "the text path must render the README through glamour")
}

func TestUT_TemplateInfoJSON_ContainsNoANSI(t *testing.T) {
	dir := seedInfoTemplate(t)

	run := runCLI(t, templateInfoCommand(), "info", dir, "--format", formatJSON)
	require.NoError(t, run.Err)

	// A raw-byte scan of the encoded output would be vacuous: encoding/json
	// escapes ESC (0x1b) as , so valid JSON can never contain a literal
	// ESC byte. Decode first, then scan the string values.
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed))

	var walk func(v any)
	walk = func(v any) {
		switch val := v.(type) {
		case string:
			assert.NotContains(t, val, "\x1b", "decoded JSON string value contains a raw ESC byte")
		case map[string]any:
			for _, item := range val {
				walk(item)
			}
		case []any:
			for _, item := range val {
				walk(item)
			}
		}
	}
	walk(parsed)
}

func TestUT_BuildTemplateInfoJSON_Shape(t *testing.T) {
	t.Parallel()

	config := &scaffold.TemplateConfig{
		Name:        "go-api",
		Description: "A Go API",
		Version:     "1.0.0",
		Vars: map[string]scaffold.VariableDef{
			"project_name": {Type: scaffold.VarTypeString, Default: "app"},
		},
		Hooks: &types.HooksConfig{
			PreScaffold: []string{"echo pre"},
		},
	}

	dto := buildTemplateInfoJSON(config, true, false)

	assert.Equal(t, "go-api", dto.Name)
	assert.Equal(t, "A Go API", dto.Description)
	assert.Equal(t, "1.0.0", dto.Version)
	assert.True(t, dto.HasReadme)
	assert.False(t, dto.HasHowto)
	require.Len(t, dto.Variables, 1)
	assert.Equal(t, "project_name", dto.Variables[0].Name)
	assert.Equal(t, []string{"echo pre"}, dto.Hooks.PreScaffold)
	assert.Equal(t, []string{}, dto.Hooks.PostScaffold)
}

func TestUT_BuildTemplateInfoJSON_VariablesSortedByName(t *testing.T) {
	t.Parallel()

	config := &scaffold.TemplateConfig{
		Vars: map[string]scaffold.VariableDef{
			"zebra": {Type: scaffold.VarTypeString},
			"alpha": {Type: scaffold.VarTypeString},
			"mike":  {Type: scaffold.VarTypeString},
		},
	}

	dto := buildTemplateInfoJSON(config, false, false)

	require.Len(t, dto.Variables, 3)
	assert.Equal(t, []string{"alpha", "mike", "zebra"},
		[]string{dto.Variables[0].Name, dto.Variables[1].Name, dto.Variables[2].Name})
}

// TestUT_BuildTemplateInfoJSON_UsesResolvedVarsNotRaw hand-builds a
// TemplateConfig with RawVars populated but Vars nil — a shape the loader can
// never produce (ParseTemplateConfig always populates Vars), which is exactly
// why the builder must read Vars, not RawVars: a builder that read RawVars
// would only be caught by a hand-built fixture like this one.
func TestUT_BuildTemplateInfoJSON_UsesResolvedVarsNotRaw(t *testing.T) {
	t.Parallel()

	config := &scaffold.TemplateConfig{
		RawVars: map[string]any{"project_name": "should-not-appear"},
		Vars:    nil,
	}

	dto := buildTemplateInfoJSON(config, false, false)
	assert.Empty(t, dto.Variables)
}

func TestUT_BuildTemplateInfoJSON_VariableFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		def  scaffold.VariableDef
		want templateInfoVariableJSON
	}{
		{
			name: "short-form string default",
			def:  scaffold.VariableDef{Type: scaffold.VarTypeString, Default: "my-app"},
			want: templateInfoVariableJSON{Name: "v", Type: "string", Default: "my-app"},
		},
		{
			name: "boolean",
			def:  scaffold.VariableDef{Type: scaffold.VarTypeBoolean, Default: true},
			want: templateInfoVariableJSON{Name: "v", Type: "boolean", Default: true},
		},
		{
			name: "choice",
			def:  scaffold.VariableDef{Type: scaffold.VarTypeChoice, Options: []string{"a", "b"}},
			want: templateInfoVariableJSON{Name: "v", Type: "choice", Options: []string{"a", "b"}},
		},
		{
			name: "required with prompt",
			def:  scaffold.VariableDef{Type: scaffold.VarTypeString, Required: true, Prompt: "Enter value"},
			want: templateInfoVariableJSON{Name: "v", Type: "string", Required: true, Prompt: "Enter value"},
		},
		{
			name: "secret",
			def:  scaffold.VariableDef{Type: scaffold.VarTypeString, Secret: true},
			want: templateInfoVariableJSON{Name: "v", Type: "string", Secret: true},
		},
		{
			name: "private var",
			def:  scaffold.VariableDef{Type: scaffold.VarTypeString},
			want: templateInfoVariableJSON{Name: "_computed", Type: "string"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &scaffold.TemplateConfig{
				Vars: map[string]scaffold.VariableDef{tt.want.Name: tt.def},
			}
			dto := buildTemplateInfoJSON(config, false, false)
			require.Len(t, dto.Variables, 1)
			assert.Equal(t, tt.want, dto.Variables[0])
		})
	}
}

func TestUT_BuildTemplateInfoJSON_DocFlagsMatchTextRendering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		readmeContent []byte // nil = file not written at all
		howtoContent  []byte
		wantReadme    bool
		wantHowto     bool
	}{
		{"both missing", nil, nil, false, false},
		{"both empty", []byte(""), []byte(""), false, false},
		{"both non-empty", []byte("# R"), []byte("# H"), true, true},
		{"readme only", []byte("# R"), nil, true, false},
		{"howto only", nil, []byte("# H"), false, true},
		{"readme empty howto non-empty", []byte(""), []byte("# H"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if tt.readmeContent != nil {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), tt.readmeContent, 0o600))
			}
			if tt.howtoContent != nil {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "HOWTO.md"), tt.howtoContent, 0o600))
			}

			gotReadme := docFileHasContent(dir, types.TemplateReadme)
			gotHowto := docFileHasContent(dir, types.TemplateHowto)
			assert.Equal(t, tt.wantReadme, gotReadme)
			assert.Equal(t, tt.wantHowto, gotHowto)

			// Cross-check against the text path's own rule via a probe buffer:
			// renderDocFile prints a "--- LABEL ---" section iff the doc has
			// content, which is the exact rule docFileHasContent must mirror.
			var buf bytes.Buffer
			renderDocFile(&buf, dir, types.TemplateReadme, "README")
			assert.Equal(t, tt.wantReadme, strings.Contains(buf.String(), "--- README ---"))

			buf.Reset()
			renderDocFile(&buf, dir, types.TemplateHowto, "HOWTO")
			assert.Equal(t, tt.wantHowto, strings.Contains(buf.String(), "--- HOWTO ---"))
		})
	}
}

// TestUT_TemplateInfo_UpdateFlagWithJSON covers #350's "--update (force cache
// refresh) still works in JSON mode" acceptance criterion.
//
// What #350 can plausibly break is not the remote cache semantics (untouched)
// but the reparser's valueless-flag path: --update is a BoolFlag, so if
// reparseTrailingFlags mishandled it, it would swallow the following token and
// --format would never be seen. Both orderings are exercised for that reason,
// and a local template dir is used so no network is involved — ForceUpdate only
// applies once resolution falls through to the remote resolver.
func TestUT_TemplateInfo_UpdateFlagWithJSON(t *testing.T) {
	t.Parallel()

	dir := seedInfoTemplate(t)

	cases := []struct {
		name string
		argv []string
	}{
		{"update before format", []string{"info", dir, "--update", "--format", "json"}},
		{"format before update", []string{"info", dir, "--format", "json", "--update"}},
		{"both leading", []string{"info", "--update", "--format", "json", dir}},
		{"short alias", []string{"info", dir, "-u", "--format", "json"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := runCLI(t, templateInfoCommand(), tc.argv...)
			require.NoError(t, run.Err)

			var got map[string]any
			require.NoError(t, json.Unmarshal([]byte(run.Writer), &got),
				"--update must not disturb the JSON document")
			assert.Equal(t, "go-api", got["name"])
		})
	}
}

// TestUT_TemplateInfoJSON_AuthorSuppliedANSIIsEscapedNotGenerated makes the
// no-ANSI guarantee precise, because the guarantee is narrower than it sounds
// and the obvious test for it passes by accident.
//
// TestUT_TemplateInfoJSON_ContainsNoANSI uses a fixture containing no escape
// sequences, so it cannot tell "TAG never emits ANSI" apart from "nothing had
// any". What #350 actually requires is that TAG never GENERATES ANSI into the
// JSON path — that is, glamour's rendered output never reaches it. Bytes the
// template author put in their own description are data, and the text path
// prints them raw today; silently stripping them in JSON would corrupt user
// content, so this asserts round-tripping rather than sanitisation.
//
// Two distinct claims, both checked here:
//  1. the ENCODED document contains no raw ESC byte (encoding/json escapes it),
//     so piping `tag template info --format json` to a terminal is safe;
//  2. the decoded value is byte-for-byte what the author wrote — TAG neither
//     added escape sequences of its own nor removed the author's.
func TestUT_TemplateInfoJSON_AuthorSuppliedANSIIsEscapedNotGenerated(t *testing.T) {
	t.Parallel()

	const redDescription = "\x1b[31mdanger\x1b[0m"

	dir := t.TempDir()
	createTemplateConfig(t, dir, map[string]any{
		"name":        "ansi-template",
		"description": redDescription,
		"vars":        map[string]any{"greeting": "\x1b[32mhi\x1b[0m"},
	})

	run := runCLI(t, templateInfoCommand(), "info", dir, "--format", formatJSON)
	require.NoError(t, run.Err)

	assert.NotContains(t, run.Writer, "\x1b",
		"the encoded JSON document must never carry a raw ESC byte")

	var parsed struct {
		Description string `json:"description"`
		Variables   []struct {
			Name    string `json:"name"`
			Default any    `json:"default"`
		} `json:"variables"`
	}
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed))

	assert.Equal(t, redDescription, parsed.Description,
		"author-supplied bytes must round-trip unchanged, neither stripped nor added to")
	require.Len(t, parsed.Variables, 1)
	assert.Equal(t, "\x1b[32mhi\x1b[0m", parsed.Variables[0].Default)
}
