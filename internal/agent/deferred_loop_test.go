package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/Gitlawb/zero/internal/tools"
)

func TestOptionsDeferThresholdFieldExists(t *testing.T) {
	options := Options{DeferThreshold: 10}
	if options.DeferThreshold != 10 {
		t.Fatalf("expected DeferThreshold 10, got %d", options.DeferThreshold)
	}
}

func TestToolResultLoadedToolsField(t *testing.T) {
	result := ToolResult{LoadedTools: []string{"Alpha", "Beta"}}
	if len(result.LoadedTools) != 2 || result.LoadedTools[0] != "Alpha" || result.LoadedTools[1] != "Beta" {
		t.Fatalf("expected LoadedTools [Alpha Beta], got %#v", result.LoadedTools)
	}
	// Default zero value is nil for an ordinary result.
	if (ToolResult{}).LoadedTools != nil {
		t.Fatalf("expected nil LoadedTools by default")
	}
}

// loadSignalTool returns Meta["load_tools"] like tool_search does, so we can
// assert executeToolCall lifts it into ToolResult.LoadedTools.
type loadSignalTool struct{ value string }

func (t loadSignalTool) Name() string       { return "load_signal" }
func (t loadSignalTool) Description() string { return "emits a load_tools signal" }
func (t loadSignalTool) Parameters() tools.Schema {
	return tools.Schema{Type: "object", AdditionalProperties: false}
}
func (t loadSignalTool) Safety() tools.Safety {
	return tools.Safety{SideEffect: tools.SideEffectRead, Permission: tools.PermissionAllow}
}
func (t loadSignalTool) Run(_ context.Context, _ map[string]any) tools.Result {
	return tools.Result{Status: tools.StatusOK, Output: "ok"}
}
func (t loadSignalTool) RunWithOptions(_ context.Context, _ map[string]any, _ tools.RunOptions) tools.Result {
	return tools.Result{Status: tools.StatusOK, Output: "ok", Meta: map[string]string{"load_tools": t.value}}
}

func TestExecuteToolCallLiftsLoadTools(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(loadSignalTool{value: " Alpha , Beta ,, "})

	result, abortErr := executeToolCall(
		context.Background(),
		registry,
		ToolCall{ID: "c1", Name: "load_signal", Arguments: ""},
		PermissionModeAuto,
		Options{},
	)
	if abortErr != nil {
		t.Fatalf("unexpected abort error: %v", abortErr)
	}
	want := []string{"Alpha", "Beta"}
	if len(result.LoadedTools) != len(want) {
		t.Fatalf("expected LoadedTools %#v, got %#v", want, result.LoadedTools)
	}
	for i := range want {
		if result.LoadedTools[i] != want[i] {
			t.Fatalf("expected LoadedTools %#v, got %#v", want, result.LoadedTools)
		}
	}
}

func TestExecuteToolCallNoLoadToolsMetaLeavesNil(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(secretEmittingTool{output: "plain"})

	result, abortErr := executeToolCall(
		context.Background(),
		registry,
		ToolCall{ID: "c1", Name: "leak", Arguments: ""},
		PermissionModeAuto,
		Options{},
	)
	if abortErr != nil {
		t.Fatalf("unexpected abort error: %v", abortErr)
	}
	if result.LoadedTools != nil {
		t.Fatalf("expected nil LoadedTools for a tool with no load_tools meta, got %#v", result.LoadedTools)
	}
}

// fakeDeferredTool is deferred-eligible (implements Deferred() bool) like an MCP
// tool wrapper, so partitionTools counts and (when active) hides it.
type fakeDeferredTool struct {
	name string
	desc string
}

func (t fakeDeferredTool) Name() string       { return t.name }
func (t fakeDeferredTool) Description() string { return t.desc }
func (t fakeDeferredTool) Parameters() tools.Schema {
	return tools.Schema{Type: "object", AdditionalProperties: false}
}
func (t fakeDeferredTool) Safety() tools.Safety {
	return tools.Safety{SideEffect: tools.SideEffectRead, Permission: tools.PermissionAllow}
}
func (t fakeDeferredTool) Run(_ context.Context, _ map[string]any) tools.Result {
	return tools.Result{Status: tools.StatusOK, Output: "ok"}
}
func (t fakeDeferredTool) Deferred() bool { return true }

// fakeToolSearchTool stands in for component D's tool_search (a non-deferred
// builtin) so the inactive path can assert it is dropped.
type fakeToolSearchTool struct{}

func (fakeToolSearchTool) Name() string       { return "tool_search" }
func (fakeToolSearchTool) Description() string { return "load deferred tool schemas" }
func (fakeToolSearchTool) Parameters() tools.Schema {
	return tools.Schema{Type: "object", AdditionalProperties: false}
}
func (fakeToolSearchTool) Safety() tools.Safety {
	return tools.Safety{SideEffect: tools.SideEffectNone, Permission: tools.PermissionAllow, AdvertiseInAuto: true}
}
func (fakeToolSearchTool) Run(_ context.Context, _ map[string]any) tools.Result {
	return tools.Result{Status: tools.StatusOK, Output: "ok"}
}

func TestPartitionToolsInactiveIsByteIdenticalAndDropsToolSearch(t *testing.T) {
	root := t.TempDir()
	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool(root))
	registry.Register(fakeDeferredTool{name: "mcp__srv__a", desc: "tool a"})
	registry.Register(fakeToolSearchTool{})

	// DeferThreshold 0 => deferral disabled => inactive path.
	exposed, reminder := partitionTools(registry, PermissionModeAuto, Options{DeferThreshold: 0}, map[string]bool{})

	if reminder != "" {
		t.Fatalf("expected empty reminder on inactive path, got %q", reminder)
	}
	// Exposed must equal the legacy full-schema definitions minus tool_search.
	for _, def := range exposed {
		if def.Name == "tool_search" {
			t.Fatalf("tool_search must be dropped on inactive path, got %#v", exposed)
		}
	}
	wantNames := map[string]bool{"read_file": true, "mcp__srv__a": true}
	if len(exposed) != len(wantNames) {
		t.Fatalf("expected %d exposed tools, got %d: %#v", len(wantNames), len(exposed), exposed)
	}
	for _, def := range exposed {
		if !wantNames[def.Name] {
			t.Fatalf("unexpected exposed tool %q", def.Name)
		}
	}
	// The deferred tool keeps its FULL schema on the inactive path (not a hint).
	for _, def := range exposed {
		if def.Name == "mcp__srv__a" {
			if def.Parameters["type"] != "object" {
				t.Fatalf("expected full schema for deferred tool on inactive path, got %#v", def.Parameters)
			}
		}
	}
}

// Below-threshold-but-eligible (count < threshold) is also inactive.
func TestPartitionToolsBelowThresholdInactive(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(fakeDeferredTool{name: "mcp__srv__a", desc: "a"})
	registry.Register(fakeDeferredTool{name: "mcp__srv__b", desc: "b"})

	exposed, reminder := partitionTools(registry, PermissionModeAuto, Options{DeferThreshold: 10}, map[string]bool{})
	if reminder != "" {
		t.Fatalf("expected empty reminder below threshold, got %q", reminder)
	}
	if len(exposed) != 2 {
		t.Fatalf("expected both deferred tools exposed below threshold, got %#v", exposed)
	}
}

func TestPartitionToolsActiveHidesUnloadedExposesLoaded(t *testing.T) {
	root := t.TempDir()
	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool(root)) // non-deferred builtin
	registry.Register(fakeToolSearchTool{})        // non-deferred, must stay exposed
	registry.Register(fakeDeferredTool{name: "mcp__srv__alpha", desc: "alpha tool"})
	registry.Register(fakeDeferredTool{name: "mcp__srv__beta", desc: "beta tool"})

	loaded := map[string]bool{"mcp__srv__alpha": true}

	// 2 eligible deferred tools, threshold 2 => active.
	exposed, reminder := partitionTools(registry, PermissionModeAuto, Options{DeferThreshold: 2}, loaded)

	exposedNames := map[string]bool{}
	for _, def := range exposed {
		exposedNames[def.Name] = true
	}
	if !exposedNames["read_file"] {
		t.Fatalf("expected builtin read_file exposed, got %#v", exposed)
	}
	if !exposedNames["tool_search"] {
		t.Fatalf("expected tool_search exposed on active path, got %#v", exposed)
	}
	if !exposedNames["mcp__srv__alpha"] {
		t.Fatalf("expected loaded deferred tool exposed, got %#v", exposed)
	}
	if exposedNames["mcp__srv__beta"] {
		t.Fatalf("unloaded deferred tool must be hidden from exposed, got %#v", exposed)
	}
	if reminder == "" {
		t.Fatalf("expected a non-empty reminder for the hidden tool")
	}
	if !strings.Contains(reminder, "mcp__srv__beta") {
		t.Fatalf("expected reminder to list the hidden tool, got %q", reminder)
	}
	if strings.Contains(reminder, "mcp__srv__alpha") {
		t.Fatalf("loaded tool must not appear in the reminder, got %q", reminder)
	}
}

func TestPartitionToolsActiveNothingHiddenEmptyReminder(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(fakeDeferredTool{name: "mcp__srv__alpha", desc: "alpha"})
	registry.Register(fakeDeferredTool{name: "mcp__srv__beta", desc: "beta"})

	loaded := map[string]bool{"mcp__srv__alpha": true, "mcp__srv__beta": true}
	exposed, reminder := partitionTools(registry, PermissionModeAuto, Options{DeferThreshold: 2}, loaded)

	if len(exposed) != 2 {
		t.Fatalf("expected both loaded deferred tools exposed, got %#v", exposed)
	}
	// BuildDeferredReminder returns "" for no hidden lines.
	if reminder != "" {
		t.Fatalf("expected empty reminder when nothing is hidden, got %q", reminder)
	}
}
