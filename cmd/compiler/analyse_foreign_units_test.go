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

// TestAnalyseForeignUnits transports exact contract/layout evidence as advisory
// without inventing contract-loading CLI syntax or unsafe automatic multiplication.
// Rules: rules/analysis/pitfall_analysis.md — "FFI element count versus byte count",
// "sec analyse", "Fix safety".
func TestAnalyseForeignUnits(t *testing.T) {
	file := "../../testdata/sema/pitfall_foreign_units_valid.sec"
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
	var contracts sema.ForeignBufferExtentContractStore
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok || (fn.Name.Value != "CountAsBytes" && fn.Name.Value != "BytesAsCount") {
			continue
		}
		call := fn.Body.Statements[0].(*ast.UnsafeStatement).Body.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
		target, known := a.ResolvedCallTarget(call)
		if !known {
			t.Fatal("missing target")
		}
		unit := sema.ForeignExtentBytes
		if fn.Name.Value == "BytesAsCount" {
			unit = sema.ForeignExtentElements
		}
		if err := contracts.Record(target.Function, []sema.ForeignBufferExtentRelation{{PointerArgument: 0, ExtentArgument: 1, ExtentUnit: unit, Source: lexer.Token{File: "unit-metadata", Line: 1, Column: 1}}}); err != nil {
			t.Fatal(err)
		}
	}
	a.SetForeignBufferExtentContracts(&contracts)
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	report := buildAnalyseReport(a, map[string]bool{file: true}, hostCompilerTarget())
	var output bytes.Buffer
	report.write(&output)
	for _, want := range []string{"pitfall.ffi.extent-unit-mismatch", "measured in bytes", "measured in elements", "resolved element storage stride is 4 bytes", "suggested-edit: check whether a partial transfer is intentional", "results: 0 errors"} {
		if !strings.Contains(output.String(), want) {
			t.Fatal(want, output.String())
		}
	}
	if report.counts[analyseClassError] != 0 || strings.Contains(output.String(), "proven-fix:") {
		t.Fatal(output.String())
	}
}
