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

## Error Handling

| Error | Cause | Solution |
|-------|-------|----------|
| "template path is required" | Missing template argument | Provide a template path or reference |
| "failed to resolve template" | Invalid reference or network error | Check the template reference format |
| "output directory already exists" | Target directory exists | Use `--force` or choose a different output |
| "missing required variable" | Required variable has no value | Provide value via `--meta` or prompts |

## See Also

- [Template Authoring](../templates/authoring.md) - How to create templates
- [Remote References](../reference/remote-refs.md) - Template source formats
- [tag.template.json Reference](../reference/tag.template.json.md) - Configuration format
