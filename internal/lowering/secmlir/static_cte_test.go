package secmlir

import (
	"bytes"
	"math/big"
	"os"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// Frozen static scalar CTE values reuse the existing scalar-global schema on
// both widths and both P15/P16 schema versions. Invalid values emit no bytes.
// Rules: rules/compiler/semantic_ir.md §28; rules/declarations/static.md §§15–16;
// rules/mlir/sec_mlir_dialect.md — sec.static.global and v11.41.3.
func TestStaticScalarCTEEmitsExistingGlobalSchema(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/static_cte/scalars.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(source), "static-cte.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(testPlan(width))
		if errors := a.Analyze(program); len(errors) > 0 {
			t.Fatal(errors)
		}
		m, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 15})
		if err != nil {
			t.Fatal(err)
		}
		for _, schema := range []uint32{11, 12} {
			options := EmitOptions{DialectVersion: schema}
			output, err := EmitWithOptions(m, testPlan(width), options)
			if err != nil {
				t.Fatal(err)
			}
			again, err := EmitWithOptions(m, testPlan(width), options)
			if err != nil || !bytes.Equal(output, again) {
				t.Fatal("nondeterministic scalar static output", err)
			}
			for _, value := range []string{`initial = "42"`, `initial = "52"`, `initial = "-3"`, `initial = "-1"`, `initial = "4099"`, `initial = "3213876088517980551083924184682325205044405987565585670602752"`} {
				if !strings.Contains(string(output), value) {
					t.Fatalf("missing %s", value)
				}
			}
			verifyPackage15MLIR(t, "static_cte", output)
		}
		old := m.Statics[0].Initializer.Integer
		m.Statics[0].Initializer.Integer = new(big.Int).Lsh(big.NewInt(1), uint(width-1))
		if output, err := Emit(m, testPlan(width)); err == nil || len(output) > 0 {
			t.Fatal("forged overflowing static published output", err)
		}
		m.Statics[0].Initializer.Integer = old
	}
}
