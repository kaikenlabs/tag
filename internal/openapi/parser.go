package openapi

import (
	"errors"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// Spec wraps a parsed OpenAPI YAML AST with indexed access to top-level sections.
type Spec struct {
	File   *ast.File
	Root   *ast.MappingNode
	Indent int // detected indentation (2 or 4 spaces)
}

// Parse errors.
var (
	ErrMultiDocument = errors.New("multi-document YAML not supported for OpenAPI specs")
	ErrNoDocument    = errors.New("YAML has no documents")
	ErrNotMapping    = errors.New("document root is not a YAML mapping")
)

// ParseSpec parses an OpenAPI YAML spec into an indexed Spec structure.
func ParseSpec(data []byte) (*Spec, error) {
	file, err := parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	if len(file.Docs) == 0 {
		return nil, ErrNoDocument
	}
	if len(file.Docs) > 1 {
		return nil, ErrMultiDocument
	}

	root, ok := file.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return nil, ErrNotMapping
	}

	indent := detectIndent(data)

	return &Spec{
		File:   file,
		Root:   root,
		Indent: indent,
	}, nil
}

// Paths returns the `paths` mapping node, or nil if absent.
func (s *Spec) Paths() *ast.MappingNode {
	kv := findMappingKey(s.Root, "paths")
	if kv == nil {
		return nil
	}
	m, _ := kv.Value.(*ast.MappingNode)
	return m
}

// Schemas returns the `components.schemas` mapping node, or nil if absent.
func (s *Spec) Schemas() *ast.MappingNode {
	componentsKV := findMappingKey(s.Root, "components")
	if componentsKV == nil {
		return nil
	}
	components, ok := componentsKV.Value.(*ast.MappingNode)
	if !ok {
		return nil
	}
	schemasKV := findMappingKey(components, "schemas")
	if schemasKV == nil {
		return nil
	}
	m, _ := schemasKV.Value.(*ast.MappingNode)
	return m
}

// HasPaths returns true if the spec has a `paths` section (even if empty).
func (s *Spec) HasPaths() bool {
	return findMappingKey(s.Root, "paths") != nil
}

// HasSchemas returns true if the spec has a `components.schemas` section (even if empty).
func (s *Spec) HasSchemas() bool {
	return s.Schemas() != nil
}

// HasComponents returns true if the spec has a `components` section.
func (s *Spec) HasComponents() bool {
	return findMappingKey(s.Root, "components") != nil
}

// String serializes the spec back to YAML, preserving comments and formatting.
func (s *Spec) String() string {
	return s.File.String()
}

// detectIndent detects the dominant indentation (2 or 4 spaces) from YAML content.
// Defaults to 2 if detection is ambiguous.
func detectIndent(data []byte) int {
	twoCount := 0
	fourCount := 0

	for line := range strings.SplitSeq(string(data), "\n") {
		if line == "" || line[0] != ' ' {
			continue
		}
		// Count leading spaces
		spaces := 0
		for _, ch := range line {
			if ch != ' ' {
				break
			}
			spaces++
		}
		if spaces == 0 {
			continue
		}
		if spaces%4 == 0 {
			fourCount++
		}
		if spaces%2 == 0 {
			twoCount++
		}
	}

	// If 4-space lines dominate and aren't just a subset of 2-space, use 4
	if fourCount > 0 && fourCount == twoCount {
		return 4
	}
	return 2
}
