# tag test

Matrix-test all boolean variable combinations in a scaffold template.

## Synopsis

```bash
tag test [template-dir] [flags]
```

## Description

The `test` command validates that a template scaffolds correctly for every combination of boolean variables. Given a template with N boolean variables, it generates up to 2^N combinations, scaffolds each into a temporary directory, and optionally runs validation commands.

This is useful for template authors to ensure conditional file generation (`{% if vars.use_x %}`) works correctly across all permutations before publishing a template.

### Two-Phase Execution

Internally, `tag test` works in two phases:

1. **Plan** — loads `tag.template.json`, identifies boolean variables, generates combinations (applying `--skip`, `--pin`, `--filter`), and checks the safety limit. No scaffolding happens yet.
2. **Execute** — runs each combination through `tag scaffold` in a temporary directory, then executes validation commands if provided.

The `--dry-run` flag stops after the Plan phase and prints the combination matrix.

## Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `template-dir` | No | Path to template directory (default: current directory) |

Flags may appear before or after `template-dir`: `tag test ./my-template --format json` and `tag test --format json ./my-template` are equivalent.

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--parallel <n>` | `-p` | Max concurrent test runs (default: `4`) |
| `--fail-fast` | | Stop on first failure |
| `--dry-run` | `-d` | List combinations without running |
| `--keep-failed` | | Preserve scaffolded directories on failure |
| `--timeout <duration>` | | Per-command timeout (default: `5m`) |
| `--max-cases <n>` | | Safety limit for total combinations (default: `64`, `0` = unlimited) |
| `--run <cmd>` | | Validation command (repeatable, overrides template config) |
| `--skip <var>` | | Boolean var to exclude from permutation (repeatable) |
| `--pin <key=value>` | | Fix a variable at a specific value (repeatable) |
| `--filter <expr>` | | Run only matching combinations (index or `key=value` pairs) |
| `--meta <key=value>` | `-m` | Required string variable override (repeatable) |
| `--values <file>` | | Load variables from JSON file |
| `--accept-hooks` | | Opt-in to run template-defined hooks and test commands |
| `--format <fmt>` | | Output format: `text` (default) or `json` |
| `--verbose` | `-v` | Show full output on failures |

## Template Configuration

Templates can define test-specific settings in `tag.template.json` under the `test` key:

```json
{
  "vars": {
    "project_name": "my-service",
    "use_docker": { "type": "boolean", "default": true },
    "use_grpc": { "type": "boolean", "default": false }
  },
  "test": {
    "project_name": "test-project",
    "commands": ["go build ./...", "go vet ./..."]
  }
}
```

| Property | Type | Description |
|----------|------|-------------|
| `test.project_name` | string | Fixed project name for test scaffolds (avoids prompt) |
| `test.commands` | string[] | Validation commands run inside each scaffolded directory |

Template-defined `test.commands` require `--accept-hooks` to execute (same security model as scaffold hooks). User-provided `--run` commands always execute and override template commands.

## Examples

### Preview Combinations

```bash
# Show what would be tested without running anything
tag test ./my-template --dry-run
```

Output:
```
Template:          ./my-template
Boolean variables: use_docker, use_grpc, use_ci
Combinations:      8

[0] use_docker=true  use_grpc=true  use_ci=true
[1] use_docker=true  use_grpc=true  use_ci=false
[2] use_docker=true  use_grpc=false use_ci=true
...
```

### Run All Combinations

```bash
# Without validation commands (scaffold-only)
tag test ./my-template

# With template-defined test commands
tag test ./my-template --accept-hooks

# With custom validation commands
tag test ./my-template --run "go build ./..." --run "go vet ./..."
```

### Reduce the Matrix

```bash
# Pin a variable (not permuted)
tag test ./my-template --pin use_docker=true

# Skip a variable (uses its default value)
tag test ./my-template --skip use_ci

# Run a single combination by index
tag test ./my-template --filter 3

# Filter by variable values
tag test ./my-template --filter "use_docker=true,use_grpc=false"
```

### Provide Required Variables

Non-boolean variables (strings, numbers, choices) aren't permuted but may be required:

```bash
# Via --meta flags
tag test ./my-template -m module_path=github.com/test/project

# Via values file
tag test ./my-template --values test-values.json
```

### CI Integration

```bash
# JSON output for machine consumption
tag test ./my-template --accept-hooks --format json --fail-fast

# Check exit code
tag test ./my-template --accept-hooks
echo "Exit code: $?"
```

### Debug Failures

```bash
# Keep failed directories and show full output
tag test ./my-template --accept-hooks --keep-failed --verbose
```

Output for failures includes the preserved directory path:
```
FAIL [2] use_docker=true use_grpc=false use_ci=true
  Phase: validate:go build ./...
  Kept:  /tmp/tag-test-abc123
```

### Override Safety Limit

```bash
# Template with many booleans (default limit is 64)
tag test ./my-template --max-cases 0          # Unlimited
tag test ./my-template --max-cases 256        # Higher cap
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All combinations passed |
| `1` | One or more combinations failed validation |
| `2` | Internal error (template parse failure, config error, unsupported `--format` value, etc.) |

## JSON Output Format

When `--format json` is used, the report is written to stdout:

```json
{
  "cases": [
    {
      "case_name": "default",
      "combination": {
        "index": 0,
        "vars": {"use_docker": "true", "use_grpc": "true"}
      },
      "status": "passed",
      "duration": 1200000000
    },
    {
      "case_name": "default",
      "combination": {
        "index": 1,
        "vars": {"use_docker": "true", "use_grpc": "false"}
      },
      "status": "failed",
      "phase": "validate:go build ./...",
      "output": "build error details...",
      "duration": 800000000
    }
  ],
  "passed": 1,
  "failed": 1,
  "errored": 0,
  "total_cases": 2,
  "duration": 2100000000,
  "template_dir": "./my-template"
}
```

The `status` field is a string (`"passed"`, `"failed"`, or `"errored"`), not an integer. `case_name` is the matching `test.cases[].name` entry from `tag.template.json`, or `"default"` when the template defines no named cases.

## Error Handling

| Error | Cause | Solution |
|-------|-------|----------|
| "no boolean variables found" | Template has no boolean vars to permute | Add boolean variables to `tag.template.json` |
| "exceeds safety limit of N" | 2^N combinations exceed `--max-cases` | Use `--skip`/`--pin` to reduce, or `--max-cases 0` to override |
| "template-defined test commands require --accept-hooks" | Commands in `test.commands` without opt-in | Add `--accept-hooks` flag |
| "malformed filter expression" | Invalid `--filter` value | Use an integer index or `key=value` pairs |

## See Also

- [tag scaffold](scaffold.md) - Project scaffolding (used internally by `tag test`)
- [tag template lint](template.md#tag-template-lint) - Validate template syntax and variables
- [tag.template.json Reference](../reference/tag.template.json.md) - Configuration format including `test` block
