package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// All public native boundaries retain the current default capability gate.
// Source validity errors and unsupported lowering remain distinct, and neither
// may leave an output artifact or partially emitted standard output. Explicit
// P16/schema-12 coverage lives in the lowering package, not another CLI route.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§121–124,142;
// rules/compiler/compiler_pipeline.md — §§32–34,45(6).
func TestSliceUnsupportedCLIBoundaries(t *testing.T) {
	entry, err := os.ReadFile("../../testdata/semantic_ir/package16_boundaries/entry.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"build", "emit-mlir", "emit-llvm"} {
		for _, tc := range []struct {
			file, reason string
			status       int
		}{
			{"package16_boundaries/ffi_parameter_invalid.sec", "ABI", 3},
			{"package16_boundaries/ffi_return_invalid.sec", "ABI", 3},
			{"package16_boundaries/raw_conversion_invalid.sec", "slice", 3},
			{"package16_boundaries/equality_invalid.sec", "cannot compare", 3},
			{"package16_boundaries/ordering_invalid.sec", "cannot order", 3},
			{"package16_boundaries/signed_property_invalid.sec", "int", 3},
			{"package16_boundaries/signed_global_invalid.sec", "int", 3},
			{"package16_boundaries/move_only_read_invalid.sec", "copy", 3},
			{"package16_boundaries/element_move_invalid.sec", "not implicitly copyable", 3},
			{"package16_boundaries/stable_source_invalid.sec", "slice", 3},
			{"package16_boundaries/weak_source_invalid.sec", "slice", 3},
			{"package16_boundary_owner.sec", "[unsupported-lowering]", 4},
			{"package16_boundaries/pointer.sec", "[unsupported-lowering]", 4},
			{"package16_boundaries/membership.sec", "[unsupported-lowering]", 4},
			{"package16_boundaries/iteration.sec", "[unsupported-lowering]", 4},
			{"package16_boundaries/reverse.sec", "[unsupported-lowering]", 4},
		} {
			t.Run(command+"/"+tc.file, func(t *testing.T) {
				dir := t.TempDir()
				artifact := filepath.Join(dir, "output")
				data, err := os.ReadFile("../../testdata/semantic_ir/" + tc.file)
				if err != nil {
					t.Fatal(err)
				}
				// A real supported entry prevents a missing-main error from
				// masking the unsupported construct in the build command.
				source := filepath.Join(dir, filepath.Base(tc.file))
				_, body, found := strings.Cut(string(data), "\n")
				if !found || !strings.HasPrefix(string(data), "module main\n") {
					t.Fatal("fixture must declare module main")
				}
				if err := os.WriteFile(source, append(append([]byte(nil), entry...), body...), 0600); err != nil {
					t.Fatal(err)
				}
				args := []string{command, source, "-o", artifact}
				if tc.status == 3 {
					args = append(args, "--diagnostic-format=json")
				}
				stdout, stderr, status := runCLIForDiagnostics(t, args...)
				if status != tc.status || stdout != "" || tc.status != 3 && !strings.Contains(stderr, tc.reason) {
					t.Fatalf("wrong failure boundary: %d\n%s\n%s", status, stdout, stderr)
				}
				if tc.status == 3 {
					doc := decodeOccurrenceDocument(t, stderr)
					if len(doc.Occurrences) == 0 {
						t.Fatal("missing owning source error")
					}
					for _, d := range doc.Occurrences {
						if !strings.Contains(d.Message.Text, tc.reason) {
							t.Fatal("wrong source error", d.Message)
						}
					}
				}
				if _, err := os.Stat(artifact); !os.IsNotExist(err) {
					t.Fatal("failed command left output", err)
				}
			})
		}
	}
}
