package cli

import (
	"testing"

	"github.com/Gitlawb/zero/internal/config"
	"github.com/Gitlawb/zero/internal/sandbox"
)

// The central config->policy translation must carry scoped mode + the egress
// allow/deny lists into the engine policy (this path feeds exec, the TUI app,
// and the sandbox CLI commands).
func TestApplyConfiguredSandboxPolicyWiresScopedNetwork(t *testing.T) {
	cfg := config.SandboxConfig{
		Network:               "scoped",
		NetworkAllowedDomains: []string{"github.com", "pypi.org"},
		NetworkDeniedDomains:  []string{"evil.example"},
	}
	policy := applyConfiguredSandboxPolicy(sandbox.DefaultPolicy(), cfg)
	if policy.Network != sandbox.NetworkScoped {
		t.Fatalf("Network = %q, want scoped", policy.Network)
	}
	if len(policy.AllowedDomains) != 2 || policy.AllowedDomains[0] != "github.com" {
		t.Fatalf("AllowedDomains = %#v", policy.AllowedDomains)
	}
	if len(policy.DeniedDomains) != 1 || policy.DeniedDomains[0] != "evil.example" {
		t.Fatalf("DeniedDomains = %#v", policy.DeniedDomains)
	}
}
