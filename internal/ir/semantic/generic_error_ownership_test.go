package semantic

import (
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// Source ownership conformance is independent of the P17 per-specialization
// cleanup gate: a valid owning E must fail by name without a partial IR module.
// Rules: rules/compiler/monomorphization.md — §11.2; MD-120 §§4(6),11(4–5).
func TestGenericErrorOwnershipLoweringBoundary(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/sema/generic_error_ownership/lowering_boundary.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err == nil || module != nil || !strings.Contains(err.Error(), "ownership-sensitive argument E=") {
		t.Fatalf("boundary: module=%v error=%v", module, err)
	}
}
