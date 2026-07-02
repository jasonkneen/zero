package tools

import (
	"testing"

	"github.com/Gitlawb/zero/internal/modelregistry"
)

// TestOnlyBashModeToolNamesMatchRegisteredTools locks the modelregistry
// "onlybash" preset's EnabledTools/DisabledTools string literals to the real
// registered tool names. modes.go can't import this package to reference
// NewBashTool(...).Name() / NewSkillTool(...).Name() / ToolSearchToolName
// directly (internal/tools already imports internal/modelregistry via
// escalate_model.go, so the reverse import would cycle) — this test is the
// other half of that contract: it fails loudly if a tool is ever renamed
// without updating the mode preset.
func TestOnlyBashModeToolNamesMatchRegisteredTools(t *testing.T) {
	mode, ok := modelregistry.LookupMode("onlybash")
	if !ok {
		t.Fatal("modelregistry.LookupMode(\"onlybash\") = _, false; want a registered mode")
	}

	bashName := NewBashTool(t.TempDir()).Name()
	skillName := NewSkillTool(t.TempDir()).Name()

	wantEnabled := []string{bashName, skillName}
	if len(mode.EnabledTools) != len(wantEnabled) {
		t.Fatalf("onlybash EnabledTools = %v; want %v", mode.EnabledTools, wantEnabled)
	}
	for index, name := range wantEnabled {
		if mode.EnabledTools[index] != name {
			t.Fatalf("onlybash EnabledTools = %v; want %v", mode.EnabledTools, wantEnabled)
		}
	}

	wantDisabled := []string{ToolSearchToolName}
	if len(mode.DisabledTools) != len(wantDisabled) {
		t.Fatalf("onlybash DisabledTools = %v; want %v", mode.DisabledTools, wantDisabled)
	}
	for index, name := range wantDisabled {
		if mode.DisabledTools[index] != name {
			t.Fatalf("onlybash DisabledTools = %v; want %v", mode.DisabledTools, wantDisabled)
		}
	}
}
