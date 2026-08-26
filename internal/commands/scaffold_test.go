package commands

import (
	"bytes"
	"errors"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
)

func TestUT_SuggestConvertedTemplateName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple local path",
			input:    "./my-template",
			expected: "my-template-tag",
		},
		{
			name:     "github shorthand",
			input:    "gh:user/awesome-template",
			expected: "awesome-template-tag",
		},
		{
			name:     "github shorthand with version",
			input:    "gh:user/awesome-template@v1.0.0",
			expected: "awesome-template-tag",
		},
		{
			name:     "gitlab shorthand",
			input:    "gl:org/cookiecutter-django",
			expected: "django-tag",
		},
		{
			name:     "bitbucket shorthand",
			input:    "bb:team/cookiecutter-go-api",
			expected: "go-api-tag",
		},
		{
			name:     "cookiecutter prefix stripped",
			input:    "cookiecutter-python",
			expected: "python-tag",
		},
		{
			name:     "no cookiecutter prefix",
			input:    "my-project",
			expected: "my-project-tag",
		},
		{
			name:     "absolute path",
			input:    "/home/user/templates/cookiecutter-rust",
			expected: "rust-tag",
		},
		{
			name:     "nested subdir in shorthand",
			input:    "gh:user/templates/subdir",
			expected: "templates-tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := suggestConvertedTemplateName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_DeriveTemplateName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "github shorthand",
			input:    "gh:user/awesome-template",
			expected: "awesome-template",
		},
		{
			name:     "bitbucket shorthand",
			input:    "bb:whalar/go-ms-service-template",
			expected: "go-ms-service-template",
		},
		{
			name:     "gitlab shorthand",
			input:    "gl:org/my-template",
			expected: "my-template",
		},
		{
			name:     "github with version",
			input:    "gh:user/repo@v1.0.0",
			expected: "repo",
		},
		{
			name:     "cookiecutter prefix stripped",
			input:    "gh:user/cookiecutter-django",
			expected: "django",
		},
		{
			name:     "git URL",
			input:    "https://github.com/user/my-template.git",
			expected: "my-template",
		},
		{
			name:     "cookiecutter prefix only with .git suffix",
			input:    "gh:user/cookiecutter-.git",
			expected: "gh:user/cookiecutter-.git",
		},
		{
			name:     "cookiecutter prefix only",
			input:    "gh:user/cookiecutter-",
			expected: "gh:user/cookiecutter-",
		},
		{
			name:     "URL without .git suffix",
			input:    "https://github.com/user/my-template",
			expected: "my-template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := remote.DeriveName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_LooksLikeBareName(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// Bare names → true
		{"go-api", true},
		{"my-template", true},
		{"flask", true},

		// Local paths → false
		{"./my-template", false},
		{"../templates/foo", false},
		{"/absolute/path", false},

		// Remote shorthands → false
		{"gh:user/repo", false},
		{"gl:org/template", false},
		{"bb:team/project", false},

		// URLs → false
		{"https://github.com/user/repo.git", false},
		{"http://example.com/template.zip", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, looksLikeBareName(tt.input))
		})
	}
}

func TestUT_ScaffoldAction_MissingArguments(t *testing.T) {
	ctx := createTestCLIContext(t, []string{}, nil)

	err := scaffoldAction(ctx, testVersion)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template argument required")
}

func TestUT_ScaffoldFromRef_LibraryNameResolvesToLibrary(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "my-template")

	ctx := createTestCLIContext(t, []string{"my-template"}, nil)

	// scaffoldFromRef should detect "my-template" as a library name and
	// attempt to scaffold from the library. It will fail because the
	// library template directory has no tag.template.json, but the error
	// should come from scaffolding (not from the remote resolver).
	err := scaffoldFromRef(ctx, []string{"my-template"}, false, testVersion)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scaffolding failed")
	assert.NotContains(t, err.Error(), "failed to resolve template")
}

func TestUT_ScaffoldFromRef_RemoteRefSkipsLibrary(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "my-template")

	ctx := createTestCLIContext(t, []string{"gh:user/my-template"}, nil)

	// A remote shorthand should NOT match the library, even if a library
	// entry with the same base name exists. It should go to the remote
	// resolver and fail with a resolver error (not a scaffold init error).
	err := scaffoldFromRef(ctx, []string{"gh:user/my-template"}, false, testVersion)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve template")
}

func TestUT_ScaffoldFromRef_LocalPathSkipsLibrary(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "my-template")

	ctx := createTestCLIContext(t, []string{"./my-template"}, nil)

	// A local path should NOT match the library. It should go to the
	// remote/local resolver.
	err := scaffoldFromRef(ctx, []string{"./my-template"}, false, testVersion)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve template")
}

func TestUT_DisplayScaffoldSummary_WritesToOutput(t *testing.T) {
	var buf bytes.Buffer
	result := scaffold.ScaffoldResult{
		OutputDir:   "/tmp/my-app",
		ProjectRoot: "/tmp/my-app",
		TemplateDir: t.TempDir(),
		Vars: map[string]any{
			"project_name": "my-app",
		},
		Opts: scaffold.Options{
			TemplateName:    "go-api",
			TemplateRef:     "gh:user/go-api",
			TemplateVersion: "1.0.0",
		},
	}

	displayScaffoldSummary(&buf, result)

	output := buf.String()
	assert.Contains(t, output, "Scaffolding complete!")
	assert.Contains(t, output, "Output: /tmp/my-app")
	assert.Contains(t, output, "Project: my-app")
	assert.Contains(t, output, "Template: gh:user/go-api (1.0.0)")
	assert.Contains(t, output, "cd /tmp/my-app")
}

func TestUT_DisplayScaffoldSummary_NoTemplateOrigin(t *testing.T) {
	var buf bytes.Buffer
	result := scaffold.ScaffoldResult{
		OutputDir:   "/tmp/local-project",
		ProjectRoot: "/tmp/local-project",
		TemplateDir: t.TempDir(),
		Vars: map[string]any{
			"project_name": "local-project",
		},
		Opts: scaffold.Options{}, // No template name
	}

	displayScaffoldSummary(&buf, result)

	output := buf.String()
	assert.Contains(t, output, "Scaffolding complete!")
	assert.Contains(t, output, "Output: /tmp/local-project")
	assert.NotContains(t, output, "Template:")
}

// TestUT_SuggestConvertedTemplateName_UsesDeriveName is a no-change guard:
// suggestConvertedTemplateName (scaffold.go:607) suggests an OUTPUT
// DIRECTORY name for `tag convert`, not a library name, and #430
// deliberately left it on remote.DeriveName. This exists to fail loudly if
// a future sweep migrates that call site to remote.LibraryName by mistake.
func TestUT_SuggestConvertedTemplateName_UsesDeriveName(t *testing.T) {
	const ref = "gh:acme/service-template"
	assert.Equal(t, remote.DeriveName(ref)+"-tag", suggestConvertedTemplateName(ref))
}

func TestUT_ClassifyLibrarySlot(t *testing.T) {
	notFoundErr := func(t *testing.T) error {
		t.Helper()
		lib := library.NewLocal(t.TempDir())
		_, err := lib.Get("does-not-exist")
		require.Error(t, err)
		require.True(t, errors.Is(err, library.ErrTemplateNotFound))
		return err
	}

	const ref = "gh:acme/api@v1"

	tests := []struct {
		name        string
		entry       *library.Entry
		getErr      func(t *testing.T) error
		initErr     error
		templateRef string
		want        librarySlot
	}{
		{
			name:        "not found wrapped in a real LibraryError is free",
			getErr:      notFoundErr,
			templateRef: ref,
			want:        slotFree,
		},
		{
			name:        "identical source is the same template",
			entry:       &library.Entry{Name: "api", Source: ref},
			templateRef: ref,
			want:        slotSameSource,
		},
		{
			name:        "same repo, different version is a different template",
			entry:       &library.Entry{Name: "api", Source: "gh:acme/api@v2"},
			templateRef: ref,
			want:        slotTakenByOther,
		},
		{
			name:        "different source entirely is taken by another template",
			entry:       &library.Entry{Name: "api", Source: "gh:other/api@v1"},
			templateRef: ref,
			want:        slotTakenByOther,
		},
		{
			name:        "library init failure is unavailable",
			initErr:     errors.New("injected init error"),
			templateRef: ref,
			want:        slotUnavailable,
		},
		{
			name:        "a get error that is not ErrTemplateNotFound is unavailable",
			getErr:      func(*testing.T) error { return errors.New("disk exploded") },
			templateRef: ref,
			want:        slotUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var getErr error
			if tt.getErr != nil {
				getErr = tt.getErr(t)
			}
			got := classifyLibrarySlot(tt.entry, getErr, tt.initErr, tt.templateRef)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestUT_PrepareLibrarySlot pins the slot -> opts mapping, which is the whole
// of the #429 fix and the one piece neither classifyLibrarySlot nor
// resolveLibrarySlot covers: they prove the verdict, this proves what the
// verdict DOES. Without it, gutting prepareLibrarySlot's caller leaves every
// unit test in this package green (measured).
func TestUT_PrepareLibrarySlot(t *testing.T) {
	newCtx := func(buf *bytes.Buffer) *cli.Context {
		return cli.NewContext(&cli.App{Writer: buf}, flag.NewFlagSet("test", flag.ContinueOnError), nil)
	}

	t.Run("free slot records the name and skips the project copy", func(t *testing.T) {
		// isolateLibrary mutates package-level var — do NOT use t.Parallel()
		isolateLibrary(t)
		const ref = "gh:acme/service-template@v1"

		var buf bytes.Buffer
		opts := scaffold.Options{}
		shouldAdd := prepareLibrarySlot(newCtx(&buf), &opts, ref, false)

		assert.True(t, shouldAdd, "a free slot must still be added after the scaffold")
		assert.True(t, opts.SkipGeneratorCopy)
		assert.Equal(t, remote.LibraryName(ref), opts.TemplateName)
		assert.Empty(t, buf.String(), "a free slot reports nothing here; addToLibrary speaks after the run")
	})

	t.Run("same source behaves exactly like a free slot", func(t *testing.T) {
		// setupFakeLibraryForRef mutates package-level var — do NOT use t.Parallel()
		const ref = "gh:acme/service-template@v1"
		setupFakeLibraryForRef(t, ref)

		var buf bytes.Buffer
		opts := scaffold.Options{}
		shouldAdd := prepareLibrarySlot(newCtx(&buf), &opts, ref, false)

		assert.True(t, shouldAdd)
		assert.True(t, opts.SkipGeneratorCopy)
		assert.Equal(t, remote.LibraryName(ref), opts.TemplateName)
	})

	t.Run("taken by another source keeps the generators and records no name", func(t *testing.T) {
		// setupFakeLibraryForRef mutates package-level var — do NOT use t.Parallel()
		const seededRef = "gh:acme/service-template@v1"
		setupFakeLibraryForRef(t, seededRef)
		const requestedRef = "gh:acme/service-template@v2"

		var buf bytes.Buffer
		opts := scaffold.Options{}
		shouldAdd := prepareLibrarySlot(newCtx(&buf), &opts, requestedRef, false)

		assert.False(t, shouldAdd, "a foreign occupant must not be overwritten")
		assert.False(t, opts.SkipGeneratorCopy, "the project must keep its own generators")
		assert.Empty(t, opts.TemplateName, "recording the name would resolve the WRONG template's generators")

		out := buf.String()
		assert.Contains(t, out, seededRef, "the message must name the occupying source")
		assert.Contains(t, out, requestedRef, "the message must name this project's source")
	})

	t.Run("taken by another source stays silent under --format json", func(t *testing.T) {
		// setupFakeLibraryForRef mutates package-level var — do NOT use t.Parallel()
		setupFakeLibraryForRef(t, "gh:acme/service-template@v1")

		var buf bytes.Buffer
		opts := scaffold.Options{}
		prepareLibrarySlot(newCtx(&buf), &opts, "gh:acme/service-template@v2", true)

		assert.Empty(t, buf.String(), "stdout carries the JSON document; this message must not corrupt it")
	})

	t.Run("unavailable library fails safe", func(t *testing.T) {
		// setupFakeLibraryError mutates package-level var — do NOT use t.Parallel()
		setupFakeLibraryError(t)

		var buf bytes.Buffer
		opts := scaffold.Options{}
		shouldAdd := prepareLibrarySlot(newCtx(&buf), &opts, "gh:acme/service-template@v1", false)

		assert.False(t, shouldAdd)
		assert.False(t, opts.SkipGeneratorCopy, "an unreadable library must not cost the project its generators")
		assert.Empty(t, opts.TemplateName)
		assert.Empty(t, buf.String(), "an unreadable library is a slog.Warn, not user-facing noise")
	})
}

// TestUT_ResolveLibrarySlot exercises the impure wrapper end to end against
// a real (isolated) library, so classifyLibrarySlot's rules are proven
// against what Library.Get actually returns, not just against hand-built
// errors.
func TestUT_ResolveLibrarySlot(t *testing.T) {
	t.Run("free when nothing occupies the derived name", func(t *testing.T) {
		// isolateLibrary mutates package-level var — do NOT use t.Parallel()
		isolateLibrary(t)
		const ref = "gh:acme/service-template@v1"

		slot, entry, libName := resolveLibrarySlot(ref)

		assert.Equal(t, slotFree, slot)
		assert.Nil(t, entry)
		assert.Equal(t, remote.LibraryName(ref), libName)
	})

	t.Run("same source when the occupying entry came from the identical ref", func(t *testing.T) {
		// setupFakeLibraryForRef mutates package-level var — do NOT use t.Parallel()
		const ref = "gh:acme/service-template@v1"
		setupFakeLibraryForRef(t, ref)

		slot, entry, libName := resolveLibrarySlot(ref)

		assert.Equal(t, slotSameSource, slot)
		require.NotNil(t, entry)
		assert.Equal(t, ref, entry.Source)
		assert.Equal(t, remote.LibraryName(ref), libName)
	})

	t.Run("taken by other when a different version occupies the same derived name", func(t *testing.T) {
		// setupFakeLibraryForRef mutates package-level var — do NOT use t.Parallel()
		const seededRef = "gh:acme/service-template@v1"
		setupFakeLibraryForRef(t, seededRef)

		const requestedRef = "gh:acme/service-template@v2"
		require.Equal(t, remote.LibraryName(seededRef), remote.LibraryName(requestedRef),
			"fixture invariant: version is deliberately excluded from the digest")

		slot, entry, _ := resolveLibrarySlot(requestedRef)

		assert.Equal(t, slotTakenByOther, slot)
		require.NotNil(t, entry)
		assert.Equal(t, seededRef, entry.Source)
	})

	t.Run("unavailable when the library cannot be initialized", func(t *testing.T) {
		// setupFakeLibraryError mutates package-level var — do NOT use t.Parallel()
		setupFakeLibraryError(t)

		slot, entry, _ := resolveLibrarySlot("gh:acme/service-template")

		assert.Equal(t, slotUnavailable, slot)
		assert.Nil(t, entry)
	})
}
