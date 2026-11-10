package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// TestInterfaceReceiverContract preserves parameter/callable consumption and
// rejects a removed receiver contract even on an AST constructed by a client.
// Rules: rules/declarations/interfaces.md — §3.3.
func TestInterfaceReceiverContract(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/interface_receiver_contract_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	iface := program.Statements[1].(*ast.InterfaceDeclaration)
	iface.Methods[0].ReceiverCapability = ast.ReceiverConsuming
	a := NewAnalyzer()
	errors = a.Analyze(program)
	if len(errors) != 1 || errors[0].ID != diagnostics.InterfaceConsumingReceiver || !strings.Contains(errors[0].Help, "ordinary parameter") {
		t.Fatal(errors)
	}
}
