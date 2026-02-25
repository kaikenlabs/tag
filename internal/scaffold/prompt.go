package scaffold

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"
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

// InteractivePrompter implements Prompter using promptui.
type InteractivePrompter struct{}

// NewInteractivePrompter creates a new interactive prompter.
func NewInteractivePrompter() *InteractivePrompter {
	return &InteractivePrompter{}
}

// Input prompts for a string value.
func (p *InteractivePrompter) Input(label, defaultValue string, secret bool) (string, error) {
	prompt := promptui.Prompt{
		Label:   label,
		Default: defaultValue,
	}

	if secret {
		prompt.Mask = '*'
	}

	result, err := prompt.Run()
	if err != nil {
		if errors.Is(err, promptui.ErrInterrupt) {
			return "", ErrPromptCancelled
		}
		return "", fmt.Errorf("prompt failed: %w", err)
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

	prompt := promptui.Select{
		Label:     label,
		Items:     options,
		CursorPos: defaultIndex,
	}

	_, result, err := prompt.Run()
	if err != nil {
		if errors.Is(err, promptui.ErrInterrupt) {
			return "", ErrPromptCancelled
		}
		return "", fmt.Errorf("select prompt failed: %w", err)
	}

	return result, nil
}

// Confirm prompts for a yes/no confirmation.
func (p *InteractivePrompter) Confirm(label string, defaultValue bool) (bool, error) {
	// Format prompt to show default clearly
	var promptLabel string
	if defaultValue {
		promptLabel = label + " [Y/n]"
	} else {
		promptLabel = label + " [y/N]"
	}

	prompt := promptui.Prompt{
		Label: promptLabel,
	}

	result, err := prompt.Run()
	if err != nil {
		if errors.Is(err, promptui.ErrInterrupt) {
			return false, ErrPromptCancelled
		}
		return false, fmt.Errorf("confirm prompt failed: %w", err)
	}

	// Empty input = use default
	if result == "" {
		return defaultValue, nil
	}

	// Parse explicit response
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "y", "yes", "true", "1":
		return true, nil
	case "n", "no", "false", "0":
		return false, nil
	default:
		// Invalid input, return default
		return defaultValue, nil
	}
}

// Number prompts for a numeric value.
func (p *InteractivePrompter) Number(label string, defaultValue float64) (float64, error) {
	prompt := promptui.Prompt{
		Label:   label,
		Default: formatNumber(defaultValue),
		Validate: func(input string) error {
			_, err := strconv.ParseFloat(input, 64)
			if err != nil {
				return errors.New("invalid number")
			}
			return nil
		},
	}

	result, err := prompt.Run()
	if err != nil {
		if errors.Is(err, promptui.ErrInterrupt) {
			return 0, ErrPromptCancelled
		}
		return 0, fmt.Errorf("number prompt failed: %w", err)
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
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// GetPrompter returns an appropriate prompter based on TTY status and noInput flag.
func GetPrompter(noInput bool) Prompter {
	if noInput || !IsTTY() {
		return NewNoopPrompter()
	}
	return NewInteractivePrompter()
}
