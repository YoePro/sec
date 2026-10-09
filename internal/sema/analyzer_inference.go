// Expression inference consumes resolved type/member and storage facts.
// Compiler-known Arena operations are owned by analyzer_arena.go, with their
// allocation, domain, dependency and epoch rules documented at each operation.
// Rules: rules/compiler/compiler_analysis.md — §2(3–8);
// rules/corrections/applied/correction25-20260823.md — Part II traceability.
package sema

import (
	"fmt"
	"math/big"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"strings"
)

// Move all infer functions to this file to rerduce the size of analyzer.go

func inferGenericTypeSubstitution(pattern Type, concrete Type, substitution map[string]Type) bool {
	return inferGenericTypeSubstitutionWithFixed(pattern, concrete, substitution, nil)
}

// inferGenericTypeSubstitutionWithFixed infers only parameters not fixed by an
// explicit positional prefix. A fixed parameter is a wildcard during inference;
// the instantiated signature performs the ordinary compatibility check later.
func inferGenericTypeSubstitutionWithFixed(pattern Type, concrete Type, substitution map[string]Type, fixed map[string]struct{}) bool {
	if pattern.Kind == GenericType {
		if _, ok := fixed[pattern.Name]; ok {
			return true
		}
		if existing, ok := substitution[pattern.Name]; ok {
			return sameConcreteType(existing, concrete)
		}
		substitution[pattern.Name] = concrete
		return true
	}

	if pattern.Kind == FunctionType || concrete.Kind == FunctionType {
		if pattern.Kind != FunctionType || concrete.Kind != FunctionType {
			return false
		}
		if normalizedCallableCapability(pattern.FunctionCapability) != normalizedCallableCapability(concrete.FunctionCapability) {
			return false
		}
		if len(pattern.FunctionParameterTypes) != len(concrete.FunctionParameterTypes) {
			return false
		}
		for i := range pattern.FunctionParameterTypes {
			if !inferGenericTypeSubstitutionWithFixed(pattern.FunctionParameterTypes[i], concrete.FunctionParameterTypes[i], substitution, fixed) {
				return false
			}
		}
		if pattern.FunctionReturnType == nil || concrete.FunctionReturnType == nil {
			return pattern.FunctionReturnType == nil && concrete.FunctionReturnType == nil
		}
		return inferGenericTypeSubstitutionWithFixed(*pattern.FunctionReturnType, *concrete.FunctionReturnType, substitution, fixed)
	}

	if pattern.Element != nil || concrete.Element != nil {
		if pattern.Element == nil || concrete.Element == nil || pattern.Kind != concrete.Kind ||
			(pattern.Kind == ArrayType && !sameArrayShape(pattern, concrete)) {
			return false
		}
		return inferGenericTypeSubstitutionWithFixed(*pattern.Element, *concrete.Element, substitution, fixed)
	}

	if len(pattern.TypeArgs) > 0 || len(concrete.TypeArgs) > 0 {
		if pattern.Name != concrete.Name || len(pattern.TypeArgs) != len(concrete.TypeArgs) {
			return false
		}
		for i := range pattern.TypeArgs {
			if !inferGenericTypeSubstitutionWithFixed(pattern.TypeArgs[i], concrete.TypeArgs[i], substitution, fixed) {
				return false
			}
		}
		return true
	}

	return canInitialize(pattern, concrete, nil)
}

func (a *Analyzer) inferFunctionValueCall(expr *ast.CallExpression, calleeType Type) (Type, expressionValue) {
	if calleeType.Kind == InvalidType {
		calleeType, _ = a.inferExpression(expr.Callee)
	}
	if calleeType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if calleeType.Kind != FunctionType || calleeType.FunctionReturnType == nil {
		a.addErrorAtToken(expr.Token, "cannot call %s", typeDisplayName(calleeType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if !functionTypeAcceptsArgumentCount(calleeType, len(expr.Arguments)) {
		expected := fmt.Sprintf("%d", len(calleeType.FunctionParameterTypes))
		if calleeType.FunctionVariadic {
			expected = fmt.Sprintf("at least %d", len(calleeType.FunctionParameterTypes)-1)
		}
		a.addErrorAtToken(expr.Token, "function value expects %s arguments, got %d", expected, len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	for i, arg := range expr.Arguments {
		argType, _ := a.inferExpression(arg)
		if argType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		expected, parameterOK := functionTypeParameterForArgument(calleeType, i)
		if !parameterOK {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if !a.canInitialize(expected, argType, arg) {
			a.addErrorAtToken(expressionToken(arg), "argument %d must be %s, got %s", i+1, typeDisplayName(expected), typeDisplayName(argType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
	}
	a.recordFunctionValueCall(expr, calleeType)

	return *calleeType.FunctionReturnType, expressionValue{Display: expr.String()}
}

// inferAvailabilityExpression resolves an ownership-state query without
// reading the tested Place. This permits querying an unavailable or
// uninitialized Place while keeping availability distinct from Option, null,
// borrow authority, and device state.
//
// Rules:
//   - rules/memory/ownership.md — §5 availability states and §21 availability tests
//   - rules/memory/copy_move.md — §24 "Availability tests and copy/move"
//   - rules/corrections/applied/correction30-20260828.md — §§1–3
func (a *Analyzer) inferAvailabilityExpression(expr *ast.AvailabilityExpression) (Type, expressionValue) {
	place, ok := a.resolvePlace(expr.Place)
	if !ok || !place.Addressable {
		a.addErrorAtToken(expr.Token, "is available requires an addressable ownership Place")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	a.bindAvailabilityPlaceRoot(expr.Place)
	known, available := a.currentPlaceAvailability(place)
	value := available
	if expr.Negated {
		value = !value
	}
	a.resolvedAvailabilityTests[expr] = ResolvedAvailabilityTest{
		Place: place, Negated: expr.Negated, StaticallyKnown: known, Value: value,
	}
	return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferExpression(expr ast.Expression) (Type, expressionValue) {
	typ, value := a.inferExpressionUnrecorded(expr)
	if expr != nil {
		a.expressionTypes[expr] = typ
		a.recordResolvedOperator(expr, typ)
		a.recordResolvedOperatorEffect(expr)
	}
	return typ, value
}

func (a *Analyzer) inferExpressionUnrecorded(expr ast.Expression) (Type, expressionValue) {
	if expr == nil {
		// Editor parsing may leave an incomplete expression while the user is
		// typing. Treat it as invalid so semantic features can still respond.
		return Type{Kind: InvalidType}, expressionValue{}
	}

	switch expr := expr.(type) {
	case *ast.InvalidExpression:
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case *ast.InvalidPattern:
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case *ast.IntegerLiteral:
		switch expr.Suffix() {
		case "u":
			return Type{Name: "uint", Kind: UintType}, expressionValue{Display: expr.String()}
		case "g":
			return a.types["float"], expressionValue{Display: expr.String()}
		case "m":
			return Type{Name: "decimal", Kind: DecimalType}, expressionValue{Display: expr.String()}
		case "t":
			if !a.validCharScalarLiteral(expr) {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			return Type{Name: "char", Kind: CharType}, expressionValue{Display: expr.String()}
		case "r":
			if !a.validUnicodeScalarLiteral(expr) {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			return Type{Name: "rune", Kind: RuneType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "int", Kind: IntType}, expressionValue{Display: expr.String()}
	case *ast.FloatLiteral:
		switch expr.Suffix() {
		case "g":
			return a.types["float"], expressionValue{Display: expr.String()}
		case "m":
			return Type{Name: "decimal", Kind: DecimalType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "decimal", Kind: DecimalType}, expressionValue{Display: expr.String()}
	case *ast.StringLiteral:
		return Type{Name: "string", Kind: StringType}, expressionValue{Display: expr.String()}
	case *ast.InterpolatedStringLiteral:
		return a.inferInterpolatedStringLiteral(expr)
	case *ast.CharLiteral:
		if !validCharLiteral(expr.Token.Lexeme) {
			a.addErrorAtToken(expr.Token, "character literal must contain exactly one character")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		// rules/types/types.md — "Character literal": rune by default,
		// independently of the scalar's value (MD-043).
		return Type{Name: "rune", Kind: RuneType}, expressionValue{Display: expr.String()}
	case *ast.BooleanLiteral:
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	case *ast.Identifier:
		if typ, value, handled := a.inferCompilerKnownValue(expr); handled {
			return typ, value
		}
		if a.isNullSentinel(expr) {
			return a.inferNullSentinel(expr)
		}
		symbol, ok := a.symbols[expr.Value]
		if !ok {
			if functions := a.accessibleFunctions(a.functions[expr.Value]); len(functions) > 0 {
				if len(functions) == 1 {
					a.bindDefinition(expr.Token, functions[0].Token)
					a.recordNamedCallableIdentity(expr, functions[0])
					return functionTypeFromFunction(functions[0]), expressionValue{Display: expr.String()}
				}
				a.addErrorAtToken(expr.Token, "ambiguous function value %s; explicit function type required", expr.Value)
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			if a.inLambda {
				if _, outer := a.lambdaOuterSymbols[expr.Value]; outer {
					a.addErrorAtToken(expr.Token, "lambda cannot access outer variable %s without explicit capture", expr.Value)
					return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
				}
			}
			if a.currentImplTarget != "" {
				if target, ok := a.types[a.currentImplTarget]; ok {
					if property, ok := a.resolveReadableProperty(target, expr.Value, expr.Token); ok {
						a.recordResolvedPropertyRead(expr, target, property)
						return property.Type, expressionValue{Display: expr.String()}
					}
				}
			}
			a.addErrorAtToken(expr.Token, "undefined variable %s", expr.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		a.bindDefinition(expr.Token, symbol.Token)
		// rules/declarations/properties.md, Read access; correction12.md.
		// Implicit property symbols remain available for assignment lookup, but an
		// expression read must independently require a getter.
		if symbol.ImplicitMember {
			if property, propertyOK := a.lookupCurrentImplProperty(expr.Value); propertyOK {
				if !property.HasGetter {
					a.addErrorAtToken(expr.Token, "property %s has no getter", expr.Value)
					return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
				}
				a.recordResolvedPropertyRead(expr, a.types[a.currentImplTarget], property)
			}
			if symbol.RegisterAccess == RegisterWriteOnly {
				a.addErrorAtToken(expr.Token, "register field %s is write-only and cannot be read", expr.Value)
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
		}
		if assigned, ok := a.assigned[expr.Value]; ok && !assigned {
			a.addErrorAtToken(expr.Token, "variable %s is unassigned", expr.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if place, placeOK := a.resolvePlace(expr); placeOK && a.checkPlaceAvailableForRead(place, expr.Token) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if a.checkStaleArenaReference(symbol, expr.Token) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if a.suppressPlaceRootRead == 0 && a.checkBorrowedRead(expr.Value, expr.Token) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		a.recordDeferCapture(expr.Value, symbol, expr.Token)
		if identity, known := a.effectiveCallableIdentity(expr.Value, symbol); known {
			a.resolvedCallableIdentities[expr] = identity
		}
		return symbol.Type, expressionValue{Display: expr.String()}
	case *ast.PrefixExpression:
		return a.inferPrefixExpression(expr)
	case *ast.AvailabilityExpression:
		return a.inferAvailabilityExpression(expr)
	case *ast.StateTestExpression:
		return a.inferStateTestExpression(expr)
	case *ast.OptionBindingTestExpression:
		return a.inferOptionBindingTestExpression(expr)
	case *ast.NullTestExpression:
		return a.inferNullTestExpression(expr)
	case *ast.InfixExpression:
		return a.inferInfixExpression(expr)
	case *ast.ConversionExpression:
		return a.inferConversionExpression(expr)
	case *ast.CallExpression:
		return a.inferCallExpression(expr)
	case *ast.NewExpression:
		return a.inferNewExpression(expr, false)
	case *ast.LambdaExpression:
		return a.inferLambdaExpression(expr)
	case *ast.RuntimeCallExpression:
		return a.inferRuntimeCallExpression(expr)
	case *ast.OkExpression:
		a.addErrorAtToken(expr.Token, "Ok can only be returned from Result-returning function")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case *ast.ErrExpression:
		a.addErrorAtToken(expr.Token, "Err can only be returned from Result-returning function")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case *ast.TryExpression:
		return a.inferTryExpression(expr)
	case *ast.SpawnExpression:
		return a.inferSpawnExpression(expr)
	case *ast.AwaitExpression:
		return a.inferAwaitExpression(expr)
	case *ast.MatchExpression:
		return a.inferMatchExpression(expr)
	case *ast.SpreadExpression:
		a.addErrorAtToken(expr.Token, "spread operator is not valid as a standalone expression")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case *ast.MemberExpression:
		if root := compilerKnownReceiverRoot(expr.Object); root != nil && a.checkArenaBackingBorrowRead(root.Value, expr.Property.Token) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		typ, ok := a.inferMemberExpression(expr)
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if place, ok := a.resolvePlace(expr); ok {
			if a.checkPlaceAvailableForRead(place, expr.Property.Token) || a.checkBorrowedReadPlace(place, expr.Property.Token) {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			a.recordDeferPlace(place, expr.Property.Token)
		}
		return typ, expressionValue{Display: expr.String()}
	case *ast.ArrayLiteral:
		return a.inferArrayLiteral(expr)
	case *ast.IndexExpression:
		typ, value := a.inferIndexExpression(expr)
		if typ.Kind != InvalidType {
			if place, ok := a.resolvePlace(expr); ok {
				if a.checkBorrowedReadPlace(place, expr.Token) {
					return Type{Kind: InvalidType}, value
				}
				a.recordDeferPlace(place, expr.Token)
			}
		}
		return typ, value
	case *ast.SliceExpression:
		typ, value := a.inferSliceExpression(expr)
		if typ.Kind != InvalidType {
			if place, ok := a.resolvePlace(expr); ok {
				if a.checkBorrowedReadPlace(place, expr.Token) {
					return Type{Kind: InvalidType}, value
				}
				a.recordDeferPlace(place, expr.Token)
			}
		}
		return typ, value
	case *ast.RefExpression:
		return a.inferRefExpression(expr)
	case *ast.StructLiteral:
		return a.inferStructLiteral(expr)
	case *ast.CollectionLiteral:
		return a.inferCollectionLiteral(expr)
	case *ast.RangeExpression:
		// Ranges are contextual: for, membership, slicing, switch cases,
		// array literal segments, and Append on an owning dynamic array
		// handle them before ordinary inference. Anywhere else a range would
		// be a first-class value, which Sec 0.1 does not have.
		a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.ArrayRangeSegmentInvalid,
			"Ranges appear only in for loops, membership tests, slicing, switch cases, array literals, and Append on an owning dynamic array.",
			"a range is not a value in Sec 0.1")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	default:
		return Type{Kind: InvalidType}, expressionValue{}
	}
}

// inferInterpolatedStringLiteral analyzes every embedded expression with the
// ordinary expression rules while retaining string as the type of the complete
// literal. This records definition, type, ownership, and effect facts at the
// parser-preserved nested source positions and lets ordinary diagnostics reach
// editor and compiler clients.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §14.3 "Interpolated strings"
//   - rules/tooling/lsp.md — "Incomplete-source handling"
func (a *Analyzer) inferInterpolatedStringLiteral(expr *ast.InterpolatedStringLiteral) (Type, expressionValue) {
	plan := ResolvedInterpolationPlan{}
	valid := true
	// rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md
	// § 6.12: a candidate retained after L1021 is not a string value; the
	// lexer diagnostic already explains it, so no cascading error is added.
	if expr.Malformed {
		for _, part := range expr.Parts {
			if part.Expression != nil {
				a.inferExpression(part.Expression)
			}
		}
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	for sourceIndex, part := range expr.Parts {
		if part.Expression == nil {
			continue
		}
		valueType, _ := a.inferExpression(part.Expression)
		if valueType.Kind == InvalidType {
			valid = false
			continue
		}
		hole, ok := a.resolveInterpolationFormatter(sourceIndex, valueType)
		if !ok {
			a.addErrorAtTokenWithMetadata(
				expressionToken(part.Expression),
				diagnostics.OperatorInvalidInterpolationValue,
				"Define an exact shared fn ToString() Result[string, StringError] method or interpolate a supported printable value.",
				"%s has no canonical interpolation formatting contract",
				typeDisplayName(valueType),
			)
			valid = false
			continue
		}
		plan.Holes = append(plan.Holes, hole)
		if hole.UserFunction != nil && !a.summaryPass && a.callGraphPathReachable {
			a.callGraph.addCall(a.currentCallable, *hole.UserFunction, part.Token, CallDispatchStaticMethod, CallExecutionSynchronous)
		}
	}
	if valid {
		a.resolvedInterpolationPlans[expr] = plan
		a.recordInterpolationStringConcatPlan(expr, plan)
	}
	return Type{Name: "string", Kind: StringType}, expressionValue{Display: expr.String()}
}

// inferExpressionWithExpected applies contextual literal shaping and records
// the resolved expression type consumed by Semantic IR and other clients.
//
// Rules:
//   - rules/types/types.md — "Context shaping" and "Character literal"
//   - rules/foundations/lexical_structure.md — §13 "Character literals"
func (a *Analyzer) inferExpressionWithExpected(expr ast.Expression, expected Type) (Type, expressionValue) {
	if tryExpr, ok := expr.(*ast.TryExpression); ok && tryExpr.Expression != nil {
		a.expectedExpressionTypes[tryExpr.Expression] = expected
		defer delete(a.expectedExpressionTypes, tryExpr.Expression)
	}
	if expr != nil {
		a.expectedExpressionTypes[expr] = expected
		defer delete(a.expectedExpressionTypes, expr)
	}
	if typ, shaped := a.shapeCharacterLiteral(expr, expected); shaped {
		return typ, expressionValue{Display: expr.String()}
	}
	if lit, ok := expr.(*ast.ArrayLiteral); ok {
		// Record the contextually typed literal like every other inferred
		// expression; later provenance and ownership passes read this fact
		// and must not re-infer an empty literal without its context.
		typ, value := a.inferArrayLiteralWithExpected(lit, expected)
		a.expressionTypes[expr] = typ
		return typ, value
	}
	if typ, value, ok := a.inferExpectedUnionVariantExpression(expr, expected); ok {
		a.expressionTypes[expr] = typ
		return typ, value
	}
	call, ok := expr.(*ast.CallExpression)
	if !ok || expected.Kind == InvalidType || expected.Kind == "" {
		typ, value := a.inferExpression(expr)
		return a.typeWithExpectedDimension(expr, typ, value, expected)
	}
	if typ, value, ok := a.inferCallAsUnionVariantConstructor(call, &expected); ok {
		a.expressionTypes[expr] = typ
		return typ, value
	}
	if callExpressionName(call) == "" {
		return a.inferExpression(expr)
	}
	if typ, value, ok := a.inferCallExpressionWithExpected(call, expected); ok {
		return typ, value
	}
	typ, value := a.inferExpression(expr)
	return a.typeWithExpectedDimension(expr, typ, value, expected)
}

func (a *Analyzer) inferSpawnExpression(expr *ast.SpawnExpression) (Type, expressionValue) {
	if expr.Body != nil {
		a.addErrorAtToken(expr.Token, "spawn block syntax is deprecated; spawn requires a callable expression")
		a.withCancellableContext(func() {
			a.analyzeBlockStatements(expr.Body)
		})
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if expr.Value == nil {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	switch expr.Value.(type) {
	case *ast.CallExpression, *ast.LambdaExpression:
	default:
		a.addErrorAtToken(expressionToken(expr.Value), "spawn requires a callable expression")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	execution, graphSpawn := spawnExecutionRelation(expr.Kind)
	spawnedType, _ := a.inferSpawnValue(expr.Value, execution, graphSpawn)
	if spawnedType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	returnType := spawnedType
	if spawnedType.Kind == FunctionType {
		if spawnedType.FunctionReturnType == nil {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		returnType = *spawnedType.FunctionReturnType
	}
	switch expr.Kind {
	case "", "task":
		return taskType(returnType), expressionValue{Display: expr.String()}
	case "thread":
		return threadType(returnType), expressionValue{Display: expr.String()}
	case "process":
		a.addErrorAtToken(expr.Token, "spawn process is not implemented yet")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	default:
		a.addErrorAtToken(expr.Token, "unknown spawn kind %s", expr.Kind)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
}

func (a *Analyzer) inferSpawnValue(expr ast.Expression, execution CallExecutionRelation, graphSpawn bool) (Type, expressionValue) {
	call, isCall := expr.(*ast.CallExpression)
	if !isCall {
		return a.inferExpressionInCancellableContext(expr)
	}
	previousCall := a.spawnCallExpression
	previousExecution := a.spawnCallExecution
	a.spawnCallExpression = call
	if graphSpawn {
		a.spawnCallExecution = execution
	} else {
		a.spawnCallExecution = ""
	}
	defer func() {
		a.spawnCallExpression = previousCall
		a.spawnCallExecution = previousExecution
	}()
	return a.inferExpressionInCancellableContext(expr)
}

func (a *Analyzer) inferExpressionInCancellableContext(expr ast.Expression) (Type, expressionValue) {
	var typ Type
	var value expressionValue
	a.withCancellableContext(func() {
		typ, value = a.inferExpression(expr)
	})
	return typ, value
}

func (a *Analyzer) inferAwaitExpression(expr *ast.AwaitExpression) (Type, expressionValue) {
	if a.inDeferBlock {
		a.addErrorAtToken(expr.Token, "await is not allowed inside defer")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if expr.Value == nil {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	a.checkLiveMutexGuardsAcrossBoundary(expr.Token, "await")
	a.recordBlockingOperation("await", expr.Token)
	valueType, _ := a.inferExpression(expr.Value)
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if !isTaskType(valueType) {
		a.addErrorAtToken(expressionToken(expr.Value), "await requires Task[T], got %s", typeDisplayName(valueType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	a.markMoveSource(expr.Value)
	// rules/concurrency/await.md §§2, 5, and 17: await consumes Task[T] but
	// never erases cancellation, panic, or task-execution failure. The complete
	// task return T remains nested in TaskOutcome[T]. Runtime commit/cancellation
	// cleanup remains a later Semantic IR and lowering responsibility.
	return a.intrinsicGenericType("TaskOutcome", valueType.TypeArgs[0]), expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferExpectedUnionVariantExpression(expr ast.Expression, expected Type) (Type, expressionValue, bool) {
	if expected.Kind != UnionType {
		return Type{}, expressionValue{}, false
	}
	switch expr := expr.(type) {
	case *ast.Identifier:
		variant, ok := lookupUnionVariant(expected, expr.Value)
		if !ok {
			return Type{}, expressionValue{}, false
		}
		if variant.Payload != nil || len(variant.PayloadFields) > 0 {
			a.addErrorAtToken(expr.Token, "union variant %s.%s requires payload", typeDisplayName(expected), variant.Name)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return expected, expressionValue{Display: expr.String()}, true
	case *ast.MemberExpression:
		// rules/declarations/unions.md and rules/types/types.md: an expected
		// generic union type supplies the missing type arguments for a
		// qualified payload-less constructor such as Option.None. Resolve the
		// qualifier here instead of first producing the open Option type.
		owner, ok := typePathFromExpression(expr.Object)
		if !ok || a.resolveTypeName(owner) != expected.Name {
			return Type{}, expressionValue{}, false
		}
		variant, ok := lookupUnionVariant(expected, expr.Property.Value)
		if !ok {
			return Type{}, expressionValue{}, false
		}
		if variant.Payload != nil || len(variant.PayloadFields) > 0 {
			a.addErrorAtToken(expr.Property.Token, "union variant %s.%s requires payload", typeDisplayName(expected), variant.Name)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.bindDefinition(expr.Property.Token, variant.Token)
		return expected, expressionValue{Display: expr.String()}, true
	case *ast.CallExpression:
		name := callExpressionName(expr)
		if name == "" {
			return Type{}, expressionValue{}, false
		}
		variant, ok := lookupUnionVariant(expected, name)
		if !ok {
			return Type{}, expressionValue{}, false
		}
		if len(variant.PayloadFields) > 0 {
			a.addErrorAtToken(expr.Token, "union variant %s.%s requires named payload fields", typeDisplayName(expected), variant.Name)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if variant.Payload == nil {
			if len(expr.Arguments) != 0 {
				a.addErrorAtToken(expr.Token, "union variant %s.%s expects 0 arguments, got %d", typeDisplayName(expected), variant.Name, len(expr.Arguments))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
			return expected, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "union variant %s.%s expects 1 argument, got %d", typeDisplayName(expected), variant.Name, len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		payloadType := *variant.Payload
		valueType, _ := a.inferOwningConstructionValue(expr.Arguments[0], payloadType)
		if valueType.Kind != InvalidType && !a.canInitialize(payloadType, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "union variant %s.%s payload must be %s, got %s", typeDisplayName(expected), variant.Name, typeDisplayName(payloadType), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if valueType.Kind != InvalidType && a.checkCompileTimeContractExpression(payloadType, expr.Arguments[0]) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if valueType.Kind != InvalidType && !a.validateOwningConstructionSource(expr.Arguments[0]) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return expected, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func (a *Analyzer) inferStructLiteral(expr *ast.StructLiteral) (Type, expressionValue) {
	if unionType, value, unionOK := a.inferStructLiteralAsUnionVariant(expr); unionOK {
		return unionType, value
	}

	// rules/types/default_values.md: a materialized default already carries
	// its exact (possibly generic-instance) struct type.
	typ, ok := a.synthesizedStructTypes[expr]
	if !ok {
		typ, ok = a.resolveType(expr.Type)
	}
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	// rules/concurrency/mutex.md §13: representation is runtime-private,
	// with no source constructor defined by temporal.md §4.
	if typ.MonotonicPoint {
		a.addErrorAtToken(expr.Token, "Instant has no source struct constructor; its representation is runtime-private")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if typ.Kind != StructType {
		a.addErrorAtToken(expr.Token, "%s is not a struct type", typ.Name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	seen := map[string]lexer.Token{}
	fieldIDs := map[string]uint32{}
	finalFields := make([]ResolvedStructFinalField, len(typ.Fields))
	for index, field := range typ.Fields {
		fieldIDs[field.Name] = uint32(index)
		finalFields[index] = ResolvedStructFinalField{
			FieldID: uint32(index), FieldName: field.Name, FieldType: field.Type, SourceEntryIndex: -1,
		}
	}
	entries := make([]ResolvedStructEntry, 0, len(expr.Fields))
	planValid := true
	ownershipValid := true
	spreadSuppliesFields := false
	for sourceIndex, field := range expr.Fields {
		if field.Spread {
			spreadType, _ := a.inferExpression(field.Value)
			if spreadType.Kind == InvalidType {
				planValid = false
				continue
			}
			if !sameConcreteType(typ, spreadType) {
				a.addErrorAtToken(field.Token, "cannot spread %s into %s; spread source must have type %s", typeDisplayName(spreadType), typeDisplayName(typ), typeDisplayName(typ))
				planValid = false
				continue
			}
			if !implicitlyCopyable(spreadType) {
				a.addErrorAtToken(field.Token, "cannot spread %s into %s; %s is not implicitly copyable", typeDisplayName(spreadType), typeDisplayName(typ), typeDisplayName(spreadType))
				planValid = false
				continue
			}
			entries = append(entries, ResolvedStructEntry{SourceIndex: sourceIndex, Kind: StructEntrySpread, Expression: field.Value, Type: spreadType})
			for index, declared := range typ.Fields {
				// rules/declarations/spread.md: a spread never overwrites an
				// explicit field, but a later spread replaces an earlier spread.
				if _, explicit := seen[declared.Name]; explicit {
					continue
				}
				finalFields[index].SourceKind = StructFieldSourceSpread
				finalFields[index].SourceEntryIndex = sourceIndex
				finalFields[index].SpreadFieldID = uint32(index)
				finalFields[index].Action = resolvedStructCopyAction(declared.Type)
			}
			// rules/declarations/spread.md; correction14.md permits only a fully
			// validated spread to suppress omitted-field diagnostics.
			spreadSuppliesFields = true
			continue
		}
		if _, exists := seen[field.Name.Value]; exists {
			a.addErrorAtToken(field.Name.Token, "duplicate field %q in struct literal %s", field.Name.Value, typ.Name)
			planValid = false
			continue
		}
		seen[field.Name.Value] = field.Name.Token

		fieldType, ok := lookupStructField(typ, field.Name.Value)
		if !ok {
			a.addErrorAtToken(field.Name.Token, "unknown field %q in struct %s", field.Name.Value, typ.Name)
			planValid = false
			continue
		}
		if !a.canAccessStructField(typ, field.Name.Value) {
			a.addErrorAtToken(field.Name.Token, "field %s on %s is not accessible from module %s", field.Name.Value, typeDisplayName(typ), moduleDisplayName(a.currentModule))
			planValid = false
			continue
		}
		if definition, exists := memberDefinitionToken(typ, field.Name.Value); exists {
			a.bindDefinition(field.Name.Token, definition)
		}

		// rules/types/types.md and rules/declarations/unions.md: aggregate
		// fields provide the expected union type, allowing canonical contextual
		// constructors such as None or None() for Option[T].
		valueType, _ := a.inferOwningConstructionValue(field.Value, fieldType)
		if valueType.Kind != InvalidType && !a.canInitialize(fieldType, valueType, field.Value) {
			a.addErrorAtToken(expressionToken(field.Value), "cannot initialize field %s with %s", field.Name.Value, typeDisplayName(valueType))
			planValid = false
			continue
		}
		if valueType.Kind == InvalidType {
			planValid = false
			continue
		}
		if a.checkCompileTimeContractExpression(fieldType, field.Value) {
			planValid = false
		}
		if !a.validateOwningConstructionSource(field.Value) {
			planValid = false
			ownershipValid = false
			continue
		}
		fieldID := fieldIDs[field.Name.Value]
		entries = append(entries, ResolvedStructEntry{SourceIndex: sourceIndex, Kind: StructEntryExplicit, FieldName: field.Name.Value, FieldID: fieldID, Expression: field.Value, Type: valueType})
		finalFields[fieldID].SourceKind = StructFieldSourceExplicit
		finalFields[fieldID].SourceEntryIndex = sourceIndex
		finalFields[fieldID].Action = resolvedStructExpressionAction(field.Value, fieldType)
	}
	for index, field := range typ.Fields {
		if finalFields[index].SourceKind != "" || isEventType(field.Type) {
			continue
		}
		// A syntactically supplied explicit field may already have its own type
		// diagnostic. Do not cascade with a misleading omitted-field error.
		if _, supplied := seen[field.Name]; supplied {
			planValid = false
			continue
		}
		resolution := DefaultValueOf(field.Type)
		if resolution.Kind == NoDefault {
			start := len(a.errors)
			// rules/types/default_values.md, "Diagnostics": an omitted field
			// whose type default is invalid or ambiguous keeps that specific
			// identity instead of the generic non-defaultable-field diagnostic.
			if field.Type.InvalidExplicitDefault {
				a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.InvalidDefaultedField, "initialize the field explicitly or correct the explicit default of "+typeDisplayName(field.Type), "omitted field %q in struct %s cannot be default-initialized because the explicit default of %s is invalid", field.Name, typ.Name, typeDisplayName(field.Type))
			} else if id, help, reason, ambiguous := noDefaultDiagnostic(field.Type); ambiguous {
				a.addErrorAtTokenWithMetadata(expr.Token, id, "initialize the field explicitly or "+help, "omitted field %q in struct %s must be initialized because %s", field.Name, typ.Name, reason)
			} else {
				a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.MissingNonDefaultableField, "initialize the field explicitly", "field %q in struct %s has no default value and must be initialized", field.Name, typ.Name)
			}
			a.relateMissingDefault(start, field.Type, field.Token)
			planValid = false
			continue
		}
		finalFields[index].SourceKind = StructFieldSourceDefault
		finalFields[index].Action = StructFieldConstructDirect
		finalFields[index].Default = resolution
	}
	if planValid {
		a.resolvedStructLiteralPlans[expr] = ResolvedStructLiteralPlan{
			StructType: typ, Entries: entries, FinalFields: finalFields, FullyInitialized: true,
		}
	}
	if !ownershipValid {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	// A valid same-type spread supplies every direct field. Without one, apply
	// semantic defaults only after all explicit entries have been resolved.
	if a.legacyDefaultAST && !spreadSuppliesFields {
		for _, field := range typ.Fields {
			if isEventType(field.Type) {
				continue
			}
			if _, supplied := seen[field.Name]; supplied {
				continue
			}
			resolution := DefaultValueOf(field.Type)
			value := defaultExpression(resolution, field.Type, expr.Token)
			if value == nil {
				continue
			}
			a.recordSynthesizedDefaultTypes(value, field.Type)
			expr.Fields = append(expr.Fields, &ast.StructLiteralField{Token: expr.Token, Name: &ast.Identifier{Token: field.Token, Value: field.Name}, Value: value})
		}
	}

	return typ, expressionValue{Display: expr.String()}
}

// inferOwningConstructionValue admits expression-level <- exactly where an
// aggregate or union payload owns its value. The surrounding return context,
// when present, is inherited by nested construction.
func (a *Analyzer) inferOwningConstructionValue(expr ast.Expression, expected Type) (Type, expressionValue) {
	a.constructionOwnershipDepth++
	defer func() { a.constructionOwnershipDepth-- }()
	return a.inferExpressionWithExpected(expr, expected)
}

func (a *Analyzer) inferStructLiteralAsUnionVariant(expr *ast.StructLiteral) (Type, expressionValue, bool) {
	if expr.Type == nil || !strings.Contains(expr.Type.Name, ".") {
		return Type{}, expressionValue{}, false
	}

	unionName, variantName, ok := splitUnionVariantTypeName(expr.Type.Name)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	unionName = a.resolveTypeName(unionName)
	unionType, ok := a.synthesizedStructTypes[expr]
	if !ok {
		unionType, ok = a.types[unionName]
	}
	if !ok || unionType.Kind != UnionType {
		return Type{}, expressionValue{}, false
	}

	variant, ok := lookupUnionVariant(unionType, variantName)
	if !ok {
		a.addErrorAtToken(expr.Token, "unknown union variant %s.%s", unionType.Name, variantName)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if len(variant.PayloadFields) == 0 {
		a.addErrorAtToken(expr.Token, "union variant %s.%s requires unnamed payload construction", typeDisplayName(unionType), variant.Name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	a.checkUnionPayloadFields(unionType, variant, expr, expr.Token)
	return unionType, expressionValue{Display: expr.String()}, true
}

// inferLambdaExpression validates a lambda, constructs its capture environment
// in the enclosing callable, and analyzes its body under a distinct callable
// identity and lexical capture scope.
//
// Rules:
//   - rules/declarations/lambda-functions.md — §§13–21 explicit captures and lambda bodies
//   - rules/analysis/closure_analysis.md — "Callable creation"
//   - rules/analysis/call_graph.md — "Callable node" and "Call-site record"
func (a *Analyzer) inferLambdaExpression(expr *ast.LambdaExpression) (Type, expressionValue) {
	returnType, ok := a.resolveType(expr.ReturnType)
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	params := make([]Type, 0, len(expr.Parameters))
	seenParams := map[string]lexer.Token{}
	for _, param := range expr.Parameters {
		if _, exists := seenParams[param.Name.Value]; exists {
			a.addErrorAtToken(param.Name.Token, "duplicate parameter %q", param.Name.Value)
			continue
		}
		a.checkConfusableInDomains(param.Name.Value, param.Name.Token, seenParams)
		seenParams[param.Name.Value] = param.Name.Token

		paramType, paramOK := a.resolveType(param.Type)
		if !paramOK {
			continue
		}
		params = append(params, paramType)
	}

	lambdaType := Type{
		Name:                   functionTypeName(params, returnType, CallableShared),
		Kind:                   FunctionType,
		FunctionParameterTypes: params,
		FunctionReturnType:     &returnType,
		FunctionCapability:     CallableShared,
	}
	if len(params) != len(expr.Parameters) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	a.validateLambdaCaptureOwnership(expr)
	a.recordLambdaCaptureFacts(expr)
	a.recordLambdaCallableIdentity(expr)
	lambdaCallable := CallableID("")
	if identity, exists := a.resolvedCallableIdentities[expr]; exists {
		lambdaCallable = a.callGraph.addClosureCallable(identity, a.currentModule)
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousFunctionName := a.currentFunctionName
	previousCallable := a.currentCallable
	previousFunctionReturn := a.currentFunctionReturn
	previousInFunctionBody := a.inFunctionBody
	previousInLambda := a.inLambda
	previousLambdaOuterSymbols := a.lambdaOuterSymbols
	previousLoopDepth := a.loopDepth
	previousTest := a.currentTest

	// rules/declarations/lambda-functions.md, capture eligibility; correction11.md
	// separates enclosing locals from ordinary module/type lookup. Non-local
	// declarations remain directly visible and cannot be captured.
	captureCandidates := map[string]Symbol{}
	a.symbols = map[string]Symbol{}
	a.constInts = map[string]*big.Int{}
	a.assigned = map[string]bool{}
	a.moved = copyMoved(previousMoved)
	a.moveReasons = copyMoveReasons(previousMoveReasons)
	for _, capture := range expr.Captures {
		if capture.Name != nil {
			clearRootPlaceStateMaps(a.moved, a.moveReasons, capture.Name.Value)
		}
	}
	for name, symbol := range previousSymbols {
		if symbol.Local {
			captureCandidates[name] = symbol
			continue
		}
		a.symbols[name] = symbol
		a.assigned[name] = previousAssigned[name]
		if value, ok := previousConstInts[name]; ok {
			a.constInts[name] = new(big.Int).Set(value)
		}
	}
	a.currentFunctionName = "lambda"
	a.currentFunctionReturn = returnType
	a.inFunctionBody = true
	previousTryHandlerDepth := a.tryHandlerDepth
	a.tryHandlerDepth = 0
	a.inLambda = true
	a.lambdaOuterSymbols = captureCandidates
	a.loopDepth = 0
	// A lambda declared inside a test is its own callable boundary. Its return
	// statements follow the declared lambda return type, not test-return rules.
	a.currentTest = nil
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.moveReasons = previousMoveReasons
		a.currentFunctionName = previousFunctionName
		a.currentCallable = previousCallable
		a.currentFunctionReturn = previousFunctionReturn
		a.inFunctionBody = previousInFunctionBody
		a.tryHandlerDepth = previousTryHandlerDepth
		a.inLambda = previousInLambda
		a.lambdaOuterSymbols = previousLambdaOuterSymbols
		a.loopDepth = previousLoopDepth
		a.currentTest = previousTest
	}()

	a.defineLambdaCaptures(expr, captureCandidates, previousAssigned)
	// Environment construction executes in the enclosing callable. Only the
	// lambda body itself changes the active call-graph caller.
	a.currentCallable = lambdaCallable

	for i, param := range expr.Parameters {
		mutableBinding := !param.Ref && !param.MutableRef && params[i].Kind != ReferenceType
		if !a.defineSymbol(param.Name.Value, params[i], mutableBinding, param.Name.Token) {
			continue
		}
		a.assigned[param.Name.Value] = true
	}

	a.analyzeBlockStatements(expr.Body)

	if !a.blockDefinitelyReturns(expr.Body) && returnType.Kind != VoidType {
		a.addErrorAtToken(expr.Token, "lambda must return %s", typeDisplayName(returnType))
	}

	return lambdaType, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferMemberExpression(expr *ast.MemberExpression) (Type, bool) {
	if enumType, ok := a.inferEnumValueExpression(expr); ok {
		return enumType, true
	}
	if unionType, ok := a.inferUnionVariantExpression(expr); ok {
		return unionType, true
	}
	if staticType, ok := a.inferStaticMemberExpression(expr); ok {
		return staticType, true
	}

	objectType, _ := a.inferPlaceBase(expr.Object)
	if objectType.Kind == InvalidType {
		return Type{Kind: InvalidType}, false
	}
	objectType = dereferenceType(objectType)
	if definition, exists := memberDefinitionToken(objectType, expr.Property.Value); exists {
		a.bindDefinition(expr.Property.Token, definition)
	}

	if expr.Property.Value == "Ptr" || expr.Property.Value == "ptr" {
		member, exists := compilerKnownMember(objectType, expr.Property.Value, false)
		if !exists || member.Kind != CompilerKnownProperty {
			a.addErrorAtToken(expr.Property.Token, "unknown member %s on %s", expr.Property.Value, typeDisplayName(objectType))
			return Type{Kind: InvalidType}, false
		}
		a.compilerKnownMemberFacts[sourceTokenLocation(expr.Property.Token)] = member
		return a.inferPointerMember(expr, objectType)
	}

	if member, ok := compilerKnownMember(objectType, expr.Property.Value, false); ok && member.Kind == CompilerKnownProperty {
		a.compilerKnownMemberFacts[sourceTokenLocation(expr.Property.Token)] = member
		if member.Name == "OkRef" || member.Name == "ErrRef" {
			return a.inferBorrowedResultProjection(expr, objectType, member)
		}
		return a.refinedCompilerKnownType(member.Result), true
	}

	if memberType, ok := a.inferChannelMember(expr, objectType); ok {
		return memberType, true
	}

	if fieldType, ok := lookupStructField(objectType, expr.Property.Value); ok {
		if !a.canAccessStructField(objectType, expr.Property.Value) {
			a.addErrorAtToken(expr.Property.Token, "field %s on %s is not accessible from module %s", expr.Property.Value, typeDisplayName(objectType), moduleDisplayName(a.currentModule))
			return Type{Kind: InvalidType}, false
		}
		for fieldID, field := range objectType.Fields {
			if field.Name != expr.Property.Value {
				continue
			}
			a.resolvedStructMemberPlans[expr] = ResolvedStructMemberPlan{
				Kind: MemberStoredField, OwnerType: objectType, MemberType: fieldType,
				FieldID: uint32(fieldID), FieldName: field.Name, Tags: cloneStructTags(field.Tags),
				Action: resolvedStructCopyAction(fieldType),
			}
			break
		}
		if fieldType.Kind == ReferenceType {
			fieldType = a.referenceTypeWithOriginFromExpression(fieldType, expr.Object)
		}
		return fieldType, true
	}
	if event, ok := lookupEvent(objectType, expr.Property.Value); ok {
		return event.Type, true
	}
	if isMutexGuardType(objectType) {
		protected := objectType.TypeArgs[0]
		if fieldType, ok := lookupStructField(protected, expr.Property.Value); ok {
			if definition, exists := memberDefinitionToken(protected, expr.Property.Value); exists {
				a.bindDefinition(expr.Property.Token, definition)
			}
			return fieldType, true
		}
		if _, ok := a.lookupResolvedProperty(protected, expr.Property.Value); ok {
			returnProperty, readable := a.resolveReadableProperty(protected, expr.Property.Value, expr.Property.Token)
			if !readable {
				return Type{Kind: InvalidType}, false
			}
			a.recordResolvedPropertyRead(expr, protected, returnProperty)
			return returnProperty.Type, true
		}
	}
	if field, ok := lookupRegisterFieldInfo(objectType, expr.Property.Value); ok {
		if field.Access == RegisterWriteOnly {
			a.addErrorAtToken(expr.Property.Token, "register field %s.%s is write-only and cannot be read", typeDisplayName(objectType), expr.Property.Value)
			return Type{Kind: InvalidType}, false
		}
		return field.Type, true
	}
	if objectType.Kind == RegisterType && expr.Property.Value == "_" {
		a.addErrorAtToken(expr.Property.Token, "reserved register field _ cannot be accessed")
		return Type{Kind: InvalidType}, false
	}

	if candidate, exists := a.lookupResolvedProperty(objectType, expr.Property.Value); exists {
		if candidate.Static {
			a.addErrorAtToken(expr.Property.Token, "static property %s.%s must be accessed through type %s", typeDisplayName(objectType), expr.Property.Value, typeDisplayName(objectType))
			return Type{Kind: InvalidType}, false
		}
		property, readable := a.resolveReadableProperty(objectType, expr.Property.Value, expr.Property.Token)
		if !readable {
			return Type{Kind: InvalidType}, false
		}
		a.resolvedStructMemberPlans[expr] = ResolvedStructMemberPlan{
			Kind: MemberProperty, OwnerType: objectType, MemberType: property.Type, FieldName: property.Name,
		}
		a.recordResolvedPropertyRead(expr, objectType, property)
		return property.Type, true
	}

	// A legacy lowercase spelling of a CamelCase compiler-known member, such as
	// `worker.status`, is not canonical; name the member it meant.
	// Rules: rules/concurrency/threads.md — § 26(4)–(5); rules/compiler/compiler_known_members.md — naming
	if canonical, ok := caseInsensitiveCompilerKnownMember(objectType, expr.Property.Value); ok {
		a.addErrorAtTokenWithMetadata(expr.Property.Token, "",
			"use the canonical CamelCase member `"+canonical+"`; lowercase member spellings are not Sec 0.1 syntax",
			"unknown member %s on %s", expr.Property.Value, typeDisplayName(objectType))
		return Type{Kind: InvalidType}, false
	}
	a.addErrorAtToken(expr.Property.Token, "unknown member %s on %s", expr.Property.Value, typeDisplayName(objectType))
	return Type{Kind: InvalidType}, false
}

func (a *Analyzer) inferPointerMember(expr *ast.MemberExpression, objectType Type) (Type, bool) {
	memberName := expr.Property.Value
	if !a.inUnsafe {
		a.addErrorAtToken(expr.Property.Token, "member %s requires unsafe", memberName)
		return Type{Kind: InvalidType}, false
	}
	if !isAddressablePointerSource(expr.Object) {
		a.addErrorAtToken(expr.Property.Token, "member %s requires an addressable value", memberName)
		return Type{Kind: InvalidType}, false
	}
	return compilerKnownRawPointerResult(objectType), true
}

func (a *Analyzer) inferChannelMember(expr *ast.MemberExpression, objectType Type) (Type, bool) {
	if !isChannelType(objectType) {
		return Type{}, false
	}
	messageType := objectType.TypeArgs[0]
	switch expr.Property.Value {
	case "tx":
		return senderType(messageType), true
	case "rx":
		return receiverType(messageType), true
	default:
		return Type{}, false
	}
}

func (a *Analyzer) inferStaticMemberExpression(expr *ast.MemberExpression) (Type, bool) {
	// A lexical value (including self) wins over a same-shaped type path.
	// rules/declarations/static.md, section 8 requires a type-qualified access.
	if root, ok := expressionRootIdentifier(expr.Object); ok && root == "self" {
		return Type{}, false
	}
	path, ok := typePathFromExpression(expr.Object)
	if !ok {
		return Type{}, false
	}
	typeName := a.resolveTypeName(path)
	typ, exists := a.types[typeName]
	if !exists {
		return Type{}, false
	}
	memberName := typeName + "." + expr.Property.Value
	symbol, exists := a.symbols[memberName]
	if !exists {
		// rules/declarations/static.md, sections 8 and 11. Static
		// properties use type-qualified lookup and getter semantics.
		if property, found := lookupProperty(typ, expr.Property.Value); found {
			if !property.Static {
				a.addErrorAtToken(expr.Property.Token, "instance property %s.%s requires a value receiver", typeDisplayName(typ), expr.Property.Value)
				return Type{Kind: InvalidType}, true
			}
			a.bindDefinition(expr.Property.Token, property.Token)
			if !property.HasGetter {
				a.addErrorAtToken(expr.Property.Token, "property %s has no getter", expr.Property.Value)
				return Type{Kind: InvalidType}, true
			}
			a.resolvedStructMemberPlans[expr] = ResolvedStructMemberPlan{
				Kind: MemberProperty, OwnerType: typ, MemberType: property.Type, FieldName: property.Name,
			}
			a.recordResolvedPropertyRead(expr, typ, property)
			return property.Type, true
		}
		if member, ok := compilerKnownMember(typ, expr.Property.Value, true); ok && member.Kind == CompilerKnownProperty {
			a.compilerKnownMemberFacts[sourceTokenLocation(expr.Property.Token)] = member
			return a.refinedCompilerKnownType(member.Result), true
		}
		return Type{}, false
	}
	a.bindDefinition(expr.Property.Token, symbol.Token)
	return symbol.Type, true
}

func (a *Analyzer) inferArrayLiteral(expr *ast.ArrayLiteral) (Type, expressionValue) {
	plan, ok := a.resolveArrayLiteralPlan(expr, Type{Kind: InvalidType})
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if len(plan.Entries) == 0 {
		a.addErrorAtToken(expr.Token, "cannot infer element type of empty array literal")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	firstType := arrayLiteralEntryElementType(plan.Entries[0])
	if firstType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	for _, entry := range plan.Entries[1:] {
		elementType := arrayLiteralEntryElementType(entry)
		if elementType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if !sameConcreteType(firstType, elementType) {
			a.addErrorAtToken(expressionToken(arrayLiteralEntryExpression(expr, entry)), "array literal elements must have one identical type")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
	}

	plan.ElementType = firstType
	a.recordResolvedArrayLiteralPlan(expr, plan)
	return NewFixedArrayType(firstType, plan.Length), expressionValue{Display: expr.String()}
}

// inferArrayLiteralWithExpected resolves every source element once, validates
// the target element/shape, and consumes the resulting exact length proof for
// any named collection contracts.
//
// Rules:
//   - rules/collections/collections.md — §5.5 "Array literals" and §5.6 "Spread in fixed-array literals"
//   - rules/types/contracts.md — "String and collection contracts"
func (a *Analyzer) inferArrayLiteralWithExpected(expr *ast.ArrayLiteral, expected Type) (Type, expressionValue) {
	if expected.Kind != ArrayType || expected.Element == nil {
		return a.inferArrayLiteral(expr)
	}
	if _, fixed := exactFixedArrayLength(expected); !fixed {
		// Owning dynamic-array literals are not defined in Sec 0.1 (user
		// decision 2026-10-05), so a range segment never expands into T[].
		for _, element := range expr.Elements {
			if segment, ok := element.(*ast.RangeExpression); ok {
				a.addErrorAtTokenWithMetadata(segment.Token, diagnostics.ArrayRangeSegmentInvalid,
					"Build a fixed array, as in `let values := [1..9]`, or append the range to the dynamic array with `try values.Append(1..9)`.",
					"range segment cannot initialize owning dynamic array %s; range segments build fixed arrays", typeDisplayName(expected))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
		}
	}
	plan, ok := a.resolveArrayLiteralPlan(expr, *expected.Element)
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if a.checkArrayLiteralContracts(expected, expr, plan.Length) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	expectedLength, fixedExpected := exactFixedArrayLength(expected)
	if fixedExpected && plan.Length.Cmp(expectedLength) != 0 {
		if arrayShapeOf(expected) == ArrayShapeFixed {
			a.addErrorAtToken(expr.Token, "array literal has %s elements, expected %s", plan.Length.String(), expectedLength.String())
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
	}
	elementIndex := new(big.Int)
	for _, entry := range plan.Entries {
		elementType := arrayLiteralEntryElementType(entry)
		if elementType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		source := arrayLiteralEntryExpression(expr, entry)
		if entry.Kind == ArrayLiteralRange {
			// The segment's bounds were already resolved against the target
			// element type.
			elementIndex.Add(elementIndex, entry.Length)
			continue
		}
		if !a.canInitialize(*expected.Element, elementType, source) {
			a.addErrorAtToken(expressionToken(source), "array element %s must be %s, got %s", new(big.Int).Add(elementIndex, big.NewInt(1)).String(), typeDisplayName(*expected.Element), typeDisplayName(elementType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		// rules/types/types.md "Context shaping" and rules/types/contracts.md
		// "Initialization and assignment": an element literal is shaped by the
		// element type, so its representability and contracts are proven here.
		if entry.Kind != ArrayLiteralSpread && a.checkDeclaredContractExpression(*expected.Element, source) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		elementIndex.Add(elementIndex, entry.Length)
	}
	plan.ElementType = *expected.Element
	if fixedExpected {
		a.recordResolvedArrayLiteralPlan(expr, plan)
	}
	return expected, expressionValue{Display: expr.String()}
}

// inferIndexExpression resolves indexed reads for the canonical string,
// array, slice, variadic-pack, and list surfaces.
//
// Rules:
//   - rules/collections/collections.md — §8 "Indexing"
func (a *Analyzer) inferIndexExpression(expr *ast.IndexExpression) (Type, expressionValue) {
	if elementType, ok := a.inferStringPointerIndex(expr); ok {
		return elementType, expressionValue{Display: expr.String()}
	}
	leftType, _ := a.inferPlaceBase(expr.Left)
	if leftType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	leftType = dereferenceType(leftType)
	indexType, _ := a.inferExpression(expr.Index)
	if indexType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if !isIntegerType(indexType) {
		a.addErrorAtToken(expressionToken(expr.Index), "%s index must be integer, got %s", indexableKindName(leftType), typeDisplayName(indexType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	switch leftType.Kind {
	case ArrayType, SliceType, VariadicPackType:
		if leftType.Element == nil {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if leftType.Kind != VariadicPackType && !a.checkConstantIndexBounds(expr, leftType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		elementType := *leftType.Element
		if leftType.Kind == ArrayType && arrayShapeOf(leftType) == ArrayShapeFixed {
			a.recordFixedArrayIndexPlan(expr, leftType, elementType, indexType, ArrayIndexRead)
		} else if leftType.Kind == SliceType || arrayShapeOf(leftType) == ArrayShapeDynamic {
			a.recordSequenceIndexEffect(expr)
		}
		if elementType.Kind == ReferenceType {
			elementType = a.referenceTypeWithOriginFromExpression(elementType, expr.Left)
		}
		return elementType, expressionValue{Display: expr.String()}
	case StringType:
		return Type{Name: "rune", Kind: RuneType}, expressionValue{Display: expr.String()}
	default:
		if isCompilerKnownListType(leftType) {
			elementType := leftType.TypeArgs[0]
			if !a.checkConstantListIndexBounds(expr, leftType) {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			a.recordListIndexPlan(expr, leftType, elementType, indexType, ArrayIndexRead)
			if elementType.Kind == ReferenceType {
				elementType = a.referenceTypeWithOriginFromExpression(elementType, expr.Left)
			}
			return elementType, expressionValue{Display: expr.String()}
		}
		a.addErrorAtToken(expr.Token, "type %s is not indexable", typeDisplayName(leftType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
}

func (a *Analyzer) inferStringPointerIndex(expr *ast.IndexExpression) (Type, bool) {
	member, ok := expr.Left.(*ast.MemberExpression)
	if !ok || member.Property == nil || member.Property.Value != "ptr" {
		return Type{}, false
	}
	objectType, _ := a.inferExpression(member.Object)
	if dereferenceType(objectType).Kind != StringType {
		return Type{}, false
	}
	if !a.inUnsafe {
		a.addErrorAtToken(member.Property.Token, "member ptr requires unsafe")
		return Type{Kind: InvalidType}, true
	}
	if !isAddressablePointerSource(member.Object) {
		a.addErrorAtToken(member.Property.Token, "member ptr requires an addressable value")
		return Type{Kind: InvalidType}, true
	}
	indexType, _ := a.inferExpression(expr.Index)
	if indexType.Kind == InvalidType {
		return Type{Kind: InvalidType}, true
	}
	if !isIntegerType(indexType) {
		a.addErrorAtToken(expressionToken(expr.Index), "string byte index must be integer, got %s", typeDisplayName(indexType))
		return Type{Kind: InvalidType}, true
	}
	return a.types["byte"], true
}

func (a *Analyzer) inferSliceExpression(expr *ast.SliceExpression) (Type, expressionValue) {
	leftType, _ := a.inferPlaceBase(expr.Left)
	if leftType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	leftType = dereferenceType(leftType)
	if leftType.Kind != ArrayType && leftType.Kind != SliceType {
		a.addErrorAtToken(expr.Token, "type %s cannot be sliced", typeDisplayName(leftType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if leftType.Element == nil {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	a.checkSliceBounds(expr, leftType)
	originName, originToken, originLocal, originStorage, generation := a.referenceOriginForExpression(expr.Left)
	return Type{
		Name:                       typeDisplayName(*leftType.Element) + "[]",
		Kind:                       SliceType,
		Element:                    leftType.Element,
		ReferenceOriginName:        originName,
		ReferenceOriginToken:       originToken,
		ReferenceOriginLocal:       originLocal,
		ReferenceOriginStorage:     originStorage,
		ReferenceOriginGeneration:  generation,
		ReferenceOriginMatchScoped: a.referenceOriginMatchScopedForExpression(expr.Left),
	}, expressionValue{Display: expr.String()}
}

// inferRefExpression preserves the shared or mutable access mode on indexed
// element Places after their collection-specific type and bounds resolution.
//
// Rules:
//   - rules/collections/collections.md — §8.1 "General rule" and §19 "Borrowing and structural mutation"
func (a *Analyzer) inferRefExpression(expr *ast.RefExpression) (Type, expressionValue) {
	if a.checkBorrowCreation(expr.Value, expr.Mutable, expr.Token) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	valueType, _ := a.inferExpression(expr.Value)
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if index, ok := expr.Value.(*ast.IndexExpression); ok {
		use := ArrayIndexBorrow
		if expr.Mutable {
			use = ArrayIndexMutBorrow
		}
		a.setResolvedIndexUse(index, use)
	}
	originName, originToken, originLocal, originStorage, generation := a.referenceOriginForExpression(expr.Value)
	return Type{
		Name:                       referenceTypeName(valueType, expr.Mutable),
		Kind:                       ReferenceType,
		Element:                    &valueType,
		ReferenceMutable:           expr.Mutable,
		ReferenceOriginName:        originName,
		ReferenceOriginToken:       originToken,
		ReferenceOriginLocal:       originLocal,
		ReferenceOriginStorage:     originStorage,
		ReferenceOriginGeneration:  generation,
		ReferenceOriginMatchScoped: a.referenceOriginMatchScopedForExpression(expr.Value),
	}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferUnionVariantExpression(expr *ast.MemberExpression) (Type, bool) {
	typeName, ok := typePathFromExpression(expr.Object)
	if !ok {
		return Type{}, false
	}
	typeName = a.resolveTypeName(typeName)

	typ, ok := a.types[typeName]
	if !ok || typ.Kind != UnionType {
		return Type{}, false
	}
	if len(typ.GenericParameters) > 0 && a.currentFunctionReturn.Kind == UnionType && a.currentFunctionReturn.Name == typ.Name {
		typ = a.currentFunctionReturn
	}
	for _, variant := range typ.UnionVariants {
		if variant.Name != expr.Property.Value {
			continue
		}
		if variant.Payload != nil {
			a.bindDefinition(expr.Property.Token, variant.Token)
			a.addErrorAtToken(expr.Property.Token, "union variant %s.%s requires payload", typeDisplayName(typ), variant.Name)
			return Type{Kind: InvalidType}, true
		}
		a.bindDefinition(expr.Property.Token, variant.Token)
		return typ, true
	}
	a.addErrorAtToken(expr.Property.Token, "unknown union variant %s.%s", typ.Name, expr.Property.Value)
	return Type{Kind: InvalidType}, true
}

// inferEnumValueExpression resolves a qualified enum member against either its
// ordinary owner or the concrete generic owner written as Enum[T].Member.
//
// Rules:
//   - rules/declarations/enums.md — "Generic enums"
//   - rules/declarations/generics.md — "Generic enums"
func (a *Analyzer) inferEnumValueExpression(expr *ast.MemberExpression) (Type, bool) {
	owner := expr.Object
	if len(expr.OwnerGenericArguments) > 0 {
		if indexed, ok := owner.(*ast.IndexExpression); ok {
			owner = indexed.Left
		}
	}
	typeName, ok := typePathFromExpression(owner)
	if !ok {
		return Type{}, false
	}
	typeName = a.resolveTypeName(typeName)

	typ, ok := a.types[typeName]
	if !ok || typ.Kind != EnumType {
		return Type{}, false
	}
	if len(expr.OwnerGenericArguments) > 0 {
		concrete, resolved := a.resolveType(&ast.TypeReference{
			Token:    expr.Token,
			Name:     typeName,
			TypeArgs: expr.OwnerGenericArguments,
		})
		if !resolved {
			return Type{Kind: InvalidType}, true
		}
		typ = concrete
	} else if len(typ.GenericParameters) > 0 {
		a.addErrorAtToken(expr.Token, "%s requires %d generic arguments, got 0", typeName, len(typ.GenericParameters))
		return Type{Kind: InvalidType}, true
	}
	value, ok := typ.EnumConsts[expr.Property.Value]
	if !ok {
		a.addErrorAtToken(expr.Property.Token, "unknown enum value: %s.%s has never been declared", typeName, expr.Property.Value)
		return Type{Kind: InvalidType}, true
	}
	a.bindDefinition(expr.Property.Token, value.Token)
	return typ, true
}

func (a *Analyzer) inferConversionExpression(expr *ast.ConversionExpression) (Type, expressionValue) {
	targetType, ok := a.resolveType(expr.Type)
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	valueType, _ := a.inferExpression(expr.Value)
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if (targetType.Kind == RawPtrType || valueType.Kind == RawPtrType) && !a.inUnsafe {
		a.addErrorAtToken(expr.Token, "conversion involving RawPtr requires unsafe")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if !a.canExplicitConvert(targetType, valueType) {
		a.addErrorAtToken(expr.Token, "cannot convert %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if (hasUnitSemantics(targetType) || hasUnitSemantics(valueType)) && !a.validateExplicitUnitConversion(expr.Token, targetType, valueType) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if converted, ok := numericCarrierConversionResult(targetType, valueType); ok {
		return converted, expressionValue{Display: expr.String()}
	}
	if targetType.Kind == EnumType {
		if a.checkNominalMembershipValue(targetType, expr.Value) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		conversionType, valid := a.enumConversionResultType(targetType, valueType, expr.Value)
		if !valid {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return conversionType, expressionValue{Display: expr.String()}
	}
	if targetType.Kind == RegisterType && isIntegerType(valueType) {
		return a.integerToRegisterConversionResultType(targetType, valueType, expr.Value), expressionValue{Display: expr.String()}
	}
	if isIntegerType(targetType) && valueType.Kind == EnumType {
		return a.enumToIntegerConversionResultType(targetType, valueType, expr.Value), expressionValue{Display: expr.String()}
	}
	if targetType.Kind == StringType && valueType.Kind == EnumType && valueType.Underlying == "string" {
		return targetType, expressionValue{Display: expr.String()}
	}
	if !a.validateConstantIntegerConversion(targetType, valueType, expr.Value) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if a.checkEmptyListLiteralContracts(targetType, expr.Value) || a.checkStringLiteralContracts(targetType, expr.Value) || a.checkNominalMembershipValue(targetType, expr.Value) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	return targetType, expressionValue{Display: expr.String()}
}

// inferFactorProvidedUnitConversion implements rules/types/units.md,
// "Runtime/configured currency conversion". The frontend proves the unit
// equation source*factor=target but deliberately does not inspect factor value.
func (a *Analyzer) inferFactorProvidedUnitConversion(expr *ast.CallExpression, target Type) (Type, expressionValue) {
	source, _ := a.inferExpression(expr.Arguments[0])
	factor, _ := a.inferExpression(expr.Arguments[1])
	if source.Kind == InvalidType || factor.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	to, from := effectiveUnitSemantics(target), effectiveUnitSemantics(source)
	if !isNumericType(source) || !isNumericType(factor) || !hasUnitSemantics(source) || !hasUnitSemantics(factor) ||
		!isLinearRatioSemantics(to) || !isLinearRatioSemantics(from) || !isLinearRatioSemantics(factor.UnitSemantics) {
		a.addErrorAtToken(expr.Token, "factor-provided conversion to %s requires linear unit-bearing source and factor quantities", typeDisplayName(target))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	expectedFactorDimension := target.Dimension.Div(source.Dimension)
	if !factor.Dimension.Equal(expectedFactorDimension) {
		a.addErrorAtToken(expressionToken(expr.Arguments[1]), "conversion factor for %s from %s must have unit dimension %v", typeDisplayName(target), typeDisplayName(source), expectedFactorDimension.Base)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	return target, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferNewExpression(expr *ast.NewExpression, handled bool) (Type, expressionValue) {
	result := expressionValue{Display: expr.String()}
	if expr.Type == nil {
		return Type{Kind: InvalidType}, result
	}
	target, ok := a.resolveType(expr.Type)
	if !ok || target.Kind == InvalidType {
		return Type{Kind: InvalidType}, result
	}
	if target.Kind == InterfaceType {
		a.addErrorAtToken(expr.Type.Token, "cannot construct interface type %s with new", typeDisplayName(target))
		return Type{Kind: InvalidType}, result
	}

	key := initFunctionName(target.Name)
	initializers := a.functions[key]
	hasMatchingArity := false
	for _, initializer := range initializers {
		if len(initializer.Parameters) == len(expr.Arguments) {
			hasMatchingArity = true
			break
		}
	}
	if len(initializers) == 0 || (len(expr.Arguments) == 0 && !hasMatchingArity && IsDefaultable(target)) {
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "new %s has no init overload accepting %d arguments", typeDisplayName(target), len(expr.Arguments))
			return Type{Kind: InvalidType}, result
		}
		if !IsDefaultable(target) {
			a.addErrorAtToken(expr.Token, "new %s() has no explicit init and %s has no valid implicit construction path", typeDisplayName(target), typeDisplayName(target))
			return Type{Kind: InvalidType}, result
		}
		a.resolvedConstructions[expr] = ResolvedConstruction{Target: target, Implicit: true}
		if definition, exists := a.typeDefinitionTokens[target.Name]; exists {
			a.bindDefinition(expr.Type.Token, definition)
		}
		return target, result
	}

	// Bind lifecycle navigation to the `new` keyword. The following type token
	// remains a type occurrence instead of being misclassified as a method.
	callee := &ast.Identifier{Token: expr.Token, Value: key}
	call := &ast.CallExpression{Token: expr.Token, Callee: callee, Function: callee, Arguments: expr.Arguments}
	_, _ = a.inferCallExpression(call)
	resolved, ok := a.resolvedCalls[call]
	if !ok || !resolved.Function.Initializer || resolved.Function.ConstructionType == nil {
		return Type{Kind: InvalidType}, result
	}
	selected := resolved.Function
	target = *selected.ConstructionType
	a.resolvedConstructions[expr] = ResolvedConstruction{
		Initializer: selected,
		Target:      target,
		ErrorType:   selected.ConstructionError,
	}
	if selected.ConstructionError == nil {
		return target, result
	}
	if !handled {
		a.addErrorAtToken(expr.Token, "new %s selects a fallible init with construction error %s; use try or handle the error locally", typeDisplayName(target), typeDisplayName(*selected.ConstructionError))
		return target, result
	}
	return Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{target, *selected.ConstructionError}}, result
}

// inferTestingOperationCall validates the implemented compiler-known testing
// operations. It recognizes testing contextually and never resolves it as an
// imported module, ordinary object, or user-declared function namespace.
//
// Rules:
//   - rules/tooling/testing.md — §11 "Compiler-known testing namespace"
//   - rules/tooling/testing.md — §§12–14 "testing.Pass", "testing.Fail", "testing.Skip"
//   - rules/tooling/testing.md — §15 "testing.Log"
//   - rules/tooling/testing.md — §16 "testing.Expect"
//   - rules/tooling/testing.md — §17 "testing.Require"
//   - rules/tooling/testing.md — §18 "Equality expectations"
//   - rules/tooling/diagnostics.md — §§12 and 30 "Diagnostic registry" and "Testing requirements"
func (a *Analyzer) inferTestingOperationCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member == nil || member.Property == nil {
		return Type{}, expressionValue{}, false
	}
	namespace, ok := member.Object.(*ast.Identifier)
	if !ok || namespace == nil || namespace.Value != "testing" {
		return Type{}, expressionValue{}, false
	}

	var kind TestingOperationKind
	var diagnosticID string
	switch member.Property.Value {
	case "Pass":
		kind = TestingOperationPass
		diagnosticID = diagnostics.InvalidTestingTerminationArguments
	case "Fail":
		kind = TestingOperationFail
		diagnosticID = diagnostics.InvalidTestingTerminationArguments
	case "Skip":
		kind = TestingOperationSkip
		diagnosticID = diagnostics.InvalidTestingTerminationArguments
	case "Expect":
		kind = TestingOperationExpect
		diagnosticID = diagnostics.InvalidTestingExpectArguments
	case "Require":
		kind = TestingOperationRequire
		diagnosticID = diagnostics.InvalidTestingRequireArguments
	case "Log":
		kind = TestingOperationLog
		diagnosticID = diagnostics.InvalidTestingLogArguments
	case "ExpectEqual":
		kind = TestingOperationExpectEqual
		diagnosticID = diagnostics.InvalidTestingExpectEqualArguments
	case "RequireEqual":
		kind = TestingOperationRequireEqual
		diagnosticID = diagnostics.InvalidTestingRequireEqualArguments
	default:
		return Type{}, expressionValue{}, false
	}

	display := expressionValue{Display: expr.String()}
	if a.currentTest == nil {
		for _, argument := range expr.Arguments {
			a.inferExpression(argument)
		}
		a.addErrorAtTokenWithID(
			namespace.Token,
			diagnostics.TestingOutsideTestContext,
			"compiler-known testing.%s is only available inside a test declaration",
			member.Property.Value,
		)
		return Type{Kind: InvalidType}, display, true
	}
	if len(expr.GenericArguments) != 0 {
		a.addErrorAtTokenWithID(
			member.Property.Token,
			diagnosticID,
			"testing.%s does not accept generic arguments",
			member.Property.Value,
		)
		return Type{Kind: InvalidType}, display, true
	}
	if kind == TestingOperationPass {
		if len(expr.Arguments) != 0 {
			for _, argument := range expr.Arguments {
				a.inferExpression(argument)
			}
			a.addErrorAtTokenWithID(member.Property.Token, diagnosticID,
				"testing.Pass expects no arguments, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, display, true
		}
		a.resolvedTestingOperations[expr] = ResolvedTestingOperation{Kind: kind, Test: a.currentTest}
		return Type{Name: "void", Kind: VoidType}, display, true
	}
	if kind == TestingOperationLog || kind == TestingOperationFail || kind == TestingOperationSkip {
		if len(expr.Arguments) != 1 {
			for _, argument := range expr.Arguments {
				a.inferExpression(argument)
			}
			a.addErrorAtTokenWithID(
				member.Property.Token,
				diagnosticID,
				"testing.%s expects exactly one string message, got %d arguments",
				member.Property.Value,
				len(expr.Arguments),
			)
			return Type{Kind: InvalidType}, display, true
		}
		message := expr.Arguments[0]
		stringType := a.types["string"]
		messageType, _ := a.inferExpressionWithExpected(message, stringType)
		if messageType.Kind == InvalidType {
			return Type{Kind: InvalidType}, display, true
		}
		if !a.canInitialize(stringType, messageType, message) {
			a.addErrorAtTokenWithID(
				expressionToken(message),
				diagnosticID,
				"testing.%s message must be string, got %s",
				member.Property.Value,
				typeDisplayName(messageType),
			)
			return Type{Kind: InvalidType}, display, true
		}
		a.resolvedTestingOperations[expr] = ResolvedTestingOperation{
			Kind:    kind,
			Test:    a.currentTest,
			Message: message,
		}
		return Type{Name: "void", Kind: VoidType}, display, true
	}
	if kind == TestingOperationExpectEqual || kind == TestingOperationRequireEqual {
		if len(expr.Arguments) < 2 || len(expr.Arguments) > 3 {
			for _, argument := range expr.Arguments {
				a.inferExpression(argument)
			}
			a.addErrorAtTokenWithID(member.Property.Token, diagnosticID,
				"testing.%s expects expected and actual values and an optional string message, got %d arguments",
				member.Property.Value, len(expr.Arguments))
			return Type{Kind: InvalidType}, display, true
		}
		expected, actual := expr.Arguments[0], expr.Arguments[1]
		expectedType, _ := a.inferExpression(expected)
		actualType, _ := a.inferExpression(actual)
		valid := expectedType.Kind != InvalidType && actualType.Kind != InvalidType
		if valid {
			valid = a.validateTestingEquality(expected, actual, expectedType, actualType, member.Property.Token, diagnosticID)
		}
		var message ast.Expression
		if len(expr.Arguments) == 3 {
			message = expr.Arguments[2]
			stringType := a.types["string"]
			messageType, _ := a.inferExpressionWithExpected(message, stringType)
			if messageType.Kind == InvalidType {
				valid = false
			} else if !a.canInitialize(stringType, messageType, message) {
				a.addErrorAtTokenWithID(expressionToken(message), diagnosticID,
					"testing.%s message must be string, got %s", member.Property.Value, typeDisplayName(messageType))
				valid = false
			}
		}
		if !valid {
			return Type{Kind: InvalidType}, display, true
		}
		a.resolvedTestingOperations[expr] = ResolvedTestingOperation{
			Kind: kind, Test: a.currentTest, Expected: expected, Actual: actual, Message: message,
		}
		return Type{Name: "void", Kind: VoidType}, display, true
	}
	if len(expr.Arguments) < 1 || len(expr.Arguments) > 2 {
		for _, argument := range expr.Arguments {
			a.inferExpression(argument)
		}
		a.addErrorAtTokenWithID(
			member.Property.Token,
			diagnosticID,
			"testing.%s expects one bool condition and an optional string message, got %d arguments",
			member.Property.Value,
			len(expr.Arguments),
		)
		return Type{Kind: InvalidType}, display, true
	}

	boolType := a.types["bool"]
	conditionType, _ := a.inferExpressionWithExpected(expr.Arguments[0], boolType)
	valid := true
	if conditionType.Kind != InvalidType && !a.canInitialize(boolType, conditionType, expr.Arguments[0]) {
		a.addErrorAtTokenWithID(
			expressionToken(expr.Arguments[0]),
			diagnosticID,
			"testing.%s condition must be bool, got %s",
			member.Property.Value,
			typeDisplayName(conditionType),
		)
		valid = false
	}

	var message ast.Expression
	if len(expr.Arguments) == 2 {
		message = expr.Arguments[1]
		stringType := a.types["string"]
		messageType, _ := a.inferExpressionWithExpected(message, stringType)
		if messageType.Kind != InvalidType && !a.canInitialize(stringType, messageType, message) {
			a.addErrorAtTokenWithID(
				expressionToken(message),
				diagnosticID,
				"testing.%s message must be string, got %s",
				member.Property.Value,
				typeDisplayName(messageType),
			)
			valid = false
		}
	}
	if !valid || conditionType.Kind == InvalidType {
		return Type{Kind: InvalidType}, display, true
	}

	a.resolvedTestingOperations[expr] = ResolvedTestingOperation{
		Kind:      kind,
		Test:      a.currentTest,
		Condition: expr.Arguments[0],
		Message:   message,
	}
	return Type{Name: "void", Kind: VoidType}, display, true
}

func (a *Analyzer) inferCallExpression(expr *ast.CallExpression) (Type, expressionValue) {
	if typ, value, ok := a.inferTestingOperationCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferCompilerKnownFunction(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferCompilerKnownConstructor(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferCompilerKnownMemberCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferChannelCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferEventCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferSubscriptionCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferMutexCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferAtomicCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferArenaCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferRawPointerCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferRuneArrayToStringCall(expr); ok {
		return typ, value
	}

	name := callExpressionName(expr)
	functions, ok := a.functions[name]
	methodReceiver := methodReceiverInfo{}
	isMethodCall := false
	if !ok || len(functions) == 0 {
		if implName, implOK := a.implScopedFunctionName(name); implOK {
			if implFunctions := a.functions[implName]; len(implFunctions) > 0 {
				name = implName
				functions = implFunctions
				ok = true
			}
		}
	}
	if !ok || len(functions) == 0 {
		if methodName, methodOK := a.methodCallName(expr); methodOK {
			if methodFunctions := a.functions[methodName]; len(methodFunctions) > 0 {
				methodReceiver, isMethodCall = a.methodCallReceiver(expr)
				if !isMethodCall {
					return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
				}
				if member, ok := expr.Callee.(*ast.MemberExpression); ok && member.Property != nil {
					if selected, ok := resolvedInterfaceOverloads(methodReceiver.Type, member.Property.Value); ok {
						methodFunctions = selected
					}
				}
				name = methodName
				functions = methodFunctions
				ok = true
			}
		}
	}
	if !ok || len(functions) == 0 {
		if constraintFunctions, receiver, constraintOK := a.constrainedGenericMethodCall(expr); constraintOK {
			functions = constraintFunctions
			methodReceiver = receiver
			isMethodCall = true
			ok = true
		}
	}
	if !ok || len(functions) == 0 {
		if methodName, methodOK := a.inheritedCoreMethodCallName(expr); methodOK {
			if methodFunctions := a.functions[methodName]; len(methodFunctions) > 0 {
				methodReceiver, isMethodCall = a.methodCallReceiver(expr)
				if !isMethodCall {
					return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
				}
				name = methodName
				functions = methodFunctions
				ok = true
			}
		}
	}
	if !ok || len(functions) == 0 {
		// A computed receiver such as byte(value) has no syntactic callee path,
		// but ordinary method lookup above can still resolve it from its semantic
		// type. Only treat the callee as a function value after those method
		// candidates have been exhausted.
		//
		// Rules:
		//   - rules/compiler/compiler_known_members.md — "Lookup order"
		//   - rules/compiler/compiler_known_members.md — "Built-in type member lookup"
		if name == "" {
			return a.inferFunctionValueCall(expr, Type{Kind: InvalidType})
		}
		if symbol, exists := a.symbols[name]; exists && symbol.Type.Kind == FunctionType {
			// rules/declarations/lambda-functions.md: retain the callable value's
			// resolved capability on the callee expression so LSP and later
			// compiler stages do not reconstruct it from source spelling.
			a.recordFunctionValueCalleeBinding(expr.Callee, symbol)
			return a.inferFunctionValueCall(expr, symbol.Type)
		}
		if typ, value, ok := a.inferCallAsUnionVariantConstructor(expr, nil); ok {
			return typ, value
		}
		return a.inferCallAsConversion(expr)
	}
	functions = a.accessibleFunctions(functions)
	if len(functions) == 0 {
		a.addErrorAtToken(expr.Token, "function %s is not accessible from module %s", name, a.currentModule)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	a.bindDefinitions(callCalleeDefinitionToken(expr), functionDeclarationTokens(functions))

	releaseNullContexts := a.bindNullArgumentContexts(functions, expr.Arguments)
	sourceArgTypes, sourceArgs, preparedSpreadValues, runtimeSpreadValues, ok := a.callArgumentTypes(expr.Arguments, anyFunctionIsVariadic(functions))
	releaseNullContexts()
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	arityMatches := []Function{}
	for _, function := range functions {
		argTypes := a.callArgumentTypesForFunction(function, sourceArgTypes, methodReceiver, isMethodCall)
		if functionAcceptsCallArguments(function, len(argTypes), runtimeSpreadValues) {
			arityMatches = append(arityMatches, function)
		}
	}

	if len(arityMatches) == 0 {
		maximumArity := 0
		for _, function := range functions {
			maximumArity = max(maximumArity, len(function.Parameters))
		}
		if len(expr.Arguments) > maximumArity && maximumArity < len(expr.Arguments) {
			a.addErrorAtExpression(expr.Arguments[maximumArity], "function %s expects %s arguments, got %d", name, formatFunctionArities(functions), len(sourceArgTypes))
		} else {
			a.addErrorAtToken(expr.Token, "function %s expects %s arguments, got %d", name, formatFunctionArities(functions), len(sourceArgTypes))
		}
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	matches := []overloadMatch{}
	var receiverError string
	var constraintFailure *GenericConstraintFailure
	hadGenericArityMatch := false
	hadGenericInference := false
	hadExplicitGenericCall := len(expr.GenericArguments) > 0
	hadGenericFunctionForExplicitCall := false
	hadExplicitGenericArityMatch := false
	hadExplicitGenericInferenceFailure := false
	for _, function := range arityMatches {
		if hadExplicitGenericCall {
			if len(function.GenericParameters) == 0 {
				continue
			}
			hadGenericFunctionForExplicitCall = true
			if len(expr.GenericArguments) <= len(function.GenericParameters) {
				hadExplicitGenericArityMatch = true
			}
			argTypes := a.callArgumentTypesForFunction(function, sourceArgTypes, methodReceiver, isMethodCall)
			instantiated, inferenceFailed, ok := a.explicitGenericFunctionInstance(function, expr.GenericArguments, argTypes, Type{})
			if !ok {
				hadExplicitGenericInferenceFailure = hadExplicitGenericInferenceFailure || inferenceFailed
				continue
			}
			if instantiated.GenericConstraintFailure != nil {
				if constraintFailure == nil {
					failure := *instantiated.GenericConstraintFailure
					constraintFailure = &failure
				}
				continue
			}
			function = instantiated
		} else if len(function.GenericParameters) > 0 {
			hadGenericArityMatch = true
			argTypes := a.callArgumentTypesForFunction(function, sourceArgTypes, methodReceiver, isMethodCall)
			instantiated, ok := a.inferGenericFunctionInstance(function, argTypes)
			if !ok {
				continue
			}
			hadGenericInference = true
			if instantiated.GenericConstraintFailure != nil {
				if constraintFailure == nil {
					failure := *instantiated.GenericConstraintFailure
					constraintFailure = &failure
				}
				continue
			}
			function = instantiated
		}
		matchesArguments := true
		rank := 0
		argTypes := a.callArgumentTypesForFunction(function, sourceArgTypes, methodReceiver, isMethodCall)
		if isMethodCall && functionUsesReceiver(function) && !a.canPassImplicitMethodReceiver(function, methodReceiver) {
			receiverError = a.implicitMethodReceiverError(function, methodReceiver)
			matchesArguments = false
		}
		for i := range argTypes {
			if !matchesArguments {
				break
			}
			var arg ast.Expression
			arg = sourceArgs[i]
			parameter, parameterOK := functionParameterForArgument(function, i)
			if !parameterOK {
				matchesArguments = false
				break
			}
			argType := a.contextualCallArgumentType(arg, argTypes[i], parameter.Type)
			if !a.canInitializeUnrecorded(parameter.Type, argType, arg) {
				matchesArguments = false
				break
			}
			rank += overloadArgumentRank(parameter.Type, argType)
		}
		if matchesArguments {
			matches = append(matches, overloadMatch{Function: function, Rank: rank})
		}
	}

	best := bestOverloadMatches(matches)
	if len(best) == 1 {
		if best[0].Function.Unsafe && !a.inUnsafe {
			kind := "function"
			if best[0].Function.Extern {
				kind = "extern function"
			}
			a.addErrorAtToken(expr.Token, "calling unsafe %s %s requires unsafe", kind, name)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if a.checkCompileTimeCallArgumentContracts(best[0].Function, sourceArgs) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if !a.validateCallArgumentOwnership(best[0].Function, sourceArgs, preparedSpreadValues) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if !a.validateCallArgumentBorrows(best[0].Function, sourceArgs, preparedSpreadValues) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		a.recordCallArgumentUnitConversions(best[0].Function, sourceArgs, a.callArgumentTypesForFunction(best[0].Function, sourceArgTypes, methodReceiver, isMethodCall))
		a.setDefinitions(callCalleeDefinitionToken(expr), best[0].Function.Token)
		dispatch := CallDispatchDirect
		if isMethodCall {
			dispatch = CallDispatchStaticMethod
			if dereferenceType(methodReceiver.Type).Kind == InterfaceType {
				dispatch = CallDispatchInterface
			}
		} else if best[0].Function.Extern {
			dispatch = CallDispatchForeign
		}
		contractReceiver := methodReceiver.Type
		if iface, ok := a.interfaceReceiverForCall(expr, methodReceiver.Type); ok {
			dispatch = CallDispatchInterface
			contractReceiver = iface
		}
		a.recordResolvedCall(expr, best[0].Function, dispatch, contractReceiver)
		a.recordForeignBufferExtents(expr, a.resolvedCalls[expr])
		execution, recordCall := a.callGraphExecutionForCall(expr)
		if !a.summaryPass && a.callGraphPathReachable && recordCall {
			if dispatch == CallDispatchInterface {
				a.recordInterfaceGraphCall(contractReceiver, best[0].Function, callCalleeDefinitionToken(expr), execution)
			} else {
				a.callGraph.addCall(a.currentCallable, best[0].Function, callCalleeDefinitionToken(expr), dispatch, execution)
			}
			a.recordForeignAbortEffect(best[0].Function, callCalleeDefinitionToken(expr))
			a.recordForeignAllocationEffect(best[0].Function, callCalleeDefinitionToken(expr))
			a.recordForeignBlockingEffect(best[0].Function, callCalleeDefinitionToken(expr))
			// rules/errors/panic.md § 21(3)–(4): a method called through an
			// interface reference or a constrained generic parameter has no
			// concrete body here, so its panic behavior is unknown.
			if dispatch == CallDispatchInterface || (isMethodCall && dereferenceType(methodReceiver.Type).Kind == GenericType) {
				a.callGraph.addEffect(a.currentCallable, EffectSite{Kind: EffectMayPanicUnknownCallee, Source: callCalleeDefinitionToken(expr)})
				a.callGraph.addArenaEffect(a.currentCallable, ArenaEffectSite{Kind: ArenaEffectUnknownCallee, Source: callCalleeDefinitionToken(expr), UnknownAllocation: true})
				a.recordUnknownBlockingCallee(callCalleeDefinitionToken(expr))
			}
		}
		a.setCallReferenceOrigin(expr, best[0].Function, sourceArgs, isMethodCall)
		a.markMovedCallArguments(best[0].Function, sourceArgs, preparedSpreadValues, isMethodCall)
		if isMethodCall && best[0].Function.ReceiverConsuming {
			if member, ok := expr.Callee.(*ast.MemberExpression); ok {
				a.consumeMethodReceiver(member.Object)
			}
		}
		a.markClosedResourceCall(best[0].Function, methodReceiver, isMethodCall, expr.Token)
		return best[0].Function.ReturnType, expressionValue{Display: expr.String()}
	}

	if len(best) > 1 {
		ambiguous := make([]lexer.Token, 0, len(best))
		for _, match := range best {
			ambiguous = append(ambiguous, match.Function.Token)
		}
		a.bindDefinitions(callCalleeDefinitionToken(expr), ambiguous)
		a.addErrorAtToken(expr.Token, "ambiguous call to %s", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if constraintFailure != nil {
		a.addErrorAtToken(
			expr.Token,
			"type %s does not satisfy constraint %s for %s",
			typeDisplayName(constraintFailure.Argument),
			typeDisplayName(constraintFailure.Interface),
			constraintFailure.Parameter,
		)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if hadExplicitGenericCall && !hadGenericFunctionForExplicitCall {
		a.addErrorAtToken(expr.Token, "function %s is not generic", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if hadExplicitGenericCall && !hadExplicitGenericArityMatch {
		for _, function := range arityMatches {
			if len(function.GenericParameters) > 0 {
				a.addErrorAtToken(expr.Token, "%s requires %d explicit generic arguments, got %d", name, len(function.GenericParameters), len(expr.GenericArguments))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
		}
	}
	if hadExplicitGenericInferenceFailure {
		a.addErrorAtToken(expr.Token, "cannot infer remaining generic arguments for %s", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if hadGenericArityMatch && !hadGenericInference {
		a.addErrorAtToken(expr.Token, "cannot infer generic arguments for %s", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if receiverError != "" {
		a.addErrorAtToken(expr.Token, "%s", receiverError)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	for _, function := range arityMatches {
		displayName := name
		if hadExplicitGenericCall {
			if len(function.GenericParameters) == 0 {
				continue
			}
			argTypes := a.callArgumentTypesForFunction(function, sourceArgTypes, methodReceiver, isMethodCall)
			instantiated, _, ok := a.explicitGenericFunctionInstance(function, expr.GenericArguments, argTypes, Type{})
			if !ok {
				continue
			}
			function = instantiated
			displayName = genericFunctionDisplayName(name, function)
		} else if len(function.GenericParameters) > 0 {
			argTypes := a.callArgumentTypesForFunction(function, sourceArgTypes, methodReceiver, isMethodCall)
			instantiated, ok := a.inferGenericFunctionInstance(function, argTypes)
			if !ok {
				continue
			}
			function = instantiated
			displayName = genericFunctionDisplayName(name, function)
		}
		argTypes := a.callArgumentTypesForFunction(function, sourceArgTypes, methodReceiver, isMethodCall)
		for i := range argTypes {
			arg := sourceArgs[i]
			param, parameterOK := functionParameterForArgument(function, i)
			if !parameterOK {
				break
			}
			argType := a.contextualCallArgumentType(arg, argTypes[i], param.Type)
			if !a.canInitialize(param.Type, argType, arg) {
				a.addTypeMismatchError(expressionToken(arg), param.Type, argType, arg, "argument %d to %s must be %s, got %s", i+1, displayName, typeDisplayName(param.Type), typeDisplayName(argType))
			}
		}
		break
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferCompilerKnownFunction(expr *ast.CallExpression) (Type, expressionValue, bool) {
	name := callExpressionName(expr)
	knownFunction, known := compilerKnownFunction(name)
	if !known {
		return Type{}, expressionValue{}, false
	}
	if knownFunction.Internal {
		return a.inferCompilerInternalFunction(expr, knownFunction)
	}
	if name == "fill" {
		expected, hasExpected := a.expectedExpressionTypes[expr]
		return a.inferCompilerKnownFill(expr, expected, hasExpected)
	}
	if len(expr.GenericArguments) > 0 {
		a.addErrorAtToken(expr.Token, "len infers its element type from its argument")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "len expects 1 argument, got %d", len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	argumentType, _ := a.inferExpression(expr.Arguments[0])
	if argumentType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if compilerKnownSequenceType(argumentType) {
		return a.types["int"], expressionValue{Display: expr.String()}, true
	}

	a.addErrorAtToken(expressionToken(expr.Arguments[0]), "len requires a compiler-known sequence or collection, got %s", typeDisplayName(argumentType))
	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
}

func (a *Analyzer) inferCompilerInternalFunction(expr *ast.CallExpression, known CompilerKnownFunction) (Type, expressionValue, bool) {
	result := expressionValue{Display: expr.String()}
	if !a.isTrustedCoreSourceToken(expr.Token) {
		a.addErrorAtToken(expr.Token, "%s is a compiler-internal operation available only to privileged core source", known.Name)
		return Type{Kind: InvalidType}, result, true
	}
	if known.OwnerFile != "" && !sourceFileMatchesOwner(expr.Token.File, known.OwnerFile) {
		a.addErrorAtToken(expr.Token, "%s is private to %s", known.Name, known.OwnerFile)
		return Type{Kind: InvalidType}, result, true
	}
	if len(expr.GenericArguments) > 0 {
		a.addErrorAtToken(expr.Token, "%s does not accept generic arguments", known.Name)
		return Type{Kind: InvalidType}, result, true
	}
	if len(expr.Arguments) != len(known.Parameters) {
		a.addErrorAtToken(expr.Token, "%s expects %d arguments, got %d", known.Name, len(known.Parameters), len(expr.Arguments))
		return Type{Kind: InvalidType}, result, true
	}

	valid := true
	for index, parameter := range known.Parameters {
		actual, _ := a.inferExpressionWithExpected(expr.Arguments[index], parameter.Type)
		if !a.canInitialize(parameter.Type, actual, expr.Arguments[index]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[index]), "argument %d to %s must be %s, got %s", index+1, known.Name, typeDisplayName(parameter.Type), typeDisplayName(actual))
			valid = false
		}
	}
	if !valid {
		return Type{Kind: InvalidType}, result, true
	}
	return a.refinedCompilerKnownType(known.Result), result, true
}

func (a *Analyzer) inferCompilerKnownFill(expr *ast.CallExpression, expected Type, hasExpected bool) (Type, expressionValue, bool) {
	result := expressionValue{Display: expr.String()}
	if len(expr.GenericArguments) > 0 {
		a.addErrorAtToken(expr.Token, "fill uses target context and does not accept generic arguments")
		return Type{Kind: InvalidType}, result, true
	}
	if !hasExpected || expected.Kind == InvalidType || expected.Kind == "" {
		a.addErrorAtToken(expr.Token, "fill requires an explicit array or string target type")
		return Type{Kind: InvalidType}, result, true
	}
	if expected.Kind == ArrayType && expected.Element != nil {
		if arrayShapeOf(expected) == ArrayShapeFixed {
			if !a.checkCompilerKnownCallArity(expr, "fill", 1, 1) {
				return Type{Kind: InvalidType}, result, true
			}
			valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], *expected.Element)
			if !a.canInitialize(*expected.Element, valueType, expr.Arguments[0]) {
				a.addErrorAtToken(expressionToken(expr.Arguments[0]), "fill value must be %s, got %s", typeDisplayName(*expected.Element), typeDisplayName(valueType))
				return Type{Kind: InvalidType}, result, true
			}
			return expected, result, true
		}
		if !a.checkCompilerKnownCallArity(expr, "fill", 2, 2) {
			return Type{Kind: InvalidType}, result, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], *expected.Element)
		if !a.canInitialize(*expected.Element, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "fill value must be %s, got %s", typeDisplayName(*expected.Element), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, result, true
		}
		if !a.compilerKnownCountArgument(expr.Arguments[1], "fill") {
			return Type{Kind: InvalidType}, result, true
		}
		return compilerKnownResult(expected, a.types["CollectionError"]), result, true
	}
	if expected.Kind == StringType {
		if !a.checkCompilerKnownCallArity(expr, "fill", 2, 2) {
			return Type{Kind: InvalidType}, result, true
		}
		fragmentType, _ := a.inferExpressionWithExpected(expr.Arguments[0], expected)
		if fragmentType.Kind != StringType && fragmentType.Kind != CharType && fragmentType.Kind != RuneType {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "string fill fragment must be string, char, or rune, got %s", typeDisplayName(fragmentType))
			return Type{Kind: InvalidType}, result, true
		}
		if !a.compilerKnownCountArgument(expr.Arguments[1], "fill") {
			return Type{Kind: InvalidType}, result, true
		}
		return compilerKnownResult(expected, a.types["CollectionError"]), result, true
	}
	a.addErrorAtToken(expr.Token, "fill target must be a fixed array, owning dynamic array, or string, got %s", typeDisplayName(expected))
	return Type{Kind: InvalidType}, result, true
}

func (a *Analyzer) inferCompilerKnownMemberCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	memberExpr, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || memberExpr == nil || memberExpr.Property == nil {
		return Type{}, expressionValue{}, false
	}

	if path, static := typePathFromExpression(memberExpr.Object); static {
		typ, exists := a.types[a.resolveTypeName(path)]
		if exists {
			member, memberExists := compilerKnownMember(typ, memberExpr.Property.Value, true)
			if !memberExists || (member.Kind != CompilerKnownMethod && member.Kind != CompilerKnownAssociatedFunction) {
				return Type{}, expressionValue{}, false
			}
			a.compilerKnownMemberFacts[sourceTokenLocation(memberExpr.Property.Token)] = member
			switch member.Name {
			case "FromByteArray", "FromRuneArray":
				return a.inferCompilerKnownStringConstructor(expr, member)
			case "FromBuffer", "WithCapacity", "Growable":
				return a.inferArenaConstructorCall(expr, member)
			}
			return Type{}, expressionValue{}, false
		}
		if root := compilerKnownReceiverRoot(memberExpr.Object); root != nil {
			if _, isValue := a.symbols[root.Value]; !isValue {
				return Type{}, expressionValue{}, false
			}
		}
	}

	receiverType, _ := a.inferExpression(memberExpr.Object)
	if receiverType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	lookupType := dereferenceType(receiverType)
	member, exists := compilerKnownMember(receiverType, memberExpr.Property.Value, false)
	if !exists || member.Kind != CompilerKnownMethod {
		return Type{}, expressionValue{}, false
	}
	// Interface requirements use ordinary overload/conformance validation.
	// Publish identity only after successful resolution in recordResolvedCall.
	// Rules: rules/declarations/interfaces.md — §§6,9.1.
	for _, requirement := range lookupType.InterfaceMethods {
		if requirement.CompilerKnownID == member.ID {
			return Type{}, expressionValue{}, false
		}
	}
	// A trusted core impl on a compiler-known primitive participates in the
	// same overload set as methods on declared nominal types. In particular,
	// byte.ToString(ByteStringFormat) must be considered before the universal
	// zero-argument/numeric-string fallback even though byte is not Named.
	//
	// Rules:
	//   - rules/compiler/compiler_known_members.md — "Lookup order"
	//   - rules/compiler/compiler_known_members.md — "User-defined ToString()"
	if len(a.functions[lookupType.Name+"."+member.Name]) > 0 && (lookupType.Named || member.Name == "ToString") {
		if member.Name != "ToString" {
			return Type{}, expressionValue{}, false
		}
		if _, replaced := a.exactUserToStringReplacement(lookupType); replaced {
			return Type{}, expressionValue{}, false
		}
		if len(expr.Arguments) > 0 {
			return Type{}, expressionValue{}, false
		}
	}
	a.compilerKnownMemberFacts[sourceTokenLocation(memberExpr.Property.Token)] = member
	if member.StructuralMutation && a.structurallyMutatesIteratedCollection(memberExpr.Object, member.Name, memberExpr.Property.Token) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if lookupType.Kind == RawPtrType || lookupType.Name == "Arena" {
		return Type{}, expressionValue{}, false
	}
	switch member.Name {
	case "Ok", "Err":
		return a.inferConsumingResultProjection(expr, memberExpr, receiverType, lookupType, member)
	case "Borrow", "BorrowMut", "Replace":
		if lookupType.Name == "ThreadLocal" {
			return a.inferThreadLocalCall(expr, lookupType, member)
		}
		return Type{}, expressionValue{}, false
	case "ToString":
		maxArguments := 0
		if isNumericType(lookupType) {
			maxArguments = 1
		}
		if !a.checkCompilerKnownCallArity(expr, typeDisplayName(lookupType)+".ToString", 0, maxArguments) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) == 1 {
			formatType, _ := a.inferExpressionWithExpected(expr.Arguments[0], a.types["string"])
			if formatType.Kind != StringType {
				a.addErrorAtToken(expressionToken(expr.Arguments[0]), "ToString format must be string, got %s", typeDisplayName(formatType))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "ToByteArray", "ToCharArray", "ToRuneArray", "Clear", "Reverse", "Sort", "RequestCancel", "Observe", "Start":
		if !a.checkCompilerKnownCallArity(expr, typeDisplayName(lookupType)+"."+member.Name, 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "Append", "Fill":
		if !a.checkCompilerKnownCallArity(expr, typeDisplayName(lookupType)+"."+member.Name, 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		elementType, hasElement := compilerKnownCollectionElement(lookupType)
		if !hasElement {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if segment, isRange := expr.Arguments[0].(*ast.RangeExpression); isRange {
			// rules/collections/collections.md § 6.7: Append on an owning
			// dynamic array accepts a range segment and appends its values
			// in order, all or nothing.
			if member.ID != "CKM-DYNAMIC-ARRAY-APPEND" {
				a.addErrorAtTokenWithMetadata(segment.Token, diagnostics.ArrayRangeSegmentInvalid,
					"Only Append on an owning dynamic array T[] accepts a range; append the values one at a time.",
					"%s.%s does not accept a range", typeDisplayName(lookupType), member.Name)
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
			entry, ok := a.resolveRangeSegment(segment, elementType)
			if !ok {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
			a.resolvedRangeAppends[expr] = entry
			return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], elementType)
		if !a.canInitialize(elementType, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "%s value must be %s, got %s", member.Name, typeDisplayName(elementType), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		classification := CopyClassificationOf(elementType)
		if member.Name == "Fill" && classification != CopyTrivial && classification != CopySemantic {
			a.addErrorAtToken(memberExpr.Property.Token, "Fill requires a copyable element type, got %s", typeDisplayName(elementType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "RemoveAt":
		if !a.checkCompilerKnownCallArity(expr, typeDisplayName(lookupType)+".RemoveAt", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.compilerKnownCountArgument(expr.Arguments[0], "RemoveAt") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "Insert":
		if !a.checkCompilerKnownCallArity(expr, "list.Insert", 2, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.compilerKnownCountArgument(expr.Arguments[0], "Insert") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		elementType, ok := compilerKnownCollectionElement(lookupType)
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[1], elementType)
		if !a.canInitialize(elementType, valueType, expr.Arguments[1]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[1]), "Insert value must be %s, got %s", typeDisplayName(elementType), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "Remove", "Contains", "IndexOf", "Add":
		if !a.checkCompilerKnownCallArity(expr, typeDisplayName(lookupType)+"."+member.Name, 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		argumentType, ok := compilerKnownCollectionElement(lookupType)
		if lookupType.Name == "map" && len(lookupType.TypeArgs) == 2 {
			argumentType, ok = lookupType.TypeArgs[0], true
		}
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], argumentType)
		if !a.canInitialize(argumentType, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "%s argument must be %s, got %s", member.Name, typeDisplayName(argumentType), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "ContainsKey":
		if !a.checkCompilerKnownCallArity(expr, "map.ContainsKey", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(lookupType.TypeArgs) != 2 {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		keyType := lookupType.TypeArgs[0]
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], keyType)
		if !a.canInitialize(keyType, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "ContainsKey argument must be %s, got %s", typeDisplayName(keyType), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "SortBy":
		if !a.checkCompilerKnownCallArity(expr, "list.SortBy", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		compareType, _ := a.inferExpression(expr.Arguments[0])
		if compareType.Kind != FunctionType {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "SortBy requires a comparison function, got %s", typeDisplayName(compareType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "Union", "Intersection", "Difference", "SymmetricDifference":
		if !a.checkCompilerKnownCallArity(expr, "set."+member.Name, 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		otherType, _ := a.inferExpressionWithExpected(expr.Arguments[0], lookupType)
		if !sameConcreteType(lookupType, otherType) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "%s argument must be %s, got %s", member.Name, typeDisplayName(lookupType), typeDisplayName(otherType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		elementType, ok := compilerKnownCollectionElement(lookupType)
		classification := CopyClassificationOf(elementType)
		if !ok || classification != CopyTrivial && classification != CopySemantic {
			a.addErrorAtToken(memberExpr.Property.Token, "%s requires a copyable set element type", member.Name)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func (a *Analyzer) inferCompilerKnownStringConstructor(expr *ast.CallExpression, member CompilerKnownMember) (Type, expressionValue, bool) {
	if !a.checkCompilerKnownCallArity(expr, "string."+member.Name, 1, 1) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	argumentType, _ := a.inferExpression(expr.Arguments[0])
	sequence := dereferenceType(argumentType)
	want := "byte"
	if member.Name == "FromRuneArray" {
		want = "rune"
	}
	if (sequence.Kind != ArrayType && sequence.Kind != SliceType) || sequence.Element == nil || sequence.Element.Name != want {
		a.addErrorAtToken(expressionToken(expr.Arguments[0]), "string.%s requires a %s array or slice, got %s", member.Name, want, typeDisplayName(argumentType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
}

// inferRawPointerCall validates compiler-known raw-address operations. Unsafe
// authorizes the operation class but does not manufacture a pointee type for
// RawPtr[void] or waive the operation's concrete element requirements.
//
// Rules:
//   - rules/memory/raw_pointers.md — §3(3)–(4) "RawPtr[void]"
//   - rules/memory/raw_pointers.md — §11 "Raw-pointer read"
//   - rules/memory/raw_pointers.md — §12 "Raw-pointer write"
//   - rules/memory/raw_pointers.md — §13 "Volatile raw-pointer access"
//   - rules/memory/raw_pointers.md — §14 "Pointer arithmetic"
func (a *Analyzer) inferRawPointerCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	if !a.rawPointerCallMayHaveValueReceiver(member.Object) {
		return Type{}, expressionValue{}, false
	}
	receiverType, _ := a.inferExpression(member.Object)
	if receiverType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	receiverType = dereferenceType(receiverType)
	if receiverType.Kind != RawPtrType {
		return Type{}, expressionValue{}, false
	}
	knownMember, known := compilerKnownMember(receiverType, member.Property.Value, false)

	switch member.Property.Value {
	case "Read":
		if !a.inUnsafe {
			a.addErrorAtToken(member.Property.Token, "RawPtr.Read requires unsafe because it reads through a raw pointer")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.checkCompilerKnownCallArity(expr, "RawPtr.Read", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		element := compilerKnownRawPointerElement(receiverType)
		if element.Kind == InvalidType || element.Kind == VoidType {
			a.addErrorAtToken(member.Property.Token, "RawPtr[void].Read cannot materialize a value; select a concrete pointee type")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return element, expressionValue{Display: expr.String()}, true
	case "Write":
		if !a.inUnsafe {
			a.addErrorAtToken(member.Property.Token, "RawPtr.Write requires unsafe because it writes through a raw pointer")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.checkCompilerKnownCallArity(expr, "RawPtr.Write", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		element := compilerKnownRawPointerElement(receiverType)
		if element.Kind == InvalidType || element.Kind == VoidType {
			a.addErrorAtToken(member.Property.Token, "RawPtr[void].Write cannot consume a value; select a concrete pointee type")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], element)
		if !a.canInitialize(element, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "RawPtr.Write value must be %s, got %s", typeDisplayName(element), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.types["void"], expressionValue{Display: expr.String()}, true
	case "VolatileRead":
		if !a.inUnsafe {
			a.addErrorAtToken(member.Property.Token, "RawPtr.VolatileRead requires unsafe because it performs observable physical-storage access")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.checkCompilerKnownCallArity(expr, "RawPtr.VolatileRead", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		element := compilerKnownRawPointerElement(receiverType)
		if element.Kind == InvalidType || element.Kind == VoidType {
			a.addErrorAtToken(member.Property.Token, "RawPtr[void].VolatileRead cannot materialize a value; select a concrete physical pointee type")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if known {
			a.recordCompilerKnownEffects(knownMember, member.Property.Token)
		}
		return element, expressionValue{Display: expr.String()}, true
	case "VolatileWrite":
		if !a.inUnsafe {
			a.addErrorAtToken(member.Property.Token, "RawPtr.VolatileWrite requires unsafe because it performs observable physical-storage access")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.checkCompilerKnownCallArity(expr, "RawPtr.VolatileWrite", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		element := compilerKnownRawPointerElement(receiverType)
		if element.Kind == InvalidType || element.Kind == VoidType {
			a.addErrorAtToken(member.Property.Token, "RawPtr[void].VolatileWrite cannot consume a value; select a concrete physical pointee type")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], element)
		if !a.canInitialize(element, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "RawPtr.VolatileWrite value must be %s, got %s", typeDisplayName(element), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if known {
			a.recordCompilerKnownEffects(knownMember, member.Property.Token)
		}
		return a.types["void"], expressionValue{Display: expr.String()}, true
	case "Offset":
		if !a.rawPointerOperationRequiresUnsafe(member.Property.Token, "RawPtr.Offset") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "RawPtr.Offset expects 1 argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(receiverType.TypeArgs) == 1 && receiverType.TypeArgs[0].Kind == VoidType {
			a.addErrorAtToken(member.Property.Token, "RawPtr.Offset requires a typed element pointer, got RawPtr[void]")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.rawPointerArgumentIsInt(expr.Arguments[0], "RawPtr.Offset") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return receiverType, expressionValue{Display: expr.String()}, true
	case "AddBytes":
		if !a.rawPointerOperationRequiresUnsafe(member.Property.Token, "RawPtr.AddBytes") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "RawPtr.AddBytes expects 1 argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !isRawBytePointer(receiverType) {
			a.addErrorAtToken(member.Property.Token, "RawPtr.AddBytes requires RawPtr[byte], got %s", typeDisplayName(receiverType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.rawPointerArgumentIsInt(expr.Arguments[0], "RawPtr.AddBytes") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return receiverType, expressionValue{Display: expr.String()}, true
	case "Difference":
		if !a.rawPointerOperationRequiresUnsafe(member.Property.Token, "RawPtr.Difference") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "RawPtr.Difference expects 1 argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(receiverType.TypeArgs) != 1 || receiverType.TypeArgs[0].Kind == VoidType {
			a.addErrorAtToken(member.Property.Token, "RawPtr.Difference requires a typed element pointer, got %s", typeDisplayName(receiverType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		otherType, _ := a.inferExpression(expr.Arguments[0])
		if otherType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !sameConcreteType(receiverType, otherType) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "RawPtr.Difference argument must be %s, got %s", typeDisplayName(receiverType), typeDisplayName(otherType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.types["int"], expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func (a *Analyzer) inferCompilerKnownConstructor(expr *ast.CallExpression) (Type, expressionValue, bool) {
	name := callExpressionName(expr)
	if name == "Channel" {
		if len(expr.GenericArguments) != 1 {
			a.addErrorAtToken(expr.Token, "Channel requires exactly 1 message type, got %d", len(expr.GenericArguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "Channel expects 1 capacity argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		messageType, ok := a.resolveType(expr.GenericArguments[0])
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		capacityType, _ := a.inferExpression(expr.Arguments[0])
		if capacityType.Kind != InvalidType && !isIntegerType(capacityType) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Channel capacity must be integer, got %s", typeDisplayName(capacityType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return channelType(messageType), expressionValue{Display: expr.String()}, true
	}
	if name != "Mutex" && name != "Atomic" {
		return Type{}, expressionValue{}, false
	}
	if len(expr.GenericArguments) > 0 {
		a.addErrorAtToken(expr.Token, "%s constructor infers its type argument from the initializer", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "%s expects 1 argument, got %d", name, len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	valueType, _ := a.inferExpression(expr.Arguments[0])
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if name == "Atomic" && !atomicElementTypeSupported(valueType) {
		a.addErrorAtToken(expressionToken(expr.Arguments[0]), "type %s is not supported by Atomic", typeDisplayName(valueType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	return Type{Name: name, Kind: StructType, TypeArgs: []Type{valueType}}, expressionValue{Display: expr.String()}, true
}

func (a *Analyzer) inferEventCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return Type{}, expressionValue{}, false
	}
	if member.Property.Value != "Publish" && member.Property.Value != "Subscribe" {
		return Type{}, expressionValue{}, false
	}
	receiver, ok := a.eventReceiverInfo(member.Object)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	payload := receiver.Type.TypeArgs[0]
	switch member.Property.Value {
	case "Publish":
		if !a.checkCompilerKnownCallArity(expr, "Event.Publish", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if receiver.Owner == "" || receiver.Owner != a.currentImplTarget {
			owner := receiver.Owner
			if owner == "" {
				owner = "the owning type"
			}
			a.addErrorAtToken(member.Property.Token, "event %s may only be published by %s", receiver.Name, owner)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		argType, _ := a.inferExpressionWithExpected(expr.Arguments[0], payload)
		if argType.Kind != InvalidType && !a.canInitialize(payload, argType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Event.Publish payload must be %s, got %s", typeDisplayName(payload), typeDisplayName(argType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	case "Subscribe":
		if !a.checkCompilerKnownCallArity(expr, "Event.Subscribe", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		handlerType, _ := a.inferExpression(expr.Arguments[0])
		expected := eventHandlerType(payload)
		if handlerType.Kind != InvalidType && !sameFunctionType(handlerType, expected) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Event.Subscribe handler must be %s, got %s", typeDisplayName(expected), typeDisplayName(handlerType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.types["EventSubscribeResult"], expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func (a *Analyzer) inferSubscriptionCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil || member.Property.Value != "Close" {
		return Type{}, expressionValue{}, false
	}
	if ident, ok := member.Object.(*ast.Identifier); ok {
		symbol, exists := a.symbols[ident.Value]
		if !exists || !isSubscriptionType(symbol.Type) {
			return Type{}, expressionValue{}, false
		}
	} else {
		receiverType, _ := a.inferExpression(member.Object)
		if !isSubscriptionType(receiverType) {
			return Type{}, expressionValue{}, false
		}
	}
	receiverType, _ := a.inferExpression(member.Object)
	if !isSubscriptionType(receiverType) {
		return Type{}, expressionValue{}, false
	}
	if !a.checkCompilerKnownCallArity(expr, "Subscription.Close", 0, 0) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	a.markMoveSource(member.Object)
	return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
}

func (a *Analyzer) inferChannelCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return Type{}, expressionValue{}, false
	}
	receiverType, ok := a.compilerKnownReceiverType(member.Object)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	if isSenderType(receiverType) {
		return a.inferSenderCall(expr, member, receiverType)
	}
	if isReceiverType(receiverType) {
		return a.inferReceiverCall(expr, member, receiverType)
	}
	return Type{}, expressionValue{}, false
}

func (a *Analyzer) inferSenderCall(expr *ast.CallExpression, member *ast.MemberExpression, receiverType Type) (Type, expressionValue, bool) {
	messageType := receiverType.TypeArgs[0]
	switch member.Property.Value {
	case "Share":
		if !a.checkCompilerKnownCallArity(expr, "Sender.Share", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return senderType(messageType), expressionValue{Display: expr.String()}, true
	case "Send":
		if !a.checkCompilerKnownCallArity(expr, "Sender.Send", 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.recordBlockingOperation("Sender.Send", member.Property.Token)
		messageArgType, _ := a.inferExpressionWithExpected(expr.Arguments[0], messageType)
		if messageArgType.Kind != InvalidType && !a.canInitialize(messageType, messageArgType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Sender.Send message must be %s, got %s", typeDisplayName(messageType), typeDisplayName(messageArgType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) == 2 {
			lifetimeType, _ := a.inferExpression(expr.Arguments[1])
			if lifetimeType.Kind != InvalidType && !isNumericType(lifetimeType) {
				a.addErrorAtToken(expressionToken(expr.Arguments[1]), "Sender.Send message lifetime must be duration-compatible numeric value, got %s", typeDisplayName(lifetimeType))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
		}
		a.markMoveSource(expr.Arguments[0])
		return a.intrinsicGenericType("ChannelSendResult", messageType), expressionValue{Display: expr.String()}, true
	case "SendRevocable":
		if !a.checkCompilerKnownCallArity(expr, "Sender.SendRevocable", 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.recordBlockingOperation("Sender.SendRevocable", member.Property.Token)
		messageArgType, _ := a.inferExpressionWithExpected(expr.Arguments[0], messageType)
		if messageArgType.Kind != InvalidType && !a.canInitialize(messageType, messageArgType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Sender.SendRevocable message must be %s, got %s", typeDisplayName(messageType), typeDisplayName(messageArgType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) == 2 {
			lifetimeType, _ := a.inferExpression(expr.Arguments[1])
			if lifetimeType.Kind != InvalidType && !isNumericType(lifetimeType) {
				a.addErrorAtToken(expressionToken(expr.Arguments[1]), "Sender.SendRevocable message lifetime must be duration-compatible numeric value, got %s", typeDisplayName(lifetimeType))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
		}
		a.markMoveSource(expr.Arguments[0])
		return messageTicketType(messageType), expressionValue{Display: expr.String()}, true
	case "TrySend":
		if !a.checkCompilerKnownCallArity(expr, "Sender.TrySend", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		messageArgType, _ := a.inferExpressionWithExpected(expr.Arguments[0], messageType)
		if messageArgType.Kind != InvalidType && !a.canInitialize(messageType, messageArgType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Sender.TrySend message must be %s, got %s", typeDisplayName(messageType), typeDisplayName(messageArgType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.markMoveSource(expr.Arguments[0])
		return a.intrinsicGenericType("ChannelSendResult", messageType), expressionValue{Display: expr.String()}, true
	case "Revoke":
		if !a.checkCompilerKnownCallArity(expr, "Sender.Revoke", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		ticketType, _ := a.inferExpressionWithExpected(expr.Arguments[0], messageTicketType(messageType))
		if ticketType.Kind != InvalidType && !sameConcreteType(ticketType, messageTicketType(messageType)) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Sender.Revoke ticket must be MessageTicket[%s], got %s", typeDisplayName(messageType), typeDisplayName(ticketType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.markMoveSource(expr.Arguments[0])
		return a.intrinsicGenericType("ChannelRevokeResult", messageType), expressionValue{Display: expr.String()}, true
	case "Close":
		if !a.checkCompilerKnownCallArity(expr, "Sender.Close", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.markMoveSource(member.Object)
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func (a *Analyzer) inferReceiverCall(expr *ast.CallExpression, member *ast.MemberExpression, receiverType Type) (Type, expressionValue, bool) {
	messageType := receiverType.TypeArgs[0]
	switch member.Property.Value {
	case "Receive":
		if !a.checkCompilerKnownCallArity(expr, "Receiver.Receive", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.recordBlockingOperation("Receiver.Receive", member.Property.Token)
		return a.intrinsicGenericType("Option", messageType), expressionValue{Display: expr.String()}, true
	case "TryReceive":
		if !a.checkCompilerKnownCallArity(expr, "Receiver.TryReceive", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.intrinsicGenericType("ChannelTryReceiveResult", messageType), expressionValue{Display: expr.String()}, true
	case "Discard":
		if !a.checkCompilerKnownCallArity(expr, "Receiver.Discard", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	case "Close":
		if !a.checkCompilerKnownCallArity(expr, "Receiver.Close", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.markMoveSource(member.Object)
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func (a *Analyzer) inferAtomicCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return Type{}, expressionValue{}, false
	}
	receiverType, ok := a.compilerKnownReceiverType(member.Object)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	if !isAtomicType(receiverType) {
		return Type{}, expressionValue{}, false
	}
	element := receiverType.TypeArgs[0]
	switch member.Property.Value {
	case "load":
		if !a.checkAtomicCallArity(expr, "Atomic.load", 0, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return element, expressionValue{Display: expr.String()}, true
	case "store":
		if !a.checkAtomicCallArity(expr, "Atomic.store", 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	case "swap":
		if !a.checkAtomicCallArity(expr, "Atomic.swap", 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		return element, expressionValue{Display: expr.String()}, true
	case "fetchAdd", "fetchSub", "fetchAnd", "fetchOr", "fetchXor":
		if !a.checkAtomicCallArity(expr, "Atomic."+member.Property.Value, 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !atomicFetchOperationSupported(member.Property.Value, element) {
			a.addErrorAtToken(member.Property.Token, "%s is not supported for Atomic[%s]", member.Property.Value, typeDisplayName(element))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		return element, expressionValue{Display: expr.String()}, true
	case "compareExchange":
		if !a.checkAtomicCallArity(expr, "Atomic.compareExchange", 2, 4) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		a.checkAtomicValueArgument(expr, element, 1)
		return Type{Name: "CompareExchangeResult", Kind: StructType, TypeArgs: []Type{element}}, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func (a *Analyzer) inferCallAsUnionVariantConstructor(expr *ast.CallExpression, expected *Type) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok {
		return Type{}, expressionValue{}, false
	}

	typeName, ok := typePathFromExpression(member.Object)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	typeName = a.resolveTypeName(typeName)

	template, ok := a.types[typeName]
	if !ok || template.Kind != UnionType {
		return Type{}, expressionValue{}, false
	}

	variant, ok := lookupUnionVariant(template, member.Property.Value)
	if !ok {
		a.addErrorAtToken(member.Property.Token, "unknown union variant %s.%s", template.Name, member.Property.Value)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	a.bindDefinition(member.Property.Token, variant.Token)

	unionType := template
	if len(expr.GenericArguments) > 0 {
		explicit, ok := a.resolveType(&ast.TypeReference{
			Token:    expr.Token,
			Name:     typeName,
			TypeArgs: expr.GenericArguments,
		})
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		unionType = explicit
	} else if expected != nil && expected.Kind == UnionType && expected.Name == template.Name {
		unionType = *expected
	} else if len(template.GenericParameters) > 0 {
		// Generic inference observes the same owning-payload context as the
		// subsequent concrete payload check, including explicit <- syntax.
		a.constructionOwnershipDepth++
		concrete, ok := a.inferGenericUnionVariantInstance(template, variant, expr)
		a.constructionOwnershipDepth--
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		unionType = concrete
	}

	concreteVariant, ok := lookupUnionVariant(unionType, member.Property.Value)
	if !ok {
		a.addErrorAtToken(member.Property.Token, "unknown union variant %s.%s", unionType.Name, member.Property.Value)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	if len(concreteVariant.PayloadFields) > 0 {
		a.addErrorAtToken(expr.Token, "union variant %s.%s requires named payload fields", typeDisplayName(unionType), concreteVariant.Name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	if concreteVariant.Payload == nil {
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "union variant %s.%s expects 0 arguments, got %d", typeDisplayName(unionType), concreteVariant.Name, len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return unionType, expressionValue{Display: expr.String()}, true
	}

	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "union variant %s.%s expects 1 argument, got %d", typeDisplayName(unionType), concreteVariant.Name, len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	payloadType := *concreteVariant.Payload
	valueType, _ := a.inferOwningConstructionValue(expr.Arguments[0], payloadType)
	if valueType.Kind != InvalidType && !a.canInitialize(payloadType, valueType, expr.Arguments[0]) {
		a.addErrorAtToken(expressionToken(expr.Arguments[0]), "union variant %s.%s payload must be %s, got %s", typeDisplayName(unionType), concreteVariant.Name, typeDisplayName(payloadType), typeDisplayName(valueType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if valueType.Kind != InvalidType && a.checkCompileTimeContractExpression(payloadType, expr.Arguments[0]) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if valueType.Kind != InvalidType && !a.validateOwningConstructionSource(expr.Arguments[0]) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	return unionType, expressionValue{Display: expr.String()}, true
}

func (a *Analyzer) inferGenericUnionVariantInstance(template Type, variant UnionVariant, expr *ast.CallExpression) (Type, bool) {
	if variant.Payload == nil {
		a.addErrorAtToken(expr.Token, "cannot infer generic arguments for %s.%s", template.Name, variant.Name)
		return Type{}, false
	}
	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "union variant %s.%s expects 1 argument, got %d", typeDisplayName(template), variant.Name, len(expr.Arguments))
		return Type{}, false
	}

	argType, _ := a.inferExpression(expr.Arguments[0])
	if argType.Kind == InvalidType {
		return Type{}, false
	}

	substitution := map[string]Type{}
	if !inferGenericTypeSubstitution(*variant.Payload, argType, substitution) {
		a.addErrorAtToken(expressionToken(expr.Arguments[0]), "cannot infer generic arguments for %s.%s", template.Name, variant.Name)
		return Type{}, false
	}

	typeArgs := make([]Type, 0, len(template.GenericParameters))
	for _, name := range template.GenericParameters {
		arg, ok := substitution[name]
		if !ok {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "cannot infer generic arguments for %s.%s", template.Name, variant.Name)
			return Type{}, false
		}
		typeArgs = append(typeArgs, arg)
	}

	concrete := template
	concrete.TypeArgs = typeArgs
	if !a.validateGenericTypeConstraintArguments(expr.Token, concrete) {
		return Type{}, false
	}
	return a.instantiateGenericType(concrete), true
}

func (a *Analyzer) inferCallExpressionWithExpected(expr *ast.CallExpression, expected Type) (Type, expressionValue, bool) {
	name := callExpressionName(expr)
	if name == "fill" {
		return a.inferCompilerKnownFill(expr, expected, true)
	}
	if name == "len" {
		return a.inferCompilerKnownFunction(expr)
	}
	functions, ok := a.functions[name]
	if !ok || len(functions) == 0 {
		if implName, implOK := a.implScopedFunctionName(name); implOK {
			if implFunctions := a.functions[implName]; len(implFunctions) > 0 {
				name = implName
				functions = implFunctions
				ok = true
			}
		}
	}
	if !ok || len(functions) == 0 {
		return Type{}, expressionValue{}, false
	}
	functions = a.accessibleFunctions(functions)
	if len(functions) == 0 {
		return Type{}, expressionValue{}, false
	}

	argTypes, args, _, runtimeSpreadValues, argsOK := a.callArgumentTypes(expr.Arguments, anyFunctionIsVariadic(functions))
	if !argsOK {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	matches := []overloadMatch{}
	var constraintFailure *GenericConstraintFailure
	for _, function := range functions {
		if !functionAcceptsCallArguments(function, len(argTypes), runtimeSpreadValues) || len(function.GenericParameters) == 0 {
			continue
		}
		var instantiated Function
		var ok bool
		if len(expr.GenericArguments) > 0 {
			instantiated, _, ok = a.explicitGenericFunctionInstance(function, expr.GenericArguments, argTypes, expected)
		} else {
			instantiated, ok = a.inferGenericFunctionInstanceWithExpected(function, argTypes, expected)
		}
		if !ok {
			continue
		}
		if instantiated.GenericConstraintFailure != nil {
			if constraintFailure == nil {
				failure := *instantiated.GenericConstraintFailure
				constraintFailure = &failure
			}
			continue
		}

		matchesArguments := true
		rank := 0
		for i, arg := range args {
			parameter, parameterOK := functionParameterForArgument(instantiated, i)
			if !parameterOK || !a.canInitializeUnrecorded(parameter.Type, argTypes[i], arg) {
				matchesArguments = false
				break
			}
			rank += overloadArgumentRank(parameter.Type, argTypes[i])
		}
		if matchesArguments {
			matches = append(matches, overloadMatch{Function: instantiated, Rank: rank})
		}
	}

	best := bestOverloadMatches(matches)
	if len(best) == 1 {
		if a.checkCompileTimeCallArgumentContracts(best[0].Function, args) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.recordCallArgumentUnitConversions(best[0].Function, args, argTypes)
		a.setDefinitions(callCalleeDefinitionToken(expr), best[0].Function.Token)
		a.setCallReferenceOrigin(expr, best[0].Function, args, false)
		return best[0].Function.ReturnType, expressionValue{Display: expr.String()}, true
	}
	if len(best) > 1 {
		ambiguous := make([]lexer.Token, 0, len(best))
		for _, match := range best {
			ambiguous = append(ambiguous, match.Function.Token)
		}
		a.bindDefinitions(callCalleeDefinitionToken(expr), ambiguous)
		a.addErrorAtToken(expr.Token, "ambiguous call to %s", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if constraintFailure != nil {
		a.addErrorAtToken(
			expr.Token,
			"type %s does not satisfy constraint %s for %s",
			typeDisplayName(constraintFailure.Argument),
			typeDisplayName(constraintFailure.Interface),
			constraintFailure.Parameter,
		)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	return Type{}, expressionValue{}, false
}

func (a *Analyzer) inferGenericFunctionInstance(function Function, argTypes []Type) (Function, bool) {
	if !functionAcceptsArgumentCount(function, len(argTypes)) {
		return Function{}, false
	}

	substitution := map[string]Type{}
	for i, argType := range argTypes {
		parameter, parameterOK := functionParameterForArgument(function, i)
		if !parameterOK || !inferGenericTypeSubstitution(parameter.Type, argType, substitution) {
			return Function{}, false
		}
	}
	for _, name := range function.GenericParameters {
		if _, ok := substitution[name]; !ok {
			return Function{}, false
		}
	}

	return a.instantiateGenericFunction(function, substitution), true
}

func (a *Analyzer) inferGenericFunctionInstanceWithExpected(function Function, argTypes []Type, expected Type) (Function, bool) {
	if !functionAcceptsArgumentCount(function, len(argTypes)) {
		return Function{}, false
	}

	substitution := map[string]Type{}
	for i, argType := range argTypes {
		parameter, parameterOK := functionParameterForArgument(function, i)
		if !parameterOK || !inferGenericTypeSubstitution(parameter.Type, argType, substitution) {
			return Function{}, false
		}
	}

	if expected.Kind != InvalidType && expected.Kind != "" {
		before := len(substitution)
		if !inferGenericTypeSubstitution(function.ReturnType, expected, substitution) {
			if before < len(function.GenericParameters) {
				return Function{}, false
			}
		}
	}

	for _, name := range function.GenericParameters {
		if _, ok := substitution[name]; !ok {
			return Function{}, false
		}
	}

	return a.instantiateGenericFunction(function, substitution), true
}

func (a *Analyzer) inferRuntimeCallExpression(expr *ast.RuntimeCallExpression) (Type, expressionValue) {
	// TODO: Replace hard-coded runtime hooks with proper runtime library metadata.
	switch expr.Name {
	case "runtime.PrintlnString":
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "@runtime.PrintlnString expects 1 argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		argType, _ := a.inferExpression(expr.Arguments[0])
		if argType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if argType.Kind != StringType {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "@runtime.PrintlnString argument must be string, got %s", typeDisplayName(argType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}
	default:
		a.addErrorAtToken(expr.Token, "unknown runtime function @%s", expr.Name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
}

func (a *Analyzer) inferCallAsConversion(expr *ast.CallExpression) (Type, expressionValue) {
	name := callExpressionName(expr)
	typeName := a.resolveTypeName(name)
	if _, exists := a.types[typeName]; !exists {
		a.addErrorAtToken(expr.Token, "unknown function or type %s", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	targetRef := &ast.TypeReference{
		Token:    expr.Token,
		Name:     name,
		TypeArgs: expr.GenericArguments,
	}
	targetType, ok := a.resolveType(targetRef)
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if len(expr.Arguments) == 2 && hasUnitSemantics(targetType) {
		return a.inferFactorProvidedUnitConversion(expr, targetType)
	}

	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "conversion to %s expects 1 argument, or 2 for a factor-provided unit conversion; got %d", name, len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	valueType, _ := a.inferExpression(expr.Arguments[0])
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if (targetType.Kind == RawPtrType || valueType.Kind == RawPtrType) && !a.inUnsafe {
		a.addErrorAtToken(expr.Token, "conversion involving RawPtr requires unsafe")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if !a.canExplicitConvert(targetType, valueType) {
		a.addErrorAtToken(expr.Token, "cannot convert %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if (hasUnitSemantics(targetType) || hasUnitSemantics(valueType)) && !a.validateExplicitUnitConversion(expr.Token, targetType, valueType) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if converted, ok := numericCarrierConversionResult(targetType, valueType); ok {
		return converted, expressionValue{Display: expr.String()}
	}
	if targetType.Kind == EnumType {
		if a.checkNominalMembershipValue(targetType, expr.Arguments[0]) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		conversionType, valid := a.enumConversionResultType(targetType, valueType, expr.Arguments[0])
		if !valid {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return conversionType, expressionValue{Display: expr.String()}
	}
	if targetType.Kind == RegisterType && isIntegerType(valueType) {
		return a.integerToRegisterConversionResultType(targetType, valueType, expr.Arguments[0]), expressionValue{Display: expr.String()}
	}
	if isIntegerType(targetType) && valueType.Kind == EnumType {
		return a.enumToIntegerConversionResultType(targetType, valueType, expr.Arguments[0]), expressionValue{Display: expr.String()}
	}
	if targetType.Kind == StringType && valueType.Kind == EnumType && valueType.Underlying == "string" {
		return targetType, expressionValue{Display: expr.String()}
	}
	if !a.validateConstantIntegerConversion(targetType, valueType, expr.Arguments[0]) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if a.checkEmptyListLiteralContracts(targetType, expr.Arguments[0]) || a.checkStringLiteralContracts(targetType, expr.Arguments[0]) || a.checkNominalMembershipValue(targetType, expr.Arguments[0]) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	a.expressionTypes[expr] = targetType
	if a.runtimeContractConversion(expr) {
		a.recordContractConversionEffect(expr)
	}

	return targetType, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferTryExpression(expr *ast.TryExpression) (Type, expressionValue) {
	var valueType Type
	if construction, ok := expr.Expression.(*ast.NewExpression); ok {
		valueType, _ = a.inferNewExpression(construction, true)
		if resolved, exists := a.resolvedConstructions[construction]; exists {
			a.expressionTypes[construction] = resolved.Target
		}
	} else {
		valueType, _ = a.inferExpression(expr.Expression)
	}
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	// rules/errors/errorhandling.md §11 and runtime_checks.md "What try
	// converts": every language-defined check in the protected subtree is a
	// failure point. Sets the dedicated paths below represent exactly keep
	// them; any other set uses the failure-set path.
	rootIsResult := valueType.Kind == ResultType && len(valueType.TypeArgs) == 2
	isOption := valueType.Kind == UnionType && valueType.Name == "Option" && len(valueType.TypeArgs) == 1
	points := a.collectTryFailurePoints(expr.Expression)
	if isOption && len(points) > 0 {
		a.addErrorAtTokenWithPrevious(expr.Token, points[0].Token,
			"try over an Option cannot also protect the %s from %s, because absence and errors use different channels; protect that operation with its own try",
			typeDisplayName(points[0].ErrorType), points[0].Expression.String())
		return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
	}
	if !isOption && !a.usesExistingTryPath(expr.Expression, rootIsResult, points) {
		successType := valueType
		if rootIsResult {
			successType = valueType.TypeArgs[0]
			points = append(points, TryFailurePoint{Kind: TryFailureCarrier, ErrorType: valueType.TypeArgs[1], Expression: expr.Expression, Token: expressionToken(expr.Expression)})
		}
		return a.inferFailureSetTryExpression(expr, successType, points)
	}

	if operator, ok := a.ResolvedOperatorOf(expr.Expression); ok && operator.RuntimeCheck {
		return a.inferArithmeticTryExpression(expr, operator)
	}
	if index, ok := expr.Expression.(*ast.IndexExpression); ok {
		if plan, exists := a.resolvedArrayIndexPlans[index]; exists {
			return a.inferBoundsTryExpression(expr, index, plan)
		}
	}
	if valueType.Kind == UnionType && valueType.Name == "Option" && len(valueType.TypeArgs) == 1 && len(expr.Handlers) == 0 {
		return a.inferNakedOptionTryExpression(expr, valueType)
	}
	if valueType.Kind == UnionType && valueType.Name == "Option" && len(valueType.TypeArgs) == 1 && len(expr.Handlers) > 0 {
		if a.rejectForbiddenOptionTrySuccessHandlers(expr) {
			return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
		}
		return a.inferHandledOptionTryExpression(expr, valueType)
	}

	if valueType.Kind != ResultType || len(valueType.TypeArgs) != 2 {
		a.addErrorAtToken(expr.Token, "try requires Result expression, Option expression, or a language-defined runtime check such as checked arithmetic, indexing, or a constrained conversion; %s contains none", expr.Expression.String())
		return valueType, expressionValue{Display: expr.String()}
	}

	if a.inDeferBlock && len(expr.Handlers) == 0 {
		a.addBodylessTryError(expr.Token, "bodyless try cannot propagate from inside defer; add a local try handler")
		return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
	}

	if len(expr.Handlers) > 0 {
		plan, valid := a.analyzeTryHandlers(expr, valueType)
		a.resolvedTries[expr] = ResolvedTry{
			Kind: ResolvedTryHandledResult, SuccessType: valueType.TypeArgs[0], ErrorType: valueType.TypeArgs[1],
		}
		if valid {
			a.resolvedTryPlans[expr] = plan
		}
		return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
	}

	if !a.inFunctionBody {
		a.addBodylessTryError(expr.Token, "bodyless try cannot propagate outside a function; add a local try handler")
		return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
	}

	if a.tryPropagatesToTestBoundary() {
		a.resolvedTries[expr] = ResolvedTry{
			Kind: ResolvedTryResultPropagation, SuccessType: valueType.TypeArgs[0], ErrorType: valueType.TypeArgs[1], TestBoundary: true,
		}
		return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
	}
	if a.currentFunctionReturn.Kind != ResultType || len(a.currentFunctionReturn.TypeArgs) != 2 {
		a.addBodylessTryError(
			expr.Token,
			"bodyless try propagates %s with return Err, but this function returns %s; add a local try handler or change the function return type to Result[%s, %s]",
			typeDisplayName(valueType.TypeArgs[1]),
			typeDisplayName(a.currentFunctionReturn),
			typeDisplayName(a.currentFunctionReturn),
			typeDisplayName(valueType.TypeArgs[1]),
		)
		return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
	}

	valueErrorType := valueType.TypeArgs[1]
	functionErrorType := a.currentFunctionReturn.TypeArgs[1]
	if !a.canInitialize(functionErrorType, valueErrorType, expr.Expression) {
		a.addBodylessTryError(expr.Token, "bodyless try propagates %s with return Err, but this function returns %s; add a local try handler or map %s to %s", typeDisplayName(valueErrorType), typeDisplayName(a.currentFunctionReturn), typeDisplayName(valueErrorType), typeDisplayName(functionErrorType))
	}
	a.resolvedTries[expr] = ResolvedTry{
		Kind: ResolvedTryResultPropagation, SuccessType: valueType.TypeArgs[0], ErrorType: valueErrorType,
		EnclosingResultType: a.currentFunctionReturn,
	}

	return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
}

// inferBoundsTryExpression converts a fixed-array runtime bounds check into
// typed IndexError flow. Rules: rules/errors/errorhandling.md section 10.3 and
// rules/mlir/packages/sec-mlir-dialect_package14.md sections 50-54.
func (a *Analyzer) inferBoundsTryExpression(expr *ast.TryExpression, index *ast.IndexExpression, plan ResolvedArrayIndexPlan) (Type, expressionValue) {
	result := expressionValue{Display: expr.String()}
	errorType := plan.ErrorType
	if errorType.Kind == InvalidType || errorType.Kind == "" {
		errorType = a.indexErrorType()
	}

	if len(expr.Handlers) != 0 {
		resultType := Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{plan.ElementType, errorType}}
		handlerPlan, valid := a.analyzeTryHandlers(expr, resultType)
		a.resolvedTries[expr] = ResolvedTry{Kind: ResolvedTryHandledBounds, SuccessType: plan.ElementType, ErrorType: errorType}
		if valid {
			a.commitFallibleBoundsIndex(index, plan, errorType)
			a.resolvedTryPlans[expr] = handlerPlan
		}
		return plan.ElementType, result
	}
	if a.inDeferBlock {
		a.addBodylessTryError(expr.Token, "bodyless try cannot propagate from inside defer; add a local try handler")
		return plan.ElementType, result
	}
	if !a.inFunctionBody {
		a.addBodylessTryError(expr.Token, "bodyless bounds try cannot propagate outside a function; add a local try handler")
		return plan.ElementType, result
	}
	if a.tryPropagatesToTestBoundary() {
		a.resolvedTries[expr] = ResolvedTry{Kind: ResolvedTryBoundsPropagation, SuccessType: plan.ElementType, ErrorType: errorType, TestBoundary: true}
		a.commitFallibleBoundsIndex(index, plan, errorType)
		return plan.ElementType, result
	}
	if a.currentFunctionReturn.Kind != ResultType || len(a.currentFunctionReturn.TypeArgs) != 2 {
		a.addBodylessTryError(expr.Token, "bodyless bounds try propagates IndexError with return Err, but this function returns %s", typeDisplayName(a.currentFunctionReturn))
		return plan.ElementType, result
	}
	functionError := a.currentFunctionReturn.TypeArgs[1]
	if !a.canInitialize(functionError, errorType, expr.Expression) {
		a.addBodylessTryError(expr.Token, "bodyless bounds try propagates IndexError with return Err, but this function returns %s", typeDisplayName(a.currentFunctionReturn))
		return plan.ElementType, result
	}
	a.resolvedTries[expr] = ResolvedTry{Kind: ResolvedTryBoundsPropagation, SuccessType: plan.ElementType, ErrorType: errorType, EnclosingResultType: a.currentFunctionReturn}
	a.commitFallibleBoundsIndex(index, plan, errorType)
	return plan.ElementType, result
}

func (a *Analyzer) inferArithmeticTryExpression(expr *ast.TryExpression, operator ResolvedOperator) (Type, expressionValue) {
	result := expressionValue{Display: expr.String()}
	arithmeticError := a.types["ArithmeticError"]
	if len(expr.Handlers) != 0 {
		resultType := Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{operator.ResultType, arithmeticError}}
		plan, valid := a.analyzeTryHandlers(expr, resultType)
		a.resolvedTries[expr] = ResolvedTry{
			Kind: ResolvedTryHandledArithmetic, SuccessType: operator.ResultType, ErrorType: arithmeticError,
		}
		if valid {
			a.resolvedTryPlans[expr] = plan
			a.resolveArithmeticFailureEffect(expr.Expression)
		}
		return operator.ResultType, result
	}
	if a.inDeferBlock {
		a.addBodylessTryError(expr.Token, "bodyless try cannot propagate from inside defer; add a local try handler")
		return operator.ResultType, result
	}
	if !a.inFunctionBody {
		a.addBodylessTryError(expr.Token, "bodyless arithmetic try cannot propagate outside a function; add a local try handler")
		return operator.ResultType, result
	}
	if a.tryPropagatesToTestBoundary() {
		a.resolvedTries[expr] = ResolvedTry{
			Kind: ResolvedTryArithmeticPropagation, SuccessType: operator.ResultType, ErrorType: arithmeticError, TestBoundary: true,
		}
		a.resolveArithmeticFailureEffect(expr.Expression)
		return operator.ResultType, result
	}
	if a.currentFunctionReturn.Kind != ResultType || len(a.currentFunctionReturn.TypeArgs) != 2 {
		a.addBodylessTryError(expr.Token, "bodyless arithmetic try propagates ArithmeticError with return Err, but this function returns %s; add a local try handler or change the function return type to Result[%s, ArithmeticError]", typeDisplayName(a.currentFunctionReturn), typeDisplayName(a.currentFunctionReturn))
		return operator.ResultType, result
	}
	functionError := a.currentFunctionReturn.TypeArgs[1]
	if !a.canInitialize(functionError, arithmeticError, expr.Expression) {
		a.addBodylessTryError(expr.Token, "bodyless arithmetic try propagates ArithmeticError with return Err, but this function returns %s; add a local try handler or map ArithmeticError to %s", typeDisplayName(a.currentFunctionReturn), typeDisplayName(functionError))
		return operator.ResultType, result
	}
	a.resolvedTries[expr] = ResolvedTry{
		Kind: ResolvedTryArithmeticPropagation, SuccessType: operator.ResultType,
		ErrorType: arithmeticError, EnclosingResultType: a.currentFunctionReturn,
	}
	a.resolveArithmeticFailureEffect(expr.Expression)
	return operator.ResultType, result
}

// inferMatchExpression maps a complete match to its resolved value type while
// treating parser-recovered typed-nil nodes as invalid editor input.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — match expressions
//   - rules/compiler/parser_recovery.md — "Invalid node" and tooling continuation
func (a *Analyzer) inferMatchExpression(expr *ast.MatchExpression) (Type, expressionValue) {
	if expr == nil {
		return Type{Kind: InvalidType}, expressionValue{}
	}
	typ := a.analyzeMatch(expr, true)
	return typ, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferInfixExpression(expr *ast.InfixExpression) (Type, expressionValue) {
	if a.rejectNullEquality(expr) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if isComparisonOperator(expr.Operator) && containsComparisonExpression(expr.Left) {
		a.addErrorAtToken(expr.Token, "comparison chaining is not supported")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	// MD-006 § 7.5: an ungrouped `is` test shares the equality level.
	if isComparisonOperator(expr.Operator) && (ungroupedStateTest(expr.Left) || ungroupedStateTest(expr.Right)) {
		a.addErrorAtToken(expr.Token, "comparison chaining is not supported; parenthesize the is state test before comparing it")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	a.adviseRedundantStateTestComparison(expr)

	leftType, _ := a.inferExpression(expr.Left)
	if leftType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if expr.Operator == "in" || expr.Operator == "not in" {
		return a.inferMembershipExpression(expr, leftType)
	}
	if isLogicalOperator(expr.Operator) {
		return a.inferLogicalExpression(expr, leftType)
	}

	rightType, _ := a.inferExpression(expr.Right)
	if rightType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if expr.Operator == "x" {
		return a.inferMatrixMultiplyExpression(expr, leftType, rightType)
	}

	if isComparisonOperator(expr.Operator) {
		a.recordComparisonConstantOperand(expr.Left)
		a.recordComparisonConstantOperand(expr.Right)
		// A character literal compared with a char operand is shaped to char.
		if typ, shaped := a.shapeCharacterLiteral(expr.Right, leftType); shaped {
			rightType = typ
		}
		if typ, shaped := a.shapeCharacterLiteral(expr.Left, rightType); shaped {
			leftType = typ
		}
		var valid bool
		rightType, valid = a.contextualNumericLiteralType(expr.Right, rightType, leftType)
		if !valid {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		leftType, valid = a.contextualNumericLiteralType(expr.Left, leftType, rightType)
		if !valid {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if hasUnitSemantics(leftType) || hasUnitSemantics(rightType) {
			if !a.validateUnitComparison(expr, leftType, rightType) {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
		}
	}

	if isEqualityOperator(expr.Operator) {
		if !canCompareEquality(leftType, rightType) {
			help := "Compare values of the same equality-comparable type, or use an explicit content or identity operation for views and resources."
			if intent, ok := booleanLiteralComparisonIntent(expr, leftType, rightType); ok && intent.numericLiteral {
				help = fmt.Sprintf(
					"A bool cannot be compared with an integer, but this operand represents %t. Did you mean `%s`?",
					intent.literalValue,
					intent.replacement,
				)
			}
			before := len(a.errors)
			a.addErrorAtTokenWithMetadata(
				expr.Token,
				diagnostics.OperatorNonComparable,
				help,
				"cannot compare %s and %s",
				typeDisplayName(leftType),
				typeDisplayName(rightType),
			)
			a.recordPitfallDiagnosticOwner(expr, "equality-type-compatibility", before)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	}

	if isOrderedComparisonOperator(expr.Operator) {
		if !canCompareOrdered(leftType, rightType) {
			a.addErrorAtTokenWithMetadata(
				expr.Token,
				diagnostics.OperatorNonOrderable,
				"Use compatible numeric operands, or compare two char, rune, or string values.",
				"operator %s cannot order %s and %s",
				expr.Operator,
				typeDisplayName(leftType),
				typeDisplayName(rightType),
			)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	}

	if isBitwiseOperator(expr.Operator) {
		if !isIntegerType(leftType) || !isIntegerType(rightType) {
			a.addErrorAtToken(expr.Token, "operator %s requires integer operands", expr.Operator)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if expr.Operator == "<<" || expr.Operator == ">>" {
			if !a.validateShiftExpression(expr, leftType) {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			return leftType, expressionValue{Display: expr.String()}
		}
		var valid bool
		rightType, valid = a.contextualNumericLiteralType(expr.Right, rightType, leftType)
		if !valid {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		leftType, valid = a.contextualNumericLiteralType(expr.Left, leftType, rightType)
		if !valid {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if !sameConcreteType(leftType, rightType) {
			a.addErrorAtToken(expr.Token, "cannot apply operator %s to %s and %s", expr.Operator, typeDisplayName(leftType), typeDisplayName(rightType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return leftType, expressionValue{Display: expr.String()}
	}

	if expr.Operator == "+" && (isConcatTextual(leftType) || isConcatTextual(rightType)) {
		return a.inferPlainArithmeticExpression(expr, leftType, rightType)
	}

	if isNumericType(leftType) && isNumericType(rightType) && (hasUnitSemantics(leftType) || hasUnitSemantics(rightType)) {
		return a.inferNumericUnitInfixExpression(expr, leftType, rightType)
	}

	if leftType.Kind == DecimalType || rightType.Kind == DecimalType {
		return a.inferDecimalInfixExpression(expr, leftType, rightType)
	}

	if isNumericType(leftType) && isNumericType(rightType) && (!leftType.Dimension.IsZero() || !rightType.Dimension.IsZero()) {
		return a.inferNumericUnitInfixExpression(expr, leftType, rightType)
	}

	if isArithmeticOperator(expr.Operator) {
		return a.inferPlainArithmeticExpression(expr, leftType, rightType)
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferPlainArithmeticExpression(expr *ast.InfixExpression, leftType Type, rightType Type) (Type, expressionValue) {
	if expr.Operator == "+" && (isConcatTextual(leftType) || isConcatTextual(rightType)) {
		if !isConcatOperand(leftType) || !isConcatOperand(rightType) {
			a.addInvalidConcatOperandError(expr.Token, leftType, rightType)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		a.recordStringConcatPlan(expr, expr.Left, expr.Right)
		return Type{Name: "string", Kind: StringType}, expressionValue{Display: expr.String()}
	}

	if !isNumericType(leftType) || !isNumericType(rightType) {
		if help := numericCarrierOperandHelp(leftType, rightType, expr.Left, expr.Right); help != "" {
			a.addErrorAtTokenWithMetadata(expr.Token, "", help, "operator %s requires numeric operands", expr.Operator)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		a.addErrorAtToken(expr.Token, "operator %s requires numeric operands", expr.Operator)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	var valid bool
	rightType, valid = a.contextualNumericLiteralType(expr.Right, rightType, leftType)
	if !valid {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	leftType, valid = a.contextualNumericLiteralType(expr.Left, leftType, rightType)
	if !valid {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if sameConcreteType(leftType, rightType) {
		if !a.validateCompileTimeIntegerArithmetic(expr, leftType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return leftType, expressionValue{Display: expr.String()}
	}

	if compatiblePlainNumericAlias(leftType, rightType) {
		if leftType.Named {
			if !a.validateCompileTimeIntegerArithmetic(expr, rightType) {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			return rightType, expressionValue{Display: expr.String()}
		}
		if !a.validateCompileTimeIntegerArithmetic(expr, leftType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return leftType, expressionValue{Display: expr.String()}
	}

	if isNumericLiteral(expr.Right) && a.canInitialize(leftType, rightType, expr.Right) {
		if !a.validateCompileTimeIntegerArithmetic(expr, leftType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return leftType, expressionValue{Display: expr.String()}
	}

	if isNumericLiteral(expr.Left) && a.canInitialize(rightType, leftType, expr.Left) {
		if !a.validateCompileTimeIntegerArithmetic(expr, rightType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return rightType, expressionValue{Display: expr.String()}
	}

	a.addErrorAtToken(expr.Token, "cannot apply operator %s to %s and %s", expr.Operator, typeDisplayName(leftType), typeDisplayName(rightType))
	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferCompoundAssignmentType(operator string, target Type, value Type, expr ast.Expression) (Type, bool) {
	if operator == "+=" && (isTextConcatKind(target) || isTextConcatKind(value)) {
		if target.Kind == StringType && !target.Named && isDirectTextConcatOperand(value) {
			return Type{Name: "string", Kind: StringType}, true
		}
		// The failure policy of compound `string +=` is undecided (MD-039),
		// so a fallible text operand cannot join it; the operand's own try
		// handles its StringError.
		if target.Kind == StringType && !target.Named && isToStringResultType(value) {
			a.addErrorAtTokenWithMetadata(expressionToken(expr), diagnostics.OperatorInvalidConcatOperand,
				"Handle the operand's StringError first: write `target += try value.ToString()`.",
				"string += cannot append Result[string, StringError] directly; only + concatenation joins a fallible text operand")
			return Type{Kind: InvalidType}, false
		}
		a.addInvalidConcatOperandError(expressionToken(expr), target, value)
		return Type{Kind: InvalidType}, false
	}

	if !a.canInitialize(target, value, expr) {
		a.addErrorAtToken(
			expressionToken(expr),
			"cannot %s %s to %s",
			assignmentVerb(operator),
			typeDisplayName(value),
			typeDisplayName(target),
		)
		return Type{Kind: InvalidType}, false
	}
	return target, true
}

func (a *Analyzer) inferMatrixMultiplyExpression(expr *ast.InfixExpression, leftType Type, rightType Type) (Type, expressionValue) {
	invalid := func(format string, args ...any) (Type, expressionValue) {
		a.addErrorAtToken(expr.Token, format, args...)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if leftType.Name != "matrix" || len(leftType.TypeArgs) != 1 || len(leftType.ConstArgs) != 2 {
		return invalid("left operand of x must be matrix, got %s", typeDisplayName(leftType))
	}
	if (rightType.Name != "matrix" && rightType.Name != "vector") || len(rightType.TypeArgs) != 1 {
		return invalid("right operand of x must be matrix or vector, got %s", typeDisplayName(rightType))
	}
	// rules/collections/shaped-types.md; correction27.md: x derives its
	// element and accumulator types from ordinary scalar multiplication and
	// addition. Equal operand element types are not a language requirement.
	scalarProduct := *expr
	scalarProduct.Operator = "*"
	productType, _ := a.inferScalarOperatorType(&scalarProduct, leftType.TypeArgs[0], rightType.TypeArgs[0])
	if productType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	scalarSum := *expr
	scalarSum.Operator = "+"
	accumulatorType, _ := a.inferScalarOperatorType(&scalarSum, productType, productType)
	if accumulatorType.Kind == InvalidType || !sameConcreteType(productType, accumulatorType) {
		return invalid("matrix multiplication product type %s is not stable under accumulation", typeDisplayName(productType))
	}

	inner := leftType.ConstArgs[1]
	switch rightType.Name {
	case "matrix":
		if len(rightType.ConstArgs) != 2 {
			return invalid("right operand of x must have two matrix dimensions, got %s", typeDisplayName(rightType))
		}
		if inner != rightType.ConstArgs[0] {
			return invalid("matrix multiplication inner dimensions differ: %d and %d", inner, rightType.ConstArgs[0])
		}
		result := leftType
		result.TypeArgs = []Type{productType}
		result.ConstArgs = []int64{leftType.ConstArgs[0], rightType.ConstArgs[1]}
		return result, expressionValue{Display: expr.String()}
	case "vector":
		if len(rightType.ConstArgs) != 1 {
			return invalid("right operand of x must have one vector dimension, got %s", typeDisplayName(rightType))
		}
		if inner != rightType.ConstArgs[0] {
			return invalid("matrix-vector multiplication inner dimensions differ: %d and %d", inner, rightType.ConstArgs[0])
		}
		result := rightType
		result.TypeArgs = []Type{productType}
		result.ConstArgs = []int64{leftType.ConstArgs[0]}
		return result, expressionValue{Display: expr.String()}
	default:
		return invalid("right operand of x must be matrix or vector, got %s", typeDisplayName(rightType))
	}
}

func (a *Analyzer) inferScalarOperatorType(expr *ast.InfixExpression, left, right Type) (Type, expressionValue) {
	if isNumericType(left) && isNumericType(right) && (hasUnitSemantics(left) || hasUnitSemantics(right)) {
		return a.inferNumericUnitInfixExpression(expr, left, right)
	}
	if left.Kind == DecimalType || right.Kind == DecimalType {
		return a.inferDecimalInfixExpression(expr, left, right)
	}
	if isNumericType(left) && isNumericType(right) && (!left.Dimension.IsZero() || !right.Dimension.IsZero()) {
		return a.inferNumericUnitInfixExpression(expr, left, right)
	}
	return a.inferPlainArithmeticExpression(expr, left, right)
}

func (a *Analyzer) inferNumericUnitInfixExpression(expr *ast.InfixExpression, leftType Type, rightType Type) (Type, expressionValue) {
	left := effectiveUnitSemantics(leftType)
	right := effectiveUnitSemantics(rightType)
	invalid := func(format string, args ...any) (Type, expressionValue) {
		a.addErrorAtToken(expr.Token, format, args...)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	carrier, carrierOK := unitArithmeticCarrier(leftType, rightType, expr.Operator)
	if !carrierOK {
		return invalid("cannot apply operator %s to numeric carriers %s and %s", expr.Operator, typeDisplayName(leftType), typeDisplayName(rightType))
	}

	// rules/types/units.md, "Logarithmic transform": logarithmic coordinates
	// never fall through to ordinary linear arithmetic.
	if left.Transform == LogarithmicUnitTransform || right.Transform == LogarithmicUnitTransform {
		return invalid("operator %s is not defined for logarithmic unit quantities", expr.Operator)
	}
	switch expr.Operator {
	case "+", "-", "%":
		if !leftType.Dimension.Equal(rightType.Dimension) || !unitKindCompatible(left, right) {
			return invalid("cannot %s %s to %s: incompatible unit dimension or Kind", infixVerb(expr.Operator), typeDisplayName(rightType), typeDisplayName(leftType))
		}
		if left.Role == UnitPointRolePoint && right.Role == UnitPointRolePoint && !unitOriginCompatible(left, right) {
			return invalid("cannot %s unit points with different origins", infixVerb(expr.Operator))
		}
		if expr.Operator == "%" && (left.Role == UnitPointRolePoint || right.Role == UnitPointRolePoint) {
			return invalid("remainder is not defined for unit points")
		}
		// rules/types/units.md, "Exact fixed conversions": the right operand
		// converts into the left operand's unit, statically or by its proven
		// value range.
		from, to := right, left
		if (left.Role == UnitPointRolePoint) != (right.Role == UnitPointRolePoint) {
			leftCoordinate, rightCoordinate := left, right
			leftCoordinate.Role, rightCoordinate.Role = UnitVectorRole, UnitVectorRole
			leftCoordinate.Origin, rightCoordinate.Origin = "", ""
			leftCoordinate.Offset, rightCoordinate.Offset = big.NewRat(0, 1), big.NewRat(0, 1)
			from, to = rightCoordinate, leftCoordinate
		}
		if !sameConcreteType(leftType, rightType) && !a.implicitUnitOperandConversion(expr.Right, from, to, carrier) {
			return invalid("cannot implicitly convert %s to %s without loss", typeDisplayName(rightType), typeDisplayName(leftType))
		}

		// rules/types/units.md, "Point algebra".
		if left.Role == UnitPointRolePoint || right.Role == UnitPointRolePoint {
			if left.Role == UnitPointRolePoint && right.Role == UnitPointRolePoint {
				if expr.Operator != "-" {
					return invalid("operator %s is not defined for two unit points", expr.Operator)
				}
				result := left
				result.Identity, result.Named, result.Source = StructuralUnitIdentity, "", "difference("+left.Source+")"
				result.Role, result.Transform, result.Offset = UnitDifferenceRole, LinearUnitTransform, big.NewRat(0, 1)
				return unitResultType(carrier, result, leftType.Dimension), expressionValue{Display: expr.String()}
			}
			if left.Role == UnitPointRolePoint {
				return leftType, expressionValue{Display: expr.String()}
			}
			if expr.Operator == "+" && right.Role == UnitPointRolePoint {
				return rightType, expressionValue{Display: expr.String()}
			}
			return invalid("cannot subtract a unit point from a difference")
		}
		return leftType, expressionValue{Display: expr.String()}
	case "*", "/":
		if left.Role == UnitPointRolePoint || right.Role == UnitPointRolePoint {
			return invalid("operator %s is not defined for unit points", expr.Operator)
		}
		result := combineQuantitySemantics(left, right, expr.Operator)
		dimension := leftType.Dimension.Mul(rightType.Dimension)
		if expr.Operator == "/" {
			dimension = leftType.Dimension.Div(rightType.Dimension)
		}
		if result.Categories[CurrencyUnit] {
			a.addWarningAtToken(expr.Token, "anonymous derived unit %s contains currency", result.Source)
		}
		// Scaling by a plain scalar preserves the named operand. Named ratios are
		// not plain scalars and therefore retain structural provenance.
		if expr.Operator == "*" && !hasUnitSemantics(leftType) {
			return rightType, expressionValue{Display: expr.String()}
		}
		if (expr.Operator == "*" || expr.Operator == "/") && !hasUnitSemantics(rightType) {
			return leftType, expressionValue{Display: expr.String()}
		}
		return unitResultType(carrier, result, dimension), expressionValue{Display: expr.String()}
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

// inferMembershipExpression checks the supported collection and element equality
// without converting named types implicitly or consuming the searched array.
// `not in` has the identical operand contract and produces the complement of
// the membership result without changing evaluation or ownership behavior.
//
// Rules: rules/foundations/operators.md — "Membership expression",
// "Fixed-array membership", "Dynamic-array membership", "Slice membership".
func (a *Analyzer) inferMembershipExpression(expr *ast.InfixExpression, leftType Type) (Type, expressionValue) {
	rangeExpr, ok := expr.Right.(*ast.RangeExpression)
	if ok {
		return a.inferRangeMembershipExpression(expr, rangeExpr, leftType)
	}

	rightType, _ := a.inferExpression(expr.Right)
	if rightType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	elementType, ok := membershipElementType(rightType)
	if !ok {
		a.addErrorAtTokenWithMetadata(
			expr.Token,
			diagnostics.OperatorInvalidMembership,
			"Use a contextual range, a fixed array, a dynamic array, or a slice. For other containers, call an explicit membership API such as Contains.",
			"operator %s supports ranges, fixed arrays, dynamic arrays, and slices in Sec 0.1; got %s",
			expr.Operator,
			typeDisplayName(rightType),
		)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if !EqualityComparable(elementType) {
		a.addErrorAtTokenWithMetadata(
			expr.Token,
			diagnostics.OperatorInvalidMembership,
			"Use a collection whose element type supports equality, or perform an explicit content or identity search.",
			"cannot test membership in %s because element type %s is not equality-comparable",
			typeDisplayName(rightType),
			typeDisplayName(elementType),
		)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	leftType = a.contextualMembershipValueType(expr.Left, leftType, elementType)
	if !canCompareEquality(leftType, elementType) {
		a.addErrorAtTokenWithMetadata(
			expr.Token,
			diagnostics.OperatorInvalidMembership,
			fmt.Sprintf("Search with a value compatible with the collection element type %s.", typeDisplayName(elementType)),
			"cannot test %s for membership in %s with element type %s",
			typeDisplayName(leftType),
			typeDisplayName(rightType),
			typeDisplayName(elementType),
		)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferRangeMembershipExpression(expr *ast.InfixExpression, rangeExpr *ast.RangeExpression, leftType Type) (Type, expressionValue) {
	if rangeExpr.Start != nil {
		startType, _ := a.inferExpression(rangeExpr.Start)
		if typ, shaped := a.shapeCharacterLiteral(rangeExpr.Start, leftType); shaped {
			startType = typ
		}
		if startType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if !canRangeBoundType(leftType, startType, rangeExpr.Start) {
			a.addErrorAtToken(expressionToken(rangeExpr.Start), "cannot test %s in range of %s", typeDisplayName(leftType), typeDisplayName(startType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		a.contextualRangeBoundType(rangeExpr.Start, startType, leftType)
	}

	if rangeExpr.End != nil {
		endType, _ := a.inferExpression(rangeExpr.End)
		if typ, shaped := a.shapeCharacterLiteral(rangeExpr.End, leftType); shaped {
			endType = typ
		}
		if endType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if !canRangeBoundType(leftType, endType, rangeExpr.End) {
			a.addErrorAtToken(expressionToken(rangeExpr.End), "cannot test %s in range of %s", typeDisplayName(leftType), typeDisplayName(endType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		a.contextualRangeBoundType(rangeExpr.End, endType, leftType)
	}

	if leftType.Kind != InvalidType {
		a.resolvedRangeMemberships[expr] = ResolvedRangeMembership{
			Negated:   expr.Operator == "not in",
			ValueType: leftType,
			HasStart:  rangeExpr.Start != nil,
			HasEnd:    rangeExpr.End != nil,
			Exclusive: rangeExpr.Exclusive,
		}
	}
	return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferDecimalInfixExpression(expr *ast.InfixExpression, leftType Type, rightType Type) (Type, expressionValue) {
	if isComparisonOperator(expr.Operator) {
		if isEqualityOperator(expr.Operator) && !canCompareEquality(leftType, rightType) {
			a.addErrorAtToken(expr.Token, "cannot compare %s and %s", typeDisplayName(leftType), typeDisplayName(rightType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	}

	if leftType.Kind == DecimalType && (rightType.Kind == IntType || rightType.Kind == UintType) {
		switch expr.Operator {
		case "*", "/":
			if !rightType.Dimension.IsZero() {
				if expr.Operator == "*" {
					return a.typeForDimension(DecimalType, leftType.Dimension.Mul(rightType.Dimension)), expressionValue{Display: expr.String()}
				}
				return a.typeForDimension(DecimalType, leftType.Dimension.Div(rightType.Dimension)), expressionValue{Display: expr.String()}
			}
			return leftType, expressionValue{Display: expr.String()}
		}
	}

	if (leftType.Kind == IntType || leftType.Kind == UintType) && rightType.Kind == DecimalType {
		switch expr.Operator {
		case "*":
			if !leftType.Dimension.IsZero() {
				return a.typeForDimension(DecimalType, leftType.Dimension.Mul(rightType.Dimension)), expressionValue{Display: expr.String()}
			}
			return rightType, expressionValue{Display: expr.String()}
		case "/":
			if !leftType.Dimension.IsZero() {
				return a.typeForDimension(DecimalType, leftType.Dimension.Div(rightType.Dimension)), expressionValue{Display: expr.String()}
			}
			if !rightType.Dimension.IsZero() {
				return a.typeForDimension(DecimalType, Dimension{}.Div(rightType.Dimension)), expressionValue{Display: expr.String()}
			}
		}
	}

	if leftType.Kind != DecimalType || rightType.Kind != DecimalType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	switch expr.Operator {
	case "==", "!=", "<", "<=", ">", ">=":
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	case "+", "-":
		if sameConcreteType(leftType, rightType) {
			return leftType, expressionValue{Display: expr.String()}
		}
		if leftType.Dimension.Equal(rightType.Dimension) {
			if isNominal(leftType) || isNominal(rightType) {
				a.addErrorAtToken(
					expr.Token,
					"cannot %s %s to %s",
					infixVerb(expr.Operator),
					typeDisplayName(rightType),
					typeDisplayName(leftType),
				)
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			return a.typeForDimension(DecimalType, leftType.Dimension), expressionValue{Display: expr.String()}
		}
		a.addErrorAtToken(
			expr.Token,
			"cannot %s %s to %s",
			infixVerb(expr.Operator),
			typeDisplayName(rightType),
			typeDisplayName(leftType),
		)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case "*":
		if leftType.Dimension.IsZero() && !rightType.Dimension.IsZero() {
			return rightType, expressionValue{Display: expr.String()}
		}
		if rightType.Dimension.IsZero() && !leftType.Dimension.IsZero() {
			return leftType, expressionValue{Display: expr.String()}
		}
		if leftType.Dimension.IsZero() && rightType.Dimension.IsZero() {
			return leftType, expressionValue{Display: expr.String()}
		}
		if leftType.Dimension.Equal(rightType.Dimension) && leftType.Dimension.HasCurrencyBase() {
			a.addErrorAtToken(
				expr.Token,
				"cannot multiply %s by %s",
				typeDisplayName(leftType),
				typeDisplayName(rightType),
			)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return a.typeForDimension(DecimalType, leftType.Dimension.Mul(rightType.Dimension)), expressionValue{Display: expr.String()}
	case "/":
		if leftType.Dimension.IsZero() && rightType.Dimension.IsZero() {
			return leftType, expressionValue{Display: expr.String()}
		}
		return a.typeForDimension(DecimalType, leftType.Dimension.Div(rightType.Dimension)), expressionValue{Display: expr.String()}
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferPrefixExpression(expr *ast.PrefixExpression) (Type, expressionValue) {
	a.adviseNegatedStateTest(expr)
	if expr.Operator == "<-" {
		if a.terminalReturnDepth == 0 && a.constructionOwnershipDepth == 0 {
			a.addErrorAtToken(expr.Token, "explicit <- move requires a return, consuming argument, aggregate field, or union payload context")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		rightType, rightValue := a.inferExpression(expr.Right)
		if rightType.Kind == InvalidType || !a.validateNamedOwnershipSource(ast.OwnershipMove, expr.Right, expr.Token, false, false) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return rightType, rightValue
	}
	rightType, rightValue := a.inferExpression(expr.Right)
	if rightType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	switch expr.Operator {
	case "+":
		if isNumericType(rightType) {
			return rightType, expressionValue{Display: rightValue.Display}
		}
	case "-":
		if rightType.Kind == IntType || rightType.Kind == FloatType || rightType.Kind == DecimalType {
			if isBuiltinIntegerOperatorType(rightType) {
				if value, known := a.integerConstantValue(expr.Right); known {
					representation, _, ok := a.integerRepresentation(rightType)
					if ok && representation.MinInteger != nil && value.Cmp(representation.MinInteger) == 0 {
						a.addWarningAtTokenWithMetadata(
							expr.Token,
							diagnostics.OperatorIntegerOverflow,
							"Ensure a dominating condition excludes the minimum value, or use a wider signed integer type. The operation retains checked runtime semantics.",
							"integer negation %s may overflow %s",
							expr.String(),
							typeDisplayName(rightType),
						)
					}
				}
			}
			return rightType, expressionValue{
				Display:  "-" + rightValue.Display,
				Negative: true,
			}
		}
	case "!":
		if rightType.Kind != BoolType {
			a.addErrorAtToken(expr.Token, "operator ! requires bool operand")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	case "~":
		if !isIntegerType(rightType) {
			a.addErrorAtToken(expr.Token, "operator ~ requires integer operand")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return rightType, expressionValue{Display: expr.String()}
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

// inferRuneArrayToStringCall recognizes the allocation-backed text
// materialization available on rune arrays and rune slice views.
func (a *Analyzer) inferRuneArrayToStringCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member == nil || member.Property == nil || member.Property.Value != "ToString" || a.expressionNamesType(member.Object) {
		return Type{}, expressionValue{}, false
	}

	receiverType, _ := a.inferExpression(member.Object)
	if receiverType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	receiverType = dereferenceType(receiverType)
	if (receiverType.Kind != ArrayType && receiverType.Kind != SliceType) || receiverType.Element == nil || receiverType.Element.Kind != RuneType {
		return Type{}, expressionValue{}, false
	}
	if len(expr.Arguments) != 0 {
		a.addErrorAtToken(expr.Token, "rune array ToString expects 0 arguments, got %d", len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	return toStringResultType(), expressionValue{Display: expr.String()}, true
}
