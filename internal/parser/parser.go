package parser

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaikenlabs/tag/internal/template"
)

// NewWithExecutor creates a TemplateEngine using the provided TemplateExecutor.
// The caller is responsible for configuring the executor (e.g., setting loaders
// for shared template resolution) before passing it in.
func NewWithExecutor(executor template.TemplateExecutor, templates, sharedTemplates map[string]string) TemplateEngine {
	return TemplateEngine{
		gonjaEngine:     executor,
		templates:       templates,
		sharedTemplates: sharedTemplates,
	}
}

// New creates a new TemplateEngine that uses Gonja for template processing.
func New(dirPath string, sharedPath string, fileSuffix string) (TemplateEngine, error) {
	// Initialize Gonja engine
	gonjaEngine, err := template.NewEngine()
	if err != nil {
		slog.Error("cannot create template engine", "error", err)
		return TemplateEngine{}, err
	}

	// Load primary templates
	templates, err := withTemplates(dirPath, fileSuffix)
	if err != nil {
		slog.Error("cannot load templates", "error", err)
		return TemplateEngine{}, err
	}

	// Load shared templates (errors are non-fatal but logged)
	sharedTemplates, sharedErr := withTemplates(sharedPath, fileSuffix)
	if sharedErr != nil {
		slog.Debug("shared templates not loaded", "path", sharedPath, "error", sharedErr)
	}

	// Wire shared templates into Gonja's loader if any were loaded.
	// This must happen on the concrete engine before it is stored as the interface.
	if len(sharedTemplates) > 0 {
		loader := template.CreateMemoryLoaderFromMap(sharedTemplates)
		gonjaEngine.SetLoader(loader)
		slog.Debug("loaded shared templates", "count", len(sharedTemplates))
	}

	return NewWithExecutor(gonjaEngine, templates, sharedTemplates), nil
}

// Parse processes all loaded templates with the given input data.
// It returns a slice of TemplateData sorted by action (Create, Inject, Append).
func (te *TemplateEngine) Parse(input InputData) ([]TemplateData, error) {
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
func (te *TemplateEngine) parseTemplate(tmplName, tmplContent string, input InputData) (TemplateData, error) {
	// Stage 1: Extract metadata block and body
	metaRaw, bodyRaw, err := template.ExtractMetadata(tmplContent)
	if err != nil {
		// If no metadata block, treat entire content as body with default metadata
		metaRaw = ""
		bodyRaw = tmplContent
	}

	// Build the initial Gonja context
	ctx := buildContext(input)

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

	// Convert template.Metadata to parser.TemplateData
	return TemplateData{
		Name:   tmplName,
		To:     metadata.To,
		Output: parsedBody,
		ParseData: ParseData{
			Action:        convertAction(metadata.Action),
			InjectClause:  convertInjectClause(metadata.InjectClause),
			InjectMatcher: metadata.InjectMatcher,
			Notes:         metadata.Notes,
			Meta:          mergeMetadata(input.Meta, metadata.Extra),
		},
	}, nil
}

// renderBody renders the template body using Gonja.
func (te *TemplateEngine) renderBody(tmplName, body string, ctx template.Context) ([]byte, error) {
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

// buildContext creates a Gonja context from the input data using ContextBuilder.
func buildContext(input InputData) template.Context {
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

// mergeMetadata combines CLI metadata with template-defined metadata.
// CLI values take precedence over template-defined values.
func mergeMetadata(cliMeta map[string]string, templateMeta map[string]string) map[string]string {
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

// convertAction converts template.Action to parser.ParseActions.
func convertAction(action template.Action) ParseActions {
	switch action {
	case template.ActionAppend:
		return ActionAppend
	case template.ActionInject:
		return ActionInject
	default:
		return ActionCreate
	}
}

// convertInjectClause converts template.InjectClause to parser.InjectClause.
func convertInjectClause(clause template.InjectClause) InjectClause {
	switch clause {
	case template.InjectBefore:
		return InjectBefore
	case template.InjectAfter:
		return InjectAfter
	default:
		return ""
	}
}

// withTemplates loads templates from a directory.
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

// orderTemplateData sorts templates by action: Create first, then Inject, then Append.
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
