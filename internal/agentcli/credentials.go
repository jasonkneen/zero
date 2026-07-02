package agentcli

import (
	"encoding/json"
	"fmt"
	"time"
)

// Credentials are a harness's stored local credentials, normalized to the
// shape zero's providers need to reuse them. Fields that don't apply to a
// given harness are left zero-valued.
type Credentials struct {
	APIKey       string    // plain API key, when the CLI stores one
	AccessToken  string    // OAuth access token
	RefreshToken string    // OAuth refresh token
	AccountID    string    // provider account id (e.g. codex's chatgpt-account-id)
	ExpiresAt    time.Time // zero when unknown
}

// ExtractCredentials reads and parses h's locally stored credentials.
//
// It returns (creds, true, nil) on success, (Credentials{}, false, nil) when h
// has no reusable credential source (extraction unsupported for this harness)
// or nothing is currently stored, and a non-nil error only when a credential
// store was found but could not be parsed (malformed data — a real failure
// worth surfacing, unlike "not logged in").
func ExtractCredentials(h Harness, deps Deps) (Credentials, bool, error) {
	deps = resolveDeps(deps)
	switch h.ID {
	case "codex":
		return extractCodexCredentials(h, deps)
	case "claude":
		return extractClaudeCredentials(h, deps)
	default:
		return Credentials{}, false, nil
	}
}

// codexAuthFile mirrors the relevant subset of ~/.codex/auth.json.
type codexAuthFile struct {
	OpenAIAPIKey string `json:"OPENAI_API_KEY"`
	Tokens       *struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		AccountID    string `json:"account_id"`
		IDToken      string `json:"id_token"`
	} `json:"tokens"`
}

func extractCodexCredentials(h Harness, deps Deps) (Credentials, bool, error) {
	if len(h.CredFiles) == 0 {
		return Credentials{}, false, nil
	}
	path := homeJoin(deps.Home, h.CredFiles[0])
	raw, err := deps.ReadFile(path)
	if err != nil {
		return Credentials{}, false, nil
	}
	var parsed codexAuthFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Credentials{}, false, fmt.Errorf("agentcli: parse %s: %w", path, err)
	}
	creds := Credentials{APIKey: parsed.OpenAIAPIKey}
	if parsed.Tokens != nil {
		creds.AccessToken = parsed.Tokens.AccessToken
		creds.RefreshToken = parsed.Tokens.RefreshToken
		creds.AccountID = parsed.Tokens.AccountID
	}
	if creds.APIKey == "" && creds.AccessToken == "" && creds.RefreshToken == "" {
		return Credentials{}, false, nil
	}
	return creds, true, nil
}

// claudeCredentialsFile mirrors the relevant subset of the JSON blob Claude
// Code stores both in the macOS keychain (service "Claude Code-credentials")
// and, as a fallback, at ~/.claude/.credentials.json.
type claudeCredentialsFile struct {
	ClaudeAiOauth struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresAt    int64  `json:"expiresAt"` // unix milliseconds
	} `json:"claudeAiOauth"`
}

func extractClaudeCredentials(h Harness, deps Deps) (Credentials, bool, error) {
	// Try every source in Claude Code's own storage order and take the first
	// one holding an actual OAuth token. A source merely EXISTING is not enough
	// to stop the search: the keychain entry can be present while carrying only
	// MCP-server tokens (no claudeAiOauth key at all) with the real login living
	// in the credentials file — stopping at the first non-empty blob would
	// permanently shadow that login.
	var parseErr error
	for _, source := range claudeCredentialSources(h, deps) {
		raw, err := source.read()
		if err != nil || len(raw) == 0 {
			continue
		}
		var parsed claudeCredentialsFile
		if err := json.Unmarshal(raw, &parsed); err != nil {
			parseErr = fmt.Errorf("agentcli: parse claude credentials (%s): %w", source.name, err)
			continue
		}
		if parsed.ClaudeAiOauth.AccessToken == "" && parsed.ClaudeAiOauth.RefreshToken == "" {
			continue
		}
		creds := Credentials{
			AccessToken:  parsed.ClaudeAiOauth.AccessToken,
			RefreshToken: parsed.ClaudeAiOauth.RefreshToken,
		}
		if parsed.ClaudeAiOauth.ExpiresAt > 0 {
			creds.ExpiresAt = time.UnixMilli(parsed.ClaudeAiOauth.ExpiresAt)
		}
		return creds, true, nil
	}
	// A malformed source is only worth surfacing when no other source yielded a
	// token — a usable login elsewhere makes the malformed blob moot.
	if parseErr != nil {
		return Credentials{}, false, parseErr
	}
	return Credentials{}, false, nil
}

type claudeCredentialSource struct {
	name string
	read func() ([]byte, error)
}

// claudeCredentialSources lists the places Claude Code stores its login, in
// the order Claude Code itself consults them: keychain first, then the
// on-disk credentials file(s).
func claudeCredentialSources(h Harness, deps Deps) []claudeCredentialSource {
	sources := []claudeCredentialSource{}
	if h.KeychainService != "" && deps.Keychain != nil {
		sources = append(sources, claudeCredentialSource{
			name: "keychain",
			read: func() ([]byte, error) { return deps.Keychain(h.KeychainService) },
		})
	}
	for _, file := range h.CredFiles {
		path := homeJoin(deps.Home, file)
		sources = append(sources, claudeCredentialSource{
			name: path,
			read: func() ([]byte, error) { return deps.ReadFile(path) },
		})
	}
	return sources
}
