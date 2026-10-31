package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// typePackParameter identifies the pointwise owned or borrowed type-pack signature,
// distinct from a homogeneous variadic parameter.
// Rules: rules/declarations/functions.md §38A(2–4); rules/declarations/generics.md §16A.
func typePackParameter(function Function) (FunctionParameter, bool) {
	parameter, ok := functionVariadicParameter(function)
	return parameter, ok && function.GenericTypePack != "" && isDeclaredTypePackMode(parameter.Type, function.GenericTypePack)
}

// inferredTypePack returns the ordered compile-time sequence in the existing
// generic substitution model. The map entry distinguishes empty from unresolved.
// Rules: rules/declarations/functions.md §38A(2),(9); rules/compiler/monomorphization.md §8.
func inferredTypePack(function Function) ([]Type, bool) {
	value, ok := function.GenericSubstitution[function.GenericTypePack]
	if !ok || value.Kind != GenericTypePackType {
		return nil, false
	}
	return value.TypeArgs, true
}

// inferTypePackCall infers a scalar generic prefix and the trailing ordered
// sequence. Borrowed argument holders infer their referent as ordinary ref T does;
// actual borrowing and compatibility stay with the ordinary selected-call checks.
// No type erasure, common element type, return-inferred pack sequence or runtime pack
// is introduced. The body may observe Len but not inspect individual elements.
// Rules: rules/declarations/functions.md §38A(3–4),(9); rules/memory/borrowing.md §15.2;
// rules/declarations/generics.md §20 (explicit scalar prefix).
func (a *Analyzer) inferTypePackCall(function Function, refs []*ast.TypeReference, arguments []Type, expected Type) (Function, bool) {
	substitution, ok := a.deduceTypePackArguments(function, refs, arguments)
	if !ok {
		return Function{}, false
	}
	return a.completePackOverload(function, substitution, expected)
}

// deduceTypePackArguments establishes only argument-derived substitutions; a
// scalar used solely in the result stays absent until a candidate is selected.
// Rules: rules/declarations/functions.md §38A(4)(5),(9);
// rules/declarations/generics.md §§18,20,29.
func (a *Analyzer) deduceTypePackArguments(function Function, refs []*ast.TypeReference, arguments []Type) (map[string]Type, bool) {
	parameter, supported := typePackParameter(function)
	if !supported || !a.typePackOwnerSupported(function.ImplTarget) || !functionAcceptsArgumentCount(function, len(arguments)) {
		return nil, false
	}
	if function.ImplTarget != "" && len(function.GenericParameters) != 1 {
		return nil, false
	}
	if len(refs) > len(function.GenericParameters)-1 {
		return nil, false
	}
	substitution := map[string]Type{}
	fixed := map[string]struct{}{}
	for i, ref := range refs {
		typ, ok := a.resolveType(ref)
		if !ok {
			return nil, false
		}
		name := function.GenericParameters[i]
		if name == function.GenericTypePack {
			return nil, false
		}
		substitution[name] = typ
		fixed[name] = struct{}{}
	}
	count := functionFixedParameterCount(function)
	for i, argument := range arguments[:count] {
		if isDeferredContextualPlaceholder(argument) {
			continue
		}
		pattern := function.Parameters[i].Type
		// A concrete parameter contributes no substitution. Its argument must
		// reach ordinary compatibility with the actual source expression, so
		// e.g. an unsuffixed 10 can be checked against uint (not unified as int).
		if !typePackPatternNeedsInference(pattern) {
			continue
		}
		if pattern.Kind == ReferenceType && pattern.Element != nil && argument.Kind != ReferenceType {
			pattern = *pattern.Element
		}
		if !inferGenericTypeSubstitutionWithFixed(pattern, argument, substitution, fixed) {
			return nil, false
		}
	}
	sequence := make([]Type, 0, len(arguments)-count)
	for _, typ := range arguments[count:] {
		if parameter.Type.Kind == ReferenceType && typ.Kind == ReferenceType && typ.Element != nil {
			typ = *typ.Element
		}
		if typ.Kind == InvalidType || typ.Kind == GenericTypePackType || typ.Kind == VariadicPackType {
			return nil, false
		}
		sequence = append(sequence, typ)
	}
	substitution[function.GenericTypePack] = Type{Name: function.GenericTypePack, Kind: GenericTypePackType, TypeArgs: sequence}
	return substitution, true
}

// typePackPatternNeedsInference separates type-variable deduction from concrete
// argument compatibility; only generic positions contribute substitutions.
// Rules: rules/declarations/functions.md §38A(4)(4–5);
// rules/declarations/generics.md — "Generic argument inference".
func typePackPatternNeedsInference(typ Type, name ...string) bool {
	if typ.Kind == GenericType {
		return len(name) == 0 || typ.Name == name[0]
	}
	if typ.Element != nil && typePackPatternNeedsInference(*typ.Element, name...) {
		return true
	}
	for _, argument := range typ.TypeArgs {
		if typePackPatternNeedsInference(argument, name...) {
			return true
		}
	}
	for _, parameter := range typ.FunctionParameterTypes {
		if typePackPatternNeedsInference(parameter, name...) {
			return true
		}
	}
	return typ.FunctionReturnType != nil && typePackPatternNeedsInference(*typ.FunctionReturnType, name...)
}

// supportedTypePackBody permits ordinary checking of unused packs and
// read-only Len observations, whose uint type does not depend on element types.
// Element access, loops, forwarding and capture still require pack expansion.
// Rules: rules/declarations/functions.md §38A(4–8), especially (5)(1–2);
// rules/declarations/generics.md §16A.
func (a *Analyzer) supportedTypePackBody(fn *ast.FunctionDeclaration) bool {
	if fn == nil || !a.typePackOwnerSupported(a.currentImplTarget) || fn.Body == nil || len(fn.Parameters) == 0 {
		return false
	}
	if a.currentImplTarget != "" && len(fn.GenericParameters) != 1 {
		return false
	}
	parameter := fn.Parameters[len(fn.Parameters)-1]
	if parameter == nil || parameter.Name == nil || !parameter.Variadic || parameter.Type == nil {
		return false
	}
	// The parser records reference mode on the parameter type; accept only the
	// declaration's exact pack, never homogeneous ...ref int.
	pack := genericTypePackName(fn.GenericParameters)
	if parameter.Type == nil || parameter.Type.Name != pack {
		return false
	}
	selectors := map[*ast.Identifier]bool{}
	lengthReceivers := map[*ast.Identifier]bool{}
	// A `for` over the pack iterates it type-aware (flowcontrol_for.md § 13A):
	// its body is checked once with the opaque element type of
	// typePackElementType, valid for every specialization.
	iterated := map[*ast.Identifier]bool{}
	ast.Visit(fn.Body, func(node any) bool {
		if loop, ok := node.(*ast.ForStatement); ok && loop != nil {
			if iterable, ok := loop.Iterable.(*ast.Identifier); ok && iterable.Value == parameter.Name.Value {
				iterated[iterable] = true
			}
		}
		// A nested callable must not capture the ephemeral pack merely to
		// observe its length. Such use still needs explicit expansion facts.
		if _, nested := node.(*ast.LambdaExpression); nested {
			return false
		}
		if member, ok := node.(*ast.MemberExpression); ok && member.Property != nil {
			selectors[member.Property] = true
			if receiver, ok := member.Object.(*ast.Identifier); ok && receiver.Value == parameter.Name.Value && member.Property.Value == "Len" {
				lengthReceivers[receiver] = true
			}
		}
		return true
	})
	used := false
	ast.Visit(fn.Body, func(node any) bool {
		if reference, ok := node.(*ast.TypeReference); ok && reference.Name == pack {
			used = true
		}
		if identifier, ok := node.(*ast.Identifier); ok && (identifier.Value == parameter.Name.Value || identifier.Value == pack) && !selectors[identifier] && !lengthReceivers[identifier] && !iterated[identifier] {
			used = true
		}
		return true
	})
	return !used
}

// ResolvedTypePackCall describes the selected ordered sequence and concrete
// receiving parameters; it is compiler metadata, never a runtime tuple/container.
// Rules: rules/declarations/functions.md §38A(2–4),(9–10).
type ResolvedTypePackCall struct {
	Name       string
	Length     uint64 // Exact compile-time arity of this selected specialization.
	Types      []Type
	Parameters []FunctionParameter
	FixedCount int
}

// ResolvedTypePackCallOf publishes detached facts only for selected calls. Pack
// bodies/lowering may still have their own explicit capability boundaries.
// Rules: rules/declarations/functions.md §38A(9–10); rules/compiler/compiler_analysis.md §55.
func (a *Analyzer) ResolvedTypePackCallOf(call *ast.CallExpression) (ResolvedTypePackCall, bool) {
	if a == nil || call == nil {
		return ResolvedTypePackCall{}, false
	}
	resolved, ok := a.resolvedCalls[call]
	if !ok {
		return ResolvedTypePackCall{}, false
	}
	sequence, known := inferredTypePack(resolved.Function)
	if !known {
		return ResolvedTypePackCall{}, false
	}
	result := ResolvedTypePackCall{Name: resolved.Function.GenericTypePack, Length: uint64(len(sequence)), FixedCount: functionFixedParameterCount(resolved.Function), Types: make([]Type, len(sequence))}
	for i, typ := range sequence {
		result.Types[i] = semanticSnapshotType(typ)
	}
	for i := 0; i < result.FixedCount+len(sequence); i++ {
		parameter, ok := functionParameterForArgument(resolved.Function, i)
		if !ok {
			return ResolvedTypePackCall{}, false
		}
		parameter.Type = semanticSnapshotType(parameter.Type)
		result.Parameters = append(result.Parameters, parameter)
	}
	return result, true
}

// rejectUnavailableTypePackCall refuses unrepresented reference-holder transfer
// and static spread transport before ownership/borrow commitment.
// Runtime-length heterogeneous spread is excluded by functionAcceptsCallArguments.
// Rules: rules/declarations/functions.md §38A(4),(6–7); rules/tooling/diagnostics.md §24(4h).
func (a *Analyzer) rejectUnavailableTypePackCall(function Function, call *ast.CallExpression, prepared []bool) bool {
	if function.GenericTypePack == "" {
		return false
	}
	parameter, supported := typePackParameter(function)
	if !supported {
		return false
	}
	borrowed := parameter.Type.Kind == ReferenceType
	if !borrowed {
		sequence, _ := inferredTypePack(function)
		for i, element := range sequence {
			if element.Kind == ReferenceType {
				a.addErrorAtTokenWithMetadata(call.Token, diagnostics.GenericTypePackExpansionUnavailable,
					"Owned reference-holder pack elements need a distinct transfer mode; no implicit borrow or mutable-reference copy is substituted.",
					"compiler capability unavailable: owned type-pack argument %d of type %s requires reference-holder transfer facts", functionFixedParameterCount(function)+i+1, typeDisplayName(element))
				return true
			}
		}
	}
	for i, arg := range call.Arguments {
		if _, explicit := explicitMoveArgument(arg); explicit {
			continue
		} // ordinary ownership validation handles explicit transfer/borrow modes
		if i < functionFixedParameterCount(function) {
			continue
		}
		if typ, typed := a.ResolvedTypeOf(arg); borrowed && typed && typ.Kind != ReferenceType {
			if _, placed := a.resolvePlace(arg); !placed && !a.freshTemporaryBorrowSource(arg) {
				a.addErrorAtTokenWithMetadata(expressionToken(arg), diagnostics.GenericTypePackExpansionUnavailable, "Use a named value until this temporary argument form has a resolved call-lifetime borrow plan.", "compiler capability unavailable: type-pack argument %d of type %s needs temporary materialization", i+1, typeDisplayName(typ))
				return true
			}
		}
	}

	for _, spread := range prepared {
		if spread {
			a.addErrorAtTokenWithMetadata(call.Token, diagnostics.GenericTypePackExpansionUnavailable, "Pass concrete arguments separately until typed heterogeneous spread preparation is implemented.", "compiler capability unavailable: heterogeneous type-pack spread argument preparation")
			return true
		}
	}
	return false
}

// TypePackElementName is the template name of a heterogeneous pack's
// per-position element type: `Types[i]` for pack `Types`. It is not a source
// identifier, so it never collides with the pack's own substitution key;
// Semantic IR binds it to the concrete element of each unrolled iteration.
func TypePackElementName(pack string) string { return pack + "[i]" }

// typePackElementType replaces the pack sequence inside a pack parameter's
// element type with the opaque per-position element: an unconstrained
// generic parameter, so the loop body admits exactly the operations valid for
// every element (functions.md §38A(4)(6); generics.md §16A(4)).
func typePackElementType(typ Type) (Type, bool) {
	switch {
	case typ.Kind == GenericTypePackType:
		return Type{Name: TypePackElementName(typ.Name), Kind: GenericType}, true
	case typ.Kind == ReferenceType && typ.Element != nil:
		element, ok := typePackElementType(*typ.Element)
		if !ok {
			return typ, false
		}
		result := typ
		result.Element = &element
		return result, true
	}
	return typ, false
}
