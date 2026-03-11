package testrunner_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/testrunner"
)

func TestUT_CaseStatus_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status testrunner.CaseStatus
		want   string
	}{
		{testrunner.CasePassed, "passed"},
		{testrunner.CaseFailed, "failed"},
		{testrunner.CaseErrored, "errored"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.status.String())
	}
}

func TestUT_CaseStatus_MarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status testrunner.CaseStatus
		want   string
	}{
		{testrunner.CasePassed, `"passed"`},
		{testrunner.CaseFailed, `"failed"`},
		{testrunner.CaseErrored, `"errored"`},
	}
	for _, tt := range tests {
		// Use pointer so *CaseStatus.MarshalJSON is invoked
		// (struct fields are addressable, so this matches real usage).
		data, err := json.Marshal(&tt.status)
		require.NoError(t, err)
		assert.Equal(t, tt.want, string(data))
	}
}

func TestUT_CaseStatus_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  testrunner.CaseStatus
	}{
		{`"passed"`, testrunner.CasePassed},
		{`"failed"`, testrunner.CaseFailed},
		{`"errored"`, testrunner.CaseErrored},
	}
	for _, tt := range tests {
		var got testrunner.CaseStatus
		err := json.Unmarshal([]byte(tt.input), &got)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got)
	}
}

func TestUT_CaseStatus_UnmarshalJSON_Invalid(t *testing.T) {
	t.Parallel()

	var s testrunner.CaseStatus
	err := json.Unmarshal([]byte(`"invalid"`), &s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown CaseStatus")
}

func TestUT_Report_ExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		report testrunner.Report
		want   int
	}{
		{name: "all passed", report: testrunner.Report{Passed: 3}, want: testrunner.ExitOK},
		{name: "some failed", report: testrunner.Report{Passed: 2, Failed: 1}, want: testrunner.ExitFailure},
		{name: "some errored", report: testrunner.Report{Passed: 2, Errored: 1}, want: testrunner.ExitError},
		{name: "errored wins over failed", report: testrunner.Report{Failed: 1, Errored: 1}, want: testrunner.ExitError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.report.ExitCode())
		})
	}
}
