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

// A compiler-known Iterator[T] loop over reusable iterator storage lowers
// from Sema's ResolvedForIteration plan: each iteration borrows the storage
// as Sema planned, calls the resolved concrete Next directly, tests Some and
// binds the copy-trivial payload (or discards it); None and break leave the
// loop. Semantic IR and schema 13 pass every verifier. A fresh temporary
// iterator stops at its named boundary.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — § 37 "Iterator protocol"
//   - rules/compiler/semantic_ir.md — § 65(2), § 66(1)
func TestSchema13IteratorLoops(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/iterator_loops.sec")
	if err != nil {
		t.Fatal(err)
	}
	build := func(text string) (*semantic.Module, error) {
		t.Helper()
		parsed := parser.New(lexer.NewWithFile(text, "iterator.sec")).Parse()
		if parsed.HasErrors {
			t.Fatal(parsed.Diagnostics)
		}
		analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
		if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
			t.Fatal(errors)
		}
		return semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"iterator.sec"}, MaxPackage: 17})
	}
	module, err := build(string(source))
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	text := semantic.Format(module)
	if got := strings.Count(text, "for-iterator"); got != 2 {
		t.Errorf("want 2 iterator loops, got %d:\n%s", got, text)
	}
	for _, want := range []string{
		`call.direct "main::Counter.Next()"(%5[mutable-reference])`,
		"union.is-variant %6 variant=#0", "[loop=@1 edge=condition]",
		"union.unwrap-payload %6 variant=#0[copy-trivial]", "[loop=@1 edge=break]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Semantic IR lacks %q:\n%s", want, text)
		}
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 13})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(output), `"sec.union.is_variant"`); got != 2 {
		t.Errorf("want 2 Some tests in the emitted module, got %d", got)
	}
	verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, result)
	}

	fresh := strings.Replace(string(source), "    let mut counter := Counter { current: 0, limit: 3 }\n    let mut total := 0\n    for value in counter {",
		"    let mut total := 0\n    for value in MakeCounter() {", 1)
	fresh = strings.Replace(fresh, "fn main() int {", "fn MakeCounter() Counter {\n    return Counter { current: 0, limit: 3 }\n}\n\nfn main() int {", 1)
	if fresh == string(source) {
		t.Fatal("fresh-temporary rewrite did not apply")
	}
	if built, err := build(fresh); err == nil || built != nil || !strings.Contains(err.Error(), "fresh temporary iterator") {
		t.Fatalf("a fresh temporary iterator did not stop at its boundary: %v", err)
	}
}
