package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
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
		"sec.dialect_version = 10 : i32",
		"!sec.enum<", "#sec.enum_case<", "sec.enum.constant",
		"sec.enum.from_integer", "sec.enum.to_integer", "sec.enum.cmp",
		"!sec.union<", "#sec.union_variant<", "#sec.union_field<",
		"sec.union.construct", `typeArguments = [!sec.int]`,
		`value = "340282366920938463463374607431768211456"`,
		`value = "-170141183460469231731687303715884105728"`,
		`representation = "bit-backed", bitWidth = 1`,
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

func TestPackage11ResultEnumHandlersAndArithmeticUseSchema7Enums(t *testing.T) {
	source := `module main
enum Failure error { invalid, exhausted, }
fn WideSource(value: int128) Result[int128, Failure] { return Ok(value) }
fn WideForward(value: int128) Result[int128, Failure] {
  let resolved := try WideSource(value)
  return Ok(resolved)
}
fn HugeSource(value: uint256) Result[uint256, Failure] { return Ok(value) }
fn HugeHandle(value: uint256) uint256 {
  return try HugeSource(value) { Err(error) => 0 }
}
fn Specific(value: int) int {
  return try SmallSource(value) {
    Err(Failure.invalid) => 0
    Err(error) => 1
  }
}
fn Exhaustive(value: int) int {
  return try SmallSource(value) {
    Err(Failure.invalid) => 0
    Err(Failure.exhausted) => 1
  }
}
fn SmallSource(value: int) Result[int, Failure] { return Ok(value) }
fn Divide(left: int128, right: int128) int128 {
  return try left / right {
    Err(ArithmeticError.DivisionByZero) => 0
    Err(error) => 1
  }
}
`
	module := buildPackage11Module(t, source, "package11-result.sec")
	plan := package11Plan(t, CompilerTarget{OS: "linux", Arch: "amd64"})
	output, err := secmlirlowering.Emit(module, plan)
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, expected := range []string{
		`!sec.result<si128, !sec.enum<identity = "main::Failure"`,
		`!sec.result<ui256, !sec.enum<identity = "main::Failure"`,
		`!sec.enum<identity = "core::ArithmeticError"`,
		`"sec.enum.constant"`,
		`"sec.enum.cmp"`,
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in:\n%s", expected, text)
		}
	}
	for _, forbidden := range []string{"!sec.core_error", "sec.core_error.is_variant"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("schema 7 emitted %q:\n%s", forbidden, text)
		}
	}
	verifyPackage11MLIR(t, output)
}

func TestPackage11DefaultEnumUnderlyingFollowsTargetPlan(t *testing.T) {
	module := buildPackage11Module(t, `module main
enum State { ready, waiting, }
fn Identity(value: State) State { return value }
`, "package11-target.sec")
	for _, test := range []struct {
		target CompilerTarget
		want   string
	}{
		{CompilerTarget{OS: "linux", Arch: "armv7"}, `underlying = si32`},
		{CompilerTarget{OS: "linux", Arch: "amd64"}, `underlying = si64`},
	} {
		t.Run(test.target.Arch, func(t *testing.T) {
			output, err := secmlirlowering.Emit(module, package11Plan(t, test.target))
			if err != nil {
				t.Fatal(err)
			}
			binDir := os.Getenv("SEC_MLIR_BIN")
			if binDir == "" {
				t.Skip("SEC_MLIR_BIN is not set")
			}
			path := filepath.Join(t.TempDir(), "target-enum.mlir")
			if err := os.WriteFile(path, output, 0600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(filepath.Join(binDir, "sec-mlir-opt"), path, "--sec-lower-scalar-core")
			lowered, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("sec-mlir-opt: %v\n%s\nGenerated:\n%s", err, lowered, output)
			}
			if !strings.Contains(string(lowered), test.want) {
				t.Fatalf("missing %q in:\n%s", test.want, lowered)
			}
		})
	}
}

func buildPackage11Module(t *testing.T, source, file string) *semantic.Module {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, file))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := sema.NewAnalyzer()
	if diagnostics := a.Analyze(parsed.Program); len(diagnostics) != 0 {
		t.Fatalf("sema: %v", diagnostics)
	}
	module, err := semantic.Build(parsed.Program, a, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{file}, MaxPackage: 11})
	if err != nil {
		t.Fatal(err)
	}
	return module
}

func package11Plan(t *testing.T, targetName CompilerTarget) layout.ResolvedScalarPlan {
	t.Helper()
	target, ok := findTargetDefinition(targetName)
	if !ok {
		t.Fatalf("target %s is missing", targetName.String())
	}
	plan, err := target.scalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func verifyPackage11MLIR(t *testing.T, output []byte) {
	t.Helper()
	binDir := os.Getenv("SEC_MLIR_BIN")
	if binDir == "" {
		return
	}
	path := filepath.Join(t.TempDir(), "package11.mlir")
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
