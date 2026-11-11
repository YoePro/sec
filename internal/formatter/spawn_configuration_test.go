package formatter

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestFormatNamedThreadConfiguration preserves the separate configuration and
// child callable identities, including the ordinary contiguous <- marker.
// Rules: rules/concurrency/threads.md — §10(3); spawn.md — §4(1),(10);
// rules/tooling/formatter.md — §§16,29.
func TestFormatNamedThreadConfiguration(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_named_config_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	project := func(source string) []string {
		p := parser.New(lexer.New(source))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		var result []string
		ast.Visit(program, func(node any) bool {
			if spawn, ok := node.(*ast.SpawnExpression); ok {
				result = append(result, spawn.String())
			}
			return true
		})
		return result
	}
	before := project(source)
	got := Format(Source{Text: source}, Options{})
	if got.Malformed || !strings.Contains(got.Text, "spawn thread <-config Work()") || !strings.Contains(got.Text, "spawn thread <-holder.Config Work()") {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(before, project(got.Text)) {
		t.Fatal("spawn ownership changed", got.Text)
	}
	if again := Format(Source{Text: got.Text}, Options{}); again.Malformed || again.Text != got.Text {
		t.Fatal(again)
	}
}
