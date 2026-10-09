package authority_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/codegen/llvm"
	legacy "sec/internal/codegen/mlir"
	"sec/internal/lexer"
	mlirtools "sec/internal/mlir"
	"sec/internal/parser"
)

func generatedFixture(t *testing.T, name string, whitespace, reverse bool) *ast.Program {
	t.Helper()
	data, err := os.ReadFile("../../../../testdata/core/generated_identity/" + name)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if whitespace {
		source = "// changed source positions\n\n" + source
	}
	p := parser.New(lexer.NewWithFile(source, name))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	if reverse {
		declarations := []int{}
		for index, stmt := range program.Statements {
			if _, ok := stmt.(*ast.FunctionDeclaration); ok {
				declarations = append(declarations, index)
			}
		}
		for left, right := 0, len(declarations)-1; left < right; left, right = left+1, right-1 {
			a, b := declarations[left], declarations[right]
			program.Statements[a], program.Statements[b] = program.Statements[b], program.Statements[a]
		}
	}
	return program
}

// Rules: rules/compiler/compiler.md §71; compiler_pipeline.md §§76,103;
// rules/declarations/lambda-functions.md — non-capturing callable representation.
func TestGeneratedHelperSymbolsAreDeterministicAndTraceable(t *testing.T) {
	for _, test := range []struct {
		name     string
		generate func(*ast.Program, string) (string, error)
		pattern  string
	}{
		{"lambdas.sec", llvm.GenerateWithTriple, `define private i[0-9]+ @(\.sec\.generated\.lambda-[0-9a-f]+)`},
		{"overloads.sec", legacy.GenerateWithTriple, `llvm.func @"(\.sec\.generated\.overload-[0-9a-f]+)"`},
	} {
		for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
			var baseline []string
			for _, whitespace := range []bool{false, true} {
				for _, reverse := range []bool{false, true} {
					out, err := test.generate(generatedFixture(t, test.name, whitespace, reverse), triple)
					if err != nil {
						t.Fatal(err)
					}
					ids := []string{}
					for _, match := range regexp.MustCompile(test.pattern).FindAllStringSubmatch(out, -1) {
						ids = append(ids, match[1])
					}
					sort.Strings(ids)
					if len(ids) != 2 || ids[0] == ids[1] || !strings.Contains(out, "sec-generated") || !strings.Contains(out, "origin=") || !strings.Contains(out, "source=") {
						t.Fatalf("untraceable helper output: %s", out)
					}
					if baseline == nil {
						baseline = ids
					} else if !reflect.DeepEqual(ids, baseline) {
						t.Fatalf("unstable helpers: %#v != %#v", ids, baseline)
					}
				}
			}
		}
	}
}

// Rules: rules/compiler/compiler_pipeline.md §76(3–4); compiler.md §71.
func TestLLVMGeneratedHelpersDoNotCollideWithSourceNames(t *testing.T) {
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang unavailable")
	}
	for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
		out, err := llvm.GenerateWithTriple(generatedFixture(t, "lambdas.sec", false, false), triple)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		input := filepath.Join(dir, "input.ll")
		object := filepath.Join(dir, "output.o")
		if err := os.WriteFile(input, []byte(out), 0600); err != nil {
			t.Fatal(err)
		}
		if result, err := exec.Command(clang, "-target", triple, "-c", input, "-o", object).CombinedOutput(); err != nil {
			t.Fatalf("clang: %v\n%s", err, result)
		}
		if strings.HasPrefix(triple, "x86_64") {
			executable := filepath.Join(dir, "program")
			if result, err := exec.Command(clang, input, "-o", executable).CombinedOutput(); err != nil {
				t.Fatalf("link: %v\n%s", err, result)
			}
			err := exec.Command(executable).Run()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 44 {
				t.Fatalf("result: %v", err)
			}
		}
	}
}

// Rules: rules/compiler/compiler_pipeline.md §76(2–4); compiler.md §71.
func TestGeneratedHelperRejectsExplicitLinkageCollision(t *testing.T) {
	program := generatedFixture(t, "lambdas.sec", false, false)
	out, err := llvm.GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`define private i64 @(\.sec\.generated\.lambda-[0-9a-f]+)`).FindStringSubmatch(out)
	if len(match) != 2 {
		t.Fatal("no helper identity")
	}
	for _, stmt := range program.Statements {
		if fn, ok := stmt.(*ast.FunctionDeclaration); ok && fn.Name.Value == "__sec_lambda_0" {
			fn.LinkName = match[1]
		}
	}
	out, err = llvm.GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err == nil || out != "" || !strings.Contains(err.Error(), "conflicts with source linkage") {
		t.Fatalf("collision: %q %v", out, err)
	}
}

// Rules: rules/compiler/compiler_pipeline.md §§76,103; compiler.md §71.
func TestGeneratedOverloadsVerifyTranslateAndRun(t *testing.T) {
	tools := mlirtools.NewToolchain("")
	opt, translate := filepath.Join(tools.BinDir, "mlir-opt"), filepath.Join(tools.BinDir, "mlir-translate")
	if _, err := os.Stat(opt); err != nil {
		t.Skip("MLIR tools unavailable")
	}
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang unavailable")
	}
	for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
		text, err := legacy.GenerateWithTriple(generatedFixture(t, "overloads.sec", false, false), triple)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		input := filepath.Join(dir, "input.mlir")
		verified := filepath.Join(dir, "verified.mlir")
		ir := filepath.Join(dir, "output.ll")
		object := filepath.Join(dir, "output.o")
		if err := os.WriteFile(input, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		for _, command := range []*exec.Cmd{
			exec.Command(opt, input, "--verify-each", "-o", verified),
			exec.Command(translate, "--mlir-to-llvmir", verified, "-o", ir),
			exec.Command(clang, "-target", triple, "-c", ir, "-o", object),
		} {
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, output)
			}
		}
		if strings.HasPrefix(triple, "x86_64") {
			binary := filepath.Join(dir, "program")
			if output, err := exec.Command(clang, ir, "-o", binary).CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, output)
			}
			err := exec.Command(binary).Run()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 12 {
				t.Fatalf("result: %v", err)
			}
		}
	}
}
