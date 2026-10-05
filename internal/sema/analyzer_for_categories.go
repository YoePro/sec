package sema

// compilerKnownIterator resolves only explicit Iterator[T] conformance. The
// method name Next alone is deliberately insufficient: flowcontrol_for.md
// section 37 forbids naming-convention discovery, and no interface value or
// dynamic-dispatch runtime is introduced here.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §13 "Sec 0.1 iterable categories", §37 "Compiler-known Iterator[T]"
func (a *Analyzer) compilerKnownIterator(source Type) (Type, Function, Type, bool) {
	concrete := dereferenceType(source)
	for _, iface := range concrete.Implements {
		if !iface.Intrinsic || iface.Name != "Iterator" || iface.Kind != InterfaceType || len(iface.TypeArgs) != 1 {
			continue
		}
		element := iface.TypeArgs[0]
		for _, method := range a.functions[concrete.Name+".Next"] {
			if method.Static || len(explicitInterfaceComparableParameters(method.Parameters)) != 0 {
				continue
			}
			if method.ReturnType.Name != "Option" || len(method.ReturnType.TypeArgs) != 1 || !sameConcreteType(method.ReturnType.TypeArgs[0], element) {
				continue
			}
			method.CompilerKnownID = "CKM-ITERATOR-NEXT"
			return element, method, iface, true
		}
		// Preserve useful loop binding inference while ordinary interface
		// conformance emits the canonical missing/signature diagnostic.
		required := Function{Name: "Next", ImplTarget: concrete.Name, CompilerKnownID: "CKM-ITERATOR-NEXT", ReceiverMutable: true, ReturnType: Type{Name: "Option", Kind: UnionType, TypeArgs: []Type{element}}}
		return element, required, iface, true
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
