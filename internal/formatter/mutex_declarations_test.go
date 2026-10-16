package formatter

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestFormatPreservesOpaqueMutexDeclarations checks that formatting preserves
// noCopy, one generic parameter and the absence of source representation fields.
// Rules: rules/concurrency/mutex.md — §§4–6;
// rules/tooling/formatter.md — "Attributes", "Type declarations".
func TestFormatPreservesOpaqueMutexDeclarations(t *testing.T) {
	data, err := os.ReadFile("../../sec/core/mutex.sec")
	if err != nil {
		t.Fatal(err)
	}
	formatted := Format(Source{Text: string(data)}, Options{}).Text
	if again := Format(Source{Text: formatted}, Options{}).Text; again != formatted {
		t.Fatal("not idempotent")
	}
	for _, source := range []string{string(data), formatted} {
		p := parser.New(lexer.New(source))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		count := 0
		for _, statement := range program.Statements {
			declaration, ok := statement.(*ast.TypeDeclStatement)
			if !ok {
				continue
			}
			count++
			if declaration.BaseType != nil || declaration.StructType != nil || len(declaration.GenericParameters) != 1 || len(declaration.Attributes) != 1 || declaration.Attributes[0].Name.Value != "noCopy" {
				t.Fatal(declaration)
			}
		}
		if count != 2 {
			t.Fatal(count)
		}
	}
}

// TestFormatPreservesMutexContextDeclarations keeps the opaque Context and
// operation-local error marker intact before and after formatting.
// Rules: rules/concurrency/mutex.md — §§7–8;
// rules/tooling/formatter.md — "Attributes", "Type declarations".
func TestFormatPreservesMutexContextDeclarations(t *testing.T) {
	data, err := os.ReadFile("../../sec/core/context.sec")
	if err != nil {
		t.Fatal(err)
	}
	formatted := Format(Source{Text: string(data)}, Options{}).Text
	if Format(Source{Text: formatted}, Options{}).Text != formatted {
		t.Fatal("not idempotent")
	}
	for _, source := range []string{string(data), formatted} {
		p := parser.New(lexer.New(source))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		owners, errors := 0, 0
		for _, statement := range program.Statements {
			switch declaration := statement.(type) {
			case *ast.TypeDeclStatement:
				owners++
				if declaration.Name.Value != "Context" || declaration.BaseType != nil || declaration.StructType != nil || len(declaration.GenericParameters) != 0 || len(declaration.Attributes) != 1 || declaration.Attributes[0].Name.Value != "noCopy" {
					t.Fatal(declaration)
				}
			case *ast.EnumDeclaration:
				errors++
				if declaration.Name.Value != "ContextError" || !declaration.ErrorType || len(declaration.Values) != 2 {
					t.Fatal(declaration)
				}
			}
		}
		if owners != 1 || errors != 1 {
			t.Fatal(owners, errors)
		}
	}
}
