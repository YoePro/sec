// Escape diagnostics preserve the existing lifetime decisions while assigning
// stable identities, origin locations and remedies at their actual escape sinks.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic architecture",
// "Diagnostic categories", "Diagnostic quality"; rules/tooling/diagnostics.md.
package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// reportEscapeDiagnostic publishes a registered lifetime escape failure with
// category-specific help; unknown provenance explains a missing proof rather
// than asserting a concrete dangling reference.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic quality",
// "Precision exhaustion"; rules/declarations/functions.md — §32.
func (a *Analyzer) reportEscapeDiagnostic(token, origin lexer.Token, id, format string, args ...any) {
	help := ""
	switch id {
	case diagnostics.EscapeLocalStorage:
		help = "The returned reference or carrier outlives this invocation's local storage. Return an owned value or a reference into storage owned by the caller."
	case diagnostics.EscapeOuterPlace:
		help = "The destination can outlive the local referent. Store an owned value or a reference whose storage remains valid for the destination's complete use."
	case diagnostics.EscapeMatchPayload:
		help = "This payload reference is valid only in the active match arm. Use it within that arm or transfer an owned payload instead."
	case diagnostics.EscapeClosureCapture:
		help = "The returned closure can outlive the captured local referent. Capture an owned value or a reference into caller-owned storage with a sufficient lifetime."
	case diagnostics.EscapeProvenanceUnknown:
		help = "Control-flow provenance is unknown, so the compiler cannot prove that the returned reference remains valid. Return a reference with a known caller-owned origin or return an owned value."
	case diagnostics.EscapeVariadicPack:
		help = "The variadic pack is valid only during this invocation. Construct and return an independently owned result instead of returning the pack."
	}
	before := len(a.errors)
	a.addErrorAtTokenWithPreviousMetadata(token, origin, id, help, format, args...)
	if len(a.errors) > before {
		a.errors[len(a.errors)-1].EscapeCauses = a.escapeDiagnosticCause(token, origin, id, nil)
	}
}

// checkReturningReferenceToLocal rejects returned direct and projected references into local or match-arm storage.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic categories", "Diagnostic quality";
// rules/memory/lifetime_analysis.md — §§1–4; rules/declarations/lambda-functions.md — §24.
func (a *Analyzer) checkReturningReferenceToLocal(functionName string, returnType Type, valueType Type, expr ast.Expression) bool {
	if !typeCarriesReferenceOrigin(returnType) || !typeCarriesReferenceOrigin(valueType) {
		return false
	}
	if call, ok := expr.(*ast.CallExpression); ok {
		if origin, tracked := a.expressionReferenceOrigins[call]; tracked {
			return a.checkTrackedReturnedReferenceOrigin(functionName, expr, origin)
		}
	}
	if origin, ok := a.containedOriginForAccess(expr); ok {
		return a.checkTrackedReturnedReferenceOrigin(functionName, expr, origin)
	}
	if valueType.ReferenceOriginMatchScoped {
		a.reportEscapeExpressionDiagnostic(expr, valueType.ReferenceOriginToken, diagnostics.EscapeMatchPayload, "function %s cannot return a branch-scoped union payload reference", functionName)
		return true
	}
	if !valueType.ReferenceOriginLocal {
		return false
	}
	originName := valueType.ReferenceOriginName
	if originName == "" {
		originName = "local value"
	}
	if functionName == "lambda" {
		a.reportEscapeExpressionDiagnostic(expr, valueType.ReferenceOriginToken, diagnostics.EscapeLocalStorage, "lambda cannot return reference to local variable %s", originName)
		return true
	}
	a.reportEscapeExpressionDiagnostic(expr, valueType.ReferenceOriginToken, diagnostics.EscapeLocalStorage, "function %s cannot return reference to local variable %s", functionName, originName)
	return true
}

// checkTrackedReturnedReferenceOrigin checks tracked return provenance without treating unknown origins as a proven dangling reference.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic categories", "Diagnostic quality";
// rules/memory/lifetime_analysis.md — §§1–4; rules/declarations/lambda-functions.md — §24.
func (a *Analyzer) checkTrackedReturnedReferenceOrigin(functionName string, expr ast.Expression, origin localReferenceOrigin) bool {
	if origin.Unknown {
		a.reportEscapeExpressionDiagnostic(expr, lexer.Token{}, diagnostics.EscapeProvenanceUnknown, "function %s cannot return reference with unknown control-flow provenance", functionName)
		return true
	}
	if origin.MatchScoped {
		a.reportEscapeExpressionDiagnostic(expr, origin.Token, diagnostics.EscapeMatchPayload, "function %s cannot return a branch-scoped union payload reference", functionName)
		return true
	}
	if !origin.Local {
		// The aggregate/call result can be local while the referenced storage
		// is exclusively caller-owned.
		return false
	}
	originName := origin.Name
	if originName == "" {
		originName = "local value"
	}
	if functionName == "lambda" {
		a.reportEscapeExpressionDiagnostic(expr, origin.Token, diagnostics.EscapeLocalStorage, "lambda cannot return reference to local variable %s", originName)
		return true
	}
	a.reportEscapeExpressionDiagnostic(expr, origin.Token, diagnostics.EscapeLocalStorage, "function %s cannot return reference to local variable %s", functionName, originName)
	return true
}

// checkExpressionEscapesLocalReference rejects returned carriers that retain a reference into local storage.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic categories", "Diagnostic quality";
// rules/memory/lifetime_analysis.md — §§1–4; rules/declarations/lambda-functions.md — §24.
func (a *Analyzer) checkExpressionEscapesLocalReference(functionName string, expr ast.Expression) bool {
	originName, originToken, ok := a.localReferenceOriginInExpression(expr)
	if !ok {
		return false
	}
	if originName == "" {
		originName = "local value"
	}
	if functionName == "lambda" {
		a.reportEscapeExpressionDiagnostic(expr, originToken, diagnostics.EscapeLocalStorage, "lambda cannot return value containing reference to local variable %s", originName)
		return true
	}
	a.reportEscapeExpressionDiagnostic(expr, originToken, diagnostics.EscapeLocalStorage, "function %s cannot return value containing reference to local variable %s", functionName, originName)
	return true
}

// checkExpressionEscapesMatchPayload rejects returned carriers that retain a match-arm payload reference.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic categories", "Diagnostic quality";
// rules/memory/lifetime_analysis.md — §§1–4; rules/declarations/lambda-functions.md — §24.
func (a *Analyzer) checkExpressionEscapesMatchPayload(functionName string, expr ast.Expression) bool {
	_, originToken, ok := a.matchScopedReferenceOriginInExpression(expr)
	if !ok {
		return false
	}
	a.reportEscapeExpressionDiagnostic(expr, originToken, diagnostics.EscapeMatchPayload, "function %s cannot return a value containing a branch-scoped union payload reference", functionName)
	return true
}

// checkAssignmentEscapesLocalReference rejects local reference writes into storage beyond the current invocation.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic categories", "Diagnostic quality";
// rules/memory/lifetime_analysis.md — §§1–4; rules/declarations/lambda-functions.md — §24.
func (a *Analyzer) checkAssignmentEscapesLocalReference(target ast.Expression, value ast.Expression) bool {
	if a.checkAssignmentEscapesMatchPayload(target, value) {
		return true
	}
	originName, originToken, ok := a.localReferenceOriginInExpression(value)
	if !ok {
		return false
	}
	targetRoot, ok := borrowRootName(target)
	if !ok {
		return false
	}
	targetSymbol, ok := a.symbols[targetRoot]
	if !ok {
		return false
	}
	// A reference parameter is a local binding over caller-owned storage. Writes
	// through its projected Place therefore cross the function boundary even
	// though the parameter symbol itself is local.
	if targetSymbol.Local && targetSymbol.Type.Kind != ReferenceType {
		return false
	}
	if originName == "" {
		originName = "local value"
	}
	a.recordOuterPlaceEscapeFact(value, originName, originToken)
	a.reportEscapeAssignmentDiagnostic(target, value, originToken, diagnostics.EscapeOuterPlace, "cannot store reference to local variable %s into %s", originName, targetRoot)
	return true
}

// checkAssignmentEscapesMatchPayload rejects payload reference writes beyond the active match arm.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic categories", "Diagnostic quality";
// rules/memory/lifetime_analysis.md — §§1–4; rules/declarations/lambda-functions.md — §24.
func (a *Analyzer) checkAssignmentEscapesMatchPayload(target ast.Expression, value ast.Expression) bool {
	_, originToken, ok := a.matchScopedReferenceOriginInExpression(value)
	if !ok {
		return false
	}
	targetRoot, rootOK := borrowRootName(target)
	if rootOK {
		if symbol, symbolOK := a.symbols[targetRoot]; symbolOK && symbol.Local && symbol.ScopeDepth >= a.scopeDepth {
			return false
		}
	}
	a.reportEscapeAssignmentDiagnostic(target, value, originToken, diagnostics.EscapeMatchPayload, "cannot store branch-scoped union payload reference outside its match arm")
	return true
}

// checkReturningLambdaCapturingLocalReference rejects returned closures capturing references into local storage.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic categories", "Diagnostic quality";
// rules/memory/lifetime_analysis.md — §§1–4; rules/declarations/lambda-functions.md — §24.
func (a *Analyzer) checkReturningLambdaCapturingLocalReference(functionName string, expr ast.Expression) bool {
	lambda, ok := expr.(*ast.LambdaExpression)
	if !ok {
		return false
	}
	for _, capture := range lambda.Captures {
		if capture.Name == nil {
			continue
		}
		symbol, ok := a.symbols[capture.Name.Value]
		if !ok || symbol.Type.Kind != ReferenceType || !symbol.Type.ReferenceOriginLocal {
			continue
		}
		originName := symbol.Type.ReferenceOriginName
		if originName == "" {
			originName = "local value"
		}
		if functionName == "lambda" {
			a.reportEscapeExpressionDiagnostic(capture.Name, symbol.Type.ReferenceOriginToken, diagnostics.EscapeClosureCapture, "lambda cannot return lambda capturing reference to local variable %s", originName)
			return true
		}
		a.reportEscapeExpressionDiagnostic(capture.Name, symbol.Type.ReferenceOriginToken, diagnostics.EscapeClosureCapture, "function %s cannot return lambda capturing reference to local variable %s", functionName, originName)
		return true
	}
	return false
}
