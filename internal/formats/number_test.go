package formats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_IsWholeNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		f    float64
		want bool
	}{
		{name: "positive integer", f: 5.0, want: true},
		{name: "zero", f: 0.0, want: true},
		{name: "negative integer", f: -3.0, want: true},
		{name: "positive fraction", f: 5.5, want: false},
		{name: "small fraction", f: 0.1, want: false},
		{name: "negative fraction", f: -2.7, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, IsWholeNumber(tt.f))
		})
	}
}
