# Add Generators to an Existing Project

In this tutorial you'll add TAG generators to an existing project. You'll create an endpoint generator with create and inject actions, group it into a bundle, and learn the dry-run and undo workflows.

You can start from a freshly scaffolded project:

```bash
tag scaffold ./examples/weather-api-go my-weather-app
cd my-weather-app
```

Or use any existing project — the steps are the same.

## 1. Initialize TAG

Set up the `.tag/` directory structure:

```bash
tag template init
```

This creates:

```
.tag/
├── _shared/
└── _bundles/
```

It also creates `.tagconfig.json` at the project root if one doesn't already exist.

## 2. Create an Endpoint Generator

Use the scaffolding command to create a generator:

```bash
tag template new generator endpoint
```

This creates `.tag/endpoint/` with a starter template file. Open `.tag/endpoint/endpoint.go` (or the file matching your project language) and replace its contents.

### Create Action — Handler File

Create `.tag/endpoint/handler.go`:

```
---
to: {{ name | snake }}.go
desc: Create a new endpoint handler
---
package main

import (
	"encoding/json"
	"net/http"
)

type {{ name | pascal }}Data struct {
	City   string `json:"city"`
	{{ name | pascal }} string `json:"{{ name | snake }}"`
}

func {{ name | camel }}Handler(w http.ResponseWriter, r *http.Request) {
	city := r.PathValue("city")
	data := {{ name | pascal }}Data{City: city, {{ name | pascal }}: "placeholder"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
```

### Inject Action — Route Registration

Create `.tag/endpoint/register_route.go`:

```
---
to: main.go
inject: true
after: "// ROUTES"
desc: Register the new endpoint route
---
	mux.HandleFunc("GET /{{ name | kebab }}/{city}", {{ name | camel }}Handler)
```

This injects a line into `main.go` after the `// ROUTES` marker.

> **Note:** Your project needs a `// ROUTES` comment in `main.go` for injection to work. The marker is just a plain comment — TAG searches for an exact line match.

Now run it:

```bash
tag generate endpoint humidity
```

TAG creates `humidity.go` and injects the route into `main.go`.

## 3. Use Meta Variables

Pass extra variables with `--meta` / `-m`:

```bash
tag generate endpoint air-quality -m version=v2
```

Access them in generator templates as `{{ vars.version }}`. You can set defaults in the generator's `tag.template.json`:

```json
{
  "vars": {
    "version": {
      "type": "string",
      "prompt": "API version prefix",
      "default": "v1"
    }
  }
}
```

Then use them in frontmatter or body:

```
---
to: {{ vars.version }}/{{ name | snake }}.go
---
```

The `{{ vars.* | default("fallback") }}` filter provides inline defaults without a config file.

## 4. Create a Bundle with Prerequisites

Bundles group generators to run together. Create `.tag/_bundles/full-endpoint.json`:

```json
{
  "name": "full-endpoint",
  "generators": [
    { "name": "endpoint" }
  ]
}
```

Run the bundle:

```bash
tag generate full-endpoint forecast
```

### Adding Prerequisites

If a generator only makes sense when a feature is enabled, add `requires`:

```json
{
  "name": "docker-endpoint",
  "requires": ["use_docker"],
  "generators": [
    { "name": "endpoint" }
  ]
}
```

The `use_docker` value comes from `.tagconfig.json`. If the requirement is unmet, `tag generate` aborts with a message.

`tag generate list` hides items with unmet requirements by default. Use `--all` to see everything:

```bash
tag generate list        # shows only available generators/bundles
tag generate list --all  # shows all, including those with unmet requirements
```

## 5. Add Generate Hooks

Add pre/post hooks in `.tagconfig.json`:

```json
{
  "hooks": {
    "pre_generate": ["echo 'Generating...'"],
    "post_generate": ["go fmt ./..."]
  }
}
```

- `pre_generate` runs before any files are written. If it fails, generation is aborted.
- `post_generate` runs after all files are written. If it fails, generation is aborted (but files are already written — use `tag undo` to revert).

Skip hooks with `--no-hooks`:

```bash
tag generate endpoint test --no-hooks
```

## 6. Dry-Run Workflow

Preview changes before writing anything:

```bash
tag generate endpoint forecast --dry-run
```

This renders all templates and shows a colored unified diff for each file:

- Green `+` lines show additions
- Red `-` lines show deletions (for inject/append operations)

On a TTY, each diff is followed by a prompt:

```
Apply this change? [y]es/[n]o/[a]ll/[q]uit
```

- `y` — move to next file
- `n` — skip this file
- `a` — show all remaining diffs without prompting
- `q` — exit immediately

Nothing is written to disk regardless of your answers. Hooks are not executed.

> **Tip:** Dry-run is useful in code review workflows — pipe it to a file for comparison: `tag generate endpoint forecast --dry-run 2>&1 | tee preview.diff`

## 7. Undo

Every `tag generate` records a manifest in `.tag/history.json`. Undo reverts the last generation:

```bash
# Revert the last generation (with confirmation)
tag undo

# Skip the confirmation prompt
tag undo --yes

# List generation history
tag undo --list

# Revert a specific generation by ID
tag undo --id gen_1741000000_a3f2bc
```

### Conflict Detection

If you modify a generated file after generation, `tag undo` refuses to overwrite it:

```
conflict: humidity.go was modified after generation
hint: use --force to override or --partial to skip conflicting files
```

Options:

- `--force` — override conflict detection, revert everything
- `--partial` — revert unmodified files, skip conflicting ones

## Summary

| Step | Command |
|------|---------|
| Initialize | `tag template init` |
| Create generator | `tag template new generator <name>` |
| Run generator | `tag generate <name> <target>` |
| Create bundle | Create `.tag/_bundles/<name>.json` |
| Run bundle | `tag generate <bundle> <target>` |
| Preview changes | `tag generate <name> <target> --dry-run` |
| Undo last generation | `tag undo` |
| View history | `tag undo --list` |

## See Also

- [Generate Command](../commands/generate.md) — full command reference
- [Undo Command](../commands/undo.md) — undo reference
- [Hooks Guide](../templates/hooks.md) — hook configuration
- [Build a Scaffold Template](01-scaffold-template.md) — previous tutorial
