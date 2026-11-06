package semantic

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema"
)

// buildArrayLiteral consumes the compact compiler-owned plan required by
// rules/mlir/packages/sec-mlir-dialect_package14.md sections 14-23 and 62-63.
// It emits one segment per source entry and never expands a spread by N.
// P17 §§15,65–67 add owning ordinary elements and whole-array responsibility;
// failed construction remains an explicit P17-16 boundary.
func (fb *functionBuilder) buildArrayLiteral(expr *ast.ArrayLiteral, resultType TypeID) (builtValue, error) {
	plan, ok := fb.owner.analyzer.ResolvedArrayLiteralPlanOf(expr)
	if !ok || plan.Length == nil {
		return builtValue{}, fb.unsupported("unresolved array literal", expr.Token)
	}
	nontrivial := sema.CopyClassificationOf(plan.ElementType) != sema.CopyTrivial || !sema.TriviallyDestructible(plan.ElementType)
	if nontrivial && fb.owner.maxPackage < ownershipLoweringPackage {
		return builtValue{}, fb.unsupported("non-trivial array literal element", expr.Token)
	}
	// A failing element leaves through the exit cleanup of the elements
	// already evaluated (prepareOperand; destruction.md § 7, P17-16).
	depth := len(fb.prepared)
	defer fb.takePrepared(depth)
	resolvedResult, resultOK := fb.owner.module.Types.Lookup(resultType)
	if !resultOK || resolvedResult.Kind != TypeArray || resolvedResult.Length != plan.Length.String() {
		return builtValue{}, fmt.Errorf("array literal result type disagrees with resolved plan")
	}
	elementType := resolvedResult.Element
	op := Operation{
		Kind: OpArrayConstruct, ArrayElementType: elementType,
		ArrayLength: plan.Length.String(), Location: location(expr.Token),
	}
	for _, entry := range plan.Entries {
		if entry.SourceIndex < 0 || entry.SourceIndex >= len(expr.Elements) || entry.Length == nil {
			return builtValue{}, fmt.Errorf("array literal plan has invalid source entry")
		}
		source := expr.Elements[entry.SourceIndex]
		expected := elementType
		kind := ArraySegmentElement
		action := ArrayActionConstructDirect
		if entry.Kind == sema.ArrayLiteralRange {
			// One compact segment per source range: its operand is the
			// constant lower bound, its length the value count. An empty
			// exclusive segment contributes nothing.
			if entry.Length.Sign() == 0 {
				continue
			}
			if entry.RangeLower == nil {
				return builtValue{}, fmt.Errorf("array literal range segment has no lower bound")
			}
			lower := fb.result(Operation{Kind: OpConstInt, Integer: new(big.Int).Set(entry.RangeLower), Location: location(expressionToken(source))}, elementType)
			op.Operands = append(op.Operands, lower.id)
			op.ArraySegmentKinds = append(op.ArraySegmentKinds, ArraySegmentRange)
			op.ArraySegmentLengths = append(op.ArraySegmentLengths, entry.Length.String())
			op.ArrayActions = append(op.ArrayActions, ArrayActionConstructDirect)
			continue
		}
		if entry.Kind == sema.ArrayLiteralSpread {
			spread, spreadOK := source.(*ast.SpreadExpression)
			resolvedAction, supported := semanticArraySpreadAction(entry.Action)
			if !spreadOK || !supported {
				return builtValue{}, fb.unsupported("non-trivial array spread action "+string(entry.Action), expressionToken(source))
			}
			source = spread.Value
			var internErr error
			expected, internErr = fb.owner.internType(entry.Type)
			if internErr != nil {
				return builtValue{}, internErr
			}
			kind = ArraySegmentSpread
			action = resolvedAction
		} else {
			if entry.Kind != sema.ArrayLiteralElement {
				return builtValue{}, fmt.Errorf("unknown resolved array element kind %s", entry.Kind)
			}
			switch entry.Action {
			case sema.ArrayTransferConstructDirect:
			case sema.ArrayTransferCopyTrivial:
				action = ArrayActionCopyTrivial
			case sema.ArrayTransferMove:
				if fb.owner.maxPackage < ownershipLoweringPackage {
					return builtValue{}, fb.unsupported("array element move requires P17", expressionToken(source))
				}
				action = ArrayActionMove
			default:
				return builtValue{}, fb.unsupported("non-trivial array element action "+string(entry.Action), expressionToken(source))
			}
		}
		value, buildErr := fb.buildExpr(source, expected)
		if buildErr != nil {
			return builtValue{}, buildErr
		}
		if kind == ArraySegmentElement {
			fb.prepareOperand(value, plan.ElementType, expressionToken(source))
		}
		op.Operands = append(op.Operands, value.id)
		op.ArraySegmentKinds = append(op.ArraySegmentKinds, kind)
		op.ArraySegmentLengths = append(op.ArraySegmentLengths, entry.Length.String())
		op.ArrayActions = append(op.ArrayActions, action)
	}
	if nontrivial {
		value := fb.newValue(resultType, OwnershipOwned, op.Location)
		op.Results = []Value{value}
		fb.emit(op)
		return builtValue{id: value.ID, typ: resultType}, nil
	}
	return fb.result(op, resultType), nil
}

// semanticArraySpreadAction admits only the transfer represented by Package 14
// and keeps semantic copy, move and borrow actions visible for later packages.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 21, 89, 103
//   - rules/mlir/semantic-ir/sec_semantic_ir_fixed_array_v1.md — "Array construction"
func semanticArraySpreadAction(action sema.ResolvedArrayTransferAction) (ArrayTransferAction, bool) {
	if action == sema.ArrayTransferCopyTrivial {
		return ArrayActionCopyTrivial, true
	}
	return "", false
}
