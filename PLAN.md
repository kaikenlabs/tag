# Template-Bundled Generators — Implementation Plan

## Summary

Enable project templates to ship generators alongside scaffolds. After scaffolding, users can run `tag generate <name>` to use generators defined in the original template — resolved from the library, not copied into the project. A new `.tagconfig.json` schema records the template origin and scaffold-time variables, enabling generators to access scaffold context.

## Design Decisions (Agreed)

| Decision | Choice |
|---|---|
| Generator storage | Resolve from library, not copied to project |
| `.tag/` in scaffold output | Excluded (no `CopyGenerators`) |
| Discovery order | Library template → project-local `.tag/` |
| `.tagconfig.json` | Redesigned: records template origin + scaffold variables |
| Cache miss | Error message, user runs `tag lib update` |
| Variable merging | Scaffold vars as base context, generator vars override |
| Generator listing | `tag generate list` as urfave/cli subcommand (not arg sniffing) |
| Name collisions | Local wins (documented) |
| Backward compatibility | Greenfield — no legacy support needed |
| Template convention | `.tag/` everywhere (replaces `_generators/`); keep `_generators/` skip for existing templates |
| Generator metadata | Generators support `tag.template.json` for description + typed variable prompting |
| Variable namespace | Scaffold vars injected into `vars.*`; generator vars override on collision |
| Auto-add to library | No. Show hint: `tag lib add <ref>` in post-scaffold summary |

## Current State Analysis

### What exists today
- **`.tagconfig.json`** (`config/config.go`): Currently stores `env.TAG_PATH`, `env.TAG_SHARED_PATH`, `env.TAG_BUNDLE_PATH`, and `hooks`. Used by `tag generate` and `tag init`.
- **`CopyGenerators()`** (`scaffold/output.go:277`): Copies `_generators/` from template into `.tag/` in the output project.
- **`GenerateTagConfig()`** (`scaffold/output.go:338`): Writes a `.tagconfig.json` with default env paths and empty hooks.
- **`tag lib`** (`commands/library.go` + `library/library.go`): Full library management (add, ls, rm, update, inspect, edit). Stores templates in `~/.local/share/tag/templates/`.
- **`tag run`** (`commands/run.go`): Scaffolds from library templates using `scaffold.Run()`.
- **`tag generate`** (`commands/generate.go`): Reads `.tagconfig.json` for `TAG_PATH`, discovers generators in `.tag/<name>/`, runs through `engine.Generate()`.
- **`_generators/`** (`types.GeneratorsDir`): Convention for generators inside a project template.

### What needs to change
1. **`.tagconfig.json` schema** — Add `template` (source ref), `version`, `variables` fields. Keep `env` and `hooks` for generate-time config.
2. **`GenerateTagConfig()`** — Accept template metadata and scaffold variables; write enriched config.
3. **`CopyGenerators()`** — Remove entirely. Generators stay in library.
4. **`tag generate`** — New resolution: read `.tagconfig.json` → resolve library template → find generator in template's `.tag/` → fall back to local.
5. **`tag generate list`** — New subcommand to discover generators from both sources.
6. **Output writer** — Skip `.tag/` directory in scaffold output (in addition to current `_generators/` skip).

---

## Implementation Phases

### Phase 1: Redesign `.tagconfig.json` Schema

**Files**: `internal/config/config.go`, `internal/types/constants.go`

**Changes**:
1. Add new fields to `Config` struct:
   ```go
   type Config struct {
       Template  *TemplateOrigin `json:"template,omitempty"`  // NEW
       Variables map[string]any  `json:"variables,omitempty"` // NEW
       Env       Env             `json:"env"`
       Hooks     Hooks           `json:"hooks"`
   }

   type TemplateOrigin struct {
       Source  string `json:"source"`            // e.g., "gh:acme/nextjs-starter"
       Name    string `json:"name"`              // library name, e.g., "nextjs-starter"
       Version string `json:"version,omitempty"` // from tag.template.json
   }
   ```
2. `LoadConfigFile()` already handles JSON unmarshalling — new fields will be populated automatically if present.
3. Add helper method: `Config.HasTemplateOrigin() bool`.

**Testing**: Unit tests for loading new schema format, loading legacy format (missing fields = nil/empty).

### Phase 2: Update Scaffold Output Pipeline

**Files**: `internal/scaffold/output.go`, `internal/scaffold/scaffold.go`, `internal/scaffold/types.go`

**Changes**:
1. **Remove `CopyGenerators()` call** from `Scaffold.Run()` (line 180).
2. **Update `GenerateTagConfig()`** signature to accept template metadata:
   ```go
   func GenerateTagConfig(outputDir string, opts TagConfigOptions) error

   type TagConfigOptions struct {
       TemplateSource  string         // original ref (e.g., "gh:user/repo")
       TemplateName    string         // library name
       TemplateVersion string         // from tag.template.json
       Variables       map[string]any // scaffold-time variable values
   }
   ```
3. **Skip `.tag/`** in `DefaultOutputWriter.Write()` — add a skip rule similar to the existing `_generators/` skip (output.go:78-83), but for `types.TemplatesDir` (`.tag`).
4. **Keep the `_generators/` skip** alongside the new `.tag/` skip. Existing templates in the wild still use `_generators/`. Both directories are excluded from scaffold output.
5. **Remove `CopyGenerators()` function** entirely.
6. Update `scaffold.Options` to carry template origin info:
   ```go
   // Already has TemplateRef — add:
   TemplateName    string // library name (set by tag run)
   TemplateVersion string // from tag.template.json (set after config load)
   ```

**Testing**: Unit tests verifying `.tag/` is excluded from output, `.tagconfig.json` contains template origin + variables.

### Phase 3: Update `tag scaffold` and `tag run` to Pass Template Origin

**Files**: `internal/commands/scaffold.go`, `internal/commands/run.go`, `internal/commands/flags.go`

**Changes**:
1. **`scaffoldAction()`**: After resolving template, pass `TemplateRef` as source. For `tag scaffold`, the library name is not known (user scaffolded directly), so `TemplateName` can be derived or left empty.
2. **`runAction()`**: Pass both `entry.Source` and `entry.Name` from the library entry.
3. **`Scaffold.Run()`**: After loading config (`loadAndValidateConfig`), capture `config.Version` into options. Pass to `GenerateTagConfig()`.
4. **`buildScaffoldOpts()`**: Add `TemplateName` field plumbing.

**Note**: When scaffolding via `tag scaffold gh:user/repo` (not from library), we should still record the source ref. The `TemplateName` will be empty — this means `tag generate` will need to resolve by source ref, not just library name. However, the simplest approach: **require templates to be in the library for generator resolution**. If scaffolded via `tag scaffold` directly, generators won't be available until user does `tag lib add`. This is the "conscious effort" principle.

**Alternative (recommended)**: When `tag scaffold` is used with a remote ref, auto-suggest `tag lib add` in the post-scaffold summary. This keeps the feature simple while guiding users.

### Phase 4: Redesign `tag generate` Resolution

**Files**: `internal/commands/generate.go`, `internal/config/config.go`

This is the core change. The generate command currently:
1. Reads `.tagconfig.json` → gets `TAG_PATH`
2. Joins `TAG_PATH/generatorName` → finds generator
3. Runs it

New flow:
1. Read `.tagconfig.json` → get `template.name` (library reference) + `env.TAG_PATH` (local generators)
2. **Try library first**: If `template.name` is set:
   a. Load library → get template path
   b. Look for `.tag/<generatorName>/` in library template
   c. If found → use it (also resolve `_shared/` and `_bundles/` from library template)
3. **Fall back to local**: Look in local `TAG_PATH/<generatorName>/`
4. **If not found in either**: Error with helpful message

**Changes to `generateAction()`**:
```go
func generateAction(c *cli.Context, cfg *config.Config) error {
    // ... existing validation ...

    // NEW: Determine generator source (library vs local)
    genDir, sharedDir, err := resolveGeneratorPaths(cfg, generatorOrBundleName)
    if err != nil {
        return err
    }

    // ... rest of existing logic using genDir, sharedDir ...
}

func resolveGeneratorPaths(cfg *config.Config, name string) (genDir, sharedDir string, err error) {
    // 1. Try library template
    if cfg.Template != nil && cfg.Template.Name != "" {
        lib, err := newLocalLibrary()
        if err != nil {
            return "", "", fmt.Errorf("failed to initialize library: %w", err)
        }

        templateDir, err := lib.TemplatePath(cfg.Template.Name)
        if err != nil {
            // Only fall through to local on ErrTemplateNotFound (cache miss).
            // All other errors (corruption, permissions, etc.) are returned immediately.
            if !errors.Is(err, library.ErrTemplateNotFound) {
                return "", "", fmt.Errorf("error accessing library template %q: %w", cfg.Template.Name, err)
            }
            // ErrTemplateNotFound: fall through to local resolution
        } else {
            candidate := filepath.Join(templateDir, types.TemplatesDir, name)
            if _, statErr := os.Stat(candidate); statErr == nil {
                shared := filepath.Join(templateDir, types.TemplatesDir, types.SharedDir)
                return candidate, shared, nil
            }
            // Generator not found in library template — fall through to local
        }
    }

    // 2. Fall back to local .tag/
    if cfg.Env.Path != "" {
        candidate := filepath.Join(cfg.Env.Path, name)
        if _, statErr := os.Stat(candidate); statErr == nil {
            shared := filepath.Join(cfg.Env.Path, types.SharedDir)
            return candidate, shared, nil
        }
    }

    // 3. Not found — use specific error types for nuanced handling
    if cfg.Template != nil && cfg.Template.Name != "" {
        return "", "", &ErrGeneratorNotFound{
            Generator: name,
            Template:  cfg.Template.Name,
            Source:    cfg.Template.Source,
        }
    }
    return "", "", &ErrGeneratorNotFound{Generator: name, LocalPath: cfg.Env.Path}
}
```

**Error types** — Add specific error types for generator resolution:
```go
// ErrGeneratorNotFound is returned when a generator cannot be found in any source.
type ErrGeneratorNotFound struct {
    Generator string
    Template  string // library template name (empty if no template)
    Source    string // template source ref (for helpful message)
    LocalPath string // local .tag/ path
}

func (e *ErrGeneratorNotFound) Error() string {
    if e.Template != "" {
        return fmt.Sprintf("generator %q not found in template %q or local path.\n"+
            "Ensure the template is in the library: tag lib add %s", e.Generator, e.Template, e.Source)
    }
    return fmt.Sprintf("generator %q not found in %s", e.Generator, e.LocalPath)
}

// ErrTemplateNotInLibrary is returned when .tagconfig.json references a template not in the library.
type ErrTemplateNotInLibrary struct {
    Name   string
    Source string
}

func (e *ErrTemplateNotInLibrary) Error() string {
    return fmt.Sprintf("template %q is not in the library. Run: tag lib add %s", e.Name, e.Source)
}
```

**Version mismatch warning** — When resolving from library, compare scaffold-time version with current library template version:
```go
// After successful library resolution, check for version drift
if cfg.Template.Version != "" {
    libVersion, _, _ := library.ReadTemplateMetadata(templateDir)
    if libVersion != "" && libVersion != cfg.Template.Version {
        fmt.Fprintf(os.Stderr, "Warning: template version mismatch (scaffolded: %s, library: %s). "+
            "Consider re-scaffolding or running 'tag lib update %s'.\n",
            cfg.Template.Version, libVersion, cfg.Template.Name)
    }
}
```

**Variable context merging** — When running from a library template, merge scaffold variables:
```go
// In generateTemplate / generateBundle, before calling gen.Generate():
if cfg.Variables != nil {
    // Scaffold variables available as base context
    data.ScaffoldVars = cfg.Variables
}
```

This requires extending `engine.Data` to carry scaffold variables, and the engine to inject them into the template context.

**Changes to `engine.Data`**:
```go
type Data struct {
    Name         string
    Args         string
    MetaArgs     []string
    ScaffoldVars map[string]any // NEW: variables from scaffold-time .tagconfig.json
}
```

**Changes to template context in engine**: When `ScaffoldVars` is present, inject as base layer that generator vars override.

**Variable namespace rules**:
- Scaffold vars are injected into the same `vars.*` namespace (e.g., `{{ vars.project_name }}`, `{{ vars.router }}`).
- Generator-defined variables override scaffold vars on name collision.
- Scaffold vars are available in both YAML frontmatter paths and template bodies.
- Template authors can use scaffold vars in conditionals: `{% if vars.use_docker %}...{% endif %}`.
- No separate `scaffold.*` namespace — keeping it simple and consistent with existing `vars.*` convention.

**Testing**: Unit tests for resolution priority (library > local), cache miss error, variable merging, collision override behavior.

### Phase 5: Bundle Resolution from Library

**Files**: `internal/commands/generate.go`

The `generateBundle()` function currently resolves bundle files and generator dirs relative to `cfg.Env.Path`. Update it to use the same resolution logic as `generateTemplate()`:
1. When running a bundle, resolve the bundle JSON from library first, then local.
2. When iterating generators in the bundle, **resolve each generator independently** via `resolveGeneratorPaths()`. This supports mixed-source bundles where some generators come from the library and others are local.
3. Shared templates (`_shared/`) resolve relative to the found generator's source root (library or local).

```go
func resolveBundlePath(cfg *config.Config, bundleName string) (string, error) {
    // Same library-first, local-fallback pattern as resolveGeneratorPaths
    // but for _bundles/<name>/<name>.json
}
```

**Testing**: Unit tests for library bundles, local bundles, mixed-source bundles.

### Phase 6: `tag generate list` Subcommand

**Files**: `internal/commands/generate.go`

**Changes**:
1. Use a proper urfave/cli subcommand (not argument sniffing, which would prevent having a generator named "list"):
   ```go
   func GenerateCommand(cfg *config.Config) *cli.Command {
       return &cli.Command{
           Name:  "generate",
           Usage: "Run a generator or bundle",
           Subcommands: []*cli.Command{
               generateListCommand(cfg),
           },
           // When no subcommand matches, fall through to generator execution
           Action: func(c *cli.Context) error {
               return generateAction(c, cfg)
           },
       }
   }

   func generateListCommand(cfg *config.Config) *cli.Command {
       return &cli.Command{
           Name:    "list",
           Aliases: []string{"ls"},
           Usage:   "List available generators and bundles",
           Action: func(c *cli.Context) error {
               return generateList(cfg)
           },
       }
   }
   ```

2. Implement `generateList()`:
   ```go
   func generateList(cfg *config.Config) error {
       // Collect generators from library template
       var templateGens []generatorInfo
       var templateName, templateSource, templateVersion string

       if cfg.Template != nil && cfg.Template.Name != "" {
           templateName = cfg.Template.Name
           templateSource = cfg.Template.Source
           templateVersion = cfg.Template.Version
           // ... resolve from library, scan .tag/ for subdirs
           // ... for each subdir, read tag.template.json for description
       }

       // Collect generators from local .tag/
       var localGens []generatorInfo
       if cfg.Env.Path != "" {
           // ... scan cfg.Env.Path for subdirs
           // ... each subdir without _ prefix = generator
       }

       // Collect bundles from both sources
       var templateBundles, localBundles []generatorInfo
       // ... scan _bundles/ dirs

       // Print formatted output
       // ...
   }
   ```

3. Read `description` from each generator's `tag.template.json` (if present). This ties into the generator metadata support — generators can have their own `tag.template.json` with `description` and typed variable definitions.

4. Output format:
   ```
   Generators for my-app (template: gh:acme/nextjs-starter@v1.2.0)

     TEMPLATE GENERATORS (nextjs-starter)
     component    Create a React component
     page         Create a new page/route
     api          Create an API endpoint

     PROJECT GENERATORS (.tag/)
     custom-hook  Custom React hook generator

     BUNDLES
     feature      Component + page + test (template)

   Run: tag generate <name> [target] [args]
   ```

**Testing**: Unit tests with mock library and filesystem.

### Phase 7: Update `tag init` (Simplified)

**Files**: `internal/commands/init.go`, `internal/config/config.go`

**Changes**:
Since this is greenfield, `tag init` should create a `.tagconfig.json` with the new schema:
```json
{
  "env": {
    "TAG_PATH": ".tag",
    "TAG_SHARED_PATH": "_shared",
    "TAG_BUNDLE_PATH": "_bundles"
  },
  "hooks": {
    "pre": [],
    "post": []
  }
}
```
No `template` or `variables` fields (those come from scaffolding). This is essentially unchanged.

The `CreateConfigFile()` function doesn't need changes — it already creates this structure.

### Phase 8: Documentation and Polish

**Files**: `docs/`, scaffold summary output

1. **Post-scaffold summary**: Update `displaySummary()` to show template origin and contextual hints:
   ```
   Scaffolding complete!
     Output: /path/to/my-app
     Project: my-app
     Template: gh:acme/nextjs-starter (v1.2.0)

   Next steps:
     cd my-app
     tag generate list    # see available generators
   ```
   - Only show `tag generate list` hint when the template has generators (`.tag/` exists in the library template).
   - When scaffolding via `tag scaffold` (not `tag run`), show: `"Add to library for generators: tag lib add <ref>"`

2. **Documentation**: Update `docs/commands/` for new `generate list` subcommand and `.tagconfig.json` schema.

3. **Clear documentation** on local-wins behavior for name collisions.

4. **Generator `tag.template.json` support**: Document that generators can include a `tag.template.json` at their root for metadata:
   ```json
   {
     "description": "Create a React component",
     "vars": {
       "name": { "type": "string", "prompt": "Component name" },
       "with_tests": { "type": "boolean", "default": true }
     }
   }
   ```
   This enables `tag generate list` to show descriptions, and future typed variable prompting for generators.

---

## Migration Notes

- **No backward compatibility needed** — this is greenfield.
- The existing `_generators/` convention in templates should be renamed to `.tag/` within template repos. Template authors place generators in `.tag/` inside their template.
- `tag init` continues to work for projects that only use local generators (no library template).

## File Change Summary

| File | Change Type | Description |
|---|---|---|
| `internal/config/config.go` | Modify | Add `TemplateOrigin`, `Variables` to Config |
| `internal/scaffold/output.go` | Modify | Remove `CopyGenerators`, skip `.tag/`, update `GenerateTagConfig` |
| `internal/scaffold/scaffold.go` | Modify | Remove `CopyGenerators` call, pass template metadata to `GenerateTagConfig` |
| `internal/scaffold/types.go` | Modify | Add `TemplateName`, `TemplateVersion` to Options |
| `internal/scaffold/errors.go` | Modify | Add `ErrGeneratorNotFound`, `ErrTemplateNotInLibrary` |
| `internal/commands/generate.go` | Major rewrite | New resolution logic, `list` subcommand, variable merging |
| `internal/commands/scaffold.go` | Modify | Pass template origin info, suggest `tag lib add` |
| `internal/commands/run.go` | Modify | Pass library entry name + source |
| `internal/commands/flags.go` | Minor | Plumb new Options fields |
| `internal/commands/init.go` | Minor | Verify config creation still correct |
| `internal/engine/types.go` | Modify | Add `ScaffoldVars` to Data |
| `internal/engine/engine.go` | Modify | Inject scaffold vars into template context |
| `internal/library/library.go` | Minor | Export `ReadTemplateMetadata` for version comparison |
| `internal/types/constants.go` | Verify | Ensure constants are correct |
| Test files | New/Modify | Tests for each phase |

## Resolved Questions (from Codex/Gemini Review)

1. ~~Should `tag scaffold` auto-add to library?~~ **No.** Show hint in post-scaffold summary.
2. ~~Generator `tag.template.json`?~~ **Yes.** Generators support `tag.template.json` for description + typed vars. High value for `tag generate list`.
3. ~~`.tag/` replace `_generators/`?~~ **Yes.** Use `.tag/` everywhere. Keep `_generators/` skip in output writer for existing templates in the wild.
4. ~~`list` detection fragile?~~ **Fixed.** Use urfave/cli subcommand, not argument sniffing.
5. ~~Variable namespace?~~ **Defined.** Scaffold vars in `vars.*`, generator vars override, available in frontmatter + bodies.
6. ~~Library errors swallowed?~~ **Fixed.** Only fall through on `ErrTemplateNotFound`. All other errors returned immediately.
7. ~~Version mismatch?~~ **Added.** Warn when library template version differs from scaffold-time version.
8. ~~Mixed-source bundles?~~ **Handled.** Each generator in a bundle resolved independently via `resolveGeneratorPaths()`.
