package walk

import (
	"errors"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestInspectGraph pins source order and once-only identity across shared nodes
// and cycles, and checks that consumer failure terminates traversal immediately.
// Rules: rules/compiler/compiler_pipeline.md — lowering prerequisites.
func TestInspectGraph(t *testing.T) {
	shared := &ast.Identifier{Value: "shared"}
	left := &ast.InfixExpression{Left: shared, Right: shared}
	root := &ast.InfixExpression{Left: left}
	root.Right = root
	var got []any
	if err := Inspect(root, func(node any) error { got = append(got, node); return nil }); err != nil {
		t.Fatal(err)
	}
	if want := []any{root, left, shared}; !reflect.DeepEqual(got, want) {
		t.Fatal("source order or identity changed")
	}
	stop := errors.New("stop")
	got = nil
	err := Inspect(root, func(node any) error {
		got = append(got, node)
		if node == left {
			return stop
		}
		return nil
	})
	if err != stop || len(got) != 2 {
		t.Fatal("traversal continued after failure")
	}
}

// TestSpan excludes synthetic tokens while retaining the envelope of source AST
// tokens; caller-owned delimiter recovery may extend this envelope separately.
// Rules: rules/tooling/lsp.md — "Snapshots", "Safe fixes".
func TestSpan(t *testing.T) {
	source := &ast.Identifier{Token: lexer.Token{Line: 1, ByteStart: 12, ByteEnd: 18}}
	synthetic := &ast.Identifier{Token: lexer.Token{ByteStart: 0, ByteEnd: 1000}}
	root := &ast.InfixExpression{Left: source, Right: synthetic, Token: lexer.Token{Line: 1, ByteStart: 20, ByteEnd: 21}}
	start, end, ok := Span(root)
	if !ok || start != 12 || end != 21 {
		t.Fatalf("span = %d %d %v", start, end, ok)
	}
	if _, _, ok := Span(synthetic); ok {
		t.Fatal("synthetic node has source span")
	}
}
