# tag scaffold

Create a new project from a template.

## Synopsis

```bash
tag scaffold [template] [project-name] [flags]
```

## Description

The `scaffold` command creates a new project from a local or remote template. It reads the template's `tag.template.json` configuration file to determine available variables and prompts you interactively (unless `--no-input` is specified).

### Interactive Template Picker

When no template argument is given and the terminal is interactive, TAG shows a fuzzy picker to select from templates installed in the local library. This replaces the former `tag run` command.

```bash
# No args — shows interactive fuzzy picker for library templates
tag scaffold
```

When a template name is given without a path prefix or remote shorthand, TAG first checks the library for a matching template before treating it as a local path.

## Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `template` | No | Local path, remote reference, or library template name (picker if omitted) |
| `project-name` | No | Override the `project_name` variable |

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--output <path>` | `-o` | Output directory (default: `./<project_name>`) |
| `--values <file>` | | JSON file with variable values |
| `--meta <key=value>` | `-m` | Variable override (repeatable) |
| `--no-input` | | Skip interactive prompts, use defaults only |
| `--force` | `-f` | Overwrite output directory if it exists |
| `--update` | `-u` | Force refresh of cached remote templates |
| `--replay` | | Reuse saved values from a previous scaffold |
| `--no-save` | | Don't save values for future replay |
| `--add-to-lib` | | Add the template to the library after scaffolding (enables generator resolution from library) |
| `--no-library` | | Never add the template to the shared library; generators are copied into the project instead. Beats `--add-to-lib` when both are given |
| `--accept-hooks` | | Accept hooks without prompting (disabled by default for remote templates) |
| `--dry-run` | `-d` | Preview which files would be written without creating the output directory |
| `--update-lock` | | Update the lockfile with the current template version |
| `--ignore-lock` | | Ignore the lockfile and scaffold from the current template state |
| `--format <fmt>` | | Output format: `text` (default) or `json`. Recognised before or after the `template`/`project-name` positionals |

## Template Formats

TAG supports various template sources:

| Format | Example |
|--------|---------|
| Library template | `go-api` (name only, resolved from library) |
| Local directory | `./my-template`, `/path/to/template` |
| GitHub | `gh:user/repo`, `gh:user/repo@v1.0.0`, `gh:user/repo/subdir` |
| GitLab | `gl:user/repo`, `gl:user/repo@v1.0.0` |
| Bitbucket | `bb:user/repo` |
| Git URL | `https://github.com/user/repo.git` |
| SSH Git URL | `git@github.com:user/repo.git` |
| Zip URL | `https://example.com/template.zip` |
| Local zip | `./template.zip` |

See [Remote References](../reference/remote-refs.md) for detailed format documentation.

## Examples

### Interactive Picker (Library Templates)

```bash
# No args — shows interactive fuzzy picker
tag scaffold
```

### Scaffold from Library Template

```bash
# Scaffold using a library template by name
tag scaffold go-api my-service

# With variable overrides
tag scaffold go-api my-service -m author="Jane Doe" -m license=MIT
```

### Basic Usage

```bash
# Scaffold from a local template
tag scaffold ./my-template

# Scaffold from a GitHub template
tag scaffold gh:user/awesome-template
```

### Specifying Project Name

```bash
# Project name as argument
tag scaffold gh:user/go-api my-awesome-api

# Using the -o flag for custom output directory
tag scaffold gh:user/go-api -o ./projects/my-api
```

### Providing Variables

```bash
# Single variable
tag scaffold gh:user/template -m author="John Doe"

# Multiple variables
tag scaffold gh:user/template -m author="John Doe" -m license=MIT

# From a JSON file
tag scaffold gh:user/template --values config.json
```

**Example values file (config.json):**
```json
{
  "project_name": "my-project",
  "author": "John Doe",
  "license": "MIT",
  "use_docker": true
}
```

### Version Pinning

```bash
# Specific version tag
tag scaffold gh:user/template@v1.0.0

# Specific branch
tag scaffold gh:user/template@develop

# Template in a subdirectory
tag scaffold gh:user/templates/go-api@v1.0.0
```

### Non-Interactive Mode

```bash
# Use all default values (for CI/CD)
tag scaffold gh:user/template --no-input

# Combine with --values for automated scaffolding
tag scaffold gh:user/template --values config.json --no-input

# Library template in CI/CD
tag scaffold go-api my-service --no-input --accept-hooks -m author="CI Bot"
```

### Replay Previous Inputs

```bash
# First scaffold (inputs are saved automatically)
tag scaffold gh:user/template my-project-1

# Later, scaffold again with saved values
tag scaffold gh:user/template my-project-2 --replay

# Scaffold without saving inputs
tag scaffold gh:user/template test-project --no-save
```

### Force Overwrite

```bash
# Overwrite existing directory
tag scaffold gh:user/template my-project --force
```

### Update Cached Template

```bash
# Force re-fetch of a cached remote template
tag scaffold gh:user/template --update
```

### Dry Run Mode

Use `--dry-run` to preview which files a scaffold would create. It writes neither the project, the project's `.tagconfig.json`/`.tag/history.json`, nor an entry in the shared template library. Each file that would be written is printed as:

```
(dry-run) would write: my-project/main.go
(dry-run) would write: my-project/go.mod
(dry-run) would write: my-project/Dockerfile
```

Binary files are listed the same way and are also skipped. No diff is shown for scaffold dry-run because these are new files with no existing content to compare against.

```bash
# Preview files before scaffolding
tag scaffold gh:user/template my-project --dry-run

# Useful before scaffolding from an unfamiliar remote template
tag scaffold gh:user/template --dry-run --no-input
```

For a remote template, or a local template scaffolded with `--add-to-lib`, a real run would also
add an entry to the shared library (see [Library Management](#library-management) below). A dry
run skips that write and prints one more line instead:

```
(dry-run) would add template to library as "go-api"
```

If that derived name is already held by the same source, dry-run prints the existing `Template
"<name>" already in library` message instead — the same message a real run would print — rather
than the "would add" line. This announcement is text-mode only; see
[Machine-Readable Output](#machine-readable-output) below for how `--format json` handles it.

For a remote template, a real run also creates or refreshes an entry in `<cwd>/.tag/lock.json` for
supply-chain integrity (see [Skipping the Library](#skipping-the-library---no-library) below). A
dry run does not create or refresh that entry. Where a real run would have written one — a
template's first use, or any run with `--update-lock` — the dry run prints one line to stderr
instead:

```
(dry-run) would pin gh:user/template in .tag/lock.json
```

An entry that already exists and still matches is silent under `--dry-run`, because a real run
would not have written anything there either.

Verification itself is not skipped: if an existing `.tag/lock.json` entry's checksum no longer
matches the resolved template, the run still fails with `template checksum mismatch`, exactly as a
real run would — a dry run previews the write, not the check. `--update-lock --dry-run` against a
mismatched entry reports success without rewriting the file, the same precedence `--update-lock`
has over a mismatch on a real run.

Resolving a remote reference — including under `--dry-run` — may fetch the template and replace
its cache entry, and opportunistically deletes expired entries and stale staging directories. This
is accepted: the cache is regenerable, and fetching the input is a precondition of previewing it at
all. `TAG_CACHE_DIR` (see [Environment Variables](#environment-variables) below) is the isolation
mechanism for a process that needs its dry runs not to touch a shared cache.

A Cookiecutter template detected under `--dry-run` is refused rather than converted:

```
This appears to be a Cookiecutter template.
Cannot convert during --dry-run.
Convert it first, then scaffold the converted template:
  tag convert cookiecutter gh:user/template
```

A preview can't convert-and-continue the way a real run does: conversion writes the converted
template to disk, which is itself a write a dry run must not perform, and there is no destination
left to hand off to a retried scaffold once that write is skipped. Convert with `tag convert
cookiecutter` first, then preview (or run) the converted template instead.

## Machine-Readable Output

```bash
tag scaffold ./my-template my-project --format json
tag scaffold ./my-template my-project --dry-run --format json
tag scaffold --format json ./my-template my-project   # --format works on either side of the positionals
```

```json
{
  "schema_version": 1,
  "tag_version": "v2.3.0",
  "output_dir": "/abs/path/my-project",
  "project_root": "/abs/path/my-project",
  "template": "./my-template",
  "files": [
    { "path": "README.md", "action": "create" },
    { "path": "src/main.go", "action": "create" }
  ],
  "created": 2,
  "dry_run": false
}
```

Bare object, no envelope. `output_dir` and `project_root` are always absolute paths. `action` is always
`"create"` — scaffold writes a fresh project tree, so it never reports inject/append/overwrite.
`files` is the same list, in the same order, whether `--dry-run` is set or not: both paths record
an entry at the same point right after a file is processed, and only whether the file actually
lands on disk differs.

`schema_version` and `tag_version` are covered in full, including the bump policy, in the
[JSON Contract reference](../reference/json-contract.md).

`project_root` is the directory that actually holds the generated project, and it is the one to
hand to anything that publishes or `cd`s into the result. It equals `output_dir` for most
templates. The two differ for a **project-wrapper** template — one whose root is a single
directory named by an expression, such as `{{ vars.project_name }}/`, which is what most
Cookiecutter conversions look like — combined with an explicit `--output`: that combination
deliberately does not unwrap, so the files land one level down and `output_dir` names the parent.
Without `--output` the wrapper is unwrapped instead (to avoid `my-project/my-project` nesting) and
the two are equal again.

Only files at the template root count as TAG metadata. A `tag.template.json`, `.tagignore` or
`_meta.json` sitting inside the wrapper directory is ordinary content: it is generated into the
project like any other file, whether or not the wrapper unwraps.

`files[].path` stays relative to `output_dir` in both shapes, so for a wrapper template it already
carries the project directory as a prefix. Join file paths onto `output_dir`, never onto
`project_root`. Under `--dry-run`, `project_root` names the directory the run *would* create;
nothing is written, so that directory does not exist.

That last point matters if you publish the result. A wrapper only unwraps when it holds *all* of
the template's generated content: a root with files beside the wrapper directory is written
whole instead — nothing is discarded, but `project_root` stays equal to `output_dir` even though
the template has a wrapper directory, and scaffold prints a warning naming the sibling entries
(on stderr under `--format json`). Add those entries to `.tagignore` to restore unwrapping. Walk
`files[]` relative to `output_dir` to enumerate what was written; `project_root` tells you where
the project *begins*.

`--format json` forces non-interactive behavior: it implies `--no-input` (defaults and `-m`
overrides still apply; prompts never fire), never shows the interactive template picker — running
`tag scaffold --format json` with no template argument is a usage error (exit `2`) instead — and
never prompts to convert a detected Cookiecutter template. A required variable with no default and
no `-m` override is an error rather than a blocked prompt. Hook output, the "Add template to
library?" prompt, and the post-scaffold summary/README render are all suppressed or rerouted to
stderr — stdout carries only the JSON document. `--dry-run --format json` does not create the
output directory, same as `--dry-run` in text mode. It also skips the library write, but silently:
neither the "Add template to library?" prompt nor the text-mode
`(dry-run) would add template to library as "<name>"` line is printed, and the JSON document gains
no field for it. A `--dry-run --format json` document cannot tell you whether a real run of the
same command would have added a library entry.

On failure, `--format json` writes a single JSON **error document** to stdout instead of the
success document above (never a mix of the two), and the same human-readable message goes to
stderr as a plain `tag error: <message>` line — no timestamp prefix, no color. This applies to
every failure of this command, including a flag-parse error caught before the command runs (e.g.
`tag scaffold --format json --bogus`). The process exit code is unchanged by `--format` — a
not-found template still exits `1`, a usage error still exits `2` — `error.code` is what
distinguishes the failure kind, not the exit code. See [Error documents](../reference/json-contract.md#error-documents)
in the JSON Contract reference for the document shape and the full `error.code` vocabulary.

## Variable Input Priority

Variables are resolved in this order (highest priority first):

1. `--meta` flag values
2. `--values` file
3. `--replay` saved values
4. Interactive prompts (if TTY)
5. Default values from `tag.template.json`

## Replay System

TAG automatically saves your inputs after a successful scaffold (unless `--no-save` is used). Replay files are stored in `~/.tag/replay/`, or under the directory named by the `TAG_REPLAY_DIR` environment variable if set — a relative value is a hard error. `TAG_REPLAY_DIR` is checked before `$HOME` is resolved, so it also works when `$HOME` is unset or unwritable (containers/sandboxes). If `$HOME` cannot be resolved and `TAG_REPLAY_DIR` is unset, replay saving warns instead of failing the scaffold — set `TAG_REPLAY_DIR` to silence the warning.

The replay system is useful for:
- Creating multiple projects with similar configuration
- Re-running scaffolds after template updates
- Sharing configurations across team members (copy the replay JSON files)

## Cookiecutter Template Support

TAG can automatically detect and convert Cookiecutter templates. When you run `tag scaffold` on a directory that contains `cookiecutter.json` but no `tag.template.json`, TAG will:

1. Prompt you to confirm the conversion
2. Ask for an output directory for the converted template
3. Convert the template to TAG format
4. Continue with scaffolding using the converted template

```bash
# Scaffold a Cookiecutter template (auto-detected)
tag scaffold ./my-cookiecutter-template

# Output:
# This appears to be a Cookiecutter template. Convert to TAG format? [Y/n]
# Output directory for converted template [./my-cookiecutter-template-tag]:
# Converted template to: ./my-cookiecutter-template-tag
# ...continues with normal scaffolding...
```

In non-interactive mode (`--no-input`), Cookiecutter templates cannot be auto-converted. Use `tag convert cookiecutter` first:

```bash
tag convert cookiecutter ./my-cookiecutter-template -o ./converted-template
tag scaffold ./converted-template --no-input
```

## Derived Variables

Derived variables (also called computed variables) are variables whose default value is a template expression that references other variables. Following Cookiecutter's behavior, derived variables are **not prompted** during interactive scaffolding—they are automatically computed from the values of other variables.

**Example `tag.template.json`:**
```json
{
  "vars": {
    "package_display_name": "My Package",
    "package_name": "{{ vars.package_display_name | lower | replace(' ', '_') }}",
    "github_repo": "{{ vars.package_name }}"
  }
}
```

**User experience:**
```
Enter value for package_display_name [My Package]: Awesome Library
# package_name is NOT prompted - computed as "awesome_library"
# github_repo is NOT prompted - computed as "awesome_library"
```

This ensures users only need to provide "input" values, while computed values are derived automatically.

### Evaluated-Default Variables

For a smart default that users can still override, use the **expanded form** with an explicit `prompt` and a template-expression default:

```json
{
  "vars": {
    "project_name": "my-service",
    "module_path": {
      "type": "string",
      "prompt": "Go module path",
      "default": "bitbucket.org/myorg/{{ vars.project_name }}"
    }
  }
}
```

**User experience:**
```
Enter value for project_name [my-service]: my-service
Go module path [bitbucket.org/myorg/my-service]: ⏎
```

The expression is resolved from already-collected variables and shown as the suggested default. Pressing Enter accepts it; typing replaces it. In non-TTY mode (`--no-input`), the expression resolves silently.

## File Exclusion (.tagignore)

Templates can include a `.tagignore` file at the template root to exclude files and directories from scaffold output using gitignore-style patterns. This is useful for excluding template-authoring tools (IDE configs, AI assistant files) that shouldn't appear in generated projects.

```
# Example .tagignore
.serena/
CLAUDE.md
*.log
```

The template root's `.tagignore` file is always excluded from output, and its patterns match against paths relative to the template root — including the wrapper segment of a [project-wrapper](#machine-readable-output) template. A `.tagignore` placed inside the wrapper instead of at the template root is not read for patterns; it is generated into the project like any other file. See [Template Authoring](../templates/authoring.md#excluding-files-with-tagignore) for full documentation.

## Hook Security

For security, hooks defined in remote templates are **disabled by default**. A malicious remote template could use hooks to execute arbitrary commands on your machine.

When hooks are skipped, TAG displays a warning:
```
Warning: This remote template defines hooks that have been skipped for security.
  To allow hooks, re-run with --accept-hooks
```

To allow hooks for a trusted remote template:
```bash
tag scaffold gh:trusted-org/template --accept-hooks
```

Local templates always run hooks, since you control the template source.

## Library Management

Templates must be added to the library before the interactive picker or name-based resolution can find them. See [tag lib](lib.md) for managing the library.

```bash
# Add a template to the library under a name you choose
tag lib add gh:user/go-api-template --as go-api

# List installed templates
tag lib ls

# Then scaffold (by name or picker)
tag scaffold go-api my-project
tag scaffold   # picker
```

Without `--as`, the name above would be the longer, collision-free `go-api-template-<digest>` —
see [Library Naming](lib.md#library-naming). Scaffolding directly from a remote reference (rather
than `tag lib add` followed by `tag scaffold <name>`) also adds the template to the library under
that same derived name, so its generators resolve from there on later runs. A local template is
added the same way — including having its name recorded in the generated `.tagconfig.json` — when
`--add-to-lib` is set, or interactively when the template has generators and you confirm the "Add
template to library?" prompt.

If the derived name is already held by a *different* source (a stale entry from an older TAG
version, or a manual `tag lib add --as` that happened to reuse it), TAG leaves the library
untouched, keeps the project's own generators instead of pointing at the other entry, and prints a
message naming both sources. Add the new source under a name you choose with `tag lib add <ref>
--as <name>` if you want it in the library too.

### Skipping the Library (--no-library)

`--no-library` stops a scaffold run from touching the shared library. This matters for a process scaffolding on behalf of many callers — a CI job or a Backstage-style integration — where every remote template being added unconditionally accumulates entries nobody asked for, and for a caller that wants the generated project self-contained rather than dependent on a shared library entry to resolve its generators.

`--no-library` suppresses the remote add and the local `--add-to-lib` path, including its interactive prompt. It beats `--add-to-lib` when both are given — no error, no warning, the same precedence `--ignore-lock` has over `--update-lock`. Scaffolding by library name (`tag scaffold <name>`) is unaffected, since that path doesn't write to the library either way.

The trade-off: with no library entry to resolve generators from, `--no-library` copies the template's generators into the generated project's `.tag/` directory instead, so the project is self-contained. Without the flag, a template that gets added to the library does *not* get its generators copied — they resolve from the library entry instead. The generated `.tagconfig.json` also records no template name under `--no-library` (`template.name` is empty); recording one would let generator resolution fall back to an unrelated library template that happens to share the derived name. `template.source`, `template.ref` and `template.version` are still recorded, so `tag update` keeps working.

`--no-library` doesn't affect the remote cache — `TAG_CACHE_DIR` is the cache isolation mechanism, see [Environment Variables](#environment-variables) — or replay files, which are gated by `--no-save` (see [Replay System](#replay-system) above). Combine `--no-save --no-library` for a run that writes nothing outside the output directory except `<cwd>/.tag/lock.json`, which every remote scaffold writes on first use for supply-chain integrity. Two flags suppress that write: `--dry-run` (see [Dry Run Mode](#dry-run-mode) above), which still verifies an existing entry, and `--ignore-lock`, which skips verification as well.

## Error Handling

| Error | Cause | Solution |
|-------|-------|----------|
| "template path is required" | Missing template argument in non-interactive mode | Provide a template path or reference |
| "failed to resolve template" | Invalid reference or network error | Check the template reference format |
| "output directory already exists" | Target directory exists | Use `--force` or choose a different output |
| "required variable missing" | Required variable has no value in `--no-input` mode | Error includes `--meta` and `--values` hints |
| "output directory escapes working directory" | `project_name` contains path traversal (`../`) | Use a simple project name without path separators |
| "This appears to be a Cookiecutter template" | Cookiecutter template in non-interactive mode | Use `tag convert cookiecutter` first |
| "Cannot convert during --dry-run" | Cookiecutter template detected under `--dry-run` | Run `tag convert cookiecutter <ref>` first, then scaffold the converted template |

This table describes the text-mode message. Under `--format json`, each of these failures is
reported as a JSON error document on stdout (with the same message text and a stable `error.code`)
instead of text on stderr with nothing on stdout — see
[Machine-Readable Output](#machine-readable-output) above.

## Migration from Previous Versions

The following commands have been restructured:

| Old Command | New Command |
|-------------|-------------|
| `tag run <template>` | `tag scaffold <template>` |
| `tag run` (picker) | `tag scaffold` (picker) |
| `tag init` | `tag template init` |
| `tag new <name>` | `tag template new generator <name>` |
| `tag new-bundle <name>` (alias: `nb`) | `tag template new bundle <name>` |
| `tag info <template>` | `tag template info <template>` |
| `tag version-check` | `tag version --check` |
| `tag generate --bundle <name>` | `tag generate <name>` (auto-resolved) |
| `tag lib inspect <name>` | `tag template info <name>` |

Removed flag aliases: `-tp` (use `--path`), `-sp` (use `--shared-path`), `-bp` (use `--bundle-path`).

## Environment Variables

| Variable | Description |
|----------|-------------|
| `NO_COLOR` | Disable colored output when set to any non-empty value (per [no-color.org](https://no-color.org/)) |
| `GITHUB_TOKEN` | Authentication token for GitHub remote templates |
| `GITLAB_TOKEN` | Authentication token for GitLab remote templates |
| `BITBUCKET_TOKEN` | Authentication token for Bitbucket remote templates |
| `TAG_CACHE_DIR` | Override the remote-template cache directory (default `~/.tag/cache`). Must be an absolute path — a relative value is a hard error. See [Cache Location](../reference/remote-refs.md#cache-location) |
| `TAG_REPLAY_DIR` | Override the replay-file directory (default `~/.tag/replay`). Must be an absolute path — a relative value is a hard error. See [Replay System](#replay-system) above |

Both `TAG_CACHE_DIR` and `TAG_REPLAY_DIR` are read before `$HOME` is resolved, so they also work when `$HOME` is unset or unwritable (containers/sandboxes). Leaving either unset keeps today's default path — there is no silent relocation for existing users.

**Multi-tenant / shared deployments** (e.g. a service running `tag` on behalf of multiple tenants, such as a Backstage scaffolder integration): a missing `TAG_CACHE_DIR` silently falls back to the single shared cache, and with it cross-tenant template disclosure — one tenant's cached remote template can be served to another. A multi-tenant caller must set `TAG_CACHE_DIR` explicitly and should fail its own startup if it is unset.

## See Also

- [tag lib](lib.md) - Manage the template library
- [Template Command](template.md) - Template management (new, init, info, list)
- [Template Authoring](../templates/authoring.md) - How to create templates
- [Remote References](../reference/remote-refs.md) - Template source formats
- [tag.template.json Reference](../reference/tag.template.json.md) - Configuration format
- [JSON Contract](../reference/json-contract.md) - `--format json` version keys and bump policy
