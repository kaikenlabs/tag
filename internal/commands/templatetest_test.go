package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kaikenlabs/tag/internal/templatetest"
)

func TestUT_PrintTestReport_AllPassed(t *testing.T) {
	t.Parallel()
	report := templatetest.Report{
		Cases: []templatetest.CaseResult{
			{Name: "test-one", Status: templatetest.CasePassed},
			{Name: "test-two", Status: templatetest.CasePassed},
		},
		Passed: 2,
	}

	var buf bytes.Buffer
	printTestReport(&buf, report)

	out := buf.String()
	assert.Contains(t, out, "test-one")
	assert.Contains(t, out, "test-two")
	assert.Contains(t, out, "2 passed, 0 failed, 0 errored")
	assert.Contains(t, out, "All tests passed.")
}

func TestUT_PrintTestReport_WithFailures(t *testing.T) {
	t.Parallel()
	report := templatetest.Report{
		Cases: []templatetest.CaseResult{
			{Name: "ok-test", Status: templatetest.CasePassed},
			{
				Name:   "bad-test",
				Status: templatetest.CaseFailed,
				Assertions: []templatetest.AssertionResult{
					{Passed: false, Detail: "expected foo got bar"},
				},
			},
		},
		Passed: 1,
		Failed: 1,
	}

	var buf bytes.Buffer
	printTestReport(&buf, report)

	out := buf.String()
	assert.Contains(t, out, "bad-test")
	assert.Contains(t, out, "FAIL: expected foo got bar")
	assert.Contains(t, out, "1 passed, 1 failed, 0 errored")
	assert.NotContains(t, out, "All tests passed.")
}

func TestUT_PrintTestReport_WithErrors(t *testing.T) {
	t.Parallel()
	report := templatetest.Report{
		Cases: []templatetest.CaseResult{
			{Name: "errored-test", Status: templatetest.CaseErrored, Error: "fixture load failed"},
		},
		Errored: 1,
	}

	var buf bytes.Buffer
	printTestReport(&buf, report)

	out := buf.String()
	assert.Contains(t, out, "errored-test")
	assert.Contains(t, out, "fixture load failed")
	assert.Contains(t, out, "0 passed, 0 failed, 1 errored")
}

func TestUT_PrintTestReport_Empty(t *testing.T) {
	t.Parallel()
	report := templatetest.Report{}

	var buf bytes.Buffer
	printTestReport(&buf, report)

	out := buf.String()
	assert.Contains(t, out, "0 passed, 0 failed, 0 errored")
	assert.Contains(t, out, "No test fixtures found.")
}

func TestUT_PrintTestReport_Mixed(t *testing.T) {
	t.Parallel()
	report := templatetest.Report{
		Cases: []templatetest.CaseResult{
			{Name: "pass", Status: templatetest.CasePassed},
			{
				Name:   "fail",
				Status: templatetest.CaseFailed,
				Assertions: []templatetest.AssertionResult{
					{Passed: true, Detail: "ok"},
					{Passed: false, Detail: "mismatch"},
				},
			},
			{Name: "error", Status: templatetest.CaseErrored, Error: "boom"},
		},
		Passed:  1,
		Failed:  1,
		Errored: 1,
	}

	var buf bytes.Buffer
	printTestReport(&buf, report)

	out := buf.String()
	assert.Contains(t, out, "1 passed, 1 failed, 1 errored")
	// Failed assertion detail is shown, passing ones are not
	assert.Contains(t, out, "FAIL: mismatch")
	assert.NotContains(t, out, "FAIL: ok")
}
