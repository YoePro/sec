package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// toStringResultType is the one canonical ToString result. Every ToString
// materializes text and may fail to allocate or, for a format overload, to
// format, so the compiler-known fallback and every declared overload return
// Result[string, StringError].
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "ToString()"
func toStringResultType() Type {
	return compilerKnownResult(builtinType("string"), builtinType("StringError"))
}

// isToStringResultType reports whether typ is exactly
// Result[string, StringError]: built-in string, never a named string type,
// and the compiler-known StringError.
func isToStringResultType(typ Type) bool {
	if typ.Kind != ResultType || len(typ.TypeArgs) != 2 {
		return false
	}
	value, failure := typ.TypeArgs[0], typ.TypeArgs[1]
	return value.Kind == StringType && !value.Named &&
		failure.Kind == UnionType && failure.Name == "StringError" && failure.Intrinsic
}

// checkToStringSignature rejects a declared ToString member, in an impl or an
// interface and for every overload, whose result is not
// Result[string, StringError]. Users may implement their own ToString, so
// the uniform contract is enforced on the declaration rather than assumed.
// Unresolved result types already carry their own diagnostic.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "ToString()", "User-defined ToString()"
func (a *Analyzer) checkToStringSignature(fn *ast.FunctionDeclaration, returnType Type) {
	if fn == nil || fn.Name == nil || fn.Name.Value != "ToString" || returnType.Kind == InvalidType || isToStringResultType(returnType) {
		return
	}
	token := fn.Name.Token
	if fn.ReturnType != nil {
		token = fn.ReturnType.Token
	}
	a.reportToStringSignature(token, returnType)
}

func (a *Analyzer) reportToStringSignature(token lexer.Token, returnType Type) {
	a.addErrorAtTokenWithMetadata(
		token,
		diagnostics.ToStringSignature,
		"Declare the result as Result[string, StringError]; materializing text can fail to allocate or to format.",
		"%s",
		fmt.Sprintf("ToString must return Result[string, StringError], got %s", typeDisplayName(returnType)),
	)
}
