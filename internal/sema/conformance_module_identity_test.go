package sema

import (
	"os"
	"testing"
)

// Conformance consumers receive the defining module as canonical identity,
// independently of source locations, and cannot mutate the retained snapshot.
// Rules: rules/declarations/interfaces.md — §§4–6,12;
// rules/compiler/semantic_ir.md — §109(1–4).
func TestConformanceFactsRetainModuleIdentity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/conformance_module_identity.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	facts := a.InterfaceConformances()
	if len(facts) != 1 || facts[0].TypeName != "Value" || facts[0].TypeModule != "conformance_identity" {
		t.Fatalf("module identity=%v", facts)
	}
	facts[0].TypeModule = "foreign"
	if a.InterfaceConformances()[0].TypeModule != "conformance_identity" {
		t.Fatal("module identity snapshot leaked")
	}
}
