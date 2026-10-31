package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// CLI checks distinguish supported exclusive pack loans from invalid mutable
// authority, without depending on work-in-progress library sources.
// Rules: rules/declarations/functions.md §38A(3)(2–5),(10).
func TestMutableTypePackCLI(t *testing.T) {
	file := typePackCLIFixture(t, "mutable_calls.sec")
	_, output, code := runCLIForDiagnostics(t, "check", file, "--target", "linux-amd64")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, name := range []string{"mutable_overlap_invalid.sec", "mutable_reborrow_overlap_invalid.sec", "mutable_fixed_overlap_invalid.sec", "mutable_shared_invalid.sec", "mutable_immutable_invalid.sec", "mutable_move_invalid.sec"} {
		file := typePackCLIFixture(t, name)
		_, output, code := runCLIForDiagnostics(t, "check", file, "--target", "linux-amd64", "--diagnostic-format=json")
		if code != 3 {
			t.Fatal(name, code, output)
		}
		doc := decodeOccurrenceDocument(t, output)
		if len(doc.Occurrences) == 0 {
			t.Fatal("missing authority error", name)
		}
		for _, occurrence := range doc.Occurrences {
			if occurrence.ID != nil && *occurrence.ID == diagnostics.GenericTypePackExpansionUnavailable {
				t.Fatal(name, occurrence)
			}
		}
	}
}

// Exclusive packs use ordinary mutable reference lowering, not shared-reference
// copies; source overlap failures cannot reach artifact emission.
// Rules: rules/declarations/functions.md §38A(3)(2–5),(9);
// rules/compiler/semantic_ir.md §27A(1–2),(4).
func TestMutableTypePackEmissionCLI(t *testing.T) {
	for _, name := range []string{"mutable_calls.sec", "static_mutable_boundary.sec", "exclusive_methods.sec"} {
		for _, command := range []string{"emit-ir", "emit-sec-mlir"} {
			if command == "emit-sec-mlir" && os.Getenv("SEC_MLIR_BIN") == "" {
				continue
			}
			args := []string{command, typePackCLIFixture(t, name), "--target", "linux-amd64"}
			if command == "emit-ir" {
				args = append(args, "-o", "-")
			}
			stdout, output, code := runCLIForDiagnostics(t, args...)
			if code != 0 || !strings.Contains(stdout, "ref mut int") {
				t.Fatal(name, command, code, stdout, output)
			}
		}
	}
	for _, name := range []string{"mutable_overlap_invalid.sec", "mutable_reborrow_overlap_invalid.sec", "mutable_fixed_overlap_invalid.sec"} {
		stdout, output, code := runCLIForDiagnostics(t, "emit-ir", typePackCLIFixture(t, name), "--target", "linux-amd64", "-o", "-")
		if code != 3 || stdout != "" {
			t.Fatal(name, code, stdout, output)
		}
	}
}
