package sema

import (
	"fmt"
	"strings"
)

// borrowOverlapExplanation describes the compiler-owned relationship between
// the requested Place and the retained borrow. Unknown/dynamic/alternative
// provenance never becomes a claim that one exact location is proven.
// Rules: rules/memory/borrowing.md — §§12(1–8),14.2–14.3,32(2–3),33(1).
func borrowOverlapExplanation(place Place, record borrowRecord) string {
	borrowed := record.Place
	if borrowed.Root == "" {
		borrowed = Place{Root: record.Root, AmbiguousProvenance: true}
	}
	kind := string(record.Kind) + " borrow"
	if isDeferredUseKind(record.Kind) {
		kind = "deferred use"
	}
	accessName, borrowedName := borrowDiagnosticPlaceName(place), borrowDiagnosticPlaceName(borrowed)
	switch Relationship(place, borrowed) {
	case PlaceSame:
		return fmt.Sprintf("%s and the %s of %s refer to the same storage.", accessName, kind, borrowedName)
	case PlaceContains:
		return fmt.Sprintf("The requested whole Place %s contains %s, whose storage is retained by a %s.", accessName, borrowedName, kind)
	case PlaceContainedBy:
		return fmt.Sprintf("The %s of %s also covers the requested sub-Place %s because it contains that storage.", kind, borrowedName, accessName)
	default:
		return fmt.Sprintf("%s and the %s of %s may overlap; their index, range, property or provenance facts do not prove disjoint storage.", accessName, kind, borrowedName)
	}
}

// borrowDiagnosticPlaceName lists represented finite alternatives rather than
// presenting the primary origin as the only possible referent.
// Rules: rules/memory/borrowing.md — §12(6–8),§32(2).
func borrowDiagnosticPlaceName(place Place) string {
	if place.Root == "" && len(place.AlternativeOrigins) == 0 {
		return "unknown storage"
	}
	alternatives := placeOriginAlternatives(place)
	if len(alternatives) == 1 {
		return place.String()
	}
	names := make([]string, 0, len(alternatives))
	for _, alternative := range alternatives {
		names = append(names, alternative.String())
	}
	return "possible Places {" + strings.Join(names, ", ") + "}"
}
