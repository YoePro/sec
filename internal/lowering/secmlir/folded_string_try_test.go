package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// A fully compile-time folded concatenation is static string data: the
// ordinary spelling, a bodyless try and a try with local handlers all lower to
// the folded constant, because the try's allocation failure path is
// unreachable and no handler can run (MD-039 § 2(5); operators.md § 29.5).
// A try over a runtime concatenation still stops at the named P23 string
// allocation boundary.
// Rules: rules/foundations/operators.md — §§ 29.5, 29.6;
// rules/corrections/MD-004-MD-039-string-concatenation-try-or-panic-correction-20261031.md — § 2(5).
func TestSchema13FoldedStringTry(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/folded_string_try.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(source), "folded.sec")).Parse()
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if diagnostics := analyzer.Analyze(parsed.Program); len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"folded.sec"}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	text := semantic.Format(module)
	for _, want := range []string{`const.string "AB"`, `const.string "CD"`} {
		if !strings.Contains(text, want) {
			t.Errorf("missing folded constant %s:\n%s", want, text)
		}
	}
	if strings.Contains(text, "result.is-err") || strings.Contains(text, "fallback") {
		t.Errorf("a folded try kept a failure path:\n%s", text)
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 13})
	if err != nil {
		t.Fatal(err)
	}
	verify := exec.Command(tool, "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, result)
	}

	runtime := "module main\n\nfn Tried(name: string) Result[string, StringError] {\n    let s := try \"A\" + name\n    return Ok(s)\n}\n"
	parsed = parser.New(lexer.NewWithFile(runtime, "runtime.sec")).Parse()
	analyzer = sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: "amd64", Profile: "hosted", PointerWidthBits: 64})
	if diagnostics := analyzer.Analyze(parsed.Program); len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	// Runtime string allocation has no preserving representation before P23:
	// the build fails closed instead of lowering a guessed allocation.
	built, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"runtime.sec"}, MaxPackage: 17})
	if err == nil || built != nil || !strings.Contains(err.Error(), "allocation") {
		t.Fatalf("runtime materialization did not stop at its boundary: %v", err)
	}
}
