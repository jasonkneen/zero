package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Gitlawb/zero/internal/agentcli"
	"github.com/Gitlawb/zero/internal/config"
	"github.com/Gitlawb/zero/internal/oauth"
)

// withAuthStore points the provider OAuth store at a temp file for the test.
func withAuthStore(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "oauth-tokens.json")
	t.Setenv("ZERO_OAUTH_TOKENS_PATH", path)
	return path
}

func TestRunAuthRejectsInvalidStorageMode(t *testing.T) {
	withAuthStore(t)
	// A mistyped value must fail fast, not silently fall back to plaintext while
	// the user believes encryption is active.
	t.Setenv("ZERO_OAUTH_STORAGE", "encryptd")
	var stdout, stderr bytes.Buffer
	if code := runWithDeps([]string{"auth", "status"}, &stdout, &stderr, appDeps{}); code == exitSuccess {
		t.Fatalf("invalid ZERO_OAUTH_STORAGE should fail, got success; stdout=%q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "ZERO_OAUTH_STORAGE") {
		t.Fatalf("error should name the offending env var, stderr=%q", stderr.String())
	}
}

func TestRunAuthStatusEmpty(t *testing.T) {
	withAuthStore(t)
	var stdout, stderr bytes.Buffer
	if code := runWithDeps([]string{"auth", "status"}, &stdout, &stderr, appDeps{}); code != exitSuccess {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No OAuth provider logins are stored.") {
		t.Fatalf("status output = %q", stdout.String())
	}
}

func TestRunAuthStatusReportsLoginWithoutSecret(t *testing.T) {
	path := withAuthStore(t)
	store, err := oauth.NewStore(oauth.StoreOptions{FilePath: path})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.Save(oauth.ProviderKey("demo"), oauth.Token{
		AccessToken: "super-secret", RefreshToken: "super-secret-rt", Account: "me@example.com",
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if code := runWithDeps([]string{"auth", "status"}, &stdout, &stderr, appDeps{}); code != exitSuccess {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "demo") || !strings.Contains(out, "me@example.com") {
		t.Fatalf("status should show provider + account: %q", out)
	}
	if strings.Contains(out, "super-secret") {
		t.Fatalf("status leaked token material: %q", out)
	}
}

func TestRunAuthLogoutNothing(t *testing.T) {
	withAuthStore(t)
	var stdout, stderr bytes.Buffer
	if code := runWithDeps([]string{"auth", "logout", "demo"}, &stdout, &stderr, appDeps{}); code != exitSuccess {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No stored credential for demo") {
		t.Fatalf("logout output = %q", stdout.String())
	}
}

func TestRunAuthLoginValidation(t *testing.T) {
	withAuthStore(t)
	var stdout, stderr bytes.Buffer
	// Missing provider.
	if code := runWithDeps([]string{"auth", "login"}, &stdout, &stderr, appDeps{}); code == exitSuccess {
		t.Fatal("login with no provider should fail")
	}
	// --json is rejected for the interactive login.
	stdout.Reset()
	stderr.Reset()
	if code := runWithDeps([]string{"auth", "login", "demo", "--json"}, &stdout, &stderr, appDeps{}); code == exitSuccess {
		t.Fatal("login --json should be rejected")
	}
}

func TestRunAuthLoginUnknownProvider(t *testing.T) {
	withAuthStore(t)
	var stdout, stderr bytes.Buffer
	if code := runWithDeps([]string{"auth", "login", "does-not-exist"}, &stdout, &stderr, appDeps{}); code == exitSuccess {
		t.Fatal("unknown provider login should fail")
	}
	if !strings.Contains(stderr.String(), "not configured") {
		t.Fatalf("stderr = %q, want not-configured error", stderr.String())
	}
}

func TestRunAuthRefreshNoToken(t *testing.T) {
	withAuthStore(t)
	t.Setenv("ZERO_OAUTH_DEMO_CLIENT_ID", "client") // so config resolves; refresh still fails (no token)
	var stdout, stderr bytes.Buffer
	if code := runWithDeps([]string{"auth", "refresh", "demo"}, &stdout, &stderr, appDeps{}); code == exitSuccess {
		t.Fatal("refresh with no stored token should fail")
	}
}

func TestRunAuthRejectsWrongFlags(t *testing.T) {
	withAuthStore(t)
	cases := [][]string{
		{"auth", "login", "demo", "--watch"},       // watch is refresh-only
		{"auth", "login", "demo", "--json"},        // json not for interactive login
		{"auth", "status", "demo", "--device"},     // device is login-only
		{"auth", "logout", "demo", "--scope", "x"}, // scope is login-only
		{"auth", "refresh", "demo", "--json"},      // json not for refresh
		{"auth", "login", "demo", "--scope", ""},   // empty scope rejected
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		if code := runWithDeps(args, &stdout, &stderr, appDeps{}); code == exitSuccess {
			t.Errorf("args %v should be rejected, got success", args)
		}
	}
}

func TestRunAuthOpenRouterRejectsArgs(t *testing.T) {
	withAuthStore(t)
	var stdout, stderr bytes.Buffer
	// An unexpected arg/flag must fail fast, not silently run the login.
	if code := runWithDeps([]string{"auth", "openrouter", "--json"}, &stdout, &stderr, appDeps{}); code == exitSuccess {
		t.Fatalf("openrouter with an unexpected flag should fail; stdout=%q", stdout.String())
	}
	// --help still works.
	stdout.Reset()
	stderr.Reset()
	if code := runWithDeps([]string{"auth", "openrouter", "--help"}, &stdout, &stderr, appDeps{}); code != exitSuccess {
		t.Fatalf("openrouter --help should succeed, stderr=%q", stderr.String())
	}
}

func TestRunAuthHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runWithDeps([]string{"auth", "--help"}, &stdout, &stderr, appDeps{}); code != exitSuccess {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{"zero auth", "login", "logout", "status", "refresh", "--device"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}

// authTestAgentCLIDeps stubs appDeps.detectAgentCLIs and resolveConfig so
// `zero auth status`'s "Detected agent CLIs" section is exercised hermetically
// — never touching the real PATH, filesystem, or which providers happen to be
// configured on the machine running the test.
func authTestAgentCLIDeps(t *testing.T, detections []agentcli.Detection, providers []config.ProviderProfile) appDeps {
	t.Helper()
	return appDeps{
		detectAgentCLIs: func(agentcli.Deps) []agentcli.Detection { return detections },
		getwd:           func() (string, error) { return t.TempDir(), nil },
		resolveConfig: func(string, config.Overrides) (config.ResolvedConfig, error) {
			return config.ResolvedConfig{Providers: providers}, nil
		},
	}
}

func TestRunAuthStatusListsDetectedAgentCLIs(t *testing.T) {
	withAuthStore(t)
	claudeHarness, ok := agentcli.Lookup("claude")
	if !ok {
		t.Fatal("test assumption broken: claude missing from the agentcli catalog")
	}
	codexHarness, ok := agentcli.Lookup("codex")
	if !ok {
		t.Fatal("test assumption broken: codex missing from the agentcli catalog")
	}
	geminiHarness, ok := agentcli.Lookup("gemini")
	if !ok {
		t.Fatal("test assumption broken: gemini missing from the agentcli catalog")
	}
	deps := authTestAgentCLIDeps(t,
		[]agentcli.Detection{
			{Harness: claudeHarness, Login: agentcli.LoggedIn},
			{Harness: codexHarness, Login: agentcli.LoggedOut},
			{Harness: geminiHarness, Login: agentcli.LoggedOut},
		},
		[]config.ProviderProfile{{Name: "my-claude", CatalogID: "anthropic", AuthCLI: "claude"}},
	)

	var stdout, stderr bytes.Buffer
	if code := runWithDeps([]string{"auth", "status"}, &stdout, &stderr, deps); code != exitSuccess {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Detected agent CLIs:") {
		t.Fatalf("status output missing the agent-CLI section: %q", out)
	}
	if !strings.Contains(out, "claude") || !strings.Contains(out, "logged in") {
		t.Fatalf("status output missing claude's logged-in state: %q", out)
	}
	if !strings.Contains(out, `used by profile "my-claude"`) {
		t.Fatalf("status output should attribute claude to the my-claude profile: %q", out)
	}
	if !strings.Contains(out, "codex") || !strings.Contains(out, "not logged in") {
		t.Fatalf("status output missing codex's logged-out state: %q", out)
	}
	if !strings.Contains(out, "gemini") || !strings.Contains(out, "sub-agent harness only") {
		t.Fatalf("status output should note gemini has no reusable provider credentials: %q", out)
	}
}

// TestRunAuthStatusFilteredByProviderOmitsAgentCLIs locks in that the
// agent-CLI section only appears on the unfiltered `zero auth status` — a
// `zero auth status <provider>` query is about one zero-native OAuth login,
// not the whole-machine CLI inventory.
func TestRunAuthStatusFilteredByProviderOmitsAgentCLIs(t *testing.T) {
	withAuthStore(t)
	claudeHarness, ok := agentcli.Lookup("claude")
	if !ok {
		t.Fatal("test assumption broken: claude missing from the agentcli catalog")
	}
	deps := authTestAgentCLIDeps(t, []agentcli.Detection{{Harness: claudeHarness, Login: agentcli.LoggedIn}}, nil)

	var stdout, stderr bytes.Buffer
	if code := runWithDeps([]string{"auth", "status", "demo"}, &stdout, &stderr, deps); code != exitSuccess {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "Detected agent CLIs:") {
		t.Fatalf("filtered status should not include the agent-CLI section: %q", stdout.String())
	}
}

func TestRunAuthStatusJSONIncludesAgentCLIs(t *testing.T) {
	withAuthStore(t)
	claudeHarness, ok := agentcli.Lookup("claude")
	if !ok {
		t.Fatal("test assumption broken: claude missing from the agentcli catalog")
	}
	deps := authTestAgentCLIDeps(t, []agentcli.Detection{{Harness: claudeHarness, Login: agentcli.LoggedIn}}, nil)

	var stdout, stderr bytes.Buffer
	if code := runWithDeps([]string{"auth", "status", "--json"}, &stdout, &stderr, deps); code != exitSuccess {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"id": "claude"`) {
		t.Fatalf("JSON status missing agentCLIs[].id: %s", out)
	}
	if !strings.Contains(out, `"loggedIn": true`) {
		t.Fatalf("JSON status missing agentCLIs[].loggedIn: %s", out)
	}
}
