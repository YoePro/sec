package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestTypePackDeclarationCLI separates recognized syntax from unavailable
// semantic expansion at compiler gates and preserves the registered reason.
// Rules: rules/declarations/generics.md — §16A; MD-117 — §10;
// rules/tooling/diagnostics.md — §24(4h).
func TestTypePackDeclarationCLI(t *testing.T) {
	path := typePackCLIFixture(t, "declarations.sec")
	_, output, code := runCLIForDiagnostics(t, "parse", path)
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "emit-ir"} {
		args := []string{command, path, "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", "-")
		}
		stdout, output, code := runCLIForDiagnostics(t, args...)
		doc := decodeOccurrenceDocument(t, output)
		if code != 3 || len(doc.Occurrences) != 4 || stdout != "" {
			t.Fatal(command, code, stdout, output)
		}
		for _, d := range doc.Occurrences {
			if d.ID == nil || *d.ID != diagnostics.GenericTypePackExpansionUnavailable || d.Unregistered || d.Primary == nil || len(d.Help) != 1 || !strings.Contains(d.Message.Text, "compiler capability unavailable") {
				t.Fatal(d)
			}
		}
	}
}

// Shared packs check successfully and calls emit concrete specializations in
// Semantic IR. Forms Semantic IR cannot represent yet stay named capability
// boundaries with no emitted artifact.
// Rules: rules/declarations/functions.md §38A(3–4),(9); rules/compiler/semantic_ir.md §27A.
func TestSharedTypePackSpecializationCLI(t *testing.T) {
	for _, test := range []struct{ name, boundary string }{
		{"shared_unused.sec", "non-trivial call argument"},
		{"printf_stub.sec", "non-trivial call argument"},
		{"shared_length.sec", "borrowed temporary of a non-trivial type"},
		{"shared_scalar_prefix.sec", "type-pack callable with a scalar generic prefix"},
	} {
		file := typePackCLIFixture(t, test.name)
		_, output, code := runCLIForDiagnostics(t, "check", file, "--target", "linux-amd64")
		if code != 0 {
			t.Fatal(test.name, code, output)
		}
		stdout, output, code := runCLIForDiagnostics(t, "emit-ir", file, "--target", "linux-amd64", "-o", "-")
		if code != 4 || stdout != "" || !strings.Contains(output, test.boundary) {
			t.Fatal(test.name, code, stdout, output)
		}
	}
	file := typePackCLIFixture(t, "shared_specialization.sec")
	stdout, output, code := runCLIForDiagnostics(t, "emit-ir", file, "--target", "linux-amd64", "-o", "-")
	if code != 0 || !strings.Contains(stdout, `"main::Count[...Types<int,bool>](ref int,ref bool)"`) || !strings.Contains(stdout, "type-pack(Types=") {
		t.Fatal(code, stdout, output)
	}
	if os.Getenv("SEC_MLIR_BIN") != "" {
		stdout, output, code = runCLIForDiagnostics(t, "emit-sec-mlir", file, "--target", "linux-amd64")
		if code != 0 || !strings.Contains(stdout, `sec.function_id = "main::Count[...Types<bool,int>](ref bool,ref int)"`) {
			t.Fatal(code, stdout, output)
		}
	}
}

// typePackCLIFixture isolates language capability diagnostics from work-in-progress
// library sources. The compiler's ordinary source-root discovery remains active;
// no intrinsic, diagnostic or analysis rule is disabled.
// Rules: rules/compiler/compiler_testing.md — focused frontend regressions.
func typePackCLIFixture(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/" + name)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

// Imported pack templates preserve their parameter scope during source-module
// qualification; the call-side IR boundary also refuses external pack targets.
// Rules: rules/projects/modules.md §§9,13; rules/declarations/functions.md §38A(9).
func TestImportedSharedTypePackCLI(t *testing.T) {
	main := typePackCLIFixture(t, "imported_pack/main.sec")
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/imported_pack/packlib.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(main), "packlib.sec"), data, 0600); err != nil {
		t.Fatal(err)
	}
	_, diagnostics, code := runCLIForDiagnostics(t, "check", main, "--target", "linux-amd64")
	if code != 0 {
		t.Fatal(code, diagnostics)
	}
	stdout, diagnostics, code := runCLIForDiagnostics(t, "emit-ir", main, "--target", "linux-amd64", "-o", "-")
	if code != 4 || stdout != "" || !strings.Contains(diagnostics, "heterogeneous type-pack callable expansion") {
		t.Fatal(code, stdout, diagnostics)
	}
}
