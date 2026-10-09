package corehelpers_test

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

func generatedProgram(t *testing.T, files []string, whitespace bool) *ast.Program {
	t.Helper()
	result := &ast.Program{}
	for _, file := range files {
		data, err := os.ReadFile("../../../testdata/core/generated_identity/" + file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		if whitespace {
			source = "// shifted positions\n\n" + source
			source = strings.ReplaceAll(source, "    ", "        ")
		}
		p := parser.New(lexer.NewWithFile(source, file))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		result.Statements = append(result.Statements, program.Statements...)
	}
	return result
}

// Rules: rules/compiler/compiler.md §71; compiler_pipeline.md §§76,103;
// rules/memory/arena.md §§4.2,44 (abstract frontend creation-site domains).
func TestGeneratedResourceIdentityDeterminismAndProvenance(t *testing.T) {
	var baseline []string
	for _, order := range [][]string{{"arena_a.sec", "arena_b.sec"}, {"arena_b.sec", "arena_a.sec"}} {
		for _, whitespace := range []bool{false, true} {
			program := generatedProgram(t, order, whitespace)
			a := sema.NewAnalyzer()
			for attempt := 0; attempt < 2; attempt++ {
				if errors := a.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				facts := a.GeneratedIdentities()
				if len(facts) != 3 {
					t.Fatalf("creation facts: %#v", facts)
				}
				ids := []string{}
				for _, fact := range facts {
					if fact.Kind != "arena-domain" || fact.Source.File == "" || fact.Source.Line < 1 || fact.Origin == "" || fact.Owner == "" {
						t.Fatalf("untraceable identity: %#v", fact)
					}
					ids = append(ids, fact.ID)
					if !strings.HasPrefix(fact.ID, "$arena-domain-") {
						t.Fatalf("resource provenance lost domain namespace: %#v", fact)
					}
				}
				if baseline == nil {
					baseline = ids
				} else if !reflect.DeepEqual(baseline, ids) {
					t.Fatalf("unstable identity: %#v != %#v", ids, baseline)
				}
				facts[0].ID = "poisoned"
				if a.GeneratedIdentities()[0].ID == "poisoned" {
					t.Fatal("mutable registry metadata exposed")
				}
			}
		}
	}
}

// Rules: rules/compiler/compiler_pipeline.md §103(4); compiler.md §71.
func TestGeneratedHelperIdentityKeepsTargetAndOwnerDistinct(t *testing.T) {
	program := generatedProgram(t, []string{"lambdas.sec"}, false)
	index := sema.NewGeneratedIdentityIndex(program)
	var token lexer.Token
	for _, stmt := range program.Statements {
		if fn, ok := stmt.(*ast.FunctionDeclaration); ok && fn.Name.Value == "First" {
			token = fn.Body.Statements[0].(*ast.LetStatement).Value.(*ast.LambdaExpression).Token
		}
	}
	first, ok := index.Resolve("lambda", "owner", token, "plan-a")
	if !ok {
		t.Fatal("no lexical origin")
	}
	for _, different := range [][3]string{{"lambda", "owner", "plan-b"}, {"lambda", "other", "plan-a"}, {"resource", "owner", "plan-a"}} {
		fact, ok := index.Resolve(different[0], different[1], token, different[2])
		if !ok || fact.ID == first.ID {
			t.Fatal("distinct semantic inputs merged")
		}
	}
	if _, ok := index.Resolve("lambda", "", lexer.Token{}, ""); ok {
		t.Fatal("missing origin was invented")
	}
}
