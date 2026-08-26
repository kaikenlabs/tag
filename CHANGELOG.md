# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `tag template info --format json` and `tag scaffold --format json` now emit exactly one JSON document on stdout on failure as well as on success: a document with `schema_version`, `tag_version`, and an `error` object (`code`, `message`, `exit_code`) instead of writing nothing to stdout. The same human-readable message is still written to stderr, as a plain `tag error: <message>` line without the usual `[HH:MM:SS.mmm]` timestamp prefix — this also applies to a flag-parse failure caught before the command runs (e.g. `tag scaffold --format json --bogus`), which previously dumped "Incorrect Usage" and full help text to stdout instead. `error.code` is one of a fixed vocabulary (`invalid_reference`, `template_not_found`, `auth_required`, `version_not_found`, `required_variable_missing`, `output_exists`, `circular_dependency`, `usage`, `internal`). Exit codes are unchanged — `error.code` carries the distinction between failure kinds, not the exit code. Only these two commands are affected; the other 20 `--format json` commands are unchanged (#396)
- `tag template lint`, `tag template variables` and `tag template rename-var` now recognise literal **subscript** variable access — `{{ vars["name"] }}` and `{{ vars['name'] }}` (whitespace-tolerant) — in addition to dot access. Subscript is the only way to reach a variable whose name collides with a Gonja keyword (e.g. `param.in`). Non-literal subscripts (`vars[expr]`) remain out of scope, since the key is not statically known (#339)

### Changed

- **Potentially breaking**: a remote template added to the library automatically — by `tag scaffold <remote-ref>` or by `tag lib add <remote-ref>` without `--as` — is now named `<basename>-<12-hex-digest>` instead of just `<basename>`. The digest covers the provider, host, owner, repository and subpath, so two organisations each publishing a `service-template` no longer compete for one library slot. The version is deliberately excluded, so a slot still tracks one logical template that `tag lib update` re-fetches in place, and every spelling of one repository (`gh:a/b`, `https://github.com/a/b.git`, `git@github.com:a/b.git`) resolves to the same name. Local references (`tag lib add ./my-template`) are unchanged, and `--as <name>` still overrides the derived name entirely. No migration is performed and none is needed to keep working: entries already in `library.json` keep their existing names, `tag lib update` never re-derives a name, and a project whose `.tagconfig.json` records an old short name still resolves against the old entry. What does change is that re-adding a source you already hold under the old short name creates a second entry under the new name rather than reusing the old one, and that `tag lib ls` no longer truncates the NAME column to 20 characters so the full name stays copy-pasteable. The "generator not found" hint now suggests `tag lib add <source> --as <name>`, because a bare `tag lib add <source>` would install the template under a name the project is not looking for (#430)
- **Potentially breaking**: because subscript access is now recognised (#339), templates that reference variables only through `vars["name"]` will produce findings they did not before — `tag template lint` can newly report an undeclared variable, and `tag template variables --strict` can flip a variable from unused to used. As with #337 this can change CI verdicts on templates whose source did not otherwise change; re-run once after upgrading. `tag template rename-var` now also rewrites subscript references, fixing a case where it renamed a declaration but left a live `vars["old"]` reference behind, silently corrupting the template (#339)


- **Potentially breaking**: `tag template lint` and `tag template variables` now share the same quote-aware variable scanner as `tag template rename-var`, so all three commands agree on what counts as a `vars.*` reference. Because this scanner finds real references the old regex missed and stops treating string-literal lookalikes as references, both `tag template lint` and `tag template variables --strict` can change verdict — in either direction — on templates whose source did not otherwise change. Downstream template repos relying on either command in CI should re-run once after upgrading (#337)
- **Potentially breaking**: `fileutil.ValidatePathContainment` — the internal guard deciding whether a generated file may be written, used by both `tag generate` and `tag scaffold` — now fails closed instead of open. Previously, any path it could not resolve through `EvalSymlinks` fell back to the caller-supplied unresolved path, which passed the containment check by construction. Where the write target was itself a dangling symlink pointing outside the base directory, the guard returned "contained" and the write then followed that symlink and landed outside the base. It now returns an error and the write is refused with `path safety check failed: failed to resolve path: ...`. This applies even when the dangling symlink points back inside the base directory, where the write would have been legitimate: a predicate that cannot resolve a path has no basis to grant permission, and every caller treats a nil result as authorization to write. A template or generator that writes through a pre-existing dangling symlink will now fail where it previously succeeded (#418)
- **Potentially breaking**: `tag template new generator <name> --in-bundle <bundle>` now validates the resolved bundle directory against the workspace root before using it as the base for the new generator. Previously this hop was never checked: a `--bundle-path` value that traversed outside the workspace (e.g. `--bundle-path ../../escape`), or a bundle directory that is a symlink pointing outside the workspace, was followed silently and the generator was written outside the workspace at exit 0. Both now fail with `path safety check failed: ...` at exit 1. A bundle directory that already exists inside the workspace is unaffected (#420)

### Fixed

- `tag lib add` copies a template's `_generators/` directory verbatim — it never rewrites it to `.tag/` — but generator resolution, `tag generate list`, and shell completion all read only `.tag/`. Since `SkipGeneratorCopy` is set on every scaffold that records a library origin (generators are meant to resolve from the library entry instead of being copied), `tag generate <name>` failed with "not found" and `tag generate list` reported nothing, at exit 0, for every project scaffolded from a library template. This completes the gap #429 tracked separately. All readers of a library template's generators — resolution, listing, and shell completion — now probe both roots, `.tag/` then `_generators/`, deduplicated by name; `tag template new generator --lib` and `tag template new bundle --lib` now write into whichever root the template already uses instead of always creating `.tag/`. Three behaviour changes users may notice: (1) a library template's `_generators/foo`, previously invisible, now takes precedence over a same-named project-local `.tag/foo`; (2) a newly-visible `_generators/foo` generator now takes precedence over a same-named bundle at `.tag/_bundles/foo`; (3) when one library entry has the *same* generator name in both roots, `.tag/` wins wholesale rather than the two directories being merged file-by-file — this is a known, deliberate divergence from a locally-copied project, which does merge them (#431)
- `tag scaffold <remote-ref>` produced a project with no generators at all, at exit 0, whenever the library name derived from that reference was already taken by a *different* source. The run skipped copying the template's generators into the project on the assumption that they would resolve from the library entry it was about to create, but the add then declined because the name was occupied, and nothing re-enabled the copy. Worse, the project still recorded that name in `.tagconfig.json`, and generator resolution is library-first on it, so `tag generate` could run the unrelated template's generators and write the wrong code. The library slot is now classified before the scaffold runs: a free slot, or one already holding this exact reference, behaves as before, while a slot held by a different source keeps the project's own generators, records no library name, leaves the library untouched, and reports both sources. The name collision that made this reachable in the first place is itself fixed by #430; the guard remains because a name can still be claimed explicitly with `tag lib add --as` (#429)
- `tag scaffold <local-dir> --add-to-lib` skipped copying the template's generators into the project *and* recorded no template name, leaving the project with neither a library origin nor a local copy, so `tag generate <gen>` failed with `generator "<gen>" not found in .tag`. The library slot name is now recorded for local templates too, matching what remote references have always done. (Reproduced against a build of `main`, so this predates #429 rather than regressing from it.) Note this is a precondition rather than a complete fix on its own: generators stored in a library entry's `_generators/` directory are not yet found by library-first resolution, which probes `.tag/` — tracked separately in #431 (#429)
- `tag convert cookiecutter <src>`, `tag lib add <dir>`, and the project snapshot behind `tag update --dir <dir>` / `tag diff --dir <dir>` produced silently empty results when the directory they were given was itself a symlink, completing the fix #414 and #419 applied to `tag scaffold` and the four `tag template` commands. `filepath.WalkDir` does not descend into a symlinked root — it yields the root as a single symlink entry and stops. Measured before the fix: `tag convert cookiecutter ./link -o out` reported `Files processed: 0` at exit 0 and wrote a template containing only `tag.template.json`; `tag lib add ./link --as x` exited 0, printed the success banner and library path, and stored an empty template directory; the project snapshot returned zero files with no error, which makes the 3-way merge treat every file in the project as deleted. All three now resolve the root through the shared `fileutil.ResolveSymlinkedRoot` before walking, so a symlinked directory behaves the same as the directory it points to. Symlinks *inside* the walked tree are unaffected — `convert` still skips them with its `skipped symlink: <path>` warning. `tag convert`'s default output directory still derives from the name you typed, so `tag convert cookiecutter ./linked` still writes `linked-tag`, not `<target>-tag`. Only a root the user named on their own filesystem is followed: a *fetched* template root that is a symlink is now refused with `fetched template root is a symlink: <path>` rather than followed, since a repository can commit its subpath as a symlink pointing outside the tree it was fetched into. Path spelling for a non-symlinked root is unchanged everywhere. `internal/lockfile`'s template hashing was audited and deliberately left alone: a symlinked root there fails loudly rather than producing a wrong hash, and its only caller is reached solely for remote templates, whose directory is a TAG-controlled cache path (#424)

- `tag template lint`, `tag template variables`, `tag template graph` and `tag template rename-var` against a template root that is itself a symlink reported confidently wrong results at exit 0, rather than the empty result #414 produced for `tag scaffold`: `filepath.WalkDir` does not descend into a symlinked root, but `tag.template.json` is read with `os.ReadFile`/`os.ReadDir`, which do follow one, so declaration reading and reference scanning disagreed about the same root. `lint` reported a template with a real error as valid; `variables` reported a variable that IS referenced as unused; `graph` lost the entire injection-marker section; `rename-var` reported no references found and changed nothing. The root is now resolved through the same `EvalSymlinks` fix #414 applied to the scaffold writer, shared via `fileutil.ResolveSymlinkedRoot`, so all four commands now produce identical output whether the template is reached directly or through a symlink. Symlinks *inside* a template are unaffected. No output changes for a template reached without a symlinked root (#419)
- `tag scaffold` against a template root that is itself a symlink wrote zero files and exited 0: `filepath.WalkDir` does not descend into a symlinked root, so `Write` saw only the root as a symlink entry and its own anti-exfiltration guard skipped it. The root is now resolved through `EvalSymlinks` before the walk starts, so scaffolding a symlinked root behaves the same as scaffolding the directory it points to. Symlinks *inside* a template are unaffected — they are still skipped with the existing `Warning: skipping symlink` message — and a template root that is a dangling symlink still fails, earlier, during reference resolution (#414)
- A generator directory that shipped its own `tag.template.json` (as both shipped examples, `examples/weather-api-go` and `examples/weather-api-python`, do) could not be run: `LoadTemplateFiles` read every non-directory entry into the template map, so the config file reached parsing, failed the required `to` field check, and aborted the whole `tag generate` run. The loader now skips a source file named `tag.template.json`; a generator can still emit an output file with that name by naming its source something else and setting `to:` accordingly. The same skip applies to the shared-templates directory, which uses the same loader. Relatedly, a generator `tag.template.json` that TAG cannot read or parse is now a hard error rather than being ignored: previously an unreadable or malformed one aborted the run only as a side effect of being loaded as a template, so skipping it would otherwise have let the `requires` gate it declares be silently bypassed. `tag generate <bundle>` now also enforces each bundled generator's own `requires`, which it never did before — a bundled generator that declared one could not run at all (#335)
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
