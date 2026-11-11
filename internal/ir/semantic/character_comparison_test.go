package semantic

import (
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// All six parameter comparisons preserve their character operand identities,
// source provenance and bool result through int.compare on both scalar plans.
// The verifier rejects mixed character identities, including equal-width
// integer representations, rather than treating representation as compatibility.
// Rules: rules/compiler/semantic_ir.md §125; rules/foundations/operators.md §45;
// rules/corrections/applied/md080-md082-md096-md098-md104-md105-correction-20261018.md §7.
func TestCharacterComparisonConformance(t *testing.T) {
	file := "../../../testdata/build/native/character_predicates.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		m, err := Build(program, a, BuildOptions{RequestedModule: "main", SourceFiles: []string{file}})
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		if m.PointerWidthBits != width {
			t.Fatal("lost selected scalar plan", m.PointerWidthBits, width)
		}
		functions := 0
		kindOf := func(id TypeID) TypeKind { typ, _ := m.Types.Lookup(id); return typ.Kind }
		for _, fn := range m.Functions {
			if fn.Name != "CompareChar" && fn.Name != "CompareRune" {
				continue
			}
			functions++
			kind := TypeChar
			if fn.Name == "CompareRune" {
				kind = TypeRune
			}
			if len(fn.Parameters) != 2 || kindOf(fn.Parameters[0].Value.Type) != kind || fn.Parameters[0].Value.Type != fn.Parameters[1].Value.Type {
				t.Fatal(fn.Name, fn.Parameters)
			}
			predicates := map[IntegerComparePredicate]bool{}
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					if op.Kind != OpIntCompare {
						continue
					}
					predicates[op.IntegerCompare] = true
					if op.Location.File != file || op.Location.Line == 0 || op.Location.Column == 0 || kindOf(op.Results[0].Type) != TypeBool {
						t.Fatal(op)
					}
				}
			}
			if len(predicates) != 6 {
				t.Fatal("lost predicate", fn.Name, predicates)
			}
			// Mutate just the second input identity: same operation and result,
			// but an illegal mixed-type compare must fail verification.
			original := fn.Parameters[1].Value.Type
			for index, typ := range m.Types.All() {
				id := TypeID(index + 1)
				if typ.Kind != TypeChar && typ.Kind != TypeRune {
					continue
				}
				if id == original {
					continue
				}
				fn.Parameters[1].Value.Type = id
				if err := Verify(m); err == nil || !strings.Contains(err.Error(), "int.compare") {
					t.Fatal("mixed character comparison admitted", err)
				}
			}
			bits, name := uint16(8), "uint8"
			if kind == TypeRune {
				bits, name = 32, "uint32"
			}
			fn.Parameters[1].Value.Type = m.Types.Intern(Type{Kind: TypeUint, Name: name, BitWidth: bits})
			if err := Verify(m); err == nil || !strings.Contains(err.Error(), "int.compare") {
				t.Fatal("representation confused with character identity", err)
			}
			fn.Parameters[1].Value.Type = original
		}
		if functions != 2 {
			t.Fatal("missing comparison bodies", functions)
		}
	}
}
