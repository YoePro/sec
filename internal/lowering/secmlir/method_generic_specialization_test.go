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

// TestSchema13MethodGenericSpecializations preserves static/instance and owner
// identities through emitted calls and definitions, verified by the ordinary
// schema-13 reference, cleanup and ownership passes on both scalar plans.
// Rules: rules/compiler/generics_lowering.md — §§3–4,7;
// rules/compiler/monomorphization.md — §§6.4,9.3,15.3.
func TestSchema13MethodGenericSpecializations(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/generics/specialization/method_generics.sec")
	if err != nil {
		t.Fatal(err)
	}
	baseline := map[string]string{}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "method_generics.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			if err := semantic.Verify(module); err != nil {
				t.Fatal(err)
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			symbols, err := resolvedFunctionSymbols(module, false)
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			for _, function := range module.Functions {
				if function.Specialization == nil || function.Specialization.Identity == "" {
					continue
				}
				symbol := symbols[function.ID]
				identity := function.Specialization.Identity
				if previous := baseline[identity]; previous != "" && previous != symbol {
					t.Fatalf("target width changed the specialization symbol: %s", function.ID)
				}
				baseline[identity] = symbol
				if seen[symbol] || !bytes.Contains(output, []byte("func.func @"+symbol+"(")) || !bytes.Contains(output, []byte("callee = @"+symbol+"}")) {
					t.Fatalf("missing or merged definition/call: %s", function.ID)
				}
				seen[symbol] = true
			}
			if len(seen) != 8 {
				t.Fatalf("method symbols = %d, want 8", len(seen))
			}
			verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
