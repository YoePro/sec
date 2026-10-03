package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Parameters, generic parameters, setter value parameters, and locals must
// not hide a visible declaration of the current module or an always-available
// core declaration; the diagnostic names the hidden declaration.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — §8 Shadowing, §15 Generic parameters, §17 Name lookup order, §20 Diagnostics
func TestShadowingOfVisibleDeclarationsIsRejected(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		id      string
		message string
	}{
		{"parameter shadows module variable", "let limit := 10\nfn F(limit: int) void {}", diagnostics.ParameterShadowsDeclaration, "parameter limit shadows visible variable limit"},
		{"parameter shadows function", "fn Helper() void {}\nfn F(Helper: int) void {}", diagnostics.ParameterShadowsDeclaration, "parameter Helper shadows visible function Helper"},
		{"parameter shadows generic parameter", "fn F[T](T: int) void {}", diagnostics.ParameterShadowsDeclaration, "parameter T shadows visible generic parameter T"},
		{"parameter shadows compiler-known type", "fn F(Event: int) void {}", diagnostics.ParameterShadowsType, "parameter Event shadows visible compiler-known type Event"},
		{"parameter shadows module type", "type Packet int\nfn F(Packet: int) void {}", diagnostics.ParameterShadowsType, "parameter Packet shadows visible type Packet"},
		{"generic parameter shadows function", "fn Helper() void {}\nfn F[Helper]() void {}", diagnostics.GenericParameterShadowsDeclaration, "generic parameter Helper shadows visible function Helper"},
		{"generic parameter shadows variable", "let Limit := 10\nfn F[Limit]() void {}", diagnostics.GenericParameterShadowsDeclaration, "generic parameter Limit shadows visible variable Limit"},
		{"generic parameter shadows compiler-known type", "fn F[Event]() void {}", diagnostics.GenericParameterShadowsType, "generic parameter Event shadows visible compiler-known type Event"},
		{"setter value parameter shadows variable", "let incoming := 3\ntype Counter struct { count: int, }\nimpl Counter {\n    property Count: int {\n        get { return self.count }\n        set incoming { self.count = incoming }\n    }\n}", diagnostics.ParameterShadowsDeclaration, "parameter incoming shadows visible variable incoming"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errors := NewAnalyzer().Analyze(parser.New(lexer.New("module main\n" + test.source + "\n")).ParseProgram())
			found := false
			for _, err := range errors {
				if err.ID == test.id && strings.Contains(err.Message, test.message) {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s %q in %+v", test.id, test.message, errors)
			}
		})
	}
}

func TestShadowingIgnoresQualifiedOnlyAndContextualNames(t *testing.T) {
	tests := map[string]string{
		"lowercase compiler-known type as binding": "fn F(error: int) int {\n    let total := error\n    return total\n}",
		"disjoint sibling scopes":                  "fn F(flag: bool) void {\n    if flag {\n        let result := 1\n    } else {\n        let result := 2\n    }\n}",
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			if errors := NewAnalyzer().Analyze(parser.New(lexer.New("module main\n" + source + "\n")).ParseProgram()); len(errors) != 0 {
				t.Fatalf("unexpected errors: %+v", errors)
			}
		})
	}

	// A function of another module is reachable only qualified, so a
	// parameter or local of the same spelling hides nothing.
	program := parser.New(lexer.NewWithFile("module storage\nfn Open() int { return 1 }\n", "storage.sec")).ParseProgram()
	application := parser.New(lexer.NewWithFile("module application\nfn Use(Open: int) int {\n    let Close := Open\n    return Close\n}\n", "application.sec")).ParseProgram()
	program.Statements = append(program.Statements, application.Statements...)
	if errors := NewAnalyzer().Analyze(program); len(errors) != 0 {
		t.Fatalf("declarations of another module must not count as shadowed: %+v", errors)
	}
}

// A parameter named like a project type is rejected even when another module
// declares a unit of the same spelling; unit symbols do not mask types.
func TestParameterShadowingSeesTypeBesideSameSpelledUnit(t *testing.T) {
	program := unitNamespaceProgram(unitNamespaceUnitsModule, "units.sec", "module application\ntype A int\nfn F(A: int) void {}\n", "application.sec")
	errors := NewAnalyzer().Analyze(program)
	if len(errors) != 1 || errors[0].ID != diagnostics.ParameterShadowsType {
		t.Fatalf("errors = %+v", errors)
	}
}
