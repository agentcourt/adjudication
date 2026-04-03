package cli

import (
	"fmt"
	"io"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printRootUsage(stderr)
		return fmt.Errorf("subcommand is required")
	}
	switch args[0] {
	case "acp":
		return RunACP(args[1:], stdout, stderr)
	case "acp-role":
		return RunACPRole(args[1:], stdout, stderr)
	case "case":
		return RunCase(args[1:], stdout, stderr)
	case "complain":
		return RunComplain(args[1:], stdout, stderr)
	case "llm":
		return RunLLM(args[1:], stdout, stderr)
	case "pool":
		return RunPool(args[1:], stdout, stderr)
	case "run":
		return RunScenario(args[1:], stdout, stderr)
	case "xproxy":
		return RunXProxy(args[1:], stdout, stderr)
	case "pacer":
		return RunPacer(args[1:], stdout, stderr)
	case "validate":
		return RunValidate(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		if len(args) == 1 {
			printRootUsage(stdout)
			return nil
		}
		switch args[1] {
		case "acp":
			return RunACP([]string{"-h"}, stdout, stderr)
		case "acp-role":
			return RunACPRole([]string{"-h"}, stdout, stderr)
		case "case":
			return RunCase([]string{"-h"}, stdout, stderr)
		case "complain":
			return RunComplain([]string{"-h"}, stdout, stderr)
		case "llm":
			return RunLLM([]string{"-h"}, stdout, stderr)
		case "pool":
			return RunPool([]string{"-h"}, stdout, stderr)
		case "run":
			return RunScenario([]string{"-h"}, stdout, stderr)
		case "xproxy":
			return RunXProxy([]string{"-h"}, stdout, stderr)
		case "pacer":
			return RunPacer([]string{"-h"}, stdout, stderr)
		case "validate":
			return RunValidate([]string{"-h"}, stdout, stderr)
		default:
			printRootUsage(stderr)
			return fmt.Errorf("unknown help topic %q", args[1])
		}
	default:
		printRootUsage(stderr)
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: adc <subcommand> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  acp        Send one direct prompt through the ACP agent path")
	fmt.Fprintln(w, "  acp-role   Run the next live opportunity for one role through an external ACP agent")
	fmt.Fprintln(w, "  case       Read a complaint, plan both sides, and run the case")
	fmt.Fprintln(w, "  complain   Draft complaint.md from a situation markdown file")
	fmt.Fprintln(w, "  llm        Send one prompt through the runtime xproxy path")
	fmt.Fprintln(w, "  pool       Sample an experimental juror pool from persona clusters")
	fmt.Fprintln(w, "  run        Run an existing scenario JSON")
	fmt.Fprintln(w, "  xproxy     Run the xproxy server")
	fmt.Fprintln(w, "  pacer      List or fetch PACER-style documents from sqlite")
	fmt.Fprintln(w, "  validate   Validate a scenario file for the Go runner")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Use 'adc help <subcommand>' for subcommand flags.")
}
