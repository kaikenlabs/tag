package convert

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ContentAnalyzer detects Jinja2/Gonja incompatibilities in template content.
type ContentAnalyzer struct{}

// NewContentAnalyzer creates a new content analyzer.
func NewContentAnalyzer() *ContentAnalyzer {
	return &ContentAnalyzer{}
}

// Patterns for detecting incompatibilities
var (
	// Filter with parentheses: |filter(arg) - Jinja2 style
	// Should be |filter:"arg" or |filter:arg in Gonja
	filterParenRegex = regexp.MustCompile(`\|\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)`)

	// dict.items() iteration pattern
	dictItemsRegex = regexp.MustCompile(`\.items\(\)`)

	// Macro definition
	macroRegex = regexp.MustCompile(`\{%\s*macro\s+`)

	// Template extends
	extendsRegex = regexp.MustCompile(`\{%\s*extends\s+`)

	// Import statement
	importRegex = regexp.MustCompile(`\{%\s*import\s+`)

	// Raw block start
	rawBlockRegex = regexp.MustCompile(`\{%\s*raw\s*%\}`)

	// Raw block end
	rawEndRegex = regexp.MustCompile(`\{%\s*endraw\s*%\}`)

	// Set statement with complex expression
	setComplexRegex = regexp.MustCompile(`\{%\s*set\s+\w+\s*=\s*[^%]*\.`)
)

// Analyze scans content for Jinja2/Gonja incompatibilities.
func (a *ContentAnalyzer) Analyze(path string, content []byte) []Incompatibility {
	// Skip binary files
	if !isTextContent(content) {
		return nil
	}

	var findings []Incompatibility
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	inRawBlock := false

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Track raw blocks - don't analyze inside them
		if rawBlockRegex.MatchString(line) {
			inRawBlock = true
			continue
		}
		if rawEndRegex.MatchString(line) {
			inRawBlock = false
			continue
		}
		if inRawBlock {
			continue
		}

		// Check for filter parentheses syntax
		if matches := filterParenRegex.FindAllStringSubmatch(line, -1); len(matches) > 0 {
			for _, match := range matches {
				filterName := match[1]
				args := match[2]
				original := match[0]

				// Construct suggested replacement - leave args as-is for user to decide on quoting
				suggestion := "|" + filterName + ":" + strings.Trim(args, "'\"")

				findings = append(findings, Incompatibility{
					Path:       path,
					Line:       lineNum,
					Kind:       "filter-syntax",
					Message:    "Jinja2 filter syntax uses parentheses; Gonja uses colons. Review if arg needs quoting.",
					Original:   original,
					Suggestion: suggestion,
					Severity:   SeverityWarning,
				})
			}
		}

		// Check for dict.items()
		if dictItemsRegex.MatchString(line) {
			findings = append(findings, Incompatibility{
				Path:       path,
				Line:       lineNum,
				Kind:       "dict-iteration",
				Message:    "Jinja2 dict.items() should be converted to direct dict iteration in Gonja",
				Original:   "dict.items()",
				Suggestion: "{% for k, v in dict %}",
				Severity:   SeverityWarning,
			})
		}

		// Check for macro definitions
		if macroRegex.MatchString(line) {
			findings = append(findings, Incompatibility{
				Path:     path,
				Line:     lineNum,
				Kind:     "macro",
				Message:  "Jinja2 macro syntax; Gonja uses {% func %} instead",
				Original: strings.TrimSpace(line),
				Severity: SeverityWarning,
			})
		}

		// Check for extends (template inheritance)
		if extendsRegex.MatchString(line) {
			findings = append(findings, Incompatibility{
				Path:     path,
				Line:     lineNum,
				Kind:     "extends",
				Message:  "Template inheritance may work but needs verification with Gonja",
				Original: strings.TrimSpace(line),
				Severity: SeverityInfo,
			})
		}

		// Check for import statements
		if importRegex.MatchString(line) {
			findings = append(findings, Incompatibility{
				Path:     path,
				Line:     lineNum,
				Kind:     "import",
				Message:  "Jinja2 import statement; verify compatibility with Gonja",
				Original: strings.TrimSpace(line),
				Severity: SeverityInfo,
			})
		}

		// Check for complex set statements
		if setComplexRegex.MatchString(line) {
			findings = append(findings, Incompatibility{
				Path:     path,
				Line:     lineNum,
				Kind:     "set-complex",
				Message:  "Complex set statement with method calls may need adjustment",
				Original: strings.TrimSpace(line),
				Severity: SeverityInfo,
			})
		}
	}

	// Check for scanner errors (e.g., lines too long)
	if err := scanner.Err(); err != nil {
		findings = append(findings, Incompatibility{
			Path:     path,
			Kind:     "scan-error",
			Message:  "Error scanning file: " + err.Error(),
			Severity: SeverityWarning,
		})
	}

	return findings
}

// AnalyzeString is a convenience method for analyzing string content.
func (a *ContentAnalyzer) AnalyzeString(path, content string) []Incompatibility {
	return a.Analyze(path, []byte(content))
}

// isTextContent checks if content appears to be text rather than binary.
func isTextContent(content []byte) bool {
	if len(content) == 0 {
		return true
	}

	// Check first 8KB for binary indicators
	checkLen := len(content)
	if checkLen > 8192 {
		checkLen = 8192
	}
	sample := content[:checkLen]

	// Check for null bytes (strong binary indicator)
	if bytes.Contains(sample, []byte{0}) {
		return false
	}

	// Check if it's valid UTF-8
	if !utf8.Valid(sample) {
		// Not valid UTF-8, likely binary
		return false
	}

	// Count non-printable characters
	nonPrintable := 0
	for _, b := range sample {
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			nonPrintable++
		}
	}

	// If more than 10% non-printable, likely binary
	return float64(nonPrintable)/float64(len(sample)) < 0.1
}

// KnownGonjaFilters is a list of filters supported by Gonja.
// This can be used to identify unsupported Jinja2 filters.
var KnownGonjaFilters = map[string]bool{
	// Standard Gonja filters
	"abs":            true,
	"attr":           true,
	"batch":          true,
	"capitalize":     true,
	"center":         true,
	"default":        true,
	"d":              true, // alias for default
	"dictsort":       true,
	"escape":         true,
	"e":              true, // alias for escape
	"filesizeformat": true,
	"first":          true,
	"float":          true,
	"forceescape":    true,
	"format":         true,
	"groupby":        true,
	"indent":         true,
	"int":            true,
	"join":           true,
	"last":           true,
	"length":         true,
	"list":           true,
	"lower":          true,
	"map":            true,
	"max":            true,
	"min":            true,
	"pprint":         true,
	"random":         true,
	"reject":         true,
	"rejectattr":     true,
	"replace":        true,
	"reverse":        true,
	"round":          true,
	"safe":           true,
	"select":         true,
	"selectattr":     true,
	"slice":          true,
	"sort":           true,
	"string":         true,
	"striptags":      true,
	"sum":            true,
	"title":          true,
	"trim":           true,
	"truncate":       true,
	"unique":         true,
	"upper":          true,
	"urlencode":      true,
	"urlize":         true,
	"wordcount":      true,
	"wordwrap":       true,
	"xmlattr":        true,

	// TAG custom filters
	"snake":      true,
	"pascal":     true,
	"camel":      true,
	"kebab":      true,
	"plural":     true,
	"singular":   true,
	"ordinalize": true,
	"titleize":   true,
	"humanize":   true,
	"split":      true,
	"contains":   true,
	"hasprefix":  true,
	"hassuffix":  true,
}
