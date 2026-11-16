package sema

import (
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// prepareMethodGenericCandidates fixes the generic owner's arguments before
// inferring a method's independent parameter list. No expression is evaluated
// here; normal argument preparation, overload and ownership checks still apply.
// Rules: rules/declarations/generics.md — §§9,18,20,25;
// rules/compiler/monomorphization.md — §§8.2,9.3.
func (a *Analyzer) prepareMethodGenericCandidates(call *ast.CallExpression, functions []Function, receiver methodReceiverInfo, instance bool) ([]Function, bool) {
	owner := dereferenceType(receiver.Type)
	if !instance {
		member, ok := call.Callee.(*ast.MemberExpression)
		if !ok || len(member.OwnerGenericArguments) == 0 {
			return functions, true
		}
		object := member.Object
		if indexed, ok := object.(*ast.IndexExpression); ok {
			object = indexed.Left
		}
		name, ok := typePathFromExpression(object)
		if !ok {
			return functions, true
		}
		var resolved bool
		owner, resolved = a.resolveType(&ast.TypeReference{Token: expressionToken(object), Name: name, TypeArgs: member.OwnerGenericArguments})
		if !resolved {
			return nil, false
		}
		static := make([]Function, 0, len(functions))
		for _, function := range functions {
			if function.Static {
				static = append(static, function)
			}
		}
		if len(static) == 0 {
			a.addErrorAtTokenWithMetadata(member.Property.Token, diagnostics.StaticInstanceReceiverRequired,
				"Call the instance method through a value of the owning type; generic type arguments do not provide an instance receiver.",
				"method %s requires an instance receiver", member.Property.Value)
			return nil, false
		}
		functions = static
	}
	if instance && len(owner.TypeArgs) == 0 && len(owner.GenericParameters) > 0 {
		for _, name := range owner.GenericParameters {
			typ, ok := a.genericTypes[name]
			if !ok {
				return functions, true
			}
			owner.TypeArgs = append(owner.TypeArgs, typ)
		}
	}
	if len(owner.TypeArgs) == 0 {
		return functions, true
	}
	result := make([]Function, len(functions))
	for index, function := range functions {
		if function.ImplTarget != owner.Name || function.GenericTypePack != "" || len(function.GenericParameters) == 0 {
			if !instance && function.Static {
				function = a.instantiateGenericImplMethod(function, methodReceiverInfo{Type: owner})
			}
			result[index] = function
			continue
		}
		target, ok := a.types[function.ImplTarget]
		if !ok || len(target.GenericParameters) != len(owner.TypeArgs) {
			result[index] = function
			continue
		}
		bound := genericInferenceBindings(function)
		for i, name := range target.GenericParameters {
			bound[name] = owner.TypeArgs[i]
		}
		function.GenericSubstitution = bound
		function.Parameters = append([]FunctionParameter(nil), function.Parameters...)
		for i := range function.Parameters {
			function.Parameters[i].Type = substituteGenericType(function.Parameters[i].Type, bound)
		}
		function.ReturnType = substituteGenericType(function.ReturnType, bound)
		function.GenericConstraints = append([]GenericConstraint(nil), function.GenericConstraints...)
		for i := range function.GenericConstraints {
			function.GenericConstraints[i].Interface = substituteGenericType(function.GenericConstraints[i].Interface, bound)
		}
		result[index] = function
	}
	return result, true
}

// genericInferenceBindings detaches arguments already fixed by the receiver;
// explicit method prefixes and inference may only fill the method parameters.
// Rules: rules/declarations/generics.md — §§9,18,20.
func genericInferenceBindings(function Function) map[string]Type {
	bindings := make(map[string]Type, len(function.GenericSubstitution))
	for name, typ := range function.GenericSubstitution {
		bindings[name] = typ
	}
	return bindings
}

// genericBoundArgumentsKey separates receiver-bound environments in the local
// signature cache. The exported identity separately preserves declaration and
// ordered owner/callable argument roles.
// Rules: rules/declarations/generics.md — §§18,25;
// rules/compiler/monomorphization.md — §§8.2,9.3.
func genericBoundArgumentsKey(function Function, substitution map[string]Type) string {
	if len(substitution) == 0 {
		return ""
	}
	methodParameters := map[string]bool{}
	for _, name := range function.GenericParameters {
		methodParameters[name] = true
	}
	names := make([]string, 0, len(substitution))
	for name := range substitution {
		if !methodParameters[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+canonicalTypeIdentity(substitution[name]))
	}
	return strings.Join(parts, ";")
}
