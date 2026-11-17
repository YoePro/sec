package sema

import "sort"

// linkGeneratedFieldCleanup adds the compiler-generated cleanup that runs
// after a custom `free` body to the canonical call graph: after the body, the
// remaining owned fields are destroyed in reverse declaration order, and each
// field (or element or variant payload) whose type defines its own `free` runs
// that body. The edge is synchronous and compiler-generated, so verified
// guarantees on `free` (@noPanic, @noAlloc, @noBlock) see panic-capable,
// allocating or blocking field cleanup and an early `return` from the body
// cannot evade it. A nested type with its own `free` is linked to that body
// only; its own fields are linked from it.
//
// Rules:
//   - rules/memory/destruction.md — §§ 7.2, 15.4, 15.5 (custom free, then remaining fields)
//   - rules/errors/panic.md — § 21 "@noPanic" (panic-capable cleanup)
//   - rules/corrections/applied/md030-no-panic-attribute-placement-correction-20261017.md — §§ 9(4), 9(6)
//   - rules/analysis/call_graph.md — "Callable node", compiler-generated edges
func (a *Analyzer) linkGeneratedFieldCleanup() {
	if a.callGraph == nil {
		return
	}
	names := make([]string, 0, len(a.customFreeDeclarations))
	for name := range a.customFreeDeclarations {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		free, ok := a.customFreeFunction(name)
		if !ok {
			continue
		}
		owner, ok := a.types[name]
		if !ok || owner.Kind != StructType {
			continue
		}
		caller := callableID(free)
		for index := len(owner.Fields) - 1; index >= 0; index-- {
			field := owner.Fields[index]
			source := field.Token
			if source.Line <= 0 {
				source = free.Token
			}
			for _, owned := range nearestCustomFreeTypes(field.Type, map[string]bool{}) {
				if target, ok := a.customFreeFunction(owned); ok {
					a.callGraph.addCall(caller, target, source, CallDispatchGenerated, CallExecutionSynchronous)
				}
			}
		}
	}
}

// customFreeFunction returns the resolved lifecycle `free` of a type.
func (a *Analyzer) customFreeFunction(name string) (Function, bool) {
	token, declared := a.customFreeDeclarations[name]
	if !declared {
		return Function{}, false
	}
	for _, function := range a.functions[freeFunctionName(name)] {
		if function.CustomFree && sameSourceToken(function.Token, token) {
			return function, true
		}
	}
	return Function{}, false
}

// nearestCustomFreeTypes lists the type names with a custom `free` that the
// destruction of typ runs directly: typ itself, or the nearest such types in
// its owned fields, elements and variant payloads. References own nothing.
func nearestCustomFreeTypes(typ Type, visiting map[string]bool) []string {
	if typ.CustomFree {
		return []string{typ.Name}
	}
	if typ.Kind == ReferenceType {
		return nil
	}
	if key := typeDestructionKey(typ); key != "" {
		if visiting[key] {
			return nil
		}
		visiting[key] = true
		defer delete(visiting, key)
	}
	var out []string
	add := func(inner Type) { out = append(out, nearestCustomFreeTypes(inner, visiting)...) }
	for index := len(typ.Fields) - 1; index >= 0; index-- {
		add(typ.Fields[index].Type)
	}
	for _, variant := range typ.UnionVariants {
		if variant.Payload != nil {
			add(*variant.Payload)
		}
		for index := len(variant.PayloadFields) - 1; index >= 0; index-- {
			add(variant.PayloadFields[index].Type)
		}
	}
	if typ.Element != nil {
		add(*typ.Element)
	}
	for _, argument := range typ.TypeArgs {
		add(argument)
	}
	return out
}
