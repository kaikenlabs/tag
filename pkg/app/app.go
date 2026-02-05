package app

import (
	"fmt"
	"log/slog"
	"os"
)

// Deprecated: Use Errorf and return errors instead. This function will be removed in a future version.
func Terminate(message string, attrs ...any) {
	slog.Error(fmt.Sprintf(message, attrs...))
	os.Exit(1)
}
