package secmlir

import (
	"bytes"
	"fmt"
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

// SVV-3: static union, Option and Result values are read (static.load), tested
// with `is`, and matched with copied payloads or with borrowed payload Places of
// the static's own storage. A payload Place is guarded by the active-variant
// test of the same static: static.load @N and place.static-root @N denote one
// storage for both the Semantic IR verifier and --sec-verify-places. Mutable
// static variants are stored and re-read. Schema-13 verifiers run on 32- and
// 64-bit targets.
// Rules: rules/compiler/compile_time_evaluation.md — § 32(3);
// rules/declarations/static.md — §§ 15–16, 56; rules/control-flow/flowcontrol_match.md — § 19.
func TestSchema13StaticVariantAccess(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/static_variant_access.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "access.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"access.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			if err := semantic.Verify(module); err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			body := func(name string) string {
				start := strings.Index(text, `func "main::`+name+`(`)
				if start < 0 {
					t.Fatalf("missing function %s", name)
				}
				end := strings.Index(text[start:], "\n  }\n")
				return text[start : start+end]
			}
			for name, wants := range map[string][]string{
				"Copy":         {"static.load @1"},
				"Tested":       {"static.load @2", "union.is-variant"},
				"FieldBorrow":  {"place.static-root @2", "place.union-payload", "kind=static", "ref.borrow-shared"},
				"OptionBorrow": {"place.static-root @4", "place.union-payload"},
				"ResultBorrow": {"result.is-err", "place.static-root @5", "place.union-payload"},
				"Update":       {"static.store @6", "static.store @7"},
				"ReadState":    {"static.load @6", "union.unwrap-payload"},
			} {
				part := body(name)
				for _, want := range wants {
					if !strings.Contains(part, want) {
						t.Errorf("%s lacks %q:\n%s", name, want, part)
					}
				}
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}

// A payload Place of one static is not proven by the active-variant test of
// another static: the Semantic IR verifier rejects it.
func TestStaticVariantPayloadOfAnotherStaticIsRejected(t *testing.T) {
	source := `module main
type Choice union {
    Empty
    Number(int)
}
let A: Choice := Choice.Number(1)
let B: Choice := Choice.Number(2)
fn Read() int {
    match A {
        Number(ref n) => return *n
        Empty => return 0
    }
}
`
	parsed := parser.New(lexer.NewWithFile(source, "other.sec")).Parse()
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"other.sec"}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	// Redirect the guarded payload Place to static B while its guard still
	// tests static A.
	redirected := false
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				if op.Kind == semantic.OpPlaceStaticRoot && op.Static == 1 {
					block.Operations[index].Static = 2
					redirected = true
				}
			}
		}
	}
	if !redirected {
		t.Fatal("no static payload Place to redirect")
	}
	module.Statics[1].AddressTaken = true
	if err := semantic.Verify(module); err == nil || !strings.Contains(err.Error(), "not a payload of the union its guard tested") {
		t.Fatalf("redirected payload Place verified: %v", err)
	}
}
