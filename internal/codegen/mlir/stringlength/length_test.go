package stringlength_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	mlir "sec/internal/codegen/mlir"
	"sec/internal/codegen/targetplan"
	"sec/internal/lexer"
	mlirtools "sec/internal/mlir"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Rules: rules/compiler/compiler_known_members.md — Len, RuneLen, ByteLen on strings.
// Verify actual MLIR, translate to LLVM, cross-compile, and execute Unicode counts.
func TestStringRuneAndByteLengthMLIR(t *testing.T) {
	tools := mlirtools.NewToolchain("")
	opt, translate := filepath.Join(tools.BinDir, "mlir-opt"), filepath.Join(tools.BinDir, "mlir-translate")
	if _, err := os.Stat(opt); err != nil {
		t.Skip("MLIR tools unavailable")
	}
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang unavailable")
	}
	data, err := os.ReadFile("../../../../testdata/codegen/string_contracts/lengths.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
		parsed := parser.New(lexer.New(string(data))).Parse()
		if parsed.HasErrors {
			t.Fatal(parsed.Diagnostics)
		}
		plan, err := targetplan.Plan(triple)
		if err != nil {
			t.Fatal(err)
		}
		a := sema.NewAnalyzerWithScalarPlan(plan)
		if errors := a.Analyze(parsed.Program); len(errors) != 0 {
			t.Fatal(errors)
		}
		text, err := mlir.GenerateAnalyzed(parsed.Program, a, triple)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		input, verified, llvm, object := filepath.Join(dir, "input.mlir"), filepath.Join(dir, "verified.mlir"), filepath.Join(dir, "output.ll"), filepath.Join(dir, "output.o")
		if err := os.WriteFile(input, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		for _, command := range []*exec.Cmd{exec.Command(opt, input, "--verify-each", "-o", verified), exec.Command(translate, "--mlir-to-llvmir", verified, "-o", llvm)} {
			if log, err := command.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, log)
			}
		}
		translated, err := os.ReadFile(llvm)
		if err != nil {
			t.Fatal(err)
		}
		translated = []byte(strings.Replace(string(translated), "@main()", "@SecMain()", 1))
		if err := os.WriteFile(llvm, translated, 0600); err != nil {
			t.Fatal(err)
		}
		if log, err := exec.Command(clang, "-target", triple, "-c", llvm, "-o", object).CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, log)
		}
		if triple == "x86_64-pc-linux-gnu" {
			harness, binary := filepath.Join(dir, "harness.c"), filepath.Join(dir, "counts")
			if err := os.WriteFile(harness, []byte(`#include <stdint.h>
extern uint64_t Runes(const char*,uint64_t),RuneAlias(const char*,uint64_t),Bytes(const char*,uint64_t);
int main(void) {return !(Runes("éΩ",4)==2 && RuneAlias("😀",4)==1 && Bytes("éΩ",4)==4 && Runes("a\0b",3)==3);}
`), 0600); err != nil {
				t.Fatal(err)
			}
			if log, err := exec.Command(clang, harness, object, "-o", binary).CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, log)
			}
			if log, err := exec.Command(binary).CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, log)
			}
		}
	}
}
