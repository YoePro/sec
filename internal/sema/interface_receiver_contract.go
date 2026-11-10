package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// validateInterfaceReceiverContract protects semantic analysis of imported or
// constructed ASTs from reintroducing the removed whole-self receiver contract.
// Ordinary consuming parameters and callable-value capabilities are independent.
// Rules: rules/declarations/interfaces.md — §3.3;
// rules/memory/ownership.md — Whole-self consumption.
func (a *Analyzer) validateInterfaceReceiverContract(method *ast.FunctionDeclaration) bool {
	if method.ReceiverCapability != ast.ReceiverConsuming {
		return true
	}
	a.addErrorAtTokenWithMetadata(method.Name.Token, diagnostics.InterfaceConsumingReceiver,
		"Use a shared fn or mutable mut fn requirement. Put -> on an ordinary parameter when that argument must be consumed; whole-instance destruction belongs to free.",
		"interface method %s cannot consume the complete receiver", method.Name.Value)
	return false
}
