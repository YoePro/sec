package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// BindingID is the stable identity assigned by one successful analysis to a
// parameter or local declaration. Zero is reserved for unresolved bindings.
type BindingID uint32

type BindingKind string

const (
	BindingParameter BindingKind = "parameter"
	BindingLocal     BindingKind = "local"
)

type ResolvedBinding struct {
	ID      BindingID
	Kind    BindingKind
	Name    string
	Type    Type
	Mutable bool
}

type ResolvedCallKind string

const (
	ResolvedDirectCall       ResolvedCallKind = "direct"
	ResolvedForeignCall      ResolvedCallKind = "foreign"
	ResolvedStaticMethodCall ResolvedCallKind = "static-method"
)

type ResolvedCall struct {
	Function Function
	Kind     ResolvedCallKind
}

type ResolvedOperatorKind string

const (
	ResolvedIntegerUnaryPlus                 ResolvedOperatorKind = "integer-unary-plus"
	ResolvedIntegerNegateChecked             ResolvedOperatorKind = "integer-negate-checked"
	ResolvedIntegerBitNot                    ResolvedOperatorKind = "integer-bit-not"
	ResolvedIntegerAddChecked                ResolvedOperatorKind = "integer-add-checked"
	ResolvedIntegerSubtractChecked           ResolvedOperatorKind = "integer-subtract-checked"
	ResolvedIntegerMultiplyChecked           ResolvedOperatorKind = "integer-multiply-checked"
	ResolvedIntegerDivideChecked             ResolvedOperatorKind = "integer-divide-checked"
	ResolvedIntegerRemainderChecked          ResolvedOperatorKind = "integer-remainder-checked"
	ResolvedIntegerBitAnd                    ResolvedOperatorKind = "integer-bit-and"
	ResolvedIntegerBitOr                     ResolvedOperatorKind = "integer-bit-or"
	ResolvedIntegerBitXor                    ResolvedOperatorKind = "integer-bit-xor"
	ResolvedIntegerShiftLeftUnsignedChecked  ResolvedOperatorKind = "integer-shift-left-unsigned-checked"
	ResolvedIntegerShiftLeftSignedChecked    ResolvedOperatorKind = "integer-shift-left-signed-checked"
	ResolvedIntegerShiftRightUnsignedChecked ResolvedOperatorKind = "integer-shift-right-unsigned-checked"
	ResolvedIntegerShiftRightSignedChecked   ResolvedOperatorKind = "integer-shift-right-signed-checked"
	ResolvedIntegerCompareEQ                 ResolvedOperatorKind = "integer-compare-eq"
	ResolvedIntegerCompareNE                 ResolvedOperatorKind = "integer-compare-ne"
	ResolvedIntegerCompareLT                 ResolvedOperatorKind = "integer-compare-lt"
	ResolvedIntegerCompareLE                 ResolvedOperatorKind = "integer-compare-le"
	ResolvedIntegerCompareGT                 ResolvedOperatorKind = "integer-compare-gt"
	ResolvedIntegerCompareGE                 ResolvedOperatorKind = "integer-compare-ge"
)

type OperatorFailureBehavior string

const (
	OperatorDoesNotFail       OperatorFailureBehavior = "none"
	OperatorArithmeticFailure OperatorFailureBehavior = "ordinary-arithmetic-failure"
)

type ResolvedOperator struct {
	Kind            ResolvedOperatorKind
	LeftType        Type
	RightType       *Type
	ResultType      Type
	RuntimeCheck    bool
	FailureBehavior OperatorFailureBehavior
}

// ResolvedTypeOf returns only facts recorded by the completed analysis. It
// never invokes inference and does not mutate Analyzer state.
func (a *Analyzer) ResolvedTypeOf(expr ast.Expression) (Type, bool) {
	if a == nil || expr == nil {
		return Type{}, false
	}
	typ, ok := a.expressionTypes[expr]
	return typ, ok && typ.Kind != InvalidType
}

// ResolvedFunctionForDeclaration returns the already registered declaration.
func (a *Analyzer) ResolvedFunctionForDeclaration(decl *ast.FunctionDeclaration) (Function, bool) {
	if a == nil || decl == nil || decl.Name == nil {
		return Function{}, false
	}
	return a.lookupFunctionByToken(decl.Name.Value, decl.Name.Token)
}

// ResolvedBindingOf returns the declaration identity selected by Sema for an
// identifier occurrence.
func (a *Analyzer) ResolvedBindingOf(identifier *ast.Identifier) (ResolvedBinding, bool) {
	if a == nil || identifier == nil {
		return ResolvedBinding{}, false
	}
	use := sourceTokenLocation(identifier.Token)
	definitions := a.definitionTokens[use]
	if len(definitions) != 1 {
		return ResolvedBinding{}, false
	}
	declaration := definitions[0]
	key := sourceTokenLocation(declaration)
	id, ok := a.bindingIDs[key]
	if !ok {
		return ResolvedBinding{}, false
	}
	fact := a.bindingFacts[key]
	fact.ID = id
	return fact, true
}

func (a *Analyzer) ResolvedCallTarget(call *ast.CallExpression) (ResolvedCall, bool) {
	if a == nil || call == nil {
		return ResolvedCall{}, false
	}
	resolved, ok := a.resolvedCalls[call]
	return resolved, ok
}

// ResolvedOperatorOf returns operator metadata recorded by successful Sema.
// It performs no inference or resolution and never mutates Analyzer state.
func (a *Analyzer) ResolvedOperatorOf(expr ast.Expression) (ResolvedOperator, bool) {
	if a == nil || expr == nil {
		return ResolvedOperator{}, false
	}
	resolved, ok := a.resolvedOperators[expr]
	if !ok {
		return ResolvedOperator{}, false
	}
	if resolved.RightType != nil {
		right := *resolved.RightType
		resolved.RightType = &right
	}
	return resolved, true
}

func (a *Analyzer) recordResolvedOperator(expr ast.Expression, result Type) {
	if a == nil || expr == nil || result.Kind == InvalidType {
		return
	}
	var resolved ResolvedOperator
	switch expression := expr.(type) {
	case *ast.PrefixExpression:
		operand, ok := a.expressionTypes[expression.Right]
		if !ok || !isBuiltinIntegerOperatorType(operand) {
			return
		}
		resolved = ResolvedOperator{LeftType: operand, ResultType: result, FailureBehavior: OperatorDoesNotFail}
		switch expression.Operator {
		case "+":
			resolved.Kind = ResolvedIntegerUnaryPlus
		case "-":
			if operand.Kind != IntType {
				return
			}
			resolved.Kind = ResolvedIntegerNegateChecked
			resolved.RuntimeCheck = true
			resolved.FailureBehavior = OperatorArithmeticFailure
		case "~":
			resolved.Kind = ResolvedIntegerBitNot
		default:
			return
		}
	case *ast.InfixExpression:
		left, leftOK := a.expressionTypes[expression.Left]
		right, rightOK := a.expressionTypes[expression.Right]
		if !leftOK || !rightOK || !isBuiltinIntegerOperatorType(left) || !isBuiltinIntegerOperatorType(right) {
			return
		}
		resolved = ResolvedOperator{LeftType: left, RightType: &right, ResultType: result, FailureBehavior: OperatorDoesNotFail}
		switch expression.Operator {
		case "+":
			resolved.Kind = ResolvedIntegerAddChecked
		case "-":
			resolved.Kind = ResolvedIntegerSubtractChecked
		case "*":
			resolved.Kind = ResolvedIntegerMultiplyChecked
		case "/":
			resolved.Kind = ResolvedIntegerDivideChecked
		case "%":
			resolved.Kind = ResolvedIntegerRemainderChecked
		case "&":
			resolved.Kind = ResolvedIntegerBitAnd
		case "|":
			resolved.Kind = ResolvedIntegerBitOr
		case "^":
			resolved.Kind = ResolvedIntegerBitXor
		case "<<":
			if left.Kind == UintType {
				resolved.Kind = ResolvedIntegerShiftLeftUnsignedChecked
			} else {
				resolved.Kind = ResolvedIntegerShiftLeftSignedChecked
			}
		case ">>":
			if left.Kind == UintType {
				resolved.Kind = ResolvedIntegerShiftRightUnsignedChecked
			} else {
				resolved.Kind = ResolvedIntegerShiftRightSignedChecked
			}
		case "==":
			resolved.Kind = ResolvedIntegerCompareEQ
		case "!=":
			resolved.Kind = ResolvedIntegerCompareNE
		case "<":
			resolved.Kind = ResolvedIntegerCompareLT
		case "<=":
			resolved.Kind = ResolvedIntegerCompareLE
		case ">":
			resolved.Kind = ResolvedIntegerCompareGT
		case ">=":
			resolved.Kind = ResolvedIntegerCompareGE
		default:
			return
		}
		switch resolved.Kind {
		case ResolvedIntegerAddChecked, ResolvedIntegerSubtractChecked,
			ResolvedIntegerMultiplyChecked, ResolvedIntegerDivideChecked,
			ResolvedIntegerRemainderChecked, ResolvedIntegerShiftLeftUnsignedChecked,
			ResolvedIntegerShiftLeftSignedChecked, ResolvedIntegerShiftRightUnsignedChecked,
			ResolvedIntegerShiftRightSignedChecked:
			resolved.RuntimeCheck = true
			resolved.FailureBehavior = OperatorArithmeticFailure
		}
	default:
		return
	}
	if resolved.Kind != "" {
		a.resolvedOperators[expr] = resolved
	}
}

func isBuiltinIntegerOperatorType(typ Type) bool {
	return isIntegerType(typ) && !typ.Named && !typ.Declared && typ.Unit == "" && typ.Dimension.IsZero() && len(typ.Contracts) == 0
}

func (a *Analyzer) recordBinding(token lexer.Token, kind BindingKind, name string, typ Type, mutable bool) {
	key := sourceTokenLocation(token)
	if !validDefinitionToken(token) {
		return
	}
	if _, exists := a.bindingIDs[key]; exists {
		return
	}
	a.bindingIDs[key] = a.nextBindingID
	a.bindingFacts[key] = ResolvedBinding{ID: a.nextBindingID, Kind: kind, Name: name, Type: typ, Mutable: mutable}
	a.nextBindingID++
}

func resolvedCallKind(dispatch CallDispatchKind) ResolvedCallKind {
	switch dispatch {
	case CallDispatchForeign:
		return ResolvedForeignCall
	case CallDispatchStaticMethod:
		return ResolvedStaticMethodCall
	default:
		return ResolvedDirectCall
	}
}
