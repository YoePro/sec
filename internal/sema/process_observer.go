package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// builtinProcessObserverType is a copyable, non-owning opaque observation value,
// not a Process lifecycle owner or a fabricated default/struct representation.
// Rules: rules/concurrency/processes.md §5.14; rules/concurrency/select.md §25.
func builtinProcessObserverType() Type {
	return Type{Name: "ProcessObserver", Module: "core", Kind: StructType, Named: true, Declared: true, Intrinsic: true}
}

// isProcessObserverType requires canonical intrinsic identity, not a Wait method
// spelling, structurally identical user declaration or result-type resemblance.
// Rules: rules/concurrency/processes.md §5.14; rules/concurrency/select.md §§25,51,54.
func isProcessObserverType(typ Type) bool {
	typ = dereferenceType(typ)
	return typ.Intrinsic && typ.Kind == StructType && typ.Name == "ProcessObserver" && typ.Module == "core" && len(typ.TypeArgs) == 0
}

// processObserverMembers publishes metadata and the exact repeatable wait.
// Platform remains a target-declaration boundary rather than a portable invented
// struct. Wait never grants join, detach, termination or owner result access.
// Rules: rules/concurrency/processes.md §§5.14–5.15,21.1–21.2;
// rules/concurrency/select.md §25.
func processObserverMembers(typ Type) []CompilerKnownMember {
	if !isProcessObserverType(typ) {
		return nil
	}
	members := []CompilerKnownMember{
		{ID: "CKM-PROCESSOBSERVER-ID", Name: "ID", Kind: CompilerKnownProperty, Result: builtinType("ProcessID"), Signature: "property ID: ProcessID", Documentation: "Stable observed process identity; this metadata grants no owning lifecycle capability."},
		{ID: "CKM-PROCESSOBSERVER-NAME", Name: "Name", Kind: CompilerKnownProperty, Result: builtinType("string"), Signature: "property Name: string", Documentation: "Read-only logical name of the observed process."},
		{ID: "CKM-PROCESSOBSERVER-STATUS", Name: "Status", Kind: CompilerKnownProperty, Result: builtinType("ProcessStatus"), Signature: "property Status: ProcessStatus", Documentation: "Read-only process lifecycle status; observing it does not join or reap the process."},
		{ID: "CKM-PROCESSOBSERVER-WAIT", Name: "Wait", Kind: CompilerKnownMethod, Result: builtinType("ProcessStatus"), Signature: "fn Wait() ProcessStatus", Documentation: "Repeatable non-owning wait for a terminal ProcessStatus. Selectable; only a selected branch commits. Never joins, reaps, detaches or exposes the owner's result."},
	}
	for index := range members {
		members[index].Rule = "rules/concurrency/processes.md — §5.14, §21"
		members[index].Receiver = "ProcessObserver"
	}
	return members
}

// inferProcessObserverWait validates the source operation without changing any
// owner lifecycle or observer availability. Select preparation records readiness
// through its existing context; only ordinary waits add a blocking effect.
// Rules: rules/concurrency/processes.md §§5.14,21.1–21.2;
// rules/concurrency/select.md §§25,54; rules/concurrency/blocking.md — Select.
func (a *Analyzer) inferProcessObserverWait(call *ast.CallExpression, member *ast.MemberExpression) (Type, expressionValue, bool) {
	if len(call.GenericArguments) != 0 {
		a.addErrorAtTokenWithMetadata(call.Token, diagnostics.ProcessObserverInvalidOperation, "Call observer.Wait() with no arguments or type arguments; it returns ProcessStatus without consuming owner lifecycle authority.", "ProcessObserver.Wait does not take type arguments")
		return Type{Kind: InvalidType}, expressionValue{Display: call.String()}, true
	}
	if len(call.Arguments) != 0 {
		a.addErrorAtTokenWithMetadata(call.Token, diagnostics.ProcessObserverInvalidOperation, "Call observer.Wait() with no arguments or type arguments; it returns ProcessStatus without consuming owner lifecycle authority.", "ProcessObserver.Wait expects 0 arguments, got %d", len(call.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: call.String()}, true
	}
	a.recordBlockingOperation("ProcessObserver.Wait", member.Property.Token)
	return a.types["ProcessStatus"], expressionValue{Display: call.String()}, true
}

// rejectProcessObserverPlatform keeps target-specific metadata explicit until
// the selected ProcessPlatform has a complete declaration.
// Rules: rules/concurrency/processes.md §5.15; rules/compiler/compiler_pipeline.md §32.
func (a *Analyzer) rejectProcessObserverPlatform(typ Type, name string, token lexer.Token) bool {
	if !isProcessObserverType(typ) || name != "Platform" {
		return false
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.CompilerCapabilityUnavailable, "A complete selected-target ProcessPlatform declaration is required before exposing process platform metadata.", "compiler capability unavailable: ProcessObserver.Platform target declaration")
	return true
}

// reportProcessObserverConstruction rejects fabricated runtime observation state.
// Rules: rules/concurrency/processes.md §5.14; rules/types/default_values.md — compiler-known defaults.
func (a *Analyzer) reportProcessObserverConstruction(token lexer.Token, literal bool) {
	form := "source constructor"
	if literal {
		form = "source struct constructor"
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.ProcessObserverInvalidOperation, "Obtain an observer from an established Process or Command owner through Observe(); observation state is runtime-private and cannot be fabricated.", "ProcessObserver has no %s; its representation is runtime-private", form)
}
