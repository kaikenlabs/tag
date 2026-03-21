package template

import (
	"fmt"

	"github.com/nikolalohinski/gonja/v2/exec"

	"github.com/kaikenlabs/tag/internal/dialect"
)

// RegisterDialectFilter registers the "to" filter on the given filter set.
// The filter resolves canonical type names to dialect-specific types using the
// provided registry. Example: {{ "uuid" | to("postgres") }} → "UUID".
func RegisterDialectFilter(filters *exec.FilterSet, reg *dialect.Registry) error {
	toFilter := func(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
		if in.IsError() {
			return in
		}

		args := params.Args
		if len(args) != 1 {
			return exec.AsValue(fmt.Errorf("to: expected 1 argument (dialect name), got %d", len(args)))
		}

		dialectName := args[0].String()
		canonicalType := in.String()

		result, err := reg.Resolve(canonicalType, dialectName)
		if err != nil {
			return exec.AsValue(fmt.Errorf("to: %w", err))
		}

		return exec.AsValue(result)
	}

	return registerFilter(filters, "to", toFilter)
}
