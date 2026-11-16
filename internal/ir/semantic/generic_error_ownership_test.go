package semantic

import (
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// A valid owning E specializes with its concrete ownership: the consuming
// parameter registers with E's destruction plan and transfers into the
// returned Err, and a fresh unit variant is passed by move.
// Rules: rules/compiler/monomorphization.md — §§11.2,11.4; MD-120 §§4(6),11(4–5).
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
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{`Wrap[E=main::OwnedError](main::OwnedError)`, `plan="destroy:main:union:OwnedError"`, "result.err", "(%0[move])"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
}
