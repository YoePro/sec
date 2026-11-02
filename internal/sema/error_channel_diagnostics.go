package sema

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// validateResultErrorChannel enforces error eligibility with one shared identity
// for concrete non-error types and generic parameters lacking the requirement.
// It never infers an error constraint from a parameter's name or use in Result.
// Rules: rules/errors/errorhandling.md — §4; rules/declarations/generics.md — §15A;
// MD-120 §§2–4,10(7); rules/tooling/diagnostics.md — §§2(6–10),14–15.
func (a *Analyzer) validateResultErrorChannel(ref *ast.TypeReference, typ Type) bool {
	if typ.Kind != ResultType || len(typ.TypeArgs) != 2 || len(ref.TypeArgs) != 2 {
		return true
	}
	errorType := typ.TypeArgs[1]
	if errorType.Kind == ErrorRootType || errorType.ErrorAssignable {
		return true
	}
	name := typeDisplayName(errorType)
	help := "Use the explicit error root or a declared error type as the second Result argument; an ordinary concrete type is not an error channel."
	if errorType.Kind == GenericType {
		help = fmt.Sprintf("Declare the generic requirement [%s: error] before using %s as a Result error channel. This requirement does not imply concrete variants, copying or discardability.", name, name)
	}
	a.addErrorAtTokenWithMetadata(ref.TypeArgs[1].Token, diagnostics.InvalidResultErrorChannel, help,
		"Result error type %s is not an error type; use error or declare %s with the error marker", name, name)
	return false
}

// addNonConcreteErrorVariantDiagnostic shares a rule identity across match/try
// and contextual/qualified selectors. Repair advice preserves the difference
// between an unknown constrained E and explicit open error-root narrowing.
// Rules: rules/errors/errorhandling.md — §§15.3,27.2b; MD-120 §4(4–7),10(7);
// rules/tooling/diagnostics.md — §§2(6–10),14–15.
func (a *Analyzer) addNonConcreteErrorVariantDiagnostic(token lexer.Token, channel Type, format string, args ...any) {
	help := "Bind the whole error with Err(cause); this channel does not identify one closed concrete error type."
	if channel.Kind == GenericType {
		help = "Bind the whole error with Err(cause). The generic error requirement supplies no concrete variants; Err(_) also requires proven discardability."
	}
	if channel.Kind == ErrorRootType {
		help = "Bind the whole error with Err(cause), or qualify a declared concrete error type when narrowing the open error channel."
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.ErrorVariantRequiresConcreteChannel, help, format, args...)
}

// addGenericTypeArgumentCountError preserves ordinary generic-type behavior
// while identifying incomplete interface specializations as their own rule.
// No default to error, error erasure or generic variance is invented.
// Rules: rules/declarations/interfaces.md — §§7.1,9; MD-120 §6(2–5),10(7);
// rules/tooling/diagnostics.md — §§2(6–10),14–15.
func (a *Analyzer) addGenericTypeArgumentCountError(token lexer.Token, typ Type, sourceName string, count int) {
	if typ.Kind != InterfaceType {
		a.addErrorAtToken(token, "%s requires %d generic arguments, got %d", sourceName, len(typ.GenericParameters), count)
		return
	}
	help := "Supply every declared interface type argument explicitly. For an error parameter, use a declared error type, a constrained generic error parameter, or explicit error; no implicit default or erasure is provided."
	a.addErrorAtTokenWithMetadata(token, diagnostics.GenericInterfaceArgumentCount, help, "%s requires %d generic arguments, got %d", sourceName, len(typ.GenericParameters), count)
}
