# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `tag template lint`, `tag template variables` and `tag template rename-var` now recognise literal **subscript** variable access — `{{ vars["name"] }}` and `{{ vars['name'] }}` (whitespace-tolerant) — in addition to dot access. Subscript is the only way to reach a variable whose name collides with a Gonja keyword (e.g. `param.in`). Non-literal subscripts (`vars[expr]`) remain out of scope, since the key is not statically known (#339)

### Changed

- **Potentially breaking**: because subscript access is now recognised (#339), templates that reference variables only through `vars["name"]` will produce findings they did not before — `tag template lint` can newly report an undeclared variable, and `tag template variables --strict` can flip a variable from unused to used. As with #337 this can change CI verdicts on templates whose source did not otherwise change; re-run once after upgrading. `tag template rename-var` now also rewrites subscript references, fixing a case where it renamed a declaration but left a live `vars["old"]` reference behind, silently corrupting the template (#339)


- **Potentially breaking**: `tag template lint` and `tag template variables` now share the same quote-aware variable scanner as `tag template rename-var`, so all three commands agree on what counts as a `vars.*` reference. Because this scanner finds real references the old regex missed and stops treating string-literal lookalikes as references, both `tag template lint` and `tag template variables --strict` can change verdict — in either direction — on templates whose source did not otherwise change. Downstream template repos relying on either command in CI should re-run once after upgrading (#337)

### Fixed

- The `pascal`, `camel` and `title` filters (and the `n.pascal_case`/`n.camel_case` namespace) could panic when template rendering happened concurrently, e.g. under `tag test --parallel N` with N > 1: `internal/formats` and `internal/template` each cached a package-level `golang.org/x/text/cases.Caser`, which is not safe for concurrent use. Casing now constructs a `Caser` per call instead of sharing one; filter output is unchanged (#379)
- `tag lib add`, `tag lib edit`, `tag template new bundle`, `tag template new generator`, `tag template rename-var` and `tag generate agent-file` no longer silently drop a flag placed after their positional argument(s): urfave/cli stops parsing flags at the first positional, so e.g. `tag lib add <ref> --as name` previously installed the template under the wrong name instead of applying `--as`. These six commands now rescan the argument tail like the twelve commands that already did. An unrecognised trailing flag is now a hard error naming the flag, where it was previously swallowed as a silent positional argument; pass a literal dash-prefixed value with a `--` separator after another argument (e.g. `tag template rename-var old new -- --weird-dir`) (#375)
- `tag undo` no longer silently no-ops for files recorded as `overwrite` (via `--on-existing overwrite`) or `openapi-merge` (via `action: openapi`): it previously reported the generation as reverted while leaving those files untouched. It now correctly restores them from backup, same as `inject`/`append` (#352)
- `tag template lint` and `tag template variables` no longer scan the contents of `{% raw %}...{% endraw %}` blocks as live template expressions: a literal `{{ vars.* }}` inside a raw block is no longer reported as an undefined variable, and a variable referenced only inside a raw block is now correctly reported as unused (#332)
- Quote-aware variable scanning (#337), fixing defects shared by `tag template lint` and `tag template variables`:
  - A `vars.*` mention inside a string literal (e.g. `{{ replace("{{ vars.ghost }}") }}`) is no longer treated as a reference — and, symmetrically, a variable referenced only inside a string literal is now correctly reported as unused, matching `rename-var`'s behavior
  - A `}}` or `%}` delimiter inside a string literal no longer truncates the scan early, so a reference after it (e.g. `{% if a == "%}" and vars.missing %}`) is now found
  - Multiple references in one block (e.g. `{{ vars.alpha ~ vars.beta }}`) are now all found — previously only the last was seen
  - An attribute path that merely ends in `vars.NAME` (e.g. `{{ cfg.vars.attrname }}`) is no longer mistaken for a reference
  - A `{{ }}` / `{% %}` block spanning multiple lines is now scanned at all — previously invisible to the line-by-line scanner
- Derived-variable defaults and path placeholders now go through the same scanner, so a `vars.*` mention inside a string literal there is no longer reported as an undefined reference (#337)

## [0.13.0] - 2026-03-03

### Changed

- **BREAKING**: Template syntax (`{{ }}`, `{% %}`, `{# #}`) in user-provided variable values is no longer rendered during path processing. This prevents Server-Side Template Injection (SSTI) attacks. Derived variables (whose defaults are template expressions) are still resolved. Use `--allow-recursive-render` to restore the previous behavior if your templates rely on recursive rendering of user input.
- **BREAKING**: `config.CreateConfigFile` no longer accepts `*cli.Context`; it takes a `CreateConfigOptions` struct instead, decoupling the config package from the CLI framework (#145)
- Semantic exit codes: usage errors return exit code 2, not-found errors return 3, interrupted (Ctrl-C) returns 130 (#143)

### Added

- `--allow-recursive-render` flag for `tag scaffold` to opt in to the previous recursive rendering behavior
- SSTI mitigation for file content rendering — template syntax in user-provided variable values is escaped before content rendering, not just path processing (#142)
- Semantic exit codes via `ExitCoder` interface: 0 (OK), 1 (general), 2 (usage), 3 (not found), 130 (interrupted) (#143)
- Rich help text for `tag generate` command with arguments, flags, and examples (#144)
- Generate hooks now receive scaffold variables as `TAG_VAR_*` environment variables and `TAG_PROJECT_NAME` (#146)
- Unified path containment validation via `fileutil.ValidatePathContainment` with symlink resolution
- Symlink detection in template file loading (`LoadTemplateFiles`)
- HTTPS enforcement for remote ZIP template URLs (HTTP rejected)
- TOCTOU-safe file operations in scaffold output writer (fd-based symlink verification)

### Security

- Fix SSTI vulnerability in file content rendering — extends existing path protection to template body output (#142)
- Fix SSTI vulnerability in path processor recursive rendering (#71)
- Fix TOCTOU race condition in symlink check during scaffolding (#72)
- Add symlink detection in template loader (#74)
- Reject insecure HTTP URLs for ZIP template downloads (#74)
- Consolidate path traversal validation with symlink resolution (#70)

## [2.0.0] - 2026-02-05

### Added

#### Project Scaffolding (`tag scaffold`)
- New `scaffold` command for creating complete projects from templates
- Support for local and remote template sources
- Interactive variable prompting with TTY detection
- Variable types: string, boolean, number, choice
- Computed/private variables (prefixed with `_`)
- JSON values file support (`--values`)
- Variable override via flags (`--meta`)
- Non-interactive mode (`--no-input`)
- Force overwrite (`--force`)

#### Remote Template Support
- GitHub shorthand: `gh:user/repo`, `gh:user/repo@v1.0.0`
- GitLab shorthand: `gl:user/repo`
- Bitbucket shorthand: `bb:user/repo`
- Full Git URLs (HTTPS and SSH)
- Zip file URLs
- Local zip file extraction
- Version/tag/branch pinning with `@version` suffix
- Subdirectory support within repositories
- Local caching with `--update` refresh option
- SSH key and token authentication

#### Replay System
- Automatic saving of scaffold inputs to `~/.tag/replay/`
- Replay previous inputs with `--replay` flag
- Skip saving with `--no-save` flag
- Secret variables excluded from replay files
- Version tracking for replay compatibility

#### Hooks System
- Pre-scaffold hooks (run before file generation)
- Post-scaffold hooks (run after file generation)
- Environment variables for hook scripts (`TAG_VAR_*`)
- Timeout protection (5 minutes per hook)
- Cross-platform shell execution (Unix and Windows)

#### Cookiecutter Conversion (`tag convert cookiecutter`)
- Convert `cookiecutter.json` to `tag.template.json`
- Path placeholder conversion (`{{ cookiecutter.var }}` → `__var__`)
- Directory and file renaming with filter support
- Content compatibility analysis
- Hook migration
- Dry-run mode for preview
- Remote template source support

#### Template Engine (Gonja/Jinja2)
- Jinja2-compatible syntax via [Gonja](https://github.com/noirbizarre/gonja)
- Template inheritance (`{% extends %}`, `{% block %}`)
- Macros (`{% macro %}`)
- Includes (`{% include %}`)
- Aliased namespaces: `vars.*` and `cookiecutter.*`

#### Built-in Filters
- Case transformations: `snake`, `pascal`, `camel`, `kebab`, `lower`, `upper`, `title`
- Inflections: `plural`, `singular`, `ordinalize`, `titleize`, `humanize`
- String operations: `split`, `join`, `contains`, `hasprefix`, `hassuffix`, `replace`, `trim`, `default`, `truncate`
- Filter aliases: `snake_case`, `pascal_case`, `camel_case`, `kebab_case`, `pluralize`, `singularize`

#### Path Placeholders
- Variable substitution in file/directory names: `__varname__`
- Filter support in paths: `__varname | filter__`

#### Configuration
- `tag.template.json` for template configuration
- JSON Schema validation (embedded in binary)
- Variable definitions with prompts, defaults, and validation

#### Documentation
- Comprehensive documentation in `docs/` directory
- Getting started guide
- Command references
- Template authoring guide
- Filter reference
- Hooks guide
- Migration guide from v1

### Changed

- Template engine changed from Go `text/template` to Gonja (Jinja2-compatible)
- Variable access syntax changed (see Migration Guide)
- Functions are now filters with pipe syntax

### Migration from v1

The template syntax has changed significantly. Key changes:

| v1 Syntax | v2 Syntax |
|-----------|-----------|
| `{{ .Name }}` | `{{ name }}` |
| `{{ .Meta.key }}` | `{{ vars.key }}` |
| `{{ .N.PascalCase }}` | `{{ n.pascal_case }}` |
| `{{ caseSnake .Name }}` | `{{ name\|snake }}` |
| `{{ if .Flag }}...{{ end }}` | `{% if flag %}...{% endif %}` |
| `{{ range .Items }}...{{ end }}` | `{% for item in items %}...{% endfor %}` |

### Dependencies

New dependencies added:
- `github.com/nikolalohinski/gonja/v2` - Jinja2-compatible template engine
- `github.com/go-git/go-git/v5` - Git repository fetching
- `github.com/xeipuuv/gojsonschema` - JSON Schema validation

## [1.x.x] - Previous Releases

See Git history for changes prior to v2.0.0.

[2.0.0]: https://github.com/kaikenlabs/tag/releases/tag/v2.0.0
