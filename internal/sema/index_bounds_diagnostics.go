package sema

import (
	"math/big"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// validateLengthIndexBounds owns the required direct-Len and reachable
// inclusive-endpoint proofs after canonical body facts are available. The
// same proof results feed optional pitfall explanations without rederivation;
// advisory depth and search budgets cannot disable normative bounds errors.
// Rules: rules/analysis/pitfall_analysis.md — "Proven invalidity is not a warning",
// "Direct index at length", "Inclusive upper bound against collection length",
// "Diagnostic ownership and coalescing"; rules/compiler/compiler_analysis.md — §§6, 15.
func (a *Analyzer) validateLengthIndexBounds(program *ast.Program) {
	b := &pitfallBuilder{analyzer: a, boundsOnly: true, result: newPitfallAnalysis(), counts: map[PitfallRuleID]*PitfallRuleEvaluation{
		PitfallDirectIndexAtLength:  {Rule: PitfallDirectIndexAtLength, State: PitfallStateNoFinding},
		PitfallInclusiveLengthIndex: {Rule: PitfallInclusiveLengthIndex, State: PitfallStateNoFinding},
	}}
	if program != nil {
		for _, statement := range program.Statements {
			b.walkStatement(statement)
		}
	}
}

// reportIndexBoundsError records an ordinary mandatory bounds occurrence at
// the index operand, keyed by the canonical indexed operation for coalescing.
// Rules: rules/collections/collections.md — §8.3 "Bounds";
// rules/mlir/packages/sec-mlir-dialect_package14.md — §35;
// rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing".
func (a *Analyzer) reportIndexBoundsError(expr *ast.IndexExpression, format string, args ...any) {
	before := len(a.errors)
	a.addErrorAtTokenWithMetadata(expressionToken(expr.Index), diagnostics.IndexOutOfBounds, "Use an index greater than or equal to zero and strictly below the live collection length.", format, args...)
	if len(a.errors) > before {
		a.boundsDiagnostics[sourceTokenLocation(expr.Token)] = len(a.errors) - 1
		a.errors[len(a.errors)-1].ProofState = diagnostics.ProofInvalid
	}
}

// attachLengthBoundsDiagnostic preserves one owning error per indexed
// operation. Analysis identities, evidence and suggested edits enrich its help;
// suggestions retain their non-automatic status and suppressed proofs emit no error.
// Rules: rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing",
// "Fix safety"; rules/tooling/diagnostics.md — §§8–10, 14.
func (a *Analyzer) attachLengthBoundsDiagnostic(finding *PitfallFinding) {
	if finding.State != PitfallStateFinding {
		return
	}
	key := sourceTokenLocation(finding.Subject.Source)
	index, exists := a.boundsDiagnostics[key]
	if !exists {
		token := finding.Subject.Source
		before := len(a.errors)
		a.addErrorAtTokenWithMetadata(token, diagnostics.IndexOutOfBounds, "Use an index strictly below Len; Len is the element count, not a valid zero-based index.", "index is out of bounds: %s reaches the collection Len", finding.Subject.Expression)
		if len(a.errors) == before {
			return
		}
		index = len(a.errors) - 1
		a.boundsDiagnostics[key] = index
	}
	a.errors[index].ProofState = diagnostics.ProofInvalid
	a.enrichPitfallDiagnostic(index, finding)
}

// consumeLengthBoundsFacts publishes detached owning proofs into the optional
// result, retaining endpoint uncertainty and diagnostic identity independently
// of optional rule admission and keeping suppressed accesses non-diagnostic.
// Rules: rules/analysis/pitfall_analysis.md — "Analysis states", "Evidence model",
// "Diagnostic ownership and coalescing"; rules/compiler/compiler_analysis.md — §6.
func (b *pitfallBuilder) consumeLengthBoundsFacts(token lexer.Token) {
	key := sourceTokenLocation(token)
	for _, finding := range b.analyzer.boundsFindings[key] {
		if e := b.counts[finding.Rule]; e != nil && e.Incomplete && e.State == PitfallStateNotEvaluated {
			e.State = PitfallStateNoFinding
		}
		b.add(clonePitfallFinding(finding))
	}
	if b.analyzer.boundsIncomplete[key] {
		e := b.counts[PitfallInclusiveLengthIndex]
		e.Incomplete = true
		if e.FindingCount == 0 && e.SuppressedCount == 0 {
			e.State = PitfallStateNotEvaluated
		}
	}
}

// checkConstantListIndexBounds rejects bounds failures provable without
// runtime Len: negative indexes and indexes outside a declared capacity.
// Every other list access retains its mandatory runtime Len check.
//
// Rules:
//   - rules/collections/collections.md — §8.2 "Valid index types" and §8.3 "Bounds"
func (a *Analyzer) checkConstantListIndexBounds(expr *ast.IndexExpression, typ Type) bool {
	index, ok := a.integerConstantValue(expr.Index)
	if !ok {
		return true
	}
	invalid := index.Sign() < 0
	if !invalid && len(typ.ConstArgs) == 1 {
		invalid = index.Cmp(big.NewInt(typ.ConstArgs[0])) >= 0
	}
	if !invalid {
		return true
	}
	a.reportIndexBoundsError(expr, "list index %s is out of bounds for %s", index.String(), typeDisplayName(typ))
	return false
}

// checkConstantIndexBounds enforces the exact I >= 0 && I < N rule from
// SEC-MLIR Package 14 section 35 without narrowing either operand.
// Rules: rules/mlir/packages/sec-mlir-dialect_package14.md — §35.
func (a *Analyzer) checkConstantIndexBounds(expr *ast.IndexExpression, typ Type) bool {
	index, ok := a.integerConstantValue(expr.Index)
	length, fixed := exactFixedArrayLength(typ)
	if !ok || typ.Kind != ArrayType || !fixed {
		return true
	}
	if index.Sign() < 0 || index.Cmp(length) >= 0 {
		a.reportIndexBoundsError(expr, "array index %s is out of bounds for %s", index.String(), typeDisplayName(typ))
		return false
	}
	return true
}
