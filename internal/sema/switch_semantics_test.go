package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Variant payload binding and wildcard patterns in switch cases are rejected
// with S1056 instead of cascading into undefined-name or constructor-context
// diagnostics.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §28 "No pattern matching"
//   - rules/control-flow/flowcontrol_switch.md — §40 "Required diagnostics"
func TestSwitchPatternBindingReportsS1056(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/switch_pattern_binding_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(input))
	type want struct {
		line    int
		message string
		help    string
	}
	wants := []want{
		{12, "switch does not support pattern binding; use match", "Ok payload"},
		{21, "switch does not support pattern binding; use match", "Some payload"},
		{23, "switch does not support wildcard patterns; use default or match", "default"},
		{30, "switch does not support pattern binding; use match", "Integer payload"},
		{32, "switch does not support pattern binding; use match", "Number.Text payload"},
	}
	if len(errors) != len(wants) {
		t.Fatalf("errors = %+v, want %d S1056 diagnostics", errors, len(wants))
	}
	for i, w := range wants {
		got := errors[i]
		if got.ID != diagnostics.SwitchPatternBinding || got.Line != w.line || got.Message != w.message || !strings.Contains(got.Help, w.help) {
			t.Fatalf("error %d = %+v, want S1056 at line %d with %q", i, got, w.line, w.message)
		}
	}
}

// Constructed values whose arguments resolve remain ordinary value cases, and
// unresolved arguments to ordinary function calls keep the undefined-name
// diagnostic rather than being misreported as pattern binding.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §6 "Value cases", §28 "No pattern matching"
func TestSwitchPatternDetectionLeavesValueCasesAlone(t *testing.T) {
	input := `
module main

fn Compute(x: int) int {
    return x
}

fn Select(value: int, limit: int) void {
    switch value {
    case Compute(limit):
        return
    case Compute(missing):
        return
    default:
        return
    }
}
`
	errors := analyzeSourceRaw(t, input)
	if len(errors) != 1 || errors[0].ID == diagnostics.SwitchPatternBinding || !strings.Contains(errors[0].Message, "undefined variable missing") {
		t.Fatalf("errors = %+v, want only the undefined-variable diagnostic", errors)
	}
}

// A switch creates no break or continue target: outside a loop both are
// rejected, and inside a loop they resolve to the enclosing loop, so a break in
// a nested switch is a reachable loop exit.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §31 "Loop-control statements"
//   - rules/control-flow/flowcontrol_while.md — §28 "Sema and flow-analysis requirements"
func TestSwitchCreatesNoLoopControlTarget(t *testing.T) {
	outside := `
module main

fn NoLoop(v: int) void {
    switch v {
    case 1:
        break
    case 2:
        continue
    default:
        return
    }
}
`
	errors := analyzeSourceRaw(t, outside)
	if len(errors) != 2 ||
		!strings.Contains(errors[0].Message, "break is only valid inside a loop") ||
		!strings.Contains(errors[1].Message, "continue is only valid inside a loop") {
		t.Fatalf("errors = %+v, want break and continue rejected outside a loop", errors)
	}

	inside := `
module main

fn InLoop(v: int) int {
    let mut count := 0
    while true {
        switch v {
        case 1:
            break
        default:
            count += 1
            continue
        }
    }
    return count
}
`
	parsed := parser.New(lexer.New(inside))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) != 0 {
		t.Fatalf("parser errors: %v", parsed.Errors())
	}
	analyzer := NewAnalyzer()
	assertSemaErrors(t, analyzer.Analyze(program), nil)
	function := program.Statements[1].(*ast.FunctionDeclaration)
	loop := function.Body.Statements[1].(*ast.WhileStatement)
	flow, ok := analyzer.ResolvedWhileFlowOf(loop)
	if !ok || !flow.HasReachableBreak || !flow.ContinuesAfterLoop {
		t.Fatalf("while flow = %+v, want the nested switch break to exit the loop", flow)
	}
}
