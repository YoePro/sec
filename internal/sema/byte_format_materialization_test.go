package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestByteFormattingMaterializesDigits checks the real core implementation:
// formatted ASCII buffers must use explicit text construction, never the
// universal array ToString type summary.
// Rules: rules/library/core-library.md — §8;
// rules/compiler/compiler_known_members.md — §69.1 (MD-091);
// rules/corrections/applied/md109-core-string-materialization-correction-20261121.md — §§1.2–1.7,2.1–2.6.
func TestByteFormattingMaterializesDigits(t *testing.T) {
	data, err := os.ReadFile("../../sec/core/byte.sec")
	if err != nil {
		t.Fatal(err)
	}
	const file = "sec/core/byte.sec"
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	materializations := 0
	_ = walk.Inspect(program, func(node any) error {
		returned, ok := node.(*ast.ReturnStatement)
		if !ok {
			return nil
		}
		call, ok := returned.Value.(*ast.CallExpression)
		if !ok {
			return nil
		}
		selector, ok := call.Callee.(*ast.MemberExpression)
		if !ok || selector.Property == nil {
			return nil
		}
		token := selector.Property.Token
		member, known := a.CompilerKnownMemberAt(token.File, token.Line, token.Column)
		if !known || member.ID != "CKM-STRING-FROMBYTEARRAY" {
			t.Fatalf("digit-buffer return uses %s, want explicit text materialization; array ToString only formats the collection", call.String())
		}
		materializations++
		return nil
	})
	if materializations == 0 {
		t.Fatal("no digit-buffer materialization found")
	}
}
