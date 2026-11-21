package sema

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestUnionOriginAudit compares direct, nested, bound, returned and arm-scoped
// provenance for both constructor forms at every mandatory analysis depth.
// Rules: rules/declarations/unions.md §§5,7.8;
// rules/memory/lifetime_analysis.md §§14,28.
func TestUnionOriginAudit(t *testing.T) {
	for _, invalid := range []bool{true, false} {
		suffix := "valid"
		if invalid {
			suffix = "invalid"
		}
		source, err := os.ReadFile("../../testdata/sema/union_origin_audit_" + suffix + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				t.Run(fmt.Sprintf("%s/%d/%s", suffix, width, depth), func(t *testing.T) {
					p := parser.New(lexer.NewWithFile(string(source), "union-origin.sec"))
					parsed := p.Parse()
					if parsed.HasErrors {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					errs := a.Analyze(parsed.Program)
					if invalid {
						if len(errs) != 12 {
							t.Fatalf("errors: %+v", errs)
						}
						for _, err := range errs {
							if !strings.Contains(err.Message, "reference") || err.PreviousLine == 0 {
								t.Fatalf("lost referent origin: %+v", err)
							}
						}
					} else {
						assertSemaErrors(t, errs, nil)
						checked := 0
						for _, statement := range parsed.Program.Statements {
							fn, ok := statement.(*ast.FunctionDeclaration)
							if !ok || fn.Name.Value != "ForwardSingle" && fn.Name.Value != "ForwardNamed" {
								continue
							}
							expr := fn.Body.Statements[0].(*ast.ReturnStatement).Value
							origin, known := a.expressionReferenceOrigins[expr]
							if !known || len(origin.Contained) != 1 || origin.Local || origin.MatchScoped {
								t.Fatalf("caller origin lost: %+v", origin)
							}
							checked++
							before := a.containedReferenceOrigins(expr)
							switch outer := expr.(type) {
							case *ast.CallExpression:
								outer.Arguments = nil
							case *ast.StructLiteral:
								outer.Fields = nil
							}
							after := a.containedReferenceOrigins(expr)
							if !reflect.DeepEqual(before, after) {
								t.Fatal("origin query reconstructed AST payloads")
							}
							for key, child := range after {
								child.Name = "changed"
								after[key] = child
							}
							if !reflect.DeepEqual(before, a.containedReferenceOrigins(expr)) {
								t.Fatal("contained-origin query shared mutable records")
							}

						}
						if checked != 2 {
							t.Fatal("missing forwarding sources")
						}
					}
				})
			}
		}
	}
}
