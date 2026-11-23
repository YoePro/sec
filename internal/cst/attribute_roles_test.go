package cst

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Real parser delimiters own attribute-list formatting. Runtime @calls and
// malformed attribute lists must never acquire attribute attachment roles.
// Rules: rules/tooling/formatter.md §§5,16(14–17),25.
func TestAttributeArgumentRoles(t *testing.T) {
	for _, tc := range []struct {
		source      string
		lists, ends int
	}{
		{"module main\n@when(true) @target(os: \"linux\") unsafe fn F() void {}", 2, 2},
		{"module main\n@noPanic(1) fn F() void {}", 0, 0},
		{"module main\n@when(,) fn F() void {}", 0, 0},
		{"module main\nfn F() void { @G() }", 0, 0},
	} {
		p := parser.New(lexer.New(tc.source))
		program := p.ParseProgram()
		d := Build(tc.source, "")
		d.ApplyProgramRoles(program)
		lists, ends := 0, 0
		for _, e := range d.Elements {
			if e.HasRole(AttributeArgumentListOpen) {
				lists++
			}
			if e.HasRole(AttachedAttributeEnd) {
				ends++
			}
		}
		if lists != tc.lists || ends != tc.ends {
			t.Fatalf("%s: lists %d ends %d", tc.source, lists, ends)
		}
	}
}
