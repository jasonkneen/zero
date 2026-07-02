package modelregistry

import "testing"

func TestLookupModeKnown(t *testing.T) {
	for _, name := range []string{"smart", "deep", "fast", "large", "precise"} {
		mode, ok := LookupMode(name)
		if !ok {
			t.Fatalf("LookupMode(%q) = _, false; want a registered mode", name)
		}
		if mode.Name != name {
			t.Fatalf("LookupMode(%q).Name = %q; want %q", name, mode.Name, name)
		}
		if mode.Description == "" {
			t.Fatalf("LookupMode(%q).Description is empty", name)
		}
		if mode.Model == "" {
			t.Fatalf("LookupMode(%q).Model is empty", name)
		}
	}
}

func TestLookupModeIsCaseInsensitiveAndTrimmed(t *testing.T) {
	mode, ok := LookupMode("  DEEP ")
	if !ok {
		t.Fatal("LookupMode(\"  DEEP \") = _, false; want match")
	}
	if mode.Name != "deep" {
		t.Fatalf("LookupMode normalized name = %q; want deep", mode.Name)
	}
}

func TestLookupModeOnlyBash(t *testing.T) {
	mode, ok := LookupMode("onlybash")
	if !ok {
		t.Fatal("LookupMode(\"onlybash\") = _, false; want a registered mode")
	}
	if mode.Model != "" || mode.Effort != "" || mode.MaxTurns != 0 {
		t.Fatalf("onlybash mode should leave model/effort/turns untouched, got Model=%q Effort=%q MaxTurns=%d", mode.Model, mode.Effort, mode.MaxTurns)
	}
	wantEnabled := []string{"bash", "skill"}
	if len(mode.EnabledTools) != len(wantEnabled) {
		t.Fatalf("onlybash EnabledTools = %v; want %v", mode.EnabledTools, wantEnabled)
	}
	for index, name := range wantEnabled {
		if mode.EnabledTools[index] != name {
			t.Fatalf("onlybash EnabledTools = %v; want %v", mode.EnabledTools, wantEnabled)
		}
	}
	wantDisabled := []string{"tool_search"}
	if len(mode.DisabledTools) != len(wantDisabled) || mode.DisabledTools[0] != wantDisabled[0] {
		t.Fatalf("onlybash DisabledTools = %v; want %v", mode.DisabledTools, wantDisabled)
	}
}

func TestLookupModeUnknown(t *testing.T) {
	for _, name := range []string{"", "   ", "turbo", "genius"} {
		if _, ok := LookupMode(name); ok {
			t.Fatalf("LookupMode(%q) = _, true; want false", name)
		}
	}
}

func TestModesReturnsIndependentCopies(t *testing.T) {
	modes := Modes()
	if len(modes) == 0 {
		t.Fatal("Modes() returned no presets")
	}
	modes[0].Name = "mutated"
	if again := Modes(); again[0].Name == "mutated" {
		t.Fatal("Modes() should return defensive copies, not shared state")
	}
}

func TestModeNamesMatchCatalogOrder(t *testing.T) {
	names := ModeNames()
	modes := Modes()
	if len(names) != len(modes) {
		t.Fatalf("ModeNames length %d != Modes length %d", len(names), len(modes))
	}
	for index := range names {
		if names[index] != modes[index].Name {
			t.Fatalf("ModeNames[%d] = %q; want %q", index, names[index], modes[index].Name)
		}
	}
}

func TestEveryModeResolvesToRealRegistryModel(t *testing.T) {
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry: %v", err)
	}
	for _, mode := range Modes() {
		if mode.Model == "" {
			// Zero value means "leave the configured model untouched" (e.g.
			// onlybash, which only narrows the tool surface) — nothing to resolve.
			continue
		}
		entry, ok := registry.Resolve(mode.Model)
		if !ok {
			t.Fatalf("mode %q references model %q that does not resolve in the registry", mode.Name, mode.Model)
		}
		if mode.Effort != "" {
			if !ValidReasoningEffort(mode.Effort) {
				t.Fatalf("mode %q has invalid effort %q", mode.Name, mode.Effort)
			}
			// The effort the mode requests should be honored by the resolved
			// model (so the preset never silently downgrades on apply).
			if effective := EffectiveReasoningEffort(entry, mode.Effort); effective != mode.Effort {
				t.Fatalf("mode %q effort %q is not supported by %s (effective %q)", mode.Name, mode.Effort, entry.ID, effective)
			}
		}
		if mode.MaxTurns < 0 {
			t.Fatalf("mode %q has negative MaxTurns %d", mode.Name, mode.MaxTurns)
		}
	}
}
