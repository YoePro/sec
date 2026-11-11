package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Every CLI frontend gate projects the same raw-pointer error category and
// mentor help; proven-invalid storage retains its compiler-owned source link.
// Rules: rules/memory/raw_pointers.md §34; rules/tooling/diagnostics.md §§8–9.
func TestRawPointerMentorCompilerDiagnostics(t *testing.T) {
	path := "../../testdata/tooling/raw_pointer_mentor_invalid.sec"
	var baseline []emittedOccurrence
	for _, command := range []string{"sema", "check", "analyse", "emit-ir", "emit-sec-mlir", "emit-mlir", "emit-llvm"} {
		t.Run(command, func(t *testing.T) {
			args := []string{command, path, "--diagnostic-format=json"}
			artifact := filepath.Join(t.TempDir(), "output")
			if strings.HasPrefix(command, "emit-") {
				args = append(args, "-o", artifact)
			}
			_, stderr, code := runCLIForDiagnostics(t, args...)
			if code != 3 {
				t.Fatalf("expected semantic rejection: %d %s", code, stderr)
			}
			if _, err := os.Stat(artifact); !os.IsNotExist(err) {
				t.Fatalf("rejected source produced artifact: %v", err)
			}
			doc := decodeOccurrenceDocument(t, stderr)
			if baseline == nil {
				baseline = doc.Occurrences
			} else if !reflect.DeepEqual(baseline, doc.Occurrences) {
				t.Fatalf("%s changed canonical raw-pointer occurrences", command)
			}
			counts := map[string]int{}
			for _, d := range doc.Occurrences {
				if d.ID == nil {
					t.Fatalf("missing ID: %+v", d)
				}
				id := *d.ID
				counts[id]++
				if d.Unregistered || d.Primary == nil || d.Primary.Span.File != path || len(d.Help) != 1 || len(d.Fixes) != 0 {
					t.Fatalf("missing mentor metadata: %+v", d)
				}
				if strings.HasPrefix(d.Message.Text, "RawPtr.Read requires") || strings.HasPrefix(d.Message.Text, "RawPtr.Write requires") {
					if strings.Contains(d.Help[0].Text, "volatile") {
						t.Fatalf("ordinary access acquired volatile caller obligations: %+v", d)
					}
				}
				if id == diagnostics.RawPointerNullDereference || id == diagnostics.RawPointerDanglingOrigin {
					if len(d.Related) != 1 || d.Related[0].Span.File != path {
						t.Fatalf("lost causal location: %+v", d)
					}
				}
			}
			if counts[diagnostics.RawPointerUnsafeRequired] != 11 || counts[diagnostics.RawPointerInvalidOperation] != 21 || counts[diagnostics.RawPointerNullDereference] != 6 || counts[diagnostics.RawPointerDanglingOrigin] != 1 || doc.Summary.Errors != 39 {
				t.Fatalf("wrong categories: %v %+v", counts, doc.Summary)
			}
		})
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", path)
	if code != 3 || !strings.Contains(stderr, "S1204") || !strings.Contains(stderr, "Adding unsafe cannot") || !strings.Contains(stderr, "originating value became unavailable here") {
		t.Fatalf("lost human diagnostics: %d %s", code, stderr)
	}
	_, stderr, code = runCLIForDiagnostics(t, "check", "../../testdata/tooling/raw_pointer_mentor_valid.sec")
	if code != 0 {
		t.Fatalf("valid operations rejected: %d %s", code, stderr)
	}
}
