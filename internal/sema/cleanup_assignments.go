package sema

import (
	"sort"
	"strings"

	"sec/internal/ast"
)

// CleanupPreviousValue records the destination availability already resolved
// by Sema after RHS preparation. Conditional/partial states retain their own
// cleanup planning boundary; neither may be treated as a complete old value.
// Rules: rules/memory/copy_move.md — §§12.1–12.3;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§31–33,59.
type CleanupPreviousValue string

const (
	CleanupPreviousAvailable   CleanupPreviousValue = "available"
	CleanupPreviousUnavailable CleanupPreviousValue = "unavailable"
	CleanupPreviousConditional CleanupPreviousValue = "conditional"
	CleanupPreviousPartial     CleanupPreviousValue = "partial"
)

// ResolvedOwnedCleanupAssignment preserves success-only replacement/reinit.
// Available requires old-value destruction/cancellation; Unavailable requires
// no old destruction. Conditional/Partial require their path/subobject plans.
// The fresh registration belongs to the original binding's lexical lifetime,
// even when the assignment executes in an inner block or a deferred body.
// Rules: rules/memory/destruction.md — §6.1(2–3), §13;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§31–33,38,55,79–80.
type ResolvedOwnedCleanupAssignment struct {
	Previous     CleanupPreviousValue
	Registration ResolvedOwnedCleanupRegistration
	Executor     CallableID
	// Moved lists, for a Partial previous value, the stored-field paths that
	// moved out, by field name in canonical order; a moved active payload counts
	// as its whole union field. MovedConditional reports one that moved on only
	// some paths.
	Moved            [][]string
	MovedConditional bool
}

// prepareOwnedCleanupAssignment snapshots accepted destination facts after RHS
// checking and before assignment clears moved/partial state. This producer
// currently covers whole local bindings with a declaration registration;
// parameters, projected stores and temporary ownership remain separate owners.
// Rules: rules/memory/copy_move.md — §12.1(2–3), §12.2, §12.3;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§31–33,38.
func (a *Analyzer) prepareOwnedCleanupAssignment(stmt *ast.AssignmentStatement, errorsBefore int) *ResolvedOwnedCleanupAssignment {
	if len(a.errors) != errorsBefore || stmt.Operator != "=" {
		return nil
	}
	target, ok := stmt.Target.(*ast.Identifier)
	if !ok {
		return nil
	}
	place, ok := a.rootPlace(target.Value)
	if !ok || TriviallyDestructible(place.Type) {
		return nil
	}
	binding := a.bindingIDs[sourceTokenLocation(place.RootToken)]
	declaration := a.ownedCleanupDeclarations[binding]
	initial, ok := a.ownedCleanupInitializations[declaration]
	if !ok {
		return nil
	}
	previous := CleanupPreviousAvailable
	if _, key, partial, unavailable := a.unavailablePlace(place); unavailable {
		switch {
		case partial && strings.HasPrefix(key, place.String()+".<"):
			// A moved active payload was all the union held, so only the path
			// that moved it differs (destruction.md § 13.3).
			previous = CleanupPreviousConditional
		case partial:
			previous = CleanupPreviousPartial
		case isConditionalAvailabilityReason(a.moveReasons[key]):
			previous = CleanupPreviousConditional
		default:
			previous = CleanupPreviousUnavailable
		}
	}
	initial.Action = a.cleanupIdentities.owned(binding, stmt.Token)
	initial.Place = cloneSlicePlanPlace(place)
	plan := &ResolvedOwnedCleanupAssignment{Previous: previous, Registration: initial, Executor: a.currentCallable}
	if previous == CleanupPreviousPartial {
		plan.Moved, plan.MovedConditional = a.movedFieldPaths(place)
	}
	return plan
}

// movedFieldPaths lists the stored-field paths moved out below place. A path
// through an active payload ends at its union field, which then holds nothing.
func (a *Analyzer) movedFieldPaths(place Place) ([][]string, bool) {
	prefix := place.String() + "."
	seen := map[string]bool{}
	var paths [][]string
	conditional := false
	for key := range a.moved {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		conditional = conditional || isConditionalAvailabilityReason(a.moveReasons[key])
		var path []string
		for _, name := range strings.Split(key[len(prefix):], ".") {
			if strings.HasPrefix(name, "<") || strings.Contains(name, "[") {
				break
			}
			path = append(path, name)
		}
		if len(path) == 0 || seen[strings.Join(path, ".")] {
			continue
		}
		seen[strings.Join(path, ".")] = true
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool { return strings.Join(paths[i], ".") < strings.Join(paths[j], ".") })
	return paths, conditional
}

// recordOwnedCleanupAssignment publishes only after assignment commit has
// succeeded. Earlier fallible RHS exits preserve the previous responsibility.
// Rules: rules/memory/copy_move.md — §12.1(2–3);
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§31–33,38.
func (a *Analyzer) recordOwnedCleanupAssignment(stmt *ast.AssignmentStatement, plan *ResolvedOwnedCleanupAssignment, errorsBefore int) {
	if plan == nil || len(a.errors) != errorsBefore {
		return
	}
	if a.ownedCleanupAssignments == nil {
		a.ownedCleanupAssignments = map[*ast.AssignmentStatement]ResolvedOwnedCleanupAssignment{}
	}
	a.ownedCleanupAssignments[stmt] = *plan
}

// ResolvedOwnedCleanupAssignmentOf returns a detached replacement/reinit fact
// without repeating ownership inference or changing the destination state.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§31–33,79–80.
func (a *Analyzer) ResolvedOwnedCleanupAssignmentOf(stmt *ast.AssignmentStatement) (ResolvedOwnedCleanupAssignment, bool) {
	if a == nil || stmt == nil {
		return ResolvedOwnedCleanupAssignment{}, false
	}
	plan, ok := a.ownedCleanupAssignments[stmt]
	if !ok {
		return ResolvedOwnedCleanupAssignment{}, false
	}
	plan.Registration = cloneOwnedCleanupRegistration(plan.Registration)
	return plan, true
}

// recordOwnedFieldAssignment retains what a stored field of an owned local holds
// before `p.f = v` commits: a value to replace, nothing (reinitialization), or a
// value on some paths only. The owner's registration covers the field throughout
// (package17 §§ 13, 64; destruction.md § 13).
func (a *Analyzer) recordOwnedFieldAssignment(stmt *ast.AssignmentStatement, place Place) {
	if len(place.Projections) == 0 {
		return
	}
	for _, projection := range place.Projections {
		if projection.Kind != PlaceField {
			return
		}
	}
	root, ok := a.rootPlace(place.Root)
	if !ok || TriviallyDestructible(root.Type) {
		return
	}
	if _, declared := a.ownedCleanupDeclarations[a.bindingIDs[sourceTokenLocation(root.RootToken)]]; !declared {
		return
	}
	previous := CleanupPreviousAvailable
	if _, key, partial, unavailable := a.unavailablePlace(place); unavailable {
		switch {
		case partial && strings.HasPrefix(key, place.String()+".<"):
			// A moved active payload was all the union field held.
			previous = CleanupPreviousConditional
		case partial:
			previous = CleanupPreviousPartial
		case isConditionalAvailabilityReason(a.moveReasons[key]):
			previous = CleanupPreviousConditional
		default:
			previous = CleanupPreviousUnavailable
		}
	}
	if a.ownedFieldAssignments == nil {
		a.ownedFieldAssignments = map[*ast.AssignmentStatement]CleanupPreviousValue{}
	}
	a.ownedFieldAssignments[stmt] = previous
}

// ResolvedOwnedFieldAssignmentOf reads what an assigned stored field of an owned
// local held, without repeating ownership inference.
func (a *Analyzer) ResolvedOwnedFieldAssignmentOf(stmt *ast.AssignmentStatement) (CleanupPreviousValue, bool) {
	if a == nil {
		return "", false
	}
	previous, ok := a.ownedFieldAssignments[stmt]
	return previous, ok
}
