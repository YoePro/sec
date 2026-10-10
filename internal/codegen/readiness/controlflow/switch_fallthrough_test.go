package controlflow_test

import (
	"os"
	legacy "sec/internal/codegen/mlir"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestSwitchFallthroughExecution verifies explicit transfers to successor
// bodies without evaluating their tests, chained and nested switches, default
// successors, return, and sibling lexical scopes with outer state propagation.
// Rules: rules/control-flow/flowcontrol_switch.md §§4–5, 15–19, 32, 38–39;
// rules/corrections/applied/correction24-20260823.md — "Bug 2".
func TestSwitchFallthroughExecution(t *testing.T) {
	executeControlFlowFixture(t, "switch_fallthrough", "15 switch fallthrough scenarios passed\n")
}

// TestSwitchFallthroughRejectsInvalidPlacement checks semantic placement and
// lexical isolation, plus explicit raw-backend rejection of malformed transfers.
// Rules: rules/control-flow/flowcontrol_switch.md §§16, 18–19, 38.
func TestSwitchFallthroughRejectsInvalidPlacement(t *testing.T) {
	for _, name := range []string{"final", "default", "not_last", "nested", "nested_loop", "nested_lambda", "nested_unsafe", "scope"} {
		t.Run(name, func(t *testing.T) {
			file := "../../../../testdata/control_flow/switch_fallthrough/" + name + "_invalid.sec"
			input, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			parsed := parser.New(lexer.NewWithFile(string(input), file)).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			errors := sema.NewAnalyzer().Analyze(parsed.Program)
			if len(errors) != 1 {
				t.Fatalf("expected one semantic rejection: %+v", errors)
			}
			if name == "scope" {
				if !strings.Contains(errors[0].Message, "undefined") {
					t.Fatalf("case binding leaked: %+v", errors)
				}
				return
			}
			if !strings.Contains(errors[0].Message, "fallthrough") {
				t.Fatalf("wrong semantic rejection: %+v", errors)
			}
			output, err := legacy.GenerateWithTriple(parsed.Program, "x86_64-pc-linux-gnu")
			if err == nil || output != "" {
				t.Fatalf("raw MLIR accepted invalid transfer: %v\n%s", err, output)
			}
		})
	}
}
