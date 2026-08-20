# tag convert cookiecutter

Convert a Cookiecutter template to TAG format.

## Synopsis

```bash
tag convert cookiecutter <source> [flags]
```

## Description

The `convert cookiecutter` command converts existing [Cookiecutter](https://cookiecutter.readthedocs.io/) templates to TAG format. This enables migration of the large ecosystem of Cookiecutter templates to work with TAG.

The conversion process:
1. Converts `cookiecutter.json` to `tag.template.json`
2. Converts all `cookiecutter.*` references to `vars.*` syntax (paths, file contents, hooks)
3. Preserves derived variables (those referencing other variables)
4. Copies and converts hook scripts
5. Reports any content incompatibilities between Jinja2 and Gonja

> **Note**: You can also use `tag scaffold` directly on a Cookiecutter template—TAG will auto-detect it and offer to convert interactively. See [Auto-Detection](#auto-detection) below.

## Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `source` | Yes | Path or remote reference to the Cookiecutter template |

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--output <path>` | `-o` | Output directory (default: `<source-name>-tag`) |
| `--force` | `-f` | Overwrite output directory if it exists |
| `--format <fmt>` | | Output format: `text` (default) or `json` |

Use the global `--dry-run` / `-d` flag to preview conversion without writing files.

`--force` and `--format` are recognised whether they appear before or after the `<source>` argument — a trailing `tag convert cookiecutter ./cookiecutter-myproject --format json` works the same as putting the flag first.

## Examples

### Basic Conversion

```bash
# Convert a local Cookiecutter template
tag convert cookiecutter ./cookiecutter-myproject -o ./myproject-tag

# Convert a remote template
tag convert cookiecutter gh:user/cookiecutter-django -o ./django-tag
```

### Preview Mode

```bash
# See what would be converted without writing files
tag convert cookiecutter ./cookiecutter-myproject -d
```

### Force Overwrite

```bash
# Overwrite existing output directory
tag convert cookiecutter ./cookiecutter-myproject --force
```

### Using Output Flag

```bash
# Specify output directory
tag convert cookiecutter ./cookiecutter-myproject -o ./output-dir
```

## Output

The converter displays a detailed summary:

```
Converted template: ./myproject-tag

✓ Variables: 8 converted
✓ Directories renamed: 3
✓ Files renamed: 12
✓ Files processed: 25
⚠ Hooks: 2 files copied (review required)
⚠ Content incompatibilities found: 1
  Minor adjustments may be needed:
  - setup.py.tmpl:8 - default_filter_syntax
    Found: {{ author|default('Anonymous') }}
    Gonja: {{ author|default:"Anonymous" }}

See: https://tag.kaikenlabs.com/docs/migration
```

## Machine-Readable Output

```bash
tag convert cookiecutter ./cookiecutter-myproject -o ./myproject-tag --format json
```

```json
{
  "source": "./cookiecutter-myproject",
  "destination": "./myproject-tag",
  "variables_converted": 8,
  "dirs_renamed": 3,
  "files_renamed": 12,
  "files_processed": 25,
  "hooks_copied": 2,
  "incompatibilities": [
    {
      "path": "setup.py.tmpl",
      "line": 8,
      "kind": "default_filter_syntax",
      "message": "...",
      "original": "{{ author|default('Anonymous') }}",
      "suggestion": "{{ author|default:\"Anonymous\" }}",
      "severity": "warning"
    }
  ],
  "warnings": [],
  "dry_run": false,
  "files": [
    { "from": "{{ cookiecutter.project_name }}/README.md", "to": "{{ vars.project_name }}/README.md" }
  ],
  "variables": [
    { "name": "project_name", "original_type": "string", "tag_type": "string" }
  ]
}
```

Bare object, no envelope. `severity` is a plain string (`"info"`/`"warning"`/`"error"`). The array fields (`incompatibilities`, `warnings`, `files`, `variables`) are always present as `[]`, never omitted or `null`, even when empty. `files[].to` and every path/content conversion rewrite `{{ cookiecutter.var }}` to `{{ vars.var }}` **in place** — there is no `__var__` double-underscore form. `variables[].default` appears only in JSON (the text summary above never prints variable defaults); `original`/`suggestion` on an incompatibility are shown in the text output truncated to 60 characters, but appear in full in JSON.

## What Gets Converted

### Configuration

| Cookiecutter | TAG |
|--------------|-----|
| `cookiecutter.json` | `tag.template.json` |
| Variable definitions | Converted with types inferred |
| Private variables (`_name`) | Preserved as computed variables |

### Directory Structure

| Before | After |
|--------|-------|
| `{{ cookiecutter.project_name }}/` | `{{ vars.project_name }}/` |
| `{{ cookiecutter.var \| lower }}/` | `{{ vars.var \| lower }}/` |

### Derived Variables

Variables with template expressions as defaults (e.g., `"{{ cookiecutter.name | lower }}"`) are converted to use the `vars` namespace. These derived variables:
- Are NOT prompted during scaffolding (following Cookiecutter behavior)
- Are automatically computed from other variables
- Preserve the original computation logic

Example:
```json
// Cookiecutter format:
"package_name": "{{ cookiecutter.display_name.lower().replace(' ', '_') }}"

// Converted TAG format:
"package_name": "{{ vars.display_name.lower().replace(' ', '_') }}"
```

### Template Content

Template content (Jinja2 syntax) is **mostly compatible**. The converter will:
- Convert `{{ cookiecutter.* }}` references to `{{ vars.* }}` in all text files
- Convert `{% cookiecutter.* %}` references in control blocks (if/for) to `{% vars.* %}`
- Report incompatibilities that need manual adjustment
- Document filter syntax differences

## Compatibility Notes

### Fully Compatible Features

- Variable interpolation: `{{ cookiecutter.var }}`
- Control flow: `{% if %}`, `{% for %}`, `{% endif %}`, `{% endfor %}`
- Comments: `{# comment #}`
- Template inheritance: `{% extends %}`, `{% block %}`
- Macros: `{% macro %}`
- Filters: Most common filters work identically

### Syntax Differences (Gonja vs Jinja2)

| Feature | Jinja2 | Gonja |
|---------|--------|-------|
| Filter with argument | `{{ x\|default('val') }}` | `{{ x\|default:"val" }}` |
| Format filter | `{{ x\|format('%s') }}` | `{{ x\|format:"%s" }}` |
| Dict iteration | `{% for k, v in dict.items() %}` | `{% for k, v in dict %}` |

The converter detects and reports these differences automatically.

### Hooks

Cookiecutter hooks (Python scripts in `hooks/`) are copied but require manual review:
- Shell scripts usually work as-is
- Python hooks may need adaptation to work without Cookiecutter's environment
- Environment variables are provided differently (see [Hooks Guide](../templates/hooks.md))

## Post-Conversion Steps

1. **Review incompatibilities**: Fix any reported syntax differences
2. **Test hooks**: Ensure hook scripts work with TAG's environment
3. **Verify variables**: Check `tag.template.json` for correct types
4. **Test scaffold**: Run `tag scaffold` on the converted template
5. **Update documentation**: Modify any README references to Cookiecutter

## Variable Type Mapping

| Cookiecutter Value | Inferred TAG Type |
|--------------------|-------------------|
| `"string value"` | `string` |
| `true` / `false` | `boolean` |
| `123` / `45.67` | `number` |
| `["opt1", "opt2"]` | `choice` with options |
| `"{{ computed }}"` | Computed variable (string) |

## Error Handling

| Error | Cause | Solution |
|-------|-------|----------|
| "source template is required" | Missing source argument | Provide a template path |
| "failed to initialize converter" | Invalid template structure | Verify it's a valid Cookiecutter template |
| "cookiecutter.json not found" | Missing config file | Ensure `cookiecutter.json` exists in template root |

## Auto-Detection

When running `tag scaffold` on a Cookiecutter template (a directory with `cookiecutter.json` but no `tag.template.json`), TAG automatically offers to convert it:

```bash
$ tag scaffold ./my-cookiecutter-template

This appears to be a Cookiecutter template. Convert to TAG format? [Y/n] y
Output directory for converted template [./my-cookiecutter-template-tag]:
Converted template to: ./my-cookiecutter-template-tag
  Variables: 8, Files: 25

Output directory for scaffolded project [./my-project]:
Enter value for project_name [my_project]: awesome-app
...
```

This allows seamless use of Cookiecutter templates without a separate conversion step. In non-interactive mode (`--no-input`), auto-conversion is disabled and you must convert explicitly first.

## See Also

- [Scaffold Command](../commands/scaffold.md) - Using templates (includes Cookiecutter auto-detection)
- [Template Authoring](../templates/authoring.md) - TAG template structure
- [tag.template.json Reference](../reference/tag.template.json.md) - Configuration format
