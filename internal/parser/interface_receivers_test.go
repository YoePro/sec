package parser

import (
	"os"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"testing"
)

// TestConsumingInterfaceReceiverRecovery excludes removed receiver contracts
// while preserving invalid syntax provenance, later members and declarations.
// Rules: rules/declarations/interfaces.md — §3.3;
// rules/compiler/parser_recovery.md — Invalid AST nodes.
func TestConsumingInterfaceReceiverRecovery(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/interface_consuming_receiver_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Diagnostics()) != 2 || len(program.Statements) != 3 {
		t.Fatal(p.Errors(), program.Statements)
	}
	for _, d := range p.Diagnostics() {
		if d.ID != diagnostics.ParserConsumingInterfaceReceiver {
			t.Fatal(d)
		}
	}
	iface := program.Statements[1].(*ast.InterfaceDeclaration)
	if len(iface.InvalidMembers) != 2 || len(iface.Methods) != 3 {
		t.Fatal(iface)
	}
	for _, member := range iface.InvalidMembers {
		if member.Recovery == nil || !member.Recovery.Reported || member.Recovery.End.Line < member.Token.Line {
			t.Fatal(member)
		}
	}
	for _, method := range iface.Methods {
		if method.ReceiverCapability == ast.ReceiverConsuming {
			t.Fatal(method)
		}
	}
	if !iface.Methods[2].Parameters[0].Consuming {
		t.Fatal("ordinary consuming parameter lost")
	}
}
