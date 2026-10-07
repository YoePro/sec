package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestArenaInvalidationDiagnostics checks real operation-point failures with
// separate IDs, method spans, dependency locations, help and Invalid proofs.
// Rules: rules/memory/arena.md — §§36, 44, 119–120;
// rules/tooling/diagnostics.md — §§4, 5, 9, 28.
func TestArenaInvalidationDiagnostics(t *testing.T) {
	path := "../../testdata/sema/arena_invalidation_diagnostics_invalid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	analyze := func() []Error {
		p := parser.New(lexer.NewWithFile(string(source), path))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		analyzer := NewAnalyzer()
		errs := analyzer.Analyze(program)
		graph := analyzer.CallGraph()
		for _, node := range graph.Nodes() {
			for _, effect := range graph.ArenaSummary(node.ID).DirectEffects {
				if effect.Kind == ArenaEffectReset || effect.Kind == ArenaEffectRelease {
					t.Fatal("rejected operation published a successful effect", effect)
				}
			}
		}
		return errs
	}
	errs := analyze()
	if len(errs) != 6 {
		t.Fatalf("expected one failure per operation: %v", errs)
	}
	for index, id := range []string{diagnostics.ArenaResetLiveDependency, diagnostics.ArenaReleaseLiveDependency, diagnostics.ArenaResetLiveDependency, diagnostics.ArenaReleaseLiveDependency, diagnostics.ArenaResetLiveDependency, diagnostics.ArenaResetLiveDependency} {
		e := errs[index]
		if e.ID != id || e.Severity != diagnostics.SeverityError || e.ProofState != diagnostics.ProofInvalid || e.Help == "" || e.RelatedLabel != "live Arena dependency" || e.File != path || e.PreviousFile != path || e.PreviousLine == 0 {
			t.Fatal(e)
		}
		operation := "Reset"
		if id == diagnostics.ArenaReleaseLiveDependency {
			operation = "Release"
		}
		if !strings.Contains(e.Message, "Arena."+operation) || !strings.Contains(e.Help, operation) || e.EndLine != e.Line || e.EndColumn-e.Column != len(operation) {
			t.Fatal(e)
		}
		definition, ok := diagnostics.Lookup(id)
		if !ok || !definition.Mandatory || definition.DefaultSeverity != diagnostics.SeverityError {
			t.Fatal(definition)
		}
	}
	if !strings.Contains(errs[3].Message, "on moved") || !strings.Contains(errs[4].Message, "dependency first") {
		t.Fatal(errs)
	}
	for i := 0; i < 16; i++ {
		if got := analyze(); !reflect.DeepEqual(got, errs) {
			t.Fatal("map-order dependent diagnostics", got, errs)
		}
	}
}

// TestArenaInvalidationDiagnosticControls excludes independent domains and
// ended lexical borrows and dependency-free operations without claiming full NLL.
// Rules: rules/memory/arena.md — §§36(1–4), 44(1), 45(2).
func TestArenaInvalidationDiagnosticControls(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/arena_invalidation_diagnostics_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errs := analyzeSourceRaw(t, string(data)); len(errs) != 0 {
		t.Fatal(errs)
	}
}

// TestArenaInvalidationPreconditionDiagnostics ensures basic call validation
// cannot be misreported as a live-dependency proof violation.
// Rules: rules/memory/arena.md — §§35(1,4), 43(1–7), 119(3).
func TestArenaInvalidationPreconditionDiagnostics(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/arena_invalidation_preconditions_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errs := analyzeSourceRaw(t, string(data))
	if len(errs) != 3 {
		t.Fatal(errs)
	}
	for _, e := range errs {
		if e.ID == diagnostics.ArenaResetLiveDependency || e.ID == diagnostics.ArenaReleaseLiveDependency {
			t.Fatal("call precondition misclassified", e)
		}
	}
}
