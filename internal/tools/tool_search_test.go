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
func (t searchFakeTool) Description() string  { return t.description }
func (t searchFakeTool) Parameters() Schema   { return t.parameters }
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
