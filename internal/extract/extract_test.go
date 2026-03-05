package extract

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- BuildRules ---

func TestUT_BuildRules_SingleWord(t *testing.T) {
	rules := BuildRules("user")

	// Should produce 6 rules: plural upper/pascal/lower + singular upper/pascal/lower
	assert.Len(t, rules, 6)

	// Longest first.
	assert.True(t, len(rules[0].Needle) >= len(rules[len(rules)-1].Needle),
		"rules should be sorted longest-needle-first")

	// Check all expected needles are present.
	needles := make(map[string]string, len(rules))
	for _, r := range rules {
		needles[r.Needle] = r.Expr
	}

	assert.Equal(t, "{{ name | plural | upper }}", needles["USERS"])
	assert.Equal(t, "{{ name | plural | pascal }}", needles["Users"])
	assert.Equal(t, "{{ name | plural }}", needles["users"])
	assert.Equal(t, "{{ name | upper }}", needles["USER"])
	assert.Equal(t, "{{ name | pascal }}", needles["User"])
	assert.Equal(t, "{{ name }}", needles["user"])
}

func TestUT_BuildRules_NormalizesUppercase(t *testing.T) {
	rules := BuildRules("USER")

	needles := make(map[string]bool, len(rules))
	for _, r := range rules {
		needles[r.Needle] = true
	}

	assert.True(t, needles["user"], "should contain lowercase form")
	assert.True(t, needles["User"], "should contain pascal form")
}

func TestUT_BuildRules_AlreadyPlural(t *testing.T) {
	rules := BuildRules("users")

	// "users" pluralized is still "users", so plural forms should be deduped.
	needles := make(map[string]bool, len(rules))
	for _, r := range rules {
		needles[r.Needle] = true
	}

	// Should still have singular forms (user -> users is the plural).
	// flect.Pluralize("users") = "users", so no separate plural entries.
	assert.True(t, len(rules) > 0)
}

// --- FindOccurrences ---

func TestUT_FindOccurrences_SimpleMatch(t *testing.T) {
	content := []byte("func handleUser() {}")
	rules := BuildRules("user")

	occs := FindOccurrences(content, rules)

	require.Len(t, occs, 1)
	assert.Equal(t, "User", occs[0].Rule.Needle)
	assert.Equal(t, "{{ name | pascal }}", occs[0].Rule.Expr)
}

func TestUT_FindOccurrences_SnakeCompound(t *testing.T) {
	content := []byte("user_handler := newHandler()")
	rules := BuildRules("user")

	occs := FindOccurrences(content, rules)

	require.Len(t, occs, 1)
	assert.Equal(t, "user", occs[0].Rule.Needle)
}

func TestUT_FindOccurrences_PascalCase(t *testing.T) {
	content := []byte("type UserHandler struct{}")
	rules := BuildRules("user")

	occs := FindOccurrences(content, rules)

	require.Len(t, occs, 1)
	assert.Equal(t, "User", occs[0].Rule.Needle)
}

func TestUT_FindOccurrences_CamelCase(t *testing.T) {
	content := []byte("func getUser() {}")
	rules := BuildRules("user")

	occs := FindOccurrences(content, rules)

	require.Len(t, occs, 1)
	assert.Equal(t, "User", occs[0].Rule.Needle)
}

func TestUT_FindOccurrences_Upper(t *testing.T) {
	content := []byte("const USER_ID = 1")
	rules := BuildRules("user")

	occs := FindOccurrences(content, rules)

	require.Len(t, occs, 1)
	assert.Equal(t, "USER", occs[0].Rule.Needle)
}

func TestUT_FindOccurrences_Plural(t *testing.T) {
	content := []byte("var users []User")
	rules := BuildRules("user")

	occs := FindOccurrences(content, rules)

	require.Len(t, occs, 2)

	needles := map[string]bool{}
	for _, o := range occs {
		needles[o.Rule.Needle] = true
	}
	assert.True(t, needles["users"])
	assert.True(t, needles["User"])
}

func TestUT_FindOccurrences_RejectsSuperuser(t *testing.T) {
	content := []byte("superuser := true")
	rules := BuildRules("user")

	occs := FindOccurrences(content, rules)

	assert.Empty(t, occs, "should not match 'user' inside 'superuser'")
}

func TestUT_FindOccurrences_RejectsUsername(t *testing.T) {
	content := []byte("username := input")
	rules := BuildRules("user")

	occs := FindOccurrences(content, rules)

	assert.Empty(t, occs, "should not match 'user' inside 'username'")
}

func TestUT_FindOccurrences_MultiplePerLine(t *testing.T) {
	content := []byte("func (u *User) GetUser() User {}")
	rules := BuildRules("user")

	occs := FindOccurrences(content, rules)

	assert.Len(t, occs, 3, "should find all three 'User' occurrences")
}

func TestUT_FindOccurrences_LongestFirstPriority(t *testing.T) {
	content := []byte("var users []string")
	rules := BuildRules("user")

	occs := FindOccurrences(content, rules)

	require.Len(t, occs, 1)
	assert.Equal(t, "users", occs[0].Rule.Needle, "should match 'users' not 'user'")
}

func TestUT_FindOccurrences_NoMatches(t *testing.T) {
	content := []byte("func handleOrder() {}")
	rules := BuildRules("user")

	occs := FindOccurrences(content, rules)

	assert.Empty(t, occs)
}

func TestUT_FindOccurrences_SkipTemplateExpr(t *testing.T) {
	content := []byte("{{ name | pascal }}Handler handles {{ name }} operations")
	rules := BuildRules("name")

	occs := FindOccurrences(content, rules)

	// Should not match "name" inside {{ }} expressions.
	for _, occ := range occs {
		assert.False(t, insideTemplateExpr(content, occ.Start),
			"should not match inside template expression")
	}
}

func TestUT_FindOccurrences_LineNumbers(t *testing.T) {
	content := []byte("line one\nuser here\nline three")
	rules := BuildRules("user")

	occs := FindOccurrences(content, rules)

	require.Len(t, occs, 1)
	assert.Equal(t, 2, occs[0].LineNum)
	assert.Equal(t, "user here", occs[0].Context)
}

// --- Apply ---

func TestUT_Apply_Single(t *testing.T) {
	content := []byte("type UserHandler struct{}")
	rules := BuildRules("user")
	occs := FindOccurrences(content, rules)

	result := Apply(content, occs)

	assert.Equal(t, "type {{ name | pascal }}Handler struct{}", string(result))
}

func TestUT_Apply_Multiple(t *testing.T) {
	content := []byte("// User handles user stuff")
	rules := BuildRules("user")
	occs := FindOccurrences(content, rules)

	result := Apply(content, occs)

	assert.Equal(t, "// {{ name | pascal }} handles {{ name }} stuff", string(result))
}

func TestUT_Apply_NoOp(t *testing.T) {
	content := []byte("nothing to replace here")
	result := Apply(content, nil)

	assert.Equal(t, "nothing to replace here", string(result))
}

func TestUT_Apply_PreservesContent(t *testing.T) {
	content := []byte("func main() {\n\tfmt.Println(\"hello\")\n}")
	rules := BuildRules("user")
	occs := FindOccurrences(content, rules)

	result := Apply(content, occs)

	assert.Equal(t, string(content), string(result), "no matches → content unchanged")
}

// --- BuildToPath ---

func TestUT_BuildToPath_SnakePath(t *testing.T) {
	path := "internal/handler/user_handler.go"
	rules := BuildRules("user")

	result := BuildToPath(path, rules)

	assert.Equal(t, "internal/handler/{{ name }}_handler.go", result)
}

func TestUT_BuildToPath_NestedDirs(t *testing.T) {
	path := "internal/user/user_service.go"
	rules := BuildRules("user")

	result := BuildToPath(path, rules)

	assert.Equal(t, "internal/{{ name }}/{{ name }}_service.go", result)
}

func TestUT_BuildToPath_NoMatches(t *testing.T) {
	path := "internal/handler/base.go"
	rules := BuildRules("user")

	result := BuildToPath(path, rules)

	assert.Equal(t, "internal/handler/base.go", result)
}

// --- isWordBoundary ---

func TestUT_IsWordBoundary_StartOfString(t *testing.T) {
	content := []byte("user_name")

	assert.True(t, isWordBoundary(content, 0, 4), "start of string is a boundary")
}

func TestUT_IsWordBoundary_EndOfString(t *testing.T) {
	content := []byte("get_user")

	assert.True(t, isWordBoundary(content, 4, 4), "end of string is a boundary")
}

func TestUT_IsWordBoundary_Underscore(t *testing.T) {
	content := []byte("get_user_id")

	assert.True(t, isWordBoundary(content, 4, 4), "underscore is a boundary")
}

func TestUT_IsWordBoundary_PascalTransition(t *testing.T) {
	content := []byte("getUserById")

	assert.True(t, isWordBoundary(content, 3, 4), "camel→Pascal transition is a boundary")
}

func TestUT_IsWordBoundary_InsideWord(t *testing.T) {
	content := []byte("superuser")

	assert.False(t, isWordBoundary(content, 5, 4), "inside a word is not a boundary")
}

func TestUT_IsWordBoundary_DigitBoundary(t *testing.T) {
	content := []byte("user1")

	assert.True(t, isWordBoundary(content, 0, 4), "digit after match is a boundary")
}

// --- Run (orchestration) ---

func TestUT_Run_FullOrchestration(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "user_handler.go")

	srcContent := `package handler

// UserHandler handles user operations.
type UserHandler struct {
	users []string
}
`
	require.NoError(t, os.WriteFile(src, []byte(srcContent), 0o644))

	tagDir := filepath.Join(dir, ".tag")
	var buf bytes.Buffer

	opts := Options{
		Name:   "user",
		As:     "handler",
		TagDir: tagDir,
		Writer: &buf,
	}

	result, err := opts.run(t, src)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(tagDir, "handler", "user_handler.go"), result.TemplatePath)
	assert.Contains(t, result.ToPath, "{{ name }}")
	assert.Greater(t, result.Replacements, 0)
	assert.Contains(t, result.Content, "---\nto:")
	assert.Contains(t, result.Content, "{{ name | pascal }}Handler")
	assert.Contains(t, result.Content, "{{ name | plural }}")

	// Verify file was written.
	written, err := os.ReadFile(result.TemplatePath)
	require.NoError(t, err)
	assert.Equal(t, result.Content, string(written))
}

func TestUT_Run_DryRun(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "user_handler.go")
	require.NoError(t, os.WriteFile(src, []byte("type User struct{}"), 0o644))

	tagDir := filepath.Join(dir, ".tag")
	var buf bytes.Buffer

	opts := Options{
		Name:   "user",
		As:     "handler",
		DryRun: true,
		TagDir: tagDir,
		Writer: &buf,
	}

	result, err := Run(opts, src)
	require.NoError(t, err)

	assert.Greater(t, result.Replacements, 0)
	assert.Contains(t, buf.String(), "Dry Run")

	// File should NOT be written.
	_, err = os.Stat(result.TemplatePath)
	assert.True(t, os.IsNotExist(err))
}

func TestUT_Run_InteractiveWithMock(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "user_handler.go")
	require.NoError(t, os.WriteFile(src, []byte("User and user"), 0o644))

	tagDir := filepath.Join(dir, ".tag")

	mock := &mockConfirmer{decisions: []Decision{DecisionYes, DecisionNo}}

	opts := Options{
		Name:        "user",
		As:          "handler",
		Interactive: true,
		TagDir:      tagDir,
		Prompter:    mock,
		Writer:      &bytes.Buffer{},
	}

	result, err := Run(opts, src)
	require.NoError(t, err)

	// Only first occurrence should be replaced (Yes), second skipped (No).
	assert.Equal(t, 1, result.Replacements)
}

// Helper: run via the public Run function.
func (opts Options) run(t *testing.T, src string) (*Result, error) {
	t.Helper()
	return Run(opts, src)
}

// mockConfirmer returns pre-configured decisions in order.
type mockConfirmer struct {
	decisions []Decision
	idx       int
}

func (m *mockConfirmer) Confirm(_ Occurrence) (Decision, error) {
	if m.idx >= len(m.decisions) {
		return DecisionQuit, nil
	}
	d := m.decisions[m.idx]
	m.idx++
	return d, nil
}
