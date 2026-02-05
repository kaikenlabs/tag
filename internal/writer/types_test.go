package writer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInject_Validate(t *testing.T) {
	tests := []struct {
		name    string
		matcher string
		clause  InjectClause
		want    error
	}{
		{
			name:    "should return false if both are missing",
			matcher: "",
			clause:  "",
			want:    ErrNoMatchingClause,
		},
		{
			name:    "should return false if clause is missing",
			matcher: "// flash",
			clause:  "",
			want:    ErrNoMatchingClause,
		},
		{
			name:    "should return false if matcher is missing",
			matcher: "",
			clause:  InjectAfter,
			want:    ErrNoMatchingExpression,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &Inject{
				Matcher: tt.matcher,
				Clause:  tt.clause,
			}
			assert.Equal(t, tt.want, i.Validate())
		})
	}
}
