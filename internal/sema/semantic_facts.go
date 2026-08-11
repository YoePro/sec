package sema

import (
	"math/big"

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
	ResolvedEnumCompareEQ                    ResolvedOperatorKind = "enum-compare-eq"
	ResolvedEnumCompareNE                    ResolvedOperatorKind = "enum-compare-ne"
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

type ResolvedTryKind string

const (
	ResolvedTryResultPropagation     ResolvedTryKind = "result-propagation"
	ResolvedTryHandledResult         ResolvedTryKind = "handled-result"
	ResolvedTryHandledArithmetic     ResolvedTryKind = "handled-arithmetic"
	ResolvedTryArithmeticPropagation ResolvedTryKind = "arithmetic-propagation"
)

// ResolvedTry records the exact success/error contract selected by Sema.
// Lowering consumers must not reconstruct this decision from source syntax.
type ResolvedTry struct {
	Kind                ResolvedTryKind
	SuccessType         Type
	ErrorType           Type
	EnclosingResultType Type
}

type ResolvedTryHandlerPatternKind string

const (
	TryHandlerOkBinding   ResolvedTryHandlerPatternKind = "ok-binding"
	TryHandlerOkDiscard   ResolvedTryHandlerPatternKind = "ok-discard"
	TryHandlerErrVariant  ResolvedTryHandlerPatternKind = "err-variant"
	TryHandlerErrCatchAll ResolvedTryHandlerPatternKind = "err-catch-all"
)

type ResolvedTryHandlerFlow string

const (
	TryHandlerInvalidFlow   ResolvedTryHandlerFlow = "invalid"
	TryHandlerProducesValue ResolvedTryHandlerFlow = "produces-value"
	TryHandlerReturns       ResolvedTryHandlerFlow = "returns"
	TryHandlerTerminates    ResolvedTryHandlerFlow = "terminates"
)

type ResolvedTryHandler struct {
	PatternKind ResolvedTryHandlerPatternKind
	Variant     string
	BindingName string
	BindingType Type
	Flow        ResolvedTryHandlerFlow
	ResultType  Type
	SourceIndex int
}

type ResolvedTryPlan struct {
	SuccessType   Type
	ErrorType     Type
	HasExplicitOk bool
	Exhaustive    bool
	Handlers      []ResolvedTryHandler
}

type ResolvedEnumCase struct {
	EnumType Type
	Name     string
	Ordinal  uint32
	Value    *big.Int
	Token    lexer.Token
}

type ResolvedEnumConversionKind string

const (
	ResolvedIntegerToEnum ResolvedEnumConversionKind = "integer-to-enum"
	ResolvedEnumToInteger ResolvedEnumConversionKind = "enum-to-integer"
)

type ResolvedEnumConversion struct {
	Kind        ResolvedEnumConversionKind
	EnumType    Type
	IntegerType Type
}

type ResolvedUnionVariantKind string

const (
	ResolvedUnionVariantEmpty  ResolvedUnionVariantKind = "empty"
	ResolvedUnionVariantSingle ResolvedUnionVariantKind = "single"
	ResolvedUnionVariantFields ResolvedUnionVariantKind = "fields"
)

type ResolvedUnionConstruction struct {
	UnionType        Type
	VariantName      string
	VariantIndex     uint32
	Kind             ResolvedUnionVariantKind
	CanonicalFields  []string
	SourceFieldOrder []string
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

// ResolvedTypeDeclarationLocation returns declaration provenance recorded by
// Sema without exposing its mutable symbol tables.
func (a *Analyzer) ResolvedTypeDeclarationLocation(typ Type) (lexer.Token, bool) {
	if a == nil {
		return lexer.Token{}, false
	}
	token, ok := a.typeDefinitionTokens[typ.Name]
	return token, ok
}

// ResolvedEnumCaseOf returns the exact declared case selected by a successfully
// analyzed member expression. Alias cases retain distinct ordinals.
func (a *Analyzer) ResolvedEnumCaseOf(expr *ast.MemberExpression) (ResolvedEnumCase, bool) {
	if a == nil || expr == nil || expr.Property == nil {
		return ResolvedEnumCase{}, false
	}
	typ, ok := a.ResolvedTypeOf(expr)
	if !ok || typ.Kind != EnumType {
		return ResolvedEnumCase{}, false
	}
	value, ok := typ.EnumConsts[expr.Property.Value]
	if !ok || value.Value == nil {
		return ResolvedEnumCase{}, false
	}
	for ordinal, name := range typ.EnumValues {
		if name == value.Name {
			return ResolvedEnumCase{EnumType: typ, Name: value.Name, Ordinal: uint32(ordinal), Value: new(big.Int).Set(value.Value), Token: value.Token}, true
		}
	}
	return ResolvedEnumCase{}, false
}

// ResolvedEnumConversionOf classifies only explicit conversions accepted by
// Sema. It does not infer a conversion from callee spelling alone.
func (a *Analyzer) ResolvedEnumConversionOf(call *ast.CallExpression) (ResolvedEnumConversion, bool) {
	if a == nil || call == nil || len(call.Arguments) != 1 {
		return ResolvedEnumConversion{}, false
	}
	result, resultOK := a.ResolvedTypeOf(call)
	operand, operandOK := a.ResolvedTypeOf(call.Arguments[0])
	if !resultOK || !operandOK {
		return ResolvedEnumConversion{}, false
	}
	if result.Kind == EnumType && isIntegerType(operand) {
		return ResolvedEnumConversion{Kind: ResolvedIntegerToEnum, EnumType: result, IntegerType: operand}, true
	}
	if operand.Kind == EnumType && isIntegerType(result) {
		return ResolvedEnumConversion{Kind: ResolvedEnumToInteger, EnumType: operand, IntegerType: result}, true
	}
	return ResolvedEnumConversion{}, false
}

// ResolvedUnionConstructionOf exposes the concrete union and stable variant
// selected by Sema, including source and canonical field order.
func (a *Analyzer) ResolvedUnionConstructionOf(expr ast.Expression) (ResolvedUnionConstruction, bool) {
	if a == nil || expr == nil {
		return ResolvedUnionConstruction{}, false
	}
	typ, ok := a.ResolvedTypeOf(expr)
	if !ok || typ.Kind != UnionType || len(typ.GenericParameters) != 0 {
		return ResolvedUnionConstruction{}, false
	}
	name := ""
	sourceFields := []string{}
	switch value := expr.(type) {
	case *ast.MemberExpression:
		if value.Property != nil {
			name = value.Property.Value
		}
	case *ast.Identifier:
		name = value.Value
	case *ast.CallExpression:
		if member, memberOK := value.Callee.(*ast.MemberExpression); memberOK && member.Property != nil {
			name = member.Property.Value
		} else {
			name = callExpressionName(value)
		}
	case *ast.StructLiteral:
		_, name, ok = splitUnionVariantTypeName(value.Type.Name)
		if !ok {
			return ResolvedUnionConstruction{}, false
		}
		for _, field := range value.Fields {
			if field != nil && field.Name != nil && !field.Spread {
				sourceFields = append(sourceFields, field.Name.Value)
			}
		}
	default:
		return ResolvedUnionConstruction{}, false
	}
	for index, variant := range typ.UnionVariants {
		if variant.Name != name {
			continue
		}
		resolved := ResolvedUnionConstruction{UnionType: typ, VariantName: name, VariantIndex: uint32(index), SourceFieldOrder: sourceFields}
		switch {
		case variant.Payload != nil:
			resolved.Kind = ResolvedUnionVariantSingle
		case len(variant.PayloadFields) != 0:
			resolved.Kind = ResolvedUnionVariantFields
			for _, field := range variant.PayloadFields {
				resolved.CanonicalFields = append(resolved.CanonicalFields, field.Name)
			}
		default:
			resolved.Kind = ResolvedUnionVariantEmpty
		}
		return resolved, true
	}
	return ResolvedUnionConstruction{}, false
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
	return a.ResolvedBindingAt(identifier.Token.File, identifier.Token.Line, identifier.Token.Column)
}

// ResolvedBindingAt returns the local or parameter binding selected for one
// source position without invoking inference or mutating analyzer state.
func (a *Analyzer) ResolvedBindingAt(file string, line int, column int) (ResolvedBinding, bool) {
	if a == nil {
		return ResolvedBinding{}, false
	}
	use := sourceTokenKey{File: file, Line: line, Column: column}
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

// ResolvedTryOf returns the completed Sema decision for a try expression.
func (a *Analyzer) ResolvedTryOf(expr *ast.TryExpression) (ResolvedTry, bool) {
	if a == nil || expr == nil {
		return ResolvedTry{}, false
	}
	resolved, ok := a.resolvedTries[expr]
	return resolved, ok
}

// ResolvedTryPlanOf returns the immutable handler plan recorded by successful
// Sema. It performs no pattern resolution, exhaustiveness checking or inference.
func (a *Analyzer) ResolvedTryPlanOf(expr *ast.TryExpression) (ResolvedTryPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedTryPlan{}, false
	}
	plan, ok := a.resolvedTryPlans[expr]
	if !ok {
		return ResolvedTryPlan{}, false
	}
	plan.Handlers = append([]ResolvedTryHandler(nil), plan.Handlers...)
	return plan, true
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
		if !leftOK || !rightOK {
			return
		}
		resolved = ResolvedOperator{LeftType: left, RightType: &right, ResultType: result, FailureBehavior: OperatorDoesNotFail}
		if left.Kind == EnumType && right.Kind == EnumType && sameConcreteType(left, right) {
			switch expression.Operator {
			case "==":
				resolved.Kind = ResolvedEnumCompareEQ
			case "!=":
				resolved.Kind = ResolvedEnumCompareNE
			default:
				return
			}
			break
		}
		if !isBuiltinIntegerOperatorType(left) || !isBuiltinIntegerOperatorType(right) {
			return
		}
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
