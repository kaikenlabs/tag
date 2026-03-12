package testrunner_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/testrunner"
)

func TestUT_PrintTextReport_AllPassed(t *testing.T) {
	t.Parallel()

	report := testrunner.Report{
		Cases: []testrunner.CaseResult{
			{
				CaseName:    "default",
				Combination: testrunner.Combination{Index: 0, Vars: map[string]string{"a": "false"}},
				Status:      testrunner.CasePassed,
				Duration:    150 * time.Millisecond,
			},
			{
				CaseName:    "default",
				Combination: testrunner.Combination{Index: 1, Vars: map[string]string{"a": "true"}},
				Status:      testrunner.CasePassed,
				Duration:    200 * time.Millisecond,
			},
		},
		Passed:     2,
		TotalCases: 2,
		Duration:   400 * time.Millisecond,
	}

	var buf bytes.Buffer
	testrunner.PrintTextReport(&buf, report, []string{"a"}, false)
	output := buf.String()

	assert.Contains(t, output, "✓")
	assert.Contains(t, output, "[0]")
	assert.Contains(t, output, "[1]")
	assert.Contains(t, output, "2 passed, 0 failed, 0 errored")
	assert.Contains(t, output, "All combinations passed.")
}

func TestUT_PrintTextReport_WithFailure(t *testing.T) {
	t.Parallel()

	report := testrunner.Report{
		Cases: []testrunner.CaseResult{
			{
				CaseName:    "default",
				Combination: testrunner.Combination{Index: 0, Vars: map[string]string{"a": "false"}},
				Status:      testrunner.CasePassed,
				Duration:    100 * time.Millisecond,
			},
			{
				CaseName:    "default",
				Combination: testrunner.Combination{Index: 1, Vars: map[string]string{"a": "true"}},
				Status:      testrunner.CaseFailed,
				Phase:       "validate: go build ./...",
				Error:       `command "go build ./..." failed (exit 1)`,
				Output:      "compilation error\ndetails here",
				Duration:    200 * time.Millisecond,
			},
		},
		Passed:     1,
		Failed:     1,
		TotalCases: 2,
		Duration:   300 * time.Millisecond,
	}

	var buf bytes.Buffer
	testrunner.PrintTextReport(&buf, report, []string{"a"}, false)
	output := buf.String()

	assert.Contains(t, output, "✗")
	assert.Contains(t, output, "Phase:")
	assert.Contains(t, output, "Error:")
	assert.NotContains(t, output, "Output:")
	assert.Contains(t, output, "1 passed, 1 failed, 0 errored")
	assert.NotContains(t, output, "All combinations passed.")
}

func TestUT_PrintTextReport_VerboseShowsOutput(t *testing.T) {
	t.Parallel()

	report := testrunner.Report{
		Cases: []testrunner.CaseResult{
			{
				CaseName:    "default",
				Combination: testrunner.Combination{Index: 0, Vars: map[string]string{"a": "true"}},
				Status:      testrunner.CaseFailed,
				Phase:       "validate: test",
				Error:       "failed",
				Output:      "error details",
				Duration:    100 * time.Millisecond,
			},
		},
		Failed:     1,
		TotalCases: 1,
		Duration:   100 * time.Millisecond,
	}

	var buf bytes.Buffer
	testrunner.PrintTextReport(&buf, report, []string{"a"}, true)
	output := buf.String()

	assert.Contains(t, output, "Output:")
	assert.Contains(t, output, "error details")
}

func TestUT_PrintTextReport_KeptDirShown(t *testing.T) {
	t.Parallel()

	report := testrunner.Report{
		Cases: []testrunner.CaseResult{
			{
				CaseName:    "default",
				Combination: testrunner.Combination{Index: 0, Vars: map[string]string{"a": "true"}},
				Status:      testrunner.CaseFailed,
				Phase:       "validate: test",
				Error:       "failed",
				KeptDir:     "/tmp/tag-test-0-abc123",
				Duration:    100 * time.Millisecond,
			},
		},
		Failed:     1,
		TotalCases: 1,
		Duration:   100 * time.Millisecond,
	}

	var buf bytes.Buffer
	testrunner.PrintTextReport(&buf, report, []string{"a"}, false)
	output := buf.String()

	assert.Contains(t, output, "Kept:  /tmp/tag-test-0-abc123")
}

func TestUT_PrintTextReport_SortsByIndex(t *testing.T) {
	t.Parallel()

	report := testrunner.Report{
		Cases: []testrunner.CaseResult{
			{
				CaseName:    "default",
				Combination: testrunner.Combination{Index: 2, Vars: map[string]string{"a": "true"}},
				Status:      testrunner.CasePassed,
				Duration:    100 * time.Millisecond,
			},
			{
				CaseName:    "default",
				Combination: testrunner.Combination{Index: 0, Vars: map[string]string{"a": "false"}},
				Status:      testrunner.CasePassed,
				Duration:    100 * time.Millisecond,
			},
		},
		Passed:     2,
		TotalCases: 2,
		Duration:   200 * time.Millisecond,
	}

	var buf bytes.Buffer
	testrunner.PrintTextReport(&buf, report, []string{"a"}, false)
	output := buf.String()

	// [0] should appear before [2]
	idx0 := strings.Index(output, "[0]")
	idx2 := strings.Index(output, "[2]")
	assert.Less(t, idx0, idx2, "results should be sorted by index")
}

func TestUT_PrintJSONReport(t *testing.T) {
	t.Parallel()

	report := testrunner.Report{
		Cases: []testrunner.CaseResult{
			{
				CaseName:    "build",
				Combination: testrunner.Combination{Index: 0, Vars: map[string]string{"a": "true"}},
				Status:      testrunner.CasePassed,
				Duration:    100 * time.Millisecond,
			},
		},
		Passed:      1,
		TotalCases:  1,
		Duration:    100 * time.Millisecond,
		TemplateDir: "/tmp/tpl",
	}

	var buf bytes.Buffer
	err := testrunner.PrintJSONReport(&buf, report)
	require.NoError(t, err)

	// Verify valid JSON.
	var decoded map[string]any
	err = json.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)

	// CaseStatus should be a string, not a number.
	cases := decoded["cases"].([]any)
	firstCase := cases[0].(map[string]any)
	assert.Equal(t, "passed", firstCase["status"])
	assert.Equal(t, "build", firstCase["case_name"])
}

func TestUT_PrintDryRun(t *testing.T) {
	t.Parallel()

	plan := &testrunner.TestPlan{
		BoolVars: []string{"a", "b"},
		Cases: []testrunner.TestCasePlan{
			{
				Name: "default",
				Combos: []testrunner.Combination{
					{Index: 0, Vars: map[string]string{"a": "false", "b": "false"}},
					{Index: 1, Vars: map[string]string{"a": "true", "b": "false"}},
				},
			},
		},
	}

	var buf bytes.Buffer
	testrunner.PrintDryRun(&buf, plan)
	output := buf.String()

	assert.Contains(t, output, "2 total")
	assert.Contains(t, output, "[0]")
	assert.Contains(t, output, "[1]")
	assert.Contains(t, output, "a=false b=false")
	assert.Contains(t, output, "a=true b=false")
}

func TestUT_PrintTextReport_MultipleCasesGrouped(t *testing.T) {
	t.Parallel()

	report := testrunner.Report{
		Cases: []testrunner.CaseResult{
			{
				CaseName:    "build",
				Combination: testrunner.Combination{Index: 0, Vars: map[string]string{"a": "false"}},
				Status:      testrunner.CasePassed,
				Duration:    100 * time.Millisecond,
			},
			{
				CaseName:    "lint",
				Combination: testrunner.Combination{Index: 0, Vars: map[string]string{"a": "false"}},
				Status:      testrunner.CasePassed,
				Duration:    150 * time.Millisecond,
			},
		},
		Passed:     2,
		TotalCases: 2,
		Duration:   250 * time.Millisecond,
	}

	var buf bytes.Buffer
	testrunner.PrintTextReport(&buf, report, []string{"a"}, false)
	output := buf.String()

	assert.Contains(t, output, "── build ──")
	assert.Contains(t, output, "── lint ──")
	assert.Contains(t, output, "2 passed, 0 failed, 0 errored")
}

func TestUT_PrintDryRun_MultipleCases(t *testing.T) {
	t.Parallel()

	plan := &testrunner.TestPlan{
		BoolVars: []string{"a"},
		Cases: []testrunner.TestCasePlan{
			{
				Name: "build",
				Combos: []testrunner.Combination{
					{Index: 0, Vars: map[string]string{"a": "false"}},
				},
			},
			{
				Name: "lint",
				Combos: []testrunner.Combination{
					{Index: 0, Vars: map[string]string{"a": "false"}},
					{Index: 1, Vars: map[string]string{"a": "true"}},
				},
			},
		},
	}

	var buf bytes.Buffer
	testrunner.PrintDryRun(&buf, plan)
	output := buf.String()

	assert.Contains(t, output, "3 total")
	assert.Contains(t, output, "── build ──")
	assert.Contains(t, output, "── lint ──")
}
