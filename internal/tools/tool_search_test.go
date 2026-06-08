package tools

import (
	"context"
	"strings"
	"testing"
)

// searchFakeTool is a minimal deferred-eligible tool for tool_search tests:
// it implements Tool plus the optional Deferred() bool that IsDeferred checks.
type searchFakeTool struct {
	name        string
	description string
	parameters  Schema
}

func (t searchFakeTool) Name() string        { return t.name }
func (t searchFakeTool) Description() string { return t.description }
func (t searchFakeTool) Parameters() Schema  { return t.parameters }
func (t searchFakeTool) Safety() Safety {
	return Safety{SideEffect: SideEffectRead, Permission: PermissionAllow}
}
func (t searchFakeTool) Run(context.Context, map[string]any) Result {
	return Result{Status: StatusOK}
}
func (t searchFakeTool) Deferred() bool { return true }

func newDeferredFixtureRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(searchFakeTool{
		name:        "weather_lookup",
		description: "Look up the current weather for a city.",
		parameters: Schema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"city": {Type: "string", Description: "City name to look up."},
			},
			Required:             []string{"city"},
			AdditionalProperties: false,
		},
	})
	reg.Register(searchFakeTool{
		name:        "stock_quote",
		description: "Fetch a stock quote for a ticker symbol.",
		parameters: Schema{
			Type:                 "object",
			Properties:           map[string]PropertySchema{"ticker": {Type: "string"}},
			Required:             []string{"ticker"},
			AdditionalProperties: false,
		},
	})
	return reg
}

func TestToolSearchExposesExpectedSafetyAndSchema(t *testing.T) {
	tool := NewToolSearchTool(NewRegistry())

	if tool.Name() != "tool_search" {
		t.Fatalf("name = %q, want tool_search", tool.Name())
	}
	if tool.Description() == "" {
		t.Fatal("tool_search must have a description")
	}

	safety := tool.Safety()
	if safety.SideEffect != SideEffectNone {
		t.Fatalf("side effect = %s, want none", safety.SideEffect)
	}
	if safety.Permission != PermissionAllow {
		t.Fatalf("permission = %s, want allow", safety.Permission)
	}
	if !safety.AdvertiseInAuto {
		t.Fatal("tool_search must be AdvertiseInAuto")
	}

	schema := tool.Parameters()
	if schema.Type != "object" || schema.AdditionalProperties {
		t.Fatalf("unexpected schema header: %#v", schema)
	}
	queryProp, ok := schema.Properties["query"]
	if !ok {
		t.Fatal("schema missing query property")
	}
	if queryProp.Type != "string" {
		t.Fatalf("query type = %s, want string", queryProp.Type)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "query" {
		t.Fatalf("required = %#v, want [query]", schema.Required)
	}
}

// tool_search must run through the registry's optionsAwareTool dispatch and be
// allowed without a permission grant (SideEffectNone + PermissionAllow).
func TestToolSearchRunsThroughRegistryWithoutPermission(t *testing.T) {
	reg := newDeferredFixtureRegistry()
	reg.Register(NewToolSearchTool(reg))

	result := reg.Run(context.Background(), "tool_search", map[string]any{
		"query": "select:weather_lookup",
	})

	if result.Status != StatusOK {
		t.Fatalf("status = %s, want ok; output=%q", result.Status, result.Output)
	}
	if result.Meta["load_tools"] != "weather_lookup" {
		t.Fatalf("Meta[load_tools] = %q, want weather_lookup", result.Meta["load_tools"])
	}
}

func TestToolSearchMissingQueryArgIsError(t *testing.T) {
	tool := NewToolSearchTool(NewRegistry()).(optionsAwareTool)
	result := tool.RunWithOptions(context.Background(), map[string]any{}, RunOptions{})
	if result.Status != StatusError {
		t.Fatalf("status = %s, want error for missing query", result.Status)
	}
	if !strings.Contains(result.Output, "query is required") {
		t.Fatalf("unexpected error output: %q", result.Output)
	}
}

func TestToolSearchUnknownQueryReturnsNoMeta(t *testing.T) {
	reg := newDeferredFixtureRegistry()
	tool := NewToolSearchTool(reg).(optionsAwareTool)

	for _, query := range []string{"select:does_not_exist", "select:", "zzznomatch"} {
		result := tool.RunWithOptions(context.Background(),
			map[string]any{"query": query}, RunOptions{})

		if result.Status != StatusOK {
			t.Fatalf("query %q: status = %s, want ok (informational)", query, result.Status)
		}
		if _, present := result.Meta["load_tools"]; present {
			t.Fatalf("query %q: must NOT set load_tools, got %q", query, result.Meta["load_tools"])
		}
		// Informational message should name the available tools so the model can retry.
		if !strings.Contains(result.Output, "weather_lookup") || !strings.Contains(result.Output, "stock_quote") {
			t.Fatalf("query %q: message must name available tools, got %q", query, result.Output)
		}
	}
}

func TestToolSearchEmptyRegistryReportsNothingAvailable(t *testing.T) {
	tool := NewToolSearchTool(NewRegistry()).(optionsAwareTool)

	result := tool.RunWithOptions(context.Background(),
		map[string]any{"query": "select:anything"}, RunOptions{})

	if result.Status != StatusOK {
		t.Fatalf("status = %s, want ok", result.Status)
	}
	if _, present := result.Meta["load_tools"]; present {
		t.Fatalf("empty registry must not set load_tools, got %q", result.Meta["load_tools"])
	}
	if !strings.Contains(result.Output, "No deferred tools are available") {
		t.Fatalf("unexpected message: %q", result.Output)
	}
}

func TestToolSearchKeywordRanksByNameThenDescription(t *testing.T) {
	reg := NewRegistry()
	// name match should outrank a description-only match.
	reg.Register(searchFakeTool{
		name:        "weather_lookup",
		description: "Look up the current weather for a city.",
		parameters:  Schema{Type: "object", AdditionalProperties: false},
	})
	reg.Register(searchFakeTool{
		name:        "forecast_report",
		description: "Generates a multi-day weather outlook.",
		parameters:  Schema{Type: "object", AdditionalProperties: false},
	})
	tool := NewToolSearchTool(reg).(optionsAwareTool)

	result := tool.RunWithOptions(context.Background(),
		map[string]any{"query": "weather"}, RunOptions{})

	if result.Status != StatusOK {
		t.Fatalf("status = %s, want ok", result.Status)
	}
	loaded := result.Meta["load_tools"]
	// Both match "weather"; the name match (weather_lookup) must come first.
	if loaded != "weather_lookup,forecast_report" {
		t.Fatalf("load_tools = %q, want weather_lookup,forecast_report (name match ranked first)", loaded)
	}
}

func TestToolSearchKeywordExcludesNonMatches(t *testing.T) {
	reg := newDeferredFixtureRegistry()
	tool := NewToolSearchTool(reg).(optionsAwareTool)

	result := tool.RunWithOptions(context.Background(),
		map[string]any{"query": "stock"}, RunOptions{})

	if got := result.Meta["load_tools"]; got != "stock_quote" {
		t.Fatalf("load_tools = %q, want only stock_quote", got)
	}
	if strings.Contains(result.Output, "weather_lookup") {
		t.Fatalf("non-matching tool weather_lookup leaked into output: %q", result.Output)
	}
}

func TestToolSearchSelectLoadsExactNames(t *testing.T) {
	reg := newDeferredFixtureRegistry()
	tool := NewToolSearchTool(reg)

	optioned, ok := tool.(optionsAwareTool)
	if !ok {
		t.Fatalf("tool_search must implement optionsAwareTool")
	}

	result := optioned.RunWithOptions(context.Background(),
		map[string]any{"query": "select:weather_lookup,stock_quote"}, RunOptions{})

	if result.Status != StatusOK {
		t.Fatalf("status = %s, want ok; output=%q", result.Status, result.Output)
	}
	if got := result.Meta["load_tools"]; got != "weather_lookup,stock_quote" {
		t.Fatalf("Meta[load_tools] = %q, want %q", got, "weather_lookup,stock_quote")
	}
	// Output must carry the FULL description and full schema (not a compact line).
	if !strings.Contains(result.Output, "Look up the current weather for a city.") {
		t.Fatalf("output missing full description: %q", result.Output)
	}
	if !strings.Contains(result.Output, "weather_lookup") || !strings.Contains(result.Output, "\"city\"") {
		t.Fatalf("output missing full schema for weather_lookup: %q", result.Output)
	}
	if !strings.Contains(result.Output, "stock_quote") || !strings.Contains(result.Output, "\"ticker\"") {
		t.Fatalf("output missing full schema for stock_quote: %q", result.Output)
	}
}
