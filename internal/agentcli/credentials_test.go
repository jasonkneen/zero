package agentcli

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestExtractCredentialsUnsupportedHarness(t *testing.T) {
	h, ok := Lookup("gemini")
	if !ok {
		t.Fatal("Lookup(gemini) failed")
	}
	creds, ok, err := ExtractCredentials(h, Deps{})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if ok {
		t.Fatalf("ok = true, want false for a harness with no extractor: %+v", creds)
	}
}

// --- codex -----------------------------------------------------------------

func TestExtractCodexCredentials(t *testing.T) {
	h, ok := Lookup("codex")
	if !ok {
		t.Fatal("Lookup(codex) failed")
	}

	t.Run("full", func(t *testing.T) {
		deps := Deps{Home: "/home/u", ReadFile: func(path string) ([]byte, error) {
			return []byte(`{
				"OPENAI_API_KEY": "sk-plain",
				"tokens": {
					"access_token": "at-1",
					"refresh_token": "rt-1",
					"account_id": "acct-1",
					"id_token": "it-1"
				}
			}`), nil
		}}
		creds, ok, err := ExtractCredentials(h, deps)
		if err != nil || !ok {
			t.Fatalf("ExtractCredentials: ok=%v err=%v", ok, err)
		}
		want := Credentials{APIKey: "sk-plain", AccessToken: "at-1", RefreshToken: "rt-1", AccountID: "acct-1"}
		if creds != want {
			t.Fatalf("creds = %+v, want %+v", creds, want)
		}
	})

	t.Run("key only", func(t *testing.T) {
		deps := Deps{Home: "/home/u", ReadFile: func(string) ([]byte, error) {
			return []byte(`{"OPENAI_API_KEY": "sk-plain"}`), nil
		}}
		creds, ok, err := ExtractCredentials(h, deps)
		if err != nil || !ok {
			t.Fatalf("ExtractCredentials: ok=%v err=%v", ok, err)
		}
		if creds.APIKey != "sk-plain" || creds.AccessToken != "" {
			t.Fatalf("creds = %+v, want only APIKey set", creds)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		deps := Deps{Home: "/home/u", ReadFile: func(string) ([]byte, error) {
			return nil, errors.New("no such file")
		}}
		creds, ok, err := ExtractCredentials(h, deps)
		if err != nil {
			t.Fatalf("err = %v, want nil for a missing file", err)
		}
		if ok {
			t.Fatalf("ok = true, want false for a missing file: %+v", creds)
		}
	})

	t.Run("empty object -> ok false", func(t *testing.T) {
		deps := Deps{Home: "/home/u", ReadFile: func(string) ([]byte, error) {
			return []byte(`{}`), nil
		}}
		_, ok, err := ExtractCredentials(h, deps)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if ok {
			t.Fatal("ok = true, want false for an auth.json with nothing usable in it")
		}
	})

	t.Run("malformed", func(t *testing.T) {
		deps := Deps{Home: "/home/u", ReadFile: func(string) ([]byte, error) {
			return []byte(`not json`), nil
		}}
		_, ok, err := ExtractCredentials(h, deps)
		if err == nil {
			t.Fatal("err = nil, want a parse error for malformed JSON")
		}
		if ok {
			t.Fatal("ok = true, want false alongside the parse error")
		}
	})

	t.Run("resolves under Home", func(t *testing.T) {
		var seenPath string
		deps := Deps{Home: "/custom/home", ReadFile: func(path string) ([]byte, error) {
			seenPath = path
			return nil, errors.New("no such file")
		}}
		ExtractCredentials(h, deps)
		if !strings.HasPrefix(seenPath, "/custom/home") || !strings.HasSuffix(seenPath, ".codex/auth.json") {
			t.Fatalf("path = %q, want rooted at Home and ending .codex/auth.json", seenPath)
		}
	})
}

// --- claude ------------------------------------------------------------

func TestExtractClaudeCredentials(t *testing.T) {
	h, ok := Lookup("claude")
	if !ok {
		t.Fatal("Lookup(claude) failed")
	}
	const blob = `{"claudeAiOauth":{"accessToken":"at-1","refreshToken":"rt-1","expiresAt":1700000000000}}`

	t.Run("keychain hit", func(t *testing.T) {
		var readFileCalled bool
		deps := Deps{
			Home: "/home/u",
			ReadFile: func(string) ([]byte, error) {
				readFileCalled = true
				return nil, errors.New("should not be reached")
			},
			Keychain: func(service string) ([]byte, error) {
				if service != "Claude Code-credentials" {
					return nil, errors.New("wrong service")
				}
				return []byte(blob), nil
			},
		}
		creds, ok, err := ExtractCredentials(h, deps)
		if err != nil || !ok {
			t.Fatalf("ExtractCredentials: ok=%v err=%v", ok, err)
		}
		if readFileCalled {
			t.Fatal("ReadFile should not be called when the keychain has data")
		}
		if creds.AccessToken != "at-1" || creds.RefreshToken != "rt-1" {
			t.Fatalf("creds = %+v", creds)
		}
		wantExpires := time.UnixMilli(1700000000000)
		if !creds.ExpiresAt.Equal(wantExpires) {
			t.Fatalf("ExpiresAt = %v, want %v", creds.ExpiresAt, wantExpires)
		}
	})

	t.Run("file fallback when keychain misses", func(t *testing.T) {
		deps := Deps{
			Home: "/home/u",
			ReadFile: func(path string) ([]byte, error) {
				if strings.HasSuffix(path, ".claude/.credentials.json") {
					return []byte(blob), nil
				}
				return nil, errors.New("no such file")
			},
			Keychain: func(string) ([]byte, error) {
				return nil, errors.New("not found")
			},
		}
		creds, ok, err := ExtractCredentials(h, deps)
		if err != nil || !ok {
			t.Fatalf("ExtractCredentials: ok=%v err=%v", ok, err)
		}
		if creds.AccessToken != "at-1" {
			t.Fatalf("creds = %+v", creds)
		}
	})

	t.Run("keychain returns no error but empty data still falls back to file", func(t *testing.T) {
		// Regression guard for the `len(data) > 0` gate: an err==nil, empty-body
		// response must not be treated as a hit. Keychain is always explicitly
		// injected here (never left nil) so the test can't fall through to the
		// real darwin `security` default and touch this machine's actual
		// Claude Code keychain entry.
		deps := Deps{
			Home: "/home/u",
			ReadFile: func(path string) ([]byte, error) {
				if strings.HasSuffix(path, ".claude/.credentials.json") {
					return []byte(blob), nil
				}
				return nil, errors.New("no such file")
			},
			Keychain: func(string) ([]byte, error) {
				return nil, nil
			},
		}
		creds, ok, err := ExtractCredentials(h, deps)
		if err != nil || !ok {
			t.Fatalf("ExtractCredentials: ok=%v err=%v", ok, err)
		}
		if creds.AccessToken != "at-1" {
			t.Fatalf("creds = %+v", creds)
		}
	})

	t.Run("missing everywhere", func(t *testing.T) {
		deps := Deps{
			Home:     "/home/u",
			ReadFile: func(string) ([]byte, error) { return nil, errors.New("no such file") },
			Keychain: func(string) ([]byte, error) { return nil, errors.New("not found") },
		}
		creds, ok, err := ExtractCredentials(h, deps)
		if err != nil {
			t.Fatalf("err = %v, want nil when nothing is stored", err)
		}
		if ok {
			t.Fatalf("ok = true, want false: %+v", creds)
		}
	})

	t.Run("expiresAt zero when absent", func(t *testing.T) {
		deps := Deps{
			Home: "/home/u",
			ReadFile: func(string) ([]byte, error) {
				return []byte(`{"claudeAiOauth":{"accessToken":"at-1","refreshToken":"rt-1"}}`), nil
			},
			Keychain: func(string) ([]byte, error) { return nil, errors.New("not found") },
		}
		creds, ok, err := ExtractCredentials(h, deps)
		if err != nil || !ok {
			t.Fatalf("ExtractCredentials: ok=%v err=%v", ok, err)
		}
		if !creds.ExpiresAt.IsZero() {
			t.Fatalf("ExpiresAt = %v, want zero value", creds.ExpiresAt)
		}
	})

	t.Run("malformed", func(t *testing.T) {
		deps := Deps{
			Home:     "/home/u",
			ReadFile: func(string) ([]byte, error) { return []byte(`not json`), nil },
			Keychain: func(string) ([]byte, error) { return nil, errors.New("not found") },
		}
		_, ok, err := ExtractCredentials(h, deps)
		if err == nil {
			t.Fatal("err = nil, want a parse error for malformed JSON")
		}
		if ok {
			t.Fatal("ok = true, want false alongside the parse error")
		}
	})
}

// TestExtractClaudeCredentialsKeychainWithoutOauthFallsThrough locks the fix
// for a real-world shadowing bug: the keychain entry can exist while holding
// only MCP-server tokens (a JSON blob with no claudeAiOauth key), with the
// actual login stored in ~/.claude/.credentials.json. Extraction must fall
// through to the file instead of stopping at the first non-empty source.
func TestExtractClaudeCredentialsKeychainWithoutOauthFallsThrough(t *testing.T) {
	h, ok := Lookup("claude")
	if !ok {
		t.Fatal("Lookup(claude) failed")
	}
	fileBlob := `{"claudeAiOauth":{"accessToken":"at-file","refreshToken":"rt-file","expiresAt":1700000000000}}`

	t.Run("keychain holds only mcp tokens", func(t *testing.T) {
		deps := Deps{
			Home: "/home/u",
			Keychain: func(string) ([]byte, error) {
				return []byte(`{"mcpOAuth":{"some-server|abc":{"accessToken":"mcp"}}}`), nil
			},
			ReadFile: func(path string) ([]byte, error) {
				if path != "/home/u/.claude/.credentials.json" {
					return nil, errors.New("unexpected path " + path)
				}
				return []byte(fileBlob), nil
			},
		}
		creds, ok, err := ExtractCredentials(h, deps)
		if err != nil || !ok {
			t.Fatalf("ExtractCredentials: ok=%v err=%v", ok, err)
		}
		if creds.AccessToken != "at-file" {
			t.Fatalf("AccessToken = %q, want the file's token (keychain blob shadowed the file)", creds.AccessToken)
		}
	})

	t.Run("malformed keychain blob still reaches the file", func(t *testing.T) {
		deps := Deps{
			Home:     "/home/u",
			Keychain: func(string) ([]byte, error) { return []byte(`{not json`), nil },
			ReadFile: func(string) ([]byte, error) { return []byte(fileBlob), nil },
		}
		creds, ok, err := ExtractCredentials(h, deps)
		if err != nil || !ok {
			t.Fatalf("ExtractCredentials: ok=%v err=%v", ok, err)
		}
		if creds.AccessToken != "at-file" {
			t.Fatalf("AccessToken = %q, want the file's token", creds.AccessToken)
		}
	})

	t.Run("malformed keychain surfaces only when nothing else has a token", func(t *testing.T) {
		deps := Deps{
			Home:     "/home/u",
			Keychain: func(string) ([]byte, error) { return []byte(`{not json`), nil },
			ReadFile: func(string) ([]byte, error) { return nil, errors.New("missing") },
		}
		_, ok, err := ExtractCredentials(h, deps)
		if ok {
			t.Fatal("no source held a token, ok must be false")
		}
		if err == nil {
			t.Fatal("the malformed keychain blob should surface once no other source yields a token")
		}
	})
}
