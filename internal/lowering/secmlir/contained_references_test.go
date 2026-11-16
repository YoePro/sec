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

// P16 carry-over closed in P17: a reference, slice or view stored in an
// aggregate is read out of the aggregate's field Place, never copied with the
// aggregate, with the contained origin Sema tracked for that path (package15
// § 101): `*h.target`, `h.target.left`, passing `h.target`, `v.items[0]`,
// `v.items.Len`, a local aggregate, a payload binding of a reference and of
// a slice. Package 16 keeps its named boundary for these forms.
// Rules: rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 20, 101;
// rules/memory/lifetime_analysis.md — § 11.
func TestSchema13ContainedReferenceReads(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

type Pair struct {
    left: int,
    right: int,
}

type Holder struct {
    target: ref Pair,
    count: int,
}

type Writer struct {
    target: ref mut Pair,
}

type View struct {
    items: ref int[],
}

fn Use(p: ref Pair) int {
    return p.left
}

fn Deref(h: Holder) int {
    let p := *h.target
    return p.left
}

fn Field(h: Holder) int {
    return h.target.left
}

fn Pass(h: Holder) int {
    return Use(h.target)
}

fn Write(w: Writer) void {
    w.target.left = 3
}

fn Index(v: View) int {
    return v.items[0]
}

fn Length(v: View) uint {
    return v.items.Len
}

fn Local() int {
    let pair := Pair{left: 1, right: 2}
    let h := Holder{target: ref pair, count: 1}
    return h.target.right
}

fn Payload(o: Option[ref Pair]) int {
    match o {
        Some(p) => return p.left
        None => return 0
    }
}

type Views union {
    Values(ref int[])
    Missing
}

fn First(views: Views) int {
    return match views {
        Values(found) => found[0]
        Missing => 0
    }
}

fn main() int {
    return 0
}
`
	parsed := parser.New(lexer.NewWithFile(source, "contained.sec")).Parse()
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"contained.sec"}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	text := semantic.Format(module)
	if got := strings.Count(text, " holder {"); got < 8 {
		t.Errorf("want a holder read per contained use (at least 8), got %d:\n%s", got, text)
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 13})
	if err != nil {
		t.Fatal(err)
	}
	verify := exec.Command(tool, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "--sec-verify-cleanups", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if out, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("native verify: %v\n%s", err, out)
	}
}
