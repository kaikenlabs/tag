package lint

// Severity defines the severity level of a linting issue.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Issue represents a single linting issue found during validation.
type Issue struct {
	File     string   `json:"file"`
	Line     int      `json:"line,omitempty"`
	Column   int      `json:"column,omitempty"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Rule     string   `json:"rule"`
}

// Result holds all issues found during a linting run.
type Result struct {
	Issues []Issue `json:"issues"`
}

// Add appends a new issue to the result.
func (r *Result) Add(issue Issue) {
	r.Issues = append(r.Issues, issue)
}

// HasErrors returns true if any issue has error severity.
func (r *Result) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of error-severity issues.
func (r *Result) ErrorCount() int {
	n := 0
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			n++
		}
	}
	return n
}

// WarningCount returns the number of warning-severity issues.
func (r *Result) WarningCount() int {
	n := 0
	for _, issue := range r.Issues {
		if issue.Severity == SeverityWarning {
			n++
		}
	}
	return n
}
