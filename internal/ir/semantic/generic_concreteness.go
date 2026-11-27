package semantic

import "fmt"

// verifyConcreteTypes closes the executable type table over the represented
// concrete type vocabulary and every nested type edge. Templates and unresolved
// parameters have no executable TypeKind: accepting an arbitrary kind would let
// malformed or independently loaded IR bypass the builder's substitution checks.
// Checking every table entry once also covers phantom arguments and recursive
// reference graphs without recursively expanding nominal storage. Names are not
// evidence of genericity, and target-sized types need no physical layout here.
//
// Rules: rules/compiler/generics_lowering.md — §§4–5,7–8;
// rules/compiler/monomorphization.md — §13.3;
// rules/compiler/semantic_ir.md — §20.
func verifyConcreteTypes(table *TypeTable) error {
	for index, typ := range table.types {
		id := TypeID(index + 1)
		switch typ.Kind {
		case TypeVoid, TypeNever, TypeBool, TypeByte, TypeChar, TypeRune,
			TypeString, TypeDecimal, TypeDecimal128, TypeInt, TypeUint, TypeFloat,
			TypeNamed, TypeArithmeticFailureReason, TypeCoreError, TypeResult,
			TypeEnum, TypeUnion, TypeStruct, TypeArray, TypeRegister, TypePlace,
			TypeReference, TypeRawPointer, TypeSharedSlice, TypeMutableSlice,
			TypeDynamicArray, TypeAtomic, TypeArena, TypeAllocationContext,
			TypeUnitDimension, TypeStringView, TypeHardwareInstance:
		default:
			return fmt.Errorf("generic concreteness: type !%d (%q) has unresolved or unrepresented kind %q", id, typ.Name, typ.Kind)
		}
		// All entries are visited, so an edge resolving to an entry establishes
		// closure without requiring physical size, alignment, ABI or symbols.
		for _, edge := range []struct {
			name string
			id   TypeID
		}{
			{"base", typ.Base}, {"success", typ.Success}, {"error", typ.Error},
			{"underlying", typ.Underlying}, {"element", typ.Element},
		} {
			if edge.id != 0 {
				if _, ok := table.Lookup(edge.id); !ok {
					return fmt.Errorf("generic concreteness: type !%d (%q) has invalid %s type !%d (unresolved)", id, typ.Name, edge.name, edge.id)
				}
			}
		}
		for position, argument := range typ.TypeArgs {
			if _, ok := table.Lookup(argument); !ok {
				return fmt.Errorf("generic concreteness: type !%d (%q) has invalid type argument !%d at position %d", id, typ.Name, argument, position)
			}
		}
	}
	return nil
}
