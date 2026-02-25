package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/types"
)

// newWithDir creates a Library with explicit dependencies (for testing).
func newWithDir(dataDir string, resolver Resolver) *Library {
	return &Library{
		dataDir:  dataDir,
		resolver: resolver,
	}
}

// localResolver is a test resolver that returns local paths as-is.
type localResolver struct{}

func (r *localResolver) Resolve(_ context.Context, input string, _ remote.ResolveOptions) (string, error) {
	// For local paths, just verify they exist and return them
	if _, err := os.Stat(input); err != nil {
		return "", fmt.Errorf("local path not found: %w", err)
	}
	return input, nil
}

// failResolver always returns an error.
type failResolver struct{ err error }

func (r *failResolver) Resolve(_ context.Context, _ string, _ remote.ResolveOptions) (string, error) {
	return "", r.err
}

func TestUT_DeriveName(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{"github shorthand", "gh:user/go-api", "go-api"},
		{"gitlab shorthand", "gl:org/mytemplate", "mytemplate"},
		{"bitbucket shorthand", "bb:team/project", "project"},
		{"strips cookiecutter prefix", "gh:user/cookiecutter-django", "django"},
		{"local path", "/tmp/my-template", "my-template"},
		{"local relative", "./my-template", "my-template"},
		{"strips .git suffix", "https://github.com/user/repo.git", "repo"},
		{"github HTTPS URL", "https://github.com/user/go-api", "go-api"},
		{"github HTTPS with .git", "https://github.com/user/cookiecutter-django.git", "django"},
		{"ssh URL", "git@github.com:user/my-project.git", "my-project"},
		{"bare directory name", "my-template", "my-template"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := remote.DeriveName(tt.ref)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUT_ValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "go-api", false},
		{"valid with dots", "my.template", false},
		{"dot prefix rejected", ".hidden", true},
		{"dot prefix .git", ".git", true},
		{"valid with spaces", "my template", false},
		{"empty", "", true},
		{"dot", ".", true},
		{"dotdot", "..", true},
		{"slash", "a/b", true},
		{"backslash", "a\\b", true},
		{"null byte", "a\x00b", true},
		{"newline", "a\nb", true},
		{"tab", "a\tb", true},
		{"delete char", "a\x7Fb", true},
		{"max length", strings.Repeat("a", 255), false},
		{"exceeds max length", strings.Repeat("a", 256), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidName)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUT_ValidateName_ControlChars(t *testing.T) {
	// Verify all control characters (0x00-0x1F, 0x7F) are rejected
	for c := range rune(0x20) {
		name := fmt.Sprintf("name%c", c)
		err := validateName(name)
		assert.ErrorIs(t, err, ErrInvalidName, "control char 0x%02X should be rejected", c)
	}
	err := validateName("name\x7F")
	assert.ErrorIs(t, err, ErrInvalidName, "DEL (0x7F) should be rejected")
}

// createTagTemplate creates a minimal TAG template directory.
func createTagTemplate(t *testing.T, dir, name, desc, version string) {
	t.Helper()

	config := map[string]any{
		"name":        name,
		"description": desc,
		"version":     version,
		"vars": map[string]any{
			"project_name": "my-project",
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), data, 0o644))

	// Create a sample file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Template"), 0o644))
}

// createCookiecutterTemplate creates a minimal Cookiecutter template directory.
func createCookiecutterTemplate(t *testing.T, dir string) {
	t.Helper()

	config := map[string]any{
		"project_name": "my-project",
		"author":       "Test Author",
		"_private_var": "internal",
	}

	data, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "cookiecutter.json"), data, 0o644))

	// Create the template directory (Cookiecutter convention)
	templateDir := filepath.Join(dir, "{{cookiecutter.project_name}}")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "README.md"), []byte("# {{cookiecutter.project_name}}"), 0o644))
}

func TestUT_Add_InvalidName_ExplicitEmpty(t *testing.T) {
	// When Name is empty but Ref is provided, deriveName fills in a name.
	// To test the ErrInvalidName guard, we need validateName to reject.
	// Use a name that validateName rejects: "." and ".."
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "test", "", "")

	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "."})
	assert.ErrorIs(t, err, ErrInvalidName)

	_, err = lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: ".."})
	assert.ErrorIs(t, err, ErrInvalidName)
}

func TestUT_Add_InvalidName_PathTraversal(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "test", "", "")

	lib := newWithDir(dataDir, &localResolver{})

	tests := []struct {
		name string
	}{
		{"../escape"},
		{"a/b"},
		{"a\\b"},
		{"."},
		{".."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: tt.name})
			assert.ErrorIs(t, err, ErrInvalidName)
		})
	}
}

func TestUT_Add_NonExistentRef(t *testing.T) {
	dataDir := t.TempDir()
	lib := newWithDir(dataDir, &failResolver{err: errors.New("not found")})

	_, err := lib.Add(context.Background(), AddOptions{
		Ref:  "/nonexistent/path/that/does/not/exist",
		Name: "test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resolve template")
}

func TestUT_Add_AutoDeriveName(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "test", "", "")

	lib := newWithDir(dataDir, &localResolver{})

	// Name is empty, should be derived from ref (filepath.Base of the temp dir)
	result, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc})
	require.NoError(t, err)

	// The derived name will be the base of the temp dir path
	assert.NotEmpty(t, result.Name)
	assert.Equal(t, filepath.Base(templateSrc), result.Name)
}

func TestUT_Add_ForcePreservesAddedAt(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "v1", "1.0.0")

	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api"})
	require.NoError(t, err)

	// Get the original AddedAt
	entry1, err := lib.Get("go-api")
	require.NoError(t, err)
	originalAddedAt := entry1.AddedAt

	// Force update
	createTagTemplate(t, templateSrc, "go-api", "v2", "2.0.0")
	_, err = lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api", Force: true})
	require.NoError(t, err)

	// AddedAt should be preserved
	entry2, err := lib.Get("go-api")
	require.NoError(t, err)
	assert.Equal(t, originalAddedAt, entry2.AddedAt)
	assert.NotEqual(t, entry1.UpdatedAt, entry2.UpdatedAt) // UpdatedAt should change
}

func TestUT_Add_LocalTagTemplate(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "A Go API template", "1.0.0")

	lib := newWithDir(dataDir, &localResolver{})

	result, err := lib.Add(context.Background(), AddOptions{
		Ref:  templateSrc,
		Name: "go-api",
	})
	require.NoError(t, err)

	assert.Equal(t, "go-api", result.Name)
	assert.Equal(t, templateSrc, result.Source)
	assert.Empty(t, result.ConvertedFrom)
	assert.False(t, result.IsUpdate)

	// Verify template was copied
	assert.FileExists(t, filepath.Join(result.TemplateDir, "tag.template.json"))
	assert.FileExists(t, filepath.Join(result.TemplateDir, "README.md"))

	// Verify registry
	entry, err := lib.Get("go-api")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", entry.Version)
	assert.Equal(t, "A Go API template", entry.Description)
}

func TestUT_Add_Duplicate_Errors(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "test", "1.0.0")

	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api"})
	require.NoError(t, err)

	_, err = lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api"})
	assert.ErrorIs(t, err, ErrTemplateExists)
}

func TestUT_Add_Duplicate_WithForce(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "test", "1.0.0")

	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api"})
	require.NoError(t, err)

	result, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api", Force: true})
	require.NoError(t, err)
	assert.True(t, result.IsUpdate)
}

func TestUT_Add_CookiecutterTemplate(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createCookiecutterTemplate(t, templateSrc)

	lib := newWithDir(dataDir, &localResolver{})

	result, err := lib.Add(context.Background(), AddOptions{
		Ref:  templateSrc,
		Name: "django",
	})
	require.NoError(t, err)

	assert.Equal(t, "cookiecutter", result.ConvertedFrom)
	assert.FileExists(t, filepath.Join(result.TemplateDir, "tag.template.json"))
}

func TestUT_Remove_Existing(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "test", "1.0.0")

	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api"})
	require.NoError(t, err)

	err = lib.Remove("go-api")
	require.NoError(t, err)

	// Verify removed
	_, err = lib.Get("go-api")
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestUT_Remove_Missing(t *testing.T) {
	dataDir := t.TempDir()
	lib := newWithDir(dataDir, &localResolver{})

	err := lib.Remove("nonexistent")
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestUT_List_Empty(t *testing.T) {
	dataDir := t.TempDir()
	lib := newWithDir(dataDir, &localResolver{})

	entries, err := lib.List()
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestUT_List_Sorted(t *testing.T) {
	dataDir := t.TempDir()
	lib := newWithDir(dataDir, &localResolver{})

	// Add in reverse order
	for _, name := range []string{"zebra", "alpha", "middle"} {
		src := t.TempDir()
		createTagTemplate(t, src, name, "", "")
		_, err := lib.Add(context.Background(), AddOptions{Ref: src, Name: name})
		require.NoError(t, err)
	}

	entries, err := lib.List()
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, "alpha", entries[0].Name)
	assert.Equal(t, "middle", entries[1].Name)
	assert.Equal(t, "zebra", entries[2].Name)
}

func TestUT_Get_Existing(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "A Go API", "2.0.0")

	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api"})
	require.NoError(t, err)

	entry, err := lib.Get("go-api")
	require.NoError(t, err)
	assert.Equal(t, "go-api", entry.Name)
	assert.Equal(t, "2.0.0", entry.Version)
	assert.Equal(t, "A Go API", entry.Description)
}

func TestUT_Get_Missing(t *testing.T) {
	dataDir := t.TempDir()
	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Get("nonexistent")
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestUT_TemplatePath(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "", "")

	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api"})
	require.NoError(t, err)

	path, err := lib.TemplatePath("go-api")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dataDir, "templates", "go-api"), path)
	assert.DirExists(t, path)
}

func TestUT_TemplatePath_Missing(t *testing.T) {
	dataDir := t.TempDir()
	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.TemplatePath("nonexistent")
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestUT_Registry_Persistence(t *testing.T) {
	dataDir := t.TempDir()

	// Add with one library instance
	lib1 := newWithDir(dataDir, &localResolver{})
	src := t.TempDir()
	createTagTemplate(t, src, "go-api", "test", "1.0.0")

	_, err := lib1.Add(context.Background(), AddOptions{Ref: src, Name: "go-api"})
	require.NoError(t, err)

	// Read with a new library instance (simulates new session)
	lib2 := newWithDir(dataDir, &localResolver{})
	entry, err := lib2.Get("go-api")
	require.NoError(t, err)
	assert.Equal(t, "go-api", entry.Name)
}

func TestUT_Remove_DeletesFilesFromDisk(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "test", "1.0.0")

	lib := newWithDir(dataDir, &localResolver{})

	result, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api"})
	require.NoError(t, err)

	// Verify files exist
	assert.DirExists(t, result.TemplateDir)
	assert.FileExists(t, filepath.Join(result.TemplateDir, "tag.template.json"))

	err = lib.Remove("go-api")
	require.NoError(t, err)

	// Verify directory is gone
	_, err = os.Stat(result.TemplateDir)
	assert.True(t, os.IsNotExist(err), "template directory should be deleted from disk")
}

func TestUT_Update_NonExistent(t *testing.T) {
	dataDir := t.TempDir()
	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Update(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestUT_UpdateAll_EmptyLibrary(t *testing.T) {
	dataDir := t.TempDir()
	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.UpdateAll(context.Background())
	assert.ErrorIs(t, err, ErrEmptyLibrary)
}

func TestUT_UpdateAll_MultipleTemplates(t *testing.T) {
	dataDir := t.TempDir()
	lib := newWithDir(dataDir, &localResolver{})

	for _, name := range []string{"alpha", "beta"} {
		src := t.TempDir()
		createTagTemplate(t, src, name, name+" desc", "1.0.0")
		_, err := lib.Add(context.Background(), AddOptions{Ref: src, Name: name})
		require.NoError(t, err)
	}

	results, err := lib.UpdateAll(context.Background())
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestUT_TemplatePath_DiskMissing(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "test", "1.0.0")

	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api"})
	require.NoError(t, err)

	// Manually delete the template directory (simulate disk corruption)
	templateDir := filepath.Join(dataDir, "templates", "go-api")
	require.NoError(t, os.RemoveAll(templateDir))

	_, err = lib.TemplatePath("go-api")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestUT_Add_TemplateWithNoConfig(t *testing.T) {
	// Template dir exists but has no tag.template.json — metadata should be empty, not error
	dataDir := t.TempDir()
	templateSrc := t.TempDir()

	// Only create a README, no tag.template.json
	require.NoError(t, os.WriteFile(filepath.Join(templateSrc, "README.md"), []byte("# Hello"), 0o644))

	lib := newWithDir(dataDir, &localResolver{})

	result, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "bare"})
	require.NoError(t, err)
	assert.Equal(t, "bare", result.Name)

	entry, err := lib.Get("bare")
	require.NoError(t, err)
	assert.Empty(t, entry.Version)
	assert.Empty(t, entry.Description)
}

func TestUT_Remove_InvalidName(t *testing.T) {
	dataDir := t.TempDir()
	lib := newWithDir(dataDir, &localResolver{})

	assert.ErrorIs(t, lib.Remove(""), ErrInvalidName)
	assert.ErrorIs(t, lib.Remove(".."), ErrInvalidName)
	assert.ErrorIs(t, lib.Remove("a/b"), ErrInvalidName)
	assert.ErrorIs(t, lib.Remove("a\x00b"), ErrInvalidName)
}

func TestUT_Get_InvalidName(t *testing.T) {
	dataDir := t.TempDir()
	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Get("")
	assert.ErrorIs(t, err, ErrInvalidName)
	_, err = lib.Get("..")
	assert.ErrorIs(t, err, ErrInvalidName)
	_, err = lib.Get("a/b")
	assert.ErrorIs(t, err, ErrInvalidName)
}

func TestUT_Add_NilResolver(t *testing.T) {
	dataDir := t.TempDir()
	lib := newWithDir(dataDir, nil)

	_, err := lib.Add(context.Background(), AddOptions{Ref: "/some/path", Name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resolver not configured")
}

func TestUT_Add_WithResolvedDir(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "A Go API template", "1.0.0")

	// Use nil resolver — ResolvedDir should bypass resolution entirely
	lib := NewLocal(dataDir)

	result, err := lib.Add(context.Background(), AddOptions{
		Ref:         "gh:user/go-api",
		Name:        "go-api",
		Force:       true,
		ResolvedDir: templateSrc,
	})
	require.NoError(t, err)

	assert.Equal(t, "go-api", result.Name)
	assert.Equal(t, "gh:user/go-api", result.Source)
	assert.False(t, result.IsUpdate)

	// Verify template was copied
	assert.FileExists(t, filepath.Join(result.TemplateDir, "tag.template.json"))
	assert.FileExists(t, filepath.Join(result.TemplateDir, "README.md"))

	// Verify registry entry
	entry, err := lib.Get("go-api")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", entry.Version)
	assert.Equal(t, "A Go API template", entry.Description)
	assert.Equal(t, "gh:user/go-api", entry.Source)
}

func TestUT_SecureRemoveAll_RegularDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mydir")
	require.NoError(t, os.MkdirAll(filepath.Join(target, "sub"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(target, "sub", "file.txt"), []byte("data"), 0o600))

	require.NoError(t, secureRemoveAll(target))
	_, err := os.Stat(target)
	assert.True(t, os.IsNotExist(err))
}

func TestUT_SecureRemoveAll_Symlink(t *testing.T) {
	dir := t.TempDir()

	// Create a real directory that should NOT be deleted
	realDir := filepath.Join(dir, "real")
	require.NoError(t, os.MkdirAll(realDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(realDir, "precious.txt"), []byte("keep"), 0o600))

	// Create a symlink pointing to realDir
	linkPath := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(realDir, linkPath))

	// secureRemoveAll should remove only the link, not the target
	require.NoError(t, secureRemoveAll(linkPath))

	// Link should be gone
	_, err := os.Lstat(linkPath)
	assert.True(t, os.IsNotExist(err))

	// Real directory and its contents should still exist
	assert.DirExists(t, realDir)
	assert.FileExists(t, filepath.Join(realDir, "precious.txt"))
}

func TestUT_SecureRemoveAll_NonExistent(t *testing.T) {
	assert.NoError(t, secureRemoveAll("/nonexistent/path/that/does/not/exist"))
}

func TestUT_TemplatePath_RejectsSymlink(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "test", "1.0.0")

	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api"})
	require.NoError(t, err)

	templateDir := filepath.Join(dataDir, "templates", "go-api")

	// Replace the template dir with a symlink
	decoyDir := t.TempDir()
	require.NoError(t, os.RemoveAll(templateDir))
	require.NoError(t, os.Symlink(decoyDir, templateDir))

	_, err = lib.TemplatePath("go-api")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTemplateNotFound)
	assert.Contains(t, err.Error(), "symlink")
}

func TestUT_CopyDir_PrivatePermissions(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "sub", "file.txt"), []byte("data"), 0o644))

	dst := filepath.Join(t.TempDir(), "out")
	require.NoError(t, fileutil.CopyDir(src, dst, types.DirModePrivate))

	// Verify directories have private permissions (0700)
	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm(), "root dir should be 0700")

	info, err = os.Stat(filepath.Join(dst, "sub"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm(), "sub dir should be 0700")
}

func TestUT_CopyDir_SkipsSymlinks(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "real.txt"), []byte("data"), 0o644))

	// Create a file symlink
	require.NoError(t, os.Symlink(filepath.Join(src, "real.txt"), filepath.Join(src, "link.txt")))

	// Create a directory symlink
	subDir := filepath.Join(src, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.Symlink(subDir, filepath.Join(src, "linkdir")))

	dst := filepath.Join(t.TempDir(), "out")
	require.NoError(t, fileutil.CopyDir(src, dst, types.DirModePrivate))

	// Real file should be copied
	assert.FileExists(t, filepath.Join(dst, "real.txt"))

	// Symlinks should NOT be copied
	_, err := os.Lstat(filepath.Join(dst, "link.txt"))
	assert.True(t, os.IsNotExist(err), "file symlink should not be copied")

	_, err = os.Lstat(filepath.Join(dst, "linkdir"))
	assert.True(t, os.IsNotExist(err), "directory symlink should not be copied")
}

func TestUT_Registry_RejectsNewerVersion(t *testing.T) {
	dataDir := t.TempDir()

	// Write a registry with a future version
	reg := &Registry{
		Version: registryVersion + 1,
		Entries: map[string]*Entry{},
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "library.json"), data, 0o600))

	_, err = loadRegistry(dataDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "newer than supported")
}

func TestUT_Registry_AcceptsCurrentVersion(t *testing.T) {
	dataDir := t.TempDir()

	reg := &Registry{
		Version: registryVersion,
		Entries: map[string]*Entry{},
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "library.json"), data, 0o600))

	loaded, err := loadRegistry(dataDir)
	require.NoError(t, err)
	assert.Equal(t, registryVersion, loaded.Version)
}

func TestUT_StoreTemplateFresh_InconsistentState(t *testing.T) {
	// When destPath already exists on disk but registry says "not exists",
	// storeTemplateFresh should route to atomic path (not blindly overwrite).
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "test", "1.0.0")

	// Pre-create the destination directory (simulating inconsistent state)
	destPath := filepath.Join(dataDir, "templates", "go-api")
	require.NoError(t, os.MkdirAll(destPath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(destPath, "existing.txt"), []byte("pre-existing"), 0o600))

	lib := newWithDir(dataDir, &localResolver{})

	// Add should succeed (routes through atomic path due to existing dir)
	result, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api"})
	require.NoError(t, err)
	assert.Equal(t, "go-api", result.Name)

	// New template should be in place
	assert.FileExists(t, filepath.Join(result.TemplateDir, "tag.template.json"))
}

func TestUT_ThreeStepAtomicSwap(t *testing.T) {
	// Verify that the three-step atomic swap preserves data on update
	dataDir := t.TempDir()
	lib := newWithDir(dataDir, &localResolver{})

	// Add initial template
	src1 := t.TempDir()
	createTagTemplate(t, src1, "go-api", "v1", "1.0.0")
	require.NoError(t, os.WriteFile(filepath.Join(src1, "v1-marker.txt"), []byte("v1"), 0o644))

	_, err := lib.Add(context.Background(), AddOptions{Ref: src1, Name: "go-api"})
	require.NoError(t, err)

	// Update with new template
	src2 := t.TempDir()
	createTagTemplate(t, src2, "go-api", "v2", "2.0.0")
	require.NoError(t, os.WriteFile(filepath.Join(src2, "v2-marker.txt"), []byte("v2"), 0o644))

	result, err := lib.Add(context.Background(), AddOptions{Ref: src2, Name: "go-api", Force: true})
	require.NoError(t, err)
	assert.True(t, result.IsUpdate)

	// v2 marker should exist, v1 marker should not
	assert.FileExists(t, filepath.Join(result.TemplateDir, "v2-marker.txt"))
	_, err = os.Stat(filepath.Join(result.TemplateDir, "v1-marker.txt"))
	assert.True(t, os.IsNotExist(err), "v1 marker should be gone after update")

	// No .old or .new directories should remain
	_, err = os.Stat(result.TemplateDir + ".old")
	assert.True(t, os.IsNotExist(err), ".old backup should be cleaned up")
	_, err = os.Stat(result.TemplateDir + ".new")
	assert.True(t, os.IsNotExist(err), ".new temp should be cleaned up")
}

func TestUT_Remove_SymlinkDir(t *testing.T) {
	// Verify that Remove with a symlink-replaced template dir
	// only removes the symlink, not the target
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "test", "1.0.0")

	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api"})
	require.NoError(t, err)

	templateDir := filepath.Join(dataDir, "templates", "go-api")

	// Replace with symlink to a decoy directory
	decoyDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(decoyDir, "decoy.txt"), []byte("safe"), 0o600))
	require.NoError(t, os.RemoveAll(templateDir))
	require.NoError(t, os.Symlink(decoyDir, templateDir))

	err = lib.Remove("go-api")
	require.NoError(t, err)

	// The symlink should be gone
	_, err = os.Lstat(templateDir)
	assert.True(t, os.IsNotExist(err))

	// The decoy directory should still exist with its contents
	assert.DirExists(t, decoyDir)
	assert.FileExists(t, filepath.Join(decoyDir, "decoy.txt"))
}

// selectiveFailResolver fails for specific refs.
type selectiveFailResolver struct {
	failRefs map[string]bool
}

func (r *selectiveFailResolver) Resolve(_ context.Context, input string, _ remote.ResolveOptions) (string, error) {
	if r.failRefs[input] {
		return "", fmt.Errorf("simulated failure for %s", input)
	}
	if _, err := os.Stat(input); err != nil {
		return "", fmt.Errorf("local path not found: %w", err)
	}
	return input, nil
}

func TestUT_UpdateAll_PartialFailure(t *testing.T) {
	dataDir := t.TempDir()

	// Create two templates with different source directories
	src1 := t.TempDir()
	createTagTemplate(t, src1, "alpha", "alpha desc", "1.0.0")

	src2 := t.TempDir()
	createTagTemplate(t, src2, "beta", "beta desc", "1.0.0")

	// Use a normal resolver for the initial add
	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Add(context.Background(), AddOptions{Ref: src1, Name: "alpha"})
	require.NoError(t, err)
	_, err = lib.Add(context.Background(), AddOptions{Ref: src2, Name: "beta"})
	require.NoError(t, err)

	// Now use a selective-fail resolver that fails for src2
	failLib := newWithDir(dataDir, &selectiveFailResolver{
		failRefs: map[string]bool{src2: true},
	})

	results, err := failLib.UpdateAll(context.Background())

	// Should have partial results (alpha succeeded)
	assert.Len(t, results, 1)
	assert.Equal(t, "alpha", results[0].Name)

	// Should have an error (beta failed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "beta")
}

func TestUT_ValidateName_DotPrefix(t *testing.T) {
	// Dot-prefixed names are rejected to prevent hidden directories
	for _, name := range []string{".hidden", ".git", ".ssh", ".config"} {
		err := validateName(name)
		assert.ErrorIs(t, err, ErrInvalidName, "dot-prefix name %q should be rejected", name)
	}
}

func TestUT_Update_RefetchesFromSource(t *testing.T) {
	dataDir := t.TempDir()
	templateSrc := t.TempDir()
	createTagTemplate(t, templateSrc, "go-api", "v1 desc", "1.0.0")

	lib := newWithDir(dataDir, &localResolver{})

	_, err := lib.Add(context.Background(), AddOptions{Ref: templateSrc, Name: "go-api"})
	require.NoError(t, err)

	// Modify source
	createTagTemplate(t, templateSrc, "go-api", "v2 desc", "2.0.0")

	result, err := lib.Update(context.Background(), "go-api")
	require.NoError(t, err)
	assert.True(t, result.IsUpdate)

	// Verify updated metadata
	entry, err := lib.Get("go-api")
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", entry.Version)
	assert.Equal(t, "v2 desc", entry.Description)
}
