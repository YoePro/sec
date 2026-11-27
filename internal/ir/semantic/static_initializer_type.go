package semantic

// staticInitializerType resolves storage's scalar base without changing the
// static's nominal TypeID. Sema has already proven the canonical default and
// its contracts; IR checks the physical constant domain, not diagnostic names.
// Missing/cyclic base facts cannot become a valid initializer.
// Rules: rules/types/default_values.md — §40;
// rules/types/types.md — Named types; rules/compiler/semantic_ir.md — §28.
func staticInitializerType(types *TypeTable, id TypeID) (Type, bool) {
	seen := map[TypeID]bool{}
	for !seen[id] {
		seen[id] = true
		typ, found := types.Lookup(id)
		if !found {
			return Type{}, false
		}
		if typ.Kind != TypeNamed {
			return typ, true
		}
		id = typ.Base
	}
	return Type{}, false
}
