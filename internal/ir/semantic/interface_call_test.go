package semantic

import (
	"errors"
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestInterfaceCallSelectionCannotBecomeDirectLowering protects the consumer
// boundary even before runtime interface representation/dispatch is available.
// Rules: rules/declarations/interfaces.md — §6;
// rules/compiler/semantic_ir.md — resolved call facts;
// rules/compiler/compiler_pipeline.md — lowering prerequisites.
func TestInterfaceCallSelectionCannotBecomeDirectLowering(t *testing.T) {
	path := "../../../testdata/sema/interface_calls/selected.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(data), path)).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) > 0 {
		t.Fatal(errors)
	}
	var call *ast.CallExpression
	for _, stmt := range parsed.Program.Statements {
		if fn, ok := stmt.(*ast.FunctionDeclaration); ok && fn.Name.Value == "Integer" {
			call = fn.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.CallExpression)
		}
	}
	if call == nil {
		t.Fatal("missing interface call fixture")
	}
	fb := functionBuilder{owner: &builder{analyzer: analyzer, maxPackage: 14}}
	_, err = fb.buildCall(call)
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) || unsupported.Feature != "interface method dispatch" || unsupported.Location.File != path || unsupported.Location.Line != call.Token.Line || fb.nextValue != 0 {
		t.Fatal(err)
	}
}
