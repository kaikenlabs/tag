# Getting Started with TAG

This guide will help you get started with TAG, from installation to creating your first project from a template.

## Installation

### Using Go Install

```bash
go install github.com/kaikenlabs/tag@latest
```

### From Source

```bash
git clone https://github.com/kaikenlabs/tag.git
cd tag
go build -o tag
./tag --help
```

## Quick Start

### 1. Scaffold a Project from a Template

The quickest way to start is using an existing template:

```bash
# From a GitHub template
tag scaffold gh:user/my-template

# From a local template
tag scaffold ./my-local-template

# With a project name
tag scaffold gh:user/go-api my-awesome-api
```

TAG will:
1. Fetch the template (if remote)
2. Prompt you for variable values interactively
3. Generate your project with all files processed

> **Tip:** TAG can also scaffold Cookiecutter templates directly! It will auto-detect and offer to convert them.

### 2. Create a Generator for Incremental Code Generation

For adding code to existing projects, use generators:

```bash
# Initialize TAG in your project
tag init

# Create a new generator
tag new handler

# Edit your generator templates in .tag/handler/
# Then run:
tag generate handler UserAuth
```

If your project was scaffolded from a library template (recorded in `.tagconfig.json`), you can add generators and bundles directly to that template with `--lib`:

```bash
# Create a generator in the source library template
tag new --lib handler

# Create a bundle in the source library template
tag new-bundle --lib crud
```

## Core Concepts

### Templates vs Generators

| Concept | Command | Purpose |
|---------|---------|---------|
| **Template** | `tag scaffold` | Create a complete new project from scratch |
| **Generator** | `tag generate` | Add files to an existing project |

### Template Sources

TAG supports multiple template sources:

| Format | Example | Description |
|--------|---------|-------------|
| Local directory | `./my-template` | Local filesystem |
| GitHub | `gh:user/repo` | GitHub repository |
| GitLab | `gl:user/repo` | GitLab repository |
| Bitbucket | `bb:user/repo` | Bitbucket repository |
| Git URL | `https://github.com/user/repo.git` | Any Git repository |
| Zip URL | `https://example.com/template.zip` | Remote zip file |

See [Remote References](reference/remote-refs.md) for the complete reference.

### Template Syntax (Jinja2/Gonja)

TAG uses [Gonja](https://github.com/noirbizarre/gonja), a Jinja2-compatible template engine:

~~~jinja2
# {{ vars.project_name }}

Author: {{ vars.author }}

{% if vars.use_docker %}
## Docker

Build the image:

    docker build -t {{ vars.project_name|kebab }} .

{% endif %}
~~~

See [Template Syntax](templates/syntax.md) for the complete syntax guide.

## What's Next?

- [Scaffold Command Reference](commands/scaffold.md) - All flags and options for project scaffolding
- [Template Library](commands/lib.md) - Install and manage templates locally
- [Run Command](commands/run.md) - Scaffold from library templates
- [Creating Templates](templates/authoring.md) - How to create your own templates
- [Filter Reference](templates/filters.md) - Available template filters
