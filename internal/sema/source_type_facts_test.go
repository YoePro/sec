package sema

import (
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// Source facts preserve exact interface arguments independently of similarly
// named value bindings. Query results are detached and disappear on reanalysis.
// Rules: rules/tooling/lsp.md — "Hover", "Snapshots"; MD-120 §10(7).
func TestResolvedSourceTypeFacts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/generic_error_parameters/interface.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	file := "interface.sec"
	p := parser.New(lexer.NewWithFile(source, file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	tokens := lexer.NewWithFile(source, file)
	line, column := 0, 0
	for {
		token := tokens.NextToken()
		if token.Type == lexer.EOF {
			break
		}
		if token.Lexeme == "MyInterf" && token.ByteStart == strings.Index(source, "MyInterf[IOError]") {
			line, column = token.Line, token.Column
			break
		}
	}
	typ, ok := a.ResolvedSourceTypeAt(file, line, column)
	if !ok || typ.Kind != InterfaceType || len(typ.TypeArgs) != 1 || typ.TypeArgs[0].Name != "IOError" {
		t.Fatalf("fact: %v %+v", ok, typ)
	}
	if len(typ.InterfaceMethods) != 1 || TypeDisplayName(typ.InterfaceMethods[0].ReturnType) != "Result[void, IOError]" {
		t.Fatalf("stale source requirements: %+v", typ.InterfaceMethods)
	}
	typ.TypeArgs[0] = Type{Name: "string", Kind: StringType}
	typ.InterfaceMethods[0].ReturnType.TypeArgs[1] = Type{Name: "string", Kind: StringType}
	again, ok := a.ResolvedSourceTypeAt(file, line, column)
	if !ok || again.TypeArgs[0].Name != "IOError" || TypeDisplayName(again.InterfaceMethods[0].ReturnType) != "Result[void, IOError]" {
		t.Fatal("query mutated analyzer")
	}
	a.Analyze(parser.New(lexer.New("module main")).ParseProgram())
	if _, ok := a.ResolvedSourceTypeAt(file, line, column); ok {
		t.Fatal("stale source type fact")
	}
}
