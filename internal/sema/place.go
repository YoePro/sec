package sema

import (
	"fmt"
	"math"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

type PlaceProjectionKind string

const (
	PlaceField        PlaceProjectionKind = "field"
	PlaceProperty     PlaceProjectionKind = "property"
	PlaceIndex        PlaceProjectionKind = "index"
	PlaceSlice        PlaceProjectionKind = "slice"
	PlaceDereference  PlaceProjectionKind = "dereference"
	PlaceUnionPayload PlaceProjectionKind = "union-payload"
)

type PlaceProjection struct {
	Kind            PlaceProjectionKind
	Name            string
	ConstantIndex   int64
	DynamicIndex    bool
	SliceStart      int64
	SliceEnd        int64
	SliceStartKnown bool
	SliceEndKnown   bool
	Token           lexer.Token
}

// Place identifies a reusable semantic storage path. It is frontend-only
// analysis data and introduces no runtime representation or borrow counter.
type Place struct {
	Root        string
	RootToken   lexer.Token
	Projections []PlaceProjection
	Type        Type
	Mutable     bool
	Addressable bool
	// AmbiguousProvenance is set when loop control flow joins different
	// referent Places for the same reference binding. Direct access remains
	// protected by the joined owner borrows, but creating another reference
	// requires one canonical origin and is rejected.
	AmbiguousProvenance bool
	// AlternativeOrigins contains additional compile-time referent Places when
	// control flow joins a finite set of known reference origins. The primary
	// Place remains in Root/Projections; no runtime provenance tag is emitted.
	AlternativeOrigins []Place
	// ReferenceHolder names the reference/slice binding through which this
	// canonical place was reached. Its own borrow grants access to the referent
	// and must therefore not conflict with accesses performed through it.
	ReferenceHolder string
	// PartialMoveSafe is deliberately narrower than addressability. It is true
	// only for independently tracked local struct storage, never for aliases,
	// properties, registers, indexes, slices, or other externally observable
	// storage.
	PartialMoveSafe bool
}

func (p Place) String() string {
	var out strings.Builder
	out.WriteString(p.Root)
	for _, projection := range p.Projections {
		switch projection.Kind {
		case PlaceField, PlaceProperty:
			out.WriteByte('.')
			out.WriteString(projection.Name)
		case PlaceIndex:
			if projection.DynamicIndex {
				out.WriteString("[*]")
			} else {
				fmt.Fprintf(&out, "[%d]", projection.ConstantIndex)
			}
		case PlaceSlice:
			out.WriteByte('[')
			if projection.SliceStartKnown && projection.SliceStart != 0 {
				fmt.Fprintf(&out, "%d", projection.SliceStart)
			}
			if projection.SliceEndKnown {
				out.WriteString("..<")
				fmt.Fprintf(&out, "%d", projection.SliceEnd)
			} else {
				out.WriteString("..")
			}
			out.WriteByte(']')
		case PlaceDereference:
			out.WriteString(".*")
		case PlaceUnionPayload:
			out.WriteString(".<")
			out.WriteString(projection.Name)
			out.WriteByte('>')
		}
	}
	return out.String()
}

func PlacesOverlap(left, right Place) bool {
	leftAlternatives := placeOriginAlternatives(left)
	rightAlternatives := placeOriginAlternatives(right)
	if len(leftAlternatives) > 1 || len(rightAlternatives) > 1 {
		for _, leftAlternative := range leftAlternatives {
			for _, rightAlternative := range rightAlternatives {
				if PlacesOverlap(leftAlternative, rightAlternative) {
					return true
				}
			}
		}
		return false
	}
	if left.Root == "" || right.Root == "" || left.Root != right.Root {
		return false
	}
	if placeIsStaticallyEmpty(left) || placeIsStaticallyEmpty(right) {
		return false
	}
	limit := len(left.Projections)
	if len(right.Projections) < limit {
		limit = len(right.Projections)
	}
	for index := 0; index < limit; index++ {
		leftProjection := left.Projections[index]
		rightProjection := right.Projections[index]
		if leftProjection.Kind != rightProjection.Kind {
			if projectionsAreDisjointIndexAndSlice(leftProjection, rightProjection) {
				return false
			}
			return true
		}
		switch leftProjection.Kind {
		case PlaceField:
			if leftProjection.Name != rightProjection.Name {
				return false
			}
		case PlaceProperty:
			// A setter may touch any receiver storage unless effect metadata proves
			// otherwise, so properties conservatively overlap at their receiver.
			return true
		case PlaceIndex:
			if !leftProjection.DynamicIndex && !rightProjection.DynamicIndex && leftProjection.ConstantIndex != rightProjection.ConstantIndex {
				return false
			}
		case PlaceSlice:
			if slicesAreStaticallyDisjoint(leftProjection, rightProjection) {
				return false
			}
		case PlaceDereference:
			// Equal dereference paths share provenance in this initial model.
		case PlaceUnionPayload:
			if leftProjection.Name != rightProjection.Name {
				return false
			}
		}
	}
	// Equal paths overlap, and a root/prefix place overlaps every child place.
	return true
}

func placeOriginAlternatives(place Place) []Place {
	primary := place
	primary.AlternativeOrigins = nil
	alternatives := []Place{primary}
	for _, alternative := range place.AlternativeOrigins {
		alternative.AlternativeOrigins = nil
		alternatives = append(alternatives, alternative)
	}
	return alternatives
}

func appendPlaceProjection(place Place, projection PlaceProjection) Place {
	place.Projections = append(place.Projections, projection)
	for index := range place.AlternativeOrigins {
		place.AlternativeOrigins[index].Projections = append(place.AlternativeOrigins[index].Projections, projection)
	}
	return place
}

func placeIsStaticallyEmpty(place Place) bool {
	for _, projection := range place.Projections {
		if projection.Kind == PlaceSlice && projection.SliceStartKnown && projection.SliceEndKnown && projection.SliceStart >= projection.SliceEnd {
			return true
		}
	}
	return false
}

func slicesAreStaticallyDisjoint(left, right PlaceProjection) bool {
	return left.SliceEndKnown && right.SliceStartKnown && left.SliceEnd <= right.SliceStart ||
		right.SliceEndKnown && left.SliceStartKnown && right.SliceEnd <= left.SliceStart
}

func projectionsAreDisjointIndexAndSlice(left, right PlaceProjection) bool {
	if left.Kind == PlaceSlice && right.Kind == PlaceIndex {
		return indexOutsideSlice(right, left)
	}
	if left.Kind == PlaceIndex && right.Kind == PlaceSlice {
		return indexOutsideSlice(left, right)
	}
	return false
}

func indexOutsideSlice(index, slice PlaceProjection) bool {
	if index.DynamicIndex {
		return false
	}
	return slice.SliceStartKnown && index.ConstantIndex < slice.SliceStart ||
		slice.SliceEndKnown && index.ConstantIndex >= slice.SliceEnd
}

func unionPayloadPlace(subject Place, variant string, payloadType Type, token lexer.Token) Place {
	subject = appendPlaceProjection(subject, PlaceProjection{
		Kind:  PlaceUnionPayload,
		Name:  variant,
		Token: token,
	})
	subject.Type = payloadType
	return subject
}

func (a *Analyzer) resolvePlace(expr ast.Expression) (Place, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		symbol, ok := a.symbols[expr.Value]
		if !ok {
			return Place{}, false
		}
		return Place{
			Root: expr.Value, RootToken: symbol.Token, Type: symbol.Type,
			Mutable:         a.canWriteThroughSymbol(symbol),
			Addressable:     true,
			PartialMoveSafe: symbol.Local && !symbol.ImplicitMember && !symbol.Volatile && symbol.Storage == StorageOriginInline && expr.Value != "self",
		}, true
	case *ast.MemberExpression:
		base, ok := a.resolvePlace(expr.Object)
		if !ok || expr.Property == nil {
			return Place{}, false
		}
		objectType := base.Type
		throughReference := objectType.Kind == ReferenceType
		if objectType.Kind == ReferenceType {
			if objectType.Element == nil {
				return Place{}, false
			}
			base = a.canonicalDereferencePlace(expr.Object, base, objectType)
			objectType = *objectType.Element
		}
		if property, ok := lookupProperty(objectType, expr.Property.Value); ok {
			base = appendPlaceProjection(base, PlaceProjection{Kind: PlaceProperty, Name: expr.Property.Value, Token: expr.Property.Token})
			base.Type = property.Type
			base.Mutable = property.HasSetter
			base.Addressable = false
			base.PartialMoveSafe = false
			return base, true
		}
		if fieldType, ok := lookupStructField(objectType, expr.Property.Value); ok {
			base = appendPlaceProjection(base, PlaceProjection{Kind: PlaceField, Name: expr.Property.Value, Token: expr.Property.Token})
			base.Type = fieldType
			if !throughReference {
				base.Mutable = true
			}
			return base, true
		}
		if fieldType, ok := lookupRegisterField(objectType, expr.Property.Value); ok {
			base = appendPlaceProjection(base, PlaceProjection{Kind: PlaceField, Name: expr.Property.Value, Token: expr.Property.Token})
			base.Type = fieldType
			base.PartialMoveSafe = false
			return base, true
		}
		return Place{}, false
	case *ast.IndexExpression:
		base, ok := a.resolvePlace(expr.Left)
		if !ok {
			return Place{}, false
		}
		containerType := base.Type
		if containerType.Kind == ReferenceType {
			if containerType.Element == nil {
				return Place{}, false
			}
			base = a.canonicalDereferencePlace(expr.Left, base, containerType)
			containerType = *containerType.Element
		} else if containerType.Kind == SliceType {
			base = a.canonicalSlicePlace(expr.Left, base)
			containerType = base.Type
		}
		containerType = dereferenceType(containerType)
		if containerType.Kind != ArrayType && containerType.Kind != SliceType || containerType.Element == nil {
			return Place{}, false
		}
		projection := PlaceProjection{Kind: PlaceIndex, DynamicIndex: true, Token: expressionToken(expr.Index)}
		if constant, ok := a.integerExpressionInt64(expr.Index); ok {
			projection.ConstantIndex = constant
			projection.DynamicIndex = false
		}
		base.Projections = appendIndexProjection(base.Projections, projection)
		for index := range base.AlternativeOrigins {
			base.AlternativeOrigins[index].Projections = appendIndexProjection(base.AlternativeOrigins[index].Projections, projection)
		}
		base.Type = *containerType.Element
		base.PartialMoveSafe = false
		return base, true
	case *ast.SliceExpression:
		base, ok := a.resolvePlace(expr.Left)
		if !ok {
			return Place{}, false
		}
		containerType := base.Type
		if containerType.Kind == ReferenceType {
			if containerType.Element == nil {
				return Place{}, false
			}
			base = a.canonicalDereferencePlace(expr.Left, base, containerType)
			containerType = *containerType.Element
		} else if containerType.Kind == SliceType {
			base = a.canonicalSlicePlace(expr.Left, base)
			containerType = base.Type
		}
		containerType = dereferenceType(containerType)
		if containerType.Kind != ArrayType && containerType.Kind != SliceType || containerType.Element == nil {
			return Place{}, false
		}
		projection := a.slicePlaceProjection(expr, containerType)
		base.Projections = appendSliceProjection(base.Projections, projection)
		for index := range base.AlternativeOrigins {
			base.AlternativeOrigins[index].Projections = appendSliceProjection(base.AlternativeOrigins[index].Projections, projection)
		}
		base.Type = Type{Name: typeDisplayName(*containerType.Element) + "[]", Kind: SliceType, Element: containerType.Element}
		base.PartialMoveSafe = false
		return base, true
	default:
		return Place{}, false
	}
}

func (a *Analyzer) slicePlaceProjection(expr *ast.SliceExpression, containerType Type) PlaceProjection {
	projection := PlaceProjection{Kind: PlaceSlice, SliceStartKnown: true, Token: expr.Token}
	if expr.Start != nil {
		projection.SliceStart, projection.SliceStartKnown = a.integerExpressionInt64(expr.Start)
	}
	if expr.End != nil {
		end, ok := a.integerExpressionInt64(expr.End)
		if ok && !expr.Exclusive {
			if end == math.MaxInt64 {
				ok = false
			} else {
				end++
			}
		}
		projection.SliceEnd = end
		projection.SliceEndKnown = ok
	} else if containerType.Kind == ArrayType && containerType.ArrayLength != dynamicArrayLength {
		projection.SliceEnd = containerType.ArrayLength
		projection.SliceEndKnown = true
	}
	return projection
}

func appendIndexProjection(projections []PlaceProjection, index PlaceProjection) []PlaceProjection {
	if len(projections) == 0 || index.DynamicIndex {
		return append(projections, index)
	}
	last := projections[len(projections)-1]
	if last.Kind != PlaceSlice || !last.SliceStartKnown {
		return append(projections, index)
	}
	if index.ConstantIndex < 0 || (last.SliceEndKnown && index.ConstantIndex >= last.SliceEnd-last.SliceStart) {
		return append(projections, index)
	}
	absolute, ok := addInt64(index.ConstantIndex, last.SliceStart)
	if !ok {
		return append(projections, index)
	}
	index.ConstantIndex = absolute
	return append(projections[:len(projections)-1], index)
}

func appendSliceProjection(projections []PlaceProjection, child PlaceProjection) []PlaceProjection {
	if len(projections) == 0 {
		return append(projections, child)
	}
	parent := projections[len(projections)-1]
	if parent.Kind != PlaceSlice || !parent.SliceStartKnown {
		return append(projections, child)
	}
	if (child.SliceStartKnown && child.SliceStart < 0) || (child.SliceEndKnown && child.SliceEnd < 0) {
		return append(projections, child)
	}
	if parent.SliceEndKnown {
		length := parent.SliceEnd - parent.SliceStart
		if (child.SliceStartKnown && child.SliceStart > length) || (child.SliceEndKnown && child.SliceEnd > length) {
			return append(projections, child)
		}
	}
	if absolute, ok := addInt64(child.SliceStart, parent.SliceStart); child.SliceStartKnown && ok {
		child.SliceStart = absolute
	} else if child.SliceStartKnown {
		child.SliceStartKnown = false
	}
	if child.SliceEndKnown {
		if absolute, ok := addInt64(child.SliceEnd, parent.SliceStart); ok {
			child.SliceEnd = absolute
		} else {
			child.SliceEndKnown = false
		}
	} else if parent.SliceEndKnown {
		child.SliceEnd = parent.SliceEnd
		child.SliceEndKnown = true
	}
	return append(projections[:len(projections)-1], child)
}

func addInt64(left, right int64) (int64, bool) {
	if (right > 0 && left > math.MaxInt64-right) || (right < 0 && left < math.MinInt64-right) {
		return 0, false
	}
	return left + right, true
}

func (a *Analyzer) canonicalDereferencePlace(expr ast.Expression, fallback Place, referenceType Type) Place {
	if origin, ok := a.referencePlaceOrigin(expr); ok {
		origin.Type = *referenceType.Element
		origin.Mutable = referenceType.ReferenceMutable
		origin.Addressable = true
		origin.PartialMoveSafe = false
		for index := range origin.AlternativeOrigins {
			origin.AlternativeOrigins[index].Type = *referenceType.Element
			origin.AlternativeOrigins[index].Mutable = referenceType.ReferenceMutable
			origin.AlternativeOrigins[index].Addressable = true
			origin.AlternativeOrigins[index].PartialMoveSafe = false
		}
		return origin
	}
	if a.referencePlaceOriginIsAmbiguous(expr) {
		fallback.ReferenceHolder, _ = borrowRootName(expr)
		fallback.AmbiguousProvenance = true
	}
	fallback.Projections = append(fallback.Projections, PlaceProjection{Kind: PlaceDereference, Token: expressionToken(expr)})
	fallback.Mutable = referenceType.ReferenceMutable
	fallback.PartialMoveSafe = false
	return fallback
}

func (a *Analyzer) canonicalSlicePlace(expr ast.Expression, fallback Place) Place {
	if origin, ok := a.referencePlaceOrigin(expr); ok {
		origin.Type = fallback.Type
		origin.Mutable = fallback.Mutable
		origin.Addressable = true
		origin.PartialMoveSafe = false
		for index := range origin.AlternativeOrigins {
			origin.AlternativeOrigins[index].Type = fallback.Type
			origin.AlternativeOrigins[index].Mutable = fallback.Mutable
			origin.AlternativeOrigins[index].Addressable = true
			origin.AlternativeOrigins[index].PartialMoveSafe = false
		}
		return origin
	}
	if a.referencePlaceOriginIsAmbiguous(expr) {
		fallback.ReferenceHolder, _ = borrowRootName(expr)
		fallback.AmbiguousProvenance = true
	}
	return fallback
}

func (a *Analyzer) rootPlace(name string) (Place, bool) {
	symbol, ok := a.symbols[name]
	if !ok {
		return Place{}, false
	}
	return Place{
		Root: name, RootToken: symbol.Token, Type: symbol.Type,
		Mutable:         a.canWriteThroughSymbol(symbol),
		Addressable:     true,
		PartialMoveSafe: symbol.Local && !symbol.ImplicitMember && !symbol.Volatile && symbol.Storage == StorageOriginInline && name != "self",
	}, true
}

func borrowPlacesOverlap(candidate Place, record borrowRecord) bool {
	for _, alternative := range placeOriginAlternatives(candidate) {
		if record.Place.Root == "" {
			if alternative.Root == record.Root {
				return true
			}
			continue
		}
		if PlacesOverlap(alternative, record.Place) {
			return true
		}
	}
	return false
}

func (a *Analyzer) inferPlaceBase(expr ast.Expression) (Type, expressionValue) {
	a.suppressPlaceRootRead++
	defer func() { a.suppressPlaceRootRead-- }()
	return a.inferExpression(expr)
}

func (a *Analyzer) unavailablePlace(place Place) (lexer.Token, string, bool, bool) {
	key := place.String()
	if token, ok := a.moved[place.Root]; ok {
		return token, place.Root, false, true
	}
	for movedKey, token := range a.moved {
		if movedKey == place.Root {
			continue
		}
		if movedKey == key || strings.HasPrefix(key, movedKey+".") || strings.HasPrefix(key, movedKey+"[") {
			return token, movedKey, false, true
		}
		if strings.HasPrefix(movedKey, key+".") || strings.HasPrefix(movedKey, key+"[") {
			return token, movedKey, true, true
		}
	}
	return lexer.Token{}, "", false, false
}

func (a *Analyzer) checkPlaceAvailableForRead(place Place, token lexer.Token) bool {
	movedAt, movedKey, partial, unavailable := a.unavailablePlace(place)
	if !unavailable || partial && a.suppressPlaceRootRead > 0 {
		return false
	}
	if a.loopBackedgePlaces[movedKey] {
		a.addErrorAtTokenWithPrevious(token, movedAt, "place %s may be unavailable on a later loop iteration", place.String())
		return true
	}
	if partial {
		a.addErrorAtTokenWithPrevious(token, movedAt, "cannot use partially moved value %s; place %s is unavailable", place.String(), movedKey)
		return true
	}
	reason := a.moveReasons[movedKey]
	switch reason {
	case "discarded":
		a.addErrorAtTokenWithPrevious(token, movedAt, "value %s was discarded here and is no longer available", place.String())
	case "detached":
		a.addErrorAtTokenWithPrevious(token, movedAt, "value %s was detached here and is no longer available", place.String())
	case "released":
		a.addErrorAtTokenWithPrevious(token, movedAt, "arena %s was released here and is no longer available", place.String())
	case "consumed by call":
		a.addErrorAtTokenWithPrevious(token, movedAt, "value %s was consumed by call here and is no longer available", place.String())
	default:
		a.addErrorAtTokenWithPrevious(token, movedAt, "use of moved value %s", place.String())
	}
	return true
}

func (a *Analyzer) markPlaceUnavailable(place Place, token lexer.Token, reason string) {
	key := place.String()
	a.moved[key] = token
	a.moveReasons[key] = reason
}

func (a *Analyzer) markPlaceAvailable(place Place) {
	key := place.String()
	for movedKey := range a.moved {
		if movedKey == key || strings.HasPrefix(movedKey, key+".") || strings.HasPrefix(movedKey, key+"[") {
			delete(a.moved, movedKey)
			delete(a.moveReasons, movedKey)
		}
	}
}

func (a *Analyzer) clearRootPlaceState(root string) {
	clearRootPlaceStateMaps(a.moved, a.moveReasons, root)
}

func clearRootPlaceStateMaps(moved map[string]lexer.Token, moveReasons map[string]string, root string) {
	for movedKey := range moved {
		if movedKey == root || strings.HasPrefix(movedKey, root+".") || strings.HasPrefix(movedKey, root+"[") {
			delete(moved, movedKey)
			delete(moveReasons, movedKey)
		}
	}
}
