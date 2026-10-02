package main

import (
	"errors"
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
	"sec/internal/testsupport"
)

func TestPackage10SourceEmitsAndVerifiesResultHandlers(t *testing.T) {
	source := `module main
fn Source(value: int) Result[int, ArithmeticError] { return Ok(value) }
fn Handle(value: int) int {
  return try Source(value) {
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
		"sec.dialect_version = 10 : i32",
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

	tool, configured, err := testsupport.SecMLIROptPathFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		return
	}
	path := filepath.Join(t.TempDir(), "handlers.mlir")
	if err := os.WriteFile(path, output, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(tool, path,
		"--sec-verify-checked-integer-guards", "--sec-verify-result-guards",
		"--sec-verify-try-handlers", "-o", os.DevNull)
	if combined, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sec-mlir-opt: %v\n%s\nGenerated:\n%s", err, combined, output)
	}
}

// The Sec MLIR handler metadata has only the ok, err-variant, err-catch-all,
// and merge kinds and requires catch-all finality and complete coverage, so
// guarded handlers and partial plans are rejected explicitly, while block
// recovery values emit as ordinary branches.
//
// Rules:
//   - rules/mlir/dialect-versions/sec_mlir_dialect_v6.md — §12, §20
func TestSecMLIRRejectsGuardedAndPartialTryHandlers(t *testing.T) {
	sources := map[string]string{
		"guarded": `module main
fn Add(left: int, right: int) int {
  return try left + right {
    Err(error) where left > 0 => 1
    Err(_) => 0
  }
}
`,
		"partial": `module main
fn Divide(left: int, right: int) Result[int, ArithmeticError] {
  let value := try left / right {
    Err(ArithmeticError.DivisionByZero) => 0
  }
  return Ok(value)
}
`,
		"block value": `module main
fn Add(left: int, right: int) int {
  return try left + right {
    Err(_) => {
      let fallback := 7
      fallback
    }
  }
}
`,
	}
	target, _ := findTargetDefinition(CompilerTarget{OS: "linux", Arch: "amd64"})
	plan, err := target.scalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	for name, source := range sources {
		p := parser.New(lexer.NewWithFile(source, name+".sec"))
		parsed := p.Parse()
		a := sema.NewAnalyzer()
		if diagnostics := a.Analyze(parsed.Program); len(diagnostics) != 0 {
			t.Fatalf("%s sema: %v", name, diagnostics)
		}
		module, err := semantic.Build(parsed.Program, a, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{name + ".sec"}, MaxPackage: 10})
		if err != nil {
			t.Fatalf("%s build: %v", name, err)
		}
		_, err = secmlirlowering.Emit(module, plan)
		var unsupported *secmlirlowering.UnsupportedLoweringError
		rejected := errors.As(err, &unsupported)
		if name == "block value" {
			if err != nil {
				t.Fatalf("block value emit: %v", err)
			}
			continue
		}
		if !rejected {
			t.Fatalf("%s emit error = %v, want an explicit unsupported lowering", name, err)
		}
	}
}
