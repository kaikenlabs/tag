package scaffold

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/kaikenlabs/tag/internal/formats"
)

// Prompter defines the interface for interactive prompts.
type Prompter interface {
	// Input prompts for a string value with an optional default.
	Input(label, defaultValue string, secret bool) (string, error)
	// Select prompts for a selection from a list of options.
	Select(label string, options []string, defaultIndex int) (string, error)
	// Confirm prompts for a yes/no confirmation.
	Confirm(label string, defaultValue bool) (bool, error)
	// Number prompts for a numeric value.
	Number(label string, defaultValue float64) (float64, error)
}

// InteractivePrompter implements Prompter using charmbracelet/huh.
type InteractivePrompter struct{}

// NewInteractivePrompter creates a new interactive prompter.
func NewInteractivePrompter() *InteractivePrompter {
	return &InteractivePrompter{}
}

// mapPromptErr translates huh errors to scaffold errors.
func mapPromptErr(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrPromptCancelled
	}
	return fmt.Errorf("prompt failed: %w", err)
}

// Input prompts for a string value.
func (p *InteractivePrompter) Input(label, defaultValue string, secret bool) (string, error) {
	result := defaultValue

	field := huh.NewInput().
		Title(label).
		Value(&result)

	if secret {
		field = field.EchoMode(huh.EchoModePassword)
	}

	if err := field.Run(); err != nil {
		return "", mapPromptErr(err)
	}

	return result, nil
}

// Select prompts for a selection from a list of options.
func (p *InteractivePrompter) Select(label string, options []string, defaultIndex int) (string, error) {
	if len(options) == 0 {
		return "", errors.New("no options provided for select prompt")
	}

	// Ensure defaultIndex is valid
	if defaultIndex < 0 || defaultIndex >= len(options) {
		defaultIndex = 0
	}

	result := options[defaultIndex]

	opts := make([]huh.Option[string], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, o)
	}

	if err := huh.NewSelect[string]().
		Title(label).
		Options(opts...).
		Value(&result).
		Run(); err != nil {
		return "", mapPromptErr(err)
	}

	return result, nil
}

// Confirm prompts for a yes/no confirmation.
func (p *InteractivePrompter) Confirm(label string, defaultValue bool) (bool, error) {
	result := defaultValue

	if err := huh.NewConfirm().
		Title(label).
		Value(&result).
		Run(); err != nil {
		return false, mapPromptErr(err)
	}

	return result, nil
}

// Number prompts for a numeric value.
func (p *InteractivePrompter) Number(label string, defaultValue float64) (float64, error) {
	result := formatNumber(defaultValue)

	if err := huh.NewInput().
		Title(label).
		Value(&result).
		Validate(func(s string) error {
			_, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return errors.New("invalid number")
			}
			return nil
		}).
		Run(); err != nil {
		return 0, mapPromptErr(err)
	}

	return strconv.ParseFloat(result, 64)
}

// formatNumber formats a float64 for display, avoiding unnecessary decimals.
func formatNumber(n float64) string {
	if formats.IsWholeNumber(n) {
		return strconv.FormatInt(int64(n), 10)
	}
	return fmt.Sprintf("%g", n)
}

// NoopPrompter is a prompter that returns defaults without prompting.
// Used when --no-input is specified or stdin is not a TTY.
type NoopPrompter struct{}

// NewNoopPrompter creates a new noop prompter.
func NewNoopPrompter() *NoopPrompter {
	return &NoopPrompter{}
}

// Input returns the default value without prompting.
func (p *NoopPrompter) Input(label, defaultValue string, secret bool) (string, error) {
	return defaultValue, nil
}

// Select returns the first option (or default) without prompting.
func (p *NoopPrompter) Select(label string, options []string, defaultIndex int) (string, error) {
	if len(options) == 0 {
		return "", errors.New("no options provided")
	}
	if defaultIndex >= 0 && defaultIndex < len(options) {
		return options[defaultIndex], nil
	}
	return options[0], nil
}

// Confirm returns the default value without prompting.
func (p *NoopPrompter) Confirm(label string, defaultValue bool) (bool, error) {
	return defaultValue, nil
}

// Number returns the default value without prompting.
func (p *NoopPrompter) Number(label string, defaultValue float64) (float64, error) {
	return defaultValue, nil
}

// IsTTY checks if stdin is connected to a terminal.
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) //nolint:gosec // G115: Stdin.Fd()→int is safe on all Go-supported platforms
}

// GetPrompter returns an appropriate prompter based on TTY status and noInput flag.
func GetPrompter(noInput bool) Prompter {
	if noInput || !IsTTY() {
		return NewNoopPrompter()
	}
	return NewInteractivePrompter()
}
