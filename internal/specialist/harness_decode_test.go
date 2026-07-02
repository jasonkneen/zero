package specialist

import (
	"testing"

	"github.com/Gitlawb/zero/internal/streamjson"
)

// runDecoder feeds lines through a fresh decoder (as runChildWithDecoder
// would: decodeLine per non-empty line, then finish once at exit) and
// collects every emitted event, so decoder behavior is testable without
// spawning a real child process.
func runDecoder(decoder childDecoder, lines []string, exitCode int) []streamjson.Event {
	var events []streamjson.Event
	for _, line := range lines {
		events = append(events, decoder.decodeLine(line)...)
	}
	events = append(events, decoder.finish(exitCode)...)
	return events
}

func TestNewHarnessDecoderSelectsByStreamFormat(t *testing.T) {
	cases := map[string]childDecoder{
		"claude-stream-json": &claudeStreamDecoder{},
		"codex-json":         &codexJSONDecoder{},
		"text":               &textDecoder{},
		"gemini-json":        &textDecoder{}, // unrecognized -> text fallback
		"":                   &textDecoder{}, // unrecognized -> text fallback
	}
	for stream, want := range cases {
		t.Run(stream, func(t *testing.T) {
			got := newHarnessDecoder(stream)
			gotType := typeName(got)
			wantType := typeName(want)
			if gotType != wantType {
				t.Fatalf("newHarnessDecoder(%q) = %s, want %s", stream, gotType, wantType)
			}
		})
	}
}

func typeName(decoder childDecoder) string {
	switch decoder.(type) {
	case *claudeStreamDecoder:
		return "claude"
	case *codexJSONDecoder:
		return "codex"
	case *textDecoder:
		return "text"
	default:
		return "unknown"
	}
}

func TestClaudeStreamDecoderTextAndToolCallAndFinal(t *testing.T) {
	lines := []string{
		`{"type":"system","subtype":"init","cwd":"/tmp"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Looking at the file"},{"type":"tool_use","id":"call_1","name":"read_file","input":{"path":"a.go"}}]}}`,
		`{"type":"result","subtype":"success","result":"All done"}`,
	}
	events := runDecoder(&claudeStreamDecoder{}, lines, 0)

	if len(events) != 3 {
		t.Fatalf("got %d events, want 3: %#v", len(events), events)
	}
	if events[0].Type != streamjson.EventText || events[0].Delta != "Looking at the file" {
		t.Fatalf("event 0 = %#v", events[0])
	}
	if events[1].Type != streamjson.EventToolCall || events[1].Name != "read_file" || events[1].ID != "call_1" {
		t.Fatalf("event 1 = %#v", events[1])
	}
	if args, ok := events[1].Args.(map[string]any); !ok || args["path"] != "a.go" {
		t.Fatalf("event 1 args = %#v, want map with path=a.go", events[1].Args)
	}
	if events[2].Type != streamjson.EventFinal || events[2].Text != "All done" {
		t.Fatalf("event 2 = %#v", events[2])
	}
}

func TestClaudeStreamDecoderTolerantOfMalformedAndUnknownLines(t *testing.T) {
	lines := []string{
		`not json at all`,
		`{"type":"ping"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`,
	}
	events := runDecoder(&claudeStreamDecoder{}, lines, 0)
	if len(events) != 1 || events[0].Type != streamjson.EventText || events[0].Delta != "hi" {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestClaudeStreamDecoderNoSyntheticFinalWithoutResultLine(t *testing.T) {
	lines := []string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"partial"}]}}`,
	}
	events := runDecoder(&claudeStreamDecoder{}, lines, 1)
	for _, event := range events {
		if event.Type == streamjson.EventFinal {
			t.Fatalf("expected no synthetic final event, got %#v", events)
		}
	}
}

func TestCodexJSONDecoderAgentMessageAndTaskComplete(t *testing.T) {
	lines := []string{
		`{"msg":{"type":"agent_message","message":"step one"}}`,
		`{"msg":{"type":"agent_message","message":"step two"}}`,
		`{"msg":{"type":"token_count","input_tokens":10,"output_tokens":5,"total_tokens":15}}`,
		`{"msg":{"type":"task_complete"}}`,
	}
	events := runDecoder(&codexJSONDecoder{}, lines, 0)
	if len(events) != 4 {
		t.Fatalf("got %d events, want 4: %#v", len(events), events)
	}
	if events[0].Type != streamjson.EventText || events[0].Delta != "step one" {
		t.Fatalf("event 0 = %#v", events[0])
	}
	if events[1].Type != streamjson.EventText || events[1].Delta != "step two" {
		t.Fatalf("event 1 = %#v", events[1])
	}
	if events[2].Type != streamjson.EventUsage || events[2].PromptTokens == nil || *events[2].PromptTokens != 10 {
		t.Fatalf("event 2 = %#v", events[2])
	}
	if events[3].Type != streamjson.EventFinal || events[3].Text != "step two" {
		t.Fatalf("event 3 = %#v, want final with last agent_message", events[3])
	}
}

func TestCodexJSONDecoderFinalizesOnLastMessageWhenStreamEndsWithoutTaskComplete(t *testing.T) {
	lines := []string{
		`{"msg":{"type":"agent_message","message":"only message"}}`,
	}
	events := runDecoder(&codexJSONDecoder{}, lines, -1)
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (text + synthesized final): %#v", len(events), events)
	}
	if events[1].Type != streamjson.EventFinal || events[1].Text != "only message" {
		t.Fatalf("event 1 = %#v", events[1])
	}
}

func TestCodexJSONDecoderIgnoresMalformedAndUnknownLines(t *testing.T) {
	lines := []string{
		`{not json`,
		`{"msg":{"type":"reasoning","message":"thinking..."}}`,
	}
	events := runDecoder(&codexJSONDecoder{}, lines, 0)
	if len(events) != 0 {
		t.Fatalf("expected no events, got %#v", events)
	}
}

func TestTextDecoderStreamsAndFinalizesOnCleanExit(t *testing.T) {
	lines := []string{"line one", "line two"}
	events := runDecoder(&textDecoder{}, lines, 0)
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3: %#v", len(events), events)
	}
	if events[0].Type != streamjson.EventText || events[0].Delta != "line one\n" {
		t.Fatalf("event 0 = %#v", events[0])
	}
	if events[2].Type != streamjson.EventFinal || events[2].Text != "line one\nline two" {
		t.Fatalf("event 2 = %#v", events[2])
	}
}

func TestTextDecoderNoFinalOnNonZeroExit(t *testing.T) {
	events := runDecoder(&textDecoder{}, []string{"oops"}, 1)
	if len(events) != 1 || events[0].Type != streamjson.EventText {
		t.Fatalf("expected only the streamed text event, got %#v", events)
	}
}
