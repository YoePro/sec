package main

import (
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

type conformanceSubject struct {
	name   string
	token  lexer.Token
	isType bool
}

// conformanceSubjects lists only active source declarations; possible type lenses
// are resolved from Sema later, so enumeration loads no target or sibling source.
// Rules: rules/tooling/lsp.md — "Interface conformance code lens";
// rules/declarations/interfaces.md — §§4–6,12.
func conformanceSubjects(program *ast.Program) []conformanceSubject {
	var subjects []conformanceSubject
	for _, statement := range program.Statements {
		var name *ast.Identifier
		isType := true
		switch declaration := statement.(type) {
		case *ast.InterfaceDeclaration:
			if declaration != nil {
				name = declaration.Name
			}
			isType = false
		case *ast.TypeDeclStatement:
			if declaration != nil {
				name = declaration.Name
			}
		case *ast.EnumDeclaration:
			if declaration != nil {
				name = declaration.Name
			}
		}
		if name != nil {
			subjects = append(subjects, conformanceSubject{name.Value, name.Token, isType})
		}
	}
	return subjects
}

// matches keeps exact interface declaration identity for interface views and
// module/type identity for a type's explicitly declared conformance facts.
// Rules: rules/compiler/semantic_ir.md — §109(1–4);
// rules/declarations/interfaces.md — §§4–6,12.
func (subject conformanceSubject) matches(fact sema.InterfaceConformanceFact, module string) bool {
	if subject.isType {
		return fact.TypeName == subject.name && fact.TypeModule == module
	}
	return sameSourceToken(fact.InterfaceDeclaration, subject.token)
}

// failureMember disambiguates equal member names from distinct requirements.
// Rules: rules/declarations/interfaces.md — §12; rules/tooling/lsp.md — "Code lens".
func (subject conformanceSubject) failureMember(iface, member string) string {
	if subject.isType {
		return iface + "." + member
	}
	return member
}

// visibleConformanceSummaries suppresses source types with no explicit Sema
// conformance on any selected target; absence is never presented as success.
// Rules: rules/tooling/lsp.md — "Unknown facts", "Interface conformance code lens".
func visibleConformanceSummaries(summaries []interfaceConformanceSummary) []interfaceConformanceSummary {
	result := make([]interfaceConformanceSummary, 0, len(summaries))
	for _, summary := range summaries {
		visible := !summary.typeSubject
		for _, target := range summary.targets {
			visible = visible || len(target.implementers) > 0
		}
		if visible {
			if summary.typeSubject {
				markMissingTypeConformances(&summary)
			}
			result = append(result, summary)
		}
	}
	return result
}

// crossTargetInterfaceConformance preserves the interface-only projection for
// existing consumers while sharing one cached module report with type lenses.
// Rules: rules/tooling/lsp.md — "Interface conformance code lens".
func crossTargetInterfaceConformance(uri, text string, overlay sourceOverlay) []interfaceConformanceSummary {
	var interfaces []interfaceConformanceSummary
	for _, summary := range crossTargetConformance(uri, text, overlay) {
		if !summary.typeSubject {
			interfaces = append(interfaces, summary)
		}
	}
	return interfaces
}

// conformanceDeclarationLocation projects source tokens through the shared
// UTF-16/unqualified range conversion, including imported interface names.
// Rules: rules/tooling/lsp.md — "Navigation", "Code lens".
func conformanceDeclarationLocation(uri, text string, overlay sourceOverlay, token lexer.Token, fallback lspRange) location {
	if token.File == "" {
		return location{URI: uri, Range: fallback}
	}
	return location{URI: uriFromPath(token.File), Range: definitionTokenRange(sourceTextForToken(uri, text, overlay, token), token)}
}

// conformanceDisplayName labels an implementing type and its interfaces without
// changing the existing interface-view command labels.
// Rules: rules/tooling/lsp.md — "Interface conformance code lens".
func conformanceDisplayName(summary interfaceConformanceSummary, names []string) string {
	joined := strings.Join(names, ", ")
	if summary.typeSubject {
		return summary.name + " (" + joined + ")"
	}
	return joined
}

// conformanceTypePrefix identifies which source type owns a failing report.
// Rules: rules/tooling/lsp.md — "Interface conformance code lens".
func conformanceTypePrefix(summary interfaceConformanceSummary) string {
	if summary.typeSubject {
		return summary.name + ": "
	}
	return ""
}

// markMissingTypeConformances reports each interface not established on a target
// rather than allowing another successful interface to mask its absence.
// Rules: rules/tooling/lsp.md — "Interface conformance code lens", "Unknown facts";
// rules/declarations/interfaces.md — §§4–6,12.
func markMissingTypeConformances(summary *interfaceConformanceSummary) {
	required := map[string]location{}
	for _, target := range summary.targets {
		for name, loc := range target.implementers {
			required[name] = loc
		}
	}
	names := make([]string, 0, len(required))
	for name := range required {
		names = append(names, name)
	}
	sort.Strings(names)
	for index := range summary.targets {
		target := &summary.targets[index]
		if len(target.implementers) == 0 {
			continue
		}
		for _, name := range names {
			if _, found := target.implementers[name]; !found {
				target.failures = append(target.failures, interfaceConformanceFailure{member: name + " (no declared conformance)", location: required[name]})
			}
		}
	}
}
