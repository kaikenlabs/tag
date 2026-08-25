# Template Authoring Guide

This guide covers how to create TAG templates for project scaffolding.

## Template Structure

A TAG template is a directory with the following structure:

```
my-template/
├── tag.template.json                    # Template configuration (required)
├── .tagignore                           # Optional: exclude files from output
├── {{ vars.project_name }}/             # Project files with path placeholders
│   ├── cmd/
│   │   └── main.go.tmpl                 # Processed as Jinja2 template
│   ├── internal/
│   │   └── {{ vars.module_name }}/      # Directory with placeholder
│   │       └── service.go.tmpl
│   ├── assets/
│   │   └── logo.png                     # Copied as-is (no .tmpl)
│   ├── README.md.tmpl
│   └── .gitignore
├── _generators/                         # Optional: becomes .tag/ in output
│   └── handler/
│       └── handler.go
└── _dialects/                           # Optional: dialect type-mapping overrides
    └── go.yaml                          # Override individual type mappings
```

## File Processing Rules

| File Pattern | Processing |
|--------------|------------|
| `*.tmpl` | Parsed as Jinja2 template, `.tmpl` extension removed |
| `{{ vars.name }}` in path | Replaced with variable value |
| `{{ vars.name \| filter }}` in path | Replaced with filtered variable value |
| Symlinks | Skipped (not followed, warning printed) |
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

### Private Variables

Variables starting with `_` are not prompted:

```json
{
  "vars": {
    "project_name": "my-project",
    "_internal_setting": "some-value"
  }
}
```

Private variables are useful for internal configuration values that users shouldn't edit.

### Derived Variables

Derived variables have template expressions as their defaults that reference other variables. Following Cookiecutter's behavior, derived variables are **automatically skipped** during prompting—their values are computed during template rendering.

```json
{
  "vars": {
    "project_name": "my-project",
    "package_name": "{{ vars.project_name | snake }}",
    "docker_image": "{{ vars.project_name | kebab }}"
  }
}
```

In this example:
- `project_name` will be prompted (regular variable)
- `package_name` will NOT be prompted (derived from `project_name`)
- `docker_image` will NOT be prompted (derived from `project_name`)

**Detection rules:** A variable is considered derived if it uses the **minimal form** (bare string value) and its default contains `{{ vars.`.

> **Note**: Derived variables are passed through as template expressions and evaluated during rendering. This allows complex computations like `{{ vars.name.lower().replace(' ', '_') }}`.

### Evaluated-Default Variables

Sometimes you want a smart, context-aware default while still letting the user change it. Use the **expanded form** with an explicit `prompt` alongside a template-expression default:

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

The user can press Enter to accept the resolved default, or type a custom value. This is distinct from derived variables—the difference is the explicit `prompt` key:

| Form | Prompt? | Default |
|------|---------|---------|
| `"module_path": "{{ vars.x }}"` | No — silently computed | Expression |
| `{"prompt": "...", "default": "{{ vars.x }}"}` | Yes — with resolved suggestion | Expression |

In non-TTY mode (`--no-input` or piped), the expression is resolved automatically just like a derived variable.

## Path Placeholders

Use Jinja2-style `{{ vars.name }}` syntax in file and directory names:

### Basic Substitution

```
{{ vars.project_name }}/           → my_awesome_project/
{{ vars.module_name }}.go.tmpl     → users.go
```

### With Filters

```
{{ vars.project_name | snake }}/   → my_awesome_project/
{{ vars.project_name | pascal }}/  → MyAwesomeProject/
{{ vars.model | plural }}/         → users/
```

### Method Calls

TAG also supports Python-style method calls:

```
{{ vars.name.lower() }}/                              → myproject/
{{ vars.name.lower().replace(' ', '_') }}/            → my_project/
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
    │   └── handler.go
    └── model/
        └── model.go
```

This becomes `.tag/` in the generated project, allowing users to run `tag generate` commands.

## Excluding Files with .tagignore

Place a `.tagignore` file in your template root to exclude files and directories from scaffold output. This uses standard gitignore syntax and is ideal for keeping template-authoring tools out of generated projects.

```
# AI coding assistant configs
.serena/
CLAUDE.md
.mcp.json

# Editor settings
.vscode/
.idea/

# Build artifacts
*.log
tmp/
```

### Pattern syntax

| Pattern | Matches |
|---------|---------|
| `*.log` | All `.log` files at any depth |
| `temp/` | The `temp` directory and everything inside it |
| `**/*.tmp` | `.tmp` files in any nested directory |
| `!important.log` | Re-includes `important.log` after a `*.log` exclusion |
| `# comment` | Comment (ignored) |

### Behavior

- `.tagignore` itself is always excluded from output (like `tag.template.json`)
- Matched directories are pruned entirely — their contents are not traversed
- An empty or missing `.tagignore` has no effect
- Patterns follow [gitignore rules](https://git-scm.com/docs/gitignore): patterns without `/` match at any depth; patterns with `/` anchor to the template root
- `.tagignore` also decides [project-wrapper detection](../commands/scaffold.md#machine-readable-output): a wrapper only unwraps when it holds *all* of the template's generated content, so an entry matched by `.tagignore` doesn't count as content beside the wrapper. Listing your template-authoring files there is what keeps a wrapper template unwrapping.

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

## Validating Your Template

Before scaffolding, run the linter to catch errors early:

```bash
# Lint the template directory
tag template lint ./my-template

# Machine-readable output for CI
tag template lint ./my-template --format json
```

This checks `tag.template.json` against the JSON Schema, parses all template files for Gonja syntax errors, and verifies that `{{ vars.* }}` references match declared variables.

See [tag template lint](../commands/template.md#tag-template-lint) for full details.

## Renaming a Variable

Renaming a variable by hand means touching the declaration, every expression, and any path placeholder — missing one leaves a silently empty value at render time. Let TAG do it:

```bash
# Preview every change first
tag template rename-var --dry-run project_name service_name

# Apply
tag template rename-var project_name service_name
```

This rewrites the declaration, derived defaults, hook commands, `requires` entries, all expressions, and file or directory name placeholders. Prose, comments and `{% raw %}` blocks are left alone.

See [tag template rename-var](../commands/template.md#tag-template-rename-var) for full details.

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
├── {{ vars.project_name }}/
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
        └── handler.go
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
- [tag template lint](../commands/template.md#tag-template-lint) - Validate templates before publishing
