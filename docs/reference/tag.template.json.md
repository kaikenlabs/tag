# tag.template.json Reference

The `tag.template.json` file defines the configuration for a TAG scaffold template.

## Location

Place `tag.template.json` in the root of your template directory:

```
my-template/
├── tag.template.json    ← Configuration file
├── __project_name__/
│   └── ...
```

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
| `vars` | object | No | Variable definitions |
| `hooks` | object | No | Pre and post scaffold hooks |

### Example

```json
{
  "name": "Go API Template",
  "description": "A production-ready Go API with Docker and CI/CD",
  "version": "1.2.0",
  "vars": {
    "project_name": "my-api",
    "author": "Your Name"
  },
  "hooks": {
    "post_scaffold": ["go mod tidy"]
  }
}
```

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
- Stored as literal strings in the configuration
- The Jinja2 expressions shown above are NOT pre-evaluated
- To use computed values, reference them in templates or use template filters inline

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

## See Also

- [Template Authoring](../templates/authoring.md) - Creating templates
- [Hooks Guide](../templates/hooks.md) - Hook configuration details
- [Scaffold Command](../commands/scaffold.md) - Using templates
