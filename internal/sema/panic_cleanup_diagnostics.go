package sema

import (
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// enterIndependentDeferScope separates an invoked lambda's cleanup control flow
// from the deferred block in which its value was created. Capture construction
// remains in the enclosing scope; body analysis belongs to the new invocation.
// Rules: rules/control-flow/defer.md — §§3,16–18;
// rules/declarations/lambda-functions.md — Lambda bodies.
func (a *Analyzer) enterIndependentDeferScope() func() {
	previous, outer, captures := a.inDeferBlock, a.deferOuterSymbols, a.deferCaptures
	previousFree := a.currentFunctionMetadata.CustomFree
	a.inDeferBlock, a.deferOuterSymbols, a.deferCaptures = false, nil, nil
	a.currentFunctionMetadata.CustomFree = false
	return func() {
		a.inDeferBlock, a.deferOuterSymbols, a.deferCaptures = previous, outer, captures
		a.currentFunctionMetadata.CustomFree = previousFree
	}
}

// reportInvalidDeferredCleanup explains why cleanup cannot restart its exiting
// invocation or register another cleanup, while retaining the established error.
// Rules: rules/control-flow/defer.md — §§3,14–18,28;
// rules/errors/panic.md — §§11,28(1–2).
func (a *Analyzer) reportInvalidDeferredCleanup(token lexer.Token, operation string) {
	help := "A deferred cleanup block runs while its enclosing invocation is exiting. Perform the cleanup directly and let the block finish normally; it cannot redirect the pending return, loop transfer or cancellation."
	if operation == "defer" {
		help = "A deferred block cannot register another defer. Perform both cleanup actions directly in this block, or register separate defer blocks in the enclosing invocation."
	}
	if operation == "await" {
		help = "Deferred cleanup cannot suspend with await. Finish awaited work before leaving the invocation, or use cleanup that completes synchronously."
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.InvalidDeferredCleanup, help, "%s is not allowed inside defer", operation)
}

// deferredPropagationHelp gives only repairs available during cleanup. Changing
// the enclosing return type cannot make propagation out of a defer legal.
// Rules: rules/control-flow/defer.md — §14; rules/errors/errorhandling.md — §32;
// rules/errors/panic.md — §11.1.
func deferredPropagationHelp() string {
	return "Deferred cleanup cannot propagate a failure into the already-exiting invocation, even when that invocation returns Result or Option. Handle every failure locally with exhaustive Err handlers or an unguarded None handler, and let cleanup finish normally without return or panic."
}

// panicCleanupContext reads the canonical cause path rather than rediscovering
// deferred calls from syntax. It does not infer new cleanup or execution policy.
// Rules: rules/errors/panic.md — §§11,21,28(2–3); rules/analysis/call_graph.md — Defer bodies.
func (a *Analyzer) panicCleanupContext(kindLabel string, path []CallableID) string {
	if kindLabel == "free" {
		return "free cleanup"
	}
	for _, id := range path {
		if node, ok := a.callGraph.Node(id); ok {
			if node.Kind == CallableBodyDefer {
				return "deferred cleanup"
			}
			if node.ImplTarget != "" && node.Name == freeFunctionName(node.ImplTarget) {
				return "free cleanup"
			}
		}
	}
	return ""
}

// cleanupPanicHelp proposes local, panic-free repairs without suggesting that a
// cleanup return or removing a guarantee can repair a panic-cleanup contract.
// Rules: rules/errors/panic.md — §§11–12,19,28; rules/control-flow/defer.md — §14.
func cleanupPanicHelp(kind EffectKind) string {
	prefix := "Cleanup must finish without introducing another panic; handle recoverable failures locally, without returning them from the exiting invocation. "
	switch kind {
	case EffectMayPanicArithmetic:
		return prefix + "Prove the operand ranges or use exhaustive local try handlers for ArithmeticError."
	case EffectMayPanicBounds:
		return prefix + "Prove the index or interval is in range, or handle IndexError/RangeError locally as appropriate."
	case EffectMayPanicStringMaterialization:
		return prefix + "Avoid runtime materialization or handle StringError with exhaustive local try handlers."
	case EffectMayPanicContract:
		return prefix + "Prove the conversion contract or handle the conversion's declared error locally."
	case EffectMayPanicAssertion, EffectMayPanicUnreachable:
		return prefix + "Prove the failing path unreachable or handle the condition locally without assert, unreachable or panic."
	case EffectMayPanicForeign:
		return prefix + "Unknown foreign behavior is not proof of noPanic. Supply a documented trusted @noPanic foreign contract only if the foreign function cannot abort or unwind, or move the call outside cleanup."
	case EffectMayPanicUnknownCallee:
		return prefix + "Use a closed, compiler-resolved call target set whose members are all panic-free, or move the unknown call outside cleanup."
	case EffectMayPanicInvalidReference:
		return prefix + "Keep the referenced Arena live and unreset until cleanup finishes, or remove the stale-capable access from cleanup."
	default:
		return prefix + "Remove the panic-capable operation, prove it unreachable or move it outside cleanup."
	}
}
