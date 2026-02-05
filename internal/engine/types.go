package engine

import (
	"gitlab.com/Vitrifi/tag/internal/parser"
	"gitlab.com/Vitrifi/tag/internal/writer"
)

type Core struct {
	parser parser.TemplateEngine
	fwr    writer.Write
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
