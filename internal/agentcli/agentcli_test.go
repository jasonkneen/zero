package agentcli

import (
	"errors"
	"strings"
	"testing"
)

// --- catalog sanity ----------------------------------------------------

func TestCatalogHasUniqueIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, h := range Harnesses() {
		if seen[h.ID] {
			t.Fatalf("duplicate harness ID %q", h.ID)
		}
		seen[h.ID] = true
	}
}

func TestCatalogEveryHarnessHasABinary(t *testing.T) {
	for _, h := range Harnesses() {
		if len(h.Binaries) == 0 {
			t.Fatalf("harness %q has no Binaries", h.ID)
		}
		for _, bin := range h.Binaries {
			if strings.TrimSpace(bin) == "" {
				t.Fatalf("harness %q has a blank binary name", h.ID)
			}
		}
	}
}

func TestLookupRoundTrip(t *testing.T) {
	for _, h := range Harnesses() {
		got, ok := Lookup(h.ID)
		if !ok {
			t.Fatalf("Lookup(%q) not found", h.ID)
		}
		if got.ID != h.ID || got.DisplayName != h.DisplayName {
			t.Fatalf("Lookup(%q) = %+v, want %+v", h.ID, got, h)
		}
	}
	if _, ok := Lookup("does-not-exist"); ok {
		t.Fatal("Lookup of unknown id should report false")
	}
}

func TestHarnessesReturnsIndependentCopy(t *testing.T) {
	first := Harnesses()
	first[0].Binaries[0] = "mutated"
	first[0].ID = "mutated"
	second := Harnesses()
	if second[0].ID == "mutated" || second[0].Binaries[0] == "mutated" {
		t.Fatal("Harnesses() must return a copy; mutation leaked into the catalog")
	}
}

func TestCatalogPrintArgsEmbedsPrompt(t *testing.T) {
	const prompt = "unique-prompt-token-xyz"
	for _, h := range Harnesses() {
		if h.PrintArgs == nil {
			t.Fatalf("harness %q has nil PrintArgs", h.ID)
		}
		args := h.PrintArgs(prompt)
		found := false
		for _, a := range args {
			if strings.Contains(a, prompt) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("harness %q PrintArgs(%q) = %v, want an arg containing the prompt", h.ID, prompt, args)
		}
	}
}

// --- Detect --------------------------------------------------------------

// fakeLookPath resolves only the given binary names, mapping each to
// "/fake/bin/<name>".
func fakeLookPath(found ...string) func(string) (string, error) {
	set := map[string]bool{}
	for _, f := range found {
		set[f] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/fake/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

func TestDetectOmitsHarnessesNotOnPath(t *testing.T) {
	deps := Deps{
		LookPath: fakeLookPath("claude"),
		Home:     "/home/u",
		ReadFile: func(string) ([]byte, error) { return nil, errors.New("no such file") },
		// claude declares KeychainService; inject a stub so this test can't fall
		// through to the real darwin `security` default and shell out to this
		// machine's actual Claude Code keychain entry.
		Keychain: func(string) ([]byte, error) { return nil, errors.New("not found") },
	}
	detections := Detect(deps)
	if len(detections) != 1 {
		t.Fatalf("Detect() = %d detections, want 1 (only claude on PATH): %+v", len(detections), detections)
	}
	if detections[0].Harness.ID != "claude" {
		t.Fatalf("Detect()[0].Harness.ID = %q, want claude", detections[0].Harness.ID)
	}
	if detections[0].Path != "/fake/bin/claude" {
		t.Fatalf("Detect()[0].Path = %q, want /fake/bin/claude", detections[0].Path)
	}
}

func TestDetectLoginStates(t *testing.T) {
	tests := []struct {
		name     string
		harness  string
		readFile func(string) ([]byte, error)
		keychain func(string) ([]byte, error)
		want     LoginState
	}{
		{
			name:    "aider has no probes at all -> unknown",
			harness: "aider",
			readFile: func(string) ([]byte, error) {
				return nil, errors.New("no such file")
			},
			want: LoginUnknown,
		},
		{
			name:    "codex cred file present -> logged in",
			harness: "codex",
			readFile: func(path string) ([]byte, error) {
				if strings.HasSuffix(path, ".codex/auth.json") {
					return []byte(`{}`), nil
				}
				return nil, errors.New("no such file")
			},
			want: LoggedIn,
		},
		{
			name:    "codex cred file absent -> logged out",
			harness: "codex",
			readFile: func(string) ([]byte, error) {
				return nil, errors.New("no such file")
			},
			want: LoggedOut,
		},
		{
			name:    "claude keychain hit -> logged in even without cred file",
			harness: "claude",
			readFile: func(string) ([]byte, error) {
				return nil, errors.New("no such file")
			},
			keychain: func(service string) ([]byte, error) {
				if service == "Claude Code-credentials" {
					return []byte("secret"), nil
				}
				return nil, errors.New("not found")
			},
			want: LoggedIn,
		},
		{
			name:    "claude no cred file, no keychain hit -> logged out",
			harness: "claude",
			readFile: func(string) ([]byte, error) {
				return nil, errors.New("no such file")
			},
			keychain: func(string) ([]byte, error) {
				return nil, errors.New("not found")
			},
			want: LoggedOut,
		},
		{
			name:    "crush checks the second CredFile entry too",
			harness: "crush",
			readFile: func(path string) ([]byte, error) {
				if strings.HasSuffix(path, ".local/share/crush/crush.json") {
					return []byte(`{}`), nil
				}
				return nil, errors.New("no such file")
			},
			want: LoggedIn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, ok := Lookup(tt.harness)
			if !ok {
				t.Fatalf("Lookup(%q) failed", tt.harness)
			}
			deps := Deps{
				LookPath: fakeLookPath(h.Binaries[0]),
				Home:     "/home/u",
				ReadFile: tt.readFile,
				Keychain: tt.keychain,
			}
			detections := Detect(deps)
			if len(detections) != 1 {
				t.Fatalf("Detect() = %d detections, want 1: %+v", len(detections), detections)
			}
			if detections[0].Login != tt.want {
				t.Fatalf("Login = %v, want %v", detections[0].Login, tt.want)
			}
		})
	}
}

func TestDetectHomeRelativeResolution(t *testing.T) {
	var seenPath string
	deps := Deps{
		LookPath: fakeLookPath("codex"),
		Home:     "/custom/home",
		ReadFile: func(path string) ([]byte, error) {
			seenPath = path
			return nil, errors.New("no such file")
		},
	}
	Detect(deps)
	if !strings.HasPrefix(seenPath, "/custom/home") {
		t.Fatalf("ReadFile path = %q, want it rooted at Home /custom/home", seenPath)
	}
	if !strings.HasSuffix(seenPath, ".codex/auth.json") {
		t.Fatalf("ReadFile path = %q, want suffix .codex/auth.json", seenPath)
	}
}

func TestDetectKeychainOnlyConsultedForHarnessesThatDeclareIt(t *testing.T) {
	calls := map[string]int{}
	deps := Deps{
		// Every harness's first binary is "installed" so Detect probes all of them.
		LookPath: func(name string) (string, error) { return "/fake/bin/" + name, nil },
		Home:     "/home/u",
		ReadFile: func(string) ([]byte, error) { return nil, errors.New("no such file") },
		Keychain: func(service string) ([]byte, error) {
			calls[service]++
			return nil, errors.New("not found")
		},
	}
	Detect(deps)
	if len(calls) != 1 {
		t.Fatalf("Keychain called for %d distinct services, want 1 (only claude declares KeychainService): %v", len(calls), calls)
	}
	if calls["Claude Code-credentials"] == 0 {
		t.Fatalf("Keychain never called for claude's service: %v", calls)
	}
}

func TestDetectOne(t *testing.T) {
	deps := Deps{
		LookPath: fakeLookPath("codex"),
		Home:     "/home/u",
		ReadFile: func(string) ([]byte, error) { return nil, errors.New("no such file") },
	}
	if _, ok := DetectOne("claude", deps); ok {
		t.Fatal("DetectOne(claude) should fail: not on PATH")
	}
	det, ok := DetectOne("codex", deps)
	if !ok {
		t.Fatal("DetectOne(codex) should succeed: on PATH")
	}
	if det.Harness.ID != "codex" || det.Login != LoggedOut {
		t.Fatalf("DetectOne(codex) = %+v, want ID=codex Login=LoggedOut", det)
	}
	if _, ok := DetectOne("not-a-real-harness", deps); ok {
		t.Fatal("DetectOne of an unknown id should report false")
	}
}

func TestHarnessLoginCommand(t *testing.T) {
	codex, ok := Lookup("codex")
	if !ok {
		t.Fatal("test assumption broken: codex missing from the catalog")
	}
	if got := codex.LoginCommand(); got != "codex login" {
		t.Fatalf("codex.LoginCommand() = %q, want %q (explicit LoginHint)", got, "codex login")
	}

	claude, ok := Lookup("claude")
	if !ok {
		t.Fatal("test assumption broken: claude missing from the catalog")
	}
	// claude has no explicit LoginHint — falls back to the first binary name,
	// which is always a safe suggestion (the CLI walks the user through login
	// on first interactive run).
	if got := claude.LoginCommand(); got != "claude" {
		t.Fatalf("claude.LoginCommand() = %q, want the fallback binary name %q", got, "claude")
	}

	custom := Harness{ID: "x", LoginHint: "  x auth  ", Binaries: []string{"x"}}
	if got := custom.LoginCommand(); got != "x auth" {
		t.Fatalf("LoginCommand() = %q, want the trimmed explicit hint", got)
	}
}
