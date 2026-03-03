# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
