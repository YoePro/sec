package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// validatePreparedSpreadBorrow checks each source read once against earlier
// call reservations. Prepared copies do not borrow their array, but reading
// that array must not overlap an already prepared exclusive argument. A slot
// needing element borrow/contained-reference provenance cannot reuse the
// whole array/pack as its origin: that absent implementation is S1239 rather
// than a fabricated lifetime proof or an ordinary source-validity rejection.
// Rules: rules/declarations/spread.md §§3,4.2–4.4;
// rules/memory/borrowing.md §15.4(1–5);
// rules/memory/references.md §9(10); rules/compiler/compiler_pipeline.md §32(2–4).
func (a *Analyzer) validatePreparedSpreadBorrow(plan *ResolvedCallArgumentPlan, index int, parameter FunctionParameter, reservations []borrowRecord, read, blocked map[int]bool, at lexer.Token) bool {
	if plan == nil || index >= len(plan.Expanded) || plan.Expanded[index].Source < 0 || plan.Expanded[index].Source >= len(plan.Sources) {
		a.reportCapabilityUnavailable(at, "Pass the arguments separately until their spread preparation facts are available.", "spread argument has no resolved per-element preparation")
		return false
	}
	sourceIndex := plan.Expanded[index].Source
	source := plan.Sources[sourceIndex].Expression
	if blocked[sourceIndex] {
		return false
	}
	container, known := a.ResolvedTypeOf(source)
	container = dereferenceType(container)
	_, borrowed := callParameterBorrowMode(parameter)
	if !known || container.Element == nil || borrowed || spreadElementNeedsProvenance(*container.Element, map[string]bool{}) || len(a.referenceContentsForBinding(source)) != 0 {
		blocked[sourceIndex] = true
		a.reportCapabilityUnavailable(at,
			"Pass the indexed arguments explicitly so each borrow and returned or retained reference has its own proven origin.",
			"spread argument requires per-element borrow or reference provenance that is not implemented yet")
		return false
	}
	if read[sourceIndex] {
		return true
	}
	read[sourceIndex] = true
	return a.validatePreparedSpreadSourceRead(source, reservations, at)
}

// validatePreparedSpreadSourceRead checks source evaluation even when a spread
// supplies no expanded slots. Computed sources require explicit access facts.
// Rules: rules/declarations/spread.md §§3,4.1;
// rules/memory/borrowing.md §15.4; rules/compiler/compiler_pipeline.md §32.
func (a *Analyzer) validatePreparedSpreadSourceRead(source ast.Expression, reservations []borrowRecord, at lexer.Token) bool {
	// Existing references name the referent, not the local holder. The same
	// canonical query handles owned values and flow-joined source origins.
	place, reusable := a.callArgumentBorrowPlace(source)
	if !reusable {
		if len(reservations) != 0 {
			a.reportCapabilityUnavailable(at,
				"Prepare the spread source in a named value before preparing the borrowed arguments, so its accesses can be checked separately.",
				"spread source evaluation with earlier argument borrows requires access-effect provenance that is not implemented yet")
			return false
		}
		return true
	}
	for _, reservation := range reservations {
		if reservation.Kind != mutableBorrow || !borrowRecordMayOverlap(place, reservation) {
			continue
		}
		a.addBorrowAuthorityConflict(place, at, reservation, diagnostics.BorrowOwnerReadConflict,
			"spread argument reads %s while mutable borrow prepared for argument %d remains active",
			place.String(), callReservationArgumentIndex(reservation))
		return false
	}
	return true
}

// spreadElementNeedsProvenance identifies stored references and callable
// environments whose dependency cannot be represented by an array-wide origin.
// It is a conservative capability check, never a separate lifetime proof.
// Rules: rules/declarations/spread.md §4.2; lambda-functions.md §§20,27;
// rules/memory/lifetime_analysis.md §28; compiler_pipeline.md §32.
func spreadElementNeedsProvenance(typ Type, visiting map[string]bool) bool {
	if typ.Kind == FunctionType || typeContainsReference(typ, map[string]bool{}) {
		return true
	}
	key := canonicalTypeIdentity(typ)
	if visiting[key] {
		return false
	}
	visiting[key] = true
	defer delete(visiting, key)
	if typ.Element != nil && spreadElementNeedsProvenance(*typ.Element, visiting) {
		return true
	}
	for _, field := range typ.Fields {
		if spreadElementNeedsProvenance(field.Type, visiting) {
			return true
		}
	}
	for _, argument := range typ.TypeArgs {
		if spreadElementNeedsProvenance(argument, visiting) {
			return true
		}
	}
	for _, variant := range typ.UnionVariants {
		if variant.Payload != nil && spreadElementNeedsProvenance(*variant.Payload, visiting) {
			return true
		}
		for _, field := range variant.PayloadFields {
			if spreadElementNeedsProvenance(field.Type, visiting) {
				return true
			}
		}
	}
	return false
}
