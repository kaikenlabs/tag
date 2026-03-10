package templateupdate

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/kaikenlabs/tag/internal/tmplconfig"
)

// VarChangeType classifies how a template variable changed between versions.
type VarChangeType int

const (
	// VarAdded means the variable exists in the new config but not the old.
	VarAdded VarChangeType = iota
	// VarRemoved means the variable existed in the old config but not the new.
	VarRemoved
	// VarDefaultChanged means the variable's default value changed.
	VarDefaultChanged
	// VarTypeChanged means the variable's type changed.
	VarTypeChanged
)

// String returns a human-readable label for a VarChangeType.
func (t VarChangeType) String() string {
	switch t {
	case VarAdded:
		return "added"
	case VarRemoved:
		return "removed"
	case VarDefaultChanged:
		return "default-changed"
	case VarTypeChanged:
		return "type-changed"
	default:
		return "unknown"
	}
}

// VarChange describes a single variable difference between template versions.
type VarChange struct {
	Name   string
	Type   VarChangeType
	OldDef *tmplconfig.VariableDef // nil for VarAdded
	NewDef *tmplconfig.VariableDef // nil for VarRemoved
}

// DetectVarChanges compares variable definitions between two template configs
// and returns a sorted list of changes. Private variables (starting with _)
// are included since they may affect rendering.
func DetectVarChanges(oldConfig, newConfig *tmplconfig.TemplateConfig) []VarChange {
	var changes []VarChange

	oldVars := safeVars(oldConfig)
	newVars := safeVars(newConfig)

	// Detect added and changed variables.
	for name, newDef := range newVars {
		oldDef, exists := oldVars[name]
		if !exists {
			nd := newDef
			changes = append(changes, VarChange{
				Name:   name,
				Type:   VarAdded,
				NewDef: &nd,
			})

			continue
		}

		changes = append(changes, detectFieldChanges(name, oldDef, newDef)...)
	}

	// Detect removed variables.
	for name, oldDef := range oldVars {
		if _, exists := newVars[name]; !exists {
			od := oldDef
			changes = append(changes, VarChange{
				Name:   name,
				Type:   VarRemoved,
				OldDef: &od,
			})
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Type != changes[j].Type {
			return changes[i].Type < changes[j].Type
		}

		return changes[i].Name < changes[j].Name
	})

	return changes
}

// detectFieldChanges compares individual fields of a variable definition.
func detectFieldChanges(name string, oldDef, newDef tmplconfig.VariableDef) []VarChange {
	var changes []VarChange

	if oldDef.Type != newDef.Type && newDef.Type != "" && oldDef.Type != "" {
		od, nd := oldDef, newDef
		changes = append(changes, VarChange{
			Name:   name,
			Type:   VarTypeChanged,
			OldDef: &od,
			NewDef: &nd,
		})
	}

	if !reflect.DeepEqual(oldDef.Default, newDef.Default) {
		od, nd := oldDef, newDef
		changes = append(changes, VarChange{
			Name:   name,
			Type:   VarDefaultChanged,
			OldDef: &od,
			NewDef: &nd,
		})
	}

	return changes
}

// safeVars returns the Vars map from a config, or an empty map if nil.
func safeVars(cfg *tmplconfig.TemplateConfig) map[string]tmplconfig.VariableDef {
	if cfg == nil || cfg.Vars == nil {
		return map[string]tmplconfig.VariableDef{}
	}

	return cfg.Vars
}

// NeedsPrompt returns true if the variable change requires user input.
func (vc VarChange) NeedsPrompt() bool {
	if vc.Type != VarAdded {
		return false
	}

	return vc.NewDef != nil && vc.NewDef.Required && vc.NewDef.Default == nil
}

// FormatVarChanges returns a human-readable summary of variable changes.
func FormatVarChanges(changes []VarChange, userVars map[string]any) []string {
	var lines []string

	for _, vc := range changes {
		switch vc.Type {
		case VarAdded:
			switch {
			case vc.NeedsPrompt():
				lines = append(lines, fmt.Sprintf("  + %s (new, required)", vc.Name))
			case vc.NewDef != nil && vc.NewDef.Default != nil:
				lines = append(lines, fmt.Sprintf("  + %s (new, optional) — using default: %v", vc.Name, vc.NewDef.Default))
			default:
				lines = append(lines, fmt.Sprintf("  + %s (new)", vc.Name))
			}
		case VarRemoved:
			lines = append(lines, fmt.Sprintf("  - %s (removed)", vc.Name))
		case VarDefaultChanged:
			oldDefault := "<none>"
			newDefault := "<none>"

			if vc.OldDef != nil && vc.OldDef.Default != nil {
				oldDefault = fmt.Sprintf("%v", vc.OldDef.Default)
			}

			if vc.NewDef != nil && vc.NewDef.Default != nil {
				newDefault = fmt.Sprintf("%v", vc.NewDef.Default)
			}

			userVal, hasUserVal := userVars[vc.Name]
			if hasUserVal {
				lines = append(lines, fmt.Sprintf("  ~ %s default changed: %q → %q (keeping your value: %v)",
					vc.Name, oldDefault, newDefault, userVal))
			} else {
				lines = append(lines, fmt.Sprintf("  ~ %s default changed: %q → %q", vc.Name, oldDefault, newDefault))
			}
		case VarTypeChanged:
			oldType := "untyped"
			newType := "untyped"

			if vc.OldDef != nil && vc.OldDef.Type != "" {
				oldType = string(vc.OldDef.Type)
			}

			if vc.NewDef != nil && vc.NewDef.Type != "" {
				newType = string(vc.NewDef.Type)
			}

			lines = append(lines, fmt.Sprintf("  ! %s type changed: %s → %s", vc.Name, oldType, newType))
		}
	}

	return lines
}

// ResolveNewVariables applies defaults for new optional variables and returns
// the names of new required variables that still need values.
func ResolveNewVariables(changes []VarChange, vars map[string]any, overrides map[string]string) []string {
	var needsInput []string

	for _, vc := range changes {
		if vc.Type != VarAdded || vc.NewDef == nil {
			continue
		}

		// Check if already provided via --set override.
		if val, ok := overrides[vc.Name]; ok {
			vars[vc.Name] = val
			continue
		}

		// Check if already in stored vars (e.g. from previous scaffold).
		if _, ok := vars[vc.Name]; ok {
			continue
		}

		// Apply default if available.
		if vc.NewDef.Default != nil {
			vars[vc.Name] = vc.NewDef.Default
			continue
		}

		// Required variable with no default — needs input.
		if vc.NewDef.Required {
			needsInput = append(needsInput, vc.Name)
		}
	}

	// Remove variables that were removed from the template.
	for _, vc := range changes {
		if vc.Type == VarRemoved {
			delete(vars, vc.Name)
		}
	}

	return needsInput
}
