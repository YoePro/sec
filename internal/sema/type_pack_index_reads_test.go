package sema

import (
	"os"
	"strings"
	"testing"
)

// Selected reference arguments borrow owned pack positions, while actual value
// uses keep specialization-specific copy requirements.
// Rules: rules/declarations/functions.md §38A(5)(2),(5),(10)(1–2);
// rules/memory/borrowing.md §15.2–15.4.
func TestOwnedPackIndexCallBorrow(t *testing.T) {
	for _, tc := range []struct {
		file   string
		copies int
	}{
		{"owned_index_borrow.sec", 0},
		{"owned_index_copy_invalid.sec", 2},
	} {
		t.Run(tc.file, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/generics/type_pack_declarations/" + tc.file)
			if err != nil {
				t.Fatal(err)
			}
			errors := analyzeSourceRaw(t, string(source))
			if len(errors) != tc.copies {
				t.Fatalf("want %d copy errors: %v", tc.copies, errors)
			}
			for _, err := range errors {
				if !strings.Contains(err.Message, "position 0 is Handle, which is not implicitly copyable") || err.PreviousLine == 0 {
					t.Fatalf("missing concrete copy requirement or template provenance: %+v", err)
				}
			}
		})
	}
}
