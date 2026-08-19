// Package dialect provides canonical type-to-language mapping via dialect registries.
// A dialect maps canonical type names (e.g., "string", "uuid", "datetime") to
// language-specific spellings (e.g., Go's "string", Postgres's "UUID").
package dialect

import "errors"

// Sentinel errors for dialect resolution.
var (
	// ErrUnknownDialect is returned when resolving a type against a dialect that
	// does not exist in the registry.
	ErrUnknownDialect = errors.New("unknown dialect")

	// ErrUnmappedType is returned when a canonical type has no mapping in the
	// requested dialect.
	ErrUnmappedType = errors.New("unmapped canonical type")
)

// Dialect represents a single dialect with its type mappings.
type Dialect struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Types       map[string]string `yaml:"types" json:"types"`
}

// Registry holds a collection of named dialects and resolves canonical types
// to dialect-specific types.
type Registry struct {
	dialects map[string]*Dialect
}
