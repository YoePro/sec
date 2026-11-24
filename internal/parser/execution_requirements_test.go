package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// MD-138 attachment is syntactic and right-recursive, preserving explicit
// entries, delimiters, grouping and contextual cancellable identifiers.
// Rules: rules/foundations/grammar.md — §§27(6),32(5),73(3),101A.
func TestExecutionRequirementSyntax(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/execution_requirements.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.NewWithFile(string(data), "execution_requirements.sec"))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	types := map[string]*ast.TypeReference{}
	clauses := 0
	ast.Visit(program, func(node any) bool {
		if declaration, ok := node.(*ast.TypeDeclStatement); ok {
			types[declaration.Name.Value] = declaration.BaseType
		}
		if clause, ok := node.(*ast.ExecutionRequirementClause); ok {
			clauses++
			if clause.Invalid || clause.Token.Lexeme != "require" || clause.Open.Lexeme != "(" || clause.Close.Lexeme != ")" || len(clause.Entries) == 0 || clause.Token.File != "execution_requirements.sec" {
				t.Fatalf("clause %+v", clause)
			}
		}
		return true
	})
	if clauses != 9 {
		t.Fatalf("%d clauses", clauses)
	}
	if types["Inner"].ExecutionRequirements != nil || types["Inner"].FunctionReturnType.ExecutionRequirements == nil {
		t.Fatal("inner attachment lost")
	}
	if types["Outer"].ExecutionRequirements == nil || types["Outer"].FunctionReturnType.ExecutionRequirements != nil || len(types["Outer"].FunctionReturnType.Grouping) != 1 {
		t.Fatal("outer attachment/grouping lost")
	}
	if types["Both"].ExecutionRequirements.Entries[0].Lexeme != "thread" || types["Both"].FunctionReturnType.ExecutionRequirements.Entries[0].Lexeme != "task" {
		t.Fatal("independent nested requirements lost")
	}
	entries := types["Predicate"].ExecutionRequirements
	if len(entries.Commas) != 2 || entries.Entries[0].Lexeme != "cancellable" || entries.Entries[1].Lexeme != "task" {
		t.Fatal("written entry order/trailing comma lost")
	}
}

// Malformed clauses never consume the next callable or turn duplicate suffixes
// into contracts of an enclosing callable. Each case retains an After sibling.
// Rules: rules/foundations/grammar.md — §101A(3)–(5);
// rules/compiler/parser_recovery.md — "Delimiter recovery".
func TestExecutionRequirementRecovery(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/execution_requirements_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range strings.Split(string(data), "// CASE ")[1:] {
		name, source, _ := strings.Cut(part, "\n")
		t.Run(name, func(t *testing.T) {
			p := New(lexer.New(source))
			program := p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Fatal("accepted invalid clause")
			}
			after := false
			for _, statement := range program.Statements {
				if fn, ok := statement.(*ast.FunctionDeclaration); ok && fn.Name.Value == "After" && fn.Body != nil {
					after = true
				}
			}
			if !after {
				t.Fatalf("lost following callable: %v", p.Errors())
			}
		})
	}
}
