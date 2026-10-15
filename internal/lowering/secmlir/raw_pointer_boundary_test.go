package secmlir

import (
	"errors"
	"os"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// rules/memory/raw_pointers.md §33(10): a Semantic IR raw address must not
// silently become a safe reference or integer in a backend lacking its type.
func TestRawPointerTypeStopsAtUnsupportedMLIRBoundary(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/raw_pointer_values_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "raw_pointer_values_valid.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	module, err := semantic.Build(parsed.Program, a, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 15})
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		output, err := Emit(module, testPlan(width))
		var unsupported *UnsupportedLoweringError
		if !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "raw-pointer") || len(output) != 0 {
			t.Fatalf("%d-bit boundary: output=%q, error=%v", width, output, err)
		}
	}
}
