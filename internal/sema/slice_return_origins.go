package sema

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/lexer"
	"sort"
)

// composeReturnedOriginPlace substitutes a callee's source-relative projections
// through actual referent storage. Its leading parameter dereference is already
// discharged by actual-origin binding. Runtime IDs are call-site-local facts,
// never proof that separate calls evaluate the same range.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§26–30,66–69;
// rules/memory/borrowing.md — §§12,14.3–14.4.
func (a *Analyzer) composeReturnedOriginPlace(base Place, projections []PlaceProjection, site *ast.CallExpression, path string) Place {
	for index, original := range projections {
		p := clonePlaceProjection(original)
		if index == 0 && p.Kind == PlaceDereference {
			base.Type = dereferenceType(base.Type)
			continue
		}
		if p.Kind == PlaceSlice {
			if p.SliceEndOpen && p.SliceEnd == nil {
				length := staticSliceExtent(base)
				if length == nil && (len(base.Projections) == 0 || base.Projections[len(base.Projections)-1].Kind != PlaceSlice) {
					length, _ = exactFixedArrayLength(dereferenceType(base.Type))
				}
				p.SliceEnd = length
			}
			if p.SliceStart != nil && p.SliceEnd != nil {
				p.DynamicRange = 0
			} else if p.DynamicRange != 0 && site != nil {
				loc := sourceTokenLocation(site.Token)
				p.DynamicRange = a.placeIdentities.sliceRange(fmt.Sprintf("return-slice:%s:%d:%d:%s:%d", loc.File, loc.Line, loc.Column, path, index))
			}
			base.Projections = appendSliceProjection(base.Projections, p)
			source := dereferenceType(base.Type)
			if source.Element != nil {
				base.Type = Type{Kind: SliceType, Element: source.Element}
			}
		} else if p.Kind == PlaceIndex {
			base.Projections = appendIndexProjection(base.Projections, p)
			source := dereferenceType(base.Type)
			if source.Element != nil {
				base.Type = *source.Element
			}
		} else {
			if p.Kind == PlaceField {
				if field, ok := lookupStructField(dereferenceType(base.Type), p.Name); ok {
					base.Type = field
				}
			}
			if p.Kind == PlaceDereference {
				base.Type = dereferenceType(base.Type)
			}
			base = appendPlaceProjection(base, p)
		}
	}
	return base
}

// checkBorrowThroughTransferredParameter checks the current holder when a
// reference parameter's reference value was moved but its referent survived.
// The old parameter holder's availability is not the referent's lifetime.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§40–43,63–69;
// rules/memory/borrowing.md — §§6,14.4; rules/memory/copy_move.md — §4.4.
func (a *Analyzer) checkBorrowThroughTransferredParameter(place Place, token lexer.Token) (handled, invalid bool) {
	if place.ReferenceHolder == "" || place.ReferenceHolder == place.Root {
		return false, false
	}
	root, ok := a.symbols[place.Root]
	if !ok || root.Type.Kind != ReferenceType {
		return false, false
	}
	origin, ok := a.localRefContainers[place.ReferenceHolder]
	if !ok || origin.Unknown {
		return false, false
	}
	for _, source := range localOriginPlaces(origin) {
		if source.Root == place.Root && source.RootID == place.RootID {
			holder, ok := a.rootPlace(place.ReferenceHolder)
			if ok {
				return true, a.checkPlaceAvailableForBorrow(holder, token)
			}
		}
	}
	return false, false
}

// returnedContainedOrigin selects the referenced aggregate field/element from
// its existing contained-origin map and leaves only referent-relative suffixes.
// Wildcard elements join all possible origins rather than picking one field.
// Rules: rules/memory/borrowing.md — §§12–14; rules/memory/lifetime_analysis.md — §28;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§66–67.
func returnedContainedOrigin(origin localReferenceOrigin, projections []PlaceProjection) (localReferenceOrigin, []PlaceProjection, bool) {
	path := ""
	selected := localReferenceOrigin{}
	consumed := 0
	for index, p := range projections {
		switch p.Kind {
		case PlaceField:
			path += "." + p.Name
		case PlaceIndex:
			if p.DynamicIndex {
				path += "[*]"
			} else {
				path += "[" + placeConstantIndexValue(p.ConstantIndex).String() + "]"
			}
		case PlaceDereference:
			continue
		default:
			return selected, projections[consumed:], consumed > 0
		}
		keys := make([]string, 0, len(origin.Contained))
		for key := range origin.Contained {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var matches []localReferenceOrigin
		for _, key := range keys {
			if containedPathMatches(path, key) {
				matches = append(matches, origin.Contained[key])
			}
		}
		if len(matches) > 0 {
			selected = mergeLocalReferenceOrigins(matches, false)
			consumed = index + 1
		}
	}
	return selected, projections[consumed:], consumed > 0
}
