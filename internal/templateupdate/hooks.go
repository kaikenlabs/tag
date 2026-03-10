package templateupdate

import (
	"fmt"
	"sort"

	"github.com/kaikenlabs/tag/internal/tmplconfig"
	"github.com/kaikenlabs/tag/internal/types"
)

// HookChangeType classifies how a hook changed between template versions.
type HookChangeType int

const (
	// HookAdded means the hook exists in the new config but not the old.
	HookAdded HookChangeType = iota
	// HookRemoved means the hook existed in the old config but not the new.
	HookRemoved
	// HookModified means the hook commands changed between versions.
	HookModified
)

// String returns a human-readable label for a HookChangeType.
func (t HookChangeType) String() string {
	switch t {
	case HookAdded:
		return "added"
	case HookRemoved:
		return "removed"
	case HookModified:
		return "modified"
	default:
		return "unknown"
	}
}

// HookChange describes a single hook difference between template versions.
type HookChange struct {
	Phase    string // "pre_scaffold" or "post_scaffold"
	Type     HookChangeType
	OldHooks []string // nil for HookAdded
	NewHooks []string // nil for HookRemoved
}

// DetectHookChanges compares hook definitions between two template configs
// and returns a sorted list of changes.
func DetectHookChanges(oldConfig, newConfig *tmplconfig.TemplateConfig) []HookChange {
	oldHooks := safeHooks(oldConfig)
	newHooks := safeHooks(newConfig)

	var changes []HookChange

	phases := []struct {
		name    string
		oldCmds []string
		newCmds []string
	}{
		{"pre_scaffold", oldHooks.PreScaffold, newHooks.PreScaffold},
		{"post_scaffold", oldHooks.PostScaffold, newHooks.PostScaffold},
	}

	for _, p := range phases {
		if change := detectPhaseChange(p.name, p.oldCmds, p.newCmds); change != nil {
			changes = append(changes, *change)
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Phase < changes[j].Phase
	})

	return changes
}

// detectPhaseChange compares hooks for a single phase and returns a change if any.
func detectPhaseChange(phase string, oldCmds, newCmds []string) *HookChange {
	oldEmpty := len(oldCmds) == 0
	newEmpty := len(newCmds) == 0

	if oldEmpty && newEmpty {
		return nil
	}

	if oldEmpty && !newEmpty {
		return &HookChange{Phase: phase, Type: HookAdded, NewHooks: newCmds}
	}

	if !oldEmpty && newEmpty {
		return &HookChange{Phase: phase, Type: HookRemoved, OldHooks: oldCmds}
	}

	if !hooksEqual(oldCmds, newCmds) {
		return &HookChange{Phase: phase, Type: HookModified, OldHooks: oldCmds, NewHooks: newCmds}
	}

	return nil
}

// hooksEqual returns true if two hook command slices are identical.
func hooksEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// safeHooks returns the HooksConfig from a template config, or an empty one if nil.
func safeHooks(cfg *tmplconfig.TemplateConfig) *types.HooksConfig {
	if cfg == nil || cfg.Hooks == nil {
		return &types.HooksConfig{}
	}

	return cfg.Hooks
}

// FormatHookChanges returns a human-readable summary of hook changes.
func FormatHookChanges(changes []HookChange) []string {
	var lines []string

	for _, hc := range changes {
		switch hc.Type {
		case HookAdded:
			lines = append(lines, fmt.Sprintf("  NEW %s hook:", hc.Phase))
			for _, cmd := range hc.NewHooks {
				lines = append(lines, "    + "+cmd)
			}
		case HookRemoved:
			lines = append(lines, "  REMOVED "+hc.Phase+" hook")
		case HookModified:
			lines = append(lines, fmt.Sprintf("  MODIFIED %s hook:", hc.Phase))
			for _, cmd := range hc.OldHooks {
				lines = append(lines, "    - "+cmd)
			}
			for _, cmd := range hc.NewHooks {
				lines = append(lines, "    + "+cmd)
			}
		}
	}

	return lines
}

// HasExecutableChanges returns true if any hook changes would require execution.
func HasExecutableChanges(changes []HookChange) bool {
	for _, hc := range changes {
		if hc.Type == HookAdded || hc.Type == HookModified {
			return true
		}
	}

	return false
}

// CollectNewHooks returns the new hook commands from added/modified hooks,
// organised into a HooksConfig for execution.
func CollectNewHooks(changes []HookChange) *types.HooksConfig {
	result := &types.HooksConfig{}

	for _, hc := range changes {
		if hc.Type != HookAdded && hc.Type != HookModified {
			continue
		}

		switch hc.Phase {
		case "pre_scaffold":
			result.PreScaffold = append(result.PreScaffold, hc.NewHooks...)
		case "post_scaffold":
			result.PostScaffold = append(result.PostScaffold, hc.NewHooks...)
		}
	}

	return result
}
