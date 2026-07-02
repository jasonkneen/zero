package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/Gitlawb/zero/internal/config"
	"github.com/Gitlawb/zero/internal/providercatalog"
	"github.com/Gitlawb/zero/internal/zeroruntime"
)

// TestSwitchProviderModelAllowsAuthCLIProfile: an AuthCLI profile carries no
// APIKey/AuthHeaderValue by design — its credential is live-read from the agent
// CLI's own store inside the provider factory. The picker's credential gate
// previously rejected such profiles with "no usable credential", which made a
// logged-in Claude Code profile unusable from /model even though it builds and
// runs fine.
func TestSwitchProviderModelAllowsAuthCLIProfile(t *testing.T) {
	built := 0
	m := newModel(context.Background(), Options{
		ProviderName:    "openrouter",
		ModelName:       "anthropic/claude-sonnet-5",
		Provider:        &fakeProvider{},
		ProviderProfile: config.ProviderProfile{Name: "openrouter", CatalogID: "openrouter", Model: "anthropic/claude-sonnet-5"},
		SavedProviders: []config.ProviderProfile{
			{Name: "openrouter", CatalogID: "openrouter", Model: "anthropic/claude-sonnet-5"},
			{Name: "claude", CatalogID: "anthropic", ProviderKind: config.ProviderKindAnthropic, AuthCLI: "claude", Model: "claude-sonnet-5"},
		},
		NewProvider: func(profile config.ProviderProfile) (zeroruntime.Provider, error) {
			built++
			if profile.AuthCLI != "claude" {
				t.Fatalf("provider built from profile without AuthCLI: %+v", profile)
			}
			return &fakeProvider{}, nil
		},
	})

	next, text, _ := m.switchProviderModel("claude", "claude-opus-4.8")
	if strings.Contains(text, "no usable credential") {
		t.Fatalf("AuthCLI profile rejected by the credential gate: %q", text)
	}
	if !strings.Contains(text, "Switched to claude") {
		t.Fatalf("switch notice = %q, want it to confirm the switch", text)
	}
	if built != 1 {
		t.Fatalf("provider builds = %d, want 1", built)
	}
	if next.providerName != "claude" || next.modelName != "claude-opus-4.8" {
		t.Fatalf("model/provider not switched: providerName=%q modelName=%q", next.providerName, next.modelName)
	}
}

// TestModelPickerProviderGroupCLIProfileDistinct: a CLI-authenticated profile
// resolves to the same catalog descriptor as a plain API-key profile for that
// provider, so its picker group must carry a "via <CLI>" suffix — both so the
// user can see which login the models run on and so the picker's per-group
// dedup cannot silently drop one of the two profiles.
func TestModelPickerProviderGroupCLIProfileDistinct(t *testing.T) {
	descriptor, ok := providercatalog.Get("anthropic")
	if !ok {
		t.Fatal("anthropic catalog descriptor missing")
	}

	plain := config.ProviderProfile{Name: "anthropic", CatalogID: "anthropic"}
	cli := config.ProviderProfile{Name: "claude", CatalogID: "anthropic", AuthCLI: "claude"}

	plainGroup := modelPickerProviderGroup(plain, descriptor, true)
	cliGroup := modelPickerProviderGroup(cli, descriptor, true)
	if plainGroup == cliGroup {
		t.Fatalf("CLI profile group %q collides with plain profile group — dedup would drop one", cliGroup)
	}
	if !strings.Contains(cliGroup, "Claude Code") {
		t.Fatalf("CLI profile group = %q, want the harness display name in it", cliGroup)
	}

	// An unknown harness id still yields a distinct group (raw id fallback)
	// rather than colliding with the plain profile.
	unknown := config.ProviderProfile{Name: "custom", CatalogID: "anthropic", AuthCLI: "some-future-cli"}
	unknownGroup := modelPickerProviderGroup(unknown, descriptor, true)
	if unknownGroup == plainGroup || !strings.Contains(unknownGroup, "some-future-cli") {
		t.Fatalf("unknown-harness group = %q, want distinct raw-id fallback", unknownGroup)
	}
}

// TestNormalizeProfileForProviderPreservesCLIProfile: switching models within
// a CLI-authenticated profile must not normalize its identity to the catalog
// id — that made persistence miss the real profile ("not saved: provider
// \"anthropic\" not found") — and must not autofill APIKeyEnv/APIKey, which
// would displace the CLI login with an ambient env key.
func TestNormalizeProfileForProviderPreservesCLIProfile(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "ambient-key-must-not-be-used")
	descriptor, ok := providercatalog.Get("anthropic")
	if !ok {
		t.Fatal("anthropic catalog descriptor missing")
	}
	m := newModel(context.Background(), Options{
		ProviderName:    "claude",
		ModelName:       "claude-sonnet-5",
		Provider:        &fakeProvider{},
		ProviderProfile: config.ProviderProfile{Name: "claude", CatalogID: "anthropic", AuthCLI: "claude", Model: "claude-sonnet-5"},
	})

	profile := m.normalizeProfileForProvider(descriptor)
	if profile.Name != "claude" {
		t.Fatalf("Name = %q, want the CLI profile name preserved", profile.Name)
	}
	if profile.APIKeyEnv != "" || profile.APIKey != "" {
		t.Fatalf("APIKeyEnv=%q APIKey=%q, want both empty — CLI profiles stay keyless", profile.APIKeyEnv, profile.APIKey)
	}
	if profile.AuthCLI != "claude" {
		t.Fatalf("AuthCLI = %q, want preserved", profile.AuthCLI)
	}
}
