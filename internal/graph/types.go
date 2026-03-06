package graph

// GraphReport holds the full generator dependency analysis result.
type GraphReport struct {
	Generators []GeneratorNode `json:"generators"`
	Bundles    []BundleInfo    `json:"bundles"`
	Markers    []MarkerInfo    `json:"markers"`
	Warnings   []Warning       `json:"warnings"`
}

// GeneratorNode represents a single generator and its file actions.
type GeneratorNode struct {
	Name    string       `json:"name"`
	Actions []ActionInfo `json:"actions"`
}

// ActionInfo describes a file operation performed by a generator template.
type ActionInfo struct {
	Type     string `json:"type"`               // "create", "inject", "append"
	Target   string `json:"target"`             // output file path (raw frontmatter value)
	Marker   string `json:"marker,omitempty"`   // for inject actions
	Position string `json:"position,omitempty"` // "after" or "before"
}

// BundleInfo describes a bundle and its generator execution order.
type BundleInfo struct {
	Name       string   `json:"name"`
	Order      []string `json:"order"`       // generator names in execution order
	ValidOrder bool     `json:"valid_order"` // true if create-before-inject respected
}

// MarkerInfo describes an injection marker found in scaffold source files.
type MarkerInfo struct {
	File   string   `json:"file"`
	Line   int      `json:"line"`
	Text   string   `json:"text"`
	UsedBy []string `json:"used_by"` // generator names that inject at this marker
}

// Warning represents a potential issue found during analysis.
type Warning struct {
	Code      string `json:"code"`      // e.g. "missing_target", "file_conflict", "order_violation"
	Generator string `json:"generator"` // generator name
	Message   string `json:"message"`
}
