package sema

// compilerKnownMutexMembers exposes the canonical acquisition surface
// through the shared member catalog, with no lowercase compatibility aliases.
// Rules: rules/concurrency/mutex.md — §§3,20–24,61–62.
func compilerKnownMutexMembers(typ Type) []CompilerKnownMember {
	if !isMutexType(typ) {
		return nil
	}
	guard := mutexGuardType(typ.TypeArgs[0])
	return []CompilerKnownMember{
		{
			ID: "CKM-MUTEX-LOCK", Name: "Lock", Kind: CompilerKnownMethod,
			Result: guard, Signature: "fn Lock() " + typeDisplayName(guard) + "\nfn Lock(timeout: duration) Option[" + typeDisplayName(guard) + "]\nfn Lock(context: ref Context) Result[" + typeDisplayName(guard) + ", ContextError]",
			Documentation: "Waits for acquisition. Basic Lock returns an owning guard; timeout returns Some(guard) or None; Context waiting returns Ok(guard) or ContextError.Cancelled/DeadlineExceeded. The guard does not retain the supplied Context. Current-execution cancellation remains distinct and terminal.",
			Rule:          "rules/concurrency/mutex.md — §§20–24", Receiver: "shared " + typeDisplayName(typ),
			TargetRestriction: "requires CompilationPlan lock and wait capabilities",
		},
		{
			ID: "CKM-MUTEX-TRYLOCK", Name: "TryLock", Kind: CompilerKnownMethod,
			Result: compilerKnownOption(guard), Signature: "fn TryLock() Option[" + typeDisplayName(guard) + "]",
			Documentation: "Never waits. Some(guard) means acquisition committed immediately; None means the mutex was not immediately available.",
			Rule:          "rules/concurrency/mutex.md — §§20,22", Receiver: "shared " + typeDisplayName(typ),
			TargetRestriction: "requires CompilationPlan lock capabilities",
		},
	}
}
