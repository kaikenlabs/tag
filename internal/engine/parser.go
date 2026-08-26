package engine

import (
	"cmp"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
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
			Validate:      metadata.Validate,
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
// When scaffold vars are present, they form the base layer; meta vars override on collision.
func buildParserContext(input InputData) template.Context {
	// Start with scaffold vars as base layer
	vars := make(map[string]any)
	maps.Copy(vars, input.ScaffoldVars)

	// Override with generator meta vars (explicit values take precedence)
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
func mergeParserMetadata(cliMeta, templateMeta map[string]string) map[string]string {
	result := make(map[string]string)

	// Add template-defined metadata first
	maps.Copy(result, templateMeta)

	// Override with CLI metadata
	maps.Copy(result, cliMeta)

	return result
}

// LoadTemplateFiles loads all files from a directory as templates, skipping
// types.TemplateConfigFile: that name is reserved for configuration and carries
// no frontmatter, so loading it would fail the required 'to' field check.
func LoadTemplateFiles(dirPath string) (map[string]string, error) {
	rootTemplates := map[string]string{}
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return rootTemplates, err
	}

	for _, file := range files {
		if !isTemplateFileEntry(file) {
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

// isTemplateFileEntry reports whether entry is a file LoadTemplateFiles would
// load: not a directory, and not the reserved types.TemplateConfigFile.
func isTemplateFileEntry(entry os.DirEntry) bool {
	return !entry.IsDir() && entry.Name() != types.TemplateConfigFile
}

// HasTemplateFiles reports whether dir holds at least one file LoadTemplateFiles
// would load. It is what distinguishes a generator directory from an empty
// directory that merely shares a generator's name.
//
// It reports true when the directory cannot be read: an unreadable generator
// directory fails loudly today, and reporting false would make resolution skip
// it and silently fall through to a same-named bundle instead.
func HasTemplateFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return true
	}
	return slices.ContainsFunc(entries, isTemplateFileEntry)
}

// orderTemplateData sorts templates by action: Create → OpenAPI → Inject → Append.
func orderTemplateData(data []TemplateData) []TemplateData {
	priority := map[template.Action]int{
		template.ActionCreate:  0,
		template.ActionOpenAPI: 1,
		template.ActionInject:  2,
		template.ActionAppend:  3,
	}
	slices.SortStableFunc(data, func(a, b TemplateData) int {
		return cmp.Compare(priority[a.Action], priority[b.Action])
	})
	return data
}
