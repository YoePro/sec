package main

import (
	"os"
	"testing"
)

// TestPropertyReceiverLSP uses Sema's inferred receiver requirements and
// reports the getter declaration alongside insufficient mutable authority.
// Rules: rules/declarations/properties.md — §8; rules/memory/borrowing.md — §7.
func TestPropertyReceiverLSP(t *testing.T) {
	for _, tc := range []struct {
		file            string
		errors, mutable int
	}{{"valid.sec", 0, 0}, {"authority_invalid.sec", 5, 3}, {"borrow_invalid.sec", 2, 0}} {
		data, err := os.ReadFile("../../testdata/sema/property_receivers/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		uri := "file:///property-receivers/main.sec"
		errors, mutable := 0, 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			errors++
			if d.Code == "S1219" {
				mutable++
				if len(d.RelatedInformation) == 0 || d.RelatedInformation[0].Location.URI != uri {
					t.Fatal(d)
				}
			}
		}
		if errors != tc.errors || mutable != tc.mutable {
			t.Fatal(tc.file, errors, mutable)
		}
	}
}
