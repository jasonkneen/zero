package config

import (
	"strings"
	"testing"
)

func TestResolveAcceptsScopedNetworkWithAllowlist(t *testing.T) {
	userPath := writeConfig(t, `{
		"sandbox": {"network": "scoped", "networkAllowedDomains": ["github.com", "pypi.org"], "networkDeniedDomains": ["evil.example"]}
	}`)
	resolved, err := Resolve(ResolveOptions{UserConfigPath: userPath, Env: map[string]string{}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Sandbox.Network != "scoped" {
		t.Fatalf("Network = %q, want scoped", resolved.Sandbox.Network)
	}
	if len(resolved.Sandbox.NetworkAllowedDomains) != 2 || resolved.Sandbox.NetworkAllowedDomains[0] != "github.com" {
		t.Fatalf("NetworkAllowedDomains = %#v", resolved.Sandbox.NetworkAllowedDomains)
	}
	if len(resolved.Sandbox.NetworkDeniedDomains) != 1 {
		t.Fatalf("NetworkDeniedDomains = %#v", resolved.Sandbox.NetworkDeniedDomains)
	}
}

func TestResolveRejectsScopedNetworkWithoutAllowlist(t *testing.T) {
	userPath := writeConfig(t, `{"sandbox": {"network": "scoped"}}`)
	_, err := Resolve(ResolveOptions{UserConfigPath: userPath, Env: map[string]string{}})
	if err == nil || !strings.Contains(err.Error(), "scoped requires a non-empty sandbox.networkAllowedDomains") {
		t.Fatalf("expected scoped-without-allowlist error, got %v", err)
	}
}

func TestResolveRejectsUnknownNetworkMode(t *testing.T) {
	userPath := writeConfig(t, `{"sandbox": {"network": "bogus"}}`)
	_, err := Resolve(ResolveOptions{UserConfigPath: userPath, Env: map[string]string{}})
	if err == nil || !strings.Contains(err.Error(), "expected allow, deny, or scoped") {
		t.Fatalf("expected unknown-mode error, got %v", err)
	}
}

// Security: a project config (a cloned repo) must NOT be able to add to the
// scoped egress allowlist — only the global user config can.
func TestResolveProjectCannotWidenNetworkAllowlist(t *testing.T) {
	userPath := writeConfig(t, `{"sandbox": {"network": "scoped", "networkAllowedDomains": ["github.com"]}}`)
	projectPath := writeConfig(t, `{"sandbox": {"networkAllowedDomains": ["exfil.example"]}}`)
	resolved, err := Resolve(ResolveOptions{UserConfigPath: userPath, ProjectConfigPath: projectPath, Env: map[string]string{}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	for _, d := range resolved.Sandbox.NetworkAllowedDomains {
		if d == "exfil.example" {
			t.Fatalf("project config widened the egress allowlist: %#v", resolved.Sandbox.NetworkAllowedDomains)
		}
	}
	if len(resolved.Sandbox.NetworkAllowedDomains) != 1 || resolved.Sandbox.NetworkAllowedDomains[0] != "github.com" {
		t.Fatalf("allowlist = %#v, want [github.com] only", resolved.Sandbox.NetworkAllowedDomains)
	}
}
