package secmlir

import (
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

// A try over a call chain routes each intermediate Result receiver's Err and
// lends its Ok value to the next member, and a match or handler moving an owned
// payload out of an error variant of a fresh Result observes the error through
// the subject's Place. Both reach schema 13 through every verifier, including
// the match-CFG and try-handler verifiers, on the 32- and 64-bit plans.
// Rules: rules/errors/errorhandling.md — §§ 11, 20; rules/memory/destruction.md — § 8(8).
func TestSchema13EmitsTryChainsAndOwnedErrorPayloads(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, fixture := range []struct{ file, want string }{
		{"try_chains.sec", "sec.result.unwrap_ok"},
		{"error_payload_owned.sec", "sec.own.move_from_place"},
		{"try_positions.sec", "sec.array.construct"},
		{"match_guard_owned.sec", "sec.union.unwrap_payload"},
		{"match_borrowed_subjects.sec", `action = "observe"`},
	} {
		source, err := os.ReadFile("../../../testdata/semantic_ir/" + fixture.file)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			t.Run(fmt.Sprint(fixture.file, width), func(t *testing.T) {
				parsed := parser.New(lexer.NewWithFile(string(source), fixture.file)).Parse()
				if parsed.HasErrors {
					t.Fatal(parsed.Diagnostics)
				}
				analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
				if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
					t.Fatal(errors)
				}
				module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{fixture.file}, MaxPackage: 17})
				if err != nil {
					t.Fatal(err)
				}
				output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(output), fixture.want) {
					t.Fatalf("missing %s in:\n%s", fixture.want, output)
				}
				verify := exec.Command(tool, "--sec-verify-result-guards", "--sec-verify-try-handlers", "--sec-verify-match-cfg", "--sec-verify-union-guards",
					"--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
				verify.Stdin = strings.NewReader(string(output))
				if combined, err := verify.CombinedOutput(); err != nil {
					t.Fatalf("%v\n%s\n%s", err, combined, output)
				}
			})
		}
	}
}
