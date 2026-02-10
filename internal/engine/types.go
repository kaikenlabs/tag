package engine

import (
	"github.com/kaikenlabs/tag/internal/writer"
)

type Core struct {
	parser TemplateParser
	fwr    writer.FileWriter
}

type Data struct {
	Name         string
	Args         string
	MetaArgs     []string
	ScaffoldVars map[string]any // Variables from scaffold-time .tagconfig.json
}

// GeneratorRef is a reference to a generator by name within a bundle configuration.
type GeneratorRef struct {
	Name string `json:"name" yaml:"name"`
}

type Bundle struct {
	Name       string         `json:"name" yaml:"name"`
	Generators []GeneratorRef `json:"generators" yaml:"generators"`
}
