package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"testing"
)

// Origin aliases retain the canonical source-error categories and invalidation
// locations in CLI diagnostics, without falling through to IR emission.
// Rules: rules/memory/raw_pointers.md §§23(7),24(7),34–35.
func TestRawOriginAliasCompilerDiagnostics(t *testing.T) {
	testRawOriginCompilerDiagnostics(t, "raw_origin_alias", 2)
}

// Raw accesses obey the same known-origin lifetime and borrow obligations.
// Rules: rules/memory/raw_pointers.md §§11–13,23(4),24(7),34–35.
func TestRawOriginAccessCompilerDiagnostics(t *testing.T) {
	testRawOriginCompilerDiagnostics(t, "raw_origin_access", 5)
}

// testRawOriginCompilerDiagnostics checks frontend rejection before emission,
// canonical diagnostic categories, help and earlier invalidating source links.
// Rules: rules/memory/raw_pointers.md §§23–24,34–35.
func testRawOriginCompilerDiagnostics(t *testing.T, fixture string, dangling int) {
	t.Helper()
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "out")
		args := []string{command, "../../testdata/sema/" + fixture + "_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", artifact)
		}
		_, output, code := runCLIForDiagnostics(t, args...)
		if code != 3 {
			t.Fatal(code, output)
		}
		doc := decodeOccurrenceDocument(t, output)
		if doc.Summary.Errors != dangling+2 {
			t.Fatal(output)
		}
		counts := map[string]int{}
		for _, d := range doc.Occurrences {
			if d.ID == nil || len(d.Help) != 1 || len(d.Related) == 0 {
				t.Fatalf("lost origin proof: %+v", d)
			}
			counts[*d.ID]++
			if *d.ID == diagnostics.RawPointerDanglingOrigin && d.Related[0].Span.Start.Line >= d.Primary.Span.Start.Line {
				t.Fatalf("lost earlier move: %+v", d)
			}
		}
		if counts[diagnostics.RawPointerDanglingOrigin] != dangling || counts[diagnostics.BorrowOwnerReadConflict] != 1 || counts[diagnostics.BorrowMutationConflict] != 1 {
			t.Fatal(counts)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid source emitted IR", err)
		}
	}
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/"+fixture+"_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
}
