package commands

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/pkg/app"
)

// GeneratorNotFoundError is returned when a generator cannot be found in any source.
type GeneratorNotFoundError struct {
	Generator string
	Template  string // library template name (empty if no template)
	Source    string // template source ref (for helpful message)
	LocalPath string // local .tag/ path
}

func (e *GeneratorNotFoundError) Error() string {
	if e.Template != "" {
		return fmt.Sprintf("generator %q not found in template %q or local path.\n"+
			"Ensure the template is in the library: tag lib add %s", e.Generator, e.Template, e.Source)
	}
	return fmt.Sprintf("generator %q not found in %s", e.Generator, e.LocalPath)
}

// resolveGeneratorPaths resolves the generator directory and shared path using
// library-first, local-fallback resolution. When a .tagconfig.json references
// a library template, generators from that template are preferred.
func resolveGeneratorPaths(cfg *config.Config, name string) (genDir, sharedDir string, err error) {
	// 1. Try library template
	if cfg.HasTemplateOrigin() {
		genDir, sharedDir, found, libErr := resolveFromLibrary(cfg, name)
		if libErr != nil {
			return "", "", libErr
		}
		if found {
			return genDir, sharedDir, nil
		}
	}

	// 2. Fall back to local .tag/
	if cfg.Env.Path != "" {
		candidate := filepath.Join(cfg.Env.Path, name)
		if _, statErr := os.Stat(candidate); statErr == nil {
			sharedName := cfg.Env.SharedPath
			if sharedName == "" {
				sharedName = types.SharedDir
			}
			shared := filepath.Join(cfg.Env.Path, sharedName)
			return candidate, shared, nil
		}
	}

	// 3. Not found
	if cfg.HasTemplateOrigin() {
		return "", "", app.NotFoundErrorf("%w", &GeneratorNotFoundError{
			Generator: name,
			Template:  cfg.Template.Name,
			Source:    cfg.Template.Source,
		})
	}
	return "", "", app.NotFoundErrorf("%w", &GeneratorNotFoundError{
		Generator: name,
		LocalPath: cfg.Env.Path,
	})
}

// resolveFromLibrary attempts to find a generator in the library template.
// Returns (genDir, sharedDir, found, error). When found is false and error is nil,
// the caller should fall through to local resolution.
func resolveFromLibrary(cfg *config.Config, name string) (string, string, bool, error) {
	lib, err := newLocalLibrary()
	if err != nil {
		return "", "", false, app.Errorf("failed to initialize library: %w", err)
	}

	templateDir, err := lib.TemplatePath(cfg.Template.Name)
	if err != nil {
		// Only fall through on ErrTemplateNotFound (cache miss).
		if !errors.Is(err, library.ErrTemplateNotFound) {
			return "", "", false, app.Errorf("error accessing library template %q: %w", cfg.Template.Name, err)
		}
		slog.Debug("template not found in library, falling back to local", "template", cfg.Template.Name)
		return "", "", false, nil
	}

	candidate := filepath.Join(templateDir, types.TemplatesDir, name)
	if _, statErr := os.Stat(candidate); statErr == nil {
		shared := filepath.Join(templateDir, types.TemplatesDir, types.SharedDir)
		warnVersionMismatch(cfg, templateDir)
		return candidate, shared, true, nil
	}

	// Generator not found in library template — fall through to local
	return "", "", false, nil
}

// warnVersionMismatch prints a warning if the library template version differs
// from the scaffold-time version recorded in .tagconfig.json.
func warnVersionMismatch(cfg *config.Config, templateDir string) {
	if cfg.Template.Version == "" {
		return
	}
	libVersion, _, _ := library.ReadTemplateMetadata(templateDir)
	if libVersion != "" && libVersion != cfg.Template.Version {
		fmt.Fprintf(os.Stderr, "Warning: template version mismatch (scaffolded: %s, library: %s). "+
			"Consider re-scaffolding or running 'tag lib update %s'.\n",
			cfg.Template.Version, libVersion, cfg.Template.Name)
	}
}

// resolveBundlePath resolves the bundle JSON file path using library-first, local-fallback resolution.
func resolveBundlePath(cfg *config.Config, bundleName, bundleSubDir string) (string, error) {
	bundleFile := filepath.Join(bundleName, bundleName+types.BundleExtension)

	// 1. Try library template
	if cfg.HasTemplateOrigin() {
		path, err := resolveBundleFromLibrary(cfg, bundleFile)
		if err != nil {
			return "", err
		}
		if path != "" {
			return path, nil
		}
	}

	// 2. Fall back to local
	if cfg.Env.Path != "" {
		candidate := filepath.Join(cfg.Env.Path, bundleSubDir, bundleFile)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
	}

	return "", app.NotFoundErrorf("cannot open bundle file: bundle %q not found", bundleName)
}

// resolveBundleFromLibrary attempts to find a bundle file in the library template.
// Returns ("", nil) when the bundle is not found (caller should fall through to local).
func resolveBundleFromLibrary(cfg *config.Config, bundleFile string) (string, error) {
	lib, err := newLocalLibrary()
	if err != nil {
		return "", app.Errorf("failed to initialize library: %w", err)
	}

	templateDir, err := lib.TemplatePath(cfg.Template.Name)
	if err != nil {
		if !errors.Is(err, library.ErrTemplateNotFound) {
			return "", app.Errorf("error accessing library template %q: %w", cfg.Template.Name, err)
		}
		return "", nil
	}

	candidate := filepath.Join(templateDir, types.TemplatesDir, types.BundlesDir, bundleFile)
	if _, statErr := os.Stat(candidate); statErr == nil {
		return candidate, nil
	}
	return "", nil
}

// generateTarget holds the resolved paths for a generator or bundle.
type generateTarget struct {
	IsBundle   bool
	GenDir     string // generator directory (when !IsBundle)
	SharedDir  string // shared templates dir (when !IsBundle)
	BundlePath string // bundle JSON path (when IsBundle)
}

// resolveGenerateTarget tries to resolve a name as a generator first, then as
// a bundle. If both exist, the generator wins and an info log is emitted. If
// neither exists, the generator error is returned (more common case).
func resolveGenerateTarget(cfg *config.Config, name, bundleSubDir string) (*generateTarget, error) {
	genDir, sharedDir, genErr := resolveGeneratorPaths(cfg, name)
	bundlePath, bundleErr := resolveBundlePath(cfg, name, bundleSubDir)

	genFound := genErr == nil
	bundleFound := bundleErr == nil

	switch {
	case genFound && bundleFound:
		slog.Info("found both generator and bundle, using generator", "name", name)
		return &generateTarget{GenDir: genDir, SharedDir: sharedDir}, nil
	case genFound:
		return &generateTarget{GenDir: genDir, SharedDir: sharedDir}, nil
	case bundleFound:
		return &generateTarget{IsBundle: true, BundlePath: bundlePath}, nil
	default:
		// Return the generator error — it's the more common case and has
		// better diagnostics (GeneratorNotFoundError).
		return nil, genErr
	}
}
