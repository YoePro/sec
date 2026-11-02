package sema

import (
	"os"
	"sec/internal/diagnostics"
	"testing"
)

// Static functions have no implicit receiver, including in extension impls.
// Rules: rules/declarations/static.md — §§9,9.1.
func TestStaticReceiverIsolation(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/static_receiver/receiver_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	counts := map[string]int{}
	for _, e := range errors {
		counts[e.ID]++
		if e.ID == diagnostics.StaticInstanceReceiverRequired && (e.Help == "" || e.Severity != diagnostics.SeverityError) {
			t.Fatalf("incomplete diagnostic: %+v", e)
		}
	}
	if len(errors) != 13 || counts[diagnostics.StaticInstanceReceiverRequired] != 7 {
		t.Fatalf("receiver errors: %+v", errors)
	}
	data, err = os.ReadFile("../../testdata/sema/static_receiver/receiver_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(data)); len(errors) != 0 {
		t.Fatal(errors)
	}
}
