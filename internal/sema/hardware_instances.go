package sema

import (
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// MD-077 hardware-bound registers. `@address(endpoint) let mut device: T`
// binds the nominal register layout T to an external resource; the binding
// denotes the software implementation instance, typed HardwareInstance[T].
// Its public surface is exactly the members of the mandatory type-owned
// `impl T`; physical fields resolve only inside that impl, and neither the
// instance nor its fields are ordinary memory Places of T.
//
// Rules:
//   - rules/platform/hardware-register-access.md — § 2, § 6 amendments
//   - rules/declarations/registers.md — § 11.7
//   - rules/platform/fixed-address-bindings.md — §§ 2–3 amendments

func hardwareInstanceType(register Type) Type {
	return Type{Name: "HardwareInstance", Kind: HardwareInstanceType, TypeArgs: []Type{register}}
}

// isHardwareInstance reports a hardware instance, directly or through a
// reference to it.
func isHardwareInstance(typ Type) bool {
	if typ.Kind == ReferenceType && typ.Element != nil {
		typ = *typ.Element
	}
	return typ.Kind == HardwareInstanceType
}

// hasPrimaryImpl reports a valid type-owned primary `impl` of register.
func (a *Analyzer) hasPrimaryImpl(register Type) bool {
	for impl := range a.validImplStatements {
		if impl == nil || impl.Target == nil || impl.Extends {
			continue
		}
		if target, ok := a.types[impl.Target.Name]; ok && target.Name == register.Name && target.Module == register.Module {
			return true
		}
	}
	return false
}

// requireHardwareImpl rejects a hardware binding whose register type has no
// type-owned primary impl (MD-077 § 2(1)).
func (a *Analyzer) requireHardwareImpl(register Type, token lexer.Token) bool {
	if a.hasPrimaryImpl(register) {
		return true
	}
	a.addErrorAtTokenWithMetadata(token, "",
		"declare `impl "+typeDisplayName(register)+"` with the properties and methods that expose this register; hardware is reached only through that type-owned API",
		"hardware-bound register type %s has no type-owned impl", typeDisplayName(register))
	return false
}

// publicHardwareMembers names the members `impl T` exposes, for diagnostics.
func (a *Analyzer) publicHardwareMembers(register Type) []string {
	seen := map[string]bool{}
	for name, functions := range a.functions {
		prefix := register.Name + "."
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		for _, function := range functions {
			if function.ImplTarget == register.Name && !function.Static {
				seen[strings.TrimPrefix(name, prefix)] = true
			}
		}
	}
	for _, property := range register.Properties {
		seen[property.Name] = true
	}
	members := make([]string, 0, len(seen))
	for name := range seen {
		members = append(members, name)
	}
	sort.Strings(members)
	return members
}

// rejectHardwareFieldAccess rejects a physical register field reached
// through a hardware instance outside the owning `impl T` (MD-077 § 2(6)).
// Inside that impl the field resolves to the declared logical operation.
func (a *Analyzer) rejectHardwareFieldAccess(member *ast.MemberExpression, objectType Type) bool {
	if !isHardwareInstance(objectType) || member.Property == nil {
		return false
	}
	register := dereferenceType(objectType)
	if _, isField := lookupRegisterFieldInfo(register, member.Property.Value); !isField {
		return false
	}
	if a.currentImplTarget == register.Name {
		return false
	}
	help := "Physical register fields are reached only inside `impl " + typeDisplayName(register) + "`; expose the operation as a property or method there."
	if members := a.publicHardwareMembers(register); len(members) > 0 {
		help = "Use the public API of `impl " + typeDisplayName(register) + "`: " + strings.Join(members, ", ") + "."
	}
	a.addErrorAtTokenWithMetadata(member.Property.Token, "", help,
		"register field %s of hardware instance %s is not public; hardware is reached through the members of impl %s",
		member.Property.Value, member.Object.String(), typeDisplayName(register))
	return true
}

// rejectPhysicalRegisterBorrow rejects `ref` / `ref mut` of physical register
// bits (MD-077 §§ 4(1), 17(2)): a register field path rooted at a hardware
// instance, or at `self` inside the impl of a register type (whose methods
// may run on a hardware instance). Borrowing the instance itself borrows the
// software instance and stays valid.
func (a *Analyzer) rejectPhysicalRegisterBorrow(expr ast.Expression, token lexer.Token) bool {
	var fields []string
	current := expr
	for {
		member, ok := current.(*ast.MemberExpression)
		if !ok || member.Property == nil {
			break
		}
		fields = append([]string{member.Property.Value}, fields...)
		current = member.Object
	}
	root, ok := current.(*ast.Identifier)
	if !ok || len(fields) == 0 {
		return false
	}
	var register Type
	switch {
	case root.Value == "self":
		target, exists := a.types[a.currentImplTarget]
		if !exists || target.Kind != RegisterType {
			return false
		}
		register = target
	default:
		symbol, exists := a.symbols[root.Value]
		if !exists || !isHardwareInstance(symbol.Type) {
			return false
		}
		register = dereferenceType(symbol.Type)
	}
	if _, isField := lookupRegisterFieldInfo(register, fields[0]); !isField {
		return false
	}
	a.addErrorAtTokenWithMetadata(token, "",
		"Physical register bits are not memory; read a detached snapshot or call a method of impl "+typeDisplayName(register)+" instead. Borrowing the hardware instance itself borrows only its software state.",
		"cannot borrow physical register field %s: hardware register bits are not an ordinary memory Place", expr.String())
	return true
}
