package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Generic init calls from extensions use the constructed owner's fixed
// arguments; an incompatible value cannot silently infer another owner type.
// Rules: rules/declarations/impl.md §§15–16,21–22;
// rules/declarations/generics.md §§17,25,29.
func TestGenericImplCompositionCLI(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"primary", "extension"} {
		data, err := os.ReadFile("../../testdata/sema/generic_impl_composition/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name+".sec"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, output, code := runCLIForDiagnostics(t, "check", root, "--diagnostic-format=json")
	if code != 0 {
		t.Fatal(code, output)
	}
	data, err := os.ReadFile("../../testdata/sema/generic_impl_composition/arguments_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "arguments_invalid.sec"), data, 0600); err != nil {
		t.Fatal(err)
	}
	_, output, code = runCLIForDiagnostics(t, "check", root, "--diagnostic-format=json")
	if code == 0 || !strings.Contains(output, "must be int, got bool") || !strings.Contains(output, "must return bool, got int") {
		t.Fatal(code, output)
	}
}
