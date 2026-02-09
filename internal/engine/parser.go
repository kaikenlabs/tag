package engine

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kaikenlabs/tag/internal/template"
)

// NewParserWithExecutor creates a TemplateParser using the provided TemplateExecutor.
// The caller is responsible for configuring the executor (e.g., setting loaders
// for shared template resolution) before passing it in.
func NewParserWithExecutor(executor template.TemplateExecutor, templates, sharedTemplates map[string]string) TemplateParser {
	return TemplateParser{
		gonjaEngine:     executor,
		templates:       templates,
		sharedTemplates: sharedTemplates,
	}
}

// Parse processes all loaded templates with the given input data.
// It returns a slice of TemplateData sorted by action (Create, Inject, Append).
func (te *TemplateParser) Parse(input InputData) ([]TemplateData, error) {
	result := []TemplateData{}

	for name, tmplContent := range te.templates {
		data, err := te.parseTemplate(name, tmplContent, input)
		if err != nil {
			return result, err
		}
		result = append(result, data)
	}

	return orderTemplateData(result), nil
}

// parseTemplate processes a single template using Gonja.
// Stage 1: Extract and render metadata
// Stage 2: Parse metadata into TemplateData
// Stage 3: Render the template body
func (te *TemplateParser) parseTemplate(tmplName, tmplContent string, input InputData) (TemplateData, error) {
	// Stage 1: Extract metadata block and body
	metaRaw, bodyRaw, err := template.ExtractMetadata(tmplContent)
	if err != nil {
		// If no metadata block, treat entire content as body with default metadata
		metaRaw = ""
		bodyRaw = tmplContent
	}

	// Build the initial Gonja context
	ctx := buildParserContext(input)

	// Stage 2: Render and parse metadata (if present)
	var metadata *template.Metadata
	if metaRaw != "" {
		metadata, err = te.gonjaEngine.RenderAndParseMetadata(metaRaw, ctx)
		if err != nil {
			slog.Error("cannot parse metadata", "template", tmplName, "error", err)
			return TemplateData{}, err
		}
	} else {
		// Default metadata for templates without metadata block
		metadata = &template.Metadata{
			Action: template.ActionCreate,
			Extra:  make(map[string]string),
		}
	}

	// Validate required fields
	if metadata.To == "" {
		slog.Error("template missing required 'to' field", "template", tmplName)
		return TemplateData{}, fmt.Errorf("template %q: %w", tmplName, template.ErrMissingToField)
	}

	// Merge extra metadata into context vars for body rendering
	enrichContextWithMetadata(ctx, metadata)

	// Stage 3: Render the template body
	parsedBody, err := te.renderBody(tmplName, bodyRaw, ctx)
	if err != nil {
		slog.Error("cannot render template body", "template", tmplName, "error", err)
		return TemplateData{}, err
	}

	return TemplateData{
		Name:   tmplName,
		To:     metadata.To,
		Output: parsedBody,
		ParseData: ParseData{
			Action:        metadata.Action,
			InjectClause:  metadata.InjectClause,
			InjectMatcher: metadata.InjectMatcher,
			Notes:         metadata.Notes,
			Meta:          mergeParserMetadata(input.Meta, metadata.Extra),
		},
	}, nil
}

// renderBody renders the template body using Gonja.
func (te *TemplateParser) renderBody(tmplName, body string, ctx template.Context) ([]byte, error) {
	// Create base template
	tmpl, err := te.gonjaEngine.ParseStringNamed(body, tmplName)
	if err != nil {
		return nil, err
	}

	// Execute the template
	result, err := tmpl.Execute(ctx)
	if err != nil {
		return nil, err
	}

	return []byte(result), nil
}

// buildParserContext creates a Gonja context from the input data using ContextBuilder.
func buildParserContext(input InputData) template.Context {
	// Convert string metadata to any for vars
	vars := make(map[string]any)
	for k, v := range input.Meta {
		vars[k] = v
	}

	return template.NewContextBuilder().
		WithName(input.Name).
		WithVars(vars).
		Build()
}

// enrichContextWithMetadata adds extra metadata fields to the vars namespace.
func enrichContextWithMetadata(ctx template.Context, metadata *template.Metadata) {
	if metadata == nil || len(metadata.Extra) == 0 {
		return
	}

	vars, ok := ctx["vars"].(map[string]any)
	if !ok {
		vars = make(map[string]any)
		ctx["vars"] = vars
		ctx["cookiecutter"] = vars // Keep alias in sync
	}

	// Add extra metadata to vars (template-defined values)
	// These can be used in the body, e.g., {{ vars.custom_key }}
	for k, v := range metadata.Extra {
		// Only add if not already set (CLI values take precedence)
		if _, exists := vars[k]; !exists {
			vars[k] = v
		}
	}
}

// mergeParserMetadata combines CLI metadata with template-defined metadata.
// CLI values take precedence over template-defined values.
func mergeParserMetadata(cliMeta map[string]string, templateMeta map[string]string) map[string]string {
	result := make(map[string]string)

	// Add template-defined metadata first
	for k, v := range templateMeta {
		result[k] = v
	}

	// Override with CLI metadata
	for k, v := range cliMeta {
		result[k] = v
	}

	return result
}

// LoadTemplateFiles loads all files from a directory as templates.
func LoadTemplateFiles(dirPath string) (map[string]string, error) {
	rootTemplates := map[string]string{}
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return rootTemplates, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileLocation := filepath.Join(dirPath, file.Name())
		slog.Debug("loading template", "file", fileLocation)
		data, err := os.ReadFile(filepath.Clean(fileLocation))
		if err != nil {
			slog.Error("cannot read file", "file", fileLocation)
			return rootTemplates, err
		}
		rootTemplates[fileLocation] = string(data)
	}
	return rootTemplates, nil
}

// orderTemplateData sorts templates by action: Create first, then Inject, then Append.
func orderTemplateData(data []TemplateData) []TemplateData {
	create := []TemplateData{}
	inject := []TemplateData{}
	app := []TemplateData{}

	for _, tmp := range data {
		switch tmp.Action {
		case template.ActionCreate:
			create = append(create, tmp)
		case template.ActionInject:
			inject = append(inject, tmp)
		case template.ActionAppend:
			app = append(app, tmp)
		}
	}

	result := []TemplateData{}
	result = append(result, create...)
	result = append(result, inject...)
	result = append(result, app...)
	return result
}
