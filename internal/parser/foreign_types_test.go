package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// C:: and c:: qualifications parse as one foreign-qualified name in type and
// conversion position only when `::` is written contiguously, so ordinary
// `name: Type` annotations and fields named c or C keep their meaning.
//
// Rules:
//   - rules/foundations/grammar.md — ForeignTypeReference
//   - rules/platform/ffi.md — §5, §6, §8, §51 "Parser requirements"
func TestParseForeignQualifiedNames(t *testing.T) {
	p := New(lexer.New(`module main
extern "C" fn GetVersion(flags: C::uint) C::int
fn Use(c: int) void {
    let size: c::stddef::size_t := 0
    let items: C::char[4] := [0, 0, 0, 0]
    let wide := C::long(c)
    let point := Point { c: 1, C: 2 }
}`))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	extern := program.Statements[1].(*ast.FunctionDeclaration)
	if got := extern.Parameters[0].Type.Name; got != "C::uint" || len(extern.Parameters[0].Type.ForeignSeparators) != 2 {
		t.Fatalf("parameter type = %q separators %d", got, len(extern.Parameters[0].Type.ForeignSeparators))
	}
	if extern.ReturnType.Name != "C::int" || extern.ReturnType.Token.Lexeme != "C::int" {
		t.Fatalf("return type = %+v", extern.ReturnType)
	}
	body := program.Statements[2].(*ast.FunctionDeclaration).Body.Statements
	size := body[0].(*ast.LetStatement)
	if size.Type.Name != "c::stddef::size_t" || len(size.Type.ForeignSeparators) != 4 {
		t.Fatalf("binding type = %q separators %d", size.Type.Name, len(size.Type.ForeignSeparators))
	}
	items := body[1].(*ast.LetStatement)
	if items.Type.ElementType == nil || items.Type.ElementType.Name != "C::char" {
		t.Fatalf("array element type = %+v", items.Type)
	}
	call := body[2].(*ast.LetStatement).Value.(*ast.CallExpression)
	callee := call.Function
	if callee == nil || callee.Value != "C::long" || len(callee.ForeignSeparators) != 2 {
		t.Fatalf("conversion callee = %#v", call.Function)
	}
}

func TestParseRejectsMalformedAndUnsupportedForeignForms(t *testing.T) {
	tests := []struct {
		source string
		wantID string
		want   string
	}{
		{"let x: c::size_t := 0", compilerdiagnostics.ParserMalformedForeignQualification, "c:: binding names require a namespace and a name"},
		{"let x: C::(1) := 0", compilerdiagnostics.ParserMalformedForeignQualification, "expected a name immediately after C::"},
		{"let f: C::fn() void := 0", compilerdiagnostics.ParserUnsupportedForeignForm, "C::fn is a distinct foreign form"},
		{"let f: C::flex[int] := 0", compilerdiagnostics.ParserUnsupportedForeignForm, "C::flex is a distinct foreign form"},
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
}

// `subject is null` is a dedicated raw-pointer test in if and while
// conditions; ffi.md §11 defines no negated form.
//
// Rules:
//   - rules/platform/ffi.md — §11 "null"
func TestParseNullTestCondition(t *testing.T) {
	p := New(lexer.New(`module main
fn F(raw: RawPtr[int32]) void {
    unsafe {
        if raw is null {
            return
        }
        while raw is null {
            return
        }
    }
}`))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	block := program.Statements[1].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.UnsafeStatement).Body.Statements
	test, ok := block[0].(*ast.IfStatement).Condition.(*ast.NullTestExpression)
	if !ok || test.Subject.String() != "raw" || test.NullToken.Lexeme != "null" || test.String() != "raw is null" {
		t.Fatalf("if condition = %#v", block[0].(*ast.IfStatement).Condition)
	}
	if _, ok := block[1].(*ast.WhileStatement).Condition.(*ast.NullTestExpression); !ok {
		t.Fatalf("while condition = %#v", block[1].(*ast.WhileStatement).Condition)
	}

	negated := New(lexer.New("module main\nfn F(raw: RawPtr[int32]) void {\n    unsafe {\n        if raw is not null {\n        }\n    }\n}\n"))
	negated.ParseProgram()
	if len(negated.Errors()) == 0 || !strings.Contains(strings.Join(negated.Errors(), "\n"), "is not null is not defined") {
		t.Fatalf("is not null errors = %v", negated.Errors())
	}
}
