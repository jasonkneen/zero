package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/Gitlawb/zero/internal/agent"
	"github.com/Gitlawb/zero/internal/zeroline"
)

// runZeroline launches the interactive Zero TUI with the "zeroline" skin: a Zen
// home page and a Statusline chat page with 5 switchable color themes. It reuses
// the exact same runtime wiring as the default `zero` shell (provider, tools,
// sandbox, permissions, sessions) — only the rendering differs.
//
// With --snapshot it renders a single static frame (home page) to stdout for
// local verification without a TTY.
func runZeroline(args []string, stdout io.Writer, stderr io.Writer, deps appDeps) int {
	fs := flag.NewFlagSet("zeroline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	snapshot := fs.Bool("snapshot", false, "render a single frame to stdout and exit (no TTY)")
	page := fs.String("page", "home", "snapshot page: home|chat")
	variant := fs.Int("variant", 0, "color theme 0-5 (0 ZERO, 1 Phosphor, 2 Cyan, 3 Sage, 4 Violet, 5 Mono)")
	light := fs.Bool("light", false, "use the light variant for the snapshot")
	perm := fs.Bool("perm", false, "show the centered permission modal in the chat snapshot")
	boot := fs.Int("boot", -1, "render the boot splash at the given animation frame")
	stream := fs.Bool("stream", false, "show a streaming assistant response in the chat snapshot")
	jsonMode := fs.Bool("json", false, "render the chat snapshot in JSON mode (TEXT/JSON toggle)")
	sessionsDrawer := fs.Bool("sessions", false, "show the sessions drawer in the chat snapshot")
	tool := fs.String("tool", "", "show a single tool card in the chat snapshot: diff|read|bash|grep")
	frameN := fs.Int("frame", 0, "animation frame for the chat snapshot (spinner)")
	width := fs.Int("width", 100, "snapshot width")
	height := fs.Int("height", 30, "snapshot height")
	skipUnsafe := fs.Bool("skip-permissions-unsafe", false, "launch in unsafe permission mode (enables the ! shell escape)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *snapshot {
		v := *variant // theme index directly (0 ZERO, 1 Phosphor, …)
		if v < 0 || v >= len(zeroline.Themes) {
			v = 0
		}
		if *boot >= 0 {
			if _, err := fmt.Fprintln(stdout, zeroline.RenderBoot(v, !*light, *boot, *width, *height)); err != nil {
				return 1
			}
			return 0
		}
		hdr := zeroline.Header{Cwd: "~/src/zero", Branch: "main", Model: "claude-sonnet-4.5", Provider: "anthropic"}
		var frame string
		if *page == "chat" {
			cd := zeroline.ChatData{
				Variant: v, Dark: !*light, Width: *width, Height: *height, Header: hdr,
				Rows: []zeroline.Row{
					{Kind: "user", Text: "refactor internal/agent/loop.go to extract tool execution"},
					{Kind: "tool", Tool: "grep", Text: "internal/agent", Status: "ok", Detail: "internal/agent/loop.go:88:\tfor i, call := range reply.ToolCalls {\ninternal/agent/loop.go:90:\t\tgo func() {"},
					{Kind: "tool", Tool: "read_file", Text: "internal/agent/loop.go", Status: "ok", Detail: "func (l *Loop) run(ctx context.Context) error {\n\tfor step := 0; step < l.MaxSteps; step++ {\n\t\treply, _, err := l.model.Complete(ctx, msgs, l.tools)"},
					{Kind: "tool", Tool: "edit_file", Text: "internal/agent/exec.go", Status: "ok", Detail: " func (l *Loop) run(ctx context.Context) error {\n-\tswitch t := call.Tool.(type) {\n-\tcase ReadFileTool: out, err = l.readFile(ctx, t)\n+\tout, err := l.exec.Dispatch(call)"},
					{Kind: "tool", Tool: "bash", Text: "go test ./...", Status: "ok", Detail: "ok  github.com/zero-dev/zero/internal/agent\t0.061s\nok  github.com/zero-dev/zero/internal/tools\t0.140s"},
					{Kind: "assistant", Text: "Done. Extracted a `ToolExecutor`:\n\n```go\nfunc (e *ToolExecutor) Dispatch(c Call) (Out, error) {\n\treturn e.route(c)\n}\n```\n\nThe switch in loop.go now delegates to one call. Tests pass."},
					{Kind: "final", Text: "Extracted tool dispatch into a ToolExecutor; loop.go now delegates. go test ./... is green."},
					{Kind: "done", Text: "4 tools · 1,284 tok · $0.04", Status: "ok"},
				},
				Input: "add a test for the new ToolExecutor",
				Spin:  *frameN,
			}
			if *tool != "" {
				cd.Rows = toolSnapshotRows(*tool)
			}
			if *perm {
				cd.Perm = &zeroline.Perm{Tool: "edit_file", Risk: "medium", Reason: "writes internal/agent/exec.go and loop.go", Summary: "write"}
			}
			if *jsonMode {
				cd.JSONMode = true
			}
			if *sessionsDrawer {
				cd.Drawer = &zeroline.Drawer{Sessions: zeroline.DefaultSessions()}
			}
			if *stream {
				cd.Rows = cd.Rows[:len(cd.Rows)-1] // drop the final assistant row
				cd.Working = true
				cd.Stream = "Done. I extracted a `ToolExecutor` and collapsed the dispatch switch in loop.go to a single delegated call — the"
				cd.TokS = 84
			}
			frame = zeroline.RenderChat(cd)
		} else {
			frame = zeroline.RenderChat(zeroline.ChatData{
				Variant: v, Dark: !*light, Width: *width, Height: *height, Header: hdr,
				Input:     "describe a task for zero…",
				Chips:     zeroline.DefaultChips(),
				ChipIndex: 0,
			})
		}
		if _, err := fmt.Fprintln(stdout, frame); err != nil {
			return 1
		}
		return 0
	}

	permissionMode := agent.PermissionModeAsk
	if *skipUnsafe {
		permissionMode = agent.PermissionModeUnsafe
	}
	return runInteractiveTUIWithSkin(stderr, deps, "zeroline", permissionMode)
}

// toolSnapshotRows builds a chat focused on a single tool card type, for
// `zeroline --snapshot --tool <kind>`.
func toolSnapshotRows(kind string) []zeroline.Row {
	user := zeroline.Row{Kind: "user", Text: "show me the " + kind + " tool card"}
	var tool zeroline.Row
	switch kind {
	case "diff":
		tool = zeroline.Row{Kind: "tool", Tool: "edit_file", Text: "internal/cli/root.go", Status: "ok",
			Detail: " \tfs.IntVar(&o.MaxSteps, \"max-steps\", 50, \"agent step cap\")\n+\tfs.BoolVar(&o.ShowVersion, \"version\", false, \"print version and exit\")\n \n+\tif o.ShowVersion {\n+\t\tfmt.Fprintln(o.Stdout, \"zero \"+buildinfo.Version)\n+\t}"}
	case "read":
		tool = zeroline.Row{Kind: "tool", Tool: "read_file", Text: "internal/agent/loop.go", Status: "ok",
			Detail: "File: internal/agent/loop.go (164 lines)\n\n   1 | func (l *Loop) Run(ctx context.Context, task string) error {\n   2 | \tl.emit(Event{Type: EventStart})\n   3 | \tmsgs := l.seed(task)\n   4 | \tfor step := 0; step < l.MaxSteps; step++ {\n   5 | \t\treply, usage, err := l.model.Complete(ctx, msgs, l.tools)"}
	case "bash":
		tool = zeroline.Row{Kind: "tool", Tool: "bash", Text: "go test ./internal/cli/...", Status: "ok",
			Detail: "ok  \tgithub.com/zero-dev/zero/internal/cli\t0.214s"}
	case "grep":
		tool = zeroline.Row{Kind: "tool", Tool: "grep", Text: "internal/cli", Status: "ok",
			Detail: "internal/cli/root.go:41:fs := flag.NewFlagSet(\"zero\", flag.ContinueOnError)\ninternal/cli/root.go:47:fs.StringVar(&o.Model, \"model\", env(\"ZERO_MODEL\"), \"model id\")"}
	default:
		tool = zeroline.Row{Kind: "tool", Tool: kind, Text: "(unknown tool kind: " + kind + ")", Status: "error", Detail: "supported: diff|read|bash|grep"}
	}
	return []zeroline.Row{user, tool, {Kind: "done", Text: "1 tool · 214 tok", Status: "ok"}}
}
