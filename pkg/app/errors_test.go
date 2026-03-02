package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_CommandError_Error(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "simple message",
			message: "something went wrong",
			want:    "something went wrong",
		},
		{
			name:    "empty message",
			message: "",
			want:    "",
		},
		{
			name:    "message with special characters",
			message: "error: file 'test.txt' not found",
			want:    "error: file 'test.txt' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &CommandError{Message: tt.message}
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

func TestUT_CommandError_Unwrap(t *testing.T) {
	t.Run("with cause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := &CommandError{Message: "test", Cause: cause}

		assert.Equal(t, cause, err.Unwrap())
	})

	t.Run("nil cause", func(t *testing.T) {
		err := &CommandError{Message: "test", Cause: nil}

		assert.Nil(t, err.Unwrap())
	})
}

func TestUT_CommandError_ErrorsIs(t *testing.T) {
	sentinelErr := errors.New("sentinel error")

	t.Run("matches wrapped error", func(t *testing.T) {
		err := &CommandError{Message: "wrapped", Cause: sentinelErr}

		assert.ErrorIs(t, err, sentinelErr)
	})

	t.Run("does not match different error", func(t *testing.T) {
		err := &CommandError{Message: "wrapped", Cause: errors.New("other")}

		assert.NotErrorIs(t, err, sentinelErr)
	})

	t.Run("does not match with nil cause", func(t *testing.T) {
		err := &CommandError{Message: "no cause", Cause: nil}

		assert.NotErrorIs(t, err, sentinelErr)
	})
}

func TestUT_CommandError_ErrorsAs(t *testing.T) {
	t.Run("can extract CommandError", func(t *testing.T) {
		originalErr := &CommandError{Message: "test message", Cause: errors.New("cause")}
		wrappedErr := fmt.Errorf("wrapped: %w", originalErr)

		var cmdErr *CommandError
		require.ErrorAs(t, wrappedErr, &cmdErr)
		assert.Equal(t, "test message", cmdErr.Message)
	})

	t.Run("returns false for non-CommandError", func(t *testing.T) {
		regularErr := errors.New("regular error")

		var cmdErr *CommandError
		assert.False(t, errors.As(regularErr, &cmdErr))
	})
}

func TestUT_CommandError_ExitCode(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"default code (0) returns 1", 0, ExitGeneral},
		{"explicit code 2", ExitUsage, ExitUsage},
		{"explicit code 3", ExitNotFound, ExitNotFound},
		{"explicit code 130", ExitInterrupted, ExitInterrupted},
		{"custom code 42", 42, 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &CommandError{Message: "test", Code: tt.code}
			assert.Equal(t, tt.want, err.ExitCode())
		})
	}
}

func TestUT_UsageErrorf(t *testing.T) {
	err := UsageErrorf("missing argument: %s", "name")

	cmdErr, ok := err.(*CommandError)
	require.True(t, ok)
	assert.Equal(t, ExitUsage, cmdErr.ExitCode())
	assert.Equal(t, "missing argument: name", cmdErr.Message)
}

func TestUT_NotFoundErrorf(t *testing.T) {
	err := NotFoundErrorf("template %q not found", "foo")

	cmdErr, ok := err.(*CommandError)
	require.True(t, ok)
	assert.Equal(t, ExitNotFound, cmdErr.ExitCode())
	assert.Equal(t, `template "foo" not found`, cmdErr.Message)
}

func TestUT_CommandError_ImplementsExitCoder(t *testing.T) {
	// Verify CommandError has an ExitCode() int method
	// (compatible with cli.ExitCoder interface)
	err := &CommandError{Message: "test", Code: ExitUsage}

	type exitCoder interface {
		ExitCode() int
	}

	var ec exitCoder = err
	assert.Equal(t, ExitUsage, ec.ExitCode())
}

func TestUT_Errorf_FormatsMessage(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []any
		want   string
	}{
		{
			name:   "no args",
			format: "simple message",
			args:   nil,
			want:   "simple message",
		},
		{
			name:   "string arg",
			format: "error: %s",
			args:   []any{"file not found"},
			want:   "error: file not found",
		},
		{
			name:   "multiple args",
			format: "error in %s at line %d",
			args:   []any{"test.go", 42},
			want:   "error in test.go at line 42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Errorf(tt.format, tt.args...)

			cmdErr, ok := err.(*CommandError)
			require.True(t, ok, "Errorf() should return *CommandError")
			assert.Equal(t, tt.want, cmdErr.Message)
		})
	}
}

func TestUT_Errorf_WrapsError(t *testing.T) {
	t.Run("wraps error with %w", func(t *testing.T) {
		cause := errors.New("underlying cause")
		err := Errorf("operation failed: %w", cause)

		cmdErr, ok := err.(*CommandError)
		require.True(t, ok, "Errorf() should return *CommandError")
		require.NotNil(t, cmdErr.Cause, "Errorf() with %%w should set Cause")
		assert.ErrorIs(t, err, cause)
	})

	t.Run("no cause without %w", func(t *testing.T) {
		cause := errors.New("not wrapped")
		err := Errorf("operation failed: %v", cause)

		_, ok := err.(*CommandError)
		require.True(t, ok, "Errorf() should return *CommandError")
		assert.NotErrorIs(t, err, cause, "errors.Is() should not find cause when using %%v")
	})

	t.Run("message includes cause text", func(t *testing.T) {
		cause := errors.New("disk full")
		err := Errorf("write failed: %w", cause)

		assert.Equal(t, "write failed: disk full", err.Error())
	})
}

func TestUT_Errorf_MultiWrap(t *testing.T) {
	err1 := errors.New("first cause")
	err2 := errors.New("second cause")

	err := Errorf("failed: %w and %w", err1, err2)

	require.Error(t, err)
	assert.ErrorIs(t, err, err1, "errors.Is should find first cause through multi-wrap chain")
	assert.ErrorIs(t, err, err2, "errors.Is should find second cause through multi-wrap chain")
}
