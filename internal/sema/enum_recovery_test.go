package sema

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

func TestInvalidEnumRecoveryMemberDoesNotAdvanceIota(t *testing.T) {
	p := parser.New(lexer.New(`
module main

enum Broken int {
	@,
	Present
}
`))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected malformed enum member to produce a parser error")
	}

	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("recovery-only enum member produced dependent semantic errors: %v", errors)
	}
	typ, ok := analyzer.types["Broken"]
	if !ok {
		t.Fatal("recovered enum was not registered")
	}
	present, ok := typ.EnumConsts["Present"]
	if !ok || present.Value == nil || present.Value.Sign() != 0 {
		t.Fatalf("recovery-only member advanced Present's iota value: %#v", present)
	}
}
