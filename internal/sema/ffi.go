package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// rejectUnresolvedGenericExtern prevents an extern declaration from entering
// the callable catalog without a concrete ABI signature. A later declaration
// is still registered and analyzed normally after this focused error.
//
// Rules:
//   - rules/platform/ffi.md — §48 "Generics"
//   - rules/platform/ffi.md — §52 "Sema requirements"
func (a *Analyzer) rejectUnresolvedGenericExtern(fn *ast.FunctionDeclaration) bool {
	if fn == nil || !fn.Extern || len(fn.GenericParameters) == 0 {
		return false
	}
	token := fn.Name.Token
	parameterName := ""
	for _, parameter := range fn.GenericParameters {
		if parameter != nil && parameter.Name != nil {
			token = parameter.Name.Token
			parameterName = parameter.Name.Value
			break
		}
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.UnresolvedGenericExtern,
		"declare a concrete extern signature; an unresolved generic template has no single foreign ABI representation",
		"extern %s function %s cannot declare unresolved generic parameter %s", fn.ABI, fn.Name.Value, parameterName)
	return true
}
