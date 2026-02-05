package template

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Metadata extraction and parsing errors.
var (
	ErrNoMetadataBlock    = errors.New("no metadata block found")
	ErrMalformedMetadata  = errors.New("malformed metadata line")
	ErrInvalidBoolValue   = errors.New("invalid boolean value")
	ErrConflictingAction  = errors.New("conflicting action: cannot have both append and inject")
	ErrMissingInjection   = errors.New("inject action requires before or after clause")
	ErrMissingToField     = errors.New("missing required 'to' field in metadata")
	ErrEmptyInjectMatcher = errors.New("inject clause requires non-empty matcher value")
	ErrOrphanInjectClause = errors.New("before/after clause without inject: true is ignored")
)

// Action represents the type of file operation to perform.
type Action string

const (
	ActionCreate Action = "Create"
	ActionAppend Action = "Append"
	ActionInject Action = "Inject"
)

// InjectClause represents where to inject content relative to a marker.
type InjectClause string

const (
	InjectBefore InjectClause = "Before"
	InjectAfter  InjectClause = "After"
)

// Metadata represents the parsed metadata from a template's --- block.
type Metadata struct {
	To            string            // Output file path
	Action        Action            // File operation: Create, Append, or Inject
	InjectClause  InjectClause      // Before or After (for inject action)
	InjectMatcher string            // The marker string to match for injection
	Notes         string            // Optional notes to display after generation
	Extra         map[string]string // Additional key-value pairs from metadata
}

// Known metadata field names.
const (
	fieldTo     = "to"
	fieldAppend = "append"
	fieldInject = "inject"
	fieldAfter  = "after"
	fieldBefore = "before"
	fieldNotes  = "notes"
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
func ExtractMetadata(content string) (metaRaw string, bodyRaw string, err error) {
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
func ParseMetadata(rendered string) (*Metadata, error) {
	meta := &Metadata{
		Action: ActionCreate, // Default action
		Extra:  make(map[string]string),
	}

	lines := strings.Split(rendered, tokenNewLine)
	hasAppend := false
	hasInject := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Split on first colon only (value may contain colons)
		idx := strings.Index(trimmed, tokenColon)
		if idx == -1 {
			return nil, fmt.Errorf("%w: %q (missing colon)", ErrMalformedMetadata, trimmed)
		}

		key := strings.TrimSpace(trimmed[:idx])
		value := strings.TrimSpace(trimmed[idx+1:])

		switch key {
		case fieldTo:
			meta.To = value

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
			meta.InjectClause = InjectAfter
			meta.InjectMatcher = value

		case fieldBefore:
			meta.InjectClause = InjectBefore
			meta.InjectMatcher = value

		case fieldNotes:
			meta.Notes = value

		default:
			// Store as extra metadata
			meta.Extra[key] = value
		}
	}

	// Validate conflicting actions
	if hasAppend && hasInject {
		return nil, ErrConflictingAction
	}

	// Validate inject requires before/after
	if hasInject && meta.InjectClause == "" {
		return nil, ErrMissingInjection
	}

	// Validate inject matcher is non-empty when inject is set
	if hasInject && meta.InjectMatcher == "" {
		return nil, ErrEmptyInjectMatcher
	}

	// Clear orphan inject clauses (before/after without inject: true)
	// This is a warning condition but we handle it gracefully
	if !hasInject && meta.InjectClause != "" {
		meta.InjectClause = ""
		meta.InjectMatcher = ""
	}

	return meta, nil
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
