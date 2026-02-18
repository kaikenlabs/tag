# tag template init

Initialize a TAG directory structure in the current project.

## Synopsis

```bash
tag template init
```

## Description

The `tag template init` command creates the `.tag/` directory structure needed for code generation. Use this when you want to add generators to an existing project that was not scaffolded by TAG.

If the project was scaffolded by TAG (`tag scaffold`), the `.tag/` directory and `.tagconfig.json` are created automatically — you don't need to run `tag template init`.

## Directory Structure Created

```
.tag/
├── _shared/      # Shared template fragments ({% include %})
└── _bundles/     # Bundle definitions (JSON)
```

## Examples

```bash
# Initialize TAG in an existing project
cd my-project
tag template init

# Create your first generator
tag template new generator handler

# Use it
tag generate handler User
```

## See Also

- [Template Command](template.md) - Template management overview
- [tag template new](new.md) - Create a new generator
- [tag generate](generate.md) - Run generators
- [Getting Started](../getting-started.md) - Quick start guide
