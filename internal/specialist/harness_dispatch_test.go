package specialist

import (
	"context"
	"strings"
	"testing"
)

// harnessManifest builds a minimal valid manifest pinned to the given
// agentcli harness id, for exercising runFresh's harness-dispatch branch
// without going through the markdown loader.
func harnessManifest(t *testing.T, harness string) *Manifest {
	t.Helper()
	manifest, err := ParseMarkdown(`---
name: harness-worker
description: Runs on an external harness
harness: ` + harness + `
---
Do the task.`)
	if err != nil {
		t.Fatalf("ParseMarkdown: %v", err)
	}
	return &manifest
}

// TestRunFreshHarnessRejectsRunInBackground locks in that a harness-backed
// specialist (Metadata.Harness set) refuses RunInBackground before ever
// touching runHarness/agentcli.DetectOne — the background-launch path assumes
// a self-exec zero child it can register with background.Manager, which a
// foreign harness CLI is not.
func TestRunFreshHarnessRejectsRunInBackground(t *testing.T) {
	executor := Executor{}
	_, err := executor.Run(context.Background(), TaskParameters{
		Prompt:          "do the thing",
		RunInBackground: true,
		Manifest:        harnessManifest(t, "codex"),
	}, TaskRunOptions{})
	if err == nil {
		t.Fatal("expected an error for a harness specialist run in background")
	}
	if !strings.Contains(err.Error(), "cannot run in background") {
		t.Fatalf("error = %q, want mention of \"cannot run in background\"", err.Error())
	}
	if !strings.Contains(err.Error(), "codex") {
		t.Fatalf("error = %q, want it to name the harness", err.Error())
	}
}

// TestRunFreshDispatchesHarnessNotInstalled locks in that runFresh routes a
// harness-pinned manifest to runHarness (rather than the self-exec BuildArgs
// path), by observing runHarness's "not installed" error when the harness
// binary cannot be found on PATH. PATH is redirected to an empty directory so
// this is deterministic regardless of what happens to be installed on the
// host running the test.
func TestRunFreshDispatchesHarnessNotInstalled(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	executor := Executor{}
	_, err := executor.Run(context.Background(), TaskParameters{
		Prompt:   "do the thing",
		Manifest: harnessManifest(t, "codex"),
	}, TaskRunOptions{})
	if err == nil {
		t.Fatal("expected an error when the harness binary is not on PATH")
	}
	if !strings.Contains(err.Error(), "codex") || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("error = %q, want a \"codex ... not installed\" message", err.Error())
	}
}
