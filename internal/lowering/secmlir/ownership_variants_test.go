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

// Option, Result and single-payload unions that need destruction reach schema 13
// through every verifier: constructors, locals, parameters, discards, by-value
// payload bindings and try, on the 32- and 64-bit plans.
// Rules: rules/memory/destruction.md — § 16; rules/control-flow/flowcontrol_match.md — §§ 13, 18;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 14, 25–26, 63.
func TestSchema13EmitsOwnedVariants(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/ownership_variants.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "variants.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"variants.sec"}, MaxPackage: 17})
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
				`kind = "active-variant"`, `variants = [{variant = 0 : i32`, `payloadActions = ["construct-direct"]`, `payloadAction = "move"`,
				`"sec.result.unwrap_ok"(`, `cause = "temporary-end"`, `"sec.place.read"(%v`, `action = "observe"`, `action = "candidate"`, `if_owned`, `if_active`, `"sec.place.union_payload"(`, `"sec.own.move_from_place"(`, `"sec.own.initialize_place"(`,
			} {
				if !bytes.Contains(output, []byte(want)) {
					t.Errorf("missing %s in:\n%s", want, output)
				}
			}
			verify := exec.Command(tool, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
