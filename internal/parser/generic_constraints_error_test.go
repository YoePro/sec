package parser

import (
	"strings"
	"testing"

	"sec/internal/lexer"
)

// MD-120 § 2: `[E: error]` uses the ordinary constraint position, alone and in
// a conjunction; the marker form `[E error]` does not exist.
func TestErrorCategoryConstraintSyntax(t *testing.T) {
	for _, source := range []string{"module main\nfn F[E: error](e: E) void {}\n", "module main\nfn F[E: error & Printable](e: E) void {}\n"} {
		p := New(lexer.New(source))
		p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("%q: %v", source, p.Errors())
		}
	}
	p := New(lexer.New("module main\nfn F[E error](e: E) void {}\n"))
	p.ParseProgram()
	if len(p.Errors()) == 0 || !strings.Contains(strings.Join(p.Errors(), "\n"), "expected ',' or ']' after generic parameter E") {
		t.Fatalf("marker form: %v", p.Errors())
	}
}
