# Hooks Guide

Hooks allow you to run commands and scripts before and after scaffold operations. This is useful for validation, initialization, and post-processing tasks.

## Overview

TAG supports two types of hooks:

| Hook | When it Runs | Working Directory | Failure Behavior |
|------|--------------|-------------------|------------------|
| `pre_scaffold` | After variable collection, before file generation | Template directory | Stops scaffold, no files created |
| `post_scaffold` | After all files are generated | Output directory | Warning only, files are kept |

## Configuration

Define hooks in `tag.template.json`:

```json
{
  "vars": {
    "project_name": "my-project"
  },
  "hooks": {
    "pre_scaffold": [
      "./scripts/validate.sh",
      "echo 'Starting scaffold...'"
    ],
    "post_scaffold": [
      "go mod tidy",
      "git init",
      "npm install"
    ]
  }
}
```

## Hook Execution

### Template Variables in Hook Commands

Hook command strings support template expressions using the same `{{ vars.* }}` syntax as template files. Variables are rendered through the template engine before the command is executed:

```json
{
  "hooks": {
    "post_scaffold": [
      "cd {{ vars.project_name }} && go mod tidy",
      "echo 'Created project: {{ vars.project_name | upper }}'"
    ]
  }
}
```

This renders to `cd my-project && go mod tidy` (assuming `project_name` is `my-project`).

**Notes:**
- All resolved variables are available, including derived and private variables
- Commands without `{{ }}` syntax are passed through unchanged (no performance cost)
- If a template expression fails to render (e.g., referencing an undefined variable), the scaffold fails with an error
- Hook rendering only runs when hooks are enabled — invalid template syntax in hooks won't affect scaffolding when hooks are skipped
- For shell variable expansion (`$TAG_VAR_*`), use explicit shell invocation: `sh -c 'echo $TAG_VAR_PROJECT_NAME'`

### Script Execution

Hooks are executed as direct commands (no shell wrapper). Bare commands like `go`, `npm`, and `git` are looked up on `PATH` as usual.

For script files (paths containing `/` or starting with `.`), TAG automatically resolves the interpreter:

1. **Shebang line** (`#!`): If the script starts with a shebang, the OS handles execution normally.
2. **Extension fallback**: If there is no shebang, TAG prepends an interpreter based on the file extension.
3. **No match**: If the extension is unrecognized, the file is executed as-is.

### Supported Script Languages

| Extension | Interpreter | Notes |
|-----------|-------------|-------|
| `.py` | `python3` or `python` | Tries `python3` first, falls back to `python` |
| `.sh` | `sh` | Only when no shebang is present |
| `.rb` | `ruby` | |
| `.js` | `node` | |
| `.pl` | `perl` | |

Scripts with a shebang (`#!/usr/bin/env python3`, `#!/bin/bash`, etc.) are always executed using the shebang, regardless of extension.

### Sequential Execution

Hooks run in order, one at a time. If a hook fails (non-zero exit code), subsequent hooks in that phase don't run.

### Timeout

Each hook has a maximum execution time of **5 minutes**. Hooks that exceed this timeout are terminated.

### Output

- Output is captured (stdout and stderr combined)
- Maximum output size: **1 MB** (truncated if exceeded)
- Output is displayed to the user during scaffold

## Environment Variables

Hooks receive all template variables as environment variables:

| Environment Variable | Description |
|---------------------|-------------|
| `TAG_TEMPLATE_DIR` | Absolute path to the template directory |
| `TAG_OUTPUT_DIR` | Absolute path to the project root directory (inside the wrapper directory when the template uses one) |
| `TAG_PROJECT_NAME` | Value of the `project_name` variable |
| `TAG_VAR_<NAME>` | Each variable as `TAG_VAR_` + uppercase name |

Despite the name, `TAG_OUTPUT_DIR` matches `project_root` from [`tag scaffold --format json`](../commands/scaffold.md#machine-readable-output), not `output_dir` — hooks run inside the wrapper directory when the template has one, same as the project root the JSON document reports.

### Variable Name Transformation

Variable names are converted to environment variable format:
- Uppercase
- Non-alphanumeric characters become underscores

| Variable | Environment Variable |
|----------|---------------------|
| `project_name` | `TAG_VAR_PROJECT_NAME` |
| `use-docker` | `TAG_VAR_USE_DOCKER` |
| `author` | `TAG_VAR_AUTHOR` |

### Complex Values

Complex values (arrays, objects) are JSON-encoded:

```bash
# If vars.features = ["auth", "logging"]
echo $TAG_VAR_FEATURES  # ["auth","logging"]
```

## Pre-Scaffold Hooks

Pre-scaffold hooks run **before any files are created**, in the **template directory**.

### Use Cases

1. **Environment Validation**
   ```json
   {
     "hooks": {
       "pre_scaffold": ["./scripts/check-requirements.sh"]
     }
   }
   ```

   ```bash
   #!/bin/bash
   # check-requirements.sh
   if ! command -v go &> /dev/null; then
       echo "Error: Go is required but not installed"
       exit 1
   fi
   ```

2. **API Validation**
   ```json
   {
     "hooks": {
       "pre_scaffold": ["./scripts/validate-github-token.sh"]
     }
   }
   ```

3. **User Confirmation**
   ```json
   {
     "hooks": {
       "pre_scaffold": ["echo 'Creating project: $TAG_PROJECT_NAME'"]
     }
   }
   ```

### Failure Behavior

If a pre-scaffold hook fails:
- The scaffold process stops immediately
- **No files are created**
- Error message is displayed to the user

## Post-Scaffold Hooks

Post-scaffold hooks run **after all files are created**, in the **output directory**.

### Use Cases

1. **Dependency Installation**
   ```json
   {
     "hooks": {
       "post_scaffold": [
         "go mod tidy",
         "npm install"
       ]
     }
   }
   ```

2. **Git Initialization**
   ```json
   {
     "hooks": {
       "post_scaffold": [
         "git init",
         "git add .",
         "git commit -m 'Initial commit from TAG template'"
       ]
     }
   }
   ```

3. **Code Formatting**
   ```json
   {
     "hooks": {
       "post_scaffold": [
         "gofmt -w .",
         "prettier --write '**/*.{js,ts,json}'"
       ]
     }
   }
   ```

4. **Display Next Steps**
   ```json
   {
     "hooks": {
       "post_scaffold": [
         "echo ''",
         "echo 'Project created successfully!'",
         "echo ''",
         "echo 'Next steps:'",
         "echo '  cd $TAG_PROJECT_NAME'",
         "echo '  make run'"
       ]
     }
   }
   ```

### Failure Behavior

If a post-scaffold hook fails:
- A **warning** is displayed
- **Generated files are preserved**
- User is notified that some post-scaffold tasks may not have run

Example warning:
```
Warning: post-scaffold hook failed: exit status 1
Note: Scaffold completed successfully, but some post-scaffold tasks may not have run.
```

## Writing Hook Scripts

### Best Practices

1. **Use Absolute Paths or Relative to Template**
   ```json
   {
     "hooks": {
       "pre_scaffold": ["./scripts/validate.sh"]
     }
   }
   ```

2. **Check Exit Codes**
   ```bash
   #!/bin/bash
   set -e  # Exit on error

   if ! some_command; then
       echo "Error: something failed"
       exit 1
   fi
   ```

3. **Handle Missing Tools Gracefully**
   ```bash
   #!/bin/bash
   if command -v gofmt &> /dev/null; then
       gofmt -w .
   else
       echo "Warning: gofmt not found, skipping formatting"
   fi
   ```

4. **Use Environment Variables**
   ```bash
   #!/bin/bash
   echo "Setting up $TAG_VAR_PROJECT_NAME..."

   if [ "$TAG_VAR_USE_DOCKER" = "true" ]; then
       docker-compose build
   fi
   ```

### Cross-Platform Compatibility

For maximum compatibility:

1. **Use Simple Shell Commands**
   ```json
   {
     "hooks": {
       "post_scaffold": ["echo Done"]
     }
   }
   ```

2. **Avoid Bash-Specific Features** (for Windows compatibility)
   - Use `[ ]` instead of `[[ ]]`
   - Avoid bash arrays
   - Use portable commands

3. **Consider Separate Scripts**
   ```
   my-template/
   ├── scripts/
   │   ├── post-scaffold.sh    # Unix
   │   └── post-scaffold.bat   # Windows
   ```

## Security: Remote Templates

For security, hooks are **disabled by default** when scaffolding from remote templates (e.g., `gh:user/repo`, Git URLs, zip URLs). This prevents untrusted templates from executing arbitrary commands on your machine.

To allow hooks for a trusted remote template:
```bash
tag scaffold gh:trusted-org/template --accept-hooks
```

Local templates always run hooks since you control the template source.

## Debugging Hooks

### View Hook Output

Hook output is displayed during scaffold:

```bash
$ tag scaffold ./my-template

Running pre-scaffold hooks...
Validating environment...
All checks passed!

Scaffolding project...
✓ Created my-project/cmd/main.go
✓ Created my-project/README.md

Running post-scaffold hooks...
go: finding module for package...
Initialized empty Git repository in /path/to/my-project/.git/
```

### Test Hooks Independently

Test your hook scripts before including them:

```bash
# Set environment variables manually
export TAG_VAR_PROJECT_NAME="test-project"
export TAG_OUTPUT_DIR="/tmp/test"

# Run the script
./scripts/post-scaffold.sh
```

### Test with a Temporary Directory

To test scaffolding without affecting your project:

```bash
tag scaffold ./my-template /tmp/test-output
```

## Common Hook Patterns

### Conditional Hook Execution

```bash
#!/bin/bash
# Only run if Docker is enabled
if [ "$TAG_VAR_USE_DOCKER" = "true" ]; then
    docker-compose build
fi
```

### Multiple Conditional Commands

```json
{
  "hooks": {
    "post_scaffold": [
      "[ \"$TAG_VAR_USE_DOCKER\" = \"true\" ] && docker-compose build || true",
      "[ \"$TAG_VAR_USE_CI\" = \"true\" ] && ./scripts/setup-ci.sh || true"
    ]
  }
}
```

### Python Post-Processing

Python scripts are executed automatically — TAG resolves the interpreter from the `.py` extension:

```json
{
  "hooks": {
    "post_scaffold": ["./scripts/finalize.py"]
  }
}
```

You can also specify the interpreter explicitly if preferred:

```json
{
  "hooks": {
    "post_scaffold": ["python3 ./scripts/finalize.py"]
  }
}
```

### Node.js Post-Processing

```json
{
  "hooks": {
    "post_scaffold": ["./scripts/setup.js"]
  }
}
```

## Migrating from Cookiecutter Hooks

Cookiecutter hooks (`hooks/pre_gen_project.py`, `hooks/post_gen_project.py`) work natively in TAG. The `tag convert` command copies hook files and generates the appropriate hooks configuration automatically.

Python hooks are executed using the system's `python3` (or `python`) interpreter — no manual conversion to shell scripts is required.

1. **Simple Commands**: Convert directly
   ```python
   # Cookiecutter
   subprocess.call(['git', 'init'])

   # TAG
   "git init"
   ```

2. **Python Hooks**: Reference the script directly — TAG resolves the interpreter
   ```json
   {
     "hooks": {
       "post_scaffold": ["hooks/post_gen_project.py"]
     }
   }
   ```

3. **Access Variables**: Use environment variables instead of `cookiecutter` dict
   ```python
   # Cookiecutter
   project_name = '{{ cookiecutter.project_name }}'

   # TAG (Python script)
   import os
   project_name = os.environ.get('TAG_VAR_PROJECT_NAME')
   ```

> **Note**: TAG tries `python3` first, then `python`. If neither is installed, the hook will fail with a clear "command not found" error.

## Generate Hooks

The `tag generate` command also supports pre and post hooks, defined in `.tagconfig.json` under `hooks.pre` and `hooks.post`. These hooks run before and after code generation, similar to scaffold hooks.

### Environment Variables

Generate hooks receive scaffold-time variables as environment variables:

| Environment Variable | Description |
|---------------------|-------------|
| `TAG_PROJECT_NAME` | Value of the `project_name` variable (if set) |
| `TAG_VAR_<NAME>` | Each scaffold variable as `TAG_VAR_` + uppercase name |
| `TAG_GENERATOR_NAME` | Generator or bundle name being run |
| `TAG_TARGET_NAME` | The positional name argument |

Variable names follow the same transformation rules as scaffold hooks (uppercase, non-alphanumeric characters become underscores).

> **Note**: Generate hooks do **not** receive `TAG_TEMPLATE_DIR` or `TAG_OUTPUT_DIR` — those are scaffold-specific.

### Post-Hook Failure Behavior

Post-generate hook failures are treated as **warnings** — generated files are preserved and the user is notified. Pre-generate hook failures remain **fatal** and abort generation before any files are written.

### Example

```json
{
  "hooks": {
    "post": [
      ["gofmt", "-w", "."],
      ["goimports", "-w", "."]
    ]
  }
}
```

```bash
# In a post-generation hook script:
echo "Formatting code for $TAG_PROJECT_NAME..."
```

### Skipping Hooks

Use `--no-hooks` to skip hook execution:

```bash
tag generate --no-hooks model User
```

## See Also

- [Template Authoring](authoring.md) - Complete template guide
- [tag.template.json Reference](../reference/tag.template.json.md) - Configuration format
- [Convert Command](../commands/convert.md) - Cookiecutter migration
