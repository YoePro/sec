package sema

import (
	"fmt"
	"os"
	"reflect"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"

	"sec/internal/ast"
)

// TestUnionOwnershipOrdinaryFactoryIsOpaque prevents a static factory with a
// variant-like name from transferring its borrowed argument as a payload.
// Rules: rules/declarations/unions.md §§4,5,7.8;
// rules/memory/copy_move.md §§9–10;
// rules/corrections/applied/return-context-ownership-forwarding-correction-20260903.md §1.
func TestUnionOwnershipOrdinaryFactoryIsOpaque(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/union_ownership_forms_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errs := analyzeSourceWithAnalyzerRaw(t, string(data))
	assertSemaErrors(t, errs, nil)
	found := false
	for call := range a.resolvedCalls {
		if call.Callee.String() != "Factory.Single" {
			continue
		}
		found = true
		if payload, structural := a.unionConstructorPayload(call); structural {
			t.Fatalf("ordinary factory became a constructor of %s", payload.Name)
		}
		if transfer, ok := a.ResolvedTransferPlanOf(call.Arguments[0]); ok && transfer.Action == TransferMove {
			t.Fatalf("borrowed factory argument was consumed: %+v", transfer)
		}
	}
	if !found {
		t.Fatal("missing factory call")
	}
}

// TestUnionOwnershipForms compares the same payload contract across constructor
// AST forms and exercises mandatory validity at all analysis depths/widths.
// Rules: rules/declarations/unions.md §§5,7.8;
// rules/memory/copy_move.md §§9–10; rules/memory/ownership.md §17.
func TestUnionOwnershipForms(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			for _, invalid := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/%s/invalid=%v", width, depth, invalid), func(t *testing.T) {
					suffix := "valid"
					if invalid {
						suffix = "invalid"
					}
					source, err := os.ReadFile("../../testdata/sema/union_ownership_forms_" + suffix + ".sec")
					if err != nil {
						t.Fatal(err)
					}
					p := parser.New(lexer.NewWithFile(string(source), "forms.sec"))
					parsed := p.Parse()
					if parsed.HasErrors {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					errs := a.Analyze(parsed.Program)
					if invalid {
						if len(errs) != 8 {
							t.Fatalf("errors: %+v", errs)
						}
						for i := 0; i < len(errs); i += 2 {
							if errs[i].ID != errs[i+1].ID {
								t.Fatalf("forms disagree: %+v / %+v", errs[i], errs[i+1])
							}
						}
						// The two reuse cases contain valid explicit constructions;
						// only the later moved-source reads are invalid.
						if len(a.resolvedUnionConstructions) != 2 {
							t.Fatalf("unexpected accepted facts: %+v", a.resolvedUnionConstructions)
						}
						for _, plan := range a.resolvedUnionConstructions {
							if len(plan.PayloadPreparation) != 1 || plan.PayloadPreparation[0].Action != StructFieldMove {
								t.Fatal("rejected construction published facts", plan)
							}
						}
						return
					}
					assertSemaErrors(t, errs, nil)
					functions := map[string]*ast.FunctionDeclaration{}
					for _, stmt := range parsed.Program.Statements {
						if fn, ok := stmt.(*ast.FunctionDeclaration); ok {
							functions[fn.Name.Value] = fn
						}
					}
					for _, scenario := range []string{"Explicit", "Terminal", "Fresh", "Copy"} {
						plans := []ResolvedUnionConstruction{}
						for _, form := range []string{"Single", "Named"} {
							statements := functions[form+scenario].Body.Statements
							expr := statements[len(statements)-1].(*ast.ReturnStatement).Value
							plan, found := a.ResolvedUnionConstructionOf(expr)
							if !found || len(plan.PayloadPreparation) != 1 {
								t.Fatal(form, scenario, plan)
							}
							plans = append(plans, plan)
							// Both shapes traverse the recorded ownership edges,
							// even if later consumers cannot inspect AST lists.
							switch outer := expr.(type) {
							case *ast.CallExpression:
								outer.Arguments = nil
							case *ast.StructLiteral:
								outer.Fields = nil
							}
							var operands []ast.Expression
							_, known := a.visitUnionOwnershipOperands(expr, func(source ast.Expression) bool { operands = append(operands, source); return false })
							if !known || len(operands) != 1 || operands[0] != plan.PayloadPreparation[0].Source {
								t.Fatal("ownership depended on constructor AST shape")
							}

						}
						left, right := plans[0].PayloadPreparation[0], plans[1].PayloadPreparation[0]
						if left.Action != right.Action || !reflect.DeepEqual(left.Type, right.Type) || (left.Transfer == nil) != (right.Transfer == nil) {
							t.Fatalf("%s differs: %+v / %+v", scenario, left, right)
						}
						if left.Transfer != nil && (left.Transfer.Action != right.Transfer.Action || left.Transfer.Source.Root != right.Transfer.Source.Root || left.Transfer.OwnershipAfter != right.Transfer.OwnershipAfter) {
							t.Fatalf("%s transfer differs", scenario)
						}
					}
				})
			}
		}
	}
}
