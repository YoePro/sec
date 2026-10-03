package sema

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TryFailureKind classifies one fallible point inside the expression a try
// protects.
type TryFailureKind string

const (
	// TryFailureCarrier is the protected Result carrier itself.
	TryFailureCarrier TryFailureKind = "carrier"
	// TryFailureArithmetic is a checked arithmetic operation (ArithmeticError).
	TryFailureArithmetic TryFailureKind = "arithmetic"
	// TryFailureBounds is a runtime-checked index (IndexError).
	TryFailureBounds TryFailureKind = "bounds"
	// TryFailureContract is a runtime conversion into a constrained named
	// type (ContractError).
	TryFailureContract TryFailureKind = "contract"
	// TryFailureAllocation is a runtime string concatenation or interpolation
	// whose materialization may fail to allocate (AllocationError, MD-004).
	TryFailureAllocation TryFailureKind = "allocation"
)

// TryFailurePoint is one language-defined fallible point protected by a try.
// The ordered points form the compiler-internal failure set: analysis
// metadata only, never an inferred source union or public error type.
//
// Rules:
//   - rules/errors/errorhandling.md — §11 "Compiler-internal failure sets"
//   - rules/errors/runtime_checks.md — "What try converts", "Error typing"
type TryFailurePoint struct {
	Kind       TryFailureKind
	ErrorType  Type
	Expression ast.Expression
	Token      lexer.Token
}

// tryFailureSetContext is active while analyzing the handlers of a try whose
// failure set has more than one error type.
type tryFailureSetContext struct {
	members []Type
	// commonChannel is the explicitly declared channel every member already
	// shares (the open error root of a Result[T, error] carrier), or nil.
	commonChannel *Type
}

// collectTryFailurePoints walks the protected expression in evaluation order
// (operands before their operation) and returns every language-defined
// runtime check that try converts into fallible control flow. Nested try,
// lambda bodies, and match arms are separate boundaries.
//
// Rules:
//   - rules/errors/runtime_checks.md — "What try converts": "Within its expression subtree, try converts language-defined checks"
//   - rules/errors/errorhandling.md — §9 "Operands accepted by try", §13 "try expression boundaries"
func (a *Analyzer) collectTryFailurePoints(expr ast.Expression) []TryFailurePoint {
	points := []TryFailurePoint{}
	var visit func(ast.Expression)
	visit = func(expr ast.Expression) {
		switch node := expr.(type) {
		case nil:
			return
		case *ast.TryExpression, *ast.LambdaExpression, *ast.MatchExpression:
			return
		case *ast.InfixExpression:
			visit(node.Left)
			visit(node.Right)
			if operator, ok := a.ResolvedOperatorOf(node); ok && operator.RuntimeCheck {
				points = append(points, TryFailurePoint{Kind: TryFailureArithmetic, ErrorType: a.types["ArithmeticError"], Expression: node, Token: node.Token})
			}
			points = a.appendStringMaterializationPoint(points, node, node.Token)
		case *ast.PrefixExpression:
			visit(node.Right)
			if operator, ok := a.ResolvedOperatorOf(node); ok && operator.RuntimeCheck {
				points = append(points, TryFailurePoint{Kind: TryFailureArithmetic, ErrorType: a.types["ArithmeticError"], Expression: node, Token: node.Token})
			}
		case *ast.IndexExpression:
			visit(node.Left)
			visit(node.Index)
			if a.runtimeCheckedIndex(node) {
				points = append(points, TryFailurePoint{Kind: TryFailureBounds, ErrorType: a.indexErrorType(), Expression: node, Token: node.Token})
			}
		case *ast.SliceExpression:
			visit(node.Left)
			visit(node.Start)
			visit(node.End)
		case *ast.MemberExpression:
			visit(node.Object)
		case *ast.CallExpression:
			if member, ok := node.Callee.(*ast.MemberExpression); ok {
				visit(member.Object)
			}
			for _, argument := range node.Arguments {
				visit(argument)
			}
			if a.runtimeContractConversion(node) {
				points = append(points, TryFailurePoint{Kind: TryFailureContract, ErrorType: a.types["ContractError"], Expression: node, Token: node.Token})
			}
		case *ast.ConversionExpression:
			visit(node.Value)
		case *ast.NewExpression:
			for _, argument := range node.Arguments {
				visit(argument)
			}
		case *ast.RefExpression:
			visit(node.Value)
		case *ast.ArrayLiteral:
			for _, element := range node.Elements {
				visit(element)
			}
		case *ast.StructLiteral:
			for _, field := range node.Fields {
				if field != nil {
					visit(field.Value)
				}
			}
		case *ast.OkExpression:
			visit(node.Value)
		case *ast.ErrExpression:
			visit(node.Value)
		case *ast.SpreadExpression:
			visit(node.Value)
		case *ast.InterpolatedStringLiteral:
			for _, part := range node.Parts {
				visit(part.Expression)
			}
			points = a.appendStringMaterializationPoint(points, node, node.Token)
		}
	}
	visit(expr)
	return points
}

// runtimeCheckedIndex reports an index whose bounds are checked at run time:
// a fixed-array access without a static proof, any dynamic-array or slice
// access, and any list access.
func (a *Analyzer) runtimeCheckedIndex(index *ast.IndexExpression) bool {
	if plan, ok := a.resolvedArrayIndexPlans[index]; ok {
		return plan.CheckKind == ArrayIndexRuntimeCheck
	}
	if plan, ok := a.resolvedListIndexPlans[index]; ok {
		return plan.CheckKind == ArrayIndexRuntimeCheck
	}
	containerType, ok := a.expressionTypes[index.Left]
	if !ok {
		return false
	}
	container := dereferenceType(containerType)
	return container.Kind == SliceType || arrayShapeOf(container) == ArrayShapeDynamic
}

// runtimeContractConversion reports an explicit conversion into a named type
// with contracts whose argument is not a compile-time constant; constants are
// validated at compile time, so only run-time values can fail.
//
// Rules:
//   - rules/types/contracts.md — "Runtime conversion into a constrained named type is fallible"
//   - rules/errors/runtime_checks.md — "Type contracts": failure produces ContractError
func (a *Analyzer) runtimeContractConversion(call *ast.CallExpression) bool {
	if len(call.Arguments) != 1 {
		return false
	}
	name := callExpressionName(call)
	if name == "" || len(a.functions[name]) > 0 {
		return false
	}
	target, ok := a.expressionTypes[call]
	if !ok || !hasContracts(target) {
		return false
	}
	if _, constant := a.integerConstantValue(call.Arguments[0]); constant {
		return false
	}
	switch call.Arguments[0].(type) {
	case *ast.StringLiteral, *ast.FloatLiteral, *ast.BooleanLiteral, *ast.CharLiteral:
		return false
	}
	if source, ok := a.expressionTypes[call.Arguments[0]]; ok && sameConcreteType(source, target) {
		return false
	}
	return true
}

// tryFailureSetTypes returns the distinct error types of the failure set in
// first-occurrence order.
func tryFailureSetTypes(points []TryFailurePoint) []Type {
	types := []Type{}
	for _, point := range points {
		duplicate := false
		for _, existing := range types {
			if sameConcreteType(existing, point.ErrorType) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			types = append(types, point.ErrorType)
		}
	}
	return types
}

func tryFailureSetDisplay(types []Type) string {
	names := make([]string, 0, len(types))
	for _, typ := range types {
		names = append(names, typeDisplayName(typ))
	}
	return strings.Join(names, ", ")
}

// usesExistingTryPath reports failure sets the dedicated Result, checked
// arithmetic, and fixed-array bounds paths already represent exactly.
func (a *Analyzer) usesExistingTryPath(root ast.Expression, rootIsResult bool, points []TryFailurePoint) bool {
	if rootIsResult {
		return len(points) == 0
	}
	if len(points) == 0 {
		return true
	}
	if operator, ok := a.ResolvedOperatorOf(root); ok && operator.RuntimeCheck {
		for _, point := range points {
			if point.Kind != TryFailureArithmetic {
				return false
			}
		}
		return true
	}
	if index, ok := root.(*ast.IndexExpression); ok {
		if _, fixed := a.resolvedArrayIndexPlans[index]; fixed {
			return len(points) == 1 && points[0].Expression == root
		}
	}
	return false
}

// inferFailureSetTryExpression implements try over a protected expression
// whose fallible points are not represented by a single dedicated path:
// several language-defined checks, checks nested inside a Result carrier,
// dynamic-array, slice, and list indexing, and constrained conversions.
//
// Rules:
//   - rules/errors/errorhandling.md — §9, §11 "Compiler-internal failure sets", §11.1, §12.1, §16
//   - rules/errors/runtime_checks.md — "What try converts", "Propagation rule", "Errorhandling revision-2 integration"
func (a *Analyzer) inferFailureSetTryExpression(expr *ast.TryExpression, successType Type, points []TryFailurePoint) (Type, expressionValue) {
	result := expressionValue{Display: expr.String()}
	members := tryFailureSetTypes(points)
	if len(expr.Handlers) == 0 {
		if !a.checkFailureSetPropagation(expr, points) {
			return successType, result
		}
		a.resolvedTries[expr] = ResolvedTry{
			Kind: ResolvedTryFailureSetPropagation, SuccessType: successType, ErrorType: failureSetErrorType(members),
			EnclosingResultType: a.currentFunctionReturn, Failures: append([]TryFailurePoint(nil), points...),
		}
		a.commitTryFailurePoints(points)
		return successType, result
	}

	errorType := failureSetErrorType(members)
	var context *tryFailureSetContext
	if len(members) > 1 {
		context = &tryFailureSetContext{members: members}
		for _, member := range members {
			if member.Kind == ErrorRootType {
				root := member
				context.commonChannel = &root
			}
		}
	}
	previous := a.tryFailureSet
	a.tryFailureSet = context
	plan, valid := a.analyzeTryHandlers(expr, Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{successType, errorType}})
	a.tryFailureSet = previous
	a.resolvedTries[expr] = ResolvedTry{
		Kind: ResolvedTryHandledFailureSet, SuccessType: successType, ErrorType: errorType,
		Failures: append([]TryFailurePoint(nil), points...),
	}
	if valid {
		plan.Failures = append([]TryFailurePoint(nil), points...)
		a.resolvedTryPlans[expr] = plan
		a.commitTryFailurePoints(points)
	}
	return successType, result
}

// failureSetErrorType is the single error type of a homogeneous failure set,
// or the open error root used only to analyze handler patterns of a
// heterogeneous set; it is never presented as an inferred public type.
func failureSetErrorType(members []Type) Type {
	if len(members) == 1 {
		return members[0]
	}
	return Type{Name: "error", Kind: ErrorRootType}
}

// checkFailureSetPropagation requires every failure of a naked try to be
// assignable to the enclosing Result error channel, naming the source
// operation of each failure that is not.
func (a *Analyzer) checkFailureSetPropagation(expr *ast.TryExpression, points []TryFailurePoint) bool {
	switch {
	case a.inDeferBlock:
		a.addBodylessTryError(expr.Token, "bodyless try cannot propagate from inside defer; add a local try handler")
		return false
	case !a.inFunctionBody:
		a.addBodylessTryError(expr.Token, "bodyless try cannot propagate outside a function; add a local try handler")
		return false
	case a.currentFunctionReturn.Kind != ResultType || len(a.currentFunctionReturn.TypeArgs) != 2:
		members := tryFailureSetTypes(points)
		a.addBodylessTryError(expr.Token, "bodyless try propagates %s with return Err, but this function returns %s; add a local try handler or return a Result whose error channel accepts them",
			tryFailureSetDisplay(members), typeDisplayName(a.currentFunctionReturn))
		return false
	}
	channel := a.currentFunctionReturn.TypeArgs[1]
	valid := true
	reported := []Type{}
	for _, point := range points {
		if canInitialize(channel, point.ErrorType, expr.Expression) {
			continue
		}
		already := false
		for _, typ := range reported {
			if sameConcreteType(typ, point.ErrorType) {
				already = true
			}
		}
		if already {
			continue
		}
		reported = append(reported, point.ErrorType)
		a.addErrorAtTokenWithMetadataAndPrevious(expr.Token, point.Token, diagnostics.TryPropagationIncompatible,
			"Each failure a bodyless try protects must fit the function's error channel; handle the others locally or split the expression.",
			"bodyless try propagates %s from %s with return Err, but this function returns %s; handle it locally with Err(_) or map %s to %s",
			typeDisplayName(point.ErrorType), point.Expression.String(), typeDisplayName(a.currentFunctionReturn),
			typeDisplayName(point.ErrorType), typeDisplayName(channel))
		valid = false
	}
	return valid
}

// commitTryFailurePoints removes the ordinary panic effect of every protected
// check and marks indexed plans as fallible once the try construct is valid.
func (a *Analyzer) commitTryFailurePoints(points []TryFailurePoint) {
	for _, point := range points {
		switch point.Kind {
		case TryFailureArithmetic:
			a.resolveArithmeticFailureEffect(point.Expression)
		case TryFailureContract:
			a.resolveContractFailureEffect(point.Expression.(*ast.CallExpression))
		case TryFailureBounds:
			index := point.Expression.(*ast.IndexExpression)
			a.callGraph.removeEffect(a.currentCallable, EffectMayPanicBounds, expressionToken(index))
			if plan, ok := a.resolvedArrayIndexPlans[index]; ok {
				a.commitFallibleBoundsIndex(index, plan, point.ErrorType)
			}
			if plan, ok := a.resolvedListIndexPlans[index]; ok {
				plan.FailureMode = ArrayIndexFailureFallible
				plan.ErrorType = point.ErrorType
				a.recordResolvedListIndexPlan(index, plan)
			}
		}
	}
}

// failureSetMember returns the member of the active heterogeneous failure
// set that a narrowing pattern names.
func (c *tryFailureSetContext) member(typ Type) (Type, bool) {
	for _, member := range c.members {
		if sameConcreteType(member, typ) {
			return member, true
		}
	}
	return Type{}, false
}

// rejectHeterogeneousErrorBinding enforces §11.1: Err(name) needs one
// compiler-resolved binding type, which a heterogeneous failure set provides
// only through an explicitly declared common channel.
func (a *Analyzer) rejectHeterogeneousErrorBinding(token lexer.Token, name string) bool {
	if a.tryFailureSet == nil || a.tryFailureSet.commonChannel != nil {
		return false
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.HeterogeneousTryErrorBinding,
		"Use Err(_) to handle every failure without binding it, split the expression into separate try expressions, or map the failures explicitly.",
		"Err(%s) cannot bind the failures of this try because they have different types (%s) and no common declared error channel",
		name, tryFailureSetDisplay(a.tryFailureSet.members))
	return true
}

// failureSetResidualPropagates checks, member by member, that the failures
// left unhandled by partial handlers of a heterogeneous try can propagate.
func (a *Analyzer) failureSetResidualPropagates(expr *ast.TryExpression, matched map[string]lexer.Token) bool {
	valid := true
	for _, member := range a.tryFailureSet.members {
		missing := []string{}
		if member.Kind == EnumType && len(member.EnumValues) > 0 {
			for _, variant := range member.EnumValues {
				if _, covered := matched[typeDisplayName(member)+"."+variant]; !covered {
					missing = append(missing, typeDisplayName(member)+"."+variant)
				}
			}
			if len(missing) == 0 {
				continue
			}
		}
		if !a.checkTryResidualPropagation(expr, member, missing) {
			valid = false
		}
	}
	return valid
}

// addBodylessTryError reports a bodyless try that cannot propagate. Inside a
// handler body or guard of another try it also explains that the enclosing
// try does not catch these failures.
//
// Rules:
//   - rules/errors/errorhandling.md — §22 "Handler failures are outside the protected set", §30 "Diagnostics must act as a mentor"
func (a *Analyzer) addBodylessTryError(token lexer.Token, format string, args ...any) {
	if a.tryHandlerDepth == 0 {
		a.addErrorAtTokenWithMetadata(token, diagnostics.TryPropagationIncompatible,
			"A bodyless try returns the failure from this function, so the function must return a compatible Result (or Option for None). Otherwise handle the failure locally with try handlers.",
			format, args...)
		return
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.TryPropagationIncompatible,
		"This try is inside a handler or guard of another try. The outer try protects only its own expression, so its handlers do not catch failures raised here; handle them with this try's own handlers or let them propagate.",
		format, args...)
}

// appendStringMaterializationPoint adds the allocation failure of a runtime
// string materialization plan rooted at expr and marks it as protected by the
// enclosing try.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 5.1–5.6, 5.15
func (a *Analyzer) appendStringMaterializationPoint(points []TryFailurePoint, expr ast.Expression, token lexer.Token) []TryFailurePoint {
	if !a.runtimeStringMaterialization(expr) {
		return points
	}
	a.protectedStringMaterializations[expr] = true
	return append(points, TryFailurePoint{Kind: TryFailureAllocation, ErrorType: a.types["AllocationError"], Expression: expr, Token: token})
}
