package main

import "testing"

// CLI uses the selected parameter's borrow mode when checking an owned pack
// position, retaining actual by-value copy failures with template provenance.
// Rules: rules/declarations/functions.md §38A(5)(2),(5),(10)(1–2).
func TestOwnedPackIndexCallBorrowCLI(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
		code   int
	}{
		{"owned_index_borrow.sec", 0, 0},
		{"owned_index_copy_invalid.sec", 2, 3},
	} {
		t.Run(tc.file, func(t *testing.T) {
			_, output, code := runCLIForDiagnostics(t, "sema", "../../testdata/generics/type_pack_declarations/"+tc.file, "--diagnostic-format=json")
			doc := decodeOccurrenceDocument(t, output)
			if code != tc.code || doc.Summary.Errors != tc.errors {
				t.Fatalf("code %d: %s", code, output)
			}
		})
	}
}
