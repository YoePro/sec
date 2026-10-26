package cst

import (
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Binding roles are parser facts, distinct from same-spelled field names and
// function parameters. Reapplying roles never duplicates the projected facts.
// Rules: rules/tooling/formatter.md — § 5, § 9(1)–(8), § 29(2).
func TestApplyProgramRolesMarksBindingAlignment(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/binding_alignment.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(input)))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	document := Build(string(input), "")
	for range 2 {
		document.ApplyProgramRoles(program)
		names := map[string]bool{}
		for _, element := range document.Elements {
			if element.HasRole(BindingName) {
				names[element.Text] = true
			}
			if element.HasRole(BindingInitializer) && element.Token.Type != lexer.DECLARE && element.Token.Type != lexer.MOVE_DECLARE {
				t.Fatalf("initializer role on %q", element.Text)
			}
			if element.HasRole(BindingMultiline) && element.Text != "broken" {
				t.Fatalf("unexpected multiline binding: %q", element.Text)
			}
			seen := map[Role]bool{}
			for _, role := range element.Roles {
				if seen[role] {
					t.Fatalf("duplicate role %s on %s", role, element.Text)
				}
				seen[role] = true
			}
		}
		for _, name := range []string{"host", "port", "timeout", "Short", "Longer", "following", "broken"} {
			if !names[name] {
				t.Errorf("missing binding role on %q", name)
			}
		}
		for _, name := range []string{"Endpoint", "Host", "Timeout", "Work"} {
			if names[name] {
				t.Errorf("nonbinding %q received binding role", name)
			}
		}
	}
}
