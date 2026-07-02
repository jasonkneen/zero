package provideroauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRefreshClaudeCode(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)

	t.Run("happy path", func(t *testing.T) {
		var got map[string]string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("content-type = %q", ct)
			}
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "at-new",
				"refresh_token": "rt-new",
				"expires_in":    3600,
			})
		}))
		defer server.Close()

		token, err := RefreshClaudeCode(context.Background(), "rt-old", ClaudeCodeRefreshOptions{
			HTTPClient: server.Client(),
			Endpoint:   server.URL,
			Now:        func() time.Time { return now },
		})
		if err != nil {
			t.Fatalf("RefreshClaudeCode: %v", err)
		}
		if got["grant_type"] != "refresh_token" || got["refresh_token"] != "rt-old" || got["client_id"] != claudeCodeClientID {
			t.Fatalf("request body = %v", got)
		}
		if token.AccessToken != "at-new" || token.RefreshToken != "rt-new" {
			t.Fatalf("token = %+v", token)
		}
		if want := now.Add(time.Hour); !token.ExpiresAt.Equal(want) {
			t.Fatalf("ExpiresAt = %v, want %v", token.ExpiresAt, want)
		}
	})

	t.Run("rotation omitted keeps the old refresh token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-new"})
		}))
		defer server.Close()
		token, err := RefreshClaudeCode(context.Background(), "rt-old", ClaudeCodeRefreshOptions{
			HTTPClient: server.Client(), Endpoint: server.URL,
		})
		if err != nil {
			t.Fatalf("RefreshClaudeCode: %v", err)
		}
		if token.RefreshToken != "rt-old" {
			t.Fatalf("RefreshToken = %q, want the old token carried forward", token.RefreshToken)
		}
		if !token.ExpiresAt.IsZero() {
			t.Fatalf("ExpiresAt = %v, want zero when expires_in absent", token.ExpiresAt)
		}
	})

	t.Run("non-2xx surfaces the status only", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"invalid_grant","echo":"rt-old"}`, http.StatusUnauthorized)
		}))
		defer server.Close()
		_, err := RefreshClaudeCode(context.Background(), "rt-old", ClaudeCodeRefreshOptions{
			HTTPClient: server.Client(), Endpoint: server.URL,
		})
		if err == nil {
			t.Fatal("want error on 401")
		}
		if got := err.Error(); !strings.Contains(got, "401") || strings.Contains(got, "rt-old") {
			t.Fatalf("error = %q, want the status and no echoed request material", got)
		}
	})

	t.Run("empty refresh token is rejected before any request", func(t *testing.T) {
		if _, err := RefreshClaudeCode(context.Background(), "  ", ClaudeCodeRefreshOptions{}); err == nil {
			t.Fatal("want error for empty refresh token")
		}
	})

	t.Run("missing access token in response is an error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"refresh_token": "rt-new"})
		}))
		defer server.Close()
		if _, err := RefreshClaudeCode(context.Background(), "rt-old", ClaudeCodeRefreshOptions{
			HTTPClient: server.Client(), Endpoint: server.URL,
		}); err == nil {
			t.Fatal("want error when access_token absent")
		}
	})
}

func TestClaudeCodeLogin(t *testing.T) {
	now := time.Date(2026, 7, 2, 14, 0, 0, 0, time.UTC)
	// Deterministic entropy so verifier/state are stable across the fake flow.
	entropy := strings.NewReader(strings.Repeat("e", 512))

	t.Run("happy path", func(t *testing.T) {
		var got map[string]string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode exchange: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "at-login",
				"refresh_token": "rt-login",
				"expires_in":    28800,
				"account":       map[string]any{"email_address": "user@example.com"},
			})
		}))
		defer server.Close()

		var out strings.Builder
		// The authorize page returns "code#state" for pasting; recover the state
		// zero generated from the printed URL to build a valid paste.
		var pasted strings.Builder
		reader, writer := io.Pipe()
		opts := ClaudeCodeLoginOptions{
			HTTPClient: server.Client(),
			Endpoint:   server.URL,
			Out:        &out,
			In:         reader,
			Now:        func() time.Time { return now },
			randReader: entropy,
			OpenBrowser: func(authURL string) error {
				parsed, err := url.Parse(authURL)
				if err != nil {
					t.Errorf("authorize URL: %v", err)
					return nil
				}
				query := parsed.Query()
				if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" {
					t.Errorf("authorize URL missing PKCE: %s", authURL)
				}
				if query.Get("client_id") != claudeCodeClientID {
					t.Errorf("client_id = %q", query.Get("client_id"))
				}
				pasted.WriteString("the-code#" + query.Get("state") + "\n")
				go func() { _, _ = writer.Write([]byte(pasted.String())) }()
				return nil
			},
		}
		token, err := ClaudeCodeLogin(context.Background(), opts)
		if err != nil {
			t.Fatalf("ClaudeCodeLogin: %v", err)
		}
		if token.AccessToken != "at-login" || token.RefreshToken != "rt-login" || token.Account != "user@example.com" {
			t.Fatalf("token = %+v", token)
		}
		if want := now.Add(8 * time.Hour); !token.ExpiresAt.Equal(want) {
			t.Fatalf("ExpiresAt = %v, want %v", token.ExpiresAt, want)
		}
		if got["grant_type"] != "authorization_code" || got["code"] != "the-code" || got["code_verifier"] == "" {
			t.Fatalf("exchange body = %v", got)
		}
		if got["redirect_uri"] != claudeCodeRedirectURI {
			t.Fatalf("redirect_uri = %q", got["redirect_uri"])
		}
	})

	t.Run("state mismatch is rejected before any exchange", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Error("exchange must not run on state mismatch")
		}))
		defer server.Close()
		_, err := ClaudeCodeLogin(context.Background(), ClaudeCodeLoginOptions{
			HTTPClient: server.Client(),
			Endpoint:   server.URL,
			In:         strings.NewReader("the-code#wrong-state\n"),
			randReader: strings.NewReader(strings.Repeat("x", 512)),
		})
		if err == nil || !strings.Contains(err.Error(), "state mismatch") {
			t.Fatalf("err = %v, want state mismatch", err)
		}
	})

	t.Run("empty paste is rejected", func(t *testing.T) {
		_, err := ClaudeCodeLogin(context.Background(), ClaudeCodeLoginOptions{
			In:         strings.NewReader("\n"),
			randReader: strings.NewReader(strings.Repeat("x", 512)),
		})
		if err == nil || !strings.Contains(err.Error(), "empty code") {
			t.Fatalf("err = %v, want empty code", err)
		}
	})
}
