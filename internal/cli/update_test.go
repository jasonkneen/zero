package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Gitlawb/zero/internal/updatecheck"
)

func TestRunUpdateRequiresCheck(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	checkCalled := false

	exitCode := runWithDeps([]string{"update"}, &stdout, &stderr, appDeps{
		checkUpdate: func(context.Context, updatecheck.Options) (updatecheck.Result, error) {
			checkCalled = true
			return updatecheck.Result{}, nil
		},
	})

	if exitCode != exitUsage {
		t.Fatalf("expected exit code %d, got %d", exitUsage, exitCode)
	}
	if checkCalled {
		t.Fatal("update check should not run without --check")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "Only `zero update --check` is available") {
		t.Fatalf("expected check-only error, got %q", got)
	}
}

func TestRunUpdateRejectsInvalidArguments(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unknown flag",
			args: []string{"update", "--unknown"},
			want: `unknown update flag "--unknown"`,
		},
		{
			name: "unexpected positional argument",
			args: []string{"update", "foo"},
			want: `unexpected update argument "foo"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			exitCode := runWithDeps(test.args, &stdout, &stderr, appDeps{})

			if exitCode != exitUsage {
				t.Fatalf("expected exit code %d, got %d", exitUsage, exitCode)
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected empty stdout, got %q", stdout.String())
			}
			if got := stderr.String(); !strings.Contains(got, test.want) {
				t.Fatalf("expected usage error %q, got %q", test.want, got)
			}
		})
	}
}

func TestRunUpdateCheckFormatsHumanOutput(t *testing.T) {
	withVersion(t, "0.1.0")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var checkOptions updatecheck.Options

	exitCode := runWithDeps([]string{"update", "--check"}, &stdout, &stderr, appDeps{
		checkUpdate: func(_ context.Context, options updatecheck.Options) (updatecheck.Result, error) {
			checkOptions = options
			return updatecheck.Result{
				CurrentVersion:  "0.1.0",
				LatestVersion:   "0.2.0",
				ReleaseURL:      "https://github.com/Gitlawb/zero/releases/tag/v0.2.0",
				TagName:         "v0.2.0",
				UpdateAvailable: true,
			}, nil
		},
	})

	if exitCode != exitSuccess {
		t.Fatalf("expected exit code %d, got %d: %s", exitSuccess, exitCode, stderr.String())
	}
	if checkOptions.CurrentVersion != "0.1.0" {
		t.Fatalf("CurrentVersion = %q, want injected CLI version", checkOptions.CurrentVersion)
	}
	if got := stdout.String(); !strings.Contains(got, "Update available: 0.1.0 -> 0.2.0") {
		t.Fatalf("expected update output, got %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunUpdateCheckPrintsJSON(t *testing.T) {
	withVersion(t, "0.1.0")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithDeps([]string{"update", "--check", "--json"}, &stdout, &stderr, appDeps{
		checkUpdate: func(context.Context, updatecheck.Options) (updatecheck.Result, error) {
			return updatecheck.Result{
				CurrentVersion:  "0.1.0",
				LatestVersion:   "0.1.0",
				ReleaseURL:      "https://github.com/Gitlawb/zero/releases/tag/v0.1.0",
				TagName:         "v0.1.0",
				UpdateAvailable: false,
			}, nil
		},
	})

	if exitCode != exitSuccess {
		t.Fatalf("expected exit code %d, got %d: %s", exitSuccess, exitCode, stderr.String())
	}
	var payload updatecheck.Result
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse JSON output %q: %v", stdout.String(), err)
	}
	if payload.UpdateAvailable {
		t.Fatalf("expected updateAvailable=false, got %#v", payload)
	}
	if payload.CurrentVersion != "0.1.0" || payload.LatestVersion != "0.1.0" {
		t.Fatalf("unexpected JSON payload: %#v", payload)
	}
}

func TestRunUpdateCheckReportsErrors(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithDeps([]string{"update", "--check"}, &stdout, &stderr, appDeps{
		checkUpdate: func(context.Context, updatecheck.Options) (updatecheck.Result, error) {
			return updatecheck.Result{}, errors.New("network down")
		},
	})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "Could not check for updates: network down") {
		t.Fatalf("expected update error, got %q", got)
	}
}

func TestRunUpdateHelp(t *testing.T) {
	for _, args := range [][]string{
		{"update", "--help"},
		{"update", "-h"},
		{"update", "help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			exitCode := runWithDeps(args, &stdout, &stderr, appDeps{})

			if exitCode != exitSuccess {
				t.Fatalf("expected exit code %d, got %d: %s", exitSuccess, exitCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), "zero update --check") {
				t.Fatalf("expected update help, got %q", stdout.String())
			}
		})
	}
}

func withVersion(t *testing.T, value string) {
	t.Helper()
	previous := version
	version = value
	t.Cleanup(func() {
		version = previous
	})
}
