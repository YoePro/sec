package sema

import (
	"fmt"
	"sort"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// recordRedundantDiscard records only a proven legal no-op after discardability
// and borrow/lifecycle checks. Conditional or backedge-only absence is not a
// redundancy proof. Summary/probe/dead-path visits never create advisories.
// Rules: rules/control-flow/discard.md — §§5,26,37–38;
// rules/memory/destruction.md — §§12.2(3),33.2(4).
func (a *Analyzer) recordRedundantDiscard(stmt *ast.DiscardStatement, place Place, movedKey string, endedAt lexer.Token) {
	if a.summaryPass || a.inDeferBlock || a.iterationDependencyProbe || !a.callGraphPathReachable || a.loopBackedgePlaces[movedKey] || isConditionalAvailabilityReason(a.moveReasons[movedKey]) {
		return
	}
	// Advise only direct bindings and stored fields. Computed projections or
	// alternative provenance need an independent operand-evaluation proof.
	if place.AmbiguousProvenance || len(place.AlternativeOrigins) != 0 {
		return
	}
	for _, projection := range place.Projections {
		if projection.Kind != PlaceField {
			return
		}
	}
	endLine, endColumn := stmt.Token.EndPosition()
	fact := Error{ID: diagnostics.RedundantDiscard, Severity: diagnostics.SeverityInformation,
		Message: "discard of " + place.String() + " is redundant because the value is already unavailable; this is a legal no-op",
		Help:    "Keep the discard if it documents intent. Removing this proven redundant discard does not restore ownership or make the value available.",
		File:    stmt.Token.File, Line: stmt.Token.Line, Column: stmt.Token.Column, EndLine: endLine, EndColumn: endColumn}
	if endedAt.Line > 0 {
		fact.PreviousFile, fact.PreviousLine, fact.PreviousColumn = endedAt.File, endedAt.Line, endedAt.Column
		fact.RelatedLabel = "earlier ownership-ending action"
	}
	a.explicitDiscardAdvisories[stmt] = fact
}

// emitExplicitDiscardAdvisories projects proven explicit-discard advice through the shared
// project policy only after mandatory semantic analysis has completed. A rule
// promoted to error rejects compilation without reinterpreting discard itself.
// Rules: rules/control-flow/discard.md — §§5,38;
// rules/tooling/diagnostics.md — §18(3)–(4).
func (a *Analyzer) emitExplicitDiscardAdvisories() {
	// Recovery after a mandatory error can retain incomplete move state; it
	// must not become a positive no-op proof for optional advice.
	if len(a.errors) != 0 {
		return
	}
	facts := make([]Error, 0, len(a.explicitDiscardAdvisories))
	for _, fact := range a.explicitDiscardAdvisories {
		facts = append(facts, fact)
	}
	sort.Slice(facts, func(i, j int) bool {
		if facts[i].File != facts[j].File {
			return facts[i].File < facts[j].File
		}
		if facts[i].Line != facts[j].Line {
			return facts[i].Line < facts[j].Line
		}
		return facts[i].Column < facts[j].Column
	})
	for _, fact := range facts {
		a.appendWarning(fact)
	}
}

// recordExplicitDiscardAdvisory recognizes only a legal void expression or a
// proven compile-time temporary without a Place. Discard of a binding or field
// changes ownership even when its value is known, and must not be called useless.
// Evaluating a constant uses existing semantic CTE, never a second evaluator.
// Rules: rules/control-flow/discard.md — §§7,9,13,27,37–38;
// rules/compiler/compile_time_evaluation.md — SemanticCompileTimeRequiredContext.
func (a *Analyzer) recordExplicitDiscardAdvisory(stmt *ast.DiscardStatement, typ Type) {
	if a.summaryPass || a.iterationDependencyProbe || !a.callGraphPathReachable {
		return
	}
	id := diagnostics.RedundantVoidDiscard
	message := "explicit discard of void is redundant; the expression already produces no value"
	help := "Remove only the `discard` keyword and keep the expression so its effects still execute."
	if typ.Kind != VoidType {
		if !TriviallyDestructible(typ) {
			return
		}
		if _, place := a.resolvePlace(stmt.Value); place {
			return
		}
		if _, outcome := a.semanticCompileTimeConstant(stmt.Value); outcome != compileTimeEvaluated {
			return
		}
		id = diagnostics.UselessExplicitDiscard
		message = fmt.Sprintf("discard of a compile-time %s value has no observable ownership effect", typeDisplayName(typ))
		help = "Remove this statement if the constant expression is not needed to document intent. Discard of a local binding remains an ownership-ending operation."
	}
	endLine, endColumn := stmt.Token.EndPosition()
	a.explicitDiscardAdvisories[stmt] = Error{ID: id, Severity: diagnostics.SeverityInformation,
		Message: message, Help: help, File: stmt.Token.File, Line: stmt.Token.Line, Column: stmt.Token.Column, EndLine: endLine, EndColumn: endColumn}
}
