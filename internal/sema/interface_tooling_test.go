package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestInterfaceConformanceFacts checks canonical generic/inherited/Iterator
// validation and structured failures without deriving identities from wording.
// Rules: rules/declarations/interfaces.md — §§4–6,9,12;
// rules/compiler/semantic_ir.md — §109(1–4).
func TestInterfaceConformanceFacts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/interface_model_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "interfaces.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	errors := a.Analyze(result.Program)
	if len(errors) == 0 {
		t.Fatal("invalid fixture accepted")
	}
	facts := a.InterfaceConformances()
	if len(facts) != 8 {
		t.Fatalf("facts: %+v", facts)
	}
	want := map[string]diagnostics.ProofState{"Good": diagnostics.ProofValid, "Bad": diagnostics.ProofInvalid, "IntSink": diagnostics.ProofValid, "WrongSink": diagnostics.ProofInvalid, "Inherited": diagnostics.ProofValid, "Unknown": diagnostics.ProofUnproven, "Counter": diagnostics.ProofValid, "BrokenIterator": diagnostics.ProofInvalid}
	for _, fact := range facts {
		if fact.State != want[fact.TypeName] {
			t.Fatalf("state mismatch: %+v", fact)
		}
		if fact.TypeDeclaration.Line == 0 || fact.Clause.Line == 0 {
			t.Fatalf("missing provenance: %+v", fact)
		}
		for _, failure := range fact.Failures {
			if failure.Member == "" {
				t.Fatal("missing member identity")
			}
			found := false
			for _, diagnostic := range errors {
				found = found || reflect.DeepEqual(failure.Diagnostic, diagnostic)
			}
			if !found {
				t.Fatalf("fact does not preserve owning diagnostic: %+v", failure)
			}
		}
		if fact.TypeName == "WrongSink" && fact.Interface != "Sink[string]" {
			t.Fatalf("missing substitution: %+v", fact)
		}
	}
	for i := 0; i < 20; i++ {
		if !reflect.DeepEqual(facts, a.InterfaceConformances()) {
			t.Fatal("unstable publication")
		}
	}
	for index := range facts {
		if len(facts[index].Failures) > 0 {
			facts[index].Failures[0].Member = "changed"
			facts[index].Failures[0].Diagnostic.Message = "changed"
		}
	}
	if reflect.DeepEqual(facts, a.InterfaceConformances()) {
		t.Fatal("published facts alias analyzer")
	}
	empty := parser.New(lexer.NewWithFile("module main\nfn main() void {}", "empty.sec")).ParseProgram()
	a.Analyze(empty)
	if len(a.InterfaceConformances()) != 0 || len((*Analyzer)(nil).InterfaceConformances()) != 0 {
		t.Fatal("stale conformance facts")
	}
}
