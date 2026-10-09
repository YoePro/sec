package stringcontracts_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sec/internal/ast"
	"sec/internal/codegen/llvm"
	legacymlir "sec/internal/codegen/mlir"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// Rules: rules/types/contracts.md — String and collection contracts, Conversion
// failure layers; rules/compiler/compiler_known_members.md — string length units.
// Drive the production loader and compiler, compile both target modules and
// execute dynamic input checks with exact first-failure kind/index payloads.
func TestRuntimeStringLengthContracts(t *testing.T) {
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang unavailable")
	}
	dir := t.TempDir()
	compiler := filepath.Join(dir, "sec")
	command := exec.Command("go", "build", "-o", compiler, "../../../../cmd/compiler")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	for _, target := range []string{"linux-amd64", "linux-armv7"} {
		ll := filepath.Join(dir, target+".ll")
		if output, err := exec.Command(compiler, "emit-llvm", "../../../../testdata/codegen/string_contracts/runtime.sec", "--target", target, "-o", ll).CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, output)
		}
		data, err := os.ReadFile(ll)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Replace(string(data), "@main()", "@SecMain()", 1)
		width, triple := "i64", "x86_64-pc-linux-gnu"
		if target == "linux-armv7" {
			width, triple = "i32", "armv7-unknown-linux-gnueabihf"
		}
		for _, name := range []string{"Convert", "ConvertExact", "ConvertNonempty", "ConvertBytes", "ConvertRange", "ConvertDerived", "Forward", "ConvertCall"} {
			text += fmt.Sprintf(`
define i32 @Check%s(ptr %%data, i64 %%len) {
  %%result = call %%sec.string_conversion_result @%s(ptr %%data, i64 %%len)
  %%failed = extractvalue %%sec.string_conversion_result %%result, 0
  %%kind = extractvalue %%sec.string_conversion_result %%result, 3
  %%index = extractvalue %%sec.string_conversion_result %%result, 4
  %%scaled = mul %s %%kind, 100
  %%number = add %s %%scaled, %%index
  %%code = add %s %%number, 1
  %%selected = select i1 %%failed, %s %%code, %s 0
`, name, name, width, width, width, width, width)
			if width == "i64" {
				text += "  %narrow = trunc i64 %selected to i32\n  ret i32 %narrow\n}\n"
			} else {
				text += "  ret i32 %selected\n}\n"
			}
		}
		if err := os.WriteFile(ll, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		object := filepath.Join(dir, target+".o")
		if output, err := exec.Command(clang, "-target", triple, "-c", ll, "-o", object).CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, output)
		}
		if target != "linux-amd64" {
			continue
		}
		harness := filepath.Join(dir, "runtime.c")
		if err := os.WriteFile(harness, []byte(`
#include <stdint.h>
#include <string.h>
extern int CheckConvert(const char*,uint64_t),CheckConvertExact(const char*,uint64_t),CheckConvertNonempty(const char*,uint64_t),CheckConvertBytes(const char*,uint64_t),CheckConvertRange(const char*,uint64_t),CheckConvertDerived(const char*,uint64_t),CheckForward(const char*,uint64_t),CheckConvertCall(const char*,uint64_t);
extern uint64_t Runes(const char*,uint64_t),RuneAlias(const char*,uint64_t),Bytes(const char*,uint64_t);
int main(void) {
    const char *utf8="éΩ",*nonbmp="😀",*combined="é";
    if (Runes(utf8,4)!=2 || RuneAlias(utf8,4)!=2 || Bytes(utf8,4)!=4) return 1;
    if (Runes(nonbmp,4)!=1 || Runes(combined,3)!=2 || Runes("a\0b",3)!=3) return 2;
    if (CheckConvert(utf8,4)!=0 || CheckConvert(nonbmp,4)!=0 || CheckForward(utf8,4)!=0) return 3;
    if (CheckConvert("",0)!=701 || CheckConvert("abc",3)!=802 || CheckConvert("😀é",6)!=1303) return 4;
    if (CheckConvertExact("é",2)!=901 || CheckConvertExact(utf8,4)!=0) return 5;
    if (CheckConvertNonempty("",0)!=1001 || CheckConvertNonempty("\0",1)!=0) return 6;
    if (CheckConvertBytes("é",2)!=1401 || CheckConvertBytes(utf8,4)!=0) return 7;
    if (CheckConvertRange("a",1)!=1201 || CheckConvertRange(utf8,4)!=1302 || CheckConvertRange("é",2)!=0) return 8;
    if (CheckConvertDerived("a",1)!=1404 || CheckConvertDerived(utf8,4)!=0) return 9;
    if (CheckForward("",0)!=701 || CheckConvertCall("",0)!=701 || CheckConvertCall(utf8,4)!=0) return 10;
    return 0;
}
`), 0600); err != nil {
			t.Fatal(err)
		}
		binary := filepath.Join(dir, "runtime")
		if output, err := exec.Command(clang, harness, object, "-o", binary).CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, output)
		}
		if output, err := exec.Command(binary).CombinedOutput(); err != nil {
			t.Fatalf("runtime checks: %v\n%s", err, output)
		}
	}
}

// Rules: rules/types/contracts.md — Core rule; compiler_pipeline.md — lowering prerequisites.
// Raw and stale APIs retain their explicit capability boundary with no partial
// output; the unsupported MLIR path cannot silently drop string requirements.
func TestStringContractFactsRequired(t *testing.T) {
	data, err := os.ReadFile("../../../../testdata/codegen/string_contracts/facts.sec")
	if err != nil {
		t.Fatal(err)
	}
	parse := func() *ast.Program {
		result := parser.New(lexer.NewWithFile(string(data), "facts.sec")).Parse()
		if result.HasErrors {
			t.Fatal(result.Diagnostics)
		}
		return result.Program
	}
	program := parse()
	a := sema.NewAnalyzer()
	if problems := a.Analyze(program); len(problems) != 0 {
		t.Fatal(problems)
	}
	output, err := llvm.GenerateAnalyzed(program, a, "x86_64-pc-linux-gnu")
	if err != nil || output == "" {
		t.Fatal(output, err)
	}
	for _, generate := range []func() (string, error){
		func() (string, error) { return llvm.Generate(program) },
		func() (string, error) { return llvm.GenerateAnalyzed(parse(), a, "x86_64-pc-linux-gnu") },
		func() (string, error) { return legacymlir.GenerateWithTriple(program, "x86_64-pc-linux-gnu") },
		func() (string, error) { return legacymlir.GenerateAnalyzed(program, a, "x86_64-pc-linux-gnu") },
	} {
		output, err := generate()
		var failure *semantic.UnsupportedFeatureError
		if output != "" || !errors.As(err, &failure) || failure.Location.Line == 0 {
			t.Fatal(output, err)
		}
	}
}
