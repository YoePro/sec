package cst

import (
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Member separators come from parsed receiver/member pairs; decimal points,
// ranges, comment/string contents and a recovered missing member get no role.
// Rules: rules/tooling/formatter.md §§5,17(6–7),25;
// rules/memory/raw_pointers.md §§17(7),18A,36(5).
func TestMemberAccessSeparatorRoles(t *testing.T) {
	for _, name := range []string{"raw_pointer_conversions.sec", "raw_pointer_conversion_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/formatter/" + name)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		p := parser.New(lexer.New(source))
		program := p.ParseProgram()
		valid := name == "raw_pointer_conversions.sec"
		if (len(p.Errors()) == 0) != valid {
			t.Fatalf("unexpected parser result: %v", p.Errors())
		}
		doc := Build(source, "")
		doc.ApplyProgramRoles(program)
		count := 0
		for _, e := range doc.Elements {
			if e.HasRole(MemberAccessSeparator) {
				count++
				if e.Kind != Token || e.Token.Type != lexer.DOT {
					t.Fatalf("incorrect role: %+v", e)
				}
			}
		}
		want := 0
		if valid {
			want = 7
		}
		if count != want || doc.Text() != source {
			t.Fatalf("roles %d, want %d; source changed=%t", count, want, doc.Text() != source)
		}
	}
}
