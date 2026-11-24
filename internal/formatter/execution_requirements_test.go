package formatter

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestExecutionRequirementFormatting checks MD-138 T03/T08–T10/T40 source
// preservation, parser-owned attachment and stable list layout.
// Rules: rules/tooling/formatter.md §§11,16(13a),25,29.
func TestExecutionRequirementFormatting(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/execution_requirements/clauses.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{2, 4, 6} {
		options := Options{IndentationWidth: width}
		got := Format(Source{Text: string(data)}, options).Text
		if !reflect.DeepEqual(executionRequirementShape(t, string(data)), executionRequirementShape(t, got)) {
			t.Fatalf("attachment or explicit spelling changed:\n%s", got)
		}
		for _, want := range []string{
			"fn() fn() void require(task)", "fn() (fn() void) require(task)",
			"fn() (fn() void require(task)) require(thread)",
			"cancellable, // contextual name", "task, // trailing comma",
			"fn Create() (fn() void) require(task, thread)",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("missing %q:\n%s", want, got)
			}
		}
		if again := Format(Source{Text: got}, options).Text; again != got {
			t.Fatalf("not idempotent:\n%s\n%s", got, again)
		}
	}
}

// executionRequirementShape compares clauses at their actual callable owner,
// including returned-type grouping, without source positions or list trivia.
// Rules: rules/tooling/formatter.md §16(13a); grammar.md §101A.
func executionRequirementShape(t *testing.T, source string) []string {
	t.Helper()
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parse: %v\n%s", p.Errors(), source)
	}
	result := []string{}
	ast.Visit(program, func(node any) bool {
		var clause *ast.ExecutionRequirementClause
		owner := ""
		switch n := node.(type) {
		case *ast.FunctionDeclaration:
			owner = "declaration:" + n.Name.Value
			clause = n.ExecutionRequirements
		case *ast.LambdaExpression:
			owner = "lambda"
			clause = n.ExecutionRequirements
		case *ast.TypeReference:
			owner = fmt.Sprintf("type:%s:groups=%d", n.Name, len(n.Grouping))
			clause = n.ExecutionRequirements
		default:
			return true
		}
		entries := []string{}
		if clause != nil {
			for _, entry := range clause.Entries {
				entries = append(entries, entry.Lexeme)
			}
			owner += " explicit"
		}
		result = append(result, owner+":"+strings.Join(entries, ","))
		return true
	})
	return result
}

// Lists follow the same width/trailing-comma policy as callable parameters;
// explicit implied entries and their order survive canonicalization.
// Rules: rules/tooling/formatter.md §§11,16(13a),29.
func TestExecutionRequirementListLayout(t *testing.T) {
	for _, tc := range []struct {
		file  string
		width uint64
		want  string
	}{
		{"spacing.sec", 120, "fn F() void require(thread, task, cancellable)"},
		{"multiline.sec", 120, "require(\n    task,\n    thread,\n)"},
		{"width.sec", 32, "require(\n    task,\n    thread,\n    cancellable,\n)"},
		{"comments.sec", 120, "cancellable, // explicit stronger contract"},
		{"nested_width.sec", 40, "require(\n"},
	} {
		data, err := os.ReadFile("../../testdata/formatter/execution_requirements/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		options := Options{LineWidth: tc.width}
		got := Format(Source{Text: source}, options).Text
		if !strings.Contains(got, tc.want) {
			t.Fatalf("missing %q:\n%s", tc.want, got)
		}
		if !reflect.DeepEqual(executionRequirementShape(t, source), executionRequirementShape(t, got)) {
			t.Fatalf("shape changed:\n%s", got)
		}
		if Format(Source{Text: got}, options).Text != got {
			t.Fatalf("not idempotent:\n%s", got)
		}
	}
}

// Recovered clauses remain errors after formatting; invalid entries and their
// attachment are never repaired into a different contract.
// Rules: rules/tooling/formatter.md §§16(13a),25,29.
func TestMalformedExecutionRequirementFormatting(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/execution_requirements/clauses_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range strings.Split(string(data), "// CASE ")[1:] {
		name, source, _ := strings.Cut(part, "\n")
		t.Run(name, func(t *testing.T) {
			got := Format(Source{Text: source}, Options{}).Text
			firstLine, _, _ := strings.Cut(source, "\n")
			firstLine, _, _ = strings.Cut(firstLine, "{")
			firstLine = strings.TrimSpace(firstLine)
			if !strings.Contains(got, firstLine) {
				t.Fatalf("recovery region changed:\n%s", got)
			}
			p := parser.New(lexer.New(got))
			p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Fatalf("invalid clause repaired:\n%s", got)
			}
			if Format(Source{Text: got}, Options{}).Text != got {
				t.Fatal("recovery is not idempotent")
			}
		})
	}
}
