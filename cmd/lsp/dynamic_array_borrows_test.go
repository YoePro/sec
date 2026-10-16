package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDynamicArrayMutationBorrowsLSP publishes the canonical live-borrow
// diagnostics for unsaved source, including navigation back to the loan origin.
// Rules: rules/collections/collections.md — §§6.5,19;
// rules/memory/borrowing.md — §§14.5,32.
func TestDynamicArrayMutationBorrowsLSP(t *testing.T) {
	for _, test := range []struct {
		suffix string
		want   int
	}{{"valid", 0}, {"invalid", 10}} {
		data, err := os.ReadFile("../../testdata/sema/dynamic_array_borrows_" + test.suffix + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		uri := uriFromPath(filepath.Join(t.TempDir(), "borrows.sec"))
		count := 0
		for _, diagnostic := range analyze(uri, string(data)) {
			if diagnostic.Severity != 1 {
				continue
			}
			count++
			if len(diagnostic.RelatedInformation) == 0 {
				t.Fatal("missing loan origin", diagnostic)
			}
		}
		if count != test.want {
			t.Fatal(test.suffix, count)
		}
	}
}
