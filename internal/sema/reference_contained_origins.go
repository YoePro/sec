package sema

import (
	"sec/internal/ast"
	"sort"
	"strings"
)

// containedOriginForAccess reads field-sensitive reference contents. A formal
// aggregate's untouched reference field denotes caller-provided referent storage,
// not the lifetime of the callee's by-value aggregate holder.
// Rules: rules/memory/borrowing.md — §§12–14; rules/memory/lifetime_analysis.md — §28;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§66–67.
func (a *Analyzer) containedOriginForAccess(expr ast.Expression) (localReferenceOrigin, bool) {
	root, path, ok := a.aggregateAccessPath(expr)
	if !ok || path == "" {
		return localReferenceOrigin{}, false
	}
	container, ok := a.localRefContainers[root]
	if !ok || len(container.Contained) == 0 {
		return a.parameterContainedReferenceOrigin(expr, root)
	}
	origins := make([]localReferenceOrigin, 0, 1)
	keys := make([]string, 0, len(container.Contained))
	for key := range container.Contained {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, candidate := range keys {
		if containedPathMatches(path, candidate) {
			origins = append(origins, container.Contained[candidate])
		}
	}
	if len(origins) == 0 {
		return a.parameterContainedReferenceOrigin(expr, root)
	}
	return mergeLocalReferenceOrigins(origins, false), true
}

// parameterContainedReferenceOrigin records a symbolic caller-owned reference
// field only for an actual formal parameter. No local aggregate or ordinary
// owned field gains escape permission from this fallback.
// Rules: rules/memory/lifetime_analysis.md — §28; rules/memory/borrowing.md — §§12–14;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§66–67.
func (a *Analyzer) parameterContainedReferenceOrigin(expr ast.Expression, root string) (localReferenceOrigin, bool) {
	symbol, ok := a.symbols[root]
	if !ok || !symbol.Parameter {
		return localReferenceOrigin{}, false
	}
	place, ok := a.resolvePlace(expr)
	if !ok || !typeCarriesReferenceOrigin(place.Type) {
		return localReferenceOrigin{}, false
	}
	return localOriginWithPlaces(localReferenceOrigin{Name: root, Token: symbol.Token, Mutable: place.Type.ReferenceMutable}, []Place{place}), true
}

// referenceContentsForBinding reads existing aggregate contents through value
// and reference arguments/receivers. A single known stored subobject can be
// rebased; unknown or multi-origin containers remain without a content proof.
// Rules: rules/memory/borrowing.md — §§12–14; rules/memory/lifetime_analysis.md — §28;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§66–69.
func (a *Analyzer) referenceContentsForBinding(expr ast.Expression) map[string]localReferenceOrigin {
	if ref, ok := expr.(*ast.RefExpression); ok {
		return a.referenceContentsForBinding(ref.Value)
	}
	if prefix, ok := expr.(*ast.PrefixExpression); ok && prefix.Operator == "<-" {
		return a.referenceContentsForBinding(prefix.Right)
	}
	if contents := a.containedReferenceOrigins(expr); len(contents) > 0 {
		return contents
	}
	place, ok := a.referencePlaceOrigin(expr)
	if !ok || place.AmbiguousProvenance || len(place.AlternativeOrigins) > 0 {
		return nil
	}
	container, ok := a.localRefContainers[place.Root]
	if !ok || container.Unknown || len(container.Contained) == 0 {
		return nil
	}
	path := ""
	for _, p := range place.Projections {
		switch p.Kind {
		case PlaceField:
			path += "." + p.Name
		case PlaceIndex:
			if p.DynamicIndex {
				return nil
			}
			path += "[" + placeConstantIndexValue(p.ConstantIndex).String() + "]"
		case PlaceDereference:
		default:
			return nil
		}
	}
	result := map[string]localReferenceOrigin{}
	for key, child := range container.Contained {
		if strings.HasPrefix(key, path) {
			suffix := strings.TrimPrefix(key, path)
			if strings.HasPrefix(suffix, ".") || strings.HasPrefix(suffix, "[") {
				result[suffix] = cloneLocalReferenceOrigin(child)
			}
		}
	}
	return result
}
