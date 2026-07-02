package specialist

import (
	"encoding/json"
	"strings"

	"github.com/Gitlawb/zero/internal/streamjson"
)

// newHarnessDecoder selects the childDecoder for a harness's stdout format
// (agentcli.Harness.Stream). An unrecognized value — including "text" itself,
// and any future format the catalog names before a dedicated decoder exists —
// falls back to textDecoder, which treats stdout as opaque prose: a new
// harness catalog entry never hard-fails a run just because its stream format
// isn't specially understood yet.
func newHarnessDecoder(stream string) childDecoder {
	switch stream {
	case "claude-stream-json":
		return &claudeStreamDecoder{}
	case "codex-json":
		return &codexJSONDecoder{}
	default:
		return &textDecoder{}
	}
}

// claudeStreamDecoder decodes the NDJSON stdout produced by `claude -p
// --output-format stream-json` (and cursor-agent's compatible format): a
// "system" line is startup noise (ignored), an "assistant" line's message
// content blocks become text/tool_call events, and a "result" line carries
// the run's final answer.
type claudeStreamDecoder struct {
	gotResult bool
}

type claudeStreamLine struct {
	Type    string `json:"type"`
	Message *struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	} `json:"message"`
	Result string `json:"result"`
}

func (d *claudeStreamDecoder) decodeLine(line string) []streamjson.Event {
	var parsed claudeStreamLine
	// A malformed or shape-mismatched line is tolerated, not fatal — the
	// harness's protocol may add fields/line types this decoder doesn't know
	// about yet.
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		return nil
	}
	switch parsed.Type {
	case "assistant":
		if parsed.Message == nil {
			return nil
		}
		var events []streamjson.Event
		for _, block := range parsed.Message.Content {
			switch block.Type {
			case "text":
				if block.Text != "" {
					events = append(events, streamjson.Event{Type: streamjson.EventText, Delta: block.Text})
				}
			case "tool_use":
				var args any
				if len(block.Input) > 0 {
					_ = json.Unmarshal(block.Input, &args)
				}
				events = append(events, streamjson.Event{Type: streamjson.EventToolCall, ID: block.ID, Name: block.Name, Args: args})
			}
		}
		return events
	case "result":
		d.gotResult = true
		return []streamjson.Event{{Type: streamjson.EventFinal, Text: parsed.Result}}
	default:
		// "system"/"init" and any other line types carry nothing the specialist
		// pipeline needs.
		return nil
	}
}

func (d *claudeStreamDecoder) finish(int) []streamjson.Event {
	// No synthetic final when the stream never produced a "result" line (e.g.
	// the CLI crashed mid-run): BuildFinalResult already reports a non-zero
	// exit as an error using stderr, and inventing a final here would just mask
	// the missing result.
	return nil
}

// codexJSONDecoder decodes the NDJSON stdout produced by `codex exec --json`:
// each line wraps a "msg" whose "type" distinguishes an assistant text chunk
// ("agent_message"), the run's completion marker ("task_complete"), and a
// best-effort token usage report ("token_count"). Codex's final answer is
// "the last agent_message seen", not a dedicated field, so the decoder must
// remember it across lines.
type codexJSONDecoder struct {
	lastMessage string
	done        bool
}

type codexLine struct {
	Msg struct {
		Type         string `json:"type"`
		Message      string `json:"message"`
		InputTokens  *int   `json:"input_tokens"`
		OutputTokens *int   `json:"output_tokens"`
		TotalTokens  *int   `json:"total_tokens"`
	} `json:"msg"`
}

func (d *codexJSONDecoder) decodeLine(line string) []streamjson.Event {
	var parsed codexLine
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		return nil
	}
	switch parsed.Msg.Type {
	case "agent_message":
		if parsed.Msg.Message == "" {
			return nil
		}
		d.lastMessage = parsed.Msg.Message
		return []streamjson.Event{{Type: streamjson.EventText, Delta: parsed.Msg.Message}}
	case "task_complete":
		d.done = true
		return []streamjson.Event{{Type: streamjson.EventFinal, Text: d.lastMessage}}
	case "token_count":
		event := streamjson.Event{Type: streamjson.EventUsage}
		event.PromptTokens = parsed.Msg.InputTokens
		event.CompletionTokens = parsed.Msg.OutputTokens
		event.TotalTokens = parsed.Msg.TotalTokens
		return []streamjson.Event{event}
	default:
		return nil
	}
}

func (d *codexJSONDecoder) finish(int) []streamjson.Event {
	// task_complete never arrived (e.g. the CLI was killed mid-run) but there
	// was at least one agent_message — still surface it as the final answer
	// rather than losing it.
	if d.done || d.lastMessage == "" {
		return nil
	}
	return []streamjson.Event{{Type: streamjson.EventFinal, Text: d.lastMessage}}
}

// textDecoder is the fallback for any harness whose Stream is "text" (or
// unrecognized): stdout is opaque prose, streamed line-by-line as text
// events, with the full accumulated output surfaced as the final answer once
// the process exits cleanly.
type textDecoder struct {
	lines []string
}

func (d *textDecoder) decodeLine(line string) []streamjson.Event {
	d.lines = append(d.lines, line)
	return []streamjson.Event{{Type: streamjson.EventText, Delta: line + "\n"}}
}

func (d *textDecoder) finish(exitCode int) []streamjson.Event {
	if exitCode != 0 {
		// A non-zero exit is reported as an error by BuildFinalResult using
		// stderr; the accumulated stdout is still available via the text deltas
		// already emitted above, so no separate final event is needed here.
		return nil
	}
	return []streamjson.Event{{Type: streamjson.EventFinal, Text: strings.TrimSpace(strings.Join(d.lines, "\n"))}}
}
