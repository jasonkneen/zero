package sandbox

import (
	"runtime"
	"strings"
)

// InteractiveCommandResult describes the outcome of inspecting a shell command
// for interactive programs that would hang a non-interactive agent (the agent
// has no TTY to type into, so an editor/pager/REPL would block until timeout).
type InteractiveCommandResult struct {
	// Interactive is true when the command launches a program that waits for
	// terminal input the agent cannot supply.
	Interactive bool
	// Command is the matched program/segment (e.g. "vim", "git rebase -i").
	Command string
	// Reason is a short human-readable explanation of why it would hang.
	Reason string
	// Suggestion is an actionable non-interactive alternative.
	Suggestion string
}

// interactiveProgram pairs a detected program with the guidance shown to the agent.
type interactiveProgram struct {
	reason     string
	suggestion string
	// windowsOnly limits the match to GOOS == "windows" (e.g. notepad).
	windowsOnly bool
}

// interactivePrograms maps a bare command name to its non-interactive guidance.
// These programs open a TTY session and block forever without one.
var interactivePrograms = map[string]interactiveProgram{
	// Editors.
	"vim":   {reason: "vim is a full-screen editor that waits for keystrokes", suggestion: "Use a non-interactive edit (the edit_file/write_file tools) or `sed -i`/`printf` to modify files."},
	"vi":    {reason: "vi is a full-screen editor that waits for keystrokes", suggestion: "Use a non-interactive edit (the edit_file/write_file tools) or `sed -i` to modify files."},
	"nvim":  {reason: "nvim is a full-screen editor that waits for keystrokes", suggestion: "Use a non-interactive edit (the edit_file/write_file tools) or `sed -i` to modify files."},
	"nano":  {reason: "nano is a full-screen editor that waits for keystrokes", suggestion: "Use a non-interactive edit (the edit_file/write_file tools) or `sed -i` to modify files."},
	"emacs": {reason: "emacs opens an interactive session", suggestion: "Use `emacs --batch` for scripting, or the edit_file/write_file tools."},
	"pico":  {reason: "pico is a full-screen editor that waits for keystrokes", suggestion: "Use the edit_file/write_file tools or `sed -i`."},
	// Pagers.
	"less": {reason: "less is a pager that waits for navigation keys", suggestion: "Use `cat`, `head`, or `tail -n N` to print file contents non-interactively."},
	"more": {reason: "more is a pager that waits for navigation keys", suggestion: "Use `cat`, `head`, or `tail -n N` to print file contents non-interactively."},
	"most": {reason: "most is a pager that waits for navigation keys", suggestion: "Use `cat`, `head`, or `tail -n N` to print file contents non-interactively."},
	// Process/system monitors.
	"top":   {reason: "top runs a live full-screen dashboard until you quit it", suggestion: "Use `ps aux` (optionally `| head`) for a one-shot snapshot."},
	"htop":  {reason: "htop runs a live full-screen dashboard until you quit it", suggestion: "Use `ps aux` (optionally `| head`) for a one-shot snapshot."},
	"btop":  {reason: "btop runs a live full-screen dashboard until you quit it", suggestion: "Use `ps aux` (optionally `| head`) for a one-shot snapshot."},
	"btm":   {reason: "btm runs a live full-screen dashboard until you quit it", suggestion: "Use `ps aux` for a one-shot snapshot."},
	"watch": {reason: "watch re-runs a command on a loop until interrupted", suggestion: "Run the underlying command once instead of wrapping it in `watch`."},
	// Language REPLs (only interactive when invoked with no script/expression).
	"python":  {reason: "python with no script drops into an interactive REPL", suggestion: "Run `python script.py` or `python -c '<code>'`."},
	"python3": {reason: "python3 with no script drops into an interactive REPL", suggestion: "Run `python3 script.py` or `python3 -c '<code>'`."},
	"node":    {reason: "node with no script drops into an interactive REPL", suggestion: "Run `node script.js` or `node -e '<code>'`."},
	"irb":     {reason: "irb is the interactive Ruby REPL", suggestion: "Run `ruby script.rb` or `ruby -e '<code>'`."},
	"ruby":    {reason: "ruby with no script may drop into an interactive session", suggestion: "Run `ruby script.rb` or `ruby -e '<code>'`."},
	"pry":     {reason: "pry is an interactive Ruby REPL", suggestion: "Run `ruby script.rb` instead."},
	"php":     {reason: "php with no script (-a) opens an interactive shell", suggestion: "Run `php script.php` or `php -r '<code>'`."},
	"ghci":    {reason: "ghci is the interactive Haskell REPL", suggestion: "Use `runghc script.hs` instead."},
	// Database / remote clients (interactive when no command/query is supplied).
	"psql":      {reason: "psql opens an interactive SQL prompt", suggestion: "Pass a query with `psql -c '<sql>'` or a file with `psql -f file.sql`."},
	"mysql":     {reason: "mysql opens an interactive SQL prompt", suggestion: "Pass a query with `mysql -e '<sql>'` or a file with `mysql < file.sql`."},
	"sqlite3":   {reason: "sqlite3 with no SQL opens an interactive prompt", suggestion: "Pass SQL inline: `sqlite3 db.sqlite '<sql>'`."},
	"redis-cli": {reason: "redis-cli with no command opens an interactive prompt", suggestion: "Pass the command inline: `redis-cli GET key`."},
	"mongo":     {reason: "mongo opens an interactive shell", suggestion: "Pass `--eval '<js>'` or a script file."},
	"mongosh":   {reason: "mongosh opens an interactive shell", suggestion: "Pass `--eval '<js>'` or a script file."},
	// Remote/terminal sessions (interactive when no remote command is supplied).
	"ssh":    {reason: "ssh with no remote command opens an interactive login shell", suggestion: "Append the command to run remotely: `ssh host 'command'`."},
	"telnet": {reason: "telnet opens an interactive session", suggestion: "Use `curl`/`nc` with piped input for scripted access."},
	"ftp":    {reason: "ftp opens an interactive session", suggestion: "Use `curl`/`wget` for scripted transfers."},
	"sftp":   {reason: "sftp opens an interactive session", suggestion: "Use `scp` for scripted transfers."},
	// Debuggers.
	"gdb":  {reason: "gdb opens an interactive debugger prompt", suggestion: "Use `gdb -batch -ex '<cmd>'` for scripted debugging."},
	"lldb": {reason: "lldb opens an interactive debugger prompt", suggestion: "Use `lldb --batch -o '<cmd>'` for scripted debugging."},
	// Fuzzy finders / selectors.
	"fzf":  {reason: "fzf is an interactive fuzzy finder", suggestion: "Use `grep`/`rg` to filter non-interactively."},
	"peco": {reason: "peco is an interactive selector", suggestion: "Use `grep`/`rg` to filter non-interactively."},
	// Windows-only interactive launchers.
	"notepad": {reason: "notepad opens a GUI editor", suggestion: "Use the edit_file/write_file tools instead.", windowsOnly: true},
}

// replPrograms only hang when no script/expression argument is provided. The
// listed flags switch them into non-interactive mode and should suppress the
// guard.
var nonInteractiveREPLFlags = map[string][]string{
	"python":  {"-c", "-m"},
	"python3": {"-c", "-m"},
	"node":    {"-e", "--eval", "-p", "--print"},
	"ruby":    {"-e"},
	"php":     {"-r", "-f"},
	"psql":    {"-c", "--command", "-f", "--file", "-l", "--list"},
	"mysql":   {"-e", "--execute"},
}

// interactiveSegments are multi-word interactive invocations. The detector
// matches them as substrings (after normalizing whitespace) so flags like
// `git rebase -i` or `tail -f` are caught even mid-pipeline.
var interactiveSegments = []struct {
	match      string
	command    string
	reason     string
	suggestion string
}{
	{match: "git rebase -i", command: "git rebase -i", reason: "interactive rebase opens an editor for the todo list", suggestion: "Use a non-interactive rebase (`git rebase <base>`) or scripted `git rebase --onto`, and resolve via `git rebase --continue`."},
	{match: "git rebase --interactive", command: "git rebase -i", reason: "interactive rebase opens an editor for the todo list", suggestion: "Use a non-interactive rebase (`git rebase <base>`)."},
	{match: "git add -i", command: "git add -i", reason: "interactive add opens a selection prompt", suggestion: "Stage paths explicitly: `git add <path>`."},
	{match: "git add -p", command: "git add -p", reason: "interactive patch staging opens a prompt", suggestion: "Stage paths explicitly: `git add <path>`."},
	{match: "git commit -p", command: "git commit -p", reason: "interactive patch commit opens a prompt", suggestion: "Stage with `git add <path>` then `git commit -m`."},
	{match: "tail -f", command: "tail -f", reason: "tail -f follows a file forever", suggestion: "Use `tail -n N <file>` for a bounded read."},
	{match: "tail --follow", command: "tail -f", reason: "tail --follow follows a file forever", suggestion: "Use `tail -n N <file>` for a bounded read."},
	{match: "journalctl -f", command: "journalctl -f", reason: "journalctl -f streams logs forever", suggestion: "Use `journalctl -n N` for a bounded read."},
	{match: "kubectl logs -f", command: "kubectl logs -f", reason: "kubectl logs -f streams logs forever", suggestion: "Drop -f and use `kubectl logs --tail=N`."},
	{match: "docker logs -f", command: "docker logs -f", reason: "docker logs -f streams logs forever", suggestion: "Drop -f and use `docker logs --tail N`."},
	{match: "docker attach", command: "docker attach", reason: "docker attach joins an interactive container session", suggestion: "Use `docker exec <id> <command>` for one-shot execution."},
}

// DetectInteractiveCommand inspects a shell command for interactive programs
// that would block a non-interactive agent. goos selects platform-specific
// rules (pass "" to use the host runtime.GOOS).
func DetectInteractiveCommand(command string, goos string) InteractiveCommandResult {
	command = strings.TrimSpace(command)
	if command == "" {
		return InteractiveCommandResult{}
	}
	if goos == "" {
		goos = runtime.GOOS
	}

	normalized := normalizeWhitespace(command)
	lowered := strings.ToLower(normalized)

	// Multi-word interactive invocations (flags/subcommands) take priority so
	// the more specific message wins.
	for _, segment := range interactiveSegments {
		if strings.Contains(lowered, segment.match) {
			return InteractiveCommandResult{
				Interactive: true,
				Command:     segment.command,
				Reason:      segment.reason,
				Suggestion:  segment.suggestion,
			}
		}
	}

	// Inspect each shell segment (split on &&, ||, ;, |) so an interactive
	// program hidden behind an operator is still caught.
	for _, segment := range splitShellSegments(normalized) {
		fields := strings.Fields(segment)
		first := firstProgram(fields)
		if first == "" {
			continue
		}
		program, ok := interactivePrograms[first]
		if !ok {
			continue
		}
		if program.windowsOnly && goos != "windows" {
			continue
		}
		if hasNonInteractiveFlag(first, fields) {
			continue
		}
		return InteractiveCommandResult{
			Interactive: true,
			Command:     first,
			Reason:      program.reason,
			Suggestion:  program.suggestion,
		}
	}

	return InteractiveCommandResult{}
}

// firstProgram returns the first executable name in a segment, skipping leading
// environment-variable assignments (FOO=bar cmd) and `sudo`/`command`/`env`
// prefixes that precede the real program.
func firstProgram(fields []string) string {
	for index := 0; index < len(fields); index++ {
		field := fields[index]
		if strings.Contains(field, "=") && !strings.HasPrefix(field, "=") {
			// Environment assignment prefix; keep scanning.
			continue
		}
		token := normalizeProgramToken(field)
		switch token {
		case "sudo", "command", "env", "nohup", "time", "exec", "doas":
			// Wrapper prefix; the real program follows.
			continue
		}
		return token
	}
	return ""
}

// normalizeProgramToken reduces a raw command token to a bare, lowercased program
// name: it strips surrounding quotes and shell-substitution characters, removes
// any directory prefix (so /usr/bin/vim and C:\tools\vim.exe match "vim"), and
// lowercases. This closes path/quote/substitution evasions of the detector.
func normalizeProgramToken(field string) string {
	const left = "$('\"" + "`"
	const right = ")'\"" + "`"
	token := strings.TrimSpace(field)
	token = strings.TrimLeft(token, left)
	token = strings.TrimRight(token, right)
	token = strings.TrimPrefix(token, "\\")
	if i := strings.LastIndexAny(token, "/\\"); i >= 0 {
		token = token[i+1:]
	}
	return strings.ToLower(token)
}

// hasNonInteractiveFlag reports whether a REPL-style program was invoked in a
// non-interactive way (an inline expression/script flag or a positional script
// argument), in which case it will not hang.
func hasNonInteractiveFlag(program string, fields []string) bool {
	flags, isREPL := nonInteractiveREPLFlags[program]
	if !isREPL {
		// SSH and friends are interactive only with no trailing command. If
		// there is an argument that is not an option, treat it as a remote
		// command/host+command and let it through for ssh-like programs.
		return hasTrailingCommand(program, fields)
	}
	// Find the program's own index, then inspect the args after it.
	start := programIndex(program, fields)
	if start < 0 {
		return false
	}
	for _, arg := range fields[start+1:] {
		for _, flag := range flags {
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				return true
			}
		}
		// A positional (non-flag) argument means a script path was supplied.
		if !strings.HasPrefix(arg, "-") {
			return true
		}
	}
	return false
}

// hasTrailingCommand handles ssh/telnet/db clients: presence of a trailing
// non-option argument (beyond the host) implies a one-shot command rather than
// an interactive session.
func hasTrailingCommand(program string, fields []string) bool {
	start := programIndex(program, fields)
	if start < 0 {
		return false
	}
	args := fields[start+1:]
	switch program {
	case "ssh":
		// ssh <host> <command...>: a host plus at least one more token.
		positional := 0
		for _, arg := range args {
			if !strings.HasPrefix(arg, "-") {
				positional++
			}
		}
		return positional >= 2
	case "sqlite3":
		// sqlite3 <db> <sql>: a db plus an SQL argument.
		positional := 0
		for _, arg := range args {
			if !strings.HasPrefix(arg, "-") {
				positional++
			}
		}
		return positional >= 2
	case "redis-cli":
		// redis-cli <command ...>: any positional command token.
		for _, arg := range args {
			if !strings.HasPrefix(arg, "-") {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func programIndex(program string, fields []string) int {
	for index, field := range fields {
		if strings.ToLower(strings.TrimPrefix(field, "\\")) == program {
			return index
		}
	}
	return -1
}

// splitShellSegments splits a command on the common shell operators so each
// pipeline/list element can be inspected independently.
func splitShellSegments(command string) []string {
	// Split on shell operators AND command-substitution boundaries ($(...), `...`)
	// so an interactive program hidden inside a substitution becomes its own
	// segment (e.g. `echo $(vim x)` -> segment "vim x").
	replacer := strings.NewReplacer(
		"&&", "\n", "||", "\n", ";", "\n", "|", "\n",
		"$(", "\n", ")", "\n", "`", "\n",
	)
	parts := strings.Split(replacer.Replace(command), "\n")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			segments = append(segments, trimmed)
		}
	}
	return segments
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
