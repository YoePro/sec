package semantic

import (
	"fmt"

	"sec/internal/sema"
)

// validateResolvedLogicalFlow checks the immutable Sema contract before building
// either operand. It validates represented facts without evaluating source,
// proving constants again or selecting an edge from AST spelling.
// Rules: rules/foundations/operators.md — "Short-circuit evaluation";
// rules/compiler/semantic_ir.md — §§45–46,67(2).
func validateResolvedLogicalFlow(flow sema.ResolvedLogicalFlow) error {
	if flow.LeftType.Kind != sema.BoolType || flow.RightType.Kind != sema.BoolType || flow.ResultType.Kind != sema.BoolType {
		return fmt.Errorf("invalid resolved logical flow: operands and result must be bool")
	}
	if flow.Operator != "&&" && flow.Operator != "||" {
		return fmt.Errorf("invalid resolved logical flow: unknown operator %q", flow.Operator)
	}
	if flow.EvaluateRHSWhenLeft != (flow.Operator == "&&") || flow.ShortCircuitResult != (flow.Operator == "||") {
		return fmt.Errorf("invalid resolved logical flow: contradictory selected edge or bypass value")
	}
	switch flow.RHSExecution {
	case sema.LogicalRHSNever, sema.LogicalRHSAlways, sema.LogicalRHSConditional:
		return nil
	default:
		return fmt.Errorf("invalid resolved logical flow: unknown RHS execution %q", flow.RHSExecution)
	}
}
