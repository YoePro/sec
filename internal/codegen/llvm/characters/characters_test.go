package characters_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"sec/internal/codegen/llvm"
	"sec/internal/codegen/targetplan"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

func source(t *testing.T, name string) *ast.Program {
	t.Helper()
	data, err := os.ReadFile("../../../../testdata/codegen/characters/" + name + ".sec")
	if err != nil {
		t.Fatal(err)
	}
	result := parser.New(lexer.NewWithFile(string(data), name+".sec")).Parse()
	if result.HasErrors {
		t.Fatal(result.Diagnostics)
	}
	return result.Program
}

func analyzed(t *testing.T, triple string) (*ast.Program, *sema.Analyzer) {
	t.Helper()
	program := source(t, "scalars")
	plan, err := targetplan.Plan(triple)
	if err != nil {
		t.Fatal(err)
	}
	analyzer := sema.NewAnalyzerWithScalarPlan(plan)
	if problems := analyzer.Analyze(program); len(problems) != 0 {
		t.Fatal(problems)
	}
	return program, analyzer
}

// Rules: rules/types/types.md — Character literal, Context shaping; MD-043 §§2–3.
// Scalar values cannot determine source identity, storage width or match type.
func TestResolvedCharacterWidths(t *testing.T) {
	for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
		t.Run(triple, func(t *testing.T) {
			program, analyzer := analyzed(t, triple)
			output, err := llvm.GenerateAnalyzed(program, analyzer, triple)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{
				"define i8 @ByteChar()", "ret i8 65", "ret i8 233",
				"define i32 @ASCIIRune()", "ret i32 65", "ret i32 937",
				"ret i32 128512", "ret i32 65533", "ret i8 0",
				"define i8 @NamedChar()", "define i32 @NamedRune()",
				"store i32 65", "store i32 233", "call i8 @CharIdentity(i8 233)",
				"call i32 @RuneIdentity(i32 65)", "store i32 937",
			} {
				if !strings.Contains(output, want) {
					t.Fatalf("missing %q:\n%s", want, output)
				}
			}
			if strings.Contains(output, "zext i8") {
				t.Fatalf("rune literal widened after value-based inference:\n%s", output)
			}
		})
	}
}

// Rules: rules/compiler/compiler_pipeline.md — lowering prerequisites; MD-043 §§2–3.
// Raw APIs and stale/unresolved facts must return no partial LLVM module.
func TestCharacterFactsRequired(t *testing.T) {
	program, analyzer := analyzed(t, "x86_64-pc-linux-gnu")
	for _, generate := range []func(*ast.Program) (string, error){
		llvm.Generate, llvm.NewGenerator().Generate,
		func(p *ast.Program) (string, error) { return llvm.GenerateWithTriple(p, "x86_64-pc-linux-gnu") },
		func(p *ast.Program) (string, error) {
			return llvm.GenerateAnalyzed(p, sema.NewAnalyzer(), "x86_64-pc-linux-gnu")
		},
		func(p *ast.Program) (string, error) {
			return llvm.GenerateAnalyzed(source(t, "scalars"), analyzer, "x86_64-pc-linux-gnu")
		},
	} {
		output, err := generate(program)
		var failure *semantic.UnsupportedFeatureError
		if output != "" || !errors.As(err, &failure) || failure.Location.Line == 0 || failure.Location.Column == 0 || failure.Location.File != "scalars.sec" {
			t.Fatalf("output %q, error %v", output, err)
		}
	}
}

// Rules: rules/types/types.md — char and rune domains; MD-043 §2 and §3.3.
// Invalid source is rejected by Sema; malformed ASTs cannot truncate scalars.
func TestCharacterDomainFailures(t *testing.T) {
	if problems := sema.NewAnalyzer().Analyze(source(t, "domain_invalid")); len(problems) == 0 {
		t.Fatal("out-of-domain char accepted")
	}
	for _, damaged := range []string{"Ω", "AB", string([]byte{0xff}), ""} {
		program, analyzer := analyzed(t, "x86_64-pc-linux-gnu")
		var literal *ast.CharLiteral
		err := astwalk.Inspect(program, func(node any) error {
			if c, ok := node.(*ast.CharLiteral); ok && literal == nil {
				literal = c
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		literal.Value = damaged
		output, err := llvm.GenerateAnalyzed(program, analyzer, "x86_64-pc-linux-gnu")
		if output != "" || err == nil {
			t.Fatalf("malformed %q: output %q, error %v", damaged, output, err)
		}
	}
}

// Rules: rules/types/types.md — char and rune; MD-043 §§2–3.
// Compile both target modules and execute scalar storage/calls/match on the host.
func TestCharacterLLVMExecution(t *testing.T) {
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang unavailable")
	}
	dir := t.TempDir()
	for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
		program, analyzer := analyzed(t, triple)
		output, err := llvm.GenerateAnalyzed(program, analyzer, triple)
		if err != nil {
			t.Fatal(err)
		}
		// The harness supplies main; only rename the fixture entry point.
		output = strings.Replace(output, "@main()", "@SecMain()", 1)
		path := filepath.Join(dir, triple+".ll")
		if err := os.WriteFile(path, []byte(output), 0600); err != nil {
			t.Fatal(err)
		}
		object := filepath.Join(dir, triple+".o")
		if log, err := exec.Command(clang, "-target", triple, "-c", path, "-o", object).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", triple, err, log)
		}
		if triple != "x86_64-pc-linux-gnu" {
			continue
		}
		harness := filepath.Join(dir, "harness.c")
		if err := os.WriteFile(harness, []byte(`
#include <stdint.h>
extern uint8_t ByteChar(void), LatinChar(void), Zero(void), NamedChar(void), CharCall(void);
extern uint32_t ASCIIRune(void), UnicodeRune(void), NonBMP(void), Replacement(void), NamedRune(void), InferredASCII(void), InferredLatin(void), RuneCall(void), Select(int64_t);
int main(void) {
    return !(ByteChar()==65 && LatinChar()==233 && Zero()==0 && NamedChar()==233 && CharCall()==233 && ASCIIRune()==65 && UnicodeRune()==937 && NonBMP()==128512 && Replacement()==65533 && NamedRune()==65 && InferredASCII()==65 && InferredLatin()==233 && RuneCall()==65 && Select(0)==65 && Select(1)==937);
}
`), 0600); err != nil {
			t.Fatal(err)
		}
		binary := filepath.Join(dir, "characters")
		if log, err := exec.Command(clang, harness, object, "-o", binary).CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, log)
		}
		if log, err := exec.Command(binary).CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, log)
		}
	}
}
