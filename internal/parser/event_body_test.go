package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// No event body syntax exists: a block after `event Name using storage` is
// rejected instead of silently skipped, and the event plus later members are
// retained.
//
// Rules:
//   - rules/foundations/grammar.md — "Event declaration"
func TestEventDeclarationBodyIsRejected(t *testing.T) {
	p := New(lexer.New(`module main

impl Button {
    event Pressed using storage {
        let hidden := 1
    }

    fn Press() void {
    }
}
`))
	program := p.ParseProgram()
	if errors := p.Errors(); len(errors) != 1 || !strings.Contains(errors[0], "event Pressed has no body; `event Pressed using storage` declares it completely at 4:33") {
		t.Fatalf("errors = %v", errors)
	}
	impl := program.Statements[1].(*ast.ImplStatement)
	if len(impl.Members) != 2 {
		t.Fatalf("members = %#v, want the event and the later method", impl.Members)
	}
	if event, ok := impl.Members[0].(*ast.EventDeclaration); !ok || event.Name.Value != "Pressed" || event.Storage.Value != "storage" {
		t.Fatalf("event = %#v", impl.Members[0])
	}
}
