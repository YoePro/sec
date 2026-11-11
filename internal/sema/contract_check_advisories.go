package sema

import (
	"fmt"
	"sort"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// uncheckedContractSite describes one run-time contract check outside try
// for the A2011 advisory: what is checked and against which named type.
type uncheckedContractSite struct {
	what string
	typ  Type
}

// noteUncheckedContract remembers the description of a may-panic contract
// effect site. The call-graph effect stays the single source of truth: a
// site whose effect a try later removes reports nothing.
func (a *Analyzer) noteUncheckedContract(source lexer.Token, what string, typ Type) {
	if a.uncheckedContractSites == nil {
		a.uncheckedContractSites = map[sourceTokenKey]uncheckedContractSite{}
	}
	a.uncheckedContractSites[sourceTokenLocation(source)] = uncheckedContractSite{what: what, typ: typ}
}

// recordCompoundContractEffect publishes the run-time contract check of an
// untried compound update of a constrained target. The update is valid and
// panics with ContractFailure on violation (owner decision 2026-11-11);
// `try target op= value` turns the failure into ContractError instead.
//
// Rules:
//   - rules/types/contracts.md — § 12(2)–(3)
//   - rules/errors/panic.md — ContractFailure, § 21 "@noPanic"
func (a *Analyzer) recordCompoundContractEffect(source lexer.Token, target string, typ Type) {
	if a.summaryPass {
		return
	}
	a.callGraph.removeEffect(a.currentCallable, EffectMayPanicContract, source)
	if !a.callGraphPathReachable {
		return
	}
	a.callGraph.addEffect(a.currentCallable, EffectSite{Kind: EffectMayPanicContract, Source: source, PanicReasonIDs: []diagnostics.PanicReasonID{diagnostics.PanicReasonContractFailure}})
	a.noteUncheckedContract(source, "compound update of "+target, typ)
}

// emitUncheckedContractAdvisories reports A2011 once per remaining
// may-panic contract effect: every run-time contract check that no try
// protects. It reads the final call-graph effects, so try-protected
// conversions and updates never report.
//
// Rules:
//   - rules/types/contracts.md — § 12
//   - rules/tooling/diagnostics.md — § 18 advisory policy
func (a *Analyzer) emitUncheckedContractAdvisories() {
	if a.callGraph == nil {
		return
	}
	seen := map[sourceTokenKey]bool{}
	sites := []EffectSite{}
	for _, effects := range a.callGraph.effects {
		for _, effect := range effects {
			if effect.Kind != EffectMayPanicContract {
				continue
			}
			key := sourceTokenLocation(effect.Source)
			if seen[key] {
				continue
			}
			seen[key] = true
			sites = append(sites, effect)
		}
	}
	sort.Slice(sites, func(i, j int) bool {
		left, right := sites[i].Source, sites[j].Source
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Column < right.Column
	})
	for _, site := range sites {
		described, ok := a.uncheckedContractSites[sourceTokenLocation(site.Source)]
		what, typeName := "run-time contract check", "a constrained type"
		if ok {
			what = described.what
			if described.typ.Name != "" {
				typeName = typeDisplayName(described.typ)
			}
		}
		endLine, endColumn := site.Source.EndPosition()
		a.appendWarning(Error{
			ID:       diagnostics.UncheckedContractMayPanic,
			Severity: diagnostics.SeverityInformation,
			Message:  fmt.Sprintf("%s is checked against the contracts of %s at run time and panics on violation", what, typeName),
			Help:     "This is valid. Write try to handle a violation as ContractError instead, for example `try p = Percent(value) { Err(error) => ... }`.",
			File:     site.Source.File, Line: site.Source.Line, Column: site.Source.Column, EndLine: endLine, EndColumn: endColumn,
		})
	}
}

// markContractedTryTarget records that a try assignment stores into a
// constrained field, property, element or referent, so its error channel is
// ContractError (contracts.md § 12; MD-131).
func (a *Analyzer) markContractedTryTarget(stmt *ast.AssignmentStatement) {
	if a.contractedMemberTryTargets == nil {
		a.contractedMemberTryTargets = map[*ast.AssignmentStatement]bool{}
	}
	a.contractedMemberTryTargets[stmt] = true
}
