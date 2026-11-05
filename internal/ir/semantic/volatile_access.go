package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// Volatile accesses are ordered physical effects, not copies or moves from an
// owned Place. Their results are ordinary detached snapshots. Physical planning
// is deliberately unverified: downstream emission must reject these operations
// until it can preserve the region, transaction and ordering contracts.
// Rules: rules/memory/copy_move.md §§19.1–19.3;
// rules/platform/volatile.md §§3,9–11; rules/compiler/semantic_ir.md §§47,85.
const (
	OpVolatileRead         OpKind = "volatile.read"
	OpVolatileWrite        OpKind = "volatile.write"
	VolatilePlanUnverified        = "unverified"
)

type VolatileAccessFacts struct {
	MemberID              string
	PhysicalPlan          string
	Authorization         string
	AuthorizationLocation Location
}

// buildVolatileCall consumes exact compiler-known member identity and retains
// each source access once, in evaluation order. Ordinary calls with similar
// names never enter this path. No borrow or ownership is acquired from bits.
// Rules: rules/platform/volatile.md §§3,9–11;
// rules/memory/copy_move.md §§19.1–19.3,30.6; rules/compiler/semantic_ir.md §47.
func (fb *functionBuilder) buildVolatileCall(call *ast.CallExpression) (builtValue, bool, error) {
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return builtValue{}, false, nil
	}
	site := member.Property.Token
	known, ok := fb.owner.analyzer.CompilerKnownMemberAt(site.File, site.Line, site.Column)
	if !ok || (known.ID != "CKM-RAWPTR-VOLATILE-READ" && known.ID != "CKM-RAWPTR-VOLATILE-WRITE") {
		return builtValue{}, false, nil
	}
	fail := func(reason string) (builtValue, bool, error) { return builtValue{}, true, fb.unsupported(reason, site) }
	if fb.owner.maxPackage < ownershipLoweringPackage {
		return fail("volatile access ownership boundary")
	}
	source, ok := fb.owner.analyzer.ResolvedTypeOf(member.Object)
	if !ok {
		return fail("volatile access without a resolved raw pointer")
	}
	sourceID, err := fb.owner.internType(source)
	if err != nil {
		return builtValue{}, true, err
	}
	var pointer builtValue
	if source.Kind == sema.ReferenceType {
		pointer, err = fb.lowerValueDereference(&ast.PrefixExpression{Token: site, Operator: "*", Right: member.Object})
	} else {
		pointer, err = fb.buildExpr(member.Object, sourceID)
	}
	if err != nil {
		return builtValue{}, true, err
	}

	carrier, ok := rawPointerElement(fb.owner.module.Types, pointer.typ)
	if !ok {
		return fail("volatile access receiver is not a raw pointer value")
	}
	if !volatileSnapshotType(fb.owner.module, carrier, map[TypeID]bool{}) {
		return fail("volatile decoding of managed or unsupported value representation")
	}
	write := known.ID == "CKM-RAWPTR-VOLATILE-WRITE"
	want := 0
	if write {
		want = 1
	}
	if len(call.Arguments) != want {
		return fail("volatile access arity")
	}
	authorization := ""
	var authorizedAt Location
	switch {
	case call.HasUnsafePrefix():
		authorization = "call-prefix"
		authorizedAt = location(call.Unsafe)
	case len(fb.unsafeScopes) > 0:
		authorization = "unsafe-block"
		authorizedAt = fb.unsafeScopes[len(fb.unsafeScopes)-1]
	case fb.fn.Unsafe:
		authorization = "unsafe-function"
		authorizedAt = fb.fn.Location
	default:
		return fail("volatile access without retained unsafe authorization")
	}
	operation := Operation{Kind: OpVolatileRead, Operands: []ValueID{pointer.id}, Volatile: &VolatileAccessFacts{MemberID: known.ID, PhysicalPlan: VolatilePlanUnverified, Authorization: authorization, AuthorizationLocation: authorizedAt}, Location: location(site)}
	if call.HasUnsafePrefix() {
		marker := location(call.Unsafe)
		operation.UnsafeAuthorization = &marker
	}
	if !write {
		return fb.result(operation, carrier), true, nil
	}
	value, err := fb.buildExpr(call.Arguments[0], carrier)
	if err != nil {
		return builtValue{}, true, err
	}
	operation.Kind = OpVolatileWrite
	operation.Operands = append(operation.Operands, value.id)
	fb.emit(operation)
	return builtValue{}, true, nil
}

// rawPointerElement preserves nominal address wrappers without treating a safe
// reference or an ordinary Place as physical storage authority.
// Rules: rules/memory/raw_pointers.md §§2,4,23; rules/platform/volatile.md §9.
func rawPointerElement(types *TypeTable, id TypeID) (TypeID, bool) {
	for remaining := len(types.types); remaining > 0; remaining-- {
		typ, ok := types.Lookup(id)
		if !ok {
			return 0, false
		}
		if typ.Kind == TypeRawPointer {
			return typ.Element, true
		}
		if typ.Kind != TypeNamed {
			return 0, false
		}
		id = typ.Base
	}
	return 0, false
}

// volatileSnapshotType admits only already-represented bit values; managed
// owners, safe references and strings cannot be manufactured by decoding bits.
// This is a representation boundary, not a physical-region capability proof.
// Rules: rules/memory/copy_move.md §19.3; rules/platform/volatile.md §§9–11.
func volatileSnapshotType(module *Module, id TypeID, visiting map[TypeID]bool) bool {
	if visiting[id] {
		return false
	}
	visiting[id] = true
	defer delete(visiting, id)
	typ, ok := module.Types.Lookup(id)
	if !ok {
		return false
	}
	switch typ.Kind {
	case TypeBool, TypeByte, TypeChar, TypeRune, TypeInt, TypeUint, TypeFloat, TypeRawPointer, TypeRegister:
		return true
	case TypeNamed:
		return volatileSnapshotType(module, typ.Base, visiting)
	case TypeEnum:
		return volatileSnapshotType(module, typ.Underlying, visiting)
	case TypeArray:
		return volatileSnapshotType(module, typ.Element, visiting)
	case TypeStruct:
		definition, ok := structDefinition(module, id)
		if !ok || definition.CopyClassification != "trivial" || !definition.TriviallyDestructible {
			return false
		}
		for _, field := range definition.Fields {
			if !volatileSnapshotType(module, field.Type, visiting) {
				return false
			}
		}
		return true
	}
	return false
}

// verifyVolatileAccess rejects ownership/borrow metadata and mismatched pointee
// snapshots. The only admitted plan state is unverified; a verifier must not
// let invented attributes claim physical safety or synchronization.
// Rules: rules/memory/copy_move.md §19; rules/platform/volatile.md §§9–12;
// rules/compiler/semantic_ir.md §§47,85.
func verifyVolatileAccess(module *Module, op Operation, values map[ValueID]Value) error {
	if op.Volatile == nil || op.Volatile.PhysicalPlan != VolatilePlanUnverified || op.Ownership != nil || op.Reference != nil || op.Storage != 0 || len(op.Successors) != 0 {
		return fmt.Errorf("invalid volatile access contract")
	}
	if op.Volatile.Authorization != "call-prefix" && op.Volatile.Authorization != "unsafe-block" && op.Volatile.Authorization != "unsafe-function" {
		return fmt.Errorf("volatile access lacks unsafe authorization provenance")
	}
	if op.Volatile.AuthorizationLocation.Line <= 0 || op.Volatile.AuthorizationLocation.Column <= 0 {
		return fmt.Errorf("volatile access lacks unsafe authorization location")
	}
	if op.AtomicOrder != "" || len(op.ArgumentActions) != 0 || op.ReferenceGuard != 0 {
		return fmt.Errorf("volatile access carries synchronization or ownership actions")
	}
	write := op.Kind == OpVolatileWrite
	operands, results, id := 1, 1, "CKM-RAWPTR-VOLATILE-READ"
	if write {
		operands, results, id = 2, 0, "CKM-RAWPTR-VOLATILE-WRITE"
	}
	if len(op.Operands) != operands || len(op.Results) != results || op.Volatile.MemberID != id {
		return fmt.Errorf("invalid volatile access shape or identity")
	}
	element, ok := rawPointerElement(module.Types, values[op.Operands[0]].Type)
	if !ok || !volatileSnapshotType(module, element, map[TypeID]bool{}) {
		return fmt.Errorf("invalid volatile access pointer or managed snapshot")
	}
	if write {
		if values[op.Operands[1]].Type != element {
			return fmt.Errorf("volatile write value differs from pointee")
		}
	} else if op.Results[0].Type != element {
		return fmt.Errorf("volatile read snapshot differs from pointee")
	}
	return nil
}
