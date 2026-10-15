package semantic

import (
	"fmt"

	"sec/internal/sema"
)

// TypeRawPointer is an unchecked address value, distinct from TypeReference
// and TypePlace. Element preserves the pointee interpretation (including
// void); it establishes no ownership, non-nullness, bounds, lifetime, generation
// or dereferenceability facts. Target representation and access facts are
// separate lowering work, never inferred from this type.
// Rules: rules/memory/raw_pointers.md §§ 2, 4, 23, 32;
// rules/compiler/semantic_ir.md §§ 10(6), 47.
const TypeRawPointer TypeKind = "raw-pointer"

func (b *builder) internRawPointerType(t sema.Type) (TypeID, error) {
	if t.Named || t.Declared {
		return b.internNamedScalar(t)
	}
	if len(t.TypeArgs) != 1 {
		return 0, &UnsupportedFeatureError{Feature: "raw-pointer type without one pointee", Package: b.maxPackage}
	}
	element, err := b.internType(t.TypeArgs[0])
	if err != nil {
		return 0, err
	}
	return b.module.Types.Intern(Type{Kind: TypeRawPointer, Element: element}), nil
}

func isRawPointerType(types *TypeTable, id TypeID) bool {
	// Named chains are validated separately; bound traversal also handles
	// malformed cyclic tables passed to the public verifier.
	if types == nil {
		return false
	}
	for remaining := len(types.types); remaining > 0; remaining-- {
		typ, ok := types.Lookup(id)
		if !ok {
			return false
		}
		if typ.Kind != TypeNamed {
			return typ.Kind == TypeRawPointer
		}
		id = typ.Base
	}
	return false
}

func verifyRawPointerType(types *TypeTable, id TypeID, typ Type) error {
	if typ.Kind != TypeRawPointer {
		return nil
	}
	element, ok := types.Lookup(typ.Element)
	if !ok || element.Kind == TypePlace || element.Kind == TypeNever {
		return fmt.Errorf("raw pointer !%d has invalid pointee type !%d", id, typ.Element)
	}
	if typ.Mutable || typ.Base != 0 || typ.Success != 0 || typ.Error != 0 || typ.Underlying != 0 || typ.Length != "" || len(typ.TypeArgs) != 0 || typ.Signed || typ.BitWidth != 0 || typ.TargetSize {
		return fmt.Errorf("raw pointer !%d carries incompatible type facts", id)
	}
	return nil
}
