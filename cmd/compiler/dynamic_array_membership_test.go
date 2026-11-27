package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Both public target-plan inspection routes use the one resolved dynamic
// membership builder; source checking neither consumes nor copies the owner.
// Rules: rules/foundations/operators.md — §67;
// rules/compiler/compiler_pipeline.md — §§32-34.
func TestDynamicArrayMembershipCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/semantic_ir/dynamic_membership/scalars.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "membership.sec")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"linux-amd64", "linux-armv7"} {
		if _, diagnosis, code := runCheckWithoutTools(t, "check", source, "--target", target); code != 0 {
			t.Fatal(target, code, diagnosis)
		}
		output, diagnosis, code := runCheckWithoutTools(t, "emit-ir", source, "--target", target, "-o", "-")
		if code != 0 {
			t.Fatal(target, code, diagnosis)
		}
		for _, operation := range []string{"dynamic-array.length", "dynamic-array.element-place", "dynamic-array.index-in-bounds", "float.compare eq", "bool.not"} {
			if !strings.Contains(output, operation) {
				t.Fatalf("%s missing %s", target, operation)
			}
		}
	}
}
