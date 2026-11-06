package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// MD-126 string views reach verified Sec MLIR on the 32- and 64-bit plans:
// !sec.string_view, sec.string.rune_len/view_borrow, the shared bounded-view
// slice operations over views and rune-by-value decode_next loops.
// Rules: rules/collections/collections.md — §7.10; rules/compiler/semantic_ir.md — §17(9)–(10);
// rules/control-flow/flowcontrol_for.md — §19.
func TestSchema13EmitsStringViews(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/string_views.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "string_views.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"string_views.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			again, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil || !bytes.Equal(output, again) {
				t.Fatal("emission is not deterministic", err)
			}
			for _, want := range []string{
				"!sec.string_view", `"sec.string.view_borrow"(`, `"sec.string.rune_len"(`, `"sec.string_view.decode_next"(`,
				`"sec.string_view.byte_len"(`, `"sec.slice.copy_shared"(`, `"sec.slice.reborrow_shared"(`, `"sec.slice.range_check"(`,
				`(!sec.string_view) -> !sec.uint`, `-> (i1, !sec.rune, !sec.uint)`,
			} {
				if !bytes.Contains(output, []byte(want)) {
					t.Errorf("missing %s in:\n%s", want, output)
				}
			}
			verify := exec.Command(tool, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "--sec-verify-checked-integer-guards", "--sec-verify-result-guards", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
