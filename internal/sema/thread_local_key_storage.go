package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// validateThreadLocalKeyStorage rejects automatic storage for portable TLS keys.
// Module bindings already have static storage; local static declarations retain
// it explicitly. References to keys do not themselves own a key.
// Rules: rules/concurrency/thread_local.md — §20(1–4);
// rules/declarations/static.md — static storage duration.
func (a *Analyzer) validateThreadLocalKeyStorage(stmt *ast.LetStatement, typ Type) bool {
	if !a.inFunctionBody || stmt.Static || !ownsThreadLocalKey(typ, map[*Type]bool{}) {
		return true
	}
	a.addErrorAtTokenWithMetadata(stmt.Name.Token, diagnostics.ThreadLocalAutomaticKey,
		"Declare the ThreadLocal key in static storage with static let, or borrow an existing static key.",
		"local automatic binding %s cannot own a ThreadLocal key", stmt.Name.Value)
	return false
}

// ownsThreadLocalKey follows owned storage, including named representations and
// aggregate fields, without mistaking a reference or callable signature for a key.
// Rules: rules/concurrency/thread_local.md — §§4(1),20(1–3).
func ownsThreadLocalKey(typ Type, seen map[*Type]bool) bool {
	if typ.Kind == ReferenceType || typ.Kind == RawPtrType || typ.Kind == FunctionType {
		return false
	}
	if typ.Kind == StructType && typ.Name == "ThreadLocal" && (typ.Module == "" || typ.Module == "core") {
		return true
	}
	visit := func(child *Type) bool {
		if child == nil || seen[child] {
			return false
		}
		seen[child] = true
		return ownsThreadLocalKey(*child, seen)
	}
	if visit(typ.NamedBase) || visit(typ.Element) {
		return true
	}
	for _, field := range typ.Fields {
		if ownsThreadLocalKey(field.Type, seen) {
			return true
		}
	}
	for _, variant := range typ.UnionVariants {
		if visit(variant.Payload) {
			return true
		}
	}
	return false
}
