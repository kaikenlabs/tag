package engine

import (
	"github.com/kaikenlabs/tag/internal/parser"
	"github.com/kaikenlabs/tag/internal/writer"
)

type Core struct {
	parser parser.TemplateEngine
	fwr    writer.FileWriter
}

type Data struct {
	Name     string
	Args     string
	MetaArgs []string
}

type Generators struct {
	Name string `json:"name" yaml:"name"`
}

type Bundle struct {
	Name       string       `json:"name" yaml:"name"`
	Generators []Generators `json:"generators" yaml:"generators"`
}
