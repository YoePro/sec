package sema

import (
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// InterfaceConformanceFact records one explicit conformance checked by Sema.
// Member identities are structured data, independent of diagnostic wording.
// Rules: rules/declarations/interfaces.md — §§4–6,12;
// rules/compiler/semantic_ir.md — §109(1–4).
type InterfaceConformanceFact struct {
	TypeName, InterfaceName, Interface            string
	TypeDeclaration, InterfaceDeclaration, Clause lexer.Token
	State                                         diagnostics.ProofState
	Failures                                      []InterfaceConformanceFailure
}

type InterfaceConformanceFailure struct {
	Member     string
	Diagnostic Error
}

// InterfaceConformances returns detached results in deterministic Sema
// validation order. Consumers never probe Implements or parse diagnostic text.
// Rules: rules/compiler/semantic_ir.md — §109(1–4);
// rules/compiler/compiler_analysis.md — §18(6).
func (a *Analyzer) InterfaceConformances() []InterfaceConformanceFact {
	if a == nil {
		return nil
	}
	facts := append([]InterfaceConformanceFact(nil), a.interfaceConformanceFacts...)
	for index := range facts {
		facts[index].Failures = append([]InterfaceConformanceFailure(nil), facts[index].Failures...)
		for member := range facts[index].Failures {
			facts[index].Failures[member].Diagnostic = cloneSemanticErrors([]Error{facts[index].Failures[member].Diagnostic})[0]
		}
	}
	return facts
}
