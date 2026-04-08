# BDD Acceptance Test Plan for TAG CLI

## 1. Overview

BDD acceptance tests for the `tag` CLI. Tests are black-box E2E: build the binary once, exercise it as a subprocess with known fixtures, assert on exit codes, stdout/stderr, and filesystem side-effects.

### Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Level | BDD (Gherkin) driving black-box E2E | Validates real CLI wiring end-to-end |
| Framework | `github.com/cucumber/godog` | Go-native Cucumber; Gherkin `.feature` files |
| Scope | 11 acceptance scenarios for `tag scaffold` | Core user workflows + error paths |
| Prompts | Flags only (`--meta`, `--accept-hooks`, `--no-input`) | Non-TTY subprocess; explicit flag testing |
| Fixtures | `examples/` for happy paths, `tests/bdd/testdata/` for error paths | Real templates + synthetic error cases |
| Assertions | Exit code + stdout/stderr + file tree + file contents | Full behavioral coverage |
| Binary | Built once in `TestMain` via `go build` | Fast; avoids per-scenario rebuild |
| Location | `tests/bdd/` | Separated from unit/integration tests |
| CI target | `make test-bdd` | Independent Make target |
| Parallelism | Sequential (`Concurrency: 1`) | Simplicity; filesystem isolation per-scenario |
| Network | Hermetic only (no remote templates) | Deterministic, no external dependencies |
| Hooks in tests | Deterministic inline commands (`touch`, `echo`) | Cross-platform, predictable |
| Isolation | Per-scenario temp CWD + redirected HOME/XDG dirs | Full environment isolation |
| Debug mode | `TAG_BDD_KEEP_TMP=1` preserves temp dirs | Debugging ergonomics |

---

## 2. Directory Structure

```
tests/bdd/
  PLAN.md                           # This plan
  bdd_test.go                       # TestMain (builds binary) + TestFeatures (godog suite)
  context_test.go                   # scenarioCtx struct, runTag(), buildEnv()
  steps_given_test.go               # Given step definitions
  steps_when_test.go                # When step definitions
  steps_then_test.go                # Then step definitions
  features/
    scaffold_happy_path.feature     # Scenarios 1-3: basic scaffold, meta overrides, hooks
    scaffold_errors.feature         # Scenarios 4-8: missing template, missing var, bad config, etc.
    scaffold_advanced.feature       # Scenarios 9-11: wrapper unwrap, tagconfig, exit codes
  testdata/
    minimal-template/
      tag.template.json
      {{ vars.project_name }}/
        README.md
    template-with-hooks/
      tag.template.json
      {{ vars.project_name }}/
        README.md
    template-with-required-var/
      tag.template.json
      {{ vars.project_name }}/
        README.md
    template-with-wrapper/
      tag.template.json
      {{ vars.project_name }}/
        README.md
    invalid-config/
      tag.template.json             # Malformed JSON
    missing-config/                 # Empty directory (no tag.template.json)
```

All Go files use `_test.go` suffix so lint relaxations apply and code is excluded from production builds.

---

## 3. Phase 0: Dependencies and Setup

### 3.1 Add godog dependency

```bash
go get github.com/cucumber/godog@latest
```

`google/shlex` is already a dependency (used by hook arg parsing).

### 3.2 Create directory structure

```bash
mkdir -p tests/bdd/features tests/bdd/testdata
```

### 3.3 Makefile target

**Modify:** `scripts/tests.mk`

```makefile
test-bdd: ## Run BDD acceptance tests
	@go test -v -timeout 120s ./tests/bdd/...
```

Update the `test` aggregate target to include `test-bdd`:

```makefile
test: test-unit test-integration test-bdd ## Run all tests
```

### 3.4 CI workflow

**Modify:** `.github/workflows/ci.yml`

Add after integration tests:

```yaml
      - name: Run BDD acceptance tests
        run: go test -v -timeout 120s ./tests/bdd/...
```

No separate `make build` needed -- `TestMain` builds the binary internally.

---

## 4. Phase 1: Test Harness

### 4.1 bdd_test.go -- TestMain and Suite

**File:** `tests/bdd/bdd_test.go`

```go
package bdd_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cucumber/godog"
)

// tagBinary holds the absolute, symlink-resolved path to the built tag binary.
var tagBinary string

func TestMain(m *testing.M) {
	// 1. Resolve repo root (tests/bdd -> repo root is ../..)
	repoRoot, err := filepath.Abs(filepath.Join(".", "..", ".."))
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot resolve repo root: %v\n", err)
		os.Exit(1)
	}

	// 2. Build the tag binary into a temp directory
	tmpDir, err := os.MkdirTemp("", "tag-bdd-bin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create temp dir: %v\n", err)
		os.Exit(1)
	}

	binPath := filepath.Join(tmpDir, "tag")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = repoRoot // CRITICAL: build from repo root, not tests/bdd/
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build tag binary: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	// 3. Resolve symlinks (macOS /var/folders -> /private/var/folders)
	resolved, err := filepath.EvalSymlinks(binPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve binary path: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}
	tagBinary = resolved

	// 4. Run tests
	code := m.Run()

	// 5. Cleanup BEFORE os.Exit (defer does not run after os.Exit)
	os.RemoveAll(tmpDir)

	os.Exit(code)
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:      "pretty",
			Paths:       []string{"features"},
			TestingT:    t,
			Concurrency: 1,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("BDD suite failed")
	}
}
```

Key details:
- `cmd.Dir = repoRoot` ensures `go build .` builds the CLI entry point, not the test package
- `filepath.EvalSymlinks` resolves macOS temp dir symlinks
- Cleanup runs before `os.Exit(code)` -- `defer` would NOT execute after `os.Exit`
- Binary built in isolated temp dir, not repo root (avoids collision with `make build`)

### 4.2 context_test.go -- scenarioCtx

**File:** `tests/bdd/context_test.go`

```go
package bdd_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/shlex"
)

const commandTimeout = 30 * time.Second

// scenarioCtx holds per-scenario state. A fresh instance is created for each scenario.
type scenarioCtx struct {
	tmpDir      string // Per-scenario temp directory (CWD for tag)
	homeDir     string // Fake HOME
	xdgConf     string // Fake XDG_CONFIG_HOME
	xdgData     string // Fake XDG_DATA_HOME
	exitCode    int
	stdout      bytes.Buffer
	stderr      bytes.Buffer
	repoRoot    string // Absolute path to repository root
	templateDir string // Set by Given steps, used by When steps
}

// newScenarioCtx creates a fresh per-scenario context with isolated directories.
func newScenarioCtx() (*scenarioCtx, error) {
	repoRoot, err := filepath.Abs(filepath.Join(".", "..", ".."))
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "tag-bdd-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	// Resolve symlinks for consistent path comparisons on macOS
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("resolve temp dir symlinks: %w", err)
	}

	homeDir := filepath.Join(tmpDir, "home")
	xdgConf := filepath.Join(tmpDir, "xdg-config")
	xdgData := filepath.Join(tmpDir, "xdg-data")

	for _, d := range []string{homeDir, xdgConf, xdgData} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	return &scenarioCtx{
		tmpDir:   tmpDir,
		homeDir:  homeDir,
		xdgConf:  xdgConf,
		xdgData:  xdgData,
		repoRoot: repoRoot,
	}, nil
}

// cleanup removes the temp directory unless TAG_BDD_KEEP_TMP=1.
func (sc *scenarioCtx) cleanup() {
	if os.Getenv("TAG_BDD_KEEP_TMP") == "1" {
		fmt.Fprintf(os.Stderr, "[BDD] keeping temp dir: %s\n", sc.tmpDir)
		return
	}
	os.RemoveAll(sc.tmpDir)
}

// buildEnv returns a clean, isolated environment for the tag subprocess.
func (sc *scenarioCtx) buildEnv() []string {
	pathEnv := os.Getenv("PATH")
	binDir := filepath.Dir(tagBinary)

	return []string{
		"PATH=" + binDir + string(os.PathListSeparator) + pathEnv,
		"HOME=" + sc.homeDir,
		"XDG_CONFIG_HOME=" + sc.xdgConf,
		"XDG_DATA_HOME=" + sc.xdgData,
		"NO_COLOR=1",
		"TERM=dumb",
	}
}

// runTag executes the tag binary with the given argument string.
// Arguments are split using POSIX shell quoting rules (google/shlex).
// The {template} placeholder is replaced with the current templateDir.
func (sc *scenarioCtx) runTag(argStr string) error {
	// Resolve {template} placeholder
	if sc.templateDir != "" {
		argStr = strings.ReplaceAll(argStr, "{template}", sc.templateDir)
	}

	args, err := shlex.Split(argStr)
	if err != nil {
		return fmt.Errorf("parse args %q: %w", argStr, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, tagBinary, args...)
	cmd.Dir = sc.tmpDir
	cmd.Env = sc.buildEnv()
	cmd.Stdin = nil // Prevent interactive mode hangs

	sc.stdout.Reset()
	sc.stderr.Reset()
	cmd.Stdout = &sc.stdout
	cmd.Stderr = &sc.stderr

	runErr := cmd.Run()
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			sc.exitCode = exitErr.ExitCode()
		} else {
			return fmt.Errorf("exec failed: %w\nstdout: %s\nstderr: %s",
				runErr, sc.stdout.String(), sc.stderr.String())
		}
	} else {
		sc.exitCode = 0
	}

	return nil
}

// combinedOutput returns stdout + stderr for diagnostic messages.
func (sc *scenarioCtx) combinedOutput() string {
	return fmt.Sprintf("--- stdout ---\n%s\n--- stderr ---\n%s",
		sc.stdout.String(), sc.stderr.String())
}

// fixtureDir returns the absolute path to a named fixture (testdata first, then examples).
func (sc *scenarioCtx) fixtureDir(name string) string {
	testdataPath := filepath.Join(sc.repoRoot, "tests", "bdd", "testdata", name)
	if _, err := os.Stat(testdataPath); err == nil {
		return testdataPath
	}
	return filepath.Join(sc.repoRoot, "examples", name)
}

// InitializeScenario registers step definitions and lifecycle hooks with godog.
func InitializeScenario(ctx *godog.ScenarioContext) {
	var sc *scenarioCtx

	ctx.Before(func(gCtx context.Context, _ *godog.Scenario) (context.Context, error) {
		var err error
		sc, err = newScenarioCtx()
		if err != nil {
			return gCtx, err
		}
		return gCtx, nil
	})

	ctx.After(func(gCtx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
		if sc != nil {
			if err != nil {
				fmt.Fprintf(os.Stderr, "[BDD] scenario failed, temp dir: %s\n", sc.tmpDir)
				fmt.Fprintf(os.Stderr, "%s\n", sc.combinedOutput())
			}
			sc.cleanup()
		}
		return gCtx, nil
	})

	registerGivenSteps(ctx, &sc)
	registerWhenSteps(ctx, &sc)
	registerThenSteps(ctx, &sc)
}
```

Key details:
- **XDG isolation:** `XDG_CONFIG_HOME` and `XDG_DATA_HOME` redirected (used by `internal/xdg/xdg.go` for config, library, replay)
- **`NO_COLOR=1` + `TERM=dumb`:** Suppress color escape sequences in captured output
- **`cmd.Stdin = nil`:** Prevents hangs if CLI accidentally enters interactive mode
- **`exec.CommandContext` with 30s timeout:** Prevents hung hooks from blocking CI
- **`tagBinary` dir prepended to `PATH`:** Hooks can invoke `tag` itself
- **`google/shlex`:** Handles quoted args correctly (e.g., `--meta greeting='Hello World'`)
- **`{template}` placeholder:** Resolved in `runTag()`, avoids hardcoded paths in Gherkin
- **`filepath.EvalSymlinks`:** Resolves macOS `/var/folders` symlinks
- **`**scenarioCtx` pointer-to-pointer:** Steps always dereference the current scenario's context (safe with sequential execution)

---

## 5. Phase 2: Step Definitions

### 5.1 Given Steps

**File:** `tests/bdd/steps_given_test.go`

```go
package bdd_test

import (
	"fmt"
	"os"

	"github.com/cucumber/godog"
)

func registerGivenSteps(ctx *godog.ScenarioContext, sc **scenarioCtx) {
	ctx.Step(`^the "([^"]*)" template$`, func(name string) error {
		dir := (*sc).fixtureDir(name)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("fixture template %q not found at %s", name, dir)
		}
		(*sc).templateDir = dir
		return nil
	})

	ctx.Step(`^the "([^"]*)" template does not exist$`, func(name string) error {
		// Set templateDir to a path inside tmpDir that doesn't exist.
		// The When step will use this nonexistent path directly.
		(*sc).templateDir = ""
		return nil
	})
}
```

### 5.2 When Steps

**File:** `tests/bdd/steps_when_test.go`

```go
package bdd_test

import (
	"github.com/cucumber/godog"
)

func registerWhenSteps(ctx *godog.ScenarioContext, sc **scenarioCtx) {
	ctx.Step(`^I run tag "([^"]*)"$`, func(args string) error {
		return (*sc).runTag(args)
	})
}
```

The `runTag` method handles `{template}` placeholder resolution and shlex argument splitting.

### 5.3 Then Steps

**File:** `tests/bdd/steps_then_test.go`

All step functions return `error` with diagnostic context. Never use testify `assert`/`require`.

```go
package bdd_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

func registerThenSteps(ctx *godog.ScenarioContext, sc **scenarioCtx) {
	ctx.Step(`^the exit code should be (\d+)$`, func(expected int) error {
		if (*sc).exitCode != expected {
			return fmt.Errorf("expected exit code %d, got %d\n%s",
				expected, (*sc).exitCode, (*sc).combinedOutput())
		}
		return nil
	})

	ctx.Step(`^stdout should contain "([^"]*)"$`, func(text string) error {
		if !strings.Contains((*sc).stdout.String(), text) {
			return fmt.Errorf("stdout does not contain %q\n%s", text, (*sc).combinedOutput())
		}
		return nil
	})

	ctx.Step(`^stderr should contain "([^"]*)"$`, func(text string) error {
		if !strings.Contains((*sc).stderr.String(), text) {
			return fmt.Errorf("stderr does not contain %q\n%s", text, (*sc).combinedOutput())
		}
		return nil
	})

	ctx.Step(`^the directory "([^"]*)" should exist$`, func(relPath string) error {
		absPath := filepath.Join((*sc).tmpDir, relPath)
		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("directory %q does not exist: %w\n%s",
				relPath, err, (*sc).combinedOutput())
		}
		if !info.IsDir() {
			return fmt.Errorf("%q exists but is not a directory", relPath)
		}
		return nil
	})

	ctx.Step(`^the directory "([^"]*)" should not exist$`, func(relPath string) error {
		absPath := filepath.Join((*sc).tmpDir, relPath)
		if _, err := os.Stat(absPath); err == nil {
			return fmt.Errorf("directory %q should not exist but does", relPath)
		}
		return nil
	})

	ctx.Step(`^the file "([^"]*)" should exist$`, func(relPath string) error {
		absPath := filepath.Join((*sc).tmpDir, relPath)
		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("file %q does not exist: %w\n%s",
				relPath, err, (*sc).combinedOutput())
		}
		if info.IsDir() {
			return fmt.Errorf("%q is a directory, not a file", relPath)
		}
		return nil
	})

	ctx.Step(`^the file "([^"]*)" should not exist$`, func(relPath string) error {
		absPath := filepath.Join((*sc).tmpDir, relPath)
		if _, err := os.Stat(absPath); err == nil {
			return fmt.Errorf("file %q should not exist but does", relPath)
		}
		return nil
	})

	ctx.Step(`^the file "([^"]*)" should contain "([^"]*)"$`, func(relPath, text string) error {
		absPath := filepath.Join((*sc).tmpDir, relPath)
		content, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("cannot read %q: %w", relPath, err)
		}
		if !strings.Contains(string(content), text) {
			return fmt.Errorf("file %q does not contain %q\ncontents:\n%s",
				relPath, text, string(content))
		}
		return nil
	})

	ctx.Step(`^the file "([^"]*)" should be valid JSON$`, func(relPath string) error {
		absPath := filepath.Join((*sc).tmpDir, relPath)
		content, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("cannot read %q: %w", relPath, err)
		}
		if !json.Valid(content) {
			return fmt.Errorf("file %q is not valid JSON\ncontents:\n%s",
				relPath, string(content))
		}
		return nil
	})

	ctx.Step(`^the file "([^"]*)" should have JSON field "([^"]*)"$`, func(relPath, field string) error {
		absPath := filepath.Join((*sc).tmpDir, relPath)
		content, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("cannot read %q: %w", relPath, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(content, &doc); err != nil {
			return fmt.Errorf("file %q: invalid JSON: %w", relPath, err)
		}
		parts := strings.Split(field, ".")
		var current any = doc
		for _, part := range parts {
			m, ok := current.(map[string]any)
			if !ok {
				return fmt.Errorf("field %q: not an object at %q in %q", field, part, relPath)
			}
			current, ok = m[part]
			if !ok {
				return fmt.Errorf("field %q not found in %q\ncontents:\n%s",
					field, relPath, string(content))
			}
		}
		return nil
	})
}
```

### Step Regex Summary

| Step | Regex | Params |
|---|---|---|
| Given template | `^the "([^"]*)" template$` | name |
| Given no template | `^the "([^"]*)" template does not exist$` | name |
| When run tag | `^I run tag "([^"]*)"$` | arg string |
| Then exit code | `^the exit code should be (\d+)$` | int |
| Then stdout | `^stdout should contain "([^"]*)"$` | substring |
| Then stderr | `^stderr should contain "([^"]*)"$` | substring |
| Then dir exists | `^the directory "([^"]*)" should exist$` | path |
| Then dir not exists | `^the directory "([^"]*)" should not exist$` | path |
| Then file exists | `^the file "([^"]*)" should exist$` | path |
| Then file not exists | `^the file "([^"]*)" should not exist$` | path |
| Then file contains | `^the file "([^"]*)" should contain "([^"]*)"$` | path, text |
| Then file valid JSON | `^the file "([^"]*)" should be valid JSON$` | path |
| Then file JSON field | `^the file "([^"]*)" should have JSON field "([^"]*)"$` | path, dotted field |

All regexes anchored with `^...$`. Quoted params use `[^"]*` (supports hyphens, spaces).

---

## 6. Phase 3: Test Fixtures

### 6.1 Existing fixture (from examples/)

**`weather-api-go`** -- Real template with variables, hooks, wrapper directory. Used only if we add an `examples/`-based scenario.

### 6.2 New testdata fixtures

#### `minimal-template/tag.template.json`

```json
{
  "name": "Minimal Template",
  "version": "1.0.0",
  "vars": {
    "project_name": "my-project",
    "greeting": {
      "type": "string",
      "prompt": "Greeting message",
      "default": "Hello World"
    }
  }
}
```

#### `minimal-template/{{ vars.project_name }}/README.md`

```
# {{ vars.project_name }}

{{ vars.greeting }}
```

#### `template-with-hooks/tag.template.json`

```json
{
  "name": "Template With Hooks",
  "version": "1.0.0",
  "vars": {
    "project_name": "hooked-project"
  },
  "hooks": {
    "post_scaffold": ["touch hook-marker.txt"]
  }
}
```

#### `template-with-hooks/{{ vars.project_name }}/README.md`

```
# {{ vars.project_name }}
```

#### `template-with-required-var/tag.template.json`

```json
{
  "name": "Required Var Template",
  "version": "1.0.0",
  "vars": {
    "project_name": "required-project",
    "api_key": {
      "type": "string",
      "prompt": "Enter API key",
      "required": true
    }
  }
}
```

#### `template-with-required-var/{{ vars.project_name }}/README.md`

```
# {{ vars.project_name }}
```

#### `template-with-wrapper/tag.template.json`

```json
{
  "name": "Wrapper Template",
  "version": "1.0.0",
  "vars": {
    "project_name": "wrapped-project",
    "description": {
      "type": "string",
      "default": "A wrapped project"
    }
  }
}
```

#### `template-with-wrapper/{{ vars.project_name }}/README.md`

```
# {{ vars.project_name }}

{{ vars.description }}
```

#### `invalid-config/tag.template.json`

```json
{ this is not valid json
```

#### `missing-config/`

Empty directory (no `tag.template.json`). Create with `mkdir -p tests/bdd/testdata/missing-config`.

### 6.3 Hook Design

All hooks use inline commands only:
- `touch hook-marker.txt` -- creates a marker file in the output directory (post-scaffold CWD)
- `echo pre-hook-ran` -- produces stdout output

No external scripts. This ensures cross-platform compatibility and deterministic behavior.

---

## 7. Phase 4: Feature Files

### 7.1 scaffold_happy_path.feature

```gherkin
Feature: Scaffold happy path
  As a developer
  I want to scaffold projects from templates
  So that I can quickly start new projects with consistent structure

  # Design rule: ALL scenarios pass --no-input explicitly.
  # The subprocess is non-TTY (prompts would be skipped anyway),
  # but --no-input documents our intent to test non-interactive mode.

  Scenario: Basic scaffold with defaults
    Given the "minimal-template" template
    When I run tag "scaffold {template} my-app --no-input --force"
    Then the exit code should be 0
    And the directory "my-app" should exist
    And the file "my-app/README.md" should exist
    And the file "my-app/README.md" should contain "Hello World"

  Scenario: Scaffold with meta overrides
    Given the "minimal-template" template
    When I run tag "scaffold {template} custom-app --no-input --force --meta greeting='Howdy Partner'"
    Then the exit code should be 0
    And the directory "custom-app" should exist
    And the file "custom-app/README.md" should contain "Howdy Partner"

  Scenario: Post-scaffold hooks execute with --accept-hooks
    Given the "template-with-hooks" template
    When I run tag "scaffold {template} hooked --no-input --force --accept-hooks"
    Then the exit code should be 0
    And the directory "hooked" should exist
    And the file "hooked/hook-marker.txt" should exist
```

### 7.2 scaffold_errors.feature

```gherkin
Feature: Scaffold error handling
  As a developer
  I want clear error messages when scaffolding fails
  So that I can quickly diagnose and fix problems

  Scenario: Missing template directory
    Given the "nonexistent-template" template does not exist
    When I run tag "scaffold ./nonexistent-template my-project --no-input"
    Then the exit code should be 1

  Scenario: Missing tag.template.json
    Given the "missing-config" template
    When I run tag "scaffold {template} my-project --no-input"
    Then the exit code should be 1

  Scenario: Invalid tag.template.json
    Given the "invalid-config" template
    When I run tag "scaffold {template} my-project --no-input"
    Then the exit code should be 1

  Scenario: Required variable missing in non-interactive mode
    Given the "template-with-required-var" template
    When I run tag "scaffold {template} my-project --no-input --force"
    Then the exit code should be 1
    And stderr should contain "required"

  Scenario: Output directory already exists without --force
    Given the "minimal-template" template
    When I run tag "scaffold {template} existing-dir --no-input --force"
    And I run tag "scaffold {template} existing-dir --no-input"
    Then the exit code should be 1
    And stderr should contain "already exists"
```

### 7.3 scaffold_advanced.feature

```gherkin
Feature: Scaffold advanced behaviors
  As a developer
  I want the scaffold command to handle edge cases correctly
  So that output is consistent and predictable

  Scenario: Hooks are skipped without --accept-hooks in non-interactive mode
    Given the "template-with-hooks" template
    When I run tag "scaffold {template} skipped-hooks --no-input --force"
    Then the exit code should be 0
    And stdout should contain "Skipping hooks"
    And the file "skipped-hooks/hook-marker.txt" should not exist

  Scenario: Wrapper directory unwrapping avoids double nesting
    Given the "template-with-wrapper" template
    When I run tag "scaffold {template} my-wrapped --no-input --force"
    Then the exit code should be 0
    And the directory "my-wrapped" should exist
    And the file "my-wrapped/README.md" should exist
    And the directory "my-wrapped/my-wrapped" should not exist

  Scenario: Successful scaffold writes .tagconfig.json
    Given the "minimal-template" template
    When I run tag "scaffold {template} config-check --no-input --force"
    Then the exit code should be 0
    And the file "config-check/.tagconfig.json" should exist
    And the file "config-check/.tagconfig.json" should be valid JSON
    And the file "config-check/.tagconfig.json" should have JSON field "template"

  Scenario: Usage error returns exit code 2
    When I run tag "scaffold"
    Then the exit code should be 2
```

---

## 8. Phase 5: CI and Make Integration

### 8.1 Makefile target

**Modify:** `scripts/tests.mk`

Add:
```makefile
test-bdd: ## Run BDD acceptance tests
	@go test -v -timeout 120s ./tests/bdd/...
```

Update:
```makefile
test: test-unit test-integration test-bdd ## Run all tests
```

### 8.2 CI workflow

**Modify:** `.github/workflows/ci.yml`

Add step after integration tests:
```yaml
      - name: Run BDD acceptance tests
        run: go test -v -timeout 120s ./tests/bdd/...
```

---

## 9. Implementation Sequence

```
Phase 0: Dependencies & Setup (all independent)
  ├── go get godog
  ├── mkdir tests/bdd/{features,testdata}
  ├── Update scripts/tests.mk
  └── Update .github/workflows/ci.yml

Phase 1: Test Harness (depends on Phase 0)
  ├── bdd_test.go        [first -- defines tagBinary]
  └── context_test.go    [depends on bdd_test.go]

Phase 2: Step Definitions (depends on Phase 1, independent of each other)
  ├── steps_given_test.go
  ├── steps_when_test.go
  └── steps_then_test.go

Phase 3: Test Fixtures (independent of Phases 1-2, parallelizable with them)
  ├── minimal-template/
  ├── template-with-hooks/
  ├── template-with-required-var/
  ├── template-with-wrapper/
  ├── invalid-config/
  └── missing-config/

Phase 4: Feature Files (depends on Phases 2 + 3)
  ├── scaffold_happy_path.feature
  ├── scaffold_errors.feature
  └── scaffold_advanced.feature

Phase 5: Validation (depends on Phase 4)
  └── make test-bdd passes
```

---

## 10. Design Rules

### 10.1 All Scenarios Use `--no-input`

Every `When I run tag "scaffold ..."` MUST include `--no-input`. Reasons:
1. Documents intent: we are testing non-interactive mode
2. Prevents hangs if CLI enters interactive mode
3. `cmd.Stdin = nil` is defense-in-depth; `--no-input` is the application-level control

### 10.2 Step Functions Return `error`

Never use testify `assert`/`require` in step functions. Return `fmt.Errorf(...)` with:
- What was expected vs observed
- Combined stdout/stderr dump for diagnostics

```go
// CORRECT:
return fmt.Errorf("expected exit code %d, got %d\n%s", expected, actual, sc.combinedOutput())

// WRONG:
assert.Equal(t, expected, actual) // No *testing.T in godog steps
```

### 10.3 Step Regex Conventions

- All anchored: `^...$`
- Quoted params: `([^"]*)` (supports hyphens, spaces)
- Integer params: `(\d+)`

### 10.4 Hooks in Fixtures

Inline commands only: `touch`, `echo`. No script files. Cross-platform, deterministic.

### 10.5 The `{template}` Placeholder

`runTag()` resolves `{template}` to the absolute fixture path set by the preceding `Given` step. Gherkin files never contain hardcoded paths.

### 10.6 Error Assertions Use Stable Substrings

Assert on: `"required"`, `"already exists"`, `"config"`. NOT on full error messages which may change.

---

## 11. Known Gotchas

### 11.1 macOS Temp Dir Symlinks
`os.MkdirTemp` on macOS returns `/var/folders/...` which is a symlink to `/private/var/folders/...`. Use `filepath.EvalSymlinks` on all temp paths.

### 11.2 Post-scaffold Hook CWD
Post-scaffold hooks run in the OUTPUT directory (project root). Pre-scaffold hooks run in the TEMPLATE directory. `touch hook-marker.txt` in a post-scaffold hook creates the file in the project output, which is what tests assert.

### 11.3 XDG Directory Isolation
The CLI uses `XDG_CONFIG_HOME` and `XDG_DATA_HOME` via `internal/xdg/xdg.go` for config, library, and replay storage. Without redirecting these, tests could pollute real config.

### 11.4 `go build` Target
`go build .` MUST run with `cmd.Dir = repoRoot`. From `tests/bdd/` it would build the test package (no `main`), not the CLI.

### 11.5 Exit Code Semantics
From `pkg/app/errors.go`:
- `0` -- Success
- `1` (`ExitGeneral`) -- Application error (scaffold failure, missing var, bad config)
- `2` (`ExitUsage`) -- Usage error (missing template arg, invalid flags)
- `3` (`ExitNotFound`) -- Resource not found
- `130` (`ExitInterrupted`) -- SIGINT

### 11.6 Hooks Skipped Without `--accept-hooks`
When `--no-input` is set WITHOUT `--accept-hooks`, hooks are skipped with stdout message "Skipping hooks". Exit code is still 0.

### 11.7 Non-TTY Behavior
The subprocess has no TTY. The CLI detects this via `term.IsTerminal()`. Even without `--no-input`, prompts would be skipped. But `--no-input` makes intent explicit and is mandatory in all scenarios.

### 11.8 `--force` Flag
Most happy-path scenarios include `--force` to handle the case where output already exists. This prevents flakiness if cleanup fails. Error scenario 5 specifically omits `--force` to test the "already exists" error.

### 11.9 Cleanup Before `os.Exit`
`defer` does NOT execute after `os.Exit()`. TestMain must clean up explicitly before calling `os.Exit(code)`.

### 11.10 Binary Name
The BDD binary is built as `tag` in an isolated temp dir, not in the repo root. No collision with `make build` output.
