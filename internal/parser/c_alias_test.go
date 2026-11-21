package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func parseCAliasProgram(t *testing.T, source string) (*ast.Program, []string) {
	t.Helper()
	p := New(lexer.New(source))
	result := p.Parse()
	return result.Program, p.Errors()
}

// grammar.md § 126(1): @cAlias is a standalone module-scope declaration
// whose target is a TypeReference; consecutive declarations stay separate.
func TestCAliasDeclarationParses(t *testing.T) {
	program, errs := parseCAliasProgram(t, `module amd64

@cAlias(
    name: "c::stddef::size_t",
    target: c::ulong
)
@cAlias(name: "c::example::pointer", target: RawPtr[c::char],)
fn After() void {}
`)
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	var aliases []*ast.CAliasDeclaration
	functions := 0
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *ast.CAliasDeclaration:
			aliases = append(aliases, statement)
		case *ast.FunctionDeclaration:
			functions++
			if len(statement.Attributes) != 0 {
				t.Fatalf("@cAlias attached to the following function: %v", statement.Attributes)
			}
		}
	}
	if len(aliases) != 2 || functions != 1 {
		t.Fatalf("aliases %d, functions %d", len(aliases), functions)
	}
	if aliases[0].Name.Value != "c::stddef::size_t" || aliases[0].Target.Name != "c::ulong" || aliases[0].Invalid {
		t.Fatalf("first = %+v", aliases[0])
	}
	if aliases[1].Name.Value != "c::example::pointer" || aliases[1].Target.Name != "RawPtr" || len(aliases[1].Target.TypeArgs) != 1 ||
		aliases[1].Target.TypeArgs[0].Name != "c::char" || aliases[1].Close.Type != lexer.RPAREN {
		t.Fatalf("second = %+v", aliases[1])
	}
}

// attributes.md § 95(1) and grammar.md § 126(1): malformed arguments,
// misplaced declarations and attributes on @cAlias are reported.
func TestCAliasDeclarationRejections(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"missing target", `@cAlias(name: "c::a::b")`, "requires the named argument target"},
		{"missing name", `@cAlias(target: c::long)`, "requires the named argument name"},
		{"no arguments", `@cAlias`, "requires the arguments"},
		{"positional", `@cAlias("c::a::b", c::long)`, "takes only the named arguments"},
		{"unknown argument", `@cAlias(name: "c::a::b", kind: c::long)`, "has no argument named kind"},
		{"duplicate argument", `@cAlias(name: "c::a::b", name: "c::a::c", target: c::long)`, "duplicate argument name"},
		{"name not a string", `@cAlias(name: size_t, target: c::long)`, "must be a string literal"},
		{"missing separator", `@cAlias(name: "c::a::b" target: c::long)`, "expected ',' or ')'"},
		{"local", "fn F() void {\n    @cAlias(name: \"c::a::b\", target: c::long)\n}", "standalone module-scope declaration"},
		{"in impl", "type T struct {}\nimpl T {\n    @cAlias(name: \"c::a::b\", target: c::long)\n    fn M() void {}\n}", "standalone module-scope declaration"},
		{"attribute on alias", "@target(os: \"linux\")\n@cAlias(name: \"c::a::b\", target: c::long)", "cannot annotate a standalone @cAlias declaration"},
		{"effect attribute on alias", "@noPanic\n@cAlias(name: \"c::a::b\", target: c::long)", "@noPanic may only annotate"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, errs := parseCAliasProgram(t, "module m\n\n"+test.source+"\n")
			if !strings.Contains(strings.Join(errs, "\n"), test.want) {
				t.Fatalf("want %q, got %v", test.want, errs)
			}
		})
	}
}
