package specialist

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMarkdownHarnessAndProviderHappyPath(t *testing.T) {
	manifest, err := ParseMarkdown(`---
name: codex-worker
description: Runs on the Codex CLI
harness: codex
provider: work-openai
harnessArgs: [--full-auto]
---
Do the task.`)
	if err != nil {
		t.Fatalf("ParseMarkdown returned error: %v", err)
	}
	if manifest.Metadata.Harness != "codex" {
		t.Fatalf("Harness = %q, want codex", manifest.Metadata.Harness)
	}
	if manifest.Metadata.Provider != "work-openai" {
		t.Fatalf("Provider = %q, want work-openai", manifest.Metadata.Provider)
	}
	if len(manifest.Metadata.HarnessArgs) != 1 || manifest.Metadata.HarnessArgs[0] != "--full-auto" {
		t.Fatalf("HarnessArgs = %#v, want [--full-auto]", manifest.Metadata.HarnessArgs)
	}
}

func TestParseMarkdownHarnessIsNormalizedToLowercase(t *testing.T) {
	manifest, err := ParseMarkdown(`---
name: claude-worker
description: Runs on Claude Code
harness: CLAUDE
---
Do the task.`)
	if err != nil {
		t.Fatalf("ParseMarkdown returned error: %v", err)
	}
	if manifest.Metadata.Harness != "claude" {
		t.Fatalf("Harness = %q, want normalized claude", manifest.Metadata.Harness)
	}
}

func TestParseMarkdownRejectsUnknownHarness(t *testing.T) {
	_, err := ParseMarkdown(`---
name: mystery-worker
description: Runs on a made-up CLI
harness: not-a-real-cli
---
Do the task.`)
	if err == nil {
		t.Fatal("expected an error for an unknown harness")
	}
	if !strings.Contains(err.Error(), `unknown harness "not-a-real-cli"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	// The error must be actionable: list at least one real catalog id.
	if !strings.Contains(err.Error(), "codex") || !strings.Contains(err.Error(), "claude") {
		t.Fatalf("expected error to list available harnesses, got: %v", err)
	}
}

func TestParseMarkdownRejectsScalarHarnessArgs(t *testing.T) {
	_, err := ParseMarkdown(`---
name: bad-args
description: Bad harnessArgs shape
harness: codex
harnessArgs: --full-auto
---
Do the task.`)
	if err == nil || !strings.Contains(err.Error(), "harnessArgs must be an array") {
		t.Fatalf("expected harnessArgs array error, got %v", err)
	}
}

func TestMergeExtendsPropagatesHarnessProviderAndHarnessArgs(t *testing.T) {
	tempDir := t.TempDir()
	writeManifest(t, filepath.Join(tempDir, "base.md"), `---
name: base
description: Base specialist
harness: codex
provider: work-openai
harnessArgs: [--full-auto]
---
Base prompt.`)
	writeManifest(t, filepath.Join(tempDir, "child.md"), `---
name: child
extends: base
description: Child specialist
---
Child prompt.`)

	result, err := Load(LoadOptions{Paths: Paths{UserDir: tempDir}})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	child, ok := Find(result, "child")
	if !ok {
		t.Fatal("child manifest not found")
	}
	if child.Metadata.Harness != "codex" {
		t.Fatalf("child Harness = %q, want inherited codex", child.Metadata.Harness)
	}
	if child.Metadata.Provider != "work-openai" {
		t.Fatalf("child Provider = %q, want inherited work-openai", child.Metadata.Provider)
	}
	if len(child.Metadata.HarnessArgs) != 1 || child.Metadata.HarnessArgs[0] != "--full-auto" {
		t.Fatalf("child HarnessArgs = %#v, want inherited [--full-auto]", child.Metadata.HarnessArgs)
	}
}

func TestMergeExtendsChildHarnessOverridesBase(t *testing.T) {
	tempDir := t.TempDir()
	writeManifest(t, filepath.Join(tempDir, "base.md"), `---
name: base
description: Base specialist
harness: codex
---
Base prompt.`)
	writeManifest(t, filepath.Join(tempDir, "child.md"), `---
name: child
extends: base
description: Child specialist
harness: claude
---
Child prompt.`)

	result, err := Load(LoadOptions{Paths: Paths{UserDir: tempDir}})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	child, ok := Find(result, "child")
	if !ok {
		t.Fatal("child manifest not found")
	}
	if child.Metadata.Harness != "claude" {
		t.Fatalf("child Harness = %q, want override claude", child.Metadata.Harness)
	}
}
