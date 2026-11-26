package main

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/sema"
)

// genericInstantiationHint shows the inferred type arguments of a call to a
// generic function that does not spell them, as `[T1, T2]` after the callee
// name, from Sema's resolved substitution in declaration order. A call with
// explicit type arguments, a partial or symbolic substitution, or an
// unresolved callee gets no guessed hint.
// Rules: rules/tooling/lsp.md — § 17(7a); rules/declarations/generics.md — inference.
func genericInstantiationHint(analyzer *sema.Analyzer, text string, call *ast.CallExpression, function sema.Function) (inlayHint, bool) {
	if len(call.GenericArguments) != 0 || len(function.GenericSubstitution) == 0 || function.GenericTypePack != "" {
		return inlayHint{}, false
	}
	// The instantiation keeps its substitution; the declared parameter order
	// comes from the template with the same declaration token.
	parameters := function.GenericParameters
	if len(parameters) == 0 {
		for _, candidate := range analyzer.Functions()[function.Name] {
			if candidate.Token == function.Token && len(candidate.GenericParameters) > 0 {
				parameters = candidate.GenericParameters
				break
			}
		}
	}
	if len(parameters) == 0 && len(function.GenericSubstitution) == 1 {
		for name := range function.GenericSubstitution {
			parameters = []string{name}
		}
	}
	if len(parameters) != len(function.GenericSubstitution) {
		return inlayHint{}, false
	}
	names := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		typ, ok := function.GenericSubstitution[parameter]
		if !ok || typ.Kind == sema.InvalidType || typ.Kind == sema.GenericType {
			return inlayHint{}, false
		}
		name := lspTypeName(typ)
		if name == "" {
			return inlayHint{}, false
		}
		names = append(names, name)
	}
	var callee ast.Expression = call.Callee
	token := leftmostExpressionToken(callee)
	if member, ok := callee.(*ast.MemberExpression); ok && member.Property != nil {
		token = member.Property.Token
	}
	if !validDefinitionTokenForHint(token) {
		return inlayHint{}, false
	}
	label := "[" + strings.Join(names, ", ") + "]"
	hint := inlayHint{Position: tokenRange(text, token).End, Label: label, Kind: inlayHintKindType}
	hint.detail = "inferred type arguments " + label + " for " + strings.Join(parameters, ", ")
	return hint, true
}

// tryErrorTypeHint shows, right after a propagating `try`, what leaves the
// enclosing callable on failure: `↑ E` for a Result error (including
// arithmetic, bounds and failure-set propagation), `↑ None` for Option, and
// `↑ test fails: E` at a test boundary. Locally handled try expressions and
// unresolved facts get no hint; the label never changes source.
// Rules: rules/tooling/lsp.md — § 17(7a); rules/errors/errorhandling.md — §§ 8, 13.
func tryErrorTypeHint(analyzer *sema.Analyzer, text string, try *ast.TryExpression) (inlayHint, bool) {
	resolved, ok := analyzer.ResolvedTryOf(try)
	if !ok || !validDefinitionTokenForHint(try.Token) {
		return inlayHint{}, false
	}
	var label, detail string
	switch resolved.Kind {
	case sema.ResolvedTryOptionPropagation:
		label, detail = "↑ None", "propagates None to the enclosing Option"
	case sema.ResolvedTryResultPropagation, sema.ResolvedTryResultReturnForwarding, sema.ResolvedTryArithmeticPropagation,
		sema.ResolvedTryBoundsPropagation, sema.ResolvedTryFailureSetPropagation:
		name := lspTypeName(resolved.ErrorType)
		if name == "" || resolved.ErrorType.Kind == sema.InvalidType {
			return inlayHint{}, false
		}
		label, detail = "↑ "+name, "propagates "+name+" to the enclosing Result"
		if resolved.TestBoundary {
			label, detail = "↑ test fails: "+name, "an unexpected "+name+" fails the current test after cleanup"
		}
	default:
		return inlayHint{}, false
	}
	hint := inlayHint{Position: tokenRange(text, try.Token).End, Label: label, Kind: inlayHintKindType, PaddingLeft: true, PaddingRight: true}
	hint.detail = detail
	hint.declaration, _ = analyzer.ResolvedTypeDeclarationLocation(resolved.ErrorType)
	return hint, true
}
