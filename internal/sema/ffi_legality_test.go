package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func analyzeForeignDeclaration(t *testing.T, declaration string) []Error {
	t.Helper()
	p := parser.New(lexer.New(`module main
enum Mode: uint8 { Off, On }
enum Label: string { Small = "s", Large = "l" }
type Header struct { size: int32 }
type Figure union {
    Dot,
    Circle(int32),
}
interface Drawable {
    fn Draw() void
}
` + declaration + "\n"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse %q: %v", declaration, p.Errors())
	}
	analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64, CABI: layout.CModelSysVX8664})
	return analyzer.Analyze(result.Program)
}

// C and system extern signatures accept C:: scalars, fixed-width Sec
// scalars, RawPtr[T], integer-backed enums, and call-bounded reference
// parameters whose referents are themselves foreign-representable.
//
// Rules:
//   - rules/platform/ffi.md — §5, §9, §10, §12, §21, §45
func TestForeignSignaturesAcceptLegalTypesPerPosition(t *testing.T) {
	for _, declaration := range []string{
		`extern "C" fn Flags(value: C::bool, wide: C::long_double, small: C::uchar) C::int`,
		`extern "C" fn Fixed(a: int8, b: uint16, c: int32, d: uint64, e: float32, f: float64, g: byte) float64`,
		`extern "system" fn Pointer(buffer: RawPtr[byte], untyped: RawPtr[void]) RawPtr[C::char]`,
		`extern "C" fn Borrow(first: ref mut byte, block: ref mut C::int[4], value: ref int32) void`,
		`extern "C" fn Status(mode: Mode) Mode`,
		`unsafe extern "C" fn write(fd: int32, buffer: RawPtr[byte], length: uint) int64`,
	} {
		if errors := analyzeForeignDeclaration(t, declaration); len(errors) != 0 {
			t.Fatalf("%s errors = %v, want none", declaration, errors)
		}
	}
}

// Forbidden types and position-restricted forms are rejected with one
// registered diagnostic naming the position, the type, the ABI, the reason,
// and corrective help.
//
// Rules:
//   - rules/platform/ffi.md — §9, §13, §28, §39, §46, §47, §53
func TestForeignSignaturesRejectIllegalTypesPerPosition(t *testing.T) {
	tests := []struct {
		declaration string
		want        string
	}{
		{`extern "C" fn F(flag: bool) void`, "parameter 1 flag has type bool, which cannot cross the C ABI boundary: Sec bool does not represent C _Bool"},
		{`extern "C" fn F(letter: char) void`, "char does not represent a C character type"},
		{`extern "C" fn F(point: rune) void`, "rune does not represent a C character type"},
		{`extern "C" fn F(text: string) void`, "Sec string has no implicit C string representation"},
		{`extern "C" fn F(amount: decimal) void`, "decimal has no C ABI representation"},
		{`extern "C" fn F(items: int32[]) void`, "an owning dynamic array is a native Sec collection"},
		{`extern "C" fn F(items: ref int32[]) void`, "a Sec slice is a pointer-and-length descriptor"},
		{`extern "C" fn F(block: int32[4]) void`, "fixed arrays describe inline C data but are not passed or returned by value"},
		{`extern "C" fn F() int32[4]`, "return type has type int32[4]"},
		{`extern "C" fn F(value: Option[int32]) void`, "Option has no C representation"},
		{`extern "C" fn F() Result[int32, error]`, "Result has no C representation"},
		{`extern "C" fn F(values: list[int32]) void`, "a native Sec collection has no C representation"},
		{`extern "C" fn F(header: Header) void`, "an ordinary Sec struct has no explicit foreign representation"},
		{`extern "C" fn F(header: ref Header) void`, "its referent Header cannot cross the boundary"},
		{`extern "C" fn F(figure: Figure) void`, "an ordinary Sec tagged union has no C representation"},
		{`extern "C" fn F(callback: fn(int32) int32) void`, "native Sec callable values and closures do not cross a C ABI"},
		{`extern "C" fn F() ref int32`, "returns foreign pointers as RawPtr[T], never as a call-bounded reference"},
		{`extern "C" fn F(label: Label) void`, "an enum without an integer underlying representation has no C representation"},
		{`extern "system" fn F(flag: bool) void`, "cannot cross the system ABI boundary"},
	}
	for _, test := range tests {
		errors := analyzeForeignDeclaration(t, test.declaration)
		if len(errors) != 1 || errors[0].ID != diagnostics.IllegalForeignType || errors[0].Help == "" || !strings.Contains(errors[0].Message, test.want) {
			t.Fatalf("%s errors = %+v, want one S1079 with help containing %q", test.declaration, errors, test.want)
		}
	}
}
