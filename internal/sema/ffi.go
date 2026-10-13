package sema

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// rejectUnresolvedGenericExtern prevents an extern declaration from entering
// the callable catalog without a concrete ABI signature. A later declaration
// is still registered and analyzed normally after this focused error.
//
// Rules:
//   - rules/compiler/monomorphization.md — §16.1, §16.4–16.5
//   - rules/platform/ffi.md — §48 "Generics"
//   - rules/platform/ffi.md — §52 "Sema requirements"
func (a *Analyzer) rejectUnresolvedGenericExtern(fn *ast.FunctionDeclaration) bool {
	if fn == nil || !fn.Extern || len(fn.GenericParameters) == 0 {
		return false
	}
	token := fn.Name.Token
	parameterName := ""
	var parameterNames []string
	for _, parameter := range fn.GenericParameters {
		if parameter != nil && parameter.Name != nil {
			if len(parameterNames) == 0 {
				token = parameter.Name.Token
				parameterName = parameter.Name.Value
			}
			parameterNames = append(parameterNames, parameter.Name.Value)
		}
	}
	endLine, endColumn := token.EndPosition()
	a.appendError(Error{
		ID:         diagnostics.UnresolvedGenericExtern,
		Severity:   diagnostics.SeverityError,
		ProofState: diagnostics.ProofInvalid,
		Help:       "declare a concrete extern signature; an unresolved generic template has no single foreign ABI representation. Unresolved parameters: " + strings.Join(parameterNames, ", ") + ". A generic Sec wrapper may call a concrete extern declaration",
		Message:    fmt.Sprintf("extern %s function %s cannot declare unresolved generic parameter %s", fn.ABI, fn.Name.Value, parameterName),
		File:       token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn,
		PreviousFile: fn.Name.Token.File, PreviousLine: fn.Name.Token.Line, PreviousColumn: fn.Name.Token.Column,
		RelatedLabel: "generic extern declaration",
	})
	return true
}
