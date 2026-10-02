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
extern "C" fn GetVersion(flags: C::uint) C::int
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

// C:: fundamental types resolve through the active C ABI model as distinct
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
		{name: "representable literal", body: "let v: C::int := 42\ndiscard v"},
		{name: "unrepresentable literal", body: "let v: C::int := 3000000000\ndiscard v", want: "value 3000000000 overflows C::int"},
		{name: "no implicit C to Sec", body: "let v: C::int := 1\nlet s: int32 := v\ndiscard s", want: "cannot initialize int32 with C::int"},
		{name: "no implicit Sec to C", body: "let v: C::int := small\ndiscard v", want: "cannot initialize C::int with int32"},
		{name: "no implicit C to C", body: "let v: C::int := 1\nlet l: C::long := v\ndiscard l", want: "cannot initialize C::long with C::int"},
		{name: "explicit C to Sec", body: "let v: C::int := 1\nlet s := int32(v)\ndiscard s"},
		{name: "explicit Sec to C", body: "let l := C::long(small)\ndiscard l"},
		{name: "C bool literal", body: "let b: C::bool := true\ndiscard b"},
		{name: "Sec bool is not C bool", body: "let b: C::bool := flag\ndiscard b", want: "cannot initialize C::bool with bool"},
		{name: "foreign return keeps C identity", body: "let v := GetVersion(1)\nlet s: int := v\ndiscard s", want: "cannot initialize int with C::int"},
		{name: "C double conversion", body: "let d: C::double := 1.5\nlet f := float64(d)\ndiscard f"},
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
// Sec spelling: C::long follows LP64 versus LLP64 and plain C::char follows
// the platform's character signedness.
func TestCFundamentalWidthsFollowTheActiveABIModel(t *testing.T) {
	tests := []struct {
		model layout.CABIModel
		body  string
		valid bool
	}{
		{layout.CModelSysVX8664, "let l: C::long := 3000000000\ndiscard l", true},
		{layout.CModelWindowsX64, "let l: C::long := 3000000000\ndiscard l", false},
		{layout.CModelSysVX8664, "let c: C::char := 200\ndiscard c", false},
		{layout.CModelAAPCS64, "let c: C::char := 200\ndiscard c", true},
		{layout.CModelAAPCS64, "let c: C::schar := 200\ndiscard c", false},
		{layout.CModelAAPCSBareMetal, "let l: C::ulong := 4294967295\ndiscard l", true},
		{layout.CModelAAPCSBareMetal, "let l: C::ulong := 4294967296\ndiscard l", false},
		{layout.CModelSysVX8664, "let ll: C::long_long := -9223372036854775808\ndiscard ll", true},
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
		{layout.CModelSysVX8664, "let x: C::foo := 1\ndiscard x", diagnostics.ForeignUnknownCFundamentalType},
		{layout.CModelSysVX8664, "let x: c::stddef::size_t := 1\ndiscard x", diagnostics.ForeignUnresolvedCBindingType},
	}
	for _, test := range tests {
		errors := analyzeWithCABIModel(t, test.model, test.body)
		if len(errors) != 1 || errors[0].ID != test.wantID || errors[0].Help == "" {
			t.Fatalf("%q errors = %+v, want one %s with help", test.body, errors, test.wantID)
		}
	}
	errors := analyzeWithCABIModel(t, layout.CABIModel{PointerWidthBits: 64}, "let v: C::int := 1\ndiscard v")
	if len(errors) != 3 {
		t.Fatalf("model-less errors = %v, want three S1077 occurrences", errors)
	}
	for _, err := range errors {
		if err.ID != diagnostics.ForeignCABIModelUnavailable {
			t.Fatalf("model-less error = %+v, want %s", err, diagnostics.ForeignCABIModelUnavailable)
		}
	}
}
