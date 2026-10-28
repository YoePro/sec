package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// cliCommand is one public command of the single authoritative CLI registry.
// The usage synopsis, `sec help`, `sec help <command>`, `sec --help` and
// `sec <command> --help` all render from it, so displayed help cannot drift
// from the commands the dispatcher accepts.
//
// Rules:
//   - rules/tooling/cli.md — §§ 3, 4 (command registry and help)
//   - rules/corrections/applied/md017-sec-analyse-cli-and-help-correction-20261017.md — § 8
type cliCommand struct {
	Name     string
	Aliases  []string
	Synopsis []string
	Purpose  string
	Details  string
}

// cliCommands is the canonical command registry in display order.
func cliCommands() []cliCommand {
	return []cliCommand{
		{Name: "check", Synopsis: []string{"sec check [--target <os-arch>] <file.sec|dir|glob>..."},
			Purpose: "Validate sources through the frontend (parse, types, ownership, effects) without backend tools.",
			Details: "Use check while editing: it reports every language diagnostic without building anything."},
		{Name: "analyse", Synopsis: []string{
			"sec analyse [--all | <analysis>] [-f|--function <callable>] [-d|--depth interactive|standard|deep] [--info|--verbose|--json] [--target <os-arch>] [<file.sec|dir|glob>...]"},
			Purpose: "Run the compiler's analyses (pitfalls, parameter usage, stack, ...) and report findings.",
			Details: analyseHelpText()},
		{Name: "build", Synopsis: []string{"sec build <file.sec|TargetName> [--build-number <number>] [-o <program>] [--target <os-arch>] [--keep-mlir] [--keep-llvm] [--mlir-bin <path>] [--clang <path>]"},
			Purpose: "Compile and link a program for a target.",
			Details: "build runs the single compilation track: Semantic IR, Sec dialect MLIR, standard MLIR, the LLVM dialect and LLVM IR, then links with clang. --keep-mlir keeps the Sec dialect module as <program>.mlir and --keep-llvm the LLVM IR as <program>.ll. A construct the track cannot lower yet is reported by name; nothing falls back to another backend."},
		{Name: "test", Synopsis: []string{"sec test [--list] [--run <regexp>] [--json] [--target <os-arch>] [<file_test.sec|dir|glob>...]"},
			Purpose: "Discover, compile and run Sec tests."},
		{Name: "fmt", Synopsis: []string{"sec fmt [--check] [--fix] [--indentation-width <2|4|6>] <file.sec|dir>...", "sec fmt [--fix] [--indentation-width <2|4|6>] --stdin"},
			Purpose: "Format sources canonically, or check that they are formatted.",
			Details: "Directories select .sec files recursively, including tests and internal modules. Hidden, reserved output and testdata directories are skipped; symbolic links are never followed. Every file and configuration is checked before writes. --check reports changes without writing; --fix enables Language Corrections. Malformed files remain unchanged while valid files are processed."},
		{Name: "init", Synopsis: []string{
			"sec init [path] [--dry-run] [--minimal] [--name <name>] [--target <os-arch>] [--profile <profile>]",
			"sec init module <path> [--dry-run]",
			"sec init command|library|firmware|test <name> [--target <os-arch>] [--dry-run]"},
			Purpose: "Create a project, module or target scaffold."},
		{Name: "diagnostics", Synopsis: []string{"sec diagnostics [--json|<ID>]"},
			Purpose: "List registered diagnostics or explain one diagnostic ID in depth."},
		{Name: "emit-ir", Synopsis: []string{"sec emit-ir <file.sec> [-o <file.sir|->] [--target <os-arch>]"},
			Purpose: "Print verified Semantic IR."},
		{Name: "emit-sec-mlir", Synopsis: []string{"sec emit-sec-mlir <file.sec> [-o <file.mlir|->] [--target <os-arch>] [--mlir-bin <path>]"},
			Purpose: "Print verified high-level Sec MLIR (emit-sec-mlir = verified Sec dialect)."},
		{Name: "emit-mlir", Synopsis: []string{"sec emit-mlir <file.sec> -o <file.mlir|-> [--target <os-arch>] [--mlir-bin <path>]"},
			Purpose: "Print the LLVM-dialect MLIR the Sec dialect lowers to."},
		{Name: "emit-llvm", Synopsis: []string{"sec emit-llvm <file.sec> -o <file.ll|-> [--target <os-arch>] [--mlir-bin <path>]"},
			Purpose: "Print the LLVM IR translated from that LLVM-dialect MLIR."},
		{Name: "parse", Aliases: []string{"ast", "sema"}, Synopsis: []string{"sec <parse|ast|sema> <file.sec|dir|glob>..."},
			Purpose: "Debug views: parse tree, AST or semantic analysis of sources."},
		{Name: "lex", Aliases: []string{"token", "tokens", "lexer"}, Synopsis: []string{"sec <lex|token> <file.sec>"},
			Purpose: "Debug view: the token stream of one file."},
		{Name: "help", Synopsis: []string{"sec help [<command>]", "sec --help", "sec <command> --help"},
			Purpose: "Show this command list or explain one command."},
	}
}

func findCLICommand(name string) (cliCommand, bool) {
	for _, command := range cliCommands() {
		if command.Name == name {
			return command, true
		}
		for _, alias := range command.Aliases {
			if alias == name {
				return command, true
			}
		}
	}
	return cliCommand{}, false
}

// printUsage writes the compact synopsis of every registered command.
func printUsage() {
	writeCLISynopsis(os.Stderr)
}

func writeCLISynopsis(out io.Writer) {
	first := true
	for _, command := range cliCommands() {
		for _, line := range command.Synopsis {
			prefix := "       "
			if first {
				prefix, first = "usage: ", false
			}
			fmt.Fprintln(out, prefix+line)
		}
	}
	fmt.Fprintln(out, "       MLIR boundaries: emit-sec-mlir = verified Sec dialect; emit-mlir = LLVM dialect; emit-llvm = LLVM IR (one track)")
	fmt.Fprintln(out, "       diagnostic-producing commands accept --diagnostic-format <human|json>")
	fmt.Fprintln(out, "       run `sec help <command>` for details")
}

// writeCLIHelp renders `sec help` (no topic) or `sec help <command>`.
// An unknown topic is reported with the nearest command names.
//
// Rules:
//   - rules/tooling/cli.md — § 4 (help forms, mentor-style guidance)
func writeCLIHelp(out io.Writer, topic string) error {
	if topic == "" {
		fmt.Fprintln(out, "Sec compiler commands:")
		fmt.Fprintln(out)
		for _, command := range cliCommands() {
			fmt.Fprintf(out, "  %-14s %s\n", command.Name, command.Purpose)
		}
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Run `sec help <command>` or `sec <command> --help` for options and examples.")
		fmt.Fprintln(out, "Run `sec diagnostics <ID>` to explain a diagnostic in depth.")
		return nil
	}
	command, ok := findCLICommand(topic)
	if !ok {
		return fmt.Errorf("unknown command %q%s; run `sec help` to list commands", topic, nearestSuggestion(topic, cliCommandNames()))
	}
	fmt.Fprintf(out, "sec %s — %s\n\nUsage:\n", command.Name, command.Purpose)
	for _, line := range command.Synopsis {
		fmt.Fprintf(out, "  %s\n", line)
	}
	if command.Details != "" {
		fmt.Fprintf(out, "\n%s\n", strings.TrimRight(command.Details, "\n"))
	}
	return nil
}

func cliCommandNames() []string {
	names := []string{}
	for _, command := range cliCommands() {
		names = append(names, command.Name)
		names = append(names, command.Aliases...)
	}
	sort.Strings(names)
	return names
}

// nearestSuggestion names the closest candidate when it is unambiguously
// close (edit distance at most 2), formatted as a "; did you mean" clause.
func nearestSuggestion(value string, candidates []string) string {
	best, bestDistance, ties := "", 3, 0
	for _, candidate := range candidates {
		distance := editDistance(value, candidate)
		switch {
		case distance < bestDistance:
			best, bestDistance, ties = candidate, distance, 1
		case distance == bestDistance:
			ties++
		}
	}
	if best == "" || ties != 1 {
		return ""
	}
	return fmt.Sprintf(" (did you mean %q?)", best)
}

func editDistance(left, right string) int {
	previous := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}
	for i := 1; i <= len(left); i++ {
		current := make([]int, len(right)+1)
		current[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 1
			if left[i-1] == right[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return previous[len(right)]
}

// helpRequest recognizes the help forms before command dispatch:
// `sec help [topic]`, `sec --help`/`-h`, and `sec <command> --help`/`-h`.
func helpRequest(args []string) (topic string, requested bool) {
	if len(args) == 0 {
		return "", false
	}
	switch args[0] {
	case "help":
		if len(args) > 1 {
			return args[1], true
		}
		return "", true
	case "--help", "-h", "-help":
		return "", true
	}
	for _, arg := range args[1:] {
		if arg == "--help" || arg == "-h" || arg == "-help" {
			return args[0], true
		}
	}
	return "", false
}
