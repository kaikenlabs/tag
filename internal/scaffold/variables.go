package scaffold

import (
	"container/heap"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kaikenlabs/tag/internal/replay"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/tmplconfig"
)

// VariableCollector gathers variable values from all sources.
type VariableCollector interface {
	Collect(config *TemplateConfig, opts Options, isTTY bool) (map[string]any, error)
}

// DefaultVariableCollector implements VariableCollector with the standard priority chain.
type DefaultVariableCollector struct {
	prompter Prompter
	engine   template.TemplateRenderer // optional; resolves expression defaults before prompting
}

// NewVariableCollector creates a new variable collector with the given prompter.
func NewVariableCollector(prompter Prompter) *DefaultVariableCollector {
	return &DefaultVariableCollector{prompter: prompter}
}

// WithEngine sets the template engine used to resolve expression defaults at
// prompt time for evaluated-default variables (expanded form with prompt +
// template-expression default).
func (c *DefaultVariableCollector) WithEngine(engine template.TemplateRenderer) {
	c.engine = engine
}

// Collect gathers variables following the priority chain:
// defaults -> --replay -> --values file -> --meta flags -> prompts
//
// Meta overrides are applied before prompts so that evaluated defaults
// (expressions like "{{ vars.project_name | kebab }}") see CLI-provided
// values instead of static defaults. Variables are prompted in dependency
// order (topological sort) so that evaluated defaults can reference
// previously-prompted values.
//
//nolint:gocognit,cyclop // orchestration function coordinates multiple input sources
func (c *DefaultVariableCollector) Collect(config *TemplateConfig, opts Options, isTTY bool) (map[string]any, error) {
	vars := make(map[string]any)
	// Track which variables were explicitly provided (from replay, values file, or meta)
	// These should not be re-prompted even in interactive mode
	explicitlyProvided := make(map[string]bool)

	// Get dependency-aware variable ordering (topological sort with
	// lexicographic tie-breaking for independent variables).
	varNames, err := topologicalSortVars(config.Vars)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve variable ordering: %w", err)
	}

	// Step 1: Apply defaults (but don't mark them as explicitly provided)
	for _, name := range varNames {
		def := config.Vars[name]
		if def.Default != nil {
			vars[name] = def.Default
		}
	}

	// Step 2: Load replay values (if --replay flag is set)
	//nolint:nestif // replay variable resolution requires nested conditions
	if opts.Replay {
		if opts.TemplateRef == "" {
			return nil, errors.New("--replay requires template reference to be set")
		}
		replayData, err := replay.Load(opts.TemplateRef)
		if err != nil {
			if errors.Is(err, replay.ErrReplayNotFound) {
				return nil, errors.New("no saved replay data found for this template (use scaffold without --replay first)")
			}
			if errors.Is(err, replay.ErrReplayCorrupt) {
				return nil, fmt.Errorf("replay file is corrupt: %w (delete it and try again)", err)
			}
			return nil, fmt.Errorf("failed to load replay data: %w", err)
		}
		// Apply replay values
		for k, v := range replayData.Values {
			// Coerce replay values to expected types if we know them
			if def, ok := config.Vars[k]; ok {
				coerced, err := coerceAnyValue(v, def.Type)
				if err != nil {
					// If coercion fails, use the raw value (template may have changed)
					vars[k] = v
				} else {
					vars[k] = coerced
				}
			} else {
				// Unknown variable (template may have changed), store as-is
				vars[k] = v
			}
			explicitlyProvided[k] = true
		}
	}

	// Step 3: Load values from --values file (if provided)
	if opts.ValuesFile != "" {
		fileVars, err := loadValuesFile(opts.ValuesFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load values file: %w", err)
		}
		for k, v := range fileVars {
			vars[k] = v
			explicitlyProvided[k] = true
		}
	}

	// Step 4: Apply --meta overrides early (highest priority, before prompts)
	// This ensures evaluated defaults see CLI-provided values (e.g., positional
	// project_name) instead of static defaults when resolving expressions.
	for k, v := range opts.Meta {
		// Try to coerce value to the expected type if we know it
		if def, ok := config.Vars[k]; ok {
			coerced, err := coerceValue(v, def.Type)
			if err != nil {
				return nil, NewVariableError(k, "invalid value type", err)
			}
			vars[k] = coerced
		} else {
			// Unknown variable, store as string
			vars[k] = v
		}
		explicitlyProvided[k] = true
	}

	// Step 5: Interactive prompts for variables (if TTY)
	// Prompt for all non-private, non-derived variables, even if they have defaults.
	// Skip only if the value was explicitly provided via replay, values file, or meta.
	// Variables are iterated in dependency order (topological sort) so that
	// evaluated defaults can reference previously-prompted or meta-provided values.
	if isTTY && !opts.NoInput {
		if err := c.promptAll(varNames, config, explicitlyProvided, vars); err != nil {
			return nil, err
		}
	}

	// Step 6: Validate all required variables have values
	if err := c.validateRequired(config, vars); err != nil {
		return nil, err
	}

	// Step 7: Derived variables are resolved after Collect() returns,
	// via ResolveDerivedVars() which renders their template expression defaults
	// through the template engine.

	return vars, nil
}

// promptAll iterates over variable names and prompts for each eligible variable,
// resolving evaluated-default expressions beforehand.
func (c *DefaultVariableCollector) promptAll(
	varNames []string,
	config *TemplateConfig,
	explicitlyProvided map[string]bool,
	vars map[string]any,
) error {
	for _, name := range varNames {
		def := config.Vars[name]

		// Skip private/computed variables (start with _)
		if def.IsPrivate(name) {
			continue
		}

		// Skip derived variables (default contains template expression)
		// These are computed from other variables, following Cookiecutter behavior.
		if def.IsDerived() {
			continue
		}

		// Skip if explicitly provided via replay, values file, or --meta flag
		if explicitlyProvided[name] {
			continue
		}

		// For evaluated-default variables (expanded form + prompt + expression default),
		// resolve the expression with currently collected vars so the prompt shows
		// a concrete suggested value instead of the raw template expression.
		defToPrompt := c.resolveEvaluatedDefault(def, vars)

		// Prompt for the variable (default will be pre-filled)
		value, err := c.promptForVariable(name, defToPrompt)
		if err != nil {
			return err
		}
		vars[name] = value
	}
	return nil
}

// resolveEvaluatedDefault returns a copy of def with its expression default
// resolved using the current vars. If the variable is not an evaluated default,
// the engine is unavailable, or resolution fails, the original def is returned.
func (c *DefaultVariableCollector) resolveEvaluatedDefault(def VariableDef, vars map[string]any) VariableDef {
	if !def.IsEvaluatedDefault() || c.engine == nil {
		return def
	}
	defaultStr, ok := def.Default.(string)
	if !ok {
		return def
	}
	ctx := template.NewContextBuilder().WithVars(vars).Build()
	resolved, err := c.engine.ExecuteToString(defaultStr, ctx)
	if err != nil {
		// Resolution failed (dependency not yet collected); fall back to raw expression.
		return def
	}
	def.Default = resolved
	return def
}

// ResolveDerivedVars evaluates derived and evaluated-default variable expressions
// through the template engine.
//
// Derived variables (minimal form, no explicit prompt) always resolve from their
// expression default. Evaluated-default variables (expanded form with explicit
// prompt) resolve only if their current value is still a raw template expression
// — meaning they were not collected via an interactive prompt, values file, or
// --meta flag (e.g. non-TTY mode or --no-input).
//
// Variables are processed in dependency order (topological sort) so that
// expressions referencing other derived variables see already-resolved values.
func ResolveDerivedVars(engine template.TemplateRenderer, config *TemplateConfig, vars map[string]any) error {
	varNames, err := topologicalSortVars(config.Vars)
	if err != nil {
		return fmt.Errorf("failed to resolve variable ordering: %w", err)
	}
	for _, name := range varNames {
		def := config.Vars[name]

		var exprToResolve string
		switch {
		case def.IsDerived():
			// Classic derived: always resolve from the expression default.
			defaultStr, ok := def.Default.(string)
			if !ok {
				continue
			}
			exprToResolve = defaultStr

		case def.IsEvaluatedDefault():
			// Evaluated default: only resolve if the value is still a raw template
			// expression (i.e., was not overridden by a prompt or explicit value).
			currentStr, ok := vars[name].(string)
			if !ok || !tmplconfig.ContainsTemplateExpression(currentStr) {
				continue
			}
			exprToResolve = currentStr

		default:
			continue
		}

		// Build context with current vars (including previously resolved vars)
		ctx := template.NewContextBuilder().WithVars(vars).Build()

		rendered, err := engine.ExecuteToString(exprToResolve, ctx)
		if err != nil {
			return fmt.Errorf("failed to evaluate derived variable %q: %w", name, err)
		}

		vars[name] = rendered
	}

	return nil
}

// promptForVariable prompts the user for a variable value based on its type.
func (c *DefaultVariableCollector) promptForVariable(name string, def VariableDef) (any, error) {
	prompt := def.GetPrompt(name)

	switch def.Type {
	case VarTypeBoolean:
		defaultBool := false
		if def.Default != nil {
			if b, ok := def.Default.(bool); ok {
				defaultBool = b
			}
		}
		return c.prompter.Confirm(prompt, defaultBool)

	case VarTypeNumber:
		defaultNum := 0.0
		if def.Default != nil {
			switch v := def.Default.(type) {
			case float64:
				defaultNum = v
			case int:
				defaultNum = float64(v)
			}
		}
		return c.prompter.Number(prompt, defaultNum)

	case VarTypeChoice:
		defaultIndex := 0
		if def.Default != nil {
			if defaultStr, ok := def.Default.(string); ok {
				for i, opt := range def.Options {
					if opt == defaultStr {
						defaultIndex = i
						break
					}
				}
			}
		}
		return c.prompter.Select(prompt, def.Options, defaultIndex)

	default: // VarTypeString or unspecified
		defaultStr := ""
		if def.Default != nil {
			if s, ok := def.Default.(string); ok {
				defaultStr = s
			}
		}
		return c.prompter.Input(prompt, defaultStr, def.Secret)
	}
}

// validateRequired checks that all required variables have values.
func (c *DefaultVariableCollector) validateRequired(config *TemplateConfig, vars map[string]any) error {
	var missing []string

	for name, def := range config.Vars {
		if !def.Required {
			continue
		}

		// Skip private variables - they're computed
		if def.IsPrivate(name) {
			continue
		}

		val, exists := vars[name]
		if !exists || isEmptyValue(val) {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		slices.Sort(missing)

		var metaFlags strings.Builder
		for _, name := range missing {
			fmt.Fprintf(&metaFlags, "  --meta %s=<value>\n", name)
		}

		return fmt.Errorf("%w: %s\n\nProvide values with:\n%s  --values <file.json>",
			ErrRequiredVariableMissing,
			strings.Join(missing, ", "),
			metaFlags.String(),
		)
	}

	return nil
}

// loadValuesFile loads variable values from a JSON file.
func loadValuesFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read values file: %w", err)
	}

	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("failed to parse values file: %w", err)
	}

	return values, nil
}

// coerceValue attempts to convert a string value to the expected type.
func coerceValue(value string, varType VariableType) (any, error) {
	switch varType {
	case VarTypeBoolean:
		return parseBool(value)

	case VarTypeNumber:
		return strconv.ParseFloat(value, 64)

	case VarTypeChoice, VarTypeString:
		return value, nil

	default:
		return value, nil
	}
}

// coerceAnyValue attempts to convert a value of any type to the expected type.
// This is used for replay values which are already typed from JSON.
func coerceAnyValue(value any, varType VariableType) (any, error) {
	if value == nil {
		return nil, nil //nolint:nilnil // nil value is not an error
	}

	// For string values, delegate to coerceValue to avoid duplication.
	if s, ok := value.(string); ok {
		return coerceValue(s, varType)
	}

	// Handle non-string native types.
	switch varType {
	case VarTypeBoolean:
		if v, ok := value.(bool); ok {
			return v, nil
		}
		return nil, fmt.Errorf("cannot convert %T to boolean", value)

	case VarTypeNumber:
		switch v := value.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		default:
			return nil, fmt.Errorf("cannot convert %T to number", value)
		}

	case VarTypeChoice, VarTypeString:
		return fmt.Sprintf("%v", value), nil

	default:
		return value, nil
	}
}

// parseBool parses a boolean from various string representations.
func parseBool(s string) (bool, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "yes", "y", "1", "on":
		return true, nil
	case "false", "no", "n", "0", "off", "":
		return false, nil
	default:
		return false, fmt.Errorf("cannot parse %q as boolean", s)
	}
}

// isEmptyValue checks if a value is considered "empty".
func isEmptyValue(val any) bool {
	if val == nil {
		return true
	}
	switch v := val.(type) {
	case string:
		return v == ""
	case bool:
		return false // booleans are never "empty"
	case float64:
		return false // numbers are never "empty"
	case int:
		return false
	default:
		return false
	}
}

// varRefPattern matches {{ vars.<name> }} references, capturing the variable name.
// It handles optional filters (|), method calls (.), and whitespace.
var varRefPattern = regexp.MustCompile(`\{\{\s*vars\.([a-zA-Z_][a-zA-Z0-9_]*)`)

// extractVarRefs extracts variable names referenced via {{ vars.<name> }} in a
// template expression. Returns a deduplicated, sorted slice.
func extractVarRefs(expr string) []string {
	matches := varRefPattern.FindAllStringSubmatch(expr, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(matches))
	var refs []string
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			refs = append(refs, name)
		}
	}
	slices.Sort(refs)
	return refs
}

// ErrCircularDependency is returned when variable defaults form a circular dependency.
var ErrCircularDependency = errors.New("circular variable dependency")

// topologicalSortVars returns variable names ordered so that dependencies come
// before dependents. Dependencies are extracted from default template expressions
// ({{ vars.* }}). Independent variables are sorted lexicographically for
// deterministic ordering. Returns an error if a circular dependency is detected.
func topologicalSortVars(vars map[string]VariableDef) ([]string, error) {
	// Build adjacency list and in-degree map.
	// Edge: dependency → dependent (dep must come before dependent).
	inDegree := make(map[string]int, len(vars))
	dependents := make(map[string][]string, len(vars)) // dep → list of vars that depend on it

	for name := range vars {
		inDegree[name] = 0
	}

	for name, def := range vars {
		defaultStr, ok := def.Default.(string)
		if !ok {
			continue
		}
		refs := extractVarRefs(defaultStr)
		for _, ref := range refs {
			// Only consider references to known variables.
			if _, exists := vars[ref]; !exists {
				continue
			}
			// Self-reference is a cycle.
			if ref == name {
				return nil, fmt.Errorf("%w: variable %q references itself", ErrCircularDependency, name)
			}
			dependents[ref] = append(dependents[ref], name)
			inDegree[name]++
		}
	}

	// Kahn's algorithm with a min-heap for lexicographic tie-breaking.
	h := &stringHeap{}
	heap.Init(h)
	for name, deg := range inDegree {
		if deg == 0 {
			heap.Push(h, name)
		}
	}

	sorted := make([]string, 0, len(vars))
	for h.Len() > 0 {
		name, _ := heap.Pop(h).(string)
		sorted = append(sorted, name)
		for _, dep := range dependents[name] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				heap.Push(h, dep)
			}
		}
	}

	if len(sorted) != len(vars) {
		// Find the cycle for a useful error message.
		var cycleVars []string
		for name, deg := range inDegree {
			if deg > 0 {
				cycleVars = append(cycleVars, name)
			}
		}
		slices.Sort(cycleVars)
		return nil, fmt.Errorf("%w: variables involved: %s",
			ErrCircularDependency, strings.Join(cycleVars, ", "))
	}

	return sorted, nil
}

// stringHeap implements heap.Interface for a min-heap of strings.
type stringHeap []string

func (h *stringHeap) Len() int           { return len(*h) }
func (h *stringHeap) Less(i, j int) bool { return (*h)[i] < (*h)[j] }
func (h *stringHeap) Swap(i, j int)      { (*h)[i], (*h)[j] = (*h)[j], (*h)[i] }

func (h *stringHeap) Push(x any) {
	s, _ := x.(string)
	*h = append(*h, s)
}

func (h *stringHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
