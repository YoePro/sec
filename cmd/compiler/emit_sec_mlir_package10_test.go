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

func TestPackage10SourceEmitsAndVerifiesResultHandlers(t *testing.T) {
	source := `module main
fn Source(value: int) Result[int, ArithmeticError] { return Ok(value) }
fn Handle(value: int) int {
  return try Source(value) {
    Ok(found) => found
    Err(ArithmeticError.DivisionByZero) => 0
    Err(error) => 1
  }
}
fn Divide(left: int, right: int) int {
  return try left / right {
    Err(ArithmeticError.DivisionByZero) => 0
    Err(error) => 1
  }
}
fn Forward(value: int) Result[int, ArithmeticError] {
  let resolved := try Source(value) {
    Err(error) => return Err(error)
  }
  return Ok(resolved)
}
`
	p := parser.New(lexer.NewWithFile(source, "handlers.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := sema.NewAnalyzer()
	if errors := a.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	module, err := semantic.Build(parsed.Program, a, semantic.BuildOptions{
		RequestedModule: "main", SourceFiles: []string{"handlers.sec"}, MaxPackage: 10,
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
		"sec.dialect_version = 6 : i32",
		"sec.result.is_err",
		"sec.result.unwrap_ok",
		"sec.result.unwrap_err",
		"sec.core_error.is_variant",
		"sec.arithmetic_error.from_reason",
		`sec.try_handler_kind = "err-variant"`,
		`sec.try_handler_variant = "DivisionByZero"`,
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in:\n%s", expected, text)
		}
	}

	binDir := os.Getenv("SEC_MLIR_BIN")
	if binDir == "" {
		return
	}
	path := filepath.Join(t.TempDir(), "handlers.mlir")
	if err := os.WriteFile(path, output, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(filepath.Join(binDir, "sec-mlir-opt"), path,
		"--sec-verify-checked-integer-guards", "--sec-verify-result-guards",
		"--sec-verify-try-handlers", "-o", os.DevNull)
	if combined, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sec-mlir-opt: %v\n%s\nGenerated:\n%s", err, combined, output)
	}
}
