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

// P15 carry-over closed in P17: a static of a struct or fixed-array type
// built from integer-family and bool values is statically materialized by
// semantic CTE (literals, ranges and executed Sec code), published as an
// ordered initializer tree and emitted as one aggregate sec.static.global;
// fields and elements are read, borrowed and (for a mutable static) replaced
// through the static.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 9, 32; rules/declarations/static.md — §§ 15–16;
// rules/compiler/semantic_ir.md — § 28.
func TestSchema13AggregateStatics(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

type Pair struct {
    left: int,
    right: int,
}

type Line struct {
    from: Pair,
    flag: bool,
}

fn Seed() int {
    return 7
}

static let origin: Pair := Pair{left: 1, right: 2}
static let table: int[3] := [1, 2, 3]
static let span: int[4] := [1..4]
static let line: Line := Line{from: Pair{left: 3, right: 4}, flag: true}
static let made: Pair := Pair{left: Seed(), right: Seed() + 1}
static let mut counter: Pair := Pair{left: 0, right: 0}

fn Field() int {
    return origin.left + line.from.right + made.right
}

fn Element(i: int) int {
    return table[i] + span[3]
}

fn Borrow() int {
    let r := ref origin.right
    return *r
}

fn Bump() int {
    counter.left = counter.left + 1
    return counter.left
}

fn main() int {
    return 0
}
`
	parsed := parser.New(lexer.NewWithFile(source, "statics.sec")).Parse()
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"statics.sec"}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	text := semantic.Format(module)
	for _, want := range []string{"init={1, 2}", "init={1, 2, 3}", "init={1, 2, 3, 4}", "init={{3, 4}, true}", "init={7, 8}"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing initializer %q in:\n%s", want, text)
		}
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 13})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`sec.static_initializer = [["3", "4"], "true"]`)) {
		t.Errorf("nested aggregate initializer is not emitted:\n%s", output)
	}
	verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if out, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("native verify: %v\n%s", err, out)
	}
}
