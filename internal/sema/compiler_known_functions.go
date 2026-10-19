package sema

import "sec/internal/ast"

// inferCompilerKnownFunction resolves global compiler-known functions and
// records slice length as a temporal observation, without hidden allocation.
// Rules: rules/compiler/compiler_known_members.md — "Global len";
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§47–49,118–119.
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
		if dereferenceType(argumentType).Kind == SliceType {
			a.recordSliceLengthUse(expr, expr.Arguments[0])
		}
		// The registry result: uint, the same count as .Len (MD-074/MD-088).
		return knownFunction.Result, expressionValue{Display: expr.String()}, true
	}

	a.addErrorAtToken(expressionToken(expr.Arguments[0]), "len requires a compiler-known sequence or collection, got %s", typeDisplayName(argumentType))
	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
}

// inferCompilerInternalFunction enforces privileged core ownership of internal calls.
// Rules: rules/library/core-library.md — §§1.1–1.2.
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

	if known.ID == "CKF-STRING-FROM-UTF8" {
		return a.inferStringFromUtf8Intrinsic(expr, known)
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
