package openapi

import (
	"errors"
	"fmt"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// ConflictPolicy defines how to handle merge conflicts.
type ConflictPolicy string

const (
	// ConflictError returns an error when a key exists with different content.
	ConflictError ConflictPolicy = "error"
	// ConflictSkip silently skips conflicting keys.
	ConflictSkip ConflictPolicy = "skip"
)

// MergeOptions controls merge behavior.
type MergeOptions struct {
	ValidateResult bool           // run kin-openapi validation after merge
	ConflictPolicy ConflictPolicy // skip | error (default: error)
}

// MergeResult describes what the merge operation did.
type MergeResult struct {
	Changed        bool
	AddedPaths     []string
	AddedSchemas   []string
	SkippedPaths   []string
	SkippedSchemas []string
	Conflicts      []string
}

// Editor provides structural merge of OpenAPI YAML fragments.
type Editor struct{}

// NewEditor creates a new OpenAPI editor.
func NewEditor() *Editor {
	return &Editor{}
}

// Merge merges an OpenAPI YAML fragment into a target spec.
// The fragment should contain top-level `paths` and/or `components` sections.
// Returns the merged YAML, a result summary, and any error.
func (e *Editor) Merge(spec, fragment []byte, opts MergeOptions) ([]byte, MergeResult, error) {
	if opts.ConflictPolicy == "" {
		opts.ConflictPolicy = ConflictError
	}

	specFile, err := parser.ParseBytes(spec, parser.ParseComments)
	if err != nil {
		return nil, MergeResult{}, fmt.Errorf("parse spec: %w", err)
	}

	fragFile, err := parser.ParseBytes(fragment, parser.ParseComments)
	if err != nil {
		return nil, MergeResult{}, fmt.Errorf("parse fragment: %w", err)
	}

	if len(specFile.Docs) == 0 {
		return nil, MergeResult{}, errors.New("spec has no documents")
	}
	if len(fragFile.Docs) == 0 {
		return nil, MergeResult{}, errors.New("fragment has no documents")
	}

	specRoot, ok := specFile.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return nil, MergeResult{}, errors.New("spec root is not a mapping node")
	}

	fragRoot, ok := fragFile.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return nil, MergeResult{}, errors.New("fragment root is not a mapping node")
	}

	var result MergeResult

	// Merge each top-level section from fragment into spec
	for _, fragKV := range fragRoot.Values {
		fragKey := fragKV.Key.String()

		switch fragKey {
		case "paths":
			if err := e.mergePaths(specRoot, fragKV, opts, &result); err != nil {
				return nil, result, err
			}
		case "components":
			if err := e.mergeComponents(specRoot, fragKV, opts, &result); err != nil {
				return nil, result, err
			}
		default:
			return nil, result, fmt.Errorf("unsupported top-level fragment key: %s (expected paths or components)", fragKey)
		}
	}

	out := specFile.String()
	return []byte(out), result, nil
}

// mergePaths merges fragment paths into the spec's paths section.
func (e *Editor) mergePaths(specRoot *ast.MappingNode, fragPathsKV *ast.MappingValueNode, opts MergeOptions, result *MergeResult) error {
	fragPaths, ok := fragPathsKV.Value.(*ast.MappingNode)
	if !ok {
		return errors.New("fragment paths is not a mapping")
	}

	specPathsKV := findMappingKey(specRoot, "paths")
	if specPathsKV == nil {
		// No paths section in spec — add the entire fragment section
		specRoot.Values = append(specRoot.Values, fragPathsKV)
		for _, pKV := range fragPaths.Values {
			result.AddedPaths = append(result.AddedPaths, pKV.Key.String())
		}
		result.Changed = true
		return nil
	}

	specPaths, ok := specPathsKV.Value.(*ast.MappingNode)
	if !ok {
		// Value might be empty flow-style {} which is not a MappingNode
		// Replace the entire value with the fragment paths
		specPathsKV.Value = fragPaths
		for _, pKV := range fragPaths.Values {
			result.AddedPaths = append(result.AddedPaths, pKV.Key.String())
		}
		result.Changed = true
		return nil
	}

	// If spec paths is flow-style and empty, replace it (only if fragment has values)
	if specPaths.IsFlowStyle && len(specPaths.Values) == 0 && len(fragPaths.Values) > 0 {
		specPathsKV.Value = fragPaths
		for _, pKV := range fragPaths.Values {
			result.AddedPaths = append(result.AddedPaths, pKV.Key.String())
		}
		result.Changed = true
		return nil
	}

	return e.mergeMapping(specPaths, fragPaths, opts, &result.AddedPaths, &result.SkippedPaths, &result.Conflicts, &result.Changed)
}

// mergeComponents merges fragment components into the spec's components section.
func (e *Editor) mergeComponents(specRoot *ast.MappingNode, fragComponentsKV *ast.MappingValueNode, opts MergeOptions, result *MergeResult) error {
	fragComponents, ok := fragComponentsKV.Value.(*ast.MappingNode)
	if !ok {
		return errors.New("fragment components is not a mapping")
	}

	specComponentsKV := findMappingKey(specRoot, "components")

	// Handle components.schemas specifically
	for _, fragSubKV := range fragComponents.Values {
		subKey := fragSubKV.Key.String()
		if subKey != "schemas" {
			return fmt.Errorf("unsupported components sub-key: %s (expected schemas)", subKey)
		}

		fragSchemas, ok := fragSubKV.Value.(*ast.MappingNode)
		if !ok {
			return errors.New("fragment components.schemas is not a mapping")
		}

		if specComponentsKV == nil {
			// No components section — add entire fragment
			specRoot.Values = append(specRoot.Values, fragComponentsKV)
			for _, sKV := range fragSchemas.Values {
				result.AddedSchemas = append(result.AddedSchemas, sKV.Key.String())
			}
			result.Changed = true
			return nil
		}

		specComponents, ok := specComponentsKV.Value.(*ast.MappingNode)
		if !ok {
			return errors.New("spec components is not a mapping")
		}

		specSchemasKV := findMappingKey(specComponents, "schemas")
		if specSchemasKV == nil {
			// Components exists but no schemas — add the schemas sub-key
			specComponents.Values = append(specComponents.Values, fragSubKV)
			for _, sKV := range fragSchemas.Values {
				result.AddedSchemas = append(result.AddedSchemas, sKV.Key.String())
			}
			result.Changed = true
			return nil
		}

		specSchemas, ok := specSchemasKV.Value.(*ast.MappingNode)
		if !ok || (specSchemas.IsFlowStyle && len(specSchemas.Values) == 0 && len(fragSchemas.Values) > 0) {
			// Not a mapping or empty flow-style {} with fragment values — replace
			specSchemasKV.Value = fragSchemas
			for _, sKV := range fragSchemas.Values {
				result.AddedSchemas = append(result.AddedSchemas, sKV.Key.String())
			}
			result.Changed = true
			return nil
		}

		if err := e.mergeMapping(specSchemas, fragSchemas, opts, &result.AddedSchemas, &result.SkippedSchemas, &result.Conflicts, &result.Changed); err != nil {
			return err
		}
	}

	return nil
}

// mergeMapping merges key-value pairs from src into dst.
// For each key: if missing → add; if identical → skip; if different → conflict policy.
func (e *Editor) mergeMapping(
	dst, src *ast.MappingNode,
	opts MergeOptions,
	added, skipped, conflicts *[]string,
	changed *bool,
) error {
	for _, srcKV := range src.Values {
		key := srcKV.Key.String()
		dstKV := findMappingKey(dst, key)

		if dstKV == nil {
			// Key doesn't exist — add it
			dst.Values = append(dst.Values, srcKV)
			*added = append(*added, key)
			*changed = true
			continue
		}

		// Key exists — compare values
		if nodesEqual(dstKV.Value, srcKV.Value) {
			*skipped = append(*skipped, key)
			continue
		}

		// Conflict
		switch opts.ConflictPolicy {
		case ConflictSkip:
			*skipped = append(*skipped, key)
		default: // ConflictError
			*conflicts = append(*conflicts, key)
			return fmt.Errorf("conflict on key %q: existing content differs from fragment", key)
		}
	}

	return nil
}

// findMappingKey finds a key-value node in a mapping by key name.
func findMappingKey(m *ast.MappingNode, key string) *ast.MappingValueNode {
	for _, kv := range m.Values {
		if kv.Key.String() == key {
			return kv
		}
	}
	return nil
}

// nodesEqual compares two AST nodes by their serialized YAML output,
// ignoring whitespace differences.
func nodesEqual(a, b ast.Node) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return normalizeYAML(a.String()) == normalizeYAML(b.String())
}
