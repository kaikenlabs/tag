package engine

import (
	"github.com/kaikenlabs/tag/internal/parser"
	"github.com/kaikenlabs/tag/internal/writer"
)

type Core struct {
	parser parser.TemplateEngine //nolint:staticcheck // legacy generate pipeline still active
	fwr    writer.FileWriter
}

type Data struct {
	Name     string
	Args     string
	MetaArgs []string
}

// GeneratorRef is a reference to a generator by name within a bundle configuration.
type GeneratorRef struct {
	Name string `json:"name" yaml:"name"`
}

type Bundle struct {
	Name       string         `json:"name" yaml:"name"`
	Generators []GeneratorRef `json:"generators" yaml:"generators"`
}
