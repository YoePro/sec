package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// c:: qualifications parse as one foreign-qualified name in type and
// conversion position. `::` is one token; a single segment names a
// fundamental C type and more segments a binding, which Sema distinguishes.
// Ordinary `name: Type` annotations and fields named c or C keep their
// meaning.
//
// Rules:
//   - rules/foundations/grammar.md — ForeignTypeReference
//   - rules/platform/ffi.md — §5, §6, §8, §51 "Parser requirements"
//   - rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md — §2
func TestParseForeignQualifiedNames(t *testing.T) {
	p := New(lexer.New(`module main
extern "C" fn GetVersion(flags: c::uint) c::int
fn Use(c: int) void {
    let size: c::stddef::size_t := 0
    let items: c::char[4] := [0, 0, 0, 0]
    let wide := c::long(c)
    let point := Point { c: 1, C: 2 }
}`))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	extern := program.Statements[1].(*ast.FunctionDeclaration)
	if got := extern.Parameters[0].Type.Name; got != "c::uint" || len(extern.Parameters[0].Type.ForeignSeparators) != 1 ||
		extern.Parameters[0].Type.ForeignSeparators[0].Type != lexer.DOUBLE_COLON {
		t.Fatalf("parameter type = %q separators %+v", got, extern.Parameters[0].Type.ForeignSeparators)
	}
	if extern.ReturnType.Name != "c::int" || extern.ReturnType.Token.Lexeme != "c::int" {
		t.Fatalf("return type = %+v", extern.ReturnType)
	}
	body := program.Statements[2].(*ast.FunctionDeclaration).Body.Statements
	size := body[0].(*ast.LetStatement)
	if size.Type.Name != "c::stddef::size_t" || len(size.Type.ForeignSeparators) != 2 {
		t.Fatalf("binding type = %q separators %d", size.Type.Name, len(size.Type.ForeignSeparators))
	}
	items := body[1].(*ast.LetStatement)
	if items.Type.ElementType == nil || items.Type.ElementType.Name != "c::char" {
		t.Fatalf("array element type = %+v", items.Type)
	}
	call := body[2].(*ast.LetStatement).Value.(*ast.CallExpression)
	callee := call.Function
	if callee == nil || callee.Value != "c::long" || len(callee.ForeignSeparators) != 1 {
		t.Fatalf("conversion callee = %#v", call.Function)
	}
}

func TestParseRejectsMalformedRemovedAndUnsupportedForeignForms(t *testing.T) {
	tests := []struct {
		source string
		wantID string
		want   string
	}{
		{"let x: C::int := 0", compilerdiagnostics.ParserRemovedUppercaseForeignQualifier, "the C:: qualification was removed; write c:: instead"},
		{"let x: c :: int := 0", compilerdiagnostics.ParserMalformedForeignQualification, "contains no whitespace inside or around ::"},
		{"let x: c:: int := 0", compilerdiagnostics.ParserMalformedForeignQualification, "contains no whitespace inside or around ::"},
		{"let x: c::stddef ::size_t := 0", compilerdiagnostics.ParserMalformedForeignQualification, "contains no whitespace inside or around ::"},
		{"let x: c::(1) := 0", compilerdiagnostics.ParserMalformedForeignQualification, "expected a name immediately after c::"},
		{"let f: c::fn() void := 0", compilerdiagnostics.ParserUnsupportedForeignForm, "c::fn is a distinct foreign form"},
		{"let f: c::flex[int] := 0", compilerdiagnostics.ParserUnsupportedForeignForm, "c::flex is a distinct foreign form"},
	}
	for _, test := range tests {
		p := New(lexer.New("module main\nfn Use() void {\n    " + test.source + "\n}\n"))
		p.ParseProgram()
		found := false
		for _, diagnostic := range p.Diagnostics() {
			if diagnostic.ID == test.wantID && strings.Contains(diagnostic.Message, test.want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("%q diagnostics = %+v, want %s containing %q", test.source, p.Diagnostics(), test.wantID, test.want)
		}
	}
	// Split punctuation is two ordinary colons, never joined into ::.
	p := New(lexer.New("module main\nfn Use() void {\n    let x: c: :int := 0\n}\n"))
	p.ParseProgram()
	for _, diagnostic := range p.Diagnostics() {
		if diagnostic.ID == compilerdiagnostics.ParserRemovedUppercaseForeignQualifier {
			t.Fatalf("split colons were treated as a qualification: %+v", diagnostic)
		}
	}
	if len(p.Diagnostics()) == 0 {
		t.Fatal("c: :int was accepted")
	}
}

// `subject is null` and `subject is not null` are the symmetric raw-pointer
// tests; they are ordinary bool expressions, and null is a reserved keyword.
//
// Rules:
//   - rules/platform/ffi.md — §11 "null"
//   - rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md — §5.2–5.3, §5.8
func TestParseNullTests(t *testing.T) {
	p := New(lexer.New(`module main
fn F(raw: RawPtr[int32]) void {
    unsafe {
        if raw is null {
            return
        }
        while raw is not null {
            return
        }
        let present := raw is not null && true
    }
}`))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	block := program.Statements[1].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.UnsafeStatement).Body.Statements
	test, ok := block[0].(*ast.IfStatement).Condition.(*ast.NullTestExpression)
	if !ok || test.Subject.String() != "raw" || test.NullToken.Lexeme != "null" || test.Negated || test.String() != "raw is null" {
		t.Fatalf("if condition = %#v", block[0].(*ast.IfStatement).Condition)
	}
	negated, ok := block[1].(*ast.WhileStatement).Condition.(*ast.NullTestExpression)
	if !ok || !negated.Negated || negated.String() != "raw is not null" {
		t.Fatalf("while condition = %#v", block[1].(*ast.WhileStatement).Condition)
	}
	infix, ok := block[2].(*ast.LetStatement).Value.(*ast.InfixExpression)
	if !ok || infix.Operator != "&&" {
		t.Fatalf("let value = %#v", block[2].(*ast.LetStatement).Value)
	}
	if _, ok := infix.Left.(*ast.NullTestExpression); !ok {
		t.Fatalf("left operand = %#v", infix.Left)
	}

	reserved := New(lexer.New("module main\nfn F() void {\n    let null := 5\n}\n"))
	reserved.ParseProgram()
	if !strings.Contains(strings.Join(reserved.Errors(), "\n"), "`null` is a reserved keyword") {
		t.Fatalf("let null errors = %v", reserved.Errors())
	}
}
