package semantic

import "fmt"

// Verify canonical stored-field identities and recursive slice copy restrictions.
// Rules: rules/compiler/semantic_ir.md — §20; rules/memory/copy_move.md — §§4.1,4.4;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §114.
func verifyStructDefinitions(module *Module) error {
	seen := map[TypeID]bool{}
	for _, definition := range module.Structs {
		typ, ok := module.Types.Lookup(definition.TypeID)
		if !ok || typ.Kind != TypeStruct || typ.Identity == "" || definition.SymbolID != SymbolID(typ.Identity) || definition.Name != typ.Name || seen[definition.TypeID] {
			return fmt.Errorf("invalid struct definition !%d", definition.TypeID)
		}
		seen[definition.TypeID] = true
		if containsOwnedMutableSlice(module, definition.TypeID) && definition.CopyClassification != "move-only" && definition.CopyClassification != "non-copyable" {
			return fmt.Errorf("struct !%d containing a mutable slice must be move-only or non-copyable", definition.TypeID)
		}
		if !sameTypeIDs(definition.TypeArguments, typ.TypeArgs) || !validCopyClassification(definition.CopyClassification) {
			return fmt.Errorf("invalid struct metadata !%d", definition.TypeID)
		}
		names := map[string]bool{}
		for index, field := range definition.Fields {
			if field.ID != StructFieldID(index) || field.Name == "" || names[field.Name] {
				return fmt.Errorf("invalid struct field in !%d", definition.TypeID)
			}
			if _, ok := module.Types.Lookup(field.Type); !ok {
				return fmt.Errorf("struct field %s has invalid type", field.Name)
			}
			names[field.Name] = true
		}
	}
	for index, typ := range module.Types.types {
		if typ.Kind == TypeStruct && !seen[TypeID(index+1)] {
			return fmt.Errorf("struct type !%d has no definition", index+1)
		}
	}
	return nil
}

// Detect owned mutable slice holders through nominal and aggregate storage.
// References and shared slices stop traversal: their referents are not copied.
// Rules: rules/memory/copy_move.md — §§4.1–4.4;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§39,114.
func containsOwnedMutableSlice(module *Module, id TypeID) bool {
	visited := map[TypeID]bool{}
	var visit func(TypeID) bool
	visit = func(id TypeID) bool {
		if visited[id] {
			return false
		}
		visited[id] = true
		typ, ok := module.Types.Lookup(id)
		if !ok {
			return false
		}
		switch typ.Kind {
		case TypeMutableSlice:
			return true
		case TypeNamed:
			return visit(typ.Base)
		case TypeArray:
			return visit(typ.Element)
		case TypeResult:
			return visit(typ.Success) || visit(typ.Error)
		case TypeStruct:
			if definition, ok := structDefinition(module, id); ok {
				for _, field := range definition.Fields {
					if visit(field.Type) {
						return true
					}
				}
			}
		case TypeUnion:
			for _, definition := range module.Unions {
				if definition.TypeID != id {
					continue
				}
				for _, variant := range definition.Variants {
					if visit(variant.Payload) {
						return true
					}
					for _, field := range variant.PayloadFields {
						if visit(field.Type) {
							return true
						}
					}
				}
			}
		}
		return false
	}
	return visit(id)
}
