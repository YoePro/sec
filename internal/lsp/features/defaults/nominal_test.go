package defaults

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestNominalMembershipDefaultSource proves that a compiler-owned membership
// default spells the storage's named type and produces valid explicit source,
// including an alias over a concrete generic union with no new type arguments.
// Rules: rules/types/contracts.md — Ordered membership;
// rules/types/default_values.md — Named types, LSP.
func TestNominalMembershipDefaultSource(t *testing.T) {
	data, err := os.ReadFile("../../../../testdata/sema/membership/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	p := parser.New(lexer.New(source))
	a := sema.NewAnalyzer()
	if errors := a.Analyze(p.ParseProgram()); len(errors) > 0 || len(p.Errors()) > 0 {
		t.Fatal(errors, p.Errors())
	}
	for _, name := range []string{"Allowed", "Alias", "Explicit", "GenericAllowed"} {
		value, ok := sourceDefault(a.Types()[name])
		want := name + ".Ready"
		if name == "GenericAllowed" {
			want = name + ".Yes"
		}
		if !ok || value != want {
			t.Fatal(name, value, ok)
		}
		needle := "let mut value: " + name + "\n"
		edited := strings.Replace(source, needle, "let mut value: "+name+" := "+value+"\n", 1)
		if edited == source {
			t.Fatal("missing binding", name)
		}
		p := parser.New(lexer.New(edited))
		if errors := sema.NewAnalyzer().Analyze(p.ParseProgram()); len(errors) > 0 || len(p.Errors()) > 0 {
			t.Fatal(name, errors, p.Errors())
		}
	}
}
