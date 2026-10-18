package sema

// compilerKnownIterator resolves only explicit Iterator[T] conformance. The
// method name Next alone is deliberately insufficient: flowcontrol_for.md
// section 37 forbids naming-convention discovery, and no interface value or
// dynamic-dispatch runtime is introduced here.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §13 "Sec 0.1 iterable categories", §37 "Compiler-known Iterator[T]"
//   - rules/declarations/interfaces.md — §§6, 9.1 canonical conformance.
func (a *Analyzer) compilerKnownIterator(source Type) (Type, Function, Type, bool) {
	concrete := dereferenceType(source)
	if concrete.Kind == GenericType {
		return constrainedIterator(concrete)
	}
	if isIteratorInterface(concrete) {
		// A borrowed view: the element type and Next requirement come from
		// the interface itself (flowcontrol_for.md § 37.1; MD-057).
		if required, ok := compilerKnownInterfaceRequirement(concrete, "Next"); ok {
			return concrete.TypeArgs[0], required, concrete, true
		}
	}
	for _, iface := range concrete.Implements {
		if !iface.Intrinsic || iface.Name != "Iterator" || iface.Kind != InterfaceType || len(iface.TypeArgs) != 1 {
			continue
		}
		element := iface.TypeArgs[0]
		required, ok := compilerKnownInterfaceRequirement(iface, "Next")
		if !ok {
			continue
		}
		for _, method := range a.functions[concrete.Name+"."+required.Name] {
			if !hasCompatibleInterfaceMethod(concrete, iface, []Function{method}, required) {
				continue
			}
			method.CompilerKnownID = required.CompilerKnownID
			return element, method, iface, true
		}
		// Preserve loop binding inference while ordinary conformance reports
		// the missing or mismatched implementation at the implements clause.
		required.ImplTarget = concrete.Name
		return element, required, iface, true
	}
	return Type{}, Function{}, Type{}, false
}

// constrainedIterator resolves iteration over a generic parameter whose
// declared constraints include the compiler-known Iterator[T]: the element
// type and Next requirement come from the constraint, and each
// specialization resolves the concrete Next statically. No interface value
// or dispatch is created.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §37 "Compiler-known `Iterator[T]`"
//   - rules/declarations/generics.md — §15 "Operations available on generic parameters"
//   - rules/declarations/interfaces.md — §9.1 "Compiler-known generic interfaces"
func constrainedIterator(parameter Type) (Type, Function, Type, bool) {
	for _, constraint := range parameter.GenericConstraints {
		iface := constraint.Interface
		if !iface.Intrinsic || iface.Name != "Iterator" || iface.Kind != InterfaceType || len(iface.TypeArgs) != 1 {
			continue
		}
		required, ok := compilerKnownInterfaceRequirement(iface, "Next")
		if !ok {
			continue
		}
		return iface.TypeArgs[0], required, iface, true
	}
	return Type{}, Function{}, Type{}, false
}

// isForCollectionFamily recognizes only canonical compiler-known collection
// identities with their resolved arity. Historical Vec/Set/Map names and
// ordinary source types never gain iteration by spelling alone.
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §13 "Sec 0.1 iterable categories", §§16,18,20–21
func isForCollectionFamily(typ Type) bool {
	if !typ.Intrinsic || typ.Kind != StructType {
		return false
	}
	switch typ.Name {
	case "list", "set":
		return len(typ.TypeArgs) == 1
	case "map":
		return len(typ.TypeArgs) == 2
	case "vector":
		return len(typ.TypeArgs) == 1 && len(typ.ConstArgs) == 1
	default:
		return false
	}
}

// isIteratorInterface reports the compiler-known Iterator[T] interface type
// itself, as opposed to a concrete type conforming to it.
func isIteratorInterface(typ Type) bool {
	return typ.Kind == InterfaceType && typ.Intrinsic && typ.Name == "Iterator" && len(typ.TypeArgs) == 1
}
