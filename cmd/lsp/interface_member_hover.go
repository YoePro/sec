package main

import (
	"fmt"
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// interfaceMemberHover presents interface conformance facts at an interface
// requirement or at the member that implements it: the conformed interface,
// the interface that declares an inherited requirement, the receiver
// capability required and provided, and (at a requirement) every
// implementing type. It reads Sema conformance facts only.
// Rules: rules/declarations/interfaces.md — §§ 5–7; rules/tooling/lsp.md — "Hover".
func interfaceMemberHover(analyzer *sema.Analyzer, token lexer.Token) string {
	if analyzer == nil {
		return ""
	}
	facts := analyzer.InterfaceMemberFactsAt(token.File, token.Line, token.Column)
	owner, isRequirement := analyzer.InterfaceRequirementOwnerAt(token.File, token.Line, token.Column)
	if len(facts) == 0 && !isRequirement {
		return ""
	}
	lines := []string{}
	if isRequirement {
		lines = append(lines, fmt.Sprintf("- requirement of `%s`", owner))
		if len(facts) > 0 {
			lines = append(lines, "- receiver capability: "+requirementCapability(facts[0].Member))
		}
		implementers := []string{}
		for _, fact := range facts {
			via := ""
			if fact.InterfaceName != owner {
				via = fmt.Sprintf(" via `%s`", fact.InterfaceName)
			}
			implementers = append(implementers, fmt.Sprintf("`%s`%s (%d:%d)", fact.TypeName, via, fact.Member.Implementation.Line, fact.Member.Implementation.Column))
		}
		if len(implementers) == 0 {
			lines = append(lines, "- implemented by: no explicitly conforming type in this analysis")
		} else {
			lines = append(lines, "- implemented by: "+strings.Join(implementers, ", "))
		}
		return "\n\n**Interface requirement**\n\n" + strings.Join(lines, "\n")
	}
	for _, fact := range facts {
		member := fact.Member
		line := fmt.Sprintf("- implements `%s.%s` for `%s`", fact.InterfaceName, member.Name, fact.TypeName)
		if member.DeclaredIn != "" && member.DeclaredIn != fact.InterfaceName {
			line += fmt.Sprintf("; inherited from `%s`", member.DeclaredIn)
		}
		line += fmt.Sprintf(" (requirement at %d:%d)", member.Requirement.Line, member.Requirement.Column)
		lines = append(lines, line)
		if member.Kind == "method" && !member.Static {
			provided := "this method does not mutate its receiver"
			if member.ImplementationMutable {
				provided = "this method mutates its receiver"
			}
			lines = append(lines, "- receiver capability: requirement "+requirementCapability(member)+"; "+provided)
		}
	}
	return "\n\n**Interface implementation**\n\n" + strings.Join(lines, "\n")
}

func requirementCapability(member sema.InterfaceMemberImplementation) string {
	switch {
	case member.Kind != "method":
		return "not applicable to a " + member.Kind
	case member.Static:
		return "static; no receiver"
	case member.RequirementMutable:
		return "mutable/exclusive receiver access (`mut fn`); a shared `ref` view cannot call it"
	}
	return "shared receiver access (`fn`); callable through a shared `ref` view"
}
