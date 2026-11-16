package sema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMethodGenericConformance preserves finite interface signatures: a method's
// own parameters cannot stand in for an ordinary requirement, while a generic
// owner's ordinary methods still satisfy its instantiated interface contract.
// Rules: rules/declarations/generics.md — §§9–10,14,22;
// rules/declarations/interfaces.md — §§4–6,9,12.
func TestMethodGenericConformance(t *testing.T) {
	for _, valid := range []bool{true, false} {
		name := "method_conformance_valid.sec"
		if !valid {
			name = "method_conformance_invalid.sec"
		}
		source, err := os.ReadFile("../../testdata/sema/generics/" + name)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				t.Run(fmt.Sprintf("%s/%d/%s", name, width, depth), func(t *testing.T) {
					p := parser.New(lexer.NewWithFile(string(source), name))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					errors := a.Analyze(program)
					if valid {
						if len(errors) != 0 {
							t.Fatal(errors)
						}
						facts := a.InterfaceConformances()
						if len(facts) < 2 {
							t.Fatalf("missing owner conformance: %+v", facts)
						}
						for _, fact := range facts {
							if fact.State != diagnostics.ProofValid || len(fact.Members) != 1 {
								t.Fatalf("invalid conformance: %+v", fact)
							}
							member := fact.Members[0]
							ordinary := false
							for _, method := range a.Functions()[fact.TypeName+"."+member.Name] {
								if sameSourceToken(method.Token, member.Implementation) {
									ordinary = len(method.GenericParameters) == 0 && method.GenericTypePack == ""
								}
							}
							if !ordinary {
								t.Fatalf("conformance selected a generic helper: %+v", member)
							}
						}
						return
					}
					if len(errors) != 4 {
						t.Fatalf("got %d errors, want 4: %+v", len(errors), errors)
					}
					mismatches, constraints := 0, 0
					for _, diagnostic := range errors {
						if diagnostic.ID == diagnostics.InterfaceMemberIncompatible {
							mismatches++
							if !strings.Contains(diagnostic.Message, "method-level generic") || diagnostic.PreviousLine == 0 || diagnostic.RelatedLabel != "interface requirement" {
								t.Errorf("missing generic mismatch/provenance: %+v", diagnostic)
							}
						} else if strings.Contains(diagnostic.Message, "does not satisfy constraint") {
							constraints++
						} else {
							t.Errorf("unexpected diagnostic: %+v", diagnostic)
						}
					}
					if mismatches != 3 || constraints != 1 {
						t.Fatalf("mismatches/constraints = %d/%d", mismatches, constraints)
					}
					facts := a.InterfaceConformances()
					if len(facts) != 3 {
						t.Fatalf("facts: %+v", facts)
					}
					for _, fact := range facts {
						if fact.State != diagnostics.ProofInvalid || len(fact.Members) != 0 || len(fact.Failures) != 1 {
							t.Fatalf("generic method published as witness: %+v", fact)
						}
					}
				})
			}
		}
	}
}
