package specialist

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Gitlawb/zero/internal/streamjson"
)

func TestChildProviderEnvAppendsOverrideAndKeepsInheritedVars(t *testing.T) {
	t.Setenv("ZERO_SPECIALIST_ENV_PROBE", "keep-me")

	env := childProviderEnv("work-openai")
	if env == nil {
		t.Fatal("expected a non-nil env when a provider is pinned")
	}
	if last := env[len(env)-1]; last != "ZERO_PROVIDER=work-openai" {
		t.Fatalf("last env entry = %q, want ZERO_PROVIDER=work-openai", last)
	}
	found := false
	for _, kv := range env {
		if kv == "ZERO_SPECIALIST_ENV_PROBE=keep-me" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("childProviderEnv dropped an inherited environment variable")
	}
}

func TestChildProviderEnvNilWhenNoProviderPinned(t *testing.T) {
	if env := childProviderEnv(""); env != nil {
		t.Fatalf("childProviderEnv(\"\") = %#v, want nil (inherit unchanged)", env)
	}
	if env := childProviderEnv("   "); env != nil {
		t.Fatalf("childProviderEnv(whitespace) = %#v, want nil (inherit unchanged)", env)
	}
}

func requireSh(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires /bin/sh")
	}
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("/bin/sh not available")
	}
}

// TestRunChildWithDecoderAppliesEnvAndWorkingDir exercises the shared
// process-management skeleton (runChildWithDecoder) end-to-end against a real
// subprocess, proving both the env-override construction (childProviderEnv)
// and the harness working-directory wiring reach the actual child process —
// not just the struct fields.
func TestRunChildWithDecoderAppliesEnvAndWorkingDir(t *testing.T) {
	requireSh(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write marker file: %v", err)
	}
	env := childProviderEnv("work-openai")
	script := `echo "provider=$ZERO_PROVIDER"; test -f marker.txt && echo "cwd-ok"`
	run, err := runChildWithDecoder(context.Background(), "/bin/sh", []string{"-c", script}, env, dir, &textDecoder{}, nil)
	if err != nil {
		t.Fatalf("runChildWithDecoder returned error: %v", err)
	}
	if run.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0 (stderr: %s)", run.ExitCode, run.Stderr)
	}
	final := finalText(run.Events)
	if !strings.Contains(final, "provider=work-openai") {
		t.Fatalf("child output %q missing the ZERO_PROVIDER override", final)
	}
	if !strings.Contains(final, "cwd-ok") {
		t.Fatalf("child output %q missing the working-directory marker", final)
	}
}

// TestRunChildWithDecoderCapturesNonZeroExitAndStderr is the exit-code error
// path shared by both runChildProcess (self-exec zero) and the harness path:
// a failing child's exit code and stderr must both survive into
// ChildRunResult regardless of which decoder is driving it.
func TestRunChildWithDecoderCapturesNonZeroExitAndStderr(t *testing.T) {
	requireSh(t)
	run, err := runChildWithDecoder(context.Background(), "/bin/sh", []string{"-c", "echo boom 1>&2; exit 7"}, nil, "", &textDecoder{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", run.ExitCode)
	}
	if !strings.Contains(run.Stderr, "boom") {
		t.Fatalf("Stderr = %q, want to contain boom", run.Stderr)
	}
	// textDecoder.finish suppresses the synthetic final on a non-zero exit (see
	// harness_decode_test.go), so BuildFinalResult falls back to stderr/errors.
	if text := finalText(run.Events); text != "" {
		t.Fatalf("expected no synthesized final text on non-zero exit, got %q", text)
	}
}

// TestRunChildWithDecoderProgressReceivesEveryEvent proves the harness
// decoding path feeds the SAME progress callback contract as the self-exec
// zero path (runChildProcess): every emitted event, including decoder-side
// synthesized ones like EventFinal, reaches progress in order.
func TestRunChildWithDecoderProgressReceivesEveryEvent(t *testing.T) {
	requireSh(t)
	var seen []streamjson.EventType
	_, err := runChildWithDecoder(context.Background(), "/bin/sh", []string{"-c", "echo hello"}, nil, "", &textDecoder{}, func(event streamjson.Event) {
		seen = append(seen, event.Type)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(seen) != 2 || seen[0] != streamjson.EventText || seen[1] != streamjson.EventFinal {
		t.Fatalf("progress saw %#v, want [text final]", seen)
	}
}

func finalText(events []streamjson.Event) string {
	for _, event := range events {
		if event.Type == streamjson.EventFinal {
			return event.Text
		}
	}
	return ""
}
