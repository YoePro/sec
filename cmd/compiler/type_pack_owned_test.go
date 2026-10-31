package main

import (
	"strings"
	"testing"
)

// CLI checking admits owned pack calls without pretending that IR already
// represents their ownership/cleanup modes. Invalid moves stop before emission.
// Rules: rules/declarations/functions.md §38A(3),(6),(9); rules/compiler/semantic_ir.md §27A.
func TestOwnedTypePackCLI(t *testing.T) {
	file := typePackCLIFixture(t, "owned_calls.sec")
	_, output, code := runCLIForDiagnostics(t, "check", file, "--target", "linux-amd64")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, name := range []string{"owned_unmarked_invalid.sec", "owned_reuse_invalid.sec", "owned_failed_preparation_invalid.sec", "owned_duplicate_move_invalid.sec", "owned_partial_overlap_invalid.sec", "owned_borrow_overlap_invalid.sec", "ordinary_duplicate_move_invalid.sec"} {
		file := typePackCLIFixture(t, name)
		_, output, code := runCLIForDiagnostics(t, "check", file, "--target", "linux-amd64")
		if code != 3 || strings.Contains(output, "compiler capability unavailable") {
			t.Fatal(name, code, output)
		}
	}
	file = typePackCLIFixture(t, "owned_cleanup_boundary.sec")
	stdout, output, code := runCLIForDiagnostics(t, "emit-ir", file, "--target", "linux-amd64", "-o", "-")
	if code != 4 || stdout != "" || !strings.Contains(output, "ownership-sensitive owned type-pack position") {
		t.Fatal(code, stdout, output)
	}
}
