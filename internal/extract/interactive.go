package extract

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// PromptConfirmer implements Confirmer by prompting via a terminal reader/writer.
type PromptConfirmer struct {
	scanner *bufio.Scanner
	out     io.Writer
}

// NewPromptConfirmer creates a Confirmer that reads from in and writes prompts to out.
func NewPromptConfirmer(in io.Reader, out io.Writer) *PromptConfirmer {
	return &PromptConfirmer{
		scanner: bufio.NewScanner(in),
		out:     out,
	}
}

// Confirm displays an occurrence and asks the user whether to replace it.
func (p *PromptConfirmer) Confirm(occ Occurrence) (Decision, error) {
	fmt.Fprintf(p.out, "\nLine %d: %s\n", occ.LineNum, occ.Context)
	fmt.Fprintf(p.out, "  Replace %q → %s\n", occ.Rule.Needle, occ.Rule.Expr)
	fmt.Fprintf(p.out, "  [y]es / [n]o / [a]ll / [q]uit: ")

	if !p.scanner.Scan() {
		if err := p.scanner.Err(); err != nil {
			return DecisionQuit, fmt.Errorf("reading input: %w", err)
		}
		return DecisionQuit, nil
	}

	switch strings.ToLower(strings.TrimSpace(p.scanner.Text())) {
	case "y", "yes":
		return DecisionYes, nil
	case "n", "no":
		return DecisionNo, nil
	case "a", "all":
		return DecisionAll, nil
	case "q", "quit":
		return DecisionQuit, nil
	default:
		return DecisionNo, nil
	}
}
