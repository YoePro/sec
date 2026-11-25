package diagnostics

import (
	"reflect"
	"testing"
)

// Rules: rules/errors/panic.md §§13(1)–(2),13(8); MD-140 §6, T38–T39 (registry).
func TestCanonicalPanicReasonRegistryIsStableAndComplete(t *testing.T) {
	want := []PanicReasonDefinition{
		{ID: 1, Name: "ArithmeticOverflow"},
		{ID: 2, Name: "DivisionByZero"},
		{ID: 3, Name: "InvalidShift"},
		{ID: 4, Name: "BoundsFailure"},
		{ID: 5, Name: "ContractFailure"},
		{ID: 6, Name: "AssertionFailed"},
		{ID: 7, Name: "UnreachableReached"},
		{ID: 8, Name: "InvalidReferenceGeneration"},
		{ID: 9, Name: "ExplicitPanic"},
		{ID: 10, Name: "ForeignAbort"},
		{ID: 11, Name: "StringMaterializationFailure"},
		{ID: 12, Name: "panic.arena-epoch-exhaustion", Message: "Arena validity epoch exhausted."},
	}
	got := PanicReasonDefinitions()
	if len(got) != len(want) {
		t.Fatalf("panic reason count = %d, want %d", len(got), len(want))
	}
	for index, expected := range want {
		if got[index] != expected {
			t.Fatalf("panic reason %d = %+v, want %+v", index, got[index], expected)
		}
		byID, idOK := PanicReasonByID(expected.ID)
		byName, nameOK := PanicReasonByName(expected.Name)
		if !idOK || !nameOK || byID != expected || byName != expected {
			t.Fatalf("panic reason lookup %d/%s = (%+v, %t), (%+v, %t)", expected.ID, expected.Name, byID, idOK, byName, nameOK)
		}
	}
	if _, ok := PanicReasonByID(0); ok {
		t.Fatal("reserved zero PanicID resolved")
	}
	if _, ok := PanicReasonByName("Unknown"); ok {
		t.Fatal("unknown panic reason name resolved")
	}

	got[0].Name = "changed"
	again, _ := PanicReasonByID(PanicReasonArithmeticOverflow)
	if again.Name != "ArithmeticOverflow" {
		t.Fatal("registry snapshot exposed mutable storage")
	}
}

// MD-140 T38/T39 (registry portion): epoch advancement failure has its own
// stable uint32 identity and static message. Stale-reference and other existing
// failures keep their IDs; no runtime exhaustion behavior is asserted here.
// Rules: rules/errors/panic.md §§13(2)–(8),14; MD-140 §6.
func TestArenaEpochExhaustionPanicRegistry(t *testing.T) {
	if reflect.TypeOf(PanicReasonID(0)).Kind() != reflect.Uint32 {
		t.Fatal("PanicID representation changed")
	}
	if PanicReasonArenaEpochExhaustion != 12 {
		t.Fatal("exhaustion PanicID assignment changed")
	}
	got, ok := PanicReasonByName("panic.arena-epoch-exhaustion")
	if !ok || got.ID != PanicReasonArenaEpochExhaustion || got.Message != "Arena validity epoch exhausted." {
		t.Fatalf("exhaustion definition = %+v, registered = %v", got, ok)
	}
	for _, id := range []PanicReasonID{PanicReasonInvalidReferenceGeneration, PanicReasonArithmeticOverflow, PanicReasonExplicitPanic, PanicReasonStringMaterializationFailure} {
		if got.ID == id {
			t.Fatalf("exhaustion reused panic ID %d", id)
		}
	}
	stale, ok := PanicReasonByID(PanicReasonInvalidReferenceGeneration)
	if !ok || stale.ID != 8 || stale.Name != "InvalidReferenceGeneration" || stale.Message != "" {
		t.Fatalf("stale-reference identity changed: %+v", stale)
	}
	// Lookup and message access must remain allocation-free on the minimum path.
	if allocations := testing.AllocsPerRun(100, func() {
		byID, idOK := PanicReasonByID(PanicReasonArenaEpochExhaustion)
		byName, nameOK := PanicReasonByName("panic.arena-epoch-exhaustion")
		if !idOK || !nameOK || byID != byName || byID.Message == "" {
			panic("missing canonical panic metadata")
		}
	}); allocations != 0 {
		t.Fatalf("registry lookup allocates: %v", allocations)
	}
	snapshot := PanicReasonDefinitions()
	snapshot[len(snapshot)-1].Message = "changed"
	again, _ := PanicReasonByID(got.ID)
	if again != got {
		t.Fatal("message snapshot mutated registry")
	}
	if _, ok := PanicReasonByID(PanicReasonID(1 << 31)); ok {
		t.Fatal("unknown uint32 reason resolved")
	}
}
