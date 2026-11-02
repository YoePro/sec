package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// Source conformance is checked independently of the named ownership-sensitive
// specialization boundary; failed inspection must leave no partial IR artifact.
// Rules: rules/errors/errorhandling.md — §§20,29; MD-120 §11(4–5);
// rules/compiler/monomorphization.md — §11.2.
func TestGenericErrorOwnershipCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"valid", "invalid", "lowering_boundary"} {
		data, err := os.ReadFile("../../testdata/sema/generic_error_ownership/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "main.sec")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		_, output, code := runCLIForDiagnostics(t, "check", path, "--diagnostic-format=json")
		if name != "invalid" {
			if code != 0 {
				t.Fatal(name, code, output)
			}
		} else {
			document := decodeOccurrenceDocument(t, output)
			guards := 0
			if code != 3 || document.Summary.Errors != 11 {
				t.Fatal(code, output)
			}
			for _, d := range document.Occurrences {
				if d.ID != nil && *d.ID == diagnostics.GuardConsumesBinding {
					guards++
					if d.Unregistered || len(d.Help) == 0 || d.Severity != diagnostics.SeverityError {
						t.Fatal(d)
					}
				}
			}
			if guards != 2 {
				t.Fatal(output)
			}
		}
		if name == "lowering_boundary" {
			artifact := filepath.Join(root, "out.ir")
			stdout, output, code := runCLIForDiagnostics(t, "emit-ir", path, "-o", artifact)
			if code == 0 || stdout != "" || !strings.Contains(output, "[unsupported-lowering]") || !strings.Contains(output, "generic specialization with the ownership-sensitive argument") {
				t.Fatal(code, stdout, output)
			}
			if _, err := os.Stat(artifact); !os.IsNotExist(err) {
				t.Fatalf("failed lowering left output: %v", err)
			}
		}
	}
}
