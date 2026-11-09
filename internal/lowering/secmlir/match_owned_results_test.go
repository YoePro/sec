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

// Move-only arm results from locals, payloads and explicit moves reach the
// merge after source cancellation, body cleanup and payload-scope cleanup.
// Rules: rules/control-flow/flowcontrol_match.md — §§21-21.2; rules/memory/destruction.md — § 6.2.
func TestSchema13MatchOwnedResults(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/match_owned_results.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "owned_results.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"owned_results.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-match-cfg", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
