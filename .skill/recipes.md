# TAG Recipes

Real-world patterns and examples for TAG template authoring.

## Recipe 1: CRUD Generator Bundle

Generate model, repository, handler, and route injection for a new resource.

**Generators** (each in `.tag/<name>/<name>.go`):

**Model** — `.tag/model/model.go`:
```
---
to: internal/models/{{ name | snake }}.go
desc: Create a database model struct
---
package models

import "time"

type {{ name | pascal }} struct {
	ID        int64     `json:"id" db:"id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
```

**Repository** — `.tag/repository/repository.go`:
```
---
to: internal/repository/{{ name | snake }}_repository.go
desc: Create a repository with FindByID
---
package repository

import (
	"context"
	"database/sql"
	"myapp/internal/models"
)

type {{ name | pascal }}Repository struct {
	db *sql.DB
}

func New{{ name | pascal }}Repository(db *sql.DB) *{{ name | pascal }}Repository {
	return &{{ name | pascal }}Repository{db: db}
}

func (r *{{ name | pascal }}Repository) FindByID(ctx context.Context, id int64) (*models.{{ name | pascal }}, error) {
	var m models.{{ name | pascal }}
	err := r.db.QueryRowContext(ctx, "SELECT id, created_at, updated_at FROM {{ name | snake | plural }} WHERE id = $1", id).
		Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
```

**Handler** — `.tag/handler/handler.go`:
```
---
to: internal/handler/{{ name | snake }}_handler.go
desc: Create an HTTP handler
---
package handler

import (
	"encoding/json"
	"net/http"
	"myapp/internal/repository"
)

type {{ name | pascal }}Handler struct {
	repo *repository.{{ name | pascal }}Repository
}

func New{{ name | pascal }}Handler(repo *repository.{{ name | pascal }}Repository) *{{ name | pascal }}Handler {
	return &{{ name | pascal }}Handler{repo: repo}
}

func (h *{{ name | pascal }}Handler) Get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"resource": "{{ name | snake }}"})
}
```

**Route injection** — `.tag/route-inject/route-inject.go`:
```
---
to: internal/router/router.go
inject: true
after: "// ROUTES"
desc: Inject route registration
---
	{{ name | camel }}Handler := handler.New{{ name | pascal }}Handler({{ name | camel }}Repo)
	r.Get("/{{ name | kebab | plural }}", {{ name | camel }}Handler.Get)
```

**Bundle** — `.tag/_bundles/resource.json`:
```json
{
  "name": "resource",
  "generators": [
    { "name": "model" },
    { "name": "repository" },
    { "name": "handler" },
    { "name": "route-inject" }
  ]
}
```

**Usage**: `tag generate resource product` creates all 4 files.

---

## Recipe 2: React Component Generator

Multi-file generator creating component, styles, test, and barrel export.

**Component** — `.tag/component/component.tsx`:
```
---
to: src/components/{{ name | pascal }}/{{ name | pascal }}.tsx
---
import React from 'react';
import styles from './{{ name | pascal }}.module.css';

interface {{ name | pascal }}Props {
  children?: React.ReactNode;
}

export const {{ name | pascal }}: React.FC<{{ name | pascal }}Props> = ({ children }) => {
  return <div className={styles.container}>{children}</div>;
};

export default {{ name | pascal }};
```

**Styles** — `.tag/component/styles.tsx`:
```
---
to: src/components/{{ name | pascal }}/{{ name | pascal }}.module.css
---
.container {
  /* {{ name | pascal }} styles */
}
```

**Test** — `.tag/component/test.tsx`:
```
---
to: src/components/{{ name | pascal }}/{{ name | pascal }}.test.tsx
---
import { render, screen } from '@testing-library/react';
import { {{ name | pascal }} } from './{{ name | pascal }}';

describe('{{ name | pascal }}', () => {
  it('renders children', () => {
    render(<{{ name | pascal }}>Hello</{{ name | pascal }}>);
    expect(screen.getByText('Hello')).toBeInTheDocument();
  });
});
```

**Index** — `.tag/component/index.tsx`:
```
---
to: src/components/{{ name | pascal }}/index.ts
---
export { {{ name | pascal }} } from './{{ name | pascal }}';
export type { {{ name | pascal }}Props } from './{{ name | pascal }}';
```

**Usage**: `tag generate component sidebar` creates the full component directory.

---

## Recipe 3: Inject Patterns

### Registry injection

Pre-existing file with marker:
```go
var handlers = map[string]Handler{
	// HANDLERS
}
```

Generator injects before marker:
```
---
to: internal/registry/registry.go
inject: true
before: "// HANDLERS"
---
	"{{ name | kebab }}": New{{ name | pascal }}Handler(),
```

### Import injection

```
---
to: internal/registry/registry.go
inject: true
after: "import ("
---
	"myapp/internal/handler/{{ name | snake }}"
```

### Multi-point injection

One generator directory, multiple files targeting different injection points:
```
.tag/feature/
├── create.go        # Creates the main file
├── inject-route.go  # Injects route registration
└── inject-import.go # Injects import statement
```

---

## Recipe 4: Meta Variables for Customization

Override defaults with `--meta` flags:

```
---
to: {{ vars.package | default("internal/services") }}/{{ name | snake }}.go
---
package {{ vars.package | default("services") | split("/") | last }}

type {{ name | pascal }}Service struct{}
```

```bash
tag generate service auth                       # → internal/services/auth.go
tag generate service auth -m package=pkg/svc    # → pkg/svc/auth.go
```

---

## Recipe 5: Shared Template Fragments

`.tag/_shared/header.tmpl`:
```
// Code generated by TAG. DO NOT EDIT.
// Source: {{ name }}
```

Used in generators:
```
---
to: internal/services/{{ name | snake }}.go
---
{% include "header.tmpl" %}

package services

type {{ name | pascal }}Service struct{}
```

`{% include %}` resolves by **basename**.

---

## Recipe 6: Go Service Scaffold Template

Full scaffold with variables, conditionals, hooks, and bundled generators.

**tag.template.json**:
```json
{
  "name": "go-service",
  "description": "Go microservice with HTTP server",
  "version": "1.0.0",
  "vars": {
    "project_name": "my-service",
    "go_module": {
      "type": "string",
      "prompt": "Go module path",
      "default": "github.com/myorg/{{ vars.project_name | kebab }}",
      "required": true
    },
    "port": { "type": "number", "default": 8080 },
    "use_docker": { "type": "boolean", "default": true },
    "_slug": "{{ vars.project_name | snake }}"
  },
  "hooks": {
    "post_scaffold": [
      "cd {{ vars.project_name | snake }} && go mod tidy",
      "cd {{ vars.project_name | snake }} && git init"
    ]
  }
}
```

**Conditional file** — `{{ vars.project_name | snake }}/Dockerfile`:
```dockerfile
{% if vars.use_docker %}FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM alpine:3.19
COPY --from=builder /server /server
EXPOSE {{ vars.port }}
CMD ["/server"]
{% endif %}
```

**Bundled generator** — `_generators/endpoint/endpoint.go`:
```
---
to: internal/handler/{{ name | snake }}.go
---
package handler

import "net/http"

func {{ name | pascal }}Handler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
```

**Usage**:
```bash
tag lib add ./go-service-template
tag scaffold go-service my-api
cd my-api && tag generate endpoint users
```

---

## Recipe 7: Self-Contained Bundles

Distributable bundle with generators inside:

```bash
tag template new bundle examples --self-contained
tag template new generator hello --in-bundle examples
tag template new generator greet --in-bundle examples
```

Creates:
```
.tag/_bundles/examples/
├── examples.json         # {"self_contained": true, ...}
├── _shared/              # Bundle-scoped shared templates
├── hello/hello.go
└── greet/greet.go
```

`tag generate examples world` runs all generators with `name="world"`.

Works with `--lib` to add bundles to library templates.

---

## Recipe 8: Converting Cookiecutter Templates

```bash
# Direct scaffold (auto-detects and converts on the fly)
tag scaffold gh:user/cookiecutter-django my-project

# Explicit conversion
tag convert cookiecutter ./cc-template -o ./tag-template

# Add to library (auto-converts)
tag lib add gh:user/cookiecutter-flask --as flask
```

Conversion maps: `cookiecutter.json` → `tag.template.json`, `{{ cookiecutter.var }}` → `{{ vars.var }}`.

---

## Recipe 9: Non-Interactive / CI Scaffolding

```bash
# All variables via flags
tag scaffold gh:user/template my-project \
  --no-input --accept-hooks \
  -m project_name=my-project \
  -m author="CI Bot" \
  -m license=MIT

# Or via values file
tag scaffold gh:user/template my-project \
  --no-input --accept-hooks \
  --values values.json

# Replay previous inputs
tag scaffold gh:user/template another-project --replay
```

---

## Recipe 10: .tagignore for Template Authoring

Exclude development artifacts from scaffolded output:

```
# AI assistants
.serena/
CLAUDE.md
.mcp.json
.cursor/

# IDE
.vscode/
.idea/

# Dev artifacts
*.log
tmp/
docs/internal/
```

Template layout:
```
my-template/
├── tag.template.json
├── .tagignore              ← Controls exclusions
├── .serena/                ← NOT copied to output
├── CLAUDE.md               ← NOT copied to output
├── {{ vars.project_name }}/
│   ├── main.go
│   └── README.md
└── _generators/
    └── handler/handler.go
```

---

## Recipe 11: Evaluated-Default Variables

Offer a smart, context-aware default while still letting the user change it.
Use the **expanded form** with an explicit `prompt` and a template-expression `default`.

**tag.template.json**:
```json
{
  "vars": {
    "project_name": "my-service",
    "module_path": {
      "type": "string",
      "prompt": "Go module path",
      "default": "bitbucket.org/myorg/{{ vars.project_name | kebab }}"
    },
    "docker_registry": {
      "type": "string",
      "prompt": "Docker image",
      "default": "registry.myorg.com/{{ vars.project_name | kebab }}"
    },
    "_slug": "{{ vars.project_name | snake }}"
  }
}
```

**User experience** (user accepts all suggested defaults):
```
Enter value for project_name [my-service]: ⏎
Go module path [bitbucket.org/myorg/my-service]: ⏎
Docker image [registry.myorg.com/my-service]: ⏎
```

**With user override**:
```
Enter value for project_name [my-service]: payments-api
Go module path [bitbucket.org/myorg/payments-api]: github.com/acme/payments-api
Docker image [registry.myorg.com/payments-api]: ⏎
```

**Disambiguation** — the `prompt` field is the switch:

| Definition | Prompted? | Behaviour |
|---|---|---|
| `"slug": "{{ vars.x }}"` | No | Derived — silent |
| `{"prompt": "…", "default": "{{ vars.x }}"}` | Yes | Evaluated default — prompted with resolved suggestion |
| `"_internal": "{{ vars.x }}"` | No | Private — silent |

In non-TTY / `--no-input` mode, all three resolve silently.
