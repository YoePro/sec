package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestAnalyseAllocationContexts checks shared context rendering, transitive site
// ownership and unavailable profiles without changing report classification.
// Rules: rules/memory/allocation.md — §§5,6,17,29(1),(3);
// rules/mlir/packages/sec-mlir-dialect_package19.md — §§52–54,72–76.
func TestAnalyseAllocationContexts(t *testing.T) {
	file := "../../testdata/sema/allocation_context_facts_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"hosted", "embedded-arena", "noalloc", "freestanding"} {
		t.Run(profile, func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(data), file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "baremetal", TargetArch: "cortex-m3", Profile: profile, PointerWidthBits: 32})
			a.Analyze(program)
			report := buildAnalyseReport(a, map[string]bool{file: true}, hostCompilerTarget(), "", nil)
			var output bytes.Buffer
			report.write(&output)
			for _, fact := range a.AllocationContextFacts() {
				want := sema.AllocationContextDescription(fact) + " at " + analysePosition(fact.Source)
				if !strings.Contains(output.String(), want) {
					t.Fatal(want, output.String())
				}
			}
			backing := "backing allocation context: unavailable; profile " + profile + "; origin Unknown"
			if profile == "hosted" || profile == "embedded-arena" {
				backing = "backing allocation context: available; profile " + profile + "; origin Arena"
			}
			for _, want := range []string{"implicit allocation context:", "profile " + profile, "explicit Arena allocation domain:", "(receiver moved)", "created Arena domain:", backing, "allocation context/domain unresolved"} {
				if !strings.Contains(output.String(), want) {
					t.Fatal(want, output.String())
				}
			}
			if report.counts[analyseClassError] != 0 {
				t.Fatal("presentation introduced diagnostics", report.counts)
			}
		})
	}
}
