package sema

// cloneCaptureRecords recursively detaches Place, type, and dependency data
// before capture facts cross the Analyzer snapshot boundary.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Capture record"
//   - rules/compiler/compiler_analysis.md — immutable analysis results
func cloneCaptureRecords(records []CaptureRecord) []CaptureRecord {
	cloned := make([]CaptureRecord, len(records))
	for index, record := range records {
		cloned[index] = record
		cloned[index].SourcePlace = closureSnapshotPlace(record.SourcePlace)
		cloned[index].CapturedType = semanticSnapshotType(record.CapturedType)
		cloned[index].Dependencies.ReferentPlaces = closureSnapshotPlaces(record.Dependencies.ReferentPlaces)
		cloned[index].Dependencies.Referents = cloneCaptureReferentDependencies(record.Dependencies.Referents)
		cloned[index].Dependencies.CallableTargets = cloneCallableTargetSet(record.Dependencies.CallableTargets)
	}
	return cloned
}

// cloneCaptureReferentDependencies detaches nested Place alternatives before
// capture dependencies cross the Analyzer snapshot boundary.
//
// Rules:
//   - rules/compiler/compiler_analysis.md — immutable analysis results
func cloneCaptureReferentDependencies(dependencies []CaptureReferentDependency) []CaptureReferentDependency {
	cloned := make([]CaptureReferentDependency, len(dependencies))
	for index, dependency := range dependencies {
		cloned[index] = dependency
		cloned[index].ReferentPlaces = closureSnapshotPlaces(dependency.ReferentPlaces)
	}
	return cloned
}

// closureSnapshotPlace detaches structural type data as well as projections
// and every alternative origin at the closure-analysis publication boundary.
// The ordinary internal Place copy deliberately retains types for Sema; it is
// insufficient when publishing facts to consumers that can mutate Go values.
// Rules:
//   - rules/analysis/closure_analysis.md — "SourcePlace", "Dependencies"
//   - rules/compiler/compiler_analysis.md — §55 "Analysis snapshot consistency"
func closureSnapshotPlace(place Place) Place {
	place.Type = semanticSnapshotType(place.Type)
	place.Projections = clonePlaceProjections(place.Projections)
	place.AlternativeOrigins = closureSnapshotPlaces(place.AlternativeOrigins)
	return place
}

// closureSnapshotPlaces snapshots a finite set of canonical referent Places
// without dropping alternative origins or sharing their mutable type internals.
// Rules:
//   - rules/analysis/closure_analysis.md — "Dependencies", "Closure environment as dependency carrier"
//   - rules/compiler/compiler_analysis.md — §55 "Analysis snapshot consistency"
func closureSnapshotPlaces(places []Place) []Place {
	if places == nil {
		return nil
	}
	cloned := make([]Place, len(places))
	for index, place := range places {
		cloned[index] = closureSnapshotPlace(place)
	}
	return cloned
}
