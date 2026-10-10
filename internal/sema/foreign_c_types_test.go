package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func analyzeWithCABIModel(t *testing.T, model layout.CABIModel, body string) []Error {
	t.Helper()
	p := parser.New(lexer.New(`module main
extern "C" fn GetVersion(flags: c::uint) c::int
fn Use(wide: int64, flag: bool, small: int32) void {
` + body + `
}
`))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: model.PointerWidthBits, CABI: model})
	return analyzer.Analyze(result.Program)
}

// c:: fundamental types resolve through the active C ABI model as distinct
// Sec types: explicit conversion is required in both directions, while
// representable literals are shaped directly.
//
// Rules:
//   - rules/platform/ffi.md — §5, §7, §8, §52
//   - rules/platform/abi.md — § 19 "C scalar representation"
func TestCFundamentalTypesAreDistinctAndLiteralShaped(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "representable literal", body: "let v: c::int := 42\ndiscard v"},
		{name: "unrepresentable literal", body: "let v: c::int := 3000000000\ndiscard v", want: "value 3000000000 overflows c::int"},
		{name: "no implicit C to Sec", body: "let v: c::int := 1\nlet s: int32 := v\ndiscard s", want: "cannot initialize int32 with c::int"},
		{name: "no implicit Sec to C", body: "let v: c::int := small\ndiscard v", want: "cannot initialize c::int with int32"},
		{name: "no implicit C to C", body: "let v: c::int := 1\nlet l: c::long := v\ndiscard l", want: "cannot initialize c::long with c::int"},
		{name: "explicit C to Sec", body: "let v: c::int := 1\nlet s := int32(v)\ndiscard s"},
		{name: "explicit Sec to C", body: "let l := c::long(small)\ndiscard l"},
		// ffi.md §8: a conversion that may lose range is checked and needs try.
		{name: "narrowing Sec to C requires try", body: "let n := c::int(wide)\ndiscard n", want: "converting int64 to c::int may fail with ConversionError.OutOfRange and requires try"},
		{name: "narrowing C to Sec requires try", body: "let v := GetVersion(1)\nlet s := int16(v)\ndiscard s", want: "converting c::int to int16 may fail with ConversionError.OutOfRange"},
		{name: "signedness change requires try", body: "let v := GetVersion(1)\nlet u := c::uint(v)\ndiscard u", want: "converting c::int to c::uint may fail with ConversionError.OutOfRange"},
		{name: "checked conversion with handler", body: "let n := try c::int(wide) {\n    Err(_) => c::int(0)\n}\ndiscard n"},
		{name: "widening C to Sec is infallible", body: "let v := GetVersion(1)\nlet s := int64(v)\ndiscard s"},
		{name: "constant conversion is checked at compile time", body: "let n := c::int(42)\ndiscard n"},
		{name: "C bool literal", body: "let b: c::bool := true\ndiscard b"},
		{name: "Sec bool is not C bool", body: "let b: c::bool := flag\ndiscard b", want: "cannot initialize c::bool with bool"},
		{name: "foreign return keeps C identity", body: "let v := GetVersion(1)\nlet s: int := v\ndiscard s", want: "cannot initialize int with c::int"},
		{name: "C double conversion", body: "let d: c::double := 1.5\nlet f := float64(d)\ndiscard f"},
		// MD-022 §2.2–2.4: character literals shape by code-point number.
		{name: "char literal to signed c::char", body: "let a: c::char := 'A'\ndiscard a"},
		{name: "char literal to c::schar", body: "let b: c::schar := 'B'\ndiscard b"},
		{name: "Latin-1 literal to c::uchar", body: "let e: c::uchar := '\u00e9'\ndiscard e"},
		{name: "Latin-1 literal exceeds signed c::char", body: "let e: c::char := '\u00e9'\ndiscard e", want: "character literal '\u00e9' does not fit c::char"},
		{name: "non-Latin literal exceeds c::uchar", body: "let p: c::uchar := '\u03c0'\ndiscard p", want: "does not fit c::uchar"},
		{name: "typed rune is not shaped", body: "let r: rune := 'A'\nlet c: c::char := r\ndiscard c", want: "cannot initialize c::char with rune"},
		{name: "char literal is not shaped to c::int", body: "let i: c::int := 'A'\ndiscard i", want: "cannot initialize c::int with rune"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeWithCABIModel(t, layout.CModelSysVX8664, test.body)
			if test.want == "" {
				if len(errors) != 0 {
					t.Fatalf("errors = %v, want none", errors)
				}
				return
			}
			if len(errors) != 1 || !strings.Contains(errors[0].Message, test.want) {
				t.Fatalf("errors = %v, want one containing %q", errors, test.want)
			}
		})
	}
}

// Physical widths and signedness come from the active model, never from the
// Sec spelling: c::long follows LP64 versus LLP64 and plain c::char follows
// the platform's character signedness.
func TestCFundamentalWidthsFollowTheActiveABIModel(t *testing.T) {
	tests := []struct {
		model layout.CABIModel
		body  string
		valid bool
	}{
		{layout.CModelSysVX8664, "let l: c::long := 3000000000\ndiscard l", true},
		{layout.CModelWindowsX64, "let l: c::long := 3000000000\ndiscard l", false},
		{layout.CModelSysVX8664, "let c: c::char := 200\ndiscard c", false},
		{layout.CModelAAPCS64, "let c: c::char := 200\ndiscard c", true},
		{layout.CModelAAPCS64, "let c: c::schar := 200\ndiscard c", false},
		{layout.CModelAAPCSBareMetal, "let l: c::ulong := 4294967295\ndiscard l", true},
		{layout.CModelAAPCSBareMetal, "let l: c::ulong := 4294967296\ndiscard l", false},
		{layout.CModelSysVX8664, "let ll: c::long_long := -9223372036854775808\ndiscard ll", true},
	}
	for _, test := range tests {
		errors := analyzeWithCABIModel(t, test.model, test.body)
		if test.valid != (len(errors) == 0) {
			t.Fatalf("%s %q errors = %v, want valid %v", test.model.Name, test.body, errors, test.valid)
		}
	}
}

// Unknown fundamentals, unresolved c:: bindings, and targets without a C ABI
// model use their own registered diagnostics.
func TestUnresolvedForeignTypesUseOwningDiagnostics(t *testing.T) {
	tests := []struct {
		model  layout.CABIModel
		body   string
		wantID string
	}{
		{layout.CModelSysVX8664, "let x: c::foo := 1\ndiscard x", diagnostics.ForeignUnknownCFundamentalType},
		{layout.CModelSysVX8664, "let x: c::stddef::size_t := 1\ndiscard x", diagnostics.ForeignUnresolvedCBindingType},
	}
	for _, test := range tests {
		errors := analyzeWithCABIModel(t, test.model, test.body)
		if len(errors) != 1 || errors[0].ID != test.wantID || errors[0].Help == "" {
			t.Fatalf("%q errors = %+v, want one %s with help", test.body, errors, test.wantID)
		}
	}
	errors := analyzeWithCABIModel(t, layout.CABIModel{PointerWidthBits: 64}, "let v: c::int := 1\ndiscard v")
	if len(errors) != 3 {
		t.Fatalf("model-less errors = %v, want three S1077 occurrences", errors)
	}
	for _, err := range errors {
		if err.ID != diagnostics.ForeignCABIModelUnavailable {
			t.Fatalf("model-less error = %+v, want %s", err, diagnostics.ForeignCABIModelUnavailable)
		}
	}
}
