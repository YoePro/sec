package semantic

import (
	"errors"
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestFloatingUnitConversionsRequireLoweringOperations stops exact plans from
// erasing into ordinary carrier-only IR before unit operations are supported.
// Rules: rules/corrections/applied/semantic-ir-units-correction-20260818.md — Erasure boundary.
func TestFloatingUnitConversionsRequireLoweringOperations(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/sema/float_unit_conversion/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	module, err := Build(program, a, BuildOptions{})
	var unsupported *UnsupportedFeatureError
	if module != nil || !errors.As(err, &unsupported) || unsupported.Feature != "exact floating unit conversion" || unsupported.Location.Line == 0 {
		t.Fatal(module, err)
	}
}
