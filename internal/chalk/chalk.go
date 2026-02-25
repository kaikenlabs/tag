package chalk

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
)

type colourCode string

var (
	reset  colourCode = "\033[0m"
	red    colourCode = "\033[31m"
	green  colourCode = "\033[32m"
	yellow colourCode = "\033[33m"
	blue   colourCode = "\033[34m"
	purple colourCode = "\033[35m"
	cyan   colourCode = "\033[36m"
	gray   colourCode = "\033[37m"
	white  colourCode = "\033[97m"
)

// Red - colour red
func Red(msg string) string {
	return colourTerminalOutput(msg, red)
}

// Green - colour green
func Green(msg string) string {
	return colourTerminalOutput(msg, green)
}

// Yellow - colour yellow
func Yellow(msg string) string {
	return colourTerminalOutput(msg, yellow)
}

// Blue - colour blue
func Blue(msg string) string {
	return colourTerminalOutput(msg, blue)
}

// Cyan - colour cyan
func Cyan(msg string) string {
	return colourTerminalOutput(msg, cyan)
}

func colourTerminalOutput(msg string, colourCode colourCode) string {
	if isTerminal() {
		return fmt.Sprintf("%s%s%s", colourCode, msg, reset)
	}
	return msg
}

func isTerminal() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}
