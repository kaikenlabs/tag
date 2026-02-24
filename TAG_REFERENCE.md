# TAG Reference

A comprehensive reference for AI coding assistants to use TAG proficiently.

## What is TAG

TAG is a CLI tool for two complementary workflows:

1. **Scaffolding** (`tag scaffold`) — Create entire projects from templates. Templates define directory structures, files, variables, and hooks. Think "cookiecutter" but with Jinja2-compatible syntax via Gonja. With no args, shows an interactive picker for library templates.

2. **Code Generation** (`tag generate`) — Generate code within existing projects using generators or bundles (auto-resolved). A generator is a single template file with frontmatter metadata that can create, inject into, or append to files. Generators live in `.tag/` inside your project.

**Mental model**: Scaffold creates the project. Generators evolve it.

## Decision Tree

```
Need to preview a template before using it?
  → tag template info <template> (works with local, remote, and library templates)

Need to create a new project from scratch?
  → tag scaffold <template> (remote/local/library template)
  → tag scaffold (no args = interactive library picker)

Need to generate code in an existing project?
  Single file operation?
    → Write a generator, run with: tag generate <name> <arg>
  Multiple related files?
    → Write a bundle (groups generators), run with: tag generate <bundle> <arg>
    (generators and bundles are auto-resolved — no --bundle flag needed)

Need to create a reusable project template?
  → Create a directory with tag.template.json + template files

Need to convert a Cookiecutter template?
  → tag convert cookiecutter <source> -o <dest>
  → Or just scaffold it directly: tag scaffold <cookiecutter-repo> (auto-detects)
```

## Project Structure

After running `tag template init` in a project, you get:

```
my-project/
├── .tag/                    # Generator directory
│   ├── _shared/             # Shared template fragments ({% include %})
│   ├── _bundles/            # Bundle definitions (JSON)
│   │   ├── resource.json    # Regular bundle (references generators from .tag/)
│   │   └── examples/        # Self-contained bundle (generators inside)
│   │       ├── examples.json
│   │       ├── _shared/     # Bundle-scoped shared templates
│   │       ├── hello/
│   │       │   └── hello.go
│   │       └── greet/
│   │           └── greet.go
│   ├── my-generator/        # A generator (directory with template files)
│   │   └── my-generator.go  # Generator template file
│   └── another-gen/
│       └── another-gen.ts
└── .tagconfig.json          # Project config (created by scaffold)
```

### .tagconfig.json

Created automatically when scaffolding. Records the template origin for `--lib` flag support and replay.

```json
{
  "template": {
    "name": "my-template",
    "source": "gh:user/my-template",
    "version": "v1.0.0"
  }
}
```

## Anatomy of a Generator

A generator is a **single file** inside a named directory under `.tag/`. The file has two parts: YAML-like frontmatter and a Gonja template body.

### File naming

Generator files keep their **natural extension** (`.go`, `.ts`, `.py`, etc.) — they are NOT named `.tmpl`. The extension is irrelevant to TAG; all files in the generator directory are loaded as templates.

### Frontmatter

Delimited by `---` markers. Each line is `key: value` (simple line parser, NOT full YAML — no nested objects, no arrays).

```
---
to: app/services/{{ name | snake }}.go
---
package services

type {{ name | pascal }}Service struct{}
```

### Frontmatter fields

| Field | Required | Description |
|-------|----------|-------------|
| `to` | Yes | Output file path. Supports `{{ name }}`, `{{ vars.x }}`, and filters. |
| `inject` | No | `true` to inject into an existing file instead of creating. |
| `before` | No | Marker string — inject content BEFORE this line. Requires `inject: true`. |
| `after` | No | Marker string — inject content AFTER this line. Requires `inject: true`. |
| `append` | No | `true` to append to end of existing file (mutually exclusive with `inject`). |
| `desc` | No | Short description shown in `tag generate list` output. |
| `notes` | No | Message displayed after generation (e.g., "Remember to register the route"). |

Any unrecognized `key: value` pairs become extra metadata accessible via `{{ vars.key }}` in the body.

### Actions

**Create** (default) — Write a new file. Overwrites if the file exists.

```
---
to: app/models/{{ name | snake }}.go
---
package models
```

**Inject** — Insert content into an existing file at a marker location.

```
---
to: app/routes.go
inject: true
after: "// ROUTES"
---
router.Handle("/{{ name | kebab }}", {{ name | pascal }}Handler)
```

```
---
to: app/routes.go
inject: true
before: "// END ROUTES"
---
router.Handle("/{{ name | kebab }}", {{ name | pascal }}Handler)
```

**Append** — Add content to the end of an existing file.

```
---
to: app/registry.go
append: true
---
Register("{{ name | snake }}")
```

### Execution order

When a generator has multiple template files, they execute in order: **Create → Inject → Append**. This guarantees files exist before injection/append.

### The `name` variable

The second argument to `tag generate <generator> <name>` is available as `{{ name }}` (and `{{ n.snake }}`, `{{ n.pascal }}`, etc.) in both frontmatter and body.

### The `vars` namespace

Variables passed via `--meta` flags or from scaffold-time `.tagconfig.json` are in `{{ vars.x }}`.

```bash
tag generate service user_service -m package=api -m version=v2
```

```
---
to: {{ vars.package }}/{{ name | snake }}.go
---
// Version: {{ vars.version }}
package {{ vars.package }}
```

### Name convenience shortcuts

| Expression | Equivalent | Example (name = "user_service") |
|-----------|-----------|------|
| `{{ n.snake }}` | `{{ name \| snake }}` | `user_service` |
| `{{ n.pascal }}` | `{{ name \| pascal }}` | `UserService` |
| `{{ n.camel }}` | `{{ name \| camel }}` | `userService` |
| `{{ n.kebab }}` | `{{ name \| kebab }}` | `user-service` |
| `{{ n.lower }}` | `{{ name \| lower }}` | `user_service` |
| `{{ n.upper }}` | `{{ name \| upper }}` | `USER_SERVICE` |
| `{{ n.title }}` | `{{ name \| title }}` | `User_Service` |
| `{{ n.plural }}` | `{{ name \| plural }}` | `user_services` |
| `{{ n.singular }}` | `{{ name \| singular }}` | `user_service` |
| `{{ n.past }}` | `{{ name \| past }}` | `user_serviced` |

> **Note**: The `past` filter converts the last word to past tense. It handles irregular verbs (run → ran), consonant doubling (cancel → cancelled), and preserves casing style.

## Anatomy of a Bundle

A bundle runs multiple generators sequentially with a **shared template engine** (shared cache). Bundle files are JSON, stored in `.tag/_bundles/`.

### Bundle format

```json
{
  "name": "fullstack",
  "generators": [
    { "name": "model" },
    { "name": "service" },
    { "name": "handler" },
    { "name": "route-inject" }
  ]
}
```

Each `name` must match a generator directory under `.tag/`.

### Running a bundle

```bash
tag generate fullstack order
# Runs: model, service, handler, route-inject — all with name="order"
```

### Self-contained bundles

A self-contained bundle stores its generators **inside the bundle directory** instead of referencing generators from root `.tag/`. This makes bundles distributable and independent.

```json
{
  "name": "examples",
  "self_contained": true,
  "generators": [
    { "name": "hello" },
    { "name": "greet" }
  ]
}
```

**Directory layout**:

```
.tag/_bundles/examples/
├── examples.json          # Bundle definition with "self_contained": true
├── _shared/               # Bundle-scoped shared templates (NOT root _shared)
│   └── header.tmpl
├── hello/
│   └── hello.go           # Generator template
└── greet/
    └── greet.go           # Generator template
```

Key differences from regular bundles:

- Generators are resolved from the bundle directory, not `.tag/`
- `_shared/` templates come from the bundle's own `_shared/`, not root
- Generator names in the bundle JSON are validated for path safety

**Creating a self-contained bundle**:

```bash
# Create the bundle with self_contained flag
tag template new bundle examples --self-contained

# Add generators inside the bundle
tag template new generator hello --in-bundle examples
tag template new generator greet --in-bundle examples

# Run it
tag generate examples myName
```

### When to use bundles vs single generators

- **Single generator**: One file operation (create a component, inject a route).
- **Bundle**: Multiple related files that should be created together (model + service + handler + route injection for a new resource).
- **Self-contained bundle**: Distributable generator packages that don't depend on project-level generators.

## Template Syntax

TAG uses **Gonja** (a Go port of Jinja2). Most Jinja2 syntax works, with some differences noted in [Pitfalls](#pitfalls--gotchas).

### Variables

```jinja2
{{ name }}                          {# Generator name argument #}
{{ vars.project_name }}             {# Variable from meta/scaffold #}
{{ vars.project_name | snake }}     {# With filter #}
```

### Filters

#### Case transforms
| Filter | Input | Output |
|--------|-------|--------|
| `snake` | `UserService` | `user_service` |
| `pascal` | `user_service` | `UserService` |
| `camel` | `user_service` | `userService` |
| `kebab` | `user_service` | `user-service` |
| `lower` | `UserService` | `userservice` |
| `upper` | `UserService` | `USERSERVICE` |
| `title` | `user service` | `User Service` |

Aliases: `snake_case`, `pascal_case`, `camel_case`, `kebab_case`

#### Inflection
| Filter | Input | Output |
|--------|-------|--------|
| `plural` / `pluralize` | `service` | `services` |
| `singular` / `singularize` | `services` | `service` |
| `past` / `past_tense` | `OrderCancel` | `OrderCancelled` |
| `humanize` | `user_service` | `User service` |
| `titleize` | `user_service` | `User Service` |
| `ordinalize` | `3` | `3rd` |

#### String operations
| Filter | Signature | Description |
|--------|-----------|-------------|
| `replace` | `replace(old, new)` | Replace all occurrences |
| `split` | `split(delim?)` | Split string (default: whitespace) |
| `join` | `join(sep?)` | Join list to string |
| `trim` | `trim(cutset?)` | Trim whitespace or cutset |
| `contains` | `contains(substr)` | Returns boolean |
| `hasprefix` | `hasprefix(prefix)` | Returns boolean |
| `hassuffix` | `hassuffix(suffix)` | Returns boolean |
| `default` | `default(value)` | Fallback if nil/empty |
| `truncate` | `truncate(len, ellipsis?)` | Truncate with "..." |

### String methods

Gonja supports Python-style method calls on strings:

```jinja2
{{ vars.name.lower() }}
{{ vars.name.upper() }}
{{ vars.name.replace("old", "new") }}
{{ vars.name.startswith("prefix") }}
```

The `replace` method supports an optional count argument: `{{ vars.name.replace("a", "b", 1) }}`.

### Control structures

```jinja2
{% if vars.use_docker %}
EXPOSE {{ vars.port }}
{% endif %}

{% for feature in vars.features %}
- {{ feature }}
{% endfor %}

{% if vars.db_type == "postgres" %}
  ...
{% elif vars.db_type == "mysql" %}
  ...
{% else %}
  ...
{% endif %}
```

### Include (shared templates)

Shared template fragments in `_shared/` can be included:

```jinja2
{% include "header.tmpl" %}
```

The include resolves by **basename** — `_shared/header.tmpl` is referenced as `"header.tmpl"`.

### Comments

```jinja2
{# This is a comment and won't appear in output #}
```

### Whitespace control

```jinja2
{%- if condition -%}    {# strips surrounding whitespace #}
{{- value -}}            {# strips surrounding whitespace #}
```

## Anatomy of a Scaffold Template

A scaffold template is a directory that creates entire projects.

### Directory structure

```
my-template/
├── tag.template.json                    # Template configuration (required)
├── .tagignore                           # Optional: exclude files from output (gitignore syntax)
├── {{ vars.project_name | snake }}/     # Templated directory names
│   ├── main.go                          # Regular files (rendered as templates)
│   ├── {{ vars.name | snake }}_test.go  # Templated file names
│   └── config.json                      # Any file type
├── _generators/                         # Optional: bundled generators
│   └── component/
│       └── component.tsx
├── hooks/                               # Optional: hook scripts
│   ├── pre_scaffold.sh
│   └── post_scaffold.py
└── README.md                            # Displayed after scaffolding
```

### tag.template.json

```json
{
  "name": "my-template",
  "description": "A Go web service template",
  "version": "1.0.0",
  "vars": {
    "project_name": "my-project",
    "author": {
      "type": "string",
      "prompt": "Who is the author?",
      "required": true
    },
    "license": {
      "type": "choice",
      "prompt": "Select a license",
      "options": ["MIT", "Apache-2.0", "GPL-3.0"],
      "default": "MIT"
    },
    "use_docker": {
      "type": "boolean",
      "prompt": "Include Docker support?",
      "default": true
    },
    "port": {
      "type": "number",
      "default": 8080
    },
    "_slug": "{{ vars.project_name | snake }}"
  },
  "hooks": {
    "pre_scaffold": ["echo 'Starting...'"],
    "post_scaffold": ["go mod tidy", "git init"]
  }
}
```

### Variable definition formats

**Short form** — Just a default value:

```json
{
  "vars": {
    "project_name": "my-project",
    "use_docker": true,
    "port": 8080
  }
}
```

**Long form** — Full definition:

```json
{
  "vars": {
    "author": {
      "type": "string",
      "prompt": "Who is the author?",
      "default": "Anonymous",
      "required": true,
      "secret": false
    }
  }
}
```

### Variable types

| Type | JSON type | Prompt behavior |
|------|-----------|-----------------|
| `string` | `"value"` | Text input |
| `boolean` | `true`/`false` | Yes/No confirmation |
| `number` | `123` | Numeric input |
| `choice` | requires `options` | Selection from list |

### Special variable kinds

**Private variables** — Prefixed with `_`. Not prompted, for internal use:

```json
{
  "_internal_slug": "{{ vars.project_name | snake }}"
}
```

**Derived variables** — Default contains `{{ vars.* }}`. Not prompted, computed from other variables:

```json
{
  "display_name": "My Package",
  "package_name": "{{ vars.display_name | lower | replace(' ', '_') }}"
}
```

Only `display_name` is prompted. `package_name` is computed automatically.

### Variable resolution priority

From lowest to highest priority:

1. Default values from `tag.template.json`
2. Replay values (`--replay`)
3. Values file (`--values values.json`)
4. Interactive prompts (if TTY)
5. `--meta` / `-m` flag values (highest priority)

### Path placeholders

Directory and file names can contain template expressions:

```
{{ vars.project_name | snake }}/
  {{ vars.project_name | snake }}_test.go
```

These are rendered using the collected variables before files are written.

### .tagignore

A `.tagignore` file in the template root excludes files and directories from scaffold output using gitignore-style patterns. This is useful for excluding template-authoring tools (IDE configs, AI assistant files, etc.) that shouldn't appear in generated projects.

```
# .tagignore — exclude authoring tools from output
.serena/
CLAUDE.md
.mcp.json
*.log
docs/internal/
```

Rules:
- Standard gitignore syntax (globs, `**/`, negation with `!`, `#` comments)
- `.tagignore` itself is always excluded from output (like `tag.template.json`)
- Directories matching a pattern are pruned entirely (children are not traversed)
- Missing or empty `.tagignore` has no effect — all files are included as usual

### Bundled generators

Templates can include generators in `_generators/` that get copied to the scaffolded project's `.tag/` directory. These are available for code generation after scaffolding.

## Hooks

### Phases

| Phase | Working directory | When | Failure behavior |
|-------|-------------------|------|------------------|
| `pre_scaffold` | Template directory | Before file generation | Fatal — stops scaffold |
| `post_scaffold` | Output directory | After file generation | Warning — scaffold already done |
| `pre_generate` | Project directory | Before code generation | Fatal |
| `post_generate` | Project directory | After code generation | Fatal |

### Environment variables

All hooks receive these environment variables:

| Variable | Description |
|----------|-------------|
| `TAG_TEMPLATE_DIR` | Absolute path to template directory |
| `TAG_OUTPUT_DIR` | Absolute path to project root directory |
| `TAG_PROJECT_NAME` | Value of `project_name` variable |
| `TAG_VAR_<NAME>` | Each variable as `TAG_VAR_` + UPPER_SNAKE name |

Example: variable `project_name` → `TAG_VAR_PROJECT_NAME`

### Hook execution

- Hook commands support `{{ vars.* }}` template expressions (rendered before execution)
- Commands are parsed using POSIX shell quoting rules (shlex)
- No shell interpretation by default — pipes, redirects, `$VAR` expansion don't work
- For shell features, explicitly invoke a shell: `sh -c 'echo hello | grep hello'`
- Script files are auto-detected by extension (`.py` → python, `.sh` → sh, `.rb` → ruby)
- Script files with shebangs are executed directly if they have the executable bit
- 5-minute timeout per command, 1MB output limit

### Hook confirmation

- Interactive mode: User is prompted to confirm hooks
- `--accept-hooks`: Run hooks without prompting
- `--no-input` without `--accept-hooks`: Hooks are skipped

## Remote Templates & Library

### Remote reference formats

| Format | Example |
|--------|---------|
| GitHub shorthand | `gh:user/repo` |
| GitLab shorthand | `gl:user/repo` |
| Bitbucket shorthand | `bb:user/repo` |
| With version | `gh:user/repo@v1.0.0` |
| With subpath | `gh:user/repo/templates/go-api` |
| Version + subpath | `gh:user/repo@v2.0.0/templates/go-api` |
| HTTPS URL | `https://github.com/user/repo.git` |
| SSH URL | `git@github.com:user/repo.git` |
| Zip URL | `https://example.com/template.zip` |
| Local path | `./my-template` or `/absolute/path` |

### Authentication

| Provider | Environment variable | Auth method |
|----------|---------------------|-------------|
| GitHub | `GITHUB_TOKEN` | Basic auth (`x-access-token`) |
| GitLab | `GITLAB_TOKEN` | Basic auth (`x-access-token`) |
| Bitbucket | `BITBUCKET_TOKEN` | Bearer token |
| SSH | SSH agent or default keys | SSH key |

### Template library

The library stores templates locally for quick access.

```bash
# Add from remote
tag lib add gh:user/go-api-template
tag lib add gh:user/template --as my-name     # custom name
tag lib add gh:user/template --force           # overwrite existing

# List installed
tag lib ls

# Use a library template
tag scaffold go-api-template my-project

# Interactive picker (no args = fuzzy search)
tag scaffold

# Manage
tag template info my-template   # show details + variables
tag lib edit my-template        # open in editor
tag lib update my-template      # re-fetch from source
tag lib update                  # update all
tag lib rm my-template          # remove
```

Cookiecutter templates are **auto-detected and converted** when added to the library.

## CLI Quick Reference

### Essential commands

| Command | Description |
|---------|-------------|
| `tag scaffold [template] [project-name]` | Create project from template (no args = picker) |
| `tag generate <gen-or-bundle> <name>` | Run a generator or bundle (auto-resolved) |
| `tag generate list` | List available generators and bundles |
| `tag template init` | Initialize `.tag/` directory structure |
| `tag template new generator <name>` | Create a new generator (`--in-bundle`, `--lib`) |
| `tag template new bundle <name>` | Create a new bundle (`--self-contained`, `--lib`) |
| `tag template info <template>` | Show template info without scaffolding |
| `tag template list` | List available generators and bundles |
| `tag convert cookiecutter <src> -o <dst>` | Convert Cookiecutter template |
| `tag lib add <ref>` | Install template to library |
| `tag lib ls` | List installed templates |
| `tag lib rm <name>` | Remove template from library |
| `tag lib update [name]` | Update template(s) from source |
| `tag lib edit <name>` | Open template in editor |
| `tag version [--check]` | Print version, check for updates |

### Common flags

| Flag | Commands | Description |
|------|----------|-------------|
| `-m key=value` / `--meta` | scaffold, generate | Set variable values |
| `--values <file>` | scaffold | Load variables from JSON file |
| `--no-input` | scaffold | Skip interactive prompts |
| `--replay` | scaffold | Reuse previously saved inputs |
| `--no-save` | scaffold | Don't save inputs for replay |
| `-o <dir>` / `--output` | scaffold | Output directory |
| `--force` | scaffold | Overwrite existing output |
| `--accept-hooks` | scaffold | Run hooks without prompting |
| `-l` / `--lib` | template new generator/bundle | Target library template |
| `-B` / `--in-bundle` | template new generator | Create generator inside a bundle directory |
| `-s` / `--self-contained` | template new bundle | Create bundle with `self_contained: true` |
| `--update` / `-u` | scaffold, template info | Force refresh of cached remote templates |
| `--dry-run` / `-d` | generate, convert | Preview without writing files |

## Pitfalls & Gotchas

### Gonja is NOT Jinja2

TAG uses Gonja, a Go port of Jinja2. Most syntax works identically, but there are differences:

- **No `do` tag**: Gonja doesn't support `{% do %}` for side-effect-only expressions.
- **Filter arguments**: Use `filter(arg1, arg2)` syntax. Keyword arguments may not work for all filters.
- **Custom filters only**: TAG registers its own filter set. Standard Jinja2 filters like `tojson`, `wordwrap`, `batch` are NOT available unless TAG explicitly registers them.
- **No `loop.changed`**: Some advanced loop variables may not be available.

### Namespace rules

- **Generators**: `{{ name }}` is the CLI argument. `{{ vars.x }}` are meta values.
- **Scaffold templates**: Only `{{ vars.x }}` is used. There is no `{{ name }}`.
- **Never use bare variable names**: `{{ project_name }}` does NOT work. Always use `{{ vars.project_name }}`.

### Frontmatter is NOT YAML

Generator frontmatter uses a simple `key: value` line parser:
- No nested objects
- No arrays
- No multi-line values
- Comments start with `#` but must be on their own line
- Values can contain colons (only the first `:` is the separator)

### File extensions for generators

Generator files keep their **natural extension** (`.go`, `.ts`, `.py`). Do NOT use `.tmpl`. All files in a generator directory are treated as templates regardless of extension.

### Derived variable ordering

Derived variables reference other variables via `{{ vars.x }}`. If variable A depends on variable B, B must be defined (and prompted) first. TAG processes variables in the order they appear in `tag.template.json`.

### Empty output from generators

If your template body renders to an empty string (e.g., all content is inside a false `{% if %}` block), the file is still created — as an empty file. There's no built-in "skip if empty" mechanism.

### Scaffold vs Generate variables

- **Scaffold**: Variables come from `tag.template.json` and are collected via prompts/flags.
- **Generate**: Variables come from `--meta` flags and scaffold-time `.tagconfig.json` (`ScaffoldVars`).
- They use different resolution paths but both end up in the `{{ vars.x }}` namespace.

### Path safety

- Paths in `to:` fields are validated against path traversal (`../`)
- Remote template owner/repo names are validated against path injection
- Template directory paths are resolved to absolute paths before use
