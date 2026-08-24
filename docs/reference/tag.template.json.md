# tag.template.json Reference

The `tag.template.json` file defines the configuration for a TAG scaffold template.

## Location

Place `tag.template.json` in the root of your template directory:

```
my-template/
├── tag.template.json    ← Configuration file
├── .tagignore           ← Optional: exclude files from output (gitignore syntax)
├── __project_name__/
│   └── ...
```

Both `tag.template.json` and `.tagignore` are automatically excluded from scaffold output. See [Template Authoring](../templates/authoring.md#excluding-files-with-tagignore) for `.tagignore` documentation.

## Schema Reference

For IDE autocompletion, add the `$schema` property:

```json
{
  "$schema": "https://tag.kaikenlabs.com/schemas/tag.template.schema.json",
  "name": "My Template",
  ...
}
```

## Top-Level Properties

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `name` | string | No | Template display name |
| `description` | string | No | Template description |
| `version` | string | No | Template version (semver recommended) |
| `keywords` | string[] | No | Topic keywords for discoverability in `tag lib search`. Mirrors the GitHub topic convention: all lowercase, hyphen-separated |
| `categories` | string[] | No | High-level buckets for this template (e.g. `"backend"`, `"cli"`, `"data"`) |
| `vars` | object | No | Variable definitions |
| `hooks` | object | No | Pre and post scaffold hooks |
| `test` | object | No | Matrix testing configuration |

### Example

```json
{
  "name": "Go API Template",
  "description": "A production-ready Go API with Docker and CI/CD",
  "version": "1.2.0",
  "keywords": ["go", "rest", "api"],
  "categories": ["backend"],
  "vars": {
    "project_name": "my-api",
    "author": "Your Name"
  },
  "hooks": {
    "post_scaffold": ["go mod tidy"]
  }
}
```

`keywords` and `categories` are also surfaced by
[`tag template info --format json`](../commands/template.md#tag-template-info) as `[]` when a
template declares neither.

## Variable Definitions

Variables can be defined in two formats: short form and long form.

### Short Form

Use short form for simple string defaults:

```json
{
  "vars": {
    "project_name": "my-project",
    "author": "Your Name",
    "version": "1.0.0"
  }
}
```

Short form also works for boolean and number defaults:

```json
{
  "vars": {
    "use_docker": true,
    "port": 8080
  }
}
```

### Long Form

Use long form for full control over variable behavior:

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

### Variable Properties

| Property | Type | Description |
|----------|------|-------------|
| `type` | string | Variable type: `string`, `boolean`, `number`, `choice` |
| `prompt` | string | Question shown during interactive input |
| `default` | any | Default value if not provided |
| `required` | boolean | Whether the variable must have a value (default: false) |
| `options` | string[] | Available choices (required for `choice` type) |
| `secret` | boolean | Hide input and exclude from replay (default: false) |

## Variable Types

### String Variables

```json
{
  "vars": {
    "author": {
      "type": "string",
      "prompt": "Author name",
      "default": "Your Name"
    }
  }
}
```

**Prompt display:**
```
Author name [Your Name]: █
```

### Boolean Variables

```json
{
  "vars": {
    "use_docker": {
      "type": "boolean",
      "prompt": "Include Docker setup?",
      "default": false
    }
  }
}
```

**Prompt display:**
```
Include Docker setup? [y/N]: █
```

### Number Variables

```json
{
  "vars": {
    "port": {
      "type": "number",
      "prompt": "Server port",
      "default": 8080
    }
  }
}
```

**Prompt display:**
```
Server port [8080]: █
```

### Choice Variables

```json
{
  "vars": {
    "license": {
      "type": "choice",
      "prompt": "Select a license",
      "options": ["MIT", "BSD-3", "Apache-2.0", "GPL-3.0"],
      "default": "MIT"
    }
  }
}
```

**Prompt display:**
```
Select a license:
  1. MIT
  2. BSD-3
  3. Apache-2.0
  4. GPL-3.0
Choose [1]: █
```

### Required Variables

```json
{
  "vars": {
    "module_path": {
      "type": "string",
      "prompt": "Go module path (e.g., github.com/user/project)",
      "required": true
    }
  }
}
```

Required variables:
- Must be provided via prompt, `--meta`, or `--values`
- Don't have a default value (or default is empty)
- Scaffold fails if not provided with `--no-input`

### Secret Variables

```json
{
  "vars": {
    "api_key": {
      "type": "string",
      "prompt": "API key",
      "secret": true
    }
  }
}
```

Secret variables:
- Input is hidden during prompting
- **Not saved** in replay files
- Must be re-entered on `--replay`

### Private Variables

Variables starting with `_` are private (not prompted):

```json
{
  "vars": {
    "project_name": "my-project",
    "_project_slug": "{{ vars.project_name|snake }}",
    "_docker_image": "{{ vars.project_name|kebab }}"
  }
}
```

Private variables:
- Not shown in interactive prompts
- Computed during template rendering
- Useful for internal computed values that users shouldn't edit

### Derived Variables

Derived variables have a template expression as their default value that references other variables. Following Cookiecutter's behavior, derived variables are **automatically skipped** during interactive prompting—their values are computed from other variables during template rendering.

```json
{
  "vars": {
    "package_display_name": "My Package",
    "package_name": "{{ vars.package_display_name | lower | replace(' ', '_') }}",
    "github_repo": "{{ vars.package_name }}"
  }
}
```

In this example:
- `package_display_name` will be prompted (it's a regular variable)
- `package_name` will NOT be prompted (derived from `package_display_name`)
- `github_repo` will NOT be prompted (derived from `package_name`)

**User prompt sequence:**
```
Enter value for package_display_name [My Package]: Awesome Library
```

The `package_name` will be computed as `awesome_library` and `github_repo` will also be `awesome_library`.

**Detection rules:** A variable is considered derived if it uses the **minimal form** (bare string value) and its default contains `{{ vars.` (TAG's variable namespace).

### Evaluated-Default Variables

Expanded-form variables with an explicit `prompt` and a template-expression default are prompted interactively. The expression is resolved from previously collected variables and shown as the suggested default, which the user can accept or override.

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

**User prompt sequence:**
```
Enter value for project_name [my-service]: my-service
Go module path [bitbucket.org/myorg/my-service]: ⏎
```

The key distinction from derived variables is the explicit `prompt` field:

| Variable form | `prompt` set? | Behaviour |
|---|---|---|
| `"pkg": "{{ vars.x }}"` | No | Derived — silent, never prompted |
| `{"prompt": "…", "default": "{{ vars.x }}"}` | Yes | Prompted with resolved suggestion |

In non-TTY mode the expression is resolved silently, the same as a derived variable.

## Hooks Configuration

### Structure

```json
{
  "hooks": {
    "pre_scaffold": ["command1", "command2"],
    "post_scaffold": ["command1", "command2"]
  }
}
```

### Pre-Scaffold Hooks

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

- Run **before** file generation
- Working directory: template directory
- Failure **stops** scaffold (no files created)

### Post-Scaffold Hooks

```json
{
  "hooks": {
    "post_scaffold": [
      "go mod tidy",
      "git init",
      "git add .",
      "git commit -m 'Initial commit'"
    ]
  }
}
```

- Run **after** file generation
- Working directory: output directory
- Failure is a **warning** (files are kept)

See [Hooks Guide](../templates/hooks.md) for complete documentation.

## Test Configuration

The `test` block configures `tag test` for matrix testing boolean variable combinations.

### Structure

```json
{
  "test": {
    "project_name": "test-project",
    "commands": ["go build ./...", "go vet ./..."]
  }
}
```

### Test Properties

| Property | Type | Description |
|----------|------|-------------|
| `project_name` | string | Fixed project name for test scaffolds (avoids prompt) |
| `commands` | string[] | Validation commands run inside each scaffolded directory |

### Example

```json
{
  "name": "Go API Template",
  "vars": {
    "project_name": "my-api",
    "use_docker": { "type": "boolean", "default": true },
    "use_grpc": { "type": "boolean", "default": false }
  },
  "test": {
    "project_name": "test-api",
    "commands": ["go build ./..."]
  }
}
```

With 2 boolean variables, `tag test` generates 4 combinations. Each is scaffolded with `project_name=test-api` and validated by running `go build ./...` in the output directory.

Template-defined `commands` require `--accept-hooks` to execute (same security model as scaffold hooks). See [tag test](../commands/test.md) for full usage.

## Complete Example

```json
{
  "$schema": "https://tag.kaikenlabs.com/schemas/tag.template.schema.json",
  "name": "Go Microservice Template",
  "description": "Production-ready Go microservice with Docker, CI/CD, and observability",
  "version": "2.1.0",

  "vars": {
    "project_name": "my-service",

    "module_path": {
      "type": "string",
      "prompt": "Go module path (e.g., github.com/company/service)",
      "required": true
    },

    "author": {
      "type": "string",
      "prompt": "Author name",
      "default": "Your Name"
    },

    "description": {
      "type": "string",
      "prompt": "Project description",
      "default": "A Go microservice"
    },

    "port": {
      "type": "number",
      "prompt": "HTTP server port",
      "default": 8080
    },

    "license": {
      "type": "choice",
      "prompt": "License",
      "options": ["MIT", "Apache-2.0", "BSD-3-Clause", "Proprietary"],
      "default": "MIT"
    },

    "use_docker": {
      "type": "boolean",
      "prompt": "Include Docker support?",
      "default": true
    },

    "use_ci": {
      "type": "boolean",
      "prompt": "Include GitHub Actions CI?",
      "default": true
    },

    "database": {
      "type": "choice",
      "prompt": "Database",
      "options": ["none", "postgres", "mysql", "mongodb"],
      "default": "postgres"
    },

    "_project_slug": "{{ vars.project_name|snake }}",
    "_docker_image": "{{ vars.project_name|kebab }}",
    "_package_name": "{{ vars.module_path|split:'/'|last }}"
  },

  "hooks": {
    "pre_scaffold": [
      "echo 'Creating {{ vars.project_name }}...'"
    ],
    "post_scaffold": [
      "go mod tidy",
      "git init",
      "git add .",
      "git commit -m 'Initial commit from TAG template'",
      "echo ''",
      "echo 'Project created successfully!'",
      "echo 'Next steps:'",
      "echo '  cd {{ vars.project_name }}'",
      "echo '  make run'"
    ]
  }
}
```

## Version Format

The `version` field should follow [Semantic Versioning](https://semver.org/):

```
MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]
```

Examples:
- `1.0.0`
- `2.1.0`
- `1.0.0-beta.1`
- `1.0.0+build.123`

## JSON Schema

The full JSON Schema is embedded in TAG and available at:
- Runtime: `internal/schema/tag.template.schema.json`
- Published: `https://tag.kaikenlabs.com/schemas/tag.template.schema.json`

## Validation

TAG validates `tag.template.json` at scaffold time:
- Required `options` for `choice` type
- Valid variable types
- Valid semver for `version` (if provided)
- JSON structure conformance to the schema

You can also validate your template ahead of time with `tag template lint`, which checks the schema, Gonja template syntax, and `{{ vars.* }}` references against declared variables. See [tag template lint](../commands/template.md#tag-template-lint) for details.

## See Also

- [Template Authoring](../templates/authoring.md) - Creating templates
- [Hooks Guide](../templates/hooks.md) - Hook configuration details
- [Scaffold Command](../commands/scaffold.md) - Using templates
- [tag template lint](../commands/template.md#tag-template-lint) - Validate templates
- [JSON Contract](json-contract.md) - `--format json` version keys and bump policy
