package specialist

import (
	"context"
	"fmt"
	"strings"

	"github.com/Gitlawb/zero/internal/agentcli"
)

// runHarness runs a specialist whose manifest pins an external agent-harness
// CLI (claude, codex, gemini, ...) instead of self-exec zero. It shares
// accounting (recordSpecialistStart/Stop, usage rollup) and the final-result
// shaping (BuildFinalResult) with the self-exec path in exec.go, and shares
// child-process lifecycle management with it via runChildWithDecoder — only
// the line decoder (harness_decode.go) and the resolved binary/argv differ.
func (executor Executor) runHarness(ctx context.Context, manifest Manifest, params TaskParameters, options TaskRunOptions) (ExecResult, error) {
	harnessID := strings.TrimSpace(manifest.Metadata.Harness)
	detection, ok := agentcli.DetectOne(harnessID, agentcli.Deps{})
	if !ok {
		return ExecResult{}, fmt.Errorf("%s is not installed or not on PATH", harnessID)
	}

	sessionID, err := executor.newSessionID()
	if err != nil {
		return ExecResult{}, err
	}

	wrappedPrompt := WrapSystemPrompt(manifest.Metadata.Name, manifest.SystemPrompt, params.Prompt, params.Description)
	args := append(detection.Harness.PrintArgs(wrappedPrompt), manifest.Metadata.HarnessArgs...)

	accounting := specialistAccountingInput{
		ParentSessionID: options.ParentSessionID,
		ChildSessionID:  sessionID,
		SpecialistName:  manifest.Metadata.Name,
		Description:     params.Description,
		ToolCallID:      options.ToolCallID,
		Mode:            "harness",
		Background:      false,
	}
	executor.recordSpecialistStart(accounting)

	// The Provider pin is deliberately NOT applied here: ZERO_PROVIDER only
	// means something to a self-exec zero child that reads it at startup, and a
	// harness child is a foreign CLI with its own credential/config store.
	decoder := newHarnessDecoder(detection.Harness.Stream)
	run, err := runChildWithDecoder(ctx, detection.Path, args, nil, strings.TrimSpace(options.Cwd), decoder, options.Progress)
	if err != nil {
		exitCode := run.exitCodeOr(-1)
		summary := SummarizeStream(run.Events, exitCode)
		executor.recordSpecialistStop(accounting, summary, "error", summary.ExitCode, err, false)
		return ExecResult{SessionID: sessionID}, err
	}
	summary := SummarizeStream(run.Events, run.ExitCode)
	rolledUp := executor.rollUpSpecialistUsage(accounting, summary)
	executor.recordSpecialistStop(accounting, summary, summary.Status, summary.ExitCode, nil, rolledUp)
	return ExecResult{
		Result:    BuildFinalResult(run.Events, run.Stderr, run.ExitCode, run.Signal),
		SessionID: sessionID,
	}, nil
}
