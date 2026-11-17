package parser

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// Source that still uses `new` as a name gets one targeted migration
// diagnostic in every declaration position — function, method, type, unit,
// interface, parameter (function and lambda), field, enum value, generic
// parameter, property, loop binding and match binding — instead of a generic
// missing-identifier error; let bindings keep their reserved-keyword message.
//
// Rules:
//   - rules/foundations/lexical_structure.md — § 7.2
//   - rules/declarations/impl.md — revision 2.0 source-compatibility change
func TestNewKeywordNameMigrationDiagnostics(t *testing.T) {
	for _, check := range []struct{ source, role string }{
		{"fn new() void {}", "a name"},
		{"fn F(new: int) void {}", "a parameter name"},
		{"type new struct { x: int }", "a name"},
		{"type P struct { new: int }", "a field name"},
		{"enum E { new }", "an enum value name"},
		{"fn G[new]() void {}", "a generic parameter name"},
		{"fn I() void {\n    for new in 0..3 {\n    }\n}", "a loop binding name"},
		{"type Q struct { x: int }\nimpl Q {\n    fn new() void {}\n}", "a name"},
		{"type R struct { x: int }\nimpl R {\n    property new: int {\n        get {\n            return 1\n        }\n    }\n}", "a property name"},
		{"fn J(value: Option[int]) void {\n    match value {\n        Some(new) => {}\n        None => {}\n    }\n}", "a match binding name"},
		{"unit new", "a name"},
		{"interface new {\n}", "a name"},
		{"fn K() void {\n    let f := fn(new: int) int {\n        return new\n    }\n}", "a parameter name"},
	} {
		result := New(lexer.New("module main\n\n" + check.source + "\n")).Parse()
		if len(result.Diagnostics) == 0 {
			t.Errorf("%q: no diagnostic", check.source)
			continue
		}
		first := result.Diagnostics[0]
		want := "`new` is a hard keyword since lifecycle construction (`new Type(...)`) and cannot be used as " + check.role + "; rename it"
		if first.ID != diagnostics.ParserMisplacedKeyword || !strings.Contains(first.Message, want) {
			t.Errorf("%q: first diagnostic = %s %q, want %s %q", check.source, first.ID, first.Message, diagnostics.ParserMisplacedKeyword, want)
		}
	}
	// Another keyword in a name position keeps its existing diagnostic.
	result := New(lexer.New("module main\n\nfn if() void {}\n")).Parse()
	if len(result.Diagnostics) == 0 || strings.Contains(result.Diagnostics[0].Message, "hard keyword since lifecycle construction") {
		t.Errorf("a non-new keyword got the new migration diagnostic: %+v", result.Diagnostics)
	}
}
