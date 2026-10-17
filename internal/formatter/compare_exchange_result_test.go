package formatter

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestFormatCompareExchangeResult preserves the exact generic enum and its
// single associated T through formatting, including source documentation.
// Rules: rules/concurrency/atomics.md — §§4,6; rules/tooling/formatter.md — Enum declarations.
func TestFormatCompareExchangeResult(t *testing.T) {
	data, err := os.ReadFile("../../sec/core/atomic.sec")
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
		found := false
		for _, statement := range program.Statements {
			enum, ok := statement.(*ast.EnumDeclaration)
			if !ok || enum.Name.Value != "CompareExchangeResult" {
				continue
			}
			found = true
			if len(enum.GenericParameters) != 1 || len(enum.Values) != 2 || enum.Values[0].Name.Value != "Exchanged" || enum.Values[0].Payload != nil || enum.Values[1].Name.Value != "NotExchanged" || enum.Values[1].Payload == nil || enum.Values[1].Payload.Name != "T" || enum.ErrorType {
				t.Fatal(enum)
			}
		}
		if !found {
			t.Fatal("missing canonical enum")
		}
	}
}
