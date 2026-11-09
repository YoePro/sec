package sema

import "sec/internal/lexer"

// isThreadObserverType identifies the canonical non-owning observation handle.
// Rules: rules/concurrency/threads.md — §40(1–6).
func isThreadObserverType(typ Type) bool {
	typ = dereferenceType(typ)
	return typ.Intrinsic && typ.Name == "ThreadObserver" && len(typ.TypeArgs) == 1
}

// reportThreadObserverWait explains why an observation handle cannot acquire a
// hidden wait/select capability or duplicate its owner's join responsibility.
// Rules: rules/concurrency/threads.md — §§40(6),43(1–4).
func (a *Analyzer) reportThreadObserverWait(token lexer.Token) {
	a.addErrorAtTokenWithMetadata(token, "",
		"ThreadObserver exposes read-only metadata and has no Wait or selectable completion operation. Keep the owning Thread[T] handle to join its completion.",
		"ThreadObserver has no wait or selectable completion capability")
}
