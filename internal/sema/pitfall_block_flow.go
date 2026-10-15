package sema

import "sec/internal/ast"

// walkBlock preserves owning reachability for bounds proofs while retaining
// contextual guard and intent witnesses for optional pitfall findings.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Evidence model",
// "Guards participate in pitfall reasoning".
func (b *pitfallBuilder) walkBlock(block *ast.BlockStatement) {
	if block == nil {
		return
	}
	if b.boundsOnly {
		for _, statement := range block.Statements {
			b.walkStatement(statement)
			if !b.analyzer.statementCanFallThrough(statement) {
				break
			}
		}
		return
	}
	b.inspectIneffectiveRejectionGuards(block)
	b.inspectCheckWithoutTransfer(block)
	inheritedProofs := b.activeNonEmptyProofs
	inheritedPreceding := b.activePreceding
	inheritedGuards := b.activeIndexGuards
	inheritedCapacity := b.activeCapacityEqualities
	liveCapacity := mergeCapacityEqualities(inheritedCapacity, nil)
	liveNonEmpty := clonePitfallNonEmptyProofs(inheritedProofs)
	for index, statement := range block.Statements {
		b.activeCapacityEqualities = b.capacityEntryProofs(liveCapacity, statement)
		entryProofs := b.survivingNonEmptyProofs(liveNonEmpty, statement)
		b.activeNonEmptyProofs = entryProofs
		if loop, ok := statement.(*ast.ForStatement); ok {
			b.inspectSkippedFirstElement(loop, block.Statements[:index])
			b.inspectCapacityAsLength(loop)
			b.inspectWrongBoundSource(loop, block.Statements[:index])
			b.inspectLengthEndpointIntent(loop, block.Statements, index, entryProofs)
		}
		b.activePreceding = block.Statements[:index]
		b.walkStatement(statement)
		liveCapacity = b.survivingCapacityEqualities(liveCapacity, statement)
		liveCapacity = mergeCapacityEqualities(liveCapacity, b.capacityContinuationProof(statement))
		if assertion, ok := statement.(*ast.AssertStatement); ok {
			liveCapacity = mergeCapacityEqualities(liveCapacity, b.capacityBranchProof(assertion.Condition, true))
		}
		liveNonEmpty = entryProofs
		liveNonEmpty = mergePitfallNonEmptyProofs(liveNonEmpty, b.continuationNonEmptyProofs(statement))
		if assertion, ok := statement.(*ast.AssertStatement); ok {
			liveNonEmpty = mergePitfallNonEmptyProofs(liveNonEmpty, b.nonEmptyBranchProof(assertion.Condition, true))
		}
		b.activeIndexGuards = b.survivingIndexGuards(b.activeIndexGuards, statement)
		b.activeIndexGuards = append(b.activeIndexGuards, b.continuationIndexGuards(statement)...)
		if !b.analyzer.statementCanFallThrough(statement) {
			break
		}
	}
	b.activeIndexGuards = inheritedGuards
	b.activeNonEmptyProofs = inheritedProofs
	b.activePreceding = inheritedPreceding
	b.activeCapacityEqualities = inheritedCapacity
}
