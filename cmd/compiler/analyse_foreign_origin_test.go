package main

import (
	"bytes"
	"os"
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestAnalyseForeignOrigin presents contract-backed origin evidence as advisory
// and keeps the unproven remedy separate from automatic fixes and validity errors.
// Metadata is supplied through the existing trusted API, not invented CLI syntax.
// Rules: rules/analysis/pitfall_analysis.md — "FFI pointer/extent provenance mismatch",
// "sec analyse", "Corrective actions".
func TestAnalyseForeignOrigin(t *testing.T) {
	file := "../../testdata/sema/pitfall_foreign_origin_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzerWithDepth(sema.AnalysisDeep)
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	var call *ast.CallExpression
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if ok && fn.Name.Value == "Mismatch" {
			call = fn.Body.Statements[0].(*ast.UnsafeStatement).Body.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
		}
	}
	resolved, ok := a.ResolvedCallTarget(call)
	if !ok {
		t.Fatal("missing foreign target")
	}
	var contracts sema.ForeignBufferExtentContractStore
	if err := contracts.Record(resolved.Function, []sema.ForeignBufferExtentRelation{{PointerArgument: 0, ExtentArgument: 1, ExtentUnit: sema.ForeignExtentElements, Source: lexer.Token{File: "trusted-metadata", Line: 1, Column: 1}}}); err != nil {
		t.Fatal(err)
	}
	a.SetForeignBufferExtentContracts(&contracts)
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	report := buildAnalyseReport(a, map[string]bool{file: true}, hostCompilerTarget())
	var output bytes.Buffer
	report.write(&output)
	for _, want := range []string{"pitfall.ffi.pointer-extent-origin-mismatch", "the pointer derives from first", "the extent derives from disjoint storage second", "suggested-edit: check that the extent describes", "results: 0 errors"} {
		if !strings.Contains(output.String(), want) {
			t.Fatal(want, output.String())
		}
	}
	if report.counts[analyseClassError] != 0 || report.counts[analyseClassAdvisory] < 4 || strings.Contains(output.String(), "proven-fix:") {
		t.Fatal(report.counts, output.String())
	}
}
