package template

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/kaikenlabs/tag/internal/types"
)

// Metadata extraction and parsing errors.
var (
	ErrNoMetadataBlock          = errors.New("no metadata block found")
	ErrMalformedMetadata        = errors.New("malformed metadata line")
	ErrInvalidBoolValue         = errors.New("invalid boolean value")
	ErrConflictingAction        = errors.New("conflicting action: cannot have both append and inject")
	ErrConflictingOpenAPIAction = errors.New("conflicting action: openapi cannot be combined with append or inject")
	ErrMissingInjection         = errors.New("inject action requires before or after clause")
	ErrMissingToField           = errors.New("missing required 'to' field in metadata")
	ErrEmptyInjectMatcher       = errors.New("inject clause requires non-empty matcher value")
	ErrOrphanInjectClause       = errors.New("before/after clause without inject: true is ignored")
)

// Action represents the type of file operation to perform.
type Action string

const (
	ActionCreate  Action = "Create"
	ActionAppend  Action = "Append"
	ActionInject  Action = "Inject"
	ActionOpenAPI Action = "OpenAPI"
)

// Metadata represents the parsed metadata from a template's --- block.
type Metadata struct {
	To            string             // Output file path
	Action        Action             // File operation: Create, Append, Inject, or OpenAPI
	InjectClause  types.InjectClause // Before or After (for inject action)
	InjectMatcher string             // The marker string to match for injection
	Notes         string             // Optional notes to display after generation
	Description   string             // Optional description for generator listing
	Validate      bool               // Run OpenAPI validation after merge (openapi action only)
	Extra         map[string]string  // Additional key-value pairs from metadata
}

// Known metadata field names.
const (
	fieldTo       = "to"
	fieldAction   = "action"
	fieldAppend   = "append"
	fieldInject   = "inject"
	fieldAfter    = "after"
	fieldBefore   = "before"
	fieldNotes    = "notes"
	fieldDesc     = "desc"
	fieldValidate = "validate"
)

// Token constants for metadata parsing.
const (
	tokenNewLine   = "\n"
	tokenDash      = "---"
	tokenColon     = ":"
	tokenDashCount = 2
)

// ExtractMetadata splits a template into its metadata block and body.
// The metadata block is delimited by "---" markers at the start of the template.
// Returns the raw metadata string, the template body, and any error.
//
// Example:
//
//	---
//	to: output/file.go
//	inject: true
//	---
//	template body here
func ExtractMetadata(content string) (metaRaw, bodyRaw string, err error) {
	lines := strings.Split(content, tokenNewLine)
	var metaLines []string
	dashCount := 0
	bodyStartIndex := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == tokenDash {
			dashCount++
			if dashCount == tokenDashCount {
				// Found closing ---
				bodyStartIndex = i + 1
				break
			}
			continue
		}
		if dashCount == 1 {
			// Inside metadata block
			metaLines = append(metaLines, trimmed)
		}
	}

	if dashCount < tokenDashCount {
		// No complete metadata block found
		return "", content, ErrNoMetadataBlock
	}

	metaRaw = strings.Join(metaLines, tokenNewLine)

	if bodyStartIndex < len(lines) {
		bodyRaw = strings.Join(lines[bodyStartIndex:], tokenNewLine)
	}

	return metaRaw, bodyRaw, nil
}

// ParseMetadata parses rendered metadata lines into a Metadata struct.
// The input should be the metadata content after template rendering.
// Each line should be in "key: value" format.
//
//nolint:cyclop // metadata parsing handles multiple format variations
func ParseMetadata(rendered string) (*Metadata, error) {
	meta := &Metadata{
		Action: ActionCreate, // Default action
		Extra:  make(map[string]string),
	}

	lines := strings.Split(rendered, tokenNewLine)
	hasAppend := false
	hasInject := false
	hasOpenAPI := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Split on first colon only (value may contain colons)
		rawKey, rawValue, found := strings.Cut(trimmed, tokenColon)
		if !found {
			return nil, fmt.Errorf("%w: %q (missing colon)", ErrMalformedMetadata, trimmed)
		}

		key := strings.TrimSpace(rawKey)
		value := strings.TrimSpace(rawValue)

		switch key {
		case fieldTo:
			meta.To = value

		case fieldAction:
			if strings.EqualFold(value, "openapi") {
				hasOpenAPI = true
				meta.Action = ActionOpenAPI
			}

		case fieldAppend:
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("%w: %q for field %q", ErrInvalidBoolValue, value, fieldAppend)
			}
			if boolVal {
				hasAppend = true
				meta.Action = ActionAppend
			}

		case fieldInject:
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("%w: %q for field %q", ErrInvalidBoolValue, value, fieldInject)
			}
			if boolVal {
				hasInject = true
				meta.Action = ActionInject
			}

		case fieldAfter:
			meta.InjectClause = types.InjectAfter
			meta.InjectMatcher = value

		case fieldBefore:
			meta.InjectClause = types.InjectBefore
			meta.InjectMatcher = value

		case fieldNotes:
			meta.Notes = unquote(value)

		case fieldDesc:
			meta.Description = unquote(value)

		case fieldValidate:
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("%w: %q for field %q", ErrInvalidBoolValue, value, fieldValidate)
			}
			meta.Validate = boolVal

		default:
			// Store as extra metadata
			meta.Extra[key] = value
		}
	}

	return validateMetadata(meta, hasAppend, hasInject, hasOpenAPI)
}

// validateMetadata performs post-parse validation on metadata action flags.
func validateMetadata(meta *Metadata, hasAppend, hasInject, hasOpenAPI bool) (*Metadata, error) {
	if hasAppend && hasInject {
		return nil, ErrConflictingAction
	}

	if hasOpenAPI && (hasAppend || hasInject) {
		return nil, ErrConflictingOpenAPIAction
	}

	if hasInject && meta.InjectClause == "" {
		return nil, ErrMissingInjection
	}

	if hasInject && meta.InjectMatcher == "" {
		return nil, ErrEmptyInjectMatcher
	}

	// Clear orphan inject clauses (before/after without inject: true)
	if !hasInject && meta.InjectClause != "" {
		meta.InjectClause = ""
		meta.InjectMatcher = ""
	}

	return meta, nil
}

// IsOpenAPI returns true if this metadata specifies an OpenAPI merge action.
func (m *Metadata) IsOpenAPI() bool {
	return m.Action == ActionOpenAPI
}

// unquote strips matching surrounding double or single quotes from a string.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// RenderAndParseMetadata renders the raw metadata using the template engine,
// then parses the result into a Metadata struct.
// This is a convenience function that combines rendering and parsing.
func (e *Engine) RenderAndParseMetadata(metaRaw string, ctx Context) (*Metadata, error) {
	// Render the metadata with the template engine
	rendered, err := e.ExecuteToString(metaRaw, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to render metadata: %w", err)
	}

	// Parse the rendered metadata
	return ParseMetadata(rendered)
}
