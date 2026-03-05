// Package extract provides functionality to extract generator templates from
// existing source files by detecting entity name occurrences and replacing them
// with parameterized template expressions.
package extract

import "io"

// Options configures the extract operation.
type Options struct {
	Name        string    // Entity name to extract (normalized to lowercase).
	As          string    // Generator name (output directory under .tag/).
	DryRun      bool      // If true, print preview without writing files.
	Interactive bool      // If true, confirm each replacement interactively.
	TagDir      string    // .tag directory path (from --path flag).
	Writer      io.Writer // Output writer for dry-run/preview.
	Prompter    Confirmer // For interactive mode.
}

// Result holds the outcome of an extract operation.
type Result struct {
	TemplatePath string // Where the template was written.
	ToPath       string // Parameterized to: path.
	Replacements int    // Number of replacements made.
	Content      string // Generated template content.
}

// Rule maps a literal search needle to its template expression replacement.
type Rule struct {
	Needle string // Search text, e.g. "Users".
	Expr   string // Template expression, e.g. "{{ name | plural | pascal }}".
}

// Occurrence represents a single match of a Rule within the source content.
type Occurrence struct {
	Start   int    // Byte offset in content.
	End     int    // Byte offset end (exclusive).
	Rule    Rule   // Which rule matched.
	LineNum int    // 1-indexed line number.
	Context string // The full line for display.
}

// Decision represents a user's interactive choice for a replacement.
type Decision int

const (
	// DecisionYes accepts this replacement.
	DecisionYes Decision = iota
	// DecisionNo skips this replacement.
	DecisionNo
	// DecisionAll accepts this and all remaining replacements.
	DecisionAll
	// DecisionQuit stops processing entirely.
	DecisionQuit
)

// Confirmer prompts the user to confirm individual replacements.
type Confirmer interface {
	Confirm(occ Occurrence) (Decision, error)
}
