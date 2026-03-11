package testrunner

import "time"

// Config holds the full configuration for a matrix test run.
type Config struct {
	TemplateDir string            // Path to template directory
	Meta        map[string]string // Required string variable overrides
	ValuesFile  string            // Path to values JSON file
	SkipVars    []string          // Boolean vars to exclude from permutation (use default)
	PinVars     map[string]string // Vars to fix at a specific value (not permuted)
	RunCommands []string          // Validation commands (overrides template config)
	Env         map[string]string // Extra environment variables for commands
	Filter      string            // Filter expression (index or key=value pairs)
	Parallel    int               // Max concurrent test runs
	FailFast    bool              // Stop on first failure
	DryRun      bool              // List combinations without running
	KeepFailed  bool              // Keep scaffolded dirs on failure
	Timeout     time.Duration     // Per-command timeout
	MaxCases    int               // Safety limit for total combinations (0 = unlimited)
	Verbose     bool              // Show full output on failures
	AcceptHooks bool              // Opt-in hook execution during scaffold
	Format      string            // Output format: "text" or "json"
}

// Combination represents a single set of boolean variable assignments.
type Combination struct {
	Index int               `json:"index"`
	Vars  map[string]string `json:"vars"`
}

// CaseStatus represents the outcome of a single test case.
type CaseStatus int

const (
	CasePassed CaseStatus = iota
	CaseFailed
	CaseErrored
)

// CaseResult holds the outcome of testing a single combination.
type CaseResult struct {
	Combination Combination   `json:"combination"`
	Status      CaseStatus    `json:"status"`
	Phase       string        `json:"phase,omitempty"`  // "scaffold", "validate:<cmd>", etc.
	Output      string        `json:"output,omitempty"` // Captured stdout+stderr on failure
	Error       string        `json:"error,omitempty"`  // Error message
	Duration    time.Duration `json:"duration"`
}

// Report aggregates all test case results.
type Report struct {
	Cases       []CaseResult  `json:"cases"`
	Passed      int           `json:"passed"`
	Failed      int           `json:"failed"`
	Errored     int           `json:"errored"`
	TotalCases  int           `json:"total_cases"`
	Duration    time.Duration `json:"duration"`
	TemplateDir string        `json:"template_dir"`
}

// ExitCode returns the appropriate process exit code for the report.
func (r Report) ExitCode() int {
	if r.Errored > 0 {
		return 2
	}
	if r.Failed > 0 {
		return 1
	}
	return 0
}
