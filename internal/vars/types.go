package vars

import "github.com/kaikenlabs/tag/internal/tmplconfig"

// Reference holds a single variable reference found in a template file.
type Reference struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Expression string `json:"expression"`
}

// DeclaredVar describes a variable declared in tag.template.json with its
// usage statistics across template files.
type DeclaredVar struct {
	Name           string      `json:"name"`
	Type           string      `json:"type"`
	Required       bool        `json:"required,omitempty"`
	Default        any         `json:"default,omitempty"`
	Options        []string    `json:"options,omitempty"`
	Derived        bool        `json:"derived,omitempty"`
	Private        bool        `json:"private,omitempty"`
	FileCount      int         `json:"file_count"`
	ReferenceCount int         `json:"reference_count"`
	References     []Reference `json:"references"`
}

// UndeclaredVar describes a variable used in templates but not declared in
// the config.
type UndeclaredVar struct {
	Name       string      `json:"name"`
	References []Reference `json:"references"`
}

// ScopeResult holds the analysis result for a single scope (root or generator).
type ScopeResult struct {
	Scope      string          `json:"scope"`
	Declared   []DeclaredVar   `json:"declared"`
	Undeclared []UndeclaredVar `json:"undeclared"`
	Unused     []string        `json:"unused"`
	Summary    Summary         `json:"summary"`
}

// Summary holds counts for the analysis result.
type Summary struct {
	Declared   int `json:"declared"`
	Undeclared int `json:"undeclared"`
	Unused     int `json:"unused"`
}

// Report holds the full analysis result across all scopes.
type Report struct {
	Root       ScopeResult   `json:"root"`
	Generators []ScopeResult `json:"generators"`
}

// HasIssues returns true if any scope has undeclared or unused variables.
func (r *Report) HasIssues() bool {
	if len(r.Root.Undeclared) > 0 || len(r.Root.Unused) > 0 {
		return true
	}
	for _, g := range r.Generators {
		if len(g.Undeclared) > 0 || len(g.Unused) > 0 {
			return true
		}
	}
	return false
}

// newDeclaredVar creates a DeclaredVar from a variable name and definition.
func newDeclaredVar(name string, def tmplconfig.VariableDef) DeclaredVar {
	typStr := string(def.Type)
	if typStr == "" {
		typStr = "string"
	}
	return DeclaredVar{
		Name:       name,
		Type:       typStr,
		Required:   def.Required,
		Default:    def.Default,
		Options:    def.Options,
		Derived:    def.IsDerived(),
		Private:    def.IsPrivate(name),
		References: []Reference{},
	}
}
