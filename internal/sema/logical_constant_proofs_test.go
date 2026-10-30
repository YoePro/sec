package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestLogicalSharedConstantProofs checks shared text/scalar/nominal constants
// and immutable bool composition without promoting runtime/transient values.
// Rules: rules/foundations/operators.md — "Short-circuit evaluation";
// rules/compiler/compile_time_evaluation.md — §2(3);
// rules/control-flow/flowcontrol_while.md — §19.
func TestLogicalSharedConstantProofs(t *testing.T) {
	path := "../../testdata/control_flow/logical_constants/valid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(source), path)).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := NewAnalyzer()
	if errs := analyzer.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	graph := analyzer.CallGraph()
	skippedID := callGraphNodeIDByName(t, graph, "SkippedEffects")
	if len(graph.Outgoing(skippedID)) != 0 || graph.EffectSummary(skippedID).MayPanic {
		t.Fatal("a proven skipped call leaked into reachable effects")
	}
	var choices *ast.FunctionDeclaration
	for _, statement := range parsed.Program.Statements {
		if fn, ok := statement.(*ast.FunctionDeclaration); ok && fn.Name.Value == "Choices" {
			choices = fn
		}
	}
	if choices == nil {
		t.Fatal("missing Choices")
	}
	expected := []LogicalRHSExecution{
		LogicalRHSNever, LogicalRHSNever, LogicalRHSNever, LogicalRHSNever,
		LogicalRHSNever, LogicalRHSNever, LogicalRHSNever, LogicalRHSNever,
		LogicalRHSAlways, LogicalRHSConditional, LogicalRHSConditional,
	}
	for index, want := range expected {
		expr := choices.Body.Statements[index].(*ast.LetStatement).Value.(*ast.InfixExpression)
		fact, ok := analyzer.ResolvedLogicalFlowOf(expr)
		if !ok || fact.RHSExecution != want {
			t.Errorf("logical expression %d: got %s (resolved=%v), want %s", index, fact.RHSExecution, ok, want)
		}
	}
	loop := choices.Body.Statements[12].(*ast.WhileStatement)
	expr := loop.Body.Statements[1].(*ast.LetStatement).Value.(*ast.InfixExpression)
	if fact, ok := analyzer.ResolvedLogicalFlowOf(expr); !ok || fact.RHSExecution != LogicalRHSConditional {
		t.Fatalf("transient loop value became an invariant proof: %+v", fact)
	}
}

// TestSharedConstantUnreachablePolicy applies the canonical error identity to
// excluded nonempty if/while bodies; folding does not suppress static checking.
// Rules: rules/control-flow/flowcontrol_if.md — §20;
// rules/control-flow/flowcontrol_while.md — §19; rules/tooling/diagnostics.md — §21.
func TestSharedConstantUnreachablePolicy(t *testing.T) {
	source, err := os.ReadFile("../../testdata/control_flow/logical_constants/unreachable_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errs := analyzeSourceRaw(t, string(source))
	if len(errs) != 2 {
		t.Fatalf("want two unreachable statements: %v", errs)
	}
	for i, line := range []int{5, 8} {
		if errs[i].ID != diagnostics.UnreachableStatement || errs[i].Line != line || errs[i].Help == "" {
			t.Fatal(errs[i])
		}
	}
}
