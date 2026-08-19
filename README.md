# TAG

[![CI](https://github.com/kaikenlabs/tag/actions/workflows/ci.yml/badge.svg)](https://github.com/kaikenlabs/tag/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/kaikenlabs/tag/branch/main/graph/badge.svg)](https://codecov.io/gh/kaikenlabs/tag)
[![Go Reference](https://pkg.go.dev/badge/github.com/kaikenlabs/tag.svg)](https://pkg.go.dev/github.com/kaikenlabs/tag)
[![Go Report Card](https://goreportcard.com/badge/github.com/kaikenlabs/tag)](https://goreportcard.com/report/github.com/kaikenlabs/tag)

The scaffolding tool that keeps working after day one.

Most scaffolding tools create a project and disappear. TAG stays — generating code into your project as it grows. One binary, Jinja2 templates, built for AI coding agents.

## Quick Start

### Install

```bash
# macOS/Linux
curl -sSfL https://raw.githubusercontent.com/kaikenlabs/tag/main/install.sh | sh

# With Go
go install github.com/kaikenlabs/tag@latest
```

### Scaffold a Project

```bash
# From a GitHub template
tag scaffold gh:user/go-api my-service

# From a local template
tag scaffold ./my-template
```

### Generate Code Into It Later

```bash
cd my-service
tag generate handler UserAuth
# → creates internal/handlers/user_auth_handler.go
```

## Why TAG

**Scaffold and generate in one tool.** Most scaffolding tools (Cookiecutter, Yeoman) create a project and walk away. Code generators (Plop, Hygen) only work inside existing projects. TAG does both — scaffold a complete project from a template, then keep generating code into it as your project grows.

**Built for AI coding agents.** TAG templates include [`.skills/`](.skills/SKILL.md) files that AI coding assistants understand. Your AI agent can scaffold projects, create generators, and run code generation without manual copy-paste.

**Single binary, familiar syntax.** One `curl` to install. No Python, no Node, no runtime. Templates use [Jinja2 syntax](docs/templates/syntax.md) you already know.

**Migrate from Cookiecutter.** TAG [auto-detects Cookiecutter templates and converts them](docs/commands/convert.md). Your existing templates keep working.

## How It Works

### Scaffolding

1. Pick a template — local, GitHub, GitLab, Bitbucket, or any Git repo
2. Answer interactive prompts (or pass variables on the command line)
3. TAG renders every file, runs [hooks](docs/templates/hooks.md), and outputs your project

### Code Generation

1. Run `tag template init` in your project
2. Create generators in `.tag/` with [frontmatter](docs/templates/authoring.md) — create files, append to files, or inject at markers
3. Run `tag generate <generator> <name>` to add code

## Features

| | |
|---|---|
| **Template sources** | GitHub (`gh:`), GitLab (`gl:`), Bitbucket (`bb:`), Git URLs, zips, [local paths](docs/reference/remote-refs.md) |
| **Template syntax** | Jinja2 via Gonja — `{{ vars.project_name }}`, conditionals, loops ([syntax guide](docs/templates/syntax.md)) |
| **Generators** | Create files, append to files, or inject at markers ([authoring guide](docs/templates/authoring.md)) |
| **Filters** | Case transforms, inflections, string operations ([full list](docs/templates/filters.md)) |
| **Hooks** | Run commands before/after scaffolding ([hooks guide](docs/templates/hooks.md)) |
| **Replay** | Save and reuse inputs for reproducible scaffolds |
| **Library** | Install, manage, and share templates locally ([lib commands](docs/commands/lib.md)) |
| **Shell completion** | Bash, Zsh, Fish |
| **Cookiecutter compat** | Auto-detect and convert existing templates ([convert guide](docs/commands/convert.md)) |
| **Configuration** | [`tag.template.json`](docs/reference/tag.template.json.md) for variables, hooks, and metadata |

## Commands

| Command | Description |
|---------|-------------|
| `tag scaffold [template] [project]` | Create project from template (no args = picker) |
| `tag generate <generator> <name>` | Run a generator or bundle |
| `tag template init` | Initialize TAG in a project |
| `tag template new generator <name>` | Create a new generator |
| `tag template new bundle <name>` | Create a new bundle |
| `tag template info <template>` | Show template metadata (`--format json`) |
| `tag template list` | List available generators and bundles |
| `tag template variables` | Audit variables across templates |
| `tag template graph` | Visualize generator dependencies |
| `tag lib add\|list\|remove\|update\|edit` | Manage the template library |
| `tag convert cookiecutter <source>` | Convert Cookiecutter template |
| `tag test [template-dir]` | Matrix-test all boolean variable combinations |
| `tag version [--check]` | Print version, optionally check for updates |
| `tag completion <shell>` | Output shell completion script |

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](docs/getting-started.md) | Installation and first steps |
| [Tutorials](docs/tutorials/README.md) | Step-by-step guides with working examples |
| [Scaffold Command](docs/commands/scaffold.md) | Project scaffolding reference |
| [Generate Command](docs/commands/generate.md) | Code generation reference |
| [Convert Command](docs/commands/convert.md) | Cookiecutter migration |
| [Template Authoring](docs/templates/authoring.md) | Creating templates and generators |
| [Template Syntax](docs/templates/syntax.md) | Jinja2/Gonja syntax guide |
| [Filter Reference](docs/templates/filters.md) | Available template filters |
| [Hooks Guide](docs/templates/hooks.md) | Pre and post hooks |
| [Configuration Reference](docs/reference/tag.template.json.md) | `tag.template.json` schema |
| [Test Command](docs/commands/test.md) | Matrix testing for template validation |
| [Remote References](docs/reference/remote-refs.md) | Remote template formats |

## For AI Coding Agents

If you're an AI agent working with a TAG template, start with [`.skills/SKILL.md`](.skills/SKILL.md) — it has a decision tree, generator anatomy, CLI quick reference, and common pitfalls in an LLM-optimized format. Detailed reference and recipes are in `.skills/reference.md` and `.skills/recipes.md`.

## License

MIT
