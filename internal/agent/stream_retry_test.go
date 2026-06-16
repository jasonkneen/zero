package agent

import (
	"context"
	"testing"
	"time"

	"github.com/Gitlawb/zero/internal/tools"
	"github.com/Gitlawb/zero/internal/zeroruntime"
)

func TestIsTransientNetworkError(t *testing.T) {
	transient := []string{
		`provider stream error: Post "https://ollama.com/v1/chat/completions": net/http: TLS handshake timeout`,
		"dial tcp 1.2.3.4:443: i/o timeout",
		"read tcp: connection reset by peer",
		"connection refused",
		"unexpected EOF",
		"network is unreachable",
	}
	for _, reason := range transient {
		if !isTransientNetworkError(reason) {
			t.Errorf("isTransientNetworkError(%q) = false, want true", reason)
		}
	}
	terminal := []string{
		"context canceled",
		"context deadline exceeded",
		"provider error: status code 401: invalid api key",
		"HTTP 429: rate limit exceeded",
		"context length exceeded: 200000 tokens",
		"invalid request: model not found",
	}
	for _, reason := range terminal {
		if isTransientNetworkError(reason) {
			t.Errorf("isTransientNetworkError(%q) = true, want false", reason)
		}
	}
}

func TestDefaultNetworkRetryBackoff(t *testing.T) {
	for attempt, want := range map[int]time.Duration{0: time.Second, 1: 2 * time.Second, 2: 4 * time.Second, 5: 8 * time.Second} {
		if got := defaultNetworkRetryBackoff(attempt); got != want {
			t.Errorf("defaultNetworkRetryBackoff(%d) = %s, want %s", attempt, got, want)
		}
	}
}

// withInstantBackoff zeros the retry backoff for the duration of a test.
func withInstantBackoff(t *testing.T) {
	t.Helper()
	prev := networkRetryBackoff
	networkRetryBackoff = func(int) time.Duration { return 0 }
	t.Cleanup(func() { networkRetryBackoff = prev })
}

func TestRunRetriesTransientNetworkFailure(t *testing.T) {
	withInstantBackoff(t)
	provider := &mockProvider{
		turns: [][]zeroruntime.StreamEvent{
			{{Type: zeroruntime.StreamEventError, Error: "provider stream error: net/http: TLS handshake timeout"}},
			{{Type: zeroruntime.StreamEventText, Content: "built it"}, {Type: zeroruntime.StreamEventDone}},
		},
	}
	var retries []int
	result, err := Run(context.Background(), "build", provider, Options{
		Registry:       tools.NewRegistry(),
		OnNetworkRetry: func(attempt int, _ string) { retries = append(retries, attempt) },
	})
	if err != nil {
		t.Fatalf("expected the transient failure to be retried to success, got %v", err)
	}
	if result.FinalAnswer != "built it" {
		t.Fatalf("final answer = %q, want %q", result.FinalAnswer, "built it")
	}
	if len(provider.requests) != 2 {
		t.Fatalf("expected 2 provider requests (initial + 1 retry), got %d", len(provider.requests))
	}
	if len(retries) != 1 || retries[0] != 1 {
		t.Fatalf("expected one OnNetworkRetry(attempt=1), got %#v", retries)
	}
}

func TestRunDoesNotRetryTerminalError(t *testing.T) {
	withInstantBackoff(t)
	provider := &mockProvider{
		turns: [][]zeroruntime.StreamEvent{
			{{Type: zeroruntime.StreamEventError, Error: "provider error: status code 401: invalid api key"}},
		},
	}
	_, err := Run(context.Background(), "build", provider, Options{Registry: tools.NewRegistry()})
	if err == nil {
		t.Fatal("expected a terminal (auth) error to surface, not be retried")
	}
	if len(provider.requests) != 1 {
		t.Fatalf("a terminal error must not be retried, got %d requests", len(provider.requests))
	}
}

func TestRunDoesNotRetryAfterPartialOutput(t *testing.T) {
	withInstantBackoff(t)
	// Text was already streamed before the error → retrying would duplicate it.
	provider := &mockProvider{
		turns: [][]zeroruntime.StreamEvent{
			{
				{Type: zeroruntime.StreamEventText, Content: "partial answer"},
				{Type: zeroruntime.StreamEventError, Error: "net/http: TLS handshake timeout"},
			},
		},
	}
	_, err := Run(context.Background(), "build", provider, Options{Registry: tools.NewRegistry()})
	if err == nil {
		t.Fatal("expected the mid-stream error to surface (no retry after partial output)")
	}
	if len(provider.requests) != 1 {
		t.Fatalf("must not retry once output was produced, got %d requests", len(provider.requests))
	}
}
