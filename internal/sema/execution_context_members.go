package sema

// executionContextStaticMembers supplies the exact current-execution yield
// operations, independently of any owning Task[T] or Thread[T] handle.
// Rules: rules/concurrency/scheduling.md — §§7(1–4),11(1–4),12,13,55–56.
func executionContextStaticMembers(typ Type) []CompilerKnownMember {
	if typ.Module != "" && typ.Module != "core" {
		return nil
	}
	var id, documentation, target string
	switch {
	case typ.Kind == StructType && typ.Name == "Task":
		id = "CKM-TASK-YIELD"
		documentation = "Acts on the current logical task; requires a real logical task context (ordinary main has none). A scheduling point and a current-task cancellation point; cancellation may prevent normal return. Creates no synchronization edge and guarantees no fairness or other-task progress. Does not consume a Task handle."
		target = "requires a logical task context and target task scheduling support"
	case typ.Kind == StructType && typ.Name == "Thread":
		id = "CKM-THREAD-YIELD"
		documentation = "Acts on the current physical thread as a scheduler hint. Not a cancellation point; does not implicitly inspect or commit cancellation. Creates no synchronization edge and guarantees no fairness, context switch or other-thread progress. Does not consume a Thread handle."
		target = "requires a physical Sec thread context; forbidden in interrupt routines"
	default:
		return nil
	}
	return []CompilerKnownMember{{
		ID: id, Name: "Yield", Kind: CompilerKnownAssociatedFunction,
		Result: Type{Name: "void", Kind: VoidType}, Signature: "static fn Yield() void",
		Documentation: documentation, Rule: "rules/concurrency/scheduling.md — §§7,11–13,55–56",
		Receiver: typ.Name + " (type-qualified; current execution, no handle)", TargetRestriction: target,
	}}
}
