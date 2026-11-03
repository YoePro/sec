package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Every frontend gate keeps null tests unsafe-only and preserves their owning
// diagnostics; an ordinary if must never grant raw-pointer access authority.
// Rules: rules/control-flow/flowcontrol_if.md — §23;
// rules/memory/raw_pointers.md — §7(3–9a); MD-024 correction — §5.
func TestIfRawNullUnsafeBoundaryCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"valid", "contexts_invalid"} {
		data, err := os.ReadFile("../../testdata/control_flow/if_raw_null/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, name+".sec")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		commands := []string{"check", "sema"}
		if name != "valid" {
			commands = append(commands, "emit-ir", "emit-mlir", "emit-llvm")
		}
		for _, command := range commands {
			args := []string{command, path, "--diagnostic-format=json"}
			if strings.HasPrefix(command, "emit-") {
				args = append(args, "-o", "-")
			}
			stdout, stderr, code := runCLIForDiagnostics(t, args...)
			if name == "valid" {
				if code != 0 {
					t.Fatal(command, code, stdout, stderr)
				}
				continue
			}
			doc := decodeOccurrenceDocument(t, stderr)
			if code != 3 || stdout != "" || doc.Summary.Errors != 10 || len(doc.Occurrences) != 10 {
				t.Fatal(command, code, stdout, stderr)
			}
			counts := map[string]int{}
			unsafeRead := 0
			for _, d := range doc.Occurrences {
				if d.Primary == nil || d.Primary.Span.Start.Line <= 0 {
					t.Fatalf("missing source span: %+v", d)
				}
				if d.ID != nil {
					counts[*d.ID]++
					if len(d.Help) == 0 {
						t.Fatalf("missing null diagnostic help: %+v", d)
					}
				}
				if strings.Contains(d.Message.Text, "RawPtr.Read requires unsafe") {
					unsafeRead++
				}
			}
			if counts[diagnostics.NullOutsideUnsafe] != 4 || counts[diagnostics.NullTestRequiresRawPointer] != 2 || counts[diagnostics.NullEquality] != 2 || counts[diagnostics.RawPointerNullDereference] != 1 || unsafeRead != 1 {
				t.Fatal(command, counts, stderr)
			}
		}
	}
}
