package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	secmlirlowering "sec/internal/lowering/secmlir"
	"sec/internal/parser"
	"sec/internal/sema"
)

func TestPackage11SourceEmitsAndVerifiesEnumUnionValues(t *testing.T) {
	sourcePath := filepath.Join("..", "..", "testdata", "semantic_ir", "enum_union_values.sec")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), sourcePath))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := sema.NewAnalyzer()
	if diagnostics := a.Analyze(parsed.Program); len(diagnostics) != 0 {
		t.Fatalf("sema: %v", diagnostics)
	}
	module, err := semantic.Build(parsed.Program, a, semantic.BuildOptions{
		RequestedModule: "main", SourceFiles: []string{sourcePath}, MaxPackage: 11,
	})
	if err != nil {
		t.Fatal(err)
	}
	target, _ := findTargetDefinition(CompilerTarget{OS: "linux", Arch: "amd64"})
	plan, err := target.scalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	output, err := secmlirlowering.Emit(module, plan)
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, expected := range []string{
		"sec.dialect_version = 7 : i32",
		"!sec.enum<", "#sec.enum_case<", "sec.enum.constant",
		"sec.enum.from_integer", "sec.enum.to_integer", "sec.enum.cmp",
		"!sec.union<", "#sec.union_variant<", "#sec.union_field<",
		"sec.union.construct", `typeArguments = [!sec.int]`,
		`value = "340282366920938463463374607431768211456"`,
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in:\n%s", expected, text)
		}
	}

	binDir := os.Getenv("SEC_MLIR_BIN")
	if binDir == "" {
		return
	}
	path := filepath.Join(t.TempDir(), "enum-union.mlir")
	if err := os.WriteFile(path, output, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(filepath.Join(binDir, "sec-mlir-opt"), path,
		"--sec-verify-checked-integer-guards", "--sec-verify-result-guards",
		"--sec-verify-try-handlers", "--sec-verify-union-guards", "-o", os.DevNull)
	if combined, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sec-mlir-opt: %v\n%s\nGenerated:\n%s", err, combined, output)
	}
}
