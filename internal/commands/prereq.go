package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaikenlabs/tag/internal/tmplconfig"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/pkg/app"
)

// isTruthy returns true if the value is considered "truthy" for prerequisite
// checking. A value is truthy if it is non-nil, non-empty string, non-zero
// number, or boolean true. JSON unmarshaling produces float64 for numbers.
// readGeneratorConfig reads a generator's tag.template.json, returning an empty
// config when the generator declares none. A missing file is not an error — most
// generators have none — but anything else is, so a config TAG cannot read or
// parse can never silently disable the requires gate that same file declares.
// Before the file was skipped as a template, an unreadable or malformed one
// aborted the run by accident; the gate must not depend on that.
func readGeneratorConfig(genDir string) (*tmplconfig.TemplateConfig, error) {
	data, err := os.ReadFile(filepath.Join(genDir, types.TemplateConfigFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &tmplconfig.TemplateConfig{}, nil
		}
		return nil, app.Errorf("cannot read %s: %w", types.TemplateConfigFile, err)
	}

	config, err := tmplconfig.ParseTemplateConfig(data)
	if err != nil {
		return nil, app.Errorf("cannot parse %s: %w", types.TemplateConfigFile, err)
	}
	return config, nil
}

// checkGeneratorRequires enforces the requires gate a generator declares in its
// own tag.template.json, for both a direct run and a bundled one.
func checkGeneratorRequires(genDir, name string, vars map[string]any) error {
	config, err := readGeneratorConfig(genDir)
	if err != nil {
		return err
	}
	return checkRequirements(name, "generator", config.Requires, vars)
}

func isTruthy(v any) bool {
	switch val := v.(type) {
	case nil:
		return false
	case bool:
		return val
	case string:
		return val != ""
	case float64:
		return val != 0
	case int:
		return val != 0
	default:
		// Unknown types (slices, maps, etc.) are truthy if non-nil.
		return true
	}
}

// requirementsMet returns true if all entries in requires are present and truthy
// in vars. Returns true when requires is empty (no prerequisites).
func requirementsMet(requires []string, vars map[string]any) bool {
	for _, req := range requires {
		val, ok := vars[req]
		if !ok || !isTruthy(val) {
			return false
		}
	}
	return true
}

// checkRequirements verifies that all entries in requires are present and truthy
// in vars. Returns an error listing all unmet requirements, or nil if all are met.
func checkRequirements(name, kind string, requires []string, vars map[string]any) error {
	if len(requires) == 0 {
		return nil
	}

	var unmet []string
	seen := make(map[string]struct{}, len(requires))
	for _, req := range requires {
		if _, dup := seen[req]; dup {
			continue
		}
		seen[req] = struct{}{}

		val, ok := vars[req]
		if !ok || !isTruthy(val) {
			unmet = append(unmet, req)
		}
	}

	if len(unmet) == 0 {
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s %q requires the following variables to be enabled:\n", kind, name)
	for _, u := range unmet {
		if _, ok := vars[u]; !ok {
			fmt.Fprintf(&b, "  - %s (not set in .tagconfig.json)\n", u)
		} else {
			fmt.Fprintf(&b, "  - %s (currently disabled)\n", u)
		}
	}
	b.WriteString("  hint: re-scaffold with the required variables enabled to use this " + kind)
	return app.Errorf("%s", b.String())
}
