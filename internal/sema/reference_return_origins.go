package sema

import (
	"sec/internal/ast"
	"sort"
	"strconv"
	"strings"
)

// symbolizeFunctionReturnOrigin retains caller-relative Place/range facts for
// direct and contained returned references, rejecting unavailable local roots.
// Rules: rules/memory/lifetime_analysis.md — §28; rules/mlir/packages/sec-mlir-dialect_package16.md — §§66–67.
func (a *Analyzer) symbolizeFunctionReturnOrigin(origin localReferenceOrigin) localReferenceOrigin {
	origin = cloneLocalReferenceOrigin(origin)
	places := localOriginPlaces(origin)
	symbolic := make([]Place, 0, len(places))
	unknown := origin.Unknown
	for _, place := range places {
		root, ok := a.symbolicFunctionOriginRoot(place.Root)
		if !ok {
			unknown = true
			continue
		}
		place.Root = root
		symbolic = append(symbolic, place)
	}
	origin = localOriginWithPlaces(origin, symbolic)
	origin.Local = false
	origin.MatchScoped = false
	origin.Unknown = origin.Unknown || unknown
	origin.Ambiguous = origin.Ambiguous || origin.Unknown
	for path, child := range origin.Contained {
		origin.Contained[path] = a.symbolizeFunctionReturnOrigin(child)
		if origin.Contained[path].Unknown {
			origin.Unknown = true
			origin.Ambiguous = true
		}
	}
	return origin
}

// symbolicFunctionOriginRoot binds a source root to its formal parameter,
// receiver or static-storage role; local roots never gain return permission.
// Rules: rules/memory/lifetime_analysis.md — §28; rules/mlir/packages/sec-mlir-dialect_package16.md — §§66–67.
func (a *Analyzer) symbolicFunctionOriginRoot(root string) (string, bool) {
	function := a.currentFunctionMetadata
	for index, param := range function.Parameters {
		if param.Name == root {
			return "$param:" + strconv.Itoa(index), true
		}
	}
	if root == "self" && function.ImplTarget != "" {
		return "$receiver", true
	}
	if symbol, exists := a.symbols[root]; exists && !symbol.Local {
		return "$static:" + root, true
	}
	return "", false
}

// instantiateFunctionReturnOrigin binds returned origins to this call's actual
// storage, preserving exact static composition and conservative dynamic ranges.
// Rules: rules/memory/lifetime_analysis.md — §28; rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–43,66–69.
func (a *Analyzer) instantiateFunctionReturnOrigin(summary localReferenceOrigin, args []ast.Expression, receiver ast.Expression, site *ast.CallExpression) localReferenceOrigin {
	return a.instantiateReturnOriginAt(summary, args, receiver, site, "")
}

// instantiateReturnOriginAt also binds contained origins, using the result path
// to distinguish independent runtime ranges from different aggregate fields.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§26–28,66–67.
func (a *Analyzer) instantiateReturnOriginAt(summary localReferenceOrigin, args []ast.Expression, receiver ast.Expression, site *ast.CallExpression, path string) localReferenceOrigin {
	summary = cloneLocalReferenceOrigin(summary)
	places := localOriginPlaces(summary)
	instantiatedPlaces := []Place{}
	actualOrigins := []localReferenceOrigin{}
	unknown := summary.Unknown
	for _, symbolic := range places {
		actual, ok := a.actualOriginForSummaryRoot(symbolic.Root, args, receiver)
		if !ok || actual.Unknown {
			unknown = true
			continue
		}
		projections := symbolic.Projections
		if len(actual.Contained) > 0 {
			selectedOrigin, suffix, selected := returnedContainedOrigin(actual, projections)
			if selected {
				actual, projections = selectedOrigin, suffix
				if actual.Unknown {
					unknown = true
					continue
				}
			} else if len(localOriginPlaces(actual)) == 0 {
				unknown = true
				continue
			}
		}
		actualOrigins = append(actualOrigins, actual)
		for _, base := range localOriginPlaces(actual) {
			base = a.composeReturnedOriginPlace(base, projections, site, path)
			instantiatedPlaces = append(instantiatedPlaces, base)
		}
	}
	result := localReferenceOrigin{Unknown: unknown, Ambiguous: unknown, Mutable: summary.Mutable}
	for _, actual := range actualOrigins {
		if actual.Local && !result.Local {
			result.Local, result.Name, result.Token = true, actual.Name, actual.Token
		}
		if actual.MatchScoped && !result.MatchScoped {
			result.MatchScoped = true
			if result.Name == "" {
				result.Name, result.Token = actual.Name, actual.Token
			}
		}
	}
	result = localOriginWithPlaces(result, instantiatedPlaces)
	result.Unknown = result.Unknown || unknown
	result.Ambiguous = result.Ambiguous || result.Unknown
	if len(summary.Contained) > 0 {
		result.Contained = make(map[string]localReferenceOrigin, len(summary.Contained))
		paths := make([]string, 0, len(summary.Contained))
		for key := range summary.Contained {
			paths = append(paths, key)
		}
		sort.Strings(paths)
		for _, pathKey := range paths {
			child := summary.Contained[pathKey]
			result.Contained[pathKey] = a.instantiateReturnOriginAt(child, args, receiver, site, path+"/"+pathKey)
		}
		if len(places) == 0 {
			recomputeContainedOriginSummary(&result)
			for _, child := range result.Contained {
				if child.Unknown {
					result.Unknown = true
					result.Ambiguous = true
				}
			}
		}
	}
	return result
}

// actualOriginForSummaryRoot reads existing argument/receiver provenance and
// preserves Unknown when no verified caller relation is available.
// Rules: rules/memory/lifetime_analysis.md — §28; rules/mlir/packages/sec-mlir-dialect_package16.md — §§66–67.
func (a *Analyzer) actualOriginForSummaryRoot(root string, args []ast.Expression, receiver ast.Expression) (localReferenceOrigin, bool) {
	if strings.HasPrefix(root, "$param:") {
		index, err := strconv.Atoi(strings.TrimPrefix(root, "$param:"))
		if err != nil || index < 0 || index >= len(args) {
			return localReferenceOrigin{Unknown: true, Ambiguous: true}, false
		}
		if origin, ok := a.directReferenceOrigin(args[index]); ok {
			origin.Contained = a.referenceContentsForBinding(args[index])
			return origin, true
		}
		contained := a.referenceContentsForBinding(args[index])
		if len(contained) > 0 {
			return localReferenceOrigin{Contained: contained}, true
		}
		if a.freshTemporaryBorrowSource(args[index]) {
			// MD-076: the borrow of a fresh temporary argument.
			return a.temporaryArgumentOrigin(args[index]), true
		}
		return localReferenceOrigin{Unknown: true, Ambiguous: true}, false
	}
	if root == "$receiver" {
		if receiver == nil {
			return localReferenceOrigin{Unknown: true, Ambiguous: true}, false
		}
		if origin, ok := a.directReferenceOrigin(receiver); ok {
			origin.Contained = a.referenceContentsForBinding(receiver)
			return origin, true
		}
		place, ok := a.resolvePlace(receiver)
		if !ok {
			if a.freshTemporaryBorrowSource(receiver) {
				// MD-076: the borrow of a fresh temporary receiver.
				return a.temporaryArgumentOrigin(receiver), true
			}
			return localReferenceOrigin{Unknown: true, Ambiguous: true}, false
		}
		name, token, local, _, _ := a.referenceOriginForExpression(receiver)
		origin := localOriginWithPlaces(localReferenceOrigin{Name: name, Token: token, Local: local}, placeOriginAlternatives(place))
		origin.Contained = a.referenceContentsForBinding(receiver)
		return origin, true
	}
	if strings.HasPrefix(root, "$static:") {
		name := strings.TrimPrefix(root, "$static:")
		place, ok := a.rootPlace(name)
		if !ok {
			place = Place{Root: name, Addressable: true}
		}
		return localOriginWithPlaces(localReferenceOrigin{Name: name}, []Place{place}), true
	}
	return localReferenceOrigin{Unknown: true, Ambiguous: true}, false
}
