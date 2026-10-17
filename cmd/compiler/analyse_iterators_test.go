package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestAnalyseIteratorResolutions checks canonical descriptions, stable report
// order and filtering to user-selected sources without backend prerequisites.
// Rules: rules/compiler/semantic_ir.md — §109(1–4),§115(2);
// rules/compiler/compiler_analysis.md — §18(6),§61(2).
func TestAnalyseIteratorResolutions(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/iterator_resolution.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "iterators.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errors := a.Analyze(result.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	facts := a.IteratorResolutions()
	if len(facts) != 3 {
		t.Fatal(facts)
	}
	report := buildAnalyseReport(a, map[string]bool{"iterators.sec": true}, CompilerTarget{}, "", nil)
	var output bytes.Buffer
	report.write(&output)
	last := -1
	for _, fact := range facts {
		index := strings.Index(output.String(), fact.Display())
		if index <= last {
			t.Fatalf("missing or unordered resolution: %s\n%s", fact.Display(), output.String())
		}
		last = index
	}
	for i := 0; i < 10; i++ {
		var repeated bytes.Buffer
		buildAnalyseReport(a, map[string]bool{"iterators.sec": true}, CompilerTarget{}, "", nil).write(&repeated)
		if output.String() != repeated.String() {
			t.Fatal("nondeterministic report")
		}
	}
	var filtered bytes.Buffer
	buildAnalyseReport(a, map[string]bool{"other.sec": true}, CompilerTarget{}, "", nil).write(&filtered)
	if strings.Contains(filtered.String(), "Next target:") {
		t.Fatal("unselected source exposed")
	}
}

// TestAnalyseIteratorCLI checks publication through the actual analysis command
// with source filtering despite automatically loaded core/stdlib loops.
// Rules: rules/compiler/semantic_ir.md — §109(1–4),§115(2).
func TestAnalyseIteratorCLI(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestAnalyseCLIProcess$", "--", "analyse", "--verbose", "../../testdata/tooling/iterator_resolution.sec")
	cmd.Env = append(os.Environ(), "SEC_ANALYSE_TEST_PROCESS=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("analyse failed: %v\n%s", err, output)
	}
	for _, want := range []string{"iterators:", "Iterator[int]: source Counter; yields int; Next target: Counter.Next() Option[int]", "Iterator[string]: source Words; yields string; Next target: Words.Next() Option[string]", "resolved per specialization (generic constraint)"} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("missing %q\n%s", want, output)
		}
	}
	if strings.Contains(string(output), "StringSplitIterator.Next") {
		t.Fatal("unselected core loop exposed")
	}
}
