package sema

// cancellationContextMembers is the canonical source/tooling inventory. Runtime
// state stays opaque; no observation grants cancellation authority on Context.
// Rules: rules/concurrency/cancellation.md — §§33,35–38,43.
func cancellationContextMembers(typ Type, static bool) []CompilerKnownMember {
	if !isCancellationContextType(typ) {
		return nil
	}
	typ = dereferenceType(typ)
	context, source := builtinContextType(), builtinContextSourceType()
	var members []CompilerKnownMember
	if typ.Name == "Context" {
		if static {
			members = []CompilerKnownMember{{ID: "CKM-CONTEXT-BACKGROUND", Name: "Background", Kind: CompilerKnownAssociatedFunction, Result: context, Signature: "static fn Background() Context", Documentation: "Creates a standalone root Context with no parent cancellation and no effective deadline."}}
		} else {
			members = []CompilerKnownMember{
				{ID: "CKM-CONTEXT-WITHCANCEL", Name: "WithCancel", Kind: CompilerKnownMethod, Result: source, Signature: "fn WithCancel() ContextSource", Documentation: "Creates a child Context owned by the returned ContextSource. The child source cannot outlive its parent Context."},
				{ID: "CKM-CONTEXT-WITHTIMEOUT", Name: "WithTimeout", Kind: CompilerKnownMethod, Result: source, Parameters: []FunctionParameter{{Name: "timeout", Type: builtinType("duration")}}, Signature: "fn WithTimeout(timeout: duration) ContextSource", Documentation: "Creates a child with a monotonic timeout bounded by an earlier parent deadline. The child source cannot outlive its parent Context."},
				{ID: "CKM-CONTEXT-WITHDEADLINE", Name: "WithDeadline", Kind: CompilerKnownMethod, Result: source, Parameters: []FunctionParameter{{Name: "deadline", Type: builtinType("Instant")}}, Signature: "fn WithDeadline(deadline: Instant) ContextSource", Documentation: "Creates a child with an absolute monotonic deadline bounded by an earlier parent deadline. Wall-clock datetime is not an Instant."},
				{ID: "CKM-CONTEXT-ISCANCELLED", Name: "IsCancelled", Kind: CompilerKnownProperty, Result: builtinType("bool"), Signature: "property IsCancelled: bool", Documentation: "True after explicit or inherited cancellation or deadline expiration commits a terminal Context cause; distinct from current-task terminal cancellation."},
				{ID: "CKM-CONTEXT-ERROR", Name: "Error", Kind: CompilerKnownProperty, Result: compilerKnownOption(builtinContextErrorType()), Signature: "property Error: Option[ContextError]", Documentation: "None while active; Some(Cancelled) or Some(DeadlineExceeded) after the first terminal Context cause commits. Later causes do not overwrite it."},
				{ID: "CKM-CONTEXT-DEADLINE", Name: "Deadline", Kind: CompilerKnownProperty, Result: compilerKnownOption(builtinType("Instant")), Signature: "property Deadline: Option[Instant]", Documentation: "The effective absolute monotonic deadline after parent inheritance, or None when unbounded."},
			}
		}
	} else if !static {
		members = []CompilerKnownMember{
			{ID: "CKM-CONTEXTSOURCE-CONTEXT", Name: "Context", Kind: CompilerKnownProperty, Result: compilerKnownSharedReference(context), Signature: "property Context: ref Context", Documentation: "Borrows the source-owned child Context; neither copies nor transfers it. The borrow cannot outlive ContextSource."},
			{ID: "CKM-CONTEXTSOURCE-CANCEL", Name: "Cancel", Kind: CompilerKnownMethod, Result: Type{Name: "void", Kind: VoidType}, Signature: "fn Cancel() void", Documentation: "Idempotently requests cancellation of the still-existing child Context. Does not destroy Context or ContextSource and does not require a mutable source binding."},
		}
	}
	for i := range members {
		members[i].Rule = "rules/concurrency/cancellation.md — §§35–38,43"
		members[i].Receiver = typ.Name
		members[i].TargetRestriction = "requires target cancellation/deadline support; source signature is not runtime or lifetime proof"
	}
	return members
}
