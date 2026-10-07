package sema

import (
	"fmt"
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// IterationStorageDependency names canonical storage whose structure or
// lifetime supports the loop, including backing references carried by an
// iterator. Unknown origins never become an empty dependency proof.
// Rules: rules/compiler/compiler_analysis.md — §18(2);
// rules/control-flow/flowcontrol_for.md — §§8,12,37.
type IterationStorageDependency struct {
	Place          Place
	CarrierPath    string
	Structural     bool
	MustRemainLive bool
}

// IterationBorrowDependency preserves borrow facts observed while iteration
// is active. Phase distinguishes borrows present at entry from body borrows;
// LoopCarried is the owning borrow solver's fact, not a new lifetime inference.
// OverlapsIteratorStorage is conservative for unknown/ambiguous provenance.
// Rules: rules/compiler/compiler_analysis.md — §18(2);
// rules/memory/borrowing.md — borrow lifetime and aliasing;
// rules/control-flow/flowcontrol_for.md — §§6–8,37.
type IterationBorrowDependency struct {
	LiveAtLoopExit          bool
	Place                   Place
	Holder                  string
	Kind                    string
	Source                  lexer.Token
	Phase                   string
	LoopCarried             bool
	OverlapsIteratorStorage bool
}

// ForIterationDependencies exposes the source/iterator lifetime boundary and
// its borrow/storage dependencies. A temporary lives through body execution
// and is cleaned up at loop exit, including early exits; reusable storage
// remains owned by its declaration. BodyValid is local validation evidence,
// not a lowering-readiness or complete-program proof. BackingUnknown records
// reference provenance gaps; an empty dependency list alone proves nothing.
// Rules: rules/compiler/compiler_analysis.md — §18(2),(6);
// rules/control-flow/flowcontrol_for.md — §§8,12,30–31,37;
// rules/memory/destruction.md — temporary and scope-exit destruction.
type ForIterationDependencies struct {
	Reachable                bool
	Loop                     lexer.Token
	Callable                 CallableID
	SourceType               Type
	Source                   ForIteratorSourceKind
	SourcePlace              Place
	TemporaryDestroyedAtExit bool
	Storage                  []IterationStorageDependency
	BackingUnknown           bool
	Borrows                  []IterationBorrowDependency
	BodyValid                bool
}

type activeIterationDependency struct {
	statement     *ast.ForStatement
	callable      CallableID
	errorsAtEntry int
}

// beginIterationDependencies records facts after iterable and binding
// validation and before body checking, using resolved iteration/Place/origin
// producers. It never probes Next by name or creates an implicit yielded ref.
// Rules: rules/compiler/compiler_analysis.md — §18(2),(3),(6);
// rules/control-flow/flowcontrol_for.md — §§8,12,37.
func (a *Analyzer) beginIterationDependencies(stmt *ast.ForStatement, valid bool) bool {
	if !valid || stmt == nil || stmt.Iterable == nil || a.summaryPass || a.iterationDependencyProbe {
		return false
	}
	fact := ForIterationDependencies{Reachable: a.callGraphPathReachable, Loop: stmt.Token, Callable: a.currentCallable, SourceType: a.expressionTypes[stmt.Iterable], Source: ForIteratorFreshTemporary}
	if plan, ok := a.resolvedForIterations[stmt]; ok {
		fact.SourceType, fact.Source, fact.SourcePlace = plan.SourceType, plan.Source, plan.SourcePlace
		fact.TemporaryDestroyedAtExit = plan.DestroysTemporary
	} else if place, ok := a.resolvePlace(stmt.Iterable); ok && place.Addressable {
		fact.Source, fact.SourcePlace = ForIteratorReusableStorage, place
	}
	category, _, _ := a.forIterationSource(stmt)
	structural := category == forCategorySequential || category == forCategorySet || category == forCategoryMap
	add := func(place Place, path string, structure bool) {
		if place.Root == "" {
			fact.BackingUnknown = true
			return
		}
		fact.Storage = append(fact.Storage, IterationStorageDependency{Place: place, CarrierPath: path, Structural: structure, MustRemainLive: true})
	}
	for _, place := range placeOriginAlternatives(fact.SourcePlace) {
		if place.Root != "" {
			add(place, "", structural)
		}
	}
	// Aggregate reference provenance comes from the shared contained-reference
	// producer. The loop cannot infer that an iterator with untracked backing
	// references is independent of external storage.
	var visit func(localReferenceOrigin, string)
	visit = func(origin localReferenceOrigin, path string) {
		places := localOriginPlaces(origin)
		if origin.Unknown || origin.Ambiguous || (len(places) == 0 && len(origin.Contained) == 0) {
			fact.BackingUnknown = true
		}
		for _, place := range places {
			typ := dereferenceType(place.Type)
			add(place, path, typ.Kind == ArrayType || typ.Kind == SliceType || isForCollectionFamily(typ))
		}
		paths := []string{}
		for child := range origin.Contained {
			paths = append(paths, child)
		}
		sort.Strings(paths)
		for _, child := range paths {
			visit(origin.Contained[child], path+child)
		}
	}
	origins := a.containedReferenceOrigins(stmt.Iterable)
	paths := []string{}
	for path := range origins {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		visit(origins[path], path)
	}
	// Contained origin facts do not claim exhaustive coverage of all reference
	// carriers. Preserve known backing dependencies and independent uncertainty
	// for a reference-carrying protocol state rather than certifying freedom.
	if fact.SourcePlace.AmbiguousProvenance || (typeContainsReference(fact.SourceType, map[string]bool{}) && (category == forCategoryIterator || (len(origins) == 0 && fact.SourcePlace.Root == ""))) {
		fact.BackingUnknown = true
	}
	a.iterationDependencies[stmt] = fact
	a.activeIterationDependencies = append(a.activeIterationDependencies, activeIterationDependency{stmt, a.currentCallable, len(a.errors)})
	a.recordIterationBorrowDependencies("entry")
	// Collection ref bindings are semantic element borrows even when no ordinary
	// RefExpression/borrowRecord was required to create the loop binding.
	for _, binding := range stmt.Bindings {
		if binding.Discard || binding.Mode == ast.ForBindingValue {
			continue
		}
		symbol, ok := a.symbols[binding.Name]
		if !ok || symbol.Type.Kind != ReferenceType {
			continue
		}
		place, ok := a.resolvePlace(&ast.Identifier{Token: binding.Token, Value: binding.Name})
		if !ok {
			continue
		}
		fact := a.iterationDependencies[stmt]
		kind := string(sharedBorrow)
		if binding.Mode == ast.ForBindingRefMut {
			kind = string(mutableBorrow)
		}
		fact.Borrows = append(fact.Borrows, IterationBorrowDependency{Place: place, Holder: binding.Name, Kind: kind, Source: binding.Token, Phase: "iteration-binding", OverlapsIteratorStorage: true})
		a.iterationDependencies[stmt] = fact
	}
	return true
}

// recordIterationBorrowDependencies observes canonical borrow solver facts
// before lexical cleanup can remove a body-local holder. Nested loops update
// each enclosing dependency boundary, while lambda/other callable bodies do
// not become borrow events in the enclosing invocation. Dead paths and
// speculative loop rechecking publish no runtime dependencies.
// Rules: rules/compiler/compiler_analysis.md — §18(2), immutable shared facts;
// rules/control-flow/flowcontrol_for.md — §§8,37;
// rules/analysis/call_graph.md — "Reachability".
func (a *Analyzer) recordIterationBorrowDependencies(phase string) {
	a.recordIterationBorrowState(phase, a.borrows)
}

// recordIterationBorrowState projects the canonical live/backedge borrow state
// into the owning iteration boundary, preserving conditional loop-carried facts.
// Rules: rules/compiler/compiler_analysis.md — §18(2); rules/control-flow/flowcontrol_for.md — §§6–8,37.
func (a *Analyzer) recordIterationBorrowState(phase string, borrows map[string][]borrowRecord) {
	if a.summaryPass || a.iterationDependencyProbe || !a.callGraphPathReachable {
		return
	}
	for index, active := range a.activeIterationDependencies {
		if (phase == "backedge" || phase == "exit") && index != len(a.activeIterationDependencies)-1 {
			continue
		}
		if active.callable != a.currentCallable {
			continue
		}
		fact := a.iterationDependencies[active.statement]
		roots := []string{}
		for root := range borrows {
			roots = append(roots, root)
		}
		sort.Strings(roots)
		for _, root := range roots {
			for _, record := range borrows[root] {
				candidate := IterationBorrowDependency{LiveAtLoopExit: phase == "exit", Place: record.Place, Holder: record.Holder, Kind: string(record.Kind), Source: record.Token, Phase: phase, LoopCarried: phase == "backedge" && record.LoopCarried, OverlapsIteratorStorage: fact.BackingUnknown}
				if record.Place.Root == "" {
					candidate.OverlapsIteratorStorage = true
				}
				for _, dependency := range fact.Storage {
					candidate.OverlapsIteratorStorage = candidate.OverlapsIteratorStorage || borrowRecordMayOverlap(dependency.Place, record)
				}
				exists := false
				for i, old := range fact.Borrows {
					if old.Holder == candidate.Holder && old.Source == candidate.Source && old.Kind == candidate.Kind && Relationship(old.Place, candidate.Place) == PlaceSame {
						fact.Borrows[i].LoopCarried = old.LoopCarried || candidate.LoopCarried
						fact.Borrows[i].LiveAtLoopExit = old.LiveAtLoopExit || candidate.LiveAtLoopExit
						fact.Borrows[i].OverlapsIteratorStorage = old.OverlapsIteratorStorage || candidate.OverlapsIteratorStorage
						exists = true
						break
					}
				}
				if !exists {
					fact.Borrows = append(fact.Borrows, candidate)
				}
			}
		}
		a.iterationDependencies[active.statement] = fact
	}
}

// finishIterationDependencies closes the lexical observation boundary. Failed
// body checking retains source dependencies for diagnostics without releasing
// a positive validation result. Speculative rechecks never replace facts.
// Rules: rules/compiler/compiler_analysis.md — §18(2); rules/control-flow/flowcontrol_for.md — §§30–31,37.
func (a *Analyzer) finishIterationDependencies(stmt *ast.ForStatement) {
	a.recordIterationBorrowState("exit", a.borrows)
	last := len(a.activeIterationDependencies) - 1
	active := a.activeIterationDependencies[last]
	fact := a.iterationDependencies[stmt]
	fact.BodyValid = len(a.errors) == active.errorsAtEntry
	sort.SliceStable(fact.Borrows, func(i, j int) bool {
		left, right := fact.Borrows[i], fact.Borrows[j]
		return fmt.Sprintf("%s:%09d:%09d:%s:%s:%s", left.Source.File, left.Source.Line, left.Source.Column, left.Holder, left.Kind, left.Place.String()) < fmt.Sprintf("%s:%09d:%09d:%s:%s:%s", right.Source.File, right.Source.Line, right.Source.Column, right.Holder, right.Kind, right.Place.String())
	})
	a.iterationDependencies[stmt] = fact
	a.activeIterationDependencies = a.activeIterationDependencies[:last]
}

// ForIterationDependenciesOf returns a detached snapshot of resolved source
// lifetime and observed borrow dependencies; unknown backing is explicit.
// Rules: rules/compiler/compiler_analysis.md — §18(2),(6), immutable analysis facts.
func (a *Analyzer) ForIterationDependenciesOf(stmt *ast.ForStatement) (ForIterationDependencies, bool) {
	if a == nil || stmt == nil {
		return ForIterationDependencies{}, false
	}
	fact, ok := a.iterationDependencies[stmt]
	fact.SourceType = semanticSnapshotType(fact.SourceType)
	fact.SourcePlace = iterationSnapshotPlace(fact.SourcePlace)
	fact.Storage = append([]IterationStorageDependency(nil), fact.Storage...)
	for i := range fact.Storage {
		fact.Storage[i].Place = iterationSnapshotPlace(fact.Storage[i].Place)
	}
	fact.Borrows = append([]IterationBorrowDependency(nil), fact.Borrows...)
	for i := range fact.Borrows {
		fact.Borrows[i].Place = iterationSnapshotPlace(fact.Borrows[i].Place)
	}
	return fact, ok
}

// iterationSnapshotPlace detaches both canonical projection/origin facts and
// nested type metadata so consumers cannot mutate the analyzer's borrow state.
// Rules: rules/compiler/compiler_analysis.md — immutable analysis facts.
func iterationSnapshotPlace(place Place) Place {
	place = clonePlace(place)
	place.Type = semanticSnapshotType(place.Type)
	for i := range place.AlternativeOrigins {
		place.AlternativeOrigins[i] = iterationSnapshotPlace(place.AlternativeOrigins[i])
	}
	return place
}
