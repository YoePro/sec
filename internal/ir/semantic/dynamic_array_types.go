package semantic

import "fmt"

// TypeDynamicArray is the owning dynamic array `T[]` (SEC-MLIR Package 18
// § 52): distinct from fixed arrays and slices, with its element as the only
// type parameter. Length, capacity and allocation are value facts, never type
// parameters. It is move-only for every element type (§ 5) and always has a
// non-trivial destruction plan, because a value may own backing reclamation
// (§ 54).
const TypeDynamicArray TypeKind = "dynamic-array"

// dynamicArrayLoweringPackage is the SEC-MLIR package that introduces owning
// dynamic arrays.
const dynamicArrayLoweringPackage = 18

// InternDynamicArray interns `T[]` by element identity only.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 4, 5, 52.
func (table *TypeTable) InternDynamicArray(element TypeID) (TypeID, error) {
	typ := Type{Kind: TypeDynamicArray, Element: element}
	if err := verifyDynamicArrayType(table, 0, typ); err != nil {
		return 0, err
	}
	return table.Intern(typ), nil
}

// verifyDynamicArrayType validates the canonical type: a known element that
// is a value type (not void or never) and no fixed length.
func verifyDynamicArrayType(table *TypeTable, id TypeID, typ Type) error {
	if _, ok := table.Lookup(typ.Element); !ok {
		return fmt.Errorf("dynamic array !%d has invalid element type !%d", id, typ.Element)
	}
	if typ.Length != "" {
		return fmt.Errorf("dynamic array !%d carries a length; length is a value fact, not a type parameter", id)
	}
	if typeHasKind(table, typ.Element, TypeVoid, TypeNever) {
		return fmt.Errorf("dynamic array !%d has unsupported element type", id)
	}
	return nil
}
