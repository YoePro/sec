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

// TestSchema13StatementMatchOwnedResults validates terminal arm discards,
// guarded payload transfer, locals, borrowed temporaries and try success/failure
// cleanup through the canonical schema-13 ownership and match verifiers.
// Rules: rules/control-flow/flowcontrol_match.md §19(2)-(6);
// rules/control-flow/discard.md §§9-10; rules/memory/destruction.md §8;
// rules/mlir/packages/sec-mlir-dialect_package17.md §§63,111.
func TestSchema13StatementMatchOwnedResults(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/match_statement_owned.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "match_statement_owned.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"match_statement_owned.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			again, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil || !bytes.Equal(output, again) {
				t.Fatal("unstable arm discard emission", err)
			}
			if got := bytes.Count(output, []byte(`"sec.own.discard"(`)); got != 5 {
				t.Fatalf("want 5 arm-result discards, got %d", got)
			}
			verify := exec.Command(tool, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "--sec-verify-match-cfg", "--sec-verify-union-guards", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
