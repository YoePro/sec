package sema

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// constantConditionIntegerValue accepts only integer constants whose local
// dependencies are immutable. The general constInts map also tracks the
// current value of mutable locals for bounds checks; treating that transient
// first-iteration value as loop-invariant would make reachability unsound.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §20 "Constant conditions and unreachable code"
//   - rules/control-flow/flowcontrol_while.md — §19 "Constant conditions"
func (a *Analyzer) constantConditionIntegerValue(expression ast.Expression) (*big.Int, bool) {
	if a.constantConditionReferencesMutableBinding(expression) {
		return nil, false
	}
	return a.integerConstantValue(expression)
}

// constantConditionReferencesMutableBinding rejects mutable local dependencies
// before a current-value integer fact is promoted to a compile-time path fact.
// The dependency is transitive: an immutable binding initialized from a
// mutable binding's current value (`let r := counter % 4` inside a loop) is
// as transient as the mutable binding itself.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §20 "Constant conditions and unreachable code"
//   - rules/control-flow/flowcontrol_while.md — §19 "Constant conditions"
func (a *Analyzer) constantConditionReferencesMutableBinding(expression ast.Expression) bool {
	switch expression := expression.(type) {
	case *ast.Identifier:
		symbol, exists := a.symbols[expression.Value]
		return exists && (symbol.Mutable || symbol.TransientConstant)
	case *ast.ConversionExpression:
		return a.constantConditionReferencesMutableBinding(expression.Value)
	case *ast.CallExpression:
		if !a.isExplicitConversionExpression(expression) || len(expression.Arguments) != 1 {
			return false
		}
		return a.constantConditionReferencesMutableBinding(expression.Arguments[0])
	case *ast.PrefixExpression:
		return a.constantConditionReferencesMutableBinding(expression.Right)
	case *ast.InfixExpression:
		return a.constantConditionReferencesMutableBinding(expression.Left) ||
			a.constantConditionReferencesMutableBinding(expression.Right)
	default:
		return false
	}
}

// constantBooleanValue proves the side-effect-free boolean subset currently
// shared by constant integer analysis and control-flow selection. Short-circuit
// operators may prove a result from their left operand alone; otherwise both
// operands must be compile-time known. Unknown expressions remain runtime
// conditions and receive no reachability claim.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §20 "Constant conditions and unreachable code"
//   - rules/control-flow/flowcontrol_while.md — §19 "Constant conditions"
//   - rules/tooling/diagnostics.md — §21 "Proven unreachable and dead code"
func (a *Analyzer) constantBooleanValue(expression ast.Expression) (bool, bool) {
	switch expression := expression.(type) {
	case *ast.BooleanLiteral:
		return expression.Value, true
	case *ast.PrefixExpression:
		if expression.Operator != "!" {
			return false, false
		}
		value, known := a.constantBooleanValue(expression.Right)
		return !value, known
	case *ast.InfixExpression:
		switch expression.Operator {
		case "&&":
			left, known := a.constantBooleanValue(expression.Left)
			if !known {
				return false, false
			}
			if !left {
				return false, true
			}
			return a.constantBooleanValue(expression.Right)
		case "||":
			left, known := a.constantBooleanValue(expression.Left)
			if !known {
				return false, false
			}
			if left {
				return true, true
			}
			return a.constantBooleanValue(expression.Right)
		case "==", "!=":
			if left, leftKnown := a.constantBooleanValue(expression.Left); leftKnown {
				if right, rightKnown := a.constantBooleanValue(expression.Right); rightKnown {
					equal := left == right
					return equal == (expression.Operator == "=="), true
				}
			}
		}

		left, leftKnown := a.constantConditionIntegerValue(expression.Left)
		right, rightKnown := a.constantConditionIntegerValue(expression.Right)
		if !leftKnown || !rightKnown {
			return false, false
		}
		comparison := left.Cmp(right)
		switch expression.Operator {
		case "==":
			return comparison == 0, true
		case "!=":
			return comparison != 0, true
		case "<":
			return comparison < 0, true
		case "<=":
			return comparison <= 0, true
		case ">":
			return comparison > 0, true
		case ">=":
			return comparison >= 0, true
		}
	}
	return false, false
}

// diagnoseConstantConditionUnreachableBlock reports the first statement in a
// block excluded by a proven compile-time boolean condition. Empty blocks
// remain valid, and ordinary block analysis still runs for independent source
// diagnostics.
//
// Rules:
//   - rules/tooling/diagnostics.md — §21 "Proven unreachable and dead code"
//   - rules/control-flow/flowcontrol_if.md — §20 "Constant conditions and unreachable code"
//   - rules/control-flow/flowcontrol_while.md — §19 "Constant conditions"
func (a *Analyzer) diagnoseConstantConditionUnreachableBlock(block *ast.BlockStatement, region string) {
	if block == nil || len(block.Statements) == 0 {
		return
	}
	a.addErrorAtTokenWithMetadata(
		statementToken(block.Statements[0]),
		diagnostics.UnreachableStatement,
		fmt.Sprintf("The constant condition makes this %s impossible. Remove it or change the condition.", region),
		"unreachable statement",
	)
}
