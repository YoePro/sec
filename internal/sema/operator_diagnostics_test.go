package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestOperatorOperandDiagnostics verifies shared mandatory identity, exact
// operator spans and actionable help without changing numeric compatibility.
// Rules: rules/tooling/diagnostics.md — §§5–9,12,15;
// rules/foundations/operators.md — §17.
func TestOperatorOperandDiagnostics(t *testing.T) {
	for _, mode := range []string{"valid", "invalid"} {
		file := "../../testdata/sema/operator_operand_types_" + mode + ".sec"
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				p := parser.New(lexer.NewWithFile(string(data), file))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
				errors := a.Analyze(program)
				want := 0
				if mode == "invalid" {
					want = 4
				}
				if len(errors) != want {
					t.Fatal(mode, width, depth, errors)
				}
				for i, e := range errors {
					line := strings.Split(string(data), "\n")[i+2]
					op := []string{" - ", " * ", " & ", " + "}[i]
					column := strings.Index(line, op) + 2
					if e.ID != diagnostics.OperatorIncompatibleOperands || e.Severity != diagnostics.SeverityError || e.File != file || e.Line != i+3 || e.Column != column || e.EndColumn != column+1 || e.Help == "" {
						t.Fatalf("%d/%s: %+v", width, depth, e)
					}
				}
			}
		}
	}
	definition, ok := diagnostics.Lookup(diagnostics.OperatorIncompatibleOperands)
	if !ok || definition.Name != "operator.incompatible-operand-types" || definition.Family != "operators" || !definition.Mandatory || definition.DefaultSeverity != diagnostics.SeverityError {
		t.Fatal(definition, ok)
	}
}
