package main

import (
	"fmt"
	"io"
	"strings"
)

type checkCommand struct {
	inputs []string
	target CompilerTarget
}

// parseCheckCommandArgs selects explicit source inputs and the same target
// context used by the compiler's other frontend commands.
// Rules: rules/compiler/compiler.md — §65(1–3);
// rules/compiler/compiler_pipeline.md — §56(1–3).
func parseCheckCommandArgs(args []string, host CompilerTarget) (checkCommand, error) {
	command := checkCommand{target: host}
	literal := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case literal:
			command.inputs = append(command.inputs, arg)
		case arg == "--":
			literal = true
		case arg == "--target" || strings.HasPrefix(arg, "--target="):
			value := strings.TrimPrefix(arg, "--target=")
			if arg == "--target" {
				if i+1 == len(args) {
					return checkCommand{}, fmt.Errorf("--target requires <os-arch>")
				}
				i++
				value = args[i]
			}
			target, ok := parseCompilerTarget(value)
			if !ok {
				return checkCommand{}, fmt.Errorf("invalid target %q", value)
			}
			command.target = target
		case strings.HasPrefix(arg, "-"):
			return checkCommand{}, fmt.Errorf("unknown check option %s", arg)
		default:
			command.inputs = append(command.inputs, arg)
		}
	}
	if len(command.inputs) == 0 {
		return checkCommand{}, fmt.Errorf("check requires at least one <file.sec|dir|glob>")
	}
	return command, nil
}

// runCheckCommand runs the shared parsing, project/module/import, target and
// mandatory semantic-analysis boundary. It deliberately stops before lowering
// readiness, IR generation, backend-tool discovery, verification and linking:
// none supplies an additional mandatory source-validity fact in this path.
// Rules: rules/compiler/compiler.md — §65(1–4);
// rules/compiler/compiler_pipeline.md — §§26,56(1–3).
func runCheckCommand(args []string, out io.Writer) {
	command, err := parseCheckCommandArgs(args, hostCompilerTarget())
	if err != nil {
		reportToolError("usage", "%v", err)
		printUsage()
		exitCLI(1)
	}
	program, summary := parseSourceInputs(command.inputs, command.target, true)
	analyzeProgramWithSources(program, command.target, collectSourceRootsFromInputs(command.inputs), summary)
	fmt.Fprintln(out, "OK")
}
