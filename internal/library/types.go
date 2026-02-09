// Package library provides a persistent template library for TAG.
package library

import "time"

// Entry represents an installed template in the library.
type Entry struct {
	Name          string    `json:"name"`
	Source        string    `json:"source"` // Original ref (e.g., "gh:company/go-api")
	AddedAt       time.Time `json:"added_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Version       string    `json:"version,omitempty"`        // From tag.template.json
	Description   string    `json:"description,omitempty"`    // From tag.template.json
	ConvertedFrom string    `json:"converted_from,omitempty"` // "cookiecutter" or ""
}

// Registry holds all installed library entries.
type Registry struct {
	Version int               `json:"version"`
	Entries map[string]*Entry `json:"entries"`
}

const registryVersion = 1

// AddOptions configures the lib add operation.
type AddOptions struct {
	Ref   string // Remote or local template reference
	Name  string // Override name (--as flag); empty = auto-derive from ref
	Force bool   // Overwrite existing template with same name
}

// AddResult describes what happened during lib add.
type AddResult struct {
	Name          string
	Source        string
	TemplateDir   string   // Final stored path
	ConvertedFrom string   // "cookiecutter" or ""
	Warnings      []string // Conversion warnings
	IsUpdate      bool     // true if replaced existing entry
}
