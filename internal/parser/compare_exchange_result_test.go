package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestCompareExchangeResultDeclaration parses the exact normative source
// without widening ordinary enum declarations to arbitrary associated values.
// Rules: rules/concurrency/atomics.md — §6; rules/declarations/enums.md — §§2–3.
func TestCompareExchangeResultDeclaration(t *testing.T) {
	for _, test := range []struct {
		file  string
		valid bool
	}{
		{"../../sec/core/atomic.sec", true},
		{"../../testdata/sema/atomics/compare_exchange_result/ordinary_payload_invalid.sec", false},
	} {
		data, err := os.ReadFile(test.file)
		if err != nil {
			t.Fatal(err)
		}
		p := New(lexer.NewWithFile(string(data), test.file))
		program := p.ParseProgram()
		if (len(p.Errors()) == 0) != test.valid {
			t.Fatal(test.file, p.Errors())
		}
		if !test.valid {
			continue
		}
		found := false
		for _, statement := range program.Statements {
			if enum, ok := statement.(*ast.EnumDeclaration); ok && enum.Name.Value == "CompareExchangeResult" {
				found = true
				if len(enum.Values) != 2 || enum.Values[1].Payload == nil || enum.Values[1].Payload.Name != "T" {
					t.Fatal(enum)
				}
			}
		}
		if !found {
			t.Fatal("missing enum")
		}
	}
}
