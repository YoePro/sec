package sema

import (
	"os"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestCoveredSwitchItemsKeepCanonicalProofDiagnostics checks both mandatory
// identities, including comma alternatives and the exact earlier source item.
// Rules: rules/control-flow/flowcontrol_switch.md §§20-22,34;
// rules/tooling/diagnostics.md §21(4)-(6).
func TestCoveredSwitchItemsKeepCanonicalProofDiagnostics(t *testing.T) {
	file := "../../testdata/sema/switch_covered_items_invalid.sec"
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	a.SetDiagnosticRules(diagnostics.RulePolicy{"control-flow.unreachable-statement": "off", "switch.duplicate-constant-case": "off"})
	errors := a.Analyze(program)
	if len(errors) != 10 {
		t.Fatalf("want 10 canonical coverage errors, got %+v", errors)
	}
	counts := map[string]int{}
	for _, err := range errors {
		counts[err.ID]++
		if err.Help == "" || err.PreviousLine == 0 || err.PreviousColumn == 0 || err.PreviousFile != file || err.RelatedLabel == "" || err.EndColumn <= err.Column {
			t.Errorf("missing proof provenance: %+v", err)
		}
	}
	if counts[diagnostics.UnreachableStatement] != 6 || counts[diagnostics.DuplicateSwitchCase] != 4 {
		t.Fatalf("unexpected identities: %v", counts)
	}
}

// TestPartialSwitchRelationsRemainReachable prevents overlap from being used
// as full-coverage proof, and keeps dynamic bounds conservative.
// Rules: rules/control-flow/flowcontrol_switch.md §§22-23.
func TestPartialSwitchRelationsRemainReachable(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/switch_partial_relations_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(source)); len(errors) != 0 {
		t.Fatal(errors)
	}
}
