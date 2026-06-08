package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Gitlawb/zero/internal/config"
	"github.com/Gitlawb/zero/internal/mcp"
	"github.com/Gitlawb/zero/internal/tools"
)

// cliFakeDeferredTool is deferred-eligible (implements Deferred() bool), mirroring
// an MCP registry tool, so it counts toward the deferral threshold.
type cliFakeDeferredTool struct {
	name string
}

func (t cliFakeDeferredTool) Name() string            { return t.name }
func (t cliFakeDeferredTool) Description() string      { return "fake deferred tool" }
func (t cliFakeDeferredTool) Parameters() tools.Schema { return tools.Schema{Type: "object"} }
func (t cliFakeDeferredTool) Safety() tools.Safety {
	return tools.Safety{SideEffect: tools.SideEffectNetwork, Permission: tools.PermissionAllow}
}
func (t cliFakeDeferredTool) Run(context.Context, map[string]any) tools.Result {
	return tools.Result{Status: tools.StatusOK, Output: "ok"}
}
func (t cliFakeDeferredTool) Deferred() bool { return true }

func registryHasToolSearch(registry *tools.Registry) bool {
	_, ok := registry.Get("tool_search")
	return ok
}

func TestRegisterToolSearchIfEligibleRegistersAtThreshold(t *testing.T) {
	registry := tools.NewRegistry()
	for i := 0; i < 3; i++ {
		registry.Register(cliFakeDeferredTool{name: "mcp_srv_t" + string(rune('a'+i))})
	}

	registerToolSearchIfEligible(registry, 3)

	if !registryHasToolSearch(registry) {
		t.Fatal("expected tool_search registered when eligible count == threshold")
	}
}

func TestRegisterToolSearchIfEligibleSkipsBelowThreshold(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(cliFakeDeferredTool{name: "mcp_srv_ta"})
	registry.Register(cliFakeDeferredTool{name: "mcp_srv_tb"})
	// A plain (non-deferred) MCP-named tool must NOT count toward eligibility.
	registry.Register(cliFakeMCPRegistryTool{})

	registerToolSearchIfEligible(registry, 3)

	if registryHasToolSearch(registry) {
		t.Fatal("expected no tool_search when eligible count (2) < threshold (3)")
	}
}

func TestRegisterToolSearchIfEligibleSkipsWhenThresholdZero(t *testing.T) {
	registry := tools.NewRegistry()
	for i := 0; i < 5; i++ {
		registry.Register(cliFakeDeferredTool{name: "mcp_srv_t" + string(rune('a'+i))})
	}

	registerToolSearchIfEligible(registry, 0)

	if registryHasToolSearch(registry) {
		t.Fatal("expected no tool_search when threshold is 0 (disabled)")
	}
}

func TestDeferredEligibleCountIgnoresCoreTools(t *testing.T) {
	registry := newCoreRegistry(t.TempDir())
	// newCoreRegistry holds only built-ins; none implement Deferred().
	if got := deferredEligibleCount(registry); got != 0 {
		t.Fatalf("deferredEligibleCount(core) = %d, want 0", got)
	}
}

func TestRunExecListToolsBelowThresholdHasNoToolSearch(t *testing.T) {
	cwd := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithDeps([]string{"exec", "--list-tools"}, &stdout, &stderr, appDeps{
		getwd: func() (string, error) { return cwd, nil },
		resolveConfig: func(string, config.Overrides) (config.ResolvedConfig, error) {
			return config.ResolvedConfig{}, errors.New("provider should not be resolved for --list-tools")
		},
		resolveMCPConfig: func(string) (config.MCPConfig, error) {
			return config.MCPConfig{Servers: map[string]config.MCPServerConfig{
				"docs": {Type: "stdio", Command: "docs-mcp"},
			}}, nil
		},
		newMCPStore: func() (*mcp.PermissionStore, error) { return nil, nil },
		registerMCPTools: func(_ context.Context, registry *tools.Registry, _ config.MCPConfig, _ mcp.RegisterOptions) (mcpToolRuntime, error) {
			// One deferred-eligible MCP tool — far below the default threshold of 10.
			registry.Register(cliFakeMCPRegistryTool{})
			return closeFunc(func() error { return nil }), nil
		},
	})

	if exitCode != exitSuccess {
		t.Fatalf("expected exit %d, got %d: %s", exitSuccess, exitCode, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "mcp_docs_lookup") {
		t.Fatalf("expected MCP tool advertised below threshold, got %q", out)
	}
	if strings.Contains(out, "tool_search") {
		t.Fatalf("expected NO tool_search below threshold, got %q", out)
	}
}
