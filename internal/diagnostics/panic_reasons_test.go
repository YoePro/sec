package diagnostics

import "testing"

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
