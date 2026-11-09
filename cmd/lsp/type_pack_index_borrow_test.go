package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Unsaved buffers share the compiler's owned-pack argument semantics.
// Rules: rules/declarations/functions.md §38A(5)(2),(5),(10)(1–3);
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestOwnedPackIndexCallBorrowLSP(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"owned_index_borrow.sec", 0},
		{"owned_index_copy_invalid.sec", 2},
	} {
		t.Run(tc.file, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/generics/type_pack_declarations/" + tc.file)
			if err != nil {
				t.Fatal(err)
			}
			got := analyze(uriFromPath(filepath.Join(t.TempDir(), "main.sec")), string(source))
			count := 0
			for _, diagnostic := range got {
				if diagnostic.Severity != 1 {
					continue
				}
				count++
				if !strings.Contains(diagnostic.Message, "not implicitly copyable") || len(diagnostic.RelatedInformation) != 1 {
					t.Fatalf("copy failure lost template provenance: %+v", diagnostic)
				}
			}
			if count != tc.errors {
				t.Fatalf("want %d errors, got %+v", tc.errors, got)
			}
		})
	}
}
