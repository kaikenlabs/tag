# tag scaffold

Create a new project from a template.

## Synopsis

```bash
tag scaffold <template> [project-name] [flags]
```

## Description

The `scaffold` command creates a new project from a local or remote template. It reads the template's `tag.template.json` configuration file to determine available variables and prompts you interactively (unless `--no-input` is specified).

## Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `template` | Yes | Local path or remote reference to the template |
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

## Template Formats

TAG supports various template sources:

| Format | Example |
|--------|---------|
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

## Variable Input Priority

Variables are resolved in this order (highest priority first):

1. `--meta` flag values
2. `--values` file
3. `--replay` saved values
4. Interactive prompts (if TTY)
5. Default values from `tag.template.json`

## Replay System

TAG automatically saves your inputs after a successful scaffold (unless `--no-save` is used). Replay files are stored in `~/.tag/replay/`.

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

## Error Handling

| Error | Cause | Solution |
|-------|-------|----------|
| "template path is required" | Missing template argument | Provide a template path or reference |
| "failed to resolve template" | Invalid reference or network error | Check the template reference format |
| "output directory already exists" | Target directory exists | Use `--force` or choose a different output |
| "missing required variable" | Required variable has no value | Provide value via `--meta` or prompts |
| "This appears to be a Cookiecutter template" | Cookiecutter template in non-interactive mode | Use `tag convert cookiecutter` first |

## See Also

- [tag run](run.md) - Scaffold from a library template
- [tag lib](lib.md) - Manage the template library
- [Template Authoring](../templates/authoring.md) - How to create templates
- [Remote References](../reference/remote-refs.md) - Template source formats
- [tag.template.json Reference](../reference/tag.template.json.md) - Configuration format
