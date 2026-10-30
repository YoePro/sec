package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// sharedTypePackParameter identifies the supported pointwise shared signature,
// distinct from a homogeneous variadic parameter or an owned/mutable pack.
// Rules: rules/declarations/functions.md §38A(2–4); rules/declarations/generics.md §16A.
func sharedTypePackParameter(function Function) (FunctionParameter, bool) {
	parameter, ok := functionVariadicParameter(function)
	return parameter, ok && function.GenericTypePack != "" && parameter.Type.Kind == ReferenceType && !parameter.Type.ReferenceMutable && isDeclaredTypePackMode(parameter.Type, function.GenericTypePack)
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

// inferSharedTypePackCall infers a scalar generic prefix and the trailing ordered
// sequence. Shared argument holders infer their referent as ordinary ref T does;
// actual borrowing and compatibility stay with the ordinary selected-call checks.
// No type erasure, common element type, return-inferred pack sequence or runtime pack
// is introduced. The body may observe Len but not inspect individual elements.
// Rules: rules/declarations/functions.md §38A(3–4),(9); rules/memory/borrowing.md §15.2;
// rules/declarations/generics.md §20 (explicit scalar prefix).
func (a *Analyzer) inferSharedTypePackCall(function Function, refs []*ast.TypeReference, arguments []Type, expected Type) (Function, bool) {
	if _, ok := sharedTypePackParameter(function); !ok || function.ImplTarget != "" || !functionAcceptsArgumentCount(function, len(arguments)) {
		return Function{}, false
	}
	if len(refs) > len(function.GenericParameters)-1 {
		return Function{}, false
	}
	substitution := map[string]Type{}
	fixed := map[string]struct{}{}
	for i, ref := range refs {
		typ, ok := a.resolveType(ref)
		if !ok {
			return Function{}, false
		}
		name := function.GenericParameters[i]
		if name == function.GenericTypePack {
			return Function{}, false
		}
		substitution[name] = typ
		fixed[name] = struct{}{}
	}
	count := functionFixedParameterCount(function)
	for i, argument := range arguments[:count] {
		pattern := function.Parameters[i].Type
		if pattern.Kind == ReferenceType && pattern.Element != nil && argument.Kind != ReferenceType {
			pattern = *pattern.Element
		}
		if !inferGenericTypeSubstitutionWithFixed(pattern, argument, substitution, fixed) {
			return Function{}, false
		}
	}
	if expected.Kind != "" && expected.Kind != InvalidType {
		if !inferGenericTypeSubstitutionWithFixed(function.ReturnType, expected, substitution, fixed) {
			return Function{}, false
		}
	}
	sequence := make([]Type, 0, len(arguments)-count)
	for _, typ := range arguments[count:] {
		if typ.Kind == ReferenceType && typ.Element != nil {
			typ = *typ.Element
		}
		if typ.Kind == InvalidType || typ.Kind == GenericTypePackType || typ.Kind == VariadicPackType {
			return Function{}, false
		}
		sequence = append(sequence, typ)
	}
	substitution[function.GenericTypePack] = Type{Name: function.GenericTypePack, Kind: GenericTypePackType, TypeArgs: sequence}
	for _, name := range function.GenericParameters {
		if _, ok := substitution[name]; !ok {
			return Function{}, false
		}
	}
	return a.instantiateGenericFunction(function, substitution), true
}

// supportedSharedTypePackBody permits ordinary checking of unused packs and
// read-only Len observations, whose uint type does not depend on element types.
// Element access, loops, forwarding and capture still require pack expansion.
// Rules: rules/declarations/functions.md §38A(4–8), especially (5)(1–2);
// rules/declarations/generics.md §16A.
func (a *Analyzer) supportedSharedTypePackBody(fn *ast.FunctionDeclaration) bool {
	if fn == nil || a.currentImplTarget != "" || fn.Body == nil || len(fn.Parameters) == 0 {
		return false
	}
	parameter := fn.Parameters[len(fn.Parameters)-1]
	if parameter == nil || parameter.Name == nil || !parameter.Variadic || parameter.Type == nil || !parameter.Type.Ref || parameter.Type.MutableRef {
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
	ast.Visit(fn.Body, func(node any) bool {
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
		if identifier, ok := node.(*ast.Identifier); ok && (identifier.Value == parameter.Name.Value || identifier.Value == pack) && !selectors[identifier] && !lengthReceivers[identifier] {
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

// rejectUnavailableTypePackCall refuses static spread transport whose borrowed
// per-element facts are not represented yet, before ownership/borrow commitment.
// Runtime-length heterogeneous spread is excluded by functionAcceptsCallArguments.
// Rules: rules/declarations/functions.md §38A(4),(6–7); rules/tooling/diagnostics.md §24(4h).
func (a *Analyzer) rejectUnavailableTypePackCall(function Function, call *ast.CallExpression, prepared []bool) bool {
	if function.GenericTypePack == "" {
		return false
	}
	for i, arg := range call.Arguments {
		if _, explicit := explicitMoveArgument(arg); explicit {
			continue
		} // ordinary ownership validation reports the forbidden move
		if i < functionFixedParameterCount(function) {
			continue
		}
		if typ, typed := a.ResolvedTypeOf(arg); typed && typ.Kind != ReferenceType {
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

// sharedTypePackHasFixedAlternative leaves an otherwise matching fixed
// signature in the ordinary overload competition. Expected-result inference
// must not preselect a variadic generic while skipping that candidate.
// Rules: rules/declarations/functions.md §38A(4)(5),§37; rules/declarations/generics.md §29.
func (a *Analyzer) sharedTypePackHasFixedAlternative(functions []Function, types []Type, args []ast.Expression, runtime []bool) bool {
	hasPack := false
	for _, function := range functions {
		hasPack = hasPack || function.GenericTypePack != ""
	}
	if !hasPack {
		return false
	}
	for _, function := range functions {
		if len(function.GenericParameters) > 0 || !functionAcceptsCallArguments(function, len(types), runtime) {
			continue
		}
		if _, variadic := functionVariadicParameter(function); variadic {
			continue
		}
		match := true
		for i, typ := range types {
			parameter, ok := functionParameterForArgument(function, i)
			if !ok || !a.canPassCallArgumentUnrecorded(parameter.Type, typ, args[i]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// rejectUnavailableTypePackOverloads keeps competing pack templates behind an
// explicit boundary until argument-only overload selection is implemented for
// them. In particular, expected return types must never pick one pack overload.
// Rules: rules/declarations/functions.md §38A(4)(5),(10); rules/declarations/generics.md §29.
func (a *Analyzer) rejectUnavailableTypePackOverloads(functions []Function, call *ast.CallExpression) bool {
	count := 0
	for _, function := range functions {
		if function.GenericTypePack != "" {
			count++
		}
	}
	if count < 2 {
		return false
	}
	a.addErrorAtTokenWithMetadata(call.Token, diagnostics.GenericTypePackExpansionUnavailable, "Use a single pack template until argument-only selection between heterogeneous pack overloads is implemented.", "compiler capability unavailable: overload selection between heterogeneous type-pack templates")
	return true
}
