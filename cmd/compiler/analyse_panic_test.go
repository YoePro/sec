package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestAnalysePanicCauses renders complete compiler causes rather than merely
// the first introducing function, retaining reason sets and static messages.
// Rules: rules/errors/panic.md — §§25(3),28(6)–(7);
// rules/compiler/compiler_analysis.md — §61(2).
func TestAnalysePanicCauses(t *testing.T) {
	const file = "../../testdata/sema/panic_causes_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	g := a.CallGraph()
	names := map[sema.CallableID]string{}
	for _, node := range g.Nodes() {
		names[node.ID] = node.Name
	}
	var report analyseReport
	report.addEffects(g, g.Nodes(), names)
	text := strings.Join(report.sections[0].lines, "\n")
	for _, want := range []string{"Root: panic cause may-panic-assertion in Check at " + file, "Root: panic cause may-panic-arithmetic in Divide at " + file, "Root: panic cause may-panic-explicit in Stop at " + file, "AssertionFailed", "DivisionByZero, ArithmeticOverflow", `message "value must hold"`, `message "stop here"`, "via Root at " + file, "via Middle at " + file} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
}
