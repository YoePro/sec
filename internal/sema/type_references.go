package sema

import "sec/internal/ast"

// resolveTypeReference resolves nominal identity, generic arguments, storage
// shape and units without losing inherited named-list default facts.
// Rules: rules/types/types.md — Named types, Safe references, First-class collections;
// rules/types/default_values.md — Named types and List defaults.
func (a *Analyzer) resolveTypeReference(ref *ast.TypeReference) (Type, bool) {
	if ref == nil || ref.Invalid {
		return Type{Kind: InvalidType}, false
	}

	if ref.UnitOnly {
		return a.resolveUnitOnlyType(ref)
	}

	if ref.Ref {
		if ref.Slice && ref.ElementType != nil {
			element, ok := a.resolveType(ref.ElementType)
			if !ok {
				return Type{Kind: InvalidType}, false
			}
			if element.Kind == VoidType {
				a.addErrorAtToken(ref.Token, "slice element type cannot be void")
				return Type{Kind: InvalidType}, false
			}
			slice := Type{
				Name:    typeDisplayName(element) + "[]",
				Kind:    SliceType,
				Element: &element,
			}
			name := "ref " + typeDisplayName(slice)
			if ref.MutableRef {
				name = "ref mut " + typeDisplayName(slice)
			}
			return Type{
				Name:             name,
				Kind:             ReferenceType,
				Element:          &slice,
				ReferenceMutable: ref.MutableRef,
			}, true
		}
		innerRef := *ref
		innerRef.Ref = false
		innerRef.MutableRef = false
		if innerRef.ReferentToken.Line > 0 {
			innerRef.Token = innerRef.ReferentToken
		}
		inner, ok := a.resolveType(&innerRef)
		if !ok {
			return Type{Kind: InvalidType}, false
		}
		if inner.Kind == VoidType {
			a.addErrorAtToken(ref.Token, "safe reference cannot target void; use RawPtr[void] for an opaque raw address")
			return Type{Kind: InvalidType}, false
		}
		name := "ref " + typeDisplayName(inner)
		if ref.MutableRef {
			name = "ref mut " + typeDisplayName(inner)
		}
		return Type{
			Name:             name,
			Kind:             ReferenceType,
			Element:          &inner,
			ReferenceMutable: ref.MutableRef,
		}, true
	}

	if ref.Name == "fn" || ref.FunctionReturnType != nil {
		return a.resolveFunctionType(ref)
	}

	if ref.Name == "self" && a.currentImplTarget != "" {
		target, ok := a.types[a.currentImplTarget]
		if !ok {
			a.addErrorAtToken(ref.Token, "unknown type self")
			return Type{Kind: InvalidType}, false
		}
		if definition, exists := a.typeDefinitionTokens[a.currentImplTarget]; exists {
			a.bindDefinition(ref.Token, definition)
		}
		return target, true
	}

	if ref.ElementType != nil {
		element, ok := a.resolveType(ref.ElementType)
		if !ok {
			return Type{Kind: InvalidType}, false
		}
		if element.Kind == VoidType {
			a.addErrorAtToken(ref.Token, "sequence element type cannot be void")
			return Type{Kind: InvalidType}, false
		}
		if !ref.Slice {
			length, ok := a.resolveArrayLength(ref)
			if !ok {
				return Type{Kind: InvalidType}, false
			}
			if !arrayLengthFitsUint(length, a.targetUintWidthBits) {
				a.addErrorAtToken(ref.Token, "fixed-array length %s overflows target uint%d", length.String(), a.targetUintWidthBits)
				return Type{Kind: InvalidType}, false
			}
			return NewFixedArrayType(element, length), true
		}
		return NewDynamicArrayType(element), true
	}

	if genericType, ok := a.genericTypes[ref.Name]; ok {
		if len(ref.TypeArgs) > 0 {
			a.addErrorAtToken(ref.Token, "generic parameter %s does not take type arguments", ref.Name)
			return Type{Kind: InvalidType}, false
		}
		if definition, exists := a.genericTypeDefinitions[ref.Name]; exists {
			a.bindDefinition(ref.Token, definition)
		}
		return genericType, true
	}

	typeArgs := make([]Type, 0, len(ref.TypeArgs))
	for _, arg := range ref.TypeArgs {
		argType, ok := a.resolveType(arg)
		if ok {
			typeArgs = append(typeArgs, argType)
		}
	}
	constArgs := make([]int64, 0, len(ref.ConstArgs))
	for _, arg := range ref.ConstArgs {
		value, ok := a.integerConstantValue(arg)
		if !ok {
			a.addErrorAtToken(expressionToken(arg), "%s argument must be a compile-time integer", ref.Name)
			continue
		}
		if !value.IsInt64() {
			a.addErrorAtToken(expressionToken(arg), "%s argument cannot be represented by int64", ref.Name)
			continue
		}
		constArgs = append(constArgs, value.Int64())
	}

	name := a.resolveTypeName(ref.Name)

	typ, ok := a.types[name]
	if !ok {
		if !a.reportUnresolvedForeignType(ref.Name, ref.Token) {
			a.addErrorAtToken(ref.Token, "unknown type %s", ref.Name)
		}
		return Type{Kind: InvalidType}, false
	}
	if definition, exists := a.typeDefinitionTokens[name]; exists {
		a.bindDefinition(ref.Token, definition)
	}
	if !a.canAccessDeclaredName(typ.Name, typ.Module) {
		a.addErrorAtToken(ref.Token, "type %s is not accessible from module %s", ref.Name, a.currentModule)
		return Type{Kind: InvalidType}, false
	}

	if typ.Kind == ResultType && len(ref.TypeArgs) != 2 {
		a.addErrorAtToken(ref.Token, "Result requires exactly 2 type arguments, got %d", len(ref.TypeArgs))
		return Type{Kind: InvalidType}, false
	}
	if typ.Kind != ResultType && len(typ.GenericParameters) == 0 && len(ref.TypeArgs) > 0 {
		a.addErrorAtToken(ref.Token, "%s is not generic", ref.Name)
		return Type{Kind: InvalidType}, false
	}
	if typ.Kind != ResultType && len(typ.GenericParameters) > 0 && len(ref.TypeArgs) == 0 {
		a.addErrorAtToken(ref.Token, "%s requires %d generic arguments, got 0", ref.Name, len(typ.GenericParameters))
		return Type{Kind: InvalidType}, false
	}
	if len(typ.GenericParameters) > 0 && len(ref.TypeArgs) != len(typ.GenericParameters) &&
		!acceptsExtendedCompilerKnownTypeArguments(typ, len(ref.TypeArgs)) {
		a.addErrorAtToken(ref.Token, "%s requires %d generic arguments, got %d", ref.Name, len(typ.GenericParameters), len(ref.TypeArgs))
		return Type{Kind: InvalidType}, false
	}
	if len(typeArgs) != len(ref.TypeArgs) || len(constArgs) != len(ref.ConstArgs) {
		return Type{Kind: InvalidType}, false
	}

	// A nongeneric named list already fixes its element and capacity.
	// Its empty argument spelling must preserve those inherited facts.
	// Rules: rules/types/default_values.md — Named types and List defaults.
	if !(typ.Named && typ.EmptyListDefault && len(ref.TypeArgs) == 0 && len(ref.ConstArgs) == 0) {
		typ.TypeArgs = typeArgs
		typ.ConstArgs = constArgs
	}
	if !a.validateVoidTypeArguments(ref.Token, typ) {
		return Type{Kind: InvalidType}, false
	}
	if !a.validateResultErrorChannel(ref, typ) {
		return Type{Kind: InvalidType}, false
	}
	if ref.EventCapacitySet {
		value, ok := a.integerConstantValue(ref.EventCapacityExpression)
		if !ok {
			a.addErrorAtToken(expressionToken(ref.EventCapacityExpression), "%s capacity must be a compile-time integer", ref.Name)
			return Type{Kind: InvalidType}, false
		}
		if !value.IsInt64() {
			a.addErrorAtToken(expressionToken(ref.EventCapacityExpression), "%s capacity cannot be represented by int64", ref.Name)
			return Type{Kind: InvalidType}, false
		}
		typ.EventCapacity = value.Int64()
		typ.EventCapacitySet = true
	}
	if !a.validateCompilerKnownGenericType(ref.Token, &typ) {
		return Type{Kind: InvalidType}, false
	}
	if ref.Unit != "" {
		if !a.validateConcreteUnitParameters(ref) {
			return Type{Kind: InvalidType}, false
		}
		semantics, dimension, err := resolveUnitSemantics(ref.Unit, ref.UnitExpression, a.units)
		if err != nil {
			a.addErrorAtToken(ref.Token, "invalid unit expression %s: %s", ref.Unit, err)
			return Type{Kind: InvalidType}, false
		}
		a.warnUnitStatus(ref.Token, ref.Unit)
		typ.Unit = ref.Unit
		typ.Dimension = dimension
		typ.UnitSemantics = semantics
	}
	if len(typ.GenericParameters) > 0 && (typ.Declared || typ.Kind == StructType || typ.Kind == UnionType || typ.Kind == EnumType || typ.Kind == InterfaceType) {
		if !a.validateGenericTypeConstraintArguments(ref.Token, typ) {
			return Type{Kind: InvalidType}, false
		}
		typ = a.instantiateGenericType(typ)
	}
	return typ, true
}
