package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestInvalidInterfaceEventPayloadsRetainEventsAndMembers(t *testing.T) {
	input := `
interface Source {
	event Empty[]
	event Malformed[@]
	fn Next() void
}

fn Outside() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected invalid interface event payloads to produce parser errors")
	}
	if len(program.Statements) != 2 {
		t.Fatalf("wrong declaration count. got=%d want=2", len(program.Statements))
	}

	iface, ok := program.Statements[0].(*ast.InterfaceDeclaration)
	if !ok || len(iface.Events) != 2 || len(iface.Methods) != 1 {
		t.Fatalf("invalid event payloads lost interface members: %#v", program.Statements[0])
	}
	for index, event := range iface.Events {
		if event.Payload == nil || !event.Payload.Invalid || event.Payload.Recovery == nil {
			t.Fatalf("event %d did not retain its invalid payload type: %#v", index, event)
		}
	}
	if iface.Methods[0].Name.Value != "Next" {
		t.Fatalf("following interface method was not retained: %#v", iface.Methods[0])
	}
	if outside, ok := program.Statements[1].(*ast.FunctionDeclaration); !ok || outside.Name == nil || outside.Name.Value != "Outside" {
		t.Fatalf("following declaration was not retained: %#v", program.Statements[1])
	}
}
