package formatter

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// While formatting retains logical and try condition structure, nested targets
// and try handlers; supported indentation widths reach the same stable layout.
// Rules: rules/control-flow/flowcontrol_while.md — §§2,5,7,15;
// rules/tooling/formatter.md — §§6,15,25,29.
func TestWhileConditionFormatting(t *testing.T) {
	source, err := os.ReadFile("../../testdata/control_flow/while_conditions/effects.sec")
	if err != nil {
		t.Fatal(err)
	}
	parseConditions := func(text string) []string {
		t.Helper()
		p := parser.New(lexer.New(text))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		var conditions []string
		ast.Visit(program, func(node any) bool {
			if loop, ok := node.(*ast.WhileStatement); ok {
				conditions = append(conditions, loop.Condition.String())
			}
			return true
		})
		return conditions
	}
	want := strings.Join(parseConditions(string(source)), "\n")
	messy := strings.ReplaceAll(string(source), "    ", " ")
	for _, width := range []int{2, 4, 6} {
		options := Options{IndentationWidth: width}
		output := Format(Source{Text: messy}, options).Text
		if got := strings.Join(parseConditions(output), "\n"); got != want {
			t.Fatalf("changed conditions:\n%s\nwant:\n%s", got, want)
		}
		if len(parseConditions(output)) != 7 {
			t.Fatal("lost a loop")
		}
		indent := strings.Repeat(" ", width)
		if !strings.Contains(output, "\n"+indent+"while Probe(3) && Tail(true) {\n"+indent+indent+"Bodies += 1") {
			t.Fatalf("noncanonical while layout:\n%s", output)
		}
		if twice := Format(Source{Text: output}, options).Text; twice != output {
			t.Fatal("while formatting is not idempotent")
		}
	}
}
