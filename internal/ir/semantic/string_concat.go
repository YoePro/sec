package semantic

import (
	"fmt"

	"sec/internal/ast"
)

// buildFoldedStringConcat consumes Sema's maximal compile-time concatenation
// plan as one const.string operation. It emits no operand operations, runtime
// allocation, failure edge, or intermediate string value.
//
// Rules:
//   - rules/foundations/operators.md — "Compile-time concatenation", "Maximal concatenation plan"
//   - rules/compiler/semantic_ir.md — §11 "Constants"
func (fb *functionBuilder) buildFoldedStringConcat(expr ast.Expression, resultType TypeID) (builtValue, bool, error) {
	plan, ok := fb.owner.analyzer.StringConcatPlanOf(expr)
	if !ok || plan.Runtime {
		return builtValue{}, false, nil
	}
	typ, ok := fb.owner.module.Types.Lookup(resultType)
	if !ok || typ.Kind != TypeString {
		return builtValue{}, true, fmt.Errorf("folded string concatenation has non-string Semantic IR type")
	}
	value := fb.result(Operation{Kind: OpConstString, String: plan.FoldedText, Location: locationFromExpression(expr)}, resultType)
	return value, true, nil
}
