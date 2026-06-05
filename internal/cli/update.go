package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Gitlawb/zero/internal/updatecheck"
)

type updateOptions struct {
	check bool
	json  bool
}

func runUpdate(args []string, stdout io.Writer, stderr io.Writer, deps appDeps) int {
	options, help, err := parseUpdateArgs(args)
	if err != nil {
		return writeExecUsageError(stderr, err.Error())
	}
	if help {
		if err := writeUpdateHelp(stdout); err != nil {
			return exitCrash
		}
		return exitSuccess
	}
	if !options.check {
		return writeAppError(stderr, "Only `zero update --check` is available right now.", exitUsage)
	}

	result, err := deps.checkUpdate(context.Background(), updatecheck.Options{
		CurrentVersion: version,
	})
	if err != nil {
		return writeAppError(stderr, "Could not check for updates: "+err.Error(), 1)
	}

	if options.json {
		if err := writePrettyJSON(stdout, result); err != nil {
			return exitCrash
		}
		return exitSuccess
	}
	if _, err := fmt.Fprintln(stdout, updatecheck.Format(result)); err != nil {
		return exitCrash
	}
	return exitSuccess
}

func parseUpdateArgs(args []string) (updateOptions, bool, error) {
	options := updateOptions{}
	for _, arg := range args {
		switch {
		case arg == "-h" || arg == "--help" || arg == "help":
			return options, true, nil
		case arg == "--check":
			options.check = true
		case arg == "--json":
			options.json = true
		case strings.HasPrefix(arg, "-"):
			return options, false, execUsageError{fmt.Sprintf("unknown update flag %q", arg)}
		default:
			return options, false, execUsageError{fmt.Sprintf("unexpected update argument %q", arg)}
		}
	}
	return options, false, nil
}

func writeUpdateHelp(w io.Writer) error {
	_, err := fmt.Fprint(w, `Usage:
  zero update --check [--json]

Checks the latest GitHub release without installing.

Flags:
      --check    Check the latest release
      --json     Print the update check result as JSON
  -h, --help     Show this help
`)
	return err
}
