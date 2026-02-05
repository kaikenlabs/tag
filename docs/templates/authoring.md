# Template Authoring Guide

This guide covers how to create TAG templates for project scaffolding.

## Template Structure

A TAG template is a directory with the following structure:

```
my-template/
├── tag.template.json              # Template configuration (required)
├── __project_name__/              # Project files with path placeholders
│   ├── cmd/
│   │   └── main.go.tmpl           # Processed as Jinja2 template
│   ├── internal/
│   │   └── __module_name__/       # Directory with placeholder
│   │       └── service.go.tmpl
│   ├── assets/
│   │   └── logo.png               # Copied as-is (no .tmpl)
│   ├── README.md.tmpl
│   └── .gitignore
└── _generators/                   # Optional: becomes _templates/ in output
    └── handler/
        └── handler.tmpl
```

## File Processing Rules

| File Pattern | Processing |
|--------------|------------|
| `*.tmpl` | Parsed as Jinja2 template, `.tmpl` extension removed |
| `__varname__` in path | Replaced with variable value |
| `__varname \| filter__` in path | Replaced with filtered variable value |
| All other files | Copied as-is (binary-safe) |

## tag.template.json

The `tag.template.json` file defines your template's configuration:

```json
{
  "name": "Go API Template",
  "description": "A production-ready Go API template",
  "version": "1.0.0",
  "vars": {
    "project_name": "my-project",
    "author": {
      "type": "string",
      "prompt": "Who is the author?",
      "default": "Your Name",
      "required": true
    },
    "license": {
      "type": "choice",
      "prompt": "Select a license",
      "options": ["MIT", "BSD-3", "Apache-2.0"],
      "default": "MIT"
    },
    "use_docker": {
      "type": "boolean",
      "prompt": "Include Docker setup?",
      "default": false
    },
    "port": {
      "type": "number",
      "prompt": "Server port",
      "default": 8080
    },
    "_project_slug": "{{ vars.project_name|snake }}"
  },
  "hooks": {
    "pre_scaffold": ["./scripts/validate.sh"],
    "post_scaffold": ["go mod tidy", "git init"]
  }
}
```

See [tag.template.json Reference](../reference/tag.template.json.md) for complete documentation.

## Variable Definition Formats

### Short Form (String Default)

```json
{
  "vars": {
    "project_name": "my-project"
  }
}
```

### Long Form (Full Options)

```json
{
  "vars": {
    "author": {
      "type": "string",
      "prompt": "Who is the author?",
      "default": "Your Name",
      "required": true
    }
  }
}
```

### Variable Types

| Type | Description | Example |
|------|-------------|---------|
| `string` | Free text input | Author name, description |
| `boolean` | Yes/No selection | `use_docker`, `include_tests` |
| `number` | Numeric input | Port number, version |
| `choice` | Selection from list | License, framework |

### Private/Computed Variables

Variables starting with `_` are not prompted and are treated as computed values:

```json
{
  "vars": {
    "project_name": "my-project",
    "_project_slug": "{{ vars.project_name|snake }}"
  }
}
```

> **Note**: Computed variable expressions (like `{{ vars.project_name|snake }}`) are stored as literal strings in the configuration. They are NOT pre-evaluated before template rendering - the expression is passed through as-is and must be rendered in your templates if you want the computed value.

## Path Placeholders

Use `__varname__` syntax in file and directory names:

### Basic Substitution

```
__project_name__/           → my_awesome_project/
__module_name__.go.tmpl     → users.go
```

### With Filters

```
__project_name | snake__/   → my_awesome_project/
__project_name | pascal__/  → MyAwesomeProject/
__model | plural__/         → users/
```

### Supported Path Filters

- `snake` - snake_case
- `pascal` - PascalCase
- `camel` - camelCase
- `kebab` - kebab-case
- `lower` - lowercase
- `upper` - UPPERCASE
- `plural` - pluralize
- `singular` - singularize

## Template Syntax

TAG uses Jinja2-compatible syntax (via Gonja):

### Variables

```jinja2
{{ vars.project_name }}
{{ vars.author|upper }}
```

### Conditionals

```jinja2
{% if vars.use_docker %}
FROM golang:1.21
WORKDIR /app
{% endif %}
```

### Loops

```jinja2
{% for feature in vars.features %}
- {{ feature }}
{% endfor %}
```

### Filters

```jinja2
{{ vars.project_name|snake }}
{{ vars.model|plural|pascal }}
```

See [Template Syntax](syntax.md) and [Filter Reference](filters.md) for complete documentation.

## Template Context

Variables available in scaffold templates:

| Variable | Type | Description |
|----------|------|-------------|
| `vars` | `map[string]any` | All user-defined variables |
| `cookiecutter` | `map[string]any` | Alias for `vars` (Cookiecutter compat) |
| `<varname>` | `any` | Each variable also available at root level |

All variables defined in `tag.template.json` are available both in the `vars` namespace and directly at the root level:

```jinja2
{# Both are equivalent #}
{{ vars.project_name }}
{{ project_name }}
```

> **Note**: For generator templates (used with `tag generate`), different context variables are available. See [Generate Command](../commands/generate.md) for details.

## Including Generators

To include generators in your scaffolded projects, add a `_generators/` directory:

```
my-template/
├── tag.template.json
├── __project_name__/
│   └── ...
└── _generators/
    ├── handler/
    │   └── handler.tmpl
    └── model/
        └── model.tmpl
```

This becomes `_templates/` in the generated project, allowing users to run `tag generate` commands.

## Hooks

### Pre-Scaffold Hooks

Run before file generation, in the template directory:

```json
{
  "hooks": {
    "pre_scaffold": [
      "./scripts/validate-env.sh",
      "echo 'Starting scaffold...'"
    ]
  }
}
```

Use cases:
- Environment validation
- Pre-flight checks
- User notifications

### Post-Scaffold Hooks

Run after file generation, in the output directory:

```json
{
  "hooks": {
    "post_scaffold": [
      "go mod tidy",
      "git init",
      "npm install"
    ]
  }
}
```

Use cases:
- Dependency installation
- Git initialization
- Code formatting

See [Hooks Guide](hooks.md) for complete documentation.

## Best Practices

### 1. Use Meaningful Defaults

```json
{
  "vars": {
    "author": {
      "default": "{{ env.USER }}",
      "prompt": "Author name"
    }
  }
}
```

### 2. Group Related Variables

Organize your `tag.template.json` logically:

```json
{
  "vars": {
    "project_name": "...",
    "description": "...",

    "author": "...",
    "email": "...",

    "use_docker": false,
    "use_ci": false
  }
}
```

### 3. Provide Clear Prompts

```json
{
  "vars": {
    "license": {
      "type": "choice",
      "prompt": "Select an open-source license for your project",
      "options": ["MIT", "BSD-3", "Apache-2.0", "GPL-3.0"],
      "default": "MIT"
    }
  }
}
```

### 4. Use Computed Variables for Derived Values

```json
{
  "vars": {
    "project_name": "my-project",
    "_package_name": "{{ vars.project_name|snake }}",
    "_docker_image": "{{ vars.project_name|kebab }}"
  }
}
```

### 5. Document Your Template

Include a `README.md.tmpl` in your template:

```markdown
# {{ vars.project_name }}

{{ vars.description }}

## Getting Started

1. Install dependencies: `{{ vars.package_manager }} install`
2. Run: `{{ vars.package_manager }} start`
```

## Testing Your Template

```bash
# Test locally
tag scaffold ./my-template test-output

# Test with specific values
tag scaffold ./my-template test -m project_name=test -m author="Test User"

# Test non-interactive mode
tag scaffold ./my-template test --no-input
```

## Example: Complete Go API Template

```
go-api-template/
├── tag.template.json
├── __project_name__/
│   ├── cmd/
│   │   └── main.go.tmpl
│   ├── internal/
│   │   ├── api/
│   │   │   └── handler.go.tmpl
│   │   └── config/
│   │       └── config.go.tmpl
│   ├── go.mod.tmpl
│   ├── Dockerfile.tmpl
│   ├── Makefile.tmpl
│   └── README.md.tmpl
└── _generators/
    └── handler/
        └── handler.tmpl
```

**tag.template.json:**
```json
{
  "name": "Go API Template",
  "version": "1.0.0",
  "vars": {
    "project_name": "my-api",
    "module_path": {
      "type": "string",
      "prompt": "Go module path (e.g., github.com/user/project)",
      "required": true
    },
    "port": {
      "type": "number",
      "default": 8080
    },
    "use_docker": {
      "type": "boolean",
      "default": true
    }
  },
  "hooks": {
    "post_scaffold": ["go mod tidy"]
  }
}
```

## See Also

- [Template Syntax](syntax.md) - Jinja2/Gonja syntax guide
- [Filter Reference](filters.md) - Available filters
- [Hooks Guide](hooks.md) - Pre and post hooks
- [tag.template.json Reference](../reference/tag.template.json.md) - Configuration schema
