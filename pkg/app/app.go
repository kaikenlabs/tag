package app

import (
	"fmt"
	"log/slog"
	"os"
)

func Terminate(message string, attrs ...any) {
	slog.Error(fmt.Sprintf(message, attrs...))
	os.Exit(1)
}
