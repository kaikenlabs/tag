# Tutorials

Guided walkthroughs for learning TAG. Each tutorial builds a complete, working example.

| Tutorial | Time | Audience | What You'll Learn |
|----------|------|----------|-------------------|
| [Build a Scaffold Template](01-scaffold-template.md) | ~15 min | Template authors | Template structure, variables, conditionals, generators, bundles, library |
| [Add Generators to an Existing Project](02-generators-existing-project.md) | ~10 min | Developers | `tag template init`, generators, bundles, dry-run, undo |

## Example Templates

Both tutorials reference working example templates you can scaffold right away:

```bash
# Python (Flask)
tag scaffold ./examples/weather-api-python my-weather-app

# Go (net/http)
tag scaffold ./examples/weather-api-go my-weather-app
```

## Prerequisites

- TAG installed (`tag version` works)
- Basic familiarity with the terminal

## See Also

- [Getting Started](../getting-started.md) — installation and first steps
- [Template Authoring](../templates/authoring.md) — full authoring reference
- [Template Syntax](../templates/syntax.md) — Jinja2/Gonja syntax guide
- [tag.template.json Reference](../reference/tag.template.json.md) — configuration schema
