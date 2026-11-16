package semantic

import (
	"fmt"

	"sec/internal/sema"
)

// P19 keeps one Arena type across all backing/growth policies. The allocation
// context is a separate compiler-internal capability, never ordinary user data.
// Neither type selects a physical representation, allocator or runtime ABI.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§4,6,84–85,146–148.
const (
	TypeArena             TypeKind = "arena"
	TypeAllocationContext TypeKind = "allocation-context"
	arenaLoweringPackage  uint8    = 19
)

// InternArena interns the single semantic builtin owner type. Backing kind,
// capacity, profile, provider, domain and epoch belong to value/operation facts.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§4,6–10,84.
func (t *TypeTable) InternArena() TypeID {
	return t.Intern(Type{Kind: TypeArena, Name: "Arena"})
}

// InternAllocationContext creates only the internal type identity; it exposes
// no source type or source parameter and supplies no ambient context value.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§51,55–56,85.
func (t *TypeTable) InternAllocationContext() TypeID {
	return t.Intern(Type{Kind: TypeAllocationContext})
}

// internArenaType maps Sema's builtin separately from ordinary empty structs;
// older packages retain their explicit unsupported boundary.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§4,84;
// rules/compiler/semantic_ir.md — unsupported lowerings.
func (b *builder) internArenaType(t sema.Type) (TypeID, error) {
	if b.maxPackage < arenaLoweringPackage {
		return 0, &UnsupportedFeatureError{Feature: "Arena type (P19)", Package: b.maxPackage}
	}
	return b.module.Types.InternArena(), nil
}

// verifyArenaTypes enforces canonical semantic type shapes and prevents the
// internal context from escaping into ordinary type constructors, aggregates,
// local/static storage or function results. Hidden argument production and ABI
// validation remain P19-17/P19-23; no source signature is added by this type.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§84–85,98–99;
// rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md — allocation context type.
func verifyArenaTypes(module *Module) error {
	context := func(id TypeID) bool { t, ok := module.Types.Lookup(id); return ok && t.Kind == TypeAllocationContext }
	for i, t := range module.Types.types {
		if t.Kind == TypeArena || t.Kind == TypeAllocationContext {
			want := Type{Kind: TypeArena, Name: "Arena"}
			if t.Kind == TypeAllocationContext {
				want = Type{Kind: TypeAllocationContext}
			}
			if typeKey(t) != typeKey(want) {
				return fmt.Errorf("non-canonical %s type !%d: backing, layout and source parameters are not type facts", t.Kind, i+1)
			}
		}
		for _, child := range append([]TypeID{t.Base, t.Success, t.Error, t.Underlying, t.Element}, t.TypeArgs...) {
			if context(child) {
				return fmt.Errorf("allocation-context cannot be wrapped or stored by semantic type !%d", i+1)
			}
		}
	}
	for _, definition := range module.Structs {
		for _, field := range definition.Fields {
			if context(field.Type) {
				return fmt.Errorf("allocation-context cannot be an ordinary struct field")
			}
		}
	}
	for _, definition := range module.Unions {
		for _, variant := range definition.Variants {
			if context(variant.Payload) {
				return fmt.Errorf("allocation-context cannot be an ordinary union payload")
			}
			for _, field := range variant.PayloadFields {
				if context(field.Type) {
					return fmt.Errorf("allocation-context cannot be an ordinary union field")
				}
			}
		}
	}
	for _, static := range module.Statics {
		if context(static.Type) {
			return fmt.Errorf("allocation-context cannot have static storage")
		}
	}
	for _, fn := range module.Functions {
		if fn == nil {
			continue
		}
		if context(fn.ReturnType) {
			return fmt.Errorf("allocation-context cannot be an ordinary function result")
		}
		for _, storage := range fn.Storages {
			if context(storage.Type) {
				return fmt.Errorf("allocation-context cannot have local storage")
			}
		}
		for _, parameter := range fn.Parameters {
			if context(parameter.Value.Type) && fn.Extern {
				return fmt.Errorf("allocation-context cannot be an ordinary foreign argument")
			}
		}
	}
	return nil
}
