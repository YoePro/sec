package sema

// compilerKnownCollectionOutcomeMembers owns normal absence/rejection result
// contracts for the canonical collection families. These are operation types,
// not new runtime implementations or unchecked-access errors. Sema, synthetic
// declarations and LSP all consume the same receiver-specialized registry.
// Rules: rules/collections/collections.md — §§6.9,13.6–13.10,14.7,15.3,16.
func compilerKnownCollectionOutcomeMembers(sequence Type) []CompilerKnownMember {
	var members []CompilerKnownMember
	add := func(id, name string, result Type, mutation bool, outcome string) {
		members = append(members, CompilerKnownMember{
			ID: id, Name: name, Kind: CompilerKnownMethod, Result: result,
			StructuralMutation: mutation,
			Documentation:      compilerKnownSyntheticCatalog[id].Documentation + " " + outcome,
		})
	}
	boolean := builtinType("bool")
	if sequence.Kind == ArrayType && arrayShapeOf(sequence) == ArrayShapeDynamic && sequence.Element != nil {
		add("CKM-DYNAMIC-ARRAY-REMOVEAT", "RemoveAt", compilerKnownOption(*sequence.Element), true, "Signed, unsigned and compatible named integer indexes are accepted; a negative or otherwise invalid index returns None, not an Error.")
	}
	if sequence.Name == "list" && len(sequence.TypeArgs) == 1 {
		element := sequence.TypeArgs[0]
		add("CKM-LIST-INSERT", "Insert", compilerKnownResult(boolean, builtinType("CollectionError")), true, "An invalid insertion position, including a negative signed index, returns Ok(false); only structural/resource failure uses CollectionError.")
		add("CKM-LIST-REMOVEAT", "RemoveAt", compilerKnownOption(element), true, "Signed, unsigned and compatible named integer indexes are accepted; a negative or otherwise invalid index returns None, not an Error.")
		add("CKM-LIST-REMOVE", "Remove", boolean, true, "Normal absence returns false, not an Error.")
		add("CKM-LIST-CONTAINS", "Contains", boolean, false, "Normal absence returns false, not an Error.")
		add("CKM-LIST-INDEXOF", "IndexOf", compilerKnownOption(builtinType("uint")), false, "Normal absence returns None, not an Error.")
	}
	if sequence.Name == "map" && len(sequence.TypeArgs) == 2 {
		add("CKM-MAP-REMOVE", "Remove", compilerKnownOption(sequence.TypeArgs[1]), true, "Normal absence returns None, not an Error; Some transfers the removed value.")
		add("CKM-MAP-CONTAINSKEY", "ContainsKey", boolean, false, "Normal absence returns false, not an Error.")
	}
	if sequence.Name == "set" && len(sequence.TypeArgs) == 1 {
		add("CKM-SET-ADD", "Add", compilerKnownResult(boolean, builtinType("CollectionError")), true, "An existing value returns Ok(false), not an Error; only structural/resource failure uses CollectionError.")
		add("CKM-SET-REMOVE", "Remove", boolean, true, "Normal absence returns false, not an Error.")
		add("CKM-SET-CONTAINS", "Contains", boolean, false, "Normal absence returns false, not an Error.")
	}
	return members
}
