package sema

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// pitfallRootCause names the owning semantic cause at a canonical operation.
// It does not use diagnostic message text or proximity to infer ownership.
// Rules: rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing".
type pitfallRootCause struct {
	operation sourceTokenKey
	rule      string
}

// recordPitfallDiagnosticOwner publishes an owner only when its producer emitted
// a registered normative error. Summary passes and unsuccessful reporting retain
// no owner, and unrelated errors at the same operation keep separate causes.
// Rules: rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing";
// rules/tooling/diagnostics.md — §§8–10 diagnostic identity and occurrences.
func (a *Analyzer) recordPitfallDiagnosticOwner(root ast.Expression, rule string, before int) {
	if len(a.errors) <= before || root == nil {
		return
	}
	index := len(a.errors) - 1
	if a.errors[index].ID == "" {
		return
	}
	if a.pitfallDiagnosticOwners == nil {
		a.pitfallDiagnosticOwners = map[pitfallRootCause]int{}
	}
	a.pitfallDiagnosticOwners[pitfallRootCause{sourceTokenLocation(expressionToken(root)), rule}] = index
}

// coalescePitfallDiagnostic links a supported occurrence to its exact normative
// producer and adds evidence/fix guidance to that one error. Suppressed findings
// cannot change mandatory errors, and proof quality is never upgraded here.
// Rules: rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing",
// "Proven errors cannot be heuristically suppressed", "Fix safety".
func (a *Analyzer) coalescePitfallDiagnostic(finding *PitfallFinding, operation lexer.Token) {
	if finding.State == PitfallStateSuppressed {
		return
	}
	rule := finding.OwningRule
	if rule == "checked-arithmetic-and-bounds" {
		rule = "bounds"
	}
	index, found := a.pitfallDiagnosticOwners[pitfallRootCause{sourceTokenLocation(operation), rule}]
	if !found && rule == "bounds" {
		index, found = a.boundsDiagnostics[sourceTokenLocation(operation)]
	}
	if !found || index < 0 || index >= len(a.errors) {
		return
	}
	a.enrichPitfallDiagnostic(index, finding)
}

// enrichPitfallDiagnostic preserves the owner's identity, severity and primary
// location while appending detached pitfall explanations and explicitly suggested
// edits. Automatic fixes require separate safety certification and transport.
// Rules: rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing",
// "Fix safety"; rules/tooling/diagnostics.md — §§8–10, 12, 14.
func (a *Analyzer) enrichPitfallDiagnostic(index int, finding *PitfallFinding) {
	e := &a.errors[index]
	finding.DiagnosticID = e.ID
	note := fmt.Sprintf("\n%s (%s):", finding.Rule, finding.Classification)
	if strings.Contains(e.Help, note) {
		return
	}
	if e.PreviousLine == 0 {
		for _, evidence := range finding.EvidenceFor {
			source := evidence.Source
			if validDefinitionToken(source) && (source.Line != e.Line || source.Column != e.Column || source.File != e.File) {
				e.PreviousFile, e.PreviousLine, e.PreviousColumn = source.File, source.Line, source.Column
				e.RelatedLabel = "pitfall evidence"
				break
			}
		}
	}
	e.Help += note
	for _, evidence := range finding.EvidenceFor {
		e.Help += "\n" + evidence.Fact
	}
	for _, action := range finding.Actions {
		e.Help += "\nSuggested edit: " + action.Title
		if action.Replacement != "" {
			e.Help += " (" + action.Replacement + ")"
		}
	}
}

// addForDiagnosticRoot keeps presentation (e.g. an outer bool conversion)
// independent of the inner operation which establishes the normative cause.
// Rules: rules/analysis/pitfall_analysis.md — "Source mapping", "Diagnostic ownership and coalescing".
func (b *pitfallBuilder) addForDiagnosticRoot(finding PitfallFinding, root ast.Expression) {
	if evaluation := b.counts[finding.Rule]; evaluation == nil || evaluation.State == PitfallStateNotEvaluated {
		return
	}
	b.analyzer.coalescePitfallDiagnostic(&finding, expressionToken(root))
	b.add(finding)
}
