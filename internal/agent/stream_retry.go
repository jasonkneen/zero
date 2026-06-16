package agent

import (
	"context"
	"strings"
	"time"
)

// maxNetworkRetries is how many times a transient network failure that produced
// NO output is transparently retried before the turn gives up. Three total
// attempts (initial + 2 retries) covers a brief blip without stalling for long.
const maxNetworkRetries = 2

// transientNetworkSignals are substrings of error messages that indicate a
// connection-level failure worth retrying — the request never reached the model
// or got no response, so re-sending the SAME request is safe and may succeed.
// Matched case-insensitively against the provider's error string.
var transientNetworkSignals = []string{
	"tls handshake timeout",
	"i/o timeout",
	"dial tcp",
	"connection reset",
	"connection refused",
	"broken pipe",
	"network is unreachable",
	"no route to host",
	"unexpected eof",
	"server misbehaving",                   // transient resolver failure
	"temporary failure in name resolution", // transient DNS
}

// isTransientNetworkError reports whether a provider error string looks like a
// retryable connection-level failure. A user cancellation, a context deadline,
// or any HTTP/content error (auth, rate limit, context-length) returns false so
// only genuine network blips are retried.
func isTransientNetworkError(reason string) bool {
	lower := strings.ToLower(reason)
	// Never retry a cancellation or a deadline — those are intentional/terminal.
	if strings.Contains(lower, "context canceled") || strings.Contains(lower, "context deadline exceeded") {
		return false
	}
	// Never retry an explicit HTTP status error (auth, rate limit, bad request,
	// context length, etc.) — re-sending won't help.
	if strings.Contains(lower, "status code") || strings.Contains(lower, "status:") ||
		strings.Contains(lower, "http 4") || strings.Contains(lower, "http 5") {
		return false
	}
	for _, signal := range transientNetworkSignals {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	return false
}

// networkRetryBackoff is the wait before the (0-based) attempt-th retry. It is a
// var so tests can zero it; production uses defaultNetworkRetryBackoff.
var networkRetryBackoff = defaultNetworkRetryBackoff

// defaultNetworkRetryBackoff waits exponentially — 1s, 2s, 4s — capped at 8s.
func defaultNetworkRetryBackoff(attempt int) time.Duration {
	d := time.Second << attempt
	if d > 8*time.Second {
		d = 8 * time.Second
	}
	return d
}

// sleepWithContext waits for d, returning false if the context is cancelled
// first (so a backoff never outlives a cancelled run).
func sleepWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
