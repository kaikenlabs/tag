package parser

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gobuffalo/flect"
	"github.com/kaikenlabs/tag/internal/formats"
)

const (
	caseSnake           = "caseSnake"
	caseKebab           = "caseKebab"
	casePascal          = "casePascal"
	caseLower           = "caseLower"
	caseTitle           = "caseTitle"
	caseCamel           = "caseCamel"
	pluralise           = "pluralise"
	singularise         = "singularise"
	ordinalize          = "ordinalize"
	titleize            = "titleize"
	humanize            = "humanize"
	splitByDelimiter    = "splitByDelimiter"
	splitAfterDelimiter = "splitAfterDelimiter"
	contains            = "contains"
	hasPrefix           = "hasPrefix"
	hasSuffix           = "hasSuffix"
)

var defaultFuncs = template.FuncMap{
	caseSnake:  formats.CaseSnake,
	caseKebab:  formats.CaseKebab,
	casePascal: formats.CasePascal,
	caseLower:  strings.ToLower,
	caseTitle:  strings.ToTitle,
	caseCamel:  formats.CaseCamel,
	// Inflections
	pluralise:   flect.Pluralize,
	singularise: flect.Singularize,
	ordinalize:  flect.Ordinalize,
	titleize:    flect.Titleize,
	humanize:    flect.Humanize,
	// String manipulations
	splitByDelimiter:    formats.SplitByDelimiter,
	splitAfterDelimiter: formats.SplitAfterDelimiter,
	contains:            formats.Contains,
	hasPrefix:           formats.HasPrefix,
	hasSuffix:           formats.HasSuffix,
}

func New(dirPath string, sharedPath string, fileSuffix string) (TemplateEngine, error) {
	tmp, err := withTemplates(dirPath, fileSuffix)
	if err != nil {
		slog.Error("cannot load templates", "error", err)
		return TemplateEngine{}, err
	}
	sharedTmp, _ := withTemplates(sharedPath, fileSuffix)
	return TemplateEngine{
		templates:       tmp,
		sharedTemplates: sharedTmp,
		funcs:           defaultFuncs,
	}, nil
}

func (te *TemplateEngine) Parse(data TemplateData) ([]TemplateData, error) {
	result := []TemplateData{}
	for name, tmpl := range te.templates {
		newData, err := parse(name, tmpl, data, te.funcs, te.sharedTemplates)
		if err != nil {
			return result, err
		}
		result = append(result, newData)
	}
	return orderTemplateData(result), nil
}

// withTemplates - load templates by file path
func withTemplates(dirPath string, fileSuffix string) (map[string]string, error) {
	rootTemplates := map[string]string{}
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return rootTemplates, err
	}

	for _, file := range files {
		fileLocation := filepath.Join(dirPath, file.Name())
		if strings.HasSuffix(file.Name(), fileSuffix) {
			slog.Debug("loading template", "file", fileLocation)
			data, err := os.ReadFile(filepath.Clean(fileLocation))
			if err != nil {
				slog.Error("cannot read file", "file", fileLocation)
				return rootTemplates, err
			}
			rootTemplates[fileLocation] = string(data)
		}
	}
	return rootTemplates, nil
}

func orderTemplateData(data []TemplateData) []TemplateData {
	create := []TemplateData{}
	inject := []TemplateData{}
	app := []TemplateData{}

	for _, tmp := range data {
		switch tmp.Action {
		case ActionCreate:
			create = append(create, tmp)
		case ActionInject:
			inject = append(inject, tmp)
		case ActionAppend:
			app = append(app, tmp)
		}
	}
	result := []TemplateData{}
	result = append(result, create...)
	result = append(result, inject...)
	result = append(result, app...)
	return result
}
