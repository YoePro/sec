package defaults

import (
	"math/big"
	"sec/internal/sema"
	"testing"
)

// TestEmptyDynamicArrayDefaultSource renders the shared empty-owner fact without
// looking for an element default, including a nested fixed array of owners.
// Rules: rules/types/default_values.md — "Owning dynamic arrays", "LSP";
// rules/collections/collections.md — §6.2.
func TestEmptyDynamicArrayDefaultSource(t *testing.T) {
	owner := sema.NewDynamicArrayType(sema.Type{Kind: sema.ReferenceType, Name: "ref int"})
	if value, ok := sourceDefault(owner); !ok || value != "[]" {
		t.Fatal(value, ok)
	}
	fixed := sema.NewFixedArrayType(owner, big.NewInt(2))
	if value, ok := sourceDefault(fixed); !ok || value != "[[], []]" {
		t.Fatal(value, ok)
	}
}
