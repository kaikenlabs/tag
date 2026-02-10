# TAG

A powerful code generation and project scaffolding CLI for developers.

TAG combines the project bootstrapping capabilities of [Cookiecutter](https://cookiecutter.readthedocs.io/) with incremental code generation, supporting both full project scaffolding from templates and adding code to existing projects.

## Features

- **Project Scaffolding** - Create complete projects from local or remote templates
- **Incremental Generation** - Add files, append content, or inject code into existing files
- **Remote Templates** - Fetch templates from GitHub, GitLab, Bitbucket, or any Git repository
- **Jinja2 Syntax** - Familiar template syntax via [Gonja](https://github.com/noirbizarre/gonja)
- **Cookiecutter Compatible** - Convert and use existing Cookiecutter templates
- **Interactive Prompts** - Guided variable input with defaults and choices
- **Replay System** - Save and reuse scaffold inputs for reproducibility
- **Hooks** - Run commands before and after scaffolding
- **Single Binary** - Pure Go, no external dependencies

## Quick Start

### Install

```bash
# Quick install (macOS/Linux)
curl -sSfL https://raw.githubusercontent.com/kaikenlabs/tag/main/install.sh | sh

# With Go
go install github.com/kaikenlabs/tag@latest

# Specific version
curl -sSfL https://raw.githubusercontent.com/kaikenlabs/tag/main/install.sh | sh -s -- --version v0.2.0
```

### Scaffold a Project

```bash
# From a GitHub template
tag scaffold gh:user/awesome-template

# From a local template
tag scaffold ./my-template

# With a project name
tag scaffold gh:user/go-api my-new-api
```

### Generate Code in Existing Projects

```bash
# Initialize TAG in your project
tag init

# Create a generator
tag new handler

# Generate code
tag generate handler UserAuth
```

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](docs/getting-started.md) | Installation and first steps |
| [Scaffold Command](docs/commands/scaffold.md) | Project scaffolding reference |
| [Generate Command](docs/commands/generate.md) | Code generation reference |
| [Convert Command](docs/commands/convert.md) | Cookiecutter migration |
| [Template Authoring](docs/templates/authoring.md) | Creating templates |
| [Template Syntax](docs/templates/syntax.md) | Jinja2/Gonja syntax guide |
| [Filter Reference](docs/templates/filters.md) | Available template filters |
| [Hooks Guide](docs/templates/hooks.md) | Pre and post hooks |
| [tag.template.json](docs/reference/tag.template.json.md) | Configuration reference |
| [Remote References](docs/reference/remote-refs.md) | Remote template formats |

## Template Sources

TAG supports multiple template sources:

| Format | Example |
|--------|---------|
| GitHub | `gh:user/repo`, `gh:user/repo@v1.0.0` |
| GitLab | `gl:user/repo` |
| Bitbucket | `bb:user/repo` |
| Git URL | `https://github.com/user/repo.git` |
| Zip URL | `https://example.com/template.zip` |
| Local | `./my-template`, `/path/to/template` |

## Template Syntax

TAG uses Jinja2-compatible syntax:

~~~jinja2
# {{ vars.project_name }}

Author: {{ vars.author }}

{% if vars.use_docker %}
## Docker

    docker build -t {{ vars.project_name|kebab }} .

{% endif %}

## Features

{% for feature in vars.features %}
- {{ feature|title }}
{% endfor %}
~~~

### Available Filters

**Case transformations:** `snake`, `pascal`, `camel`, `kebab`, `lower`, `upper`, `title`

**Inflections:** `plural`, `singular`, `ordinalize`, `titleize`, `humanize`

**String operations:** `split`, `join`, `contains`, `hasprefix`, `hassuffix`, `replace`, `trim`, `default`, `truncate`

## Template Configuration

Templates are configured via `tag.template.json`:

```json
{
  "name": "Go API Template",
  "version": "1.0.0",
  "vars": {
    "project_name": "my-api",
    "author": {
      "type": "string",
      "prompt": "Author name",
      "default": "Your Name"
    },
    "license": {
      "type": "choice",
      "options": ["MIT", "Apache-2.0", "BSD-3"],
      "default": "MIT"
    },
    "use_docker": {
      "type": "boolean",
      "default": true
    }
  },
  "hooks": {
    "post_scaffold": ["go mod tidy", "git init"]
  }
}
```

## Generator Templates

For incremental code generation, create templates in `.tag/`:

```
---
to: internal/handlers/{{ name | snake }}_handler.go
---
package handlers

type {{ n.pascal_case }}Handler struct {}

func New{{ n.pascal_case }}Handler() *{{ n.pascal_case }}Handler {
    return &{{ n.pascal_case }}Handler{}
}
```

Generators support three actions:
- **Create** - Write new files (default)
- **Append** - Add to existing files (`append: true`)
- **Inject** - Insert before/after markers (`inject: true` + `before:`/`after:`)

## Cookiecutter Migration

TAG can automatically detect and convert Cookiecutter templates when scaffolding:

```bash
# Auto-detection - TAG will prompt to convert
tag scaffold ./my-cookiecutter-template

# Or convert explicitly
tag convert cookiecutter gh:user/cookiecutter-django ./django-tag
```

The converter:
- Transforms `cookiecutter.json` to `tag.template.json`
- Converts path placeholders (`{{ cookiecutter.var }}` → `{{ vars.var }}`)
- Preserves derived variables (computed from other variables)
- Reports Jinja2/Gonja syntax differences
- Copies and analyzes hooks

### Derived Variables

Following Cookiecutter's behavior, derived variables (those whose defaults reference other variables) are **not prompted** during scaffolding—they're computed automatically:

```json
{
  "vars": {
    "display_name": "My Package",
    "package_name": "{{ vars.display_name | lower | replace(' ', '_') }}"
  }
}
```

Only `display_name` is prompted; `package_name` is computed as `my_package`.

## Shell Completion

TAG supports shell completion for commands, flags, generator names, and library templates.

```bash
# Bash (add to ~/.bashrc)
source <(tag completion bash)

# Zsh (add to ~/.zshrc)
source <(tag completion zsh)

# Fish
tag completion fish | source
```

## Commands

| Command | Description |
|---------|-------------|
| `tag scaffold <template>` | Create project from template |
| `tag generate <generator> <name>` | Run a generator |
| `tag convert cookiecutter <source>` | Convert Cookiecutter template |
| `tag init` | Initialize TAG in a project |
| `tag new <name> [--lib]` | Create a new generator |
| `tag new-bundle <name> [--lib]` | Create a new bundle |
| `tag completion <shell>` | Output shell completion script |

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dry-run` | `-d` | false | Preview without writing |
| `--path` | `-tp` | `.tag` | Templates directory |
| `--shared` | `-sp` | `_shared` | Shared templates directory |

## Development

```bash
# Build
make build

# Test
go test ./...

# Lint
make lint
```

## License

MIT
