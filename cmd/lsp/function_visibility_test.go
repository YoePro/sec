package main

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestOrdinaryPrivateCoreHelperCompletion shares Sema's file-owner boundary
// in general and expected-return completion rather than treating trust as access.
// Rules: rules/foundations/names_scopes_visibility.md — §§12.3, 22;
// rules/tooling/lsp.md — Completion.
func TestOrdinaryPrivateCoreHelperCompletion(t *testing.T) {
	program := &ast.Program{SourceProvenance: map[string]ast.SourceProvenance{}}
	for _, name := range []string{"helpers.sec", "sibling.sec"} {
		data, err := os.ReadFile(filepath.Join("../../testdata/core/private_helpers", name))
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), name))
		parsed := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.Statements = append(program.Statements, parsed.Statements...)
		program.SourceProvenance[name] = ast.SourceCore
	}
	a := sema.NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	result := a.Functions()["__Increment"][0].ReturnType
	for _, file := range []string{"helpers.sec", "sibling.sec"} {
		for _, withExpected := range []bool{false, true} {
			context := completionContext{SourceFile: file}
			if withExpected {
				context.ReturnValue = true
				context.ExpectedType = &result
			}
			items := globalCompletionItems("", a, context)
			private, shared := false, false
			for _, item := range items {
				if item.Label == "__Increment" && item.Kind == 3 {
					private = true
				}
				if item.Label == "_Shared" {
					shared = true
				}
			}
			if private != (file == "helpers.sec") || !shared {
				t.Fatalf("%s expected=%v: private=%v shared=%v", file, withExpected, private, shared)
			}
		}
	}
}

// Rules: rules/library/core-library.md §1.2; rules/tooling/lsp.md — Completion.
func TestCoreUnderscoreHelperCompletionRejectsUntrustedCaller(t *testing.T) {
	program := &ast.Program{SourceProvenance: map[string]ast.SourceProvenance{}}
	for _, name := range []string{"helpers.sec", "sibling.sec"} {
		data, err := os.ReadFile(filepath.Join("../../testdata/core/private_helpers", name))
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), name))
		parsed := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.Statements = append(program.Statements, parsed.Statements...)
		if name == "helpers.sec" {
			program.SourceProvenance[name] = ast.SourceCore
		}
	}
	a := sema.NewAnalyzer()
	a.Analyze(program)
	for _, expected := range []bool{false, true} {
		context := completionContext{SourceFile: "sibling.sec"}
		if expected {
			result := a.Functions()["_Shared"][0].ReturnType
			context.ReturnValue = true
			context.ExpectedType = &result
		}
		assertNoCompletionLabel(t, globalCompletionItems("", a, context), "_Shared")
	}
}
