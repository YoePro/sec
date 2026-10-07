package sema

import "sec/internal/ast"

// recordForeignExtentQuantity captures independent quantity units only when
// canonical Ptr and Len/SizeOf describe the same stable receiver. Numeric-looking
// arguments, user getters, conversions and arithmetic do not invent units.
// Rules: rules/analysis/pitfall_analysis.md — "FFI element count versus byte count";
// rules/compiler/compiler_known_members.md — "Ptr", "Len", "SizeOf".
func (a *Analyzer) recordForeignExtentQuantity(pointer, extent ast.Expression, fact *ResolvedForeignBufferExtent) {
	p, ok := pointer.(*ast.MemberExpression)
	if !ok || p.Property == nil {
		return
	}
	e, ok := extent.(*ast.MemberExpression)
	if !ok || e.Property == nil {
		return
	}
	pk, pKnown := a.compilerKnownMemberFacts[sourceTokenLocation(p.Property.Token)]
	ek, eKnown := a.compilerKnownMemberFacts[sourceTokenLocation(e.Property.Token)]
	if !pKnown || !eKnown || pk.Name != "Ptr" || pk.Kind != CompilerKnownProperty || ek.Kind != CompilerKnownProperty || (ek.Name != "Len" && ek.ID != "CKM-SIZEOF-VALUE") {
		return
	}
	left, lKnown := a.resolvePlace(p.Object)
	right, rKnown := a.resolvePlace(e.Object)
	if !lKnown || !rKnown {
		return
	}
	// Known alias origins permit matching different holders. Otherwise equality
	// of the exact stable receiver Place also matches opaque caller references.
	same := fact.PointerOrigin != nil && fact.ExtentOrigin != nil && Relationship(*fact.PointerOrigin, *fact.ExtentOrigin) == PlaceSame
	if !same {
		for _, place := range []Place{left, right} {
			if place.AmbiguousProvenance {
				return
			}
			for _, projection := range place.Projections {
				if projection.Kind == PlaceProperty {
					return
				}
			}
		}
		same = Relationship(left, right) == PlaceSame
	}
	if !same {
		return
	}
	typ := dereferenceType(left.Type)
	var element Type
	switch {
	case (typ.Kind == ArrayType || typ.Kind == SliceType) && typ.Element != nil:
		element = *typ.Element
	case typ.Name == "list" && len(typ.TypeArgs) == 1:
		element = typ.TypeArgs[0]
	case typ.Kind == StringType:
		element = builtinType("byte")
	default:
		return
	}
	bytes, known := a.resolvedForeignElementBytes(element)
	if !known {
		return
	}
	fact.ElementStorageBytes = bytes
	fact.SuppliedExtentUnit = ForeignExtentElements
	if ek.Name == "SizeOf" || typ.Kind == StringType {
		fact.SuppliedExtentUnit = ForeignExtentBytes
	}
	if length, known := exactFixedArrayLength(typ); known && length.Sign() == 0 {
		fact.ZeroExtent = true
	}
}

// resolvedForeignElementBytes consumes exact native scalar representation facts.
// It does not use advisory size estimates or guess padding/aggregate layouts.
// Fixed integer widths are canonical language widths; native integers and plain
// floats use the Analyzer's resolved target facts, never the host architecture.
// Rules: rules/memory/layout.md — §§12(3–5),18(1–4a);
// rules/types/types.md — "byte"; rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§6.8–6.9.
func (a *Analyzer) resolvedForeignElementBytes(typ Type) (int64, bool) {
	for depth := 0; depth < 8; depth++ {
		if typ.Kind == IntType || typ.Kind == UintType || typ.Kind == FloatType || typ.Kind == BoolType {
			if typ.BitWidth > 0 && typ.BitWidth%8 == 0 {
				return typ.BitWidth / 8, true
			}
		}
		if typ.Kind != IntType && typ.Kind != UintType && typ.Kind != FloatType && typ.Kind != CharType && typ.Kind != RuneType {
			return 0, false
		}
		if typ.Named || typ.Declared {
			underlying, known := a.types[typ.Underlying]
			if !known || underlying.Name == typ.Name {
				return 0, false
			}
			typ = underlying
			continue
		}
		switch typ.Kind {
		case IntType, UintType:
			switch typ.Name {
			case "byte", "int8", "uint8":
				return 1, true
			case "int16", "uint16":
				return 2, true
			case "int32", "uint32":
				return 4, true
			case "int64", "uint64":
				return 8, true
			case "int128", "uint128":
				return 16, true
			case "int256", "uint256":
				return 32, true
			case "int", "uint":
				return int64(a.targetUintWidthBits) / 8, true
			}
		case FloatType:
			if typ.FloatBits == 32 || typ.FloatBits == 64 {
				return int64(typ.FloatBits) / 8, true
			}
		case CharType:
			return 1, true
		case RuneType:
			return 4, true
		}
		return 0, false
	}
	return 0, false
}
