package swarm

import "testing"

// TestBuildSpecThreadsHarnessAndProvider guards the per-member harness/provider
// override: a Definition pinning an external CLI and/or provider profile must
// reach the launched MemberSpec unchanged, so different swarm members can run
// on different backends. buildSpec touches no Swarm state, so a zero-value
// Swarm is enough to call it directly.
func TestBuildSpecThreadsHarnessAndProvider(t *testing.T) {
	pol := Policy{Model: "orchestrator-model"}
	def := Definition{
		AgentType: "codex-worker",
		Model:     modelInherit,
		Harness:   "codex",
		Provider:  "work-openai",
		SystemPrompt: func(ctx PromptContext) string {
			return "task: " + ctx.Task
		},
	}
	s := &Swarm{}
	spec := s.buildSpec(pol, "m1", "m1", "team", def, "do it", "/workspace")
	if spec.Harness != "codex" {
		t.Fatalf("Harness = %q, want codex", spec.Harness)
	}
	if spec.Provider != "work-openai" {
		t.Fatalf("Provider = %q, want work-openai", spec.Provider)
	}
	if spec.SystemPrompt != "task: do it" {
		t.Fatalf("SystemPrompt = %q, unexpected", spec.SystemPrompt)
	}
}

// TestBuildSpecOmitsHarnessAndProviderByDefault ensures a definition that
// never sets these fields produces a MemberSpec that runs self-exec zero with
// no provider pin — the pre-existing behavior for teammate/subagent.
func TestBuildSpecOmitsHarnessAndProviderByDefault(t *testing.T) {
	pol := Policy{Model: "orchestrator-model"}
	def := Definition{AgentType: "subagent", Model: modelInherit}
	s := &Swarm{}
	spec := s.buildSpec(pol, "m1", "m1", "team", def, "do it", "/workspace")
	if spec.Harness != "" {
		t.Fatalf("Harness = %q, want empty", spec.Harness)
	}
	if spec.Provider != "" {
		t.Fatalf("Provider = %q, want empty", spec.Provider)
	}
}

// TestSpecialistManifestForMemberThreadsHarnessAndProvider guards the launcher
// side: specialistManifestForMember builds the inline specialist.Manifest that
// executor.Run actually runs from, so MemberSpec.Harness/Provider must land on
// Manifest.Metadata for the harness-execution branch in
// internal/specialist/harness.go to ever trigger for a swarm member.
func TestSpecialistManifestForMemberThreadsHarnessAndProvider(t *testing.T) {
	spec := MemberSpec{
		AgentType:    "codex-worker",
		Team:         "probe",
		Task:         "count files",
		SystemPrompt: "You are a codex-worker.",
		Harness:      "codex",
		Provider:     "work-openai",
	}
	manifest := specialistManifestForMember(spec)
	if manifest.Metadata.Harness != "codex" {
		t.Fatalf("Metadata.Harness = %q, want codex", manifest.Metadata.Harness)
	}
	if manifest.Metadata.Provider != "work-openai" {
		t.Fatalf("Metadata.Provider = %q, want work-openai", manifest.Metadata.Provider)
	}
	if manifest.SystemPrompt != spec.SystemPrompt {
		t.Fatalf("SystemPrompt = %q, want %q", manifest.SystemPrompt, spec.SystemPrompt)
	}
}
