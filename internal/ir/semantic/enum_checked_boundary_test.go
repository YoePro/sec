package semantic

import (
	"errors"
	"os"
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// Even contradictory fallible/proven facts cannot authorize an unconditional
// conversion. Unsupported consumers stop before inspecting or lowering operands.
// Rules: rules/declarations/enums.md — §12.1; rules/compiler/semantic_ir.md — §124.
func TestFallibleEnumConversionCannotEmitUnconditionally(t *testing.T) {
	path := "../../../testdata/semantic_ir/enum_checked_boundary/exact.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	call := program.Statements[2].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.CallExpression)
	fact, ok := a.ResolvedEnumConversionOf(call)
	if !ok || !fact.Proof.Proven() || fact.Fallible {
		t.Fatal(fact)
	}
	// Missing proof must also stop before touching the operand, even when the
	// fact's fallible bit was not set.
	unproven := fact
	unproven.Proof = sema.IntegerConversionProof{}
	empty := functionBuilder{owner: &builder{maxPackage: 16}}
	if _, err := empty.buildEnumConversion(call, unproven, 0); err == nil || !strings.Contains(err.Error(), "not proven exact") || empty.nextValue != 0 {
		t.Fatalf("lowered unproven facts: %v", err)
	}
	for _, inconsistentProof := range []bool{false, true} {
		corrupted := fact
		corrupted.Fallible = true
		if !inconsistentProof {
			corrupted.Proof = sema.IntegerConversionProof{}
		}
		fb := functionBuilder{owner: &builder{maxPackage: 16}}
		_, err := fb.buildEnumConversion(call, corrupted, 0)
		var unsupported *UnsupportedFeatureError
		if !errors.As(err, &unsupported) || unsupported.Feature != "fallible enum to integer conversion Closed to uint8 (needs checked failure control flow)" || unsupported.Location.File != path || unsupported.Location.Line != call.Token.Line || fb.nextValue != 0 {
			t.Fatalf("lowered fallible facts: %v", err)
		}
	}
}

// Real accepted source retains the lowering boundary with no partial module;
// exact closed-domain and constant conversions still emit verified operations.
// Rules: rules/declarations/enums.md — §12.1; rules/compiler/semantic_ir.md — §124.
func TestEnumCheckedConversionSourceBoundary(t *testing.T) {
	for _, name := range []string{"fallible", "runtime_member", "exact"} {
		program, a := sliceBoundaryInput(t, "semantic_ir/enum_checked_boundary/"+name+".sec")
		module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
		if name != "exact" {
			var unsupported *UnsupportedFeatureError
			if module != nil || !errors.As(err, &unsupported) {
				t.Fatalf("module=%v err=%v", module, err)
			}
			if unsupported.Feature != "fallible enum to integer conversion Wide to uint8 (needs checked failure control flow)" {
				t.Fatal(unsupported)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, fn := range module.Functions {
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					if op.Kind == OpEnumToInteger {
						count++
						if op.ConversionProof == nil {
							t.Fatal("missing exact proof")
						}
					}
				}
			}
		}
		if count != 2 {
			t.Fatalf("lost exact conversions: %d", count)
		}
	}
}
