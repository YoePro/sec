package sema

import (
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// rejectPortableThreadTerminationMember diagnoses the removed universal hard
// termination API only on a resolved builtin owner/observer. Explicit methods
// retain ordinary overload and unsafe validation; unrelated receivers and
// target-declared platform types never gain a spelling-based prohibition.
// Rules: rules/concurrency/threads.md — §§26(4–5),40(6),52(7),53(1–4);
// rules/corrections/applied/threads-v2-cross-rulebook-correction-20260924.md — §23.
func (a *Analyzer) rejectPortableThreadTerminationMember(typ Type, name string, token lexer.Token) bool {
	typ = dereferenceType(typ)
	if !typ.Intrinsic || typ.Kind != StructType || (typ.Name != "Thread" && typ.Name != "ThreadObserver") || len(typ.TypeArgs) != 1 {
		return false
	}
	if name != "Terminate" && name != "platform" {
		return false
	}
	if len(a.functions[typ.Name+"."+name]) != 0 {
		return false
	}
	help := "Use RequestCancel() on the owning Thread[T] for cooperative cancellation and join it to resolve lifecycle ownership. Hard termination requires an explicitly declared unsafe target-specific API with defined lifecycle, storage and cleanup effects; platform metadata alone grants no termination capability."
	if name == "platform" {
		a.addErrorAtTokenWithMetadata(token, diagnostics.ThreadTerminationNotPortable, help,
			"legacy member platform on %s does not provide a portable Terminate operation", typeDisplayName(typ))
	} else {
		a.addErrorAtTokenWithMetadata(token, diagnostics.ThreadTerminationNotPortable, help,
			"%s has no portable hard-termination operation Terminate", typeDisplayName(typ))
	}
	return true
}
