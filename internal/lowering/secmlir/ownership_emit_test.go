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

// Real source reaches schema 13 with its ownership and cleanup operations and the
// plan table, deterministically, through the slice and P15 verifiers.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v13.md; rules/mlir/packages/sec-mlir-dialect_package17.md — §86.
func TestSchema13EmitSourceOwnership(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/ownership_source_core.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "ownership.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"ownership.sec"}, MaxPackage: 17})
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
				"sec.dialect_version = 13 : i32", "sec.destruction_plans = [", `"sec.own.discard"(`, `"sec.cleanup.defer_register"(`, `"sec.cleanup.defer_yield"(`, `discard_origin = "explicit"`, `discard_origin = "implicit-call-result"`, `"sec.cleanup.track_owned"(`, `"sec.cleanup.run_function"(`,
				`"sec.cleanup.run_scope"(`, `"sec.cleanup.cancel"(`, `"sec.own.move"(`, `"sec.own.move_from_place"(`, `"sec.own.initialize_place"(`, `"sec.own.replace_place"(`,
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
