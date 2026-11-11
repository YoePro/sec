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
	TypeName, InterfaceName, Interface string
	// TypeModule identifies the defining module independently of a target's
	// selected source declaration, for cross-target conformance projections.
	TypeModule                                    string
	TypeDeclaration, InterfaceDeclaration, Clause lexer.Token
	State                                         diagnostics.ProofState
	Failures                                      []InterfaceConformanceFailure
	// Members links each interface requirement to the declaration that
	// implements it in the conforming type, in requirement order; a missing
	// member has no entry.
	Members []InterfaceMemberImplementation
}

// InterfaceMemberImplementation is one implemented interface requirement.
// Rules: rules/declarations/interfaces.md — §§ 4–6; rules/tooling/lsp.md — § 14.
type InterfaceMemberImplementation struct {
	Name           string
	Requirement    lexer.Token
	Implementation lexer.Token
	// Kind is "method", "property" or "event"; DeclaredIn names the
	// interface whose own declaration contains the requirement, which
	// differs from the conformed interface for an inherited requirement.
	Kind       string
	DeclaredIn string
	// RequirementMutable and ImplementationMutable are the receiver
	// capabilities (`mut fn`) of a method requirement and its
	// implementation; Static marks a static requirement.
	RequirementMutable    bool
	ImplementationMutable bool
	Static                bool
}

// noteInterfaceRequirementOwner records the interface that declares a
// requirement.
func (a *Analyzer) noteInterfaceRequirementOwner(token lexer.Token, iface string) {
	if token.Line == 0 || iface == "" {
		return
	}
	if a.interfaceRequirementOwners == nil {
		a.interfaceRequirementOwners = map[sourceTokenKey]string{}
	}
	a.interfaceRequirementOwners[sourceTokenLocation(token)] = iface
}

// interfaceMember builds one implemented-member fact.
func (a *Analyzer) interfaceMember(kind, name string, requirement, implementation lexer.Token) InterfaceMemberImplementation {
	return InterfaceMemberImplementation{
		Name: name, Kind: kind, Requirement: requirement, Implementation: implementation,
		DeclaredIn: a.interfaceRequirementOwners[sourceTokenLocation(requirement)],
	}
}

// InterfaceMemberFact pairs one implemented requirement with the conformance
// it belongs to.
type InterfaceMemberFact struct {
	TypeName, InterfaceName string
	Member                  InterfaceMemberImplementation
}

// InterfaceMemberFactsAt returns the implemented-requirement facts whose
// requirement or implementation is declared at a source position, in
// conformance order. Tooling uses them for receiver capability and
// inherited-requirement provenance without re-checking conformance.
// Rules: rules/declarations/interfaces.md — §§ 5–7; rules/tooling/lsp.md — "Hover".
func (a *Analyzer) InterfaceMemberFactsAt(file string, line, column int) []InterfaceMemberFact {
	if a == nil {
		return nil
	}
	here := sourceTokenKey{File: file, Line: line, Column: column}
	out := []InterfaceMemberFact{}
	for _, fact := range a.interfaceConformanceFacts {
		for _, member := range fact.Members {
			if sourceTokenLocation(member.Requirement) == here || sourceTokenLocation(member.Implementation) == here {
				out = append(out, InterfaceMemberFact{TypeName: fact.TypeName, InterfaceName: fact.InterfaceName, Member: member})
			}
		}
	}
	return out
}

// InterfaceRequirementOwnerAt names the interface that declares the
// requirement at a source position.
func (a *Analyzer) InterfaceRequirementOwnerAt(file string, line, column int) (string, bool) {
	if a == nil {
		return "", false
	}
	owner, ok := a.interfaceRequirementOwners[sourceTokenKey{File: file, Line: line, Column: column}]
	return owner, ok
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
		facts[index].Members = append([]InterfaceMemberImplementation(nil), facts[index].Members...)
		for member := range facts[index].Failures {
			facts[index].Failures[member].Diagnostic = cloneSemanticErrors([]Error{facts[index].Failures[member].Diagnostic})[0]
		}
	}
	return facts
}

// InterfaceImplementationsAt returns the implementing declarations of the
// interface or interface member declared at a source position: every
// explicitly conforming type of an interface, and the implementing member of
// every such type for an interface method, property or event requirement.
// It reads the recorded conformance facts only.
// Rules: rules/declarations/interfaces.md — §§ 4–6; rules/tooling/lsp.md — § 14(1).
func (a *Analyzer) InterfaceImplementationsAt(file string, line, column int) []lexer.Token {
	if a == nil {
		return nil
	}
	here := sourceTokenKey{File: file, Line: line, Column: column}
	out := []lexer.Token{}
	seen := map[sourceTokenKey]bool{}
	add := func(token lexer.Token) {
		key := sourceTokenLocation(token)
		if token.Line > 0 && !seen[key] {
			seen[key] = true
			out = append(out, token)
		}
	}
	for _, fact := range a.interfaceConformanceFacts {
		if sourceTokenLocation(fact.InterfaceDeclaration) == here {
			add(fact.TypeDeclaration)
		}
		for _, member := range fact.Members {
			if sourceTokenLocation(member.Requirement) == here {
				add(member.Implementation)
			}
		}
	}
	return out
}
