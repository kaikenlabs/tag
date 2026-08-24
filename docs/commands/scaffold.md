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

Use `--dry-run` to preview which files a scaffold would create without writing anything to disk or creating the output directory. Each file that would be written is printed as:

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

## Machine-Readable Output

```bash
tag scaffold ./my-template my-project --format json
tag scaffold ./my-template my-project --dry-run --format json
tag scaffold --format json ./my-template my-project   # --format works on either side of the positionals
```

```json
{
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

`project_root` is the directory that actually holds the generated project, and it is the one to
hand to anything that publishes or `cd`s into the result. It equals `output_dir` for most
templates. The two differ for a **project-wrapper** template — one whose root is a single
directory named by an expression, such as `{{ vars.project_name }}/`, which is what most
Cookiecutter conversions look like — combined with an explicit `--output`: that combination
deliberately does not unwrap, so the files land one level down and `output_dir` names the parent.
Without `--output` the wrapper is unwrapped instead (to avoid `my-project/my-project` nesting) and
the two are equal again.

`files[].path` stays relative to `output_dir` in both shapes, so for a wrapper template it already
carries the project directory as a prefix. Join file paths onto `output_dir`, never onto
`project_root`. Under `--dry-run`, `project_root` names the directory the run *would* create;
nothing is written, so that directory does not exist.

`--format json` forces non-interactive behavior: it implies `--no-input` (defaults and `-m`
overrides still apply; prompts never fire), never shows the interactive template picker — running
`tag scaffold --format json` with no template argument is a usage error (exit `2`) instead — and
never prompts to convert a detected Cookiecutter template. A required variable with no default and
no `-m` override is an error rather than a blocked prompt. Hook output, the "Add template to
library?" prompt, and the post-scaffold summary/README render are all suppressed or rerouted to
stderr — stdout carries only the JSON document. `--dry-run --format json` does not create the
output directory, same as `--dry-run` in text mode.

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

The `.tagignore` file itself is always excluded from output. See [Template Authoring](../templates/authoring.md#excluding-files-with-tagignore) for full documentation.

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
# Add a template to the library
tag lib add gh:user/go-api-template

# List installed templates
tag lib ls

# Then scaffold (by name or picker)
tag scaffold go-api-template my-project
tag scaffold   # picker
```

## Error Handling

| Error | Cause | Solution |
|-------|-------|----------|
| "template path is required" | Missing template argument in non-interactive mode | Provide a template path or reference |
| "failed to resolve template" | Invalid reference or network error | Check the template reference format |
| "output directory already exists" | Target directory exists | Use `--force` or choose a different output |
| "required variable missing" | Required variable has no value in `--no-input` mode | Error includes `--meta` and `--values` hints |
| "output directory escapes working directory" | `project_name` contains path traversal (`../`) | Use a simple project name without path separators |
| "This appears to be a Cookiecutter template" | Cookiecutter template in non-interactive mode | Use `tag convert cookiecutter` first |

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
