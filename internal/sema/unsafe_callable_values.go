package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// rejectUnsafeFunctionValue reports a named unsafe function used as a
// first-class function value.
//
// Unsafe status is part of callable compatibility, and an unsafe function must
// not implicitly convert to a safe function value. FunctionType has no unsafe
// form, so every function value is a safe callable: accepting the conversion
// would let a call through the value bypass the explicit unsafe context. The
// rejection holds inside unsafe contexts too, because unsafe accepts caller
// obligations at a call and does not change callable compatibility. A safe
// wrapper that discharges the obligations remains the way to obtain a safe
// callable.
//
// Rules:
//   - rules/memory/unsafe.md — §39(1), §39(2), §39(3), §39(4), §50(2) "unsafe.function-value-mismatch"
//   - rules/corrections/applied/md059-ffi-unsafe-callables-correction-20261016.md — §§ 2, 8 (MD-059: Sec 0.1 has no unsafe function type)
func (a *Analyzer) rejectUnsafeFunctionValue(ident *ast.Identifier, function Function) bool {
	if !function.Unsafe {
		return false
	}
	kind := "function"
	if function.Extern {
		kind = "extern function"
	}
	a.addErrorAtTokenWithMetadata(ident.Token, diagnostics.UnsafeFunctionValueMismatch,
		"An unsafe declaration carries caller obligations that a native fn(...) type cannot express: every Sec 0.1 function type is a safe callable, and entering unsafe does not change that. Call the function directly inside `unsafe { }`, or pass a safe wrapper function that actually establishes its obligations. A foreign C function pointer is the separate type c::fn(...), not a native function value.",
		"unsafe %s %s cannot be used as a function value; function types are safe callables", kind, ident.Value)
	return true
}

// callUnsafeAuthorized reports whether the unsafe obligations of call itself
// are authorized: inside an explicit unsafe block, or by the call's own direct
// `unsafe` prefix. The prefix covers only this call; its receiver, arguments
// and other operands keep the enclosing authorization (a.inUnsafe).
//
// Rules:
//   - rules/memory/unsafe.md — § 4 single-operation syntax (MD-079)
//   - rules/foundations/grammar.md — UnsafeCallExpression
func (a *Analyzer) callUnsafeAuthorized(call *ast.CallExpression) bool {
	return a.inUnsafe || call.HasUnsafePrefix()
}
