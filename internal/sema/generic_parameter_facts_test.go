package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Sema retains one fact per generic parameter declaration with its ordered
// constraint conjunction and composed guaranteed methods, and binds referent
// names inside `ref`/`ref mut` types to their declarations so tooling can
// present those facts at every use.
//
// Rules:
//   - rules/declarations/generics.md — §12 "Multiple constraints"
//   - rules/declarations/generics.md — §15 "Operations available on generic parameters"
//   - rules/tooling/lsp.md — "Hover"
func TestGenericParameterFactsKeepOrderedConstraints(t *testing.T) {
	source := `module main

interface Named {
    fn Name() string
}

interface Labelled {
    fn Name() string
    fn Label() string
}

fn Pick[T: Labelled & Named, U](first: ref mut T, second: U) string {
    discard second
    return first.Label()
}
`
	program := parser.New(lexer.New(source)).ParseProgram()
	function := program.Statements[3].(*ast.FunctionDeclaration)
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}

	fact, ok := analyzer.GenericParameterFactAt(function.GenericParameters[0].Name.Token)
	if !ok {
		t.Fatal("missing generic parameter fact for T")
	}
	if got := strings.Join(GenericConstraintDisplays(fact.Name, fact.Constraints), " & "); got != "Labelled & Named" {
		t.Fatalf("T constraints = %q", got)
	}
	methods := []string{}
	for _, method := range fact.GuaranteedMethods {
		methods = append(methods, method.Name)
	}
	if strings.Join(methods, ",") != "Name,Label" {
		t.Fatalf("T guaranteed methods = %v", methods)
	}
	unconstrained, ok := analyzer.GenericParameterFactAt(function.GenericParameters[1].Name.Token)
	if !ok || len(unconstrained.Constraints) != 0 || len(unconstrained.GuaranteedMethods) != 0 {
		t.Fatalf("U fact = %+v, ok=%v", unconstrained, ok)
	}

	header := GenericParameterDisplays(analyzer.Functions()["Pick"][0].GenericParameters, analyzer.Functions()["Pick"][0].GenericConstraints)
	if strings.Join(header, ", ") != "T: Labelled & Named, U" {
		t.Fatalf("header = %v", header)
	}

	referent := function.Parameters[0].Type.ReferentToken
	if referent.Lexeme != "T" {
		t.Fatalf("ref mut referent token = %+v", referent)
	}
	definitions := analyzer.DefinitionsAt(referent.File, referent.Line, referent.Column)
	if len(definitions) != 1 || !sameSourceToken(definitions[0], function.GenericParameters[0].Name.Token) {
		t.Fatalf("ref mut T definitions = %+v", definitions)
	}
}

// Parameters named by a generic impl target are the target declaration's own
// parameters, so the impl scope keeps their ordered constraints: method
// parameters may call guaranteed methods, unguaranteed members stay rejected,
// and the retained fact names the owning target.
//
// Rules:
//   - rules/declarations/generics.md — §8 "Generic impl blocks"
//   - rules/declarations/generics.md — §15 "Operations available on generic parameters"
func TestGenericImplParametersKeepTargetConstraints(t *testing.T) {
	source := `module main

interface Named {
    fn Name() string
}

interface Ranked {
    fn Rank() int
}

type Pair[K: Ranked & Named, V] struct {
    key: K,
    value: V,
}

impl Pair[K, V] {
    fn Describe(other: K, extra: V) string {
        discard other.Rank()
        discard extra.Name()
        return other.Name()
    }
}
`
	program := parser.New(lexer.New(source)).ParseProgram()
	impl := program.Statements[4].(*ast.ImplStatement)
	analyzer := NewAnalyzer()
	errors := analyzer.Analyze(program)
	assertSemaErrors(t, errors, []string{"unknown function or type extra.Name at 19:22"})

	fact, ok := analyzer.GenericParameterFactAt(impl.Target.TypeArgs[0].Token)
	if !ok || fact.ImplTarget != "Pair" {
		t.Fatalf("impl K fact = %+v, ok=%v", fact, ok)
	}
	if got := strings.Join(GenericConstraintDisplays(fact.Name, fact.Constraints), " & "); got != "Ranked & Named" {
		t.Fatalf("impl K constraints = %q", got)
	}
	unconstrained, ok := analyzer.GenericParameterFactAt(impl.Target.TypeArgs[1].Token)
	if !ok || unconstrained.ImplTarget != "Pair" || len(unconstrained.Constraints) != 0 {
		t.Fatalf("impl V fact = %+v, ok=%v", unconstrained, ok)
	}
}
