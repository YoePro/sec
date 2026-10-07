package sema

import "sec/internal/ast"

// foreignExtentMemberOrigin captures storage origin at the resolved call while
// lexical symbols and reference provenance are still available. Only canonical
// Ptr, Len and value SizeOf facts establish origins; user getters, raw-pointer
// parameters, static type sizes and unresolved reference targets remain unknown.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical foreign extent relationships",
// "FFI pointer/extent provenance mismatch", "FFI pointer/size origin mismatch";
// rules/compiler/compiler_known_members.md — "Ptr", "Value-form SizeOf".
func (a *Analyzer) foreignExtentMemberOrigin(expression ast.Expression, names ...string) *Place {
	member, ok := expression.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return nil
	}
	known, ok := a.compilerKnownMemberFacts[sourceTokenLocation(member.Property.Token)]
	if !ok || known.Kind != CompilerKnownProperty {
		return nil
	}
	allowed := false
	for _, name := range names {
		if name == known.Name {
			allowed = true
		}
	}
	if !allowed || known.ID == "CKM-SIZEOF-TYPE" {
		return nil
	}
	place, ok := a.resolvePlace(member.Object)
	if !ok {
		return nil
	}
	if place.Type.Kind == ReferenceType && place.Type.Element != nil {
		place = a.canonicalDereferencePlace(member.Object, place, place.Type)
	}
	// String bindings do not prove distinct backing: immutable copies/views
	// may share the active plan's encoded storage representation.
	if place.Type.Kind == StringType {
		return nil
	}
	if place.Type.Kind == SliceType {
		if _, known := a.referencePlaceOrigin(member.Object); !known {
			return nil
		}
		place = a.canonicalSlicePlace(member.Object, place)
	}
	// A reference parameter identifies the binding, not a distinct referent.
	// Two independently declared reference parameters can alias at a call.
	if root, exists := a.symbols[place.Root]; exists {
		if !sameSourceToken(root.Token, place.RootToken) || root.Type.Kind == ReferenceType || root.Type.Kind == SliceType || root.Type.Kind == StringType {
			return nil
		}
	} else {
		return nil
	}
	if place.AmbiguousProvenance || place.RootKind == PlaceRootDeref {
		return nil
	}
	for _, projection := range place.Projections {
		if projection.Kind == PlaceDereference || projection.Kind == PlaceProperty {
			return nil
		}
	}
	result := clonePlace(place)
	return &result
}

// cloneForeignBufferExtents detaches nested storage provenance as well as the
// flat foreign contract facts. Consumers cannot mutate a later analysis snapshot.
// Rules: rules/analysis/pitfall_analysis.md — "Determinism", "Canonical foreign extent relationships".
func cloneForeignBufferExtents(facts []ResolvedForeignBufferExtent) []ResolvedForeignBufferExtent {
	result := append([]ResolvedForeignBufferExtent(nil), facts...)
	for i := range result {
		if facts[i].PointerOrigin != nil {
			p := clonePlace(*facts[i].PointerOrigin)
			result[i].PointerOrigin = &p
		}
		if facts[i].ExtentOrigin != nil {
			p := clonePlace(*facts[i].ExtentOrigin)
			result[i].ExtentOrigin = &p
		}
	}
	return result
}
