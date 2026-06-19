package doctor

import (
	"testing"

	"github.com/Gitlawb/zero/internal/config"
	"github.com/Gitlawb/zero/internal/sandbox"
)

func TestDoctorSandboxPolicyWiresScopedNetwork(t *testing.T) {
	cfg := config.SandboxConfig{Network: "scoped", NetworkAllowedDomains: []string{"github.com"}}
	policy := doctorSandboxPolicy(cfg)
	if policy.Network != sandbox.NetworkScoped {
		t.Fatalf("Network = %q, want scoped", policy.Network)
	}
	if len(policy.AllowedDomains) != 1 || policy.AllowedDomains[0] != "github.com" {
		t.Fatalf("AllowedDomains = %#v", policy.AllowedDomains)
	}
}
