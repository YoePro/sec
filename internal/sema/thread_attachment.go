package sema

import "sec/internal/lexer"

// builtinThreadAttachmentType is the opaque move-only ownership token returned
// by attachment, never a ThreadContext value or ownership of the native thread.
// Rules: rules/concurrency/thread_local.md §§30–33.
func builtinThreadAttachmentType() Type {
	return Type{Name: "ThreadAttachment", Module: "core", Kind: StructType, Named: true, Declared: true, Intrinsic: true, ExplicitlyNonCopyable: true, NoCopyPolicyOrigin: "ThreadAttachment"}
}

// isThreadAttachmentType checks canonical nominal identity before exposing any
// compiler-owned member or lifecycle trait.
// Rules: rules/concurrency/thread_local.md §§30–32.
func isThreadAttachmentType(typ Type) bool {
	typ = dereferenceType(typ)
	return typ.Intrinsic && typ.Module == "core" && typ.Kind == StructType && typ.Name == "ThreadAttachment" && len(typ.TypeArgs) == 0
}

// threadAttachmentMembers publishes the exact static acquisition signature and
// the token's read-only non-owning context view. Cleanup is never a callable free.
// Rules: rules/concurrency/thread_local.md §§30(1–5),31(1–5),33.
func threadAttachmentMembers(typ Type, static bool) []CompilerKnownMember {
	var members []CompilerKnownMember
	if static && typ.Kind == StructType && typ.Name == "Thread" && (typ.Module == "" || typ.Module == "core") {
		members = []CompilerKnownMember{{ID: "CKM-THREAD-ATTACHCURRENT", Name: "AttachCurrent", Kind: CompilerKnownAssociatedFunction, Result: compilerKnownResult(builtinThreadAttachmentType(), builtinThreadAttachErrorType()), Signature: "static fn AttachCurrent() Result[ThreadAttachment, ThreadAttachError]", Documentation: "Attaches the calling physical/native thread and returns its unique owned attachment token. Already Sec-created or attached threads return AlreadyAttached. Performs no user ThreadLocal factory initialization. Runtime attachment support is required."}}
	} else if !static && isThreadAttachmentType(typ) {
		members = []CompilerKnownMember{{ID: "CKM-THREADATTACHMENT-CONTEXT", Name: "Context", Kind: CompilerKnownProperty, Result: builtinType("ThreadContext"), Signature: "property Context: ThreadContext", Documentation: "Read-only non-owning view of the attached physical thread. The attachment token owns only the Sec attachment, never the native thread. Confinement, runtime detach and TLS cleanup require their dedicated implementation."}}
	}
	for i := range members {
		members[i].Rule = "rules/concurrency/thread_local.md — §§30–33"
	}
	return members
}

// recordThreadAttachmentEffects preserves unknown target-runtime effects until
// attachment implementation facts exist. The signature alone proves neither
// allocation freedom, panic freedom nor absence of waiting; no TLS factory is run.
// Rules: rules/concurrency/thread_local.md §§31(5),53(4–5),54;
// rules/foundations/attributes.md — @noAlloc, @noPanic, @noBlock verification.
func (a *Analyzer) recordThreadAttachmentEffects(source lexer.Token) {
	if a.summaryPass || !a.callGraphPathReachable {
		return
	}
	a.callGraph.addEffect(a.currentCallable, EffectSite{Kind: EffectMayPanicUnknownCallee, Source: source})
	a.callGraph.addArenaEffect(a.currentCallable, ArenaEffectSite{Kind: ArenaEffectUnknownCallee, Source: source, UnknownAllocation: true})
	a.callGraph.addBlockEffect(a.currentCallable, BlockEffectSite{Kind: BlockEffectUnknownCallee, Source: source})
}
