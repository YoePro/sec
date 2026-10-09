package sema

import "strings"

// CompilerKnownInterfaces is the canonical, caller-owned interface registry.
// It supplies the same generic requirement metadata as source interfaces, so
// declaration conformance, generic constraints and iteration share the ordinary
// substitution/matching path. Each call returns independent mutable slices.
// Rules: rules/declarations/interfaces.md — §9.1 Compiler-known generic interfaces;
// rules/control-flow/flowcontrol_for.md — §37 Compiler-known Iterator[T];
// rules/compiler/compiler_known_members.md — Iteration protocol.
func CompilerKnownInterfaces() []Type {
	return []Type{{
		Name: "Iterator", Kind: InterfaceType, Intrinsic: true,
		GenericParameters: []string{"T"},
		InterfaceMethods: []Function{{
			Name: "Next", CompilerKnownID: "CKM-ITERATOR-NEXT", ReceiverMutable: true,
			ReturnType: Type{Name: "Option", Kind: UnionType, TypeArgs: []Type{{Name: "T", Kind: GenericType}}},
		}},
	}}
}

// compilerKnownInterfaceRequirement specializes only a registered intrinsic
// interface with the exact arity. Source spelling alone never grants a compiler
// interface identity; the requirement's method ID remains registry-owned.
// Rules: rules/declarations/interfaces.md — §§6, 9.1;
// rules/control-flow/flowcontrol_for.md — §37.
func compilerKnownInterfaceRequirement(iface Type, member string) (Function, bool) {
	if !iface.Intrinsic || iface.Kind != InterfaceType {
		return Function{}, false
	}
	for _, template := range CompilerKnownInterfaces() {
		if template.Name != iface.Name || len(template.GenericParameters) != len(iface.TypeArgs) {
			continue
		}
		substitution := map[string]Type{}
		for index, parameter := range template.GenericParameters {
			substitution[parameter] = iface.TypeArgs[index]
		}
		for _, method := range template.InterfaceMethods {
			if method.Name == member {
				method.ReturnType = substituteGenericType(method.ReturnType, substitution)
				for index := range method.Parameters {
					method.Parameters[index].Type = substituteGenericType(method.Parameters[index].Type, substitution)
				}
				return method, true
			}
		}
	}
	return Function{}, false
}

// compilerKnownInterfaceMember projects a registry-owned interface requirement
// into the shared member presentation model without granting authority by name.
// Rules: rules/declarations/interfaces.md — §9.1;
// rules/compiler/compiler_known_members.md — Built-in type member lookup, LSP.
func compilerKnownInterfaceMember(method Function) (CompilerKnownMember, bool) {
	for _, iface := range CompilerKnownInterfaces() {
		for _, required := range iface.InterfaceMethods {
			if method.CompilerKnownID != required.CompilerKnownID {
				continue
			}
			parameters := make([]string, 0, len(required.Parameters))
			for _, parameter := range required.Parameters {
				parameters = append(parameters, parameter.Name+": "+typeDisplayName(parameter.Type))
			}
			receiver := iface.Name + "[" + strings.Join(iface.GenericParameters, ", ") + "]"
			if required.ReceiverMutable {
				receiver += " (mutable)"
			}
			return CompilerKnownMember{
				ID: required.CompilerKnownID, Name: required.Name, Kind: CompilerKnownMethod,
				Result: method.ReturnType, Signature: "fn " + required.Name + "(" + strings.Join(parameters, ", ") + ") " + typeDisplayName(required.ReturnType),
				Category:          CompilerKnownOperation,
				Rule:              "rules/declarations/interfaces.md — §9.1; rules/control-flow/flowcontrol_for.md — §37",
				Receiver:          receiver,
				Documentation:     "The mutable pull-iteration requirement; returns the next owned element or None.",
				TargetRestriction: "none; compile-time interface contract",
			}, true
		}
	}
	return CompilerKnownMember{}, false
}

// CompilerKnownIntrinsicSymbolByID recovers interface-requirement and value
// presentation directly from the registries, including after an editor restart.
// Receiver-dependent members continue to use their resolved member facts.
// Rules: rules/compiler/compiler_known_members.md — Stable member identity, LSP, Synthetic definitions;
// rules/declarations/interfaces.md — §9.1.
func CompilerKnownIntrinsicSymbolByID(id string) (CompilerKnownMember, bool) {
	for _, iface := range CompilerKnownInterfaces() {
		for _, method := range iface.InterfaceMethods {
			if method.CompilerKnownID == id {
				return compilerKnownInterfaceMember(method)
			}
		}
	}
	for _, value := range CompilerKnownValues() {
		if value.ID == id {
			return compilerKnownValuePresentation(value), true
		}
	}
	return CompilerKnownMember{}, false
}
