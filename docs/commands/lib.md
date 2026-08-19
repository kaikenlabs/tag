# tag lib

Manage the template library.

## Synopsis

```bash
tag lib <subcommand> [args] [flags]
```

Alias: `tag library`

## Description

The `lib` command manages a persistent local library of project templates. Templates added to the library are stored locally and can be used with `tag scaffold` for quick scaffolding (including the interactive picker when `tag scaffold` is run with no arguments).

Cookiecutter templates are auto-detected and converted to TAG format when added.

## Subcommands

### `tag lib add`

Add a template to the library.

```bash
tag lib add <ref> [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--as <name>` | | Override the template name in the library |
| `--force` | `-f` | Overwrite existing template with same name |

**Examples:**

```bash
# Add from GitHub
tag lib add gh:user/go-api-template

# Add with a custom name
tag lib add gh:user/cookiecutter-django --as django

# Overwrite existing
tag lib add gh:user/template --force

# Add from GitLab
tag lib add gl:org/service-template

# Add from a local directory
tag lib add ./my-template --as my-local
```

When adding a Cookiecutter template, TAG auto-detects `cookiecutter.json` and converts it to TAG format automatically.

---

### `tag lib ls`

List installed templates.

```bash
tag lib ls [flags]
```

Alias: `tag lib list`

| Flag | Description |
|------|-------------|
| `--format <fmt>` | Output format: `text` (default) or `json` |

**Example output:**

```
NAME                 SOURCE                         VERSION    DESCRIPTION
----                 ------                         -------    -----------
go-api               gh:user/go-api-template        v1.2.0     Go REST API template
django               gh:user/cookiecutter-django     -          Django web app
react-app            gl:org/react-starter           v3.0.0     React SPA template
```

**Machine-readable output:**

```bash
tag lib ls --format json
```

```json
{
  "templates": [
    { "name": "go-api", "source": "gh:user/go-api-template", "version": "v1.2.0", "added_at": "...", "updated_at": "..." }
  ]
}
```

An empty library serializes as `"templates": []`, never `null`.

---

### `tag lib search`

Search GitHub for templates tagged `tag-template`.

```bash
tag lib search [query] [flags]
```

The query is variadic — all non-flag words are joined with spaces. Leave it empty to list all `tag-template`-tagged repositories.

| Flag | Default | Description |
|------|---------|-------------|
| `--limit <n>` | `10` | Maximum number of results (1-100) |
| `--sort <field>` | `stars` | Sort by: `stars`, `forks`, or `updated` |
| `--order <dir>` | `desc` | Order: `asc` or `desc` |
| `--format <fmt>` | `text` | Output format: `text` (default) or `json` |

Flags may appear before or after the query: `tag lib search kubernetes --limit 5` treats `--limit` as a flag, not as part of the query text.

**Examples:**

```bash
# Search by keyword
tag lib search kubernetes

# Limit and sort results
tag lib search go --limit 5 --sort updated --order asc

# Machine-readable output
tag lib search go --format json
```

```json
{
  "results": [
    { "name": "go-api", "full_name": "acme/go-api", "description": "...", "url": "...", "stars": 142, "updated_at": "...", "language": "Go" }
  ]
}
```

No matches serialize as `"results": []`, never `null`.

**Passing a literal dash-prefixed query term:** an unrecognised `-`-prefixed token is rejected as an unknown flag rather than folded into the query. Put it after another argument and a `--` separator:

```bash
tag lib search go -- -language:java
```

A leading `--` (`tag lib search -- -x`) does not work — urfave/cli consumes it before TAG sees it.

---

### `tag lib rm`

Remove a template from the library.

```bash
tag lib rm <name>
```

Alias: `tag lib remove`

**Example:**

```bash
tag lib rm old-template
```

---

### `tag lib update`

Update a template (or all templates) from the original source.

```bash
# Update a specific template
tag lib update <name>

# Update all templates
tag lib update
```

Re-fetches the template from its original source. Cookiecutter templates are re-converted automatically.

**Examples:**

```bash
# Update a single template
tag lib update go-api

# Update all installed templates
tag lib update
```

---

### `tag lib edit`

Open a template in your editor for local customization.

```bash
tag lib edit <name> [flags]
```

| Flag | Description |
|------|-------------|
| `--editor <cmd>` | Editor command to use (e.g., `code`, `vim`) |

Editor resolution order:
1. `--editor` flag
2. TAG global config (saved from previous prompt)
3. `$VISUAL` environment variable
4. `$EDITOR` environment variable
5. Interactive prompt (TTY only, saves choice for future)

**Examples:**

```bash
# Open in default editor
tag lib edit go-api

# Open in VS Code
tag lib edit go-api --editor "code --wait"

# Open in vim
tag lib edit go-api --editor vim
```

## Inspecting Templates

To view detailed information about a library template (variables, hooks, metadata), use `tag template info`:

```bash
tag template info go-api
```

See [tag template info](template.md#tag-template-info) for details.

## Storage

Templates are stored in the XDG data directory:

- **Linux**: `~/.local/share/tag/templates/`
- **macOS**: `~/Library/Application Support/tag/templates/`

A registry file tracks template metadata (source, version, timestamps).

## Workflow Example

```bash
# 1. Add templates to your library
tag lib add gh:company/go-service
tag lib add gh:company/react-frontend --as react

# 2. List available templates
tag lib ls

# 3. Scaffold new projects (by name or interactive picker)
tag scaffold go-service my-api
tag scaffold    # interactive picker

# 4. Generators from the template are available in scaffolded projects
cd my-api
tag generate list     # Shows generators from go-service template
tag generate handler users

# 5. Keep templates up to date
tag lib update
```

## See Also

- [tag scaffold](scaffold.md) - Scaffold from any template source
- [Template Command](template.md) - Template management (new, init, info, list)
- [Remote References](../reference/remote-refs.md) - Template source formats
