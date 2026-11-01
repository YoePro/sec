package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fixed scalar/large-array membership reaches the public IR inspection route
// without backend tools. Slice/schema-12 emission retains its explicit P16 gate.
// Rules: rules/foundations/operators.md — §A.14;
// rules/compiler/compiler_pipeline.md — §§32–34.
func TestSequenceMembershipCLI(t *testing.T) {
	// Builtin-only fixture: use an isolated source workspace so unrelated
	// core FormatSink migration does not mask the membership boundary.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/semantic_ir/sequence_membership/fixed.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "membership.sec")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"linux-amd64", "linux-armv7"} {
		_, diagnosis, code := runCheckWithoutTools(t, "check", source, "--target", target)
		if code != 0 {
			t.Fatal(target, code, diagnosis)
		}
		output, diagnosis, code := runCheckWithoutTools(t, "emit-ir", source, "--target", target, "-o", "-")
		if code != 0 {
			t.Fatal(target, code, diagnosis)
		}
		for _, operation := range []string{"array.extract", "float.compare eq", "bool.not", "proof=guarded"} {
			if !strings.Contains(output, operation) {
				t.Fatalf("%s missing %s in %s", target, operation, output)
			}
		}
	}
}
