package engine

import (
	"io"

	"github.com/kaikenlabs/tag/internal/writer"
)

type Core struct {
	parser TemplateParser
	fwr    writer.FileWriter
	out    io.Writer
}

type Data struct {
	Name         string
	RawMeta      []string
	ScaffoldVars map[string]any // Variables from scaffold-time .tagconfig.json
}

// GeneratorRef is a reference to a generator by name within a bundle configuration.
type GeneratorRef struct {
	Name string `json:"name" yaml:"name"`
}

type Bundle struct {
	Name          string         `json:"name" yaml:"name"`
	Description   string         `json:"description,omitempty" yaml:"description,omitempty"`
	SelfContained bool           `json:"self_contained,omitempty" yaml:"self_contained,omitempty"`
	Generators    []GeneratorRef `json:"generators" yaml:"generators"`
}
