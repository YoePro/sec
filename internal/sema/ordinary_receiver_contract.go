package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// validateOrdinaryReceiverContract keeps constructed or recovered declarations
// from publishing a consuming whole-self receiver. Body-derived field ownership
// effects remain mutable receiver authority; callable-value consumption is separate.
// Rules: rules/memory/ownership.md — §9;
// rules/declarations/functions.md — §§22,24–25;
// rules/corrections/applied/correction-arena-release-lifecycle-20260902.md — ordinary methods.
func (a *Analyzer) validateOrdinaryReceiverContract(fn *ast.FunctionDeclaration) bool {
	if fn.ReceiverCapability != ast.ReceiverConsuming {
		return true
	}
	a.addErrorAtTokenWithMetadata(fn.Name.Token, diagnostics.OrdinaryConsumingReceiver,
		"Use an ordinary fn with body-derived shared or mutable receiver authority. Consume owned members under exclusive access, or use free for whole-instance destruction; consuming callable values and ordinary -> parameters remain supported.",
		"ordinary function or method %s cannot declare a whole-self consuming receiver", fn.Name.Value)
	return false
}
