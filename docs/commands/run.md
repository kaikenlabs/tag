# tag run

Scaffold a project from a library template.

## Synopsis

```bash
tag run [template-name] [project-name] [flags]
```

## Description

The `run` command scaffolds a new project using a template from the local library. It is a convenience wrapper around `tag scaffold` that resolves templates from the library instead of requiring a full reference.

If no template name is given and the terminal is interactive, a fuzzy picker is shown to select from installed templates.

## Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `template-name` | No | Name of the library template (interactive picker if omitted) |
| `project-name` | No | Override the `project_name` variable |

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--output <path>` | `-o` | Output directory (default: `./<project_name>`) |
| `--values <file>` | | JSON file with variable values |
| `--meta <key=value>` | `-m` | Variable override (repeatable) |
| `--no-input` | | Skip interactive prompts, use defaults only |
| `--force` | `-f` | Overwrite output directory if it exists |
| `--replay` | | Reuse saved values from a previous scaffold |
| `--no-save` | | Don't save values for future replay |
| `--accept-hooks` | | Run hooks without prompting |

## Examples

### Interactive Template Picker

```bash
# No args — shows interactive fuzzy picker
tag run
```

### Scaffold from Library Template

```bash
# Scaffold using a specific template
tag run go-api my-service

# With variable overrides
tag run go-api my-service -m author="Jane Doe" -m license=MIT
```

### Non-Interactive Mode

```bash
# For CI/CD pipelines
tag run go-api my-service --no-input --accept-hooks -m author="CI Bot"

# With a values file
tag run go-api my-service --no-input --values config.json
```

### Replay Previous Inputs

```bash
# Reuse saved values from a previous scaffold
tag run go-api another-service --replay
```

## Library Management

Templates must be added to the library before using `tag run`. See [tag lib](lib.md) for managing the library.

```bash
# Add a template to the library
tag lib add gh:user/go-api-template

# List installed templates
tag lib ls

# Then scaffold
tag run go-api-template my-project
```

## Cookiecutter Templates

If a library template was edited and reverted to a Cookiecutter format (e.g., via `tag lib edit`), `tag run` will detect this and suggest running `tag lib update` to re-convert it.

## See Also

- [tag lib](lib.md) - Manage the template library
- [tag scaffold](scaffold.md) - Scaffold from any template source
- [Template Authoring](../templates/authoring.md) - Creating templates
