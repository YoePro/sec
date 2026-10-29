package semantic

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/sema"
)

// TestResolvedLogicalFlowBoundary rejects incoherent canonical facts before
// operand construction, rather than guessing a replacement from source text.
// Rules: rules/compiler/semantic_ir.md — §§45–46,67(2);
// rules/foundations/operators.md — "Short-circuit evaluation".
func TestResolvedLogicalFlowBoundary(t *testing.T) {
	boolean := sema.Type{Name: "bool", Kind: sema.BoolType}
	base := sema.ResolvedLogicalFlow{Operator: "&&", LeftType: boolean, RightType: boolean,
		ResultType: boolean, RHSExecution: sema.LogicalRHSConditional, EvaluateRHSWhenLeft: true}
	for _, operator := range []string{"&&", "||"} {
		for _, execution := range []sema.LogicalRHSExecution{sema.LogicalRHSNever, sema.LogicalRHSAlways, sema.LogicalRHSConditional} {
			flow := base
			flow.Operator, flow.RHSExecution = operator, execution
			flow.EvaluateRHSWhenLeft, flow.ShortCircuitResult = operator == "&&", operator == "||"
			if err := validateResolvedLogicalFlow(flow); err != nil {
				t.Fatal("valid canonical fact rejected", err)
			}
		}
	}
	for name, corrupt := range map[string]func(*sema.ResolvedLogicalFlow){
		"left":      func(f *sema.ResolvedLogicalFlow) { f.LeftType.Kind = sema.IntType },
		"right":     func(f *sema.ResolvedLogicalFlow) { f.RightType.Kind = sema.InvalidType },
		"result":    func(f *sema.ResolvedLogicalFlow) { f.ResultType.Kind = sema.VoidType },
		"operator":  func(f *sema.ResolvedLogicalFlow) { f.Operator = "&" },
		"edge":      func(f *sema.ResolvedLogicalFlow) { f.EvaluateRHSWhenLeft = false },
		"bypass":    func(f *sema.ResolvedLogicalFlow) { f.ShortCircuitResult = true },
		"execution": func(f *sema.ResolvedLogicalFlow) { f.RHSExecution = "unknown" },
	} {
		t.Run(name, func(t *testing.T) {
			flow := base
			corrupt(&flow)
			// No owner/CFG exists: invalid facts must be rejected before
			// constructing even the left operand or allocating any IR state.
			_, err := (&functionBuilder{}).buildLogical(&ast.InfixExpression{}, flow)
			if err == nil || !strings.Contains(err.Error(), "invalid resolved logical flow") {
				t.Fatalf("corrupted fact did not fail at canonical boundary: %v", err)
			}
		})
	}
}

// TestLogicalFlowSourceBridge preserves Sema-selected never/always RHS paths
// through the same source fixture used by the native compilation regression.
// Existing conditional-edge tests assert exact bool merge arguments/targets.
// Rules: rules/compiler/semantic_ir.md — §67(2);
// rules/foundations/operators.md — "Short-circuit evaluation".
func TestLogicalFlowSourceBridge(t *testing.T) {
	module := buildForFixture(t, "../../../testdata/build/native/logical_contexts.sec")
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	if countOperations(loopFunction(t, module, "FoldedSkip"), OpDirectCall) != 0 {
		t.Fatal("proven-skipped RHS reached Semantic IR")
	}
	if countOperations(loopFunction(t, module, "FoldedRequired"), OpDirectCall) != 1 {
		t.Fatal("required RHS was omitted or duplicated")
	}
}
