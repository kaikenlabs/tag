# TAG Recipes

Advanced patterns and real-world examples for TAG.

## Recipe 1: CRUD Generator Bundle

Create a bundle that generates model, repository, service, and handler files plus route injection for a new resource.

### Step 1: Initialize

```bash
tag init
```

### Step 2: Create generators

**Model** — `.tag/model/model.go`:

```
---
to: internal/models/{{ name | snake }}.go
---
package models

import "time"

// {{ name | pascal }} represents a {{ name | humanize | lower }} entity.
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
---
	{{ name | camel }}Handler := handler.New{{ name | pascal }}Handler({{ name | camel }}Repo)
	r.Get("/{{ name | kebab | plural }}", {{ name | camel }}Handler.Get)
```

### Step 3: Create bundle

`.tag/_bundles/resource.json`:

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

### Step 4: Use it

```bash
tag generate resource product
```

Creates:
- `internal/models/product.go`
- `internal/repository/product_repository.go`
- `internal/handler/product_handler.go`
- Injects route into `internal/router/router.go`

---

## Recipe 2: React Component Generator

A generator that creates a React component with its styles and test file.

### Component file — `.tag/component/component.tsx`:

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
  return (
    <div className={styles.container}>
      {children}
    </div>
  );
};

export default {{ name | pascal }};
```

### Styles — `.tag/component/styles.tsx`:

```
---
to: src/components/{{ name | pascal }}/{{ name | pascal }}.module.css
---
.container {
  /* {{ name | pascal }} styles */
}
```

### Test — `.tag/component/test.tsx`:

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

### Index barrel — `.tag/component/index.tsx`:

```
---
to: src/components/{{ name | pascal }}/index.ts
---
export { {{ name | pascal }} } from './{{ name | pascal }}';
export type { {{ name | pascal }}Props } from './{{ name | pascal }}';
```

### Usage

```bash
tag generate component sidebar
# Creates: src/components/Sidebar/Sidebar.tsx, .module.css, .test.tsx, index.ts
```

---

## Recipe 3: Go Service Scaffold Template

A full scaffold template for creating Go microservices.

### Directory layout

```
go-service-template/
├── tag.template.json
├── {{ vars.project_name | snake }}/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go
│   │   └── handler/
│   │       └── health.go
│   ├── go.mod
│   ├── Makefile
│   ├── Dockerfile
│   └── README.md
├── _generators/
│   └── endpoint/
│       └── endpoint.go
└── hooks/
    └── post_scaffold.sh
```

### tag.template.json

```json
{
  "name": "go-service",
  "description": "Go microservice with HTTP server",
  "version": "1.0.0",
  "vars": {
    "project_name": "my-service",
    "go_module": {
      "type": "string",
      "prompt": "Go module path (e.g., github.com/user/repo)",
      "required": true
    },
    "port": {
      "type": "number",
      "prompt": "HTTP port",
      "default": 8080
    },
    "use_docker": {
      "type": "boolean",
      "prompt": "Include Dockerfile?",
      "default": true
    },
    "_slug": "{{ vars.project_name | snake }}"
  },
  "hooks": {
    "post_scaffold": [
      "go mod tidy",
      "git init"
    ]
  }
}
```

### Template file example — `{{ vars.project_name | snake }}/go.mod`:

```
module {{ vars.go_module }}

go 1.23
```

### Template file — `{{ vars.project_name | snake }}/cmd/server/main.go`:

```go
package main

import (
	"fmt"
	"log"
	"net/http"

	"{{ vars.go_module }}/internal/handler"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.Health)

	addr := fmt.Sprintf(":%d", {{ vars.port }})
	log.Printf("Starting {{ vars.project_name }} on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
```

### Conditional file — `{{ vars.project_name | snake }}/Dockerfile`:

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

### Bundled generator — `_generators/endpoint/endpoint.go`:

```
---
to: internal/handler/{{ name | snake }}.go
---
package handler

import "net/http"

// {{ name | pascal }}Handler handles {{ name | humanize | lower }} requests.
func {{ name | pascal }}Handler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
```

### Usage

```bash
# Install to library
tag lib add ./go-service-template

# Scaffold
tag run go-service my-api

# Later, add endpoints using the bundled generator
cd my-api
tag generate endpoint users
```

---

## Recipe 4: Adding Generators to an Existing Project

You can add generators to any project, even one not created by TAG.

```bash
# In your project root
tag init                      # Creates .tag/ directory
tag new service               # Creates .tag/service/service.go

# Edit the generator
# Then use it
tag generate service payment
```

### Working with library templates

If your project was scaffolded from a library template, generators can target the library template instead of the local `.tag/`:

```bash
# Create a generator in the library template
tag new endpoint --lib

# This creates the generator inside the library template's .tag/ directory,
# making it available to all projects scaffolded from that template.
```

---

## Recipe 5: Shared Template Fragments

Use `_shared/` for reusable template fragments across generators.

### Create shared fragments

`.tag/_shared/header.tmpl`:

```
// Code generated by TAG. DO NOT EDIT.
// Source: {{ name }}
```

`.tag/_shared/license.tmpl`:

```
// Copyright {{ vars.year }} {{ vars.author }}. All rights reserved.
// Licensed under the {{ vars.license }} License.
```

### Use in generators

`.tag/service/service.go`:

```
---
to: internal/services/{{ name | snake }}.go
---
{% include "header.tmpl" %}

package services

type {{ name | pascal }}Service struct{}
```

The `{% include %}` directive resolves by basename, so `_shared/header.tmpl` is referenced as `"header.tmpl"`.

---

## Recipe 6: Inject-Based Patterns

### Registry pattern

Keep a central registry file and inject new entries:

**Pre-existing file** — `internal/registry/registry.go`:

```go
package registry

var handlers = map[string]Handler{
	// HANDLERS
}
```

**Generator** — `.tag/register/register.go`:

```
---
to: internal/registry/registry.go
inject: true
before: "// HANDLERS"
---
	"{{ name | kebab }}": New{{ name | pascal }}Handler(),
```

### Import injection

**Generator** — `.tag/register/imports.go`:

```
---
to: internal/registry/registry.go
inject: true
after: "import ("
---
	"myapp/internal/handler/{{ name | snake }}"
```

### Multiple injections in one generator

A generator directory can have multiple files, each targeting different injection points:

```
.tag/feature/
├── create.go        # Creates the main feature file
├── inject-route.go  # Injects route registration
└── inject-import.go # Injects import statement
```

---

## Recipe 7: Meta Variables for Customization

Use `--meta` flags to pass additional context to generators:

### Generator with package customization

`.tag/service/service.go`:

```
---
to: {{ vars.package | default("internal/services") }}/{{ name | snake }}.go
---
package {{ vars.package | default("services") | split("/") | last }}

type {{ name | pascal }}Service struct{}
```

### Usage with meta

```bash
# Default package
tag generate service auth
# → internal/services/auth.go

# Custom package
tag generate service auth -m package=pkg/services
# → pkg/services/auth.go
```

---

## Recipe 8: Converting Cookiecutter Templates

### Direct scaffolding (auto-detection)

```bash
# TAG auto-detects Cookiecutter templates and converts on the fly
tag scaffold gh:user/cookiecutter-django my-project
```

### Explicit conversion

```bash
# Convert and keep the TAG version
tag convert cookiecutter ./cookiecutter-template -d ./tag-template

# Or add directly to library (auto-converts)
tag lib add gh:user/cookiecutter-flask --as flask
```

### What gets converted

- `cookiecutter.json` → `tag.template.json`
- `{{ cookiecutter.var }}` → `{{ vars.var }}`
- Directory names `{{cookiecutter.project_slug}}` → `{{ vars.project_slug }}`
- Jinja2 filters are preserved where compatible
- Hooks in `hooks/` are copied as-is

---

## Recipe 9: Replay and Non-Interactive Scaffolding

### Save and replay inputs

```bash
# First scaffold saves inputs automatically
tag scaffold gh:user/template my-project

# Later, scaffold with same inputs
tag scaffold gh:user/template another-project --replay
```

### CI/CD non-interactive scaffolding

```bash
# Pass all variables via flags
tag scaffold gh:user/template my-project \
  --no-input \
  --accept-hooks \
  -m project_name=my-project \
  -m author="CI Bot" \
  -m license=MIT
```

### Values file

```bash
# values.json
# {
#   "project_name": "my-project",
#   "author": "CI Bot",
#   "license": "MIT"
# }

tag scaffold gh:user/template my-project \
  --no-input \
  --accept-hooks \
  --values values.json
```
