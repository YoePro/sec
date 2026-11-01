package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// Both direct method and iterator dispatch require the receiver contract's
// authority; valid exclusive views remain accepted. No capability is upgraded.
// Rules: rules/declarations/interfaces.md — §§3.2,6,7.1,12.
func TestInterfaceReceiverDiagnosticIdentities(t *testing.T) {
	for _, name := range []string{"receiver_invalid", "valid"} {
		data, err := os.ReadFile("../../testdata/sema/interface_diagnostics/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		errors := analyzeSourceRaw(t, string(data))
		if name == "valid" {
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			continue
		}
		if len(errors) != 2 {
			t.Fatalf("errors: %+v", errors)
		}
		for _, e := range errors {
			if e.ID != diagnostics.InterfaceReceiverCapability || e.Severity != diagnostics.SeverityError || e.Help == "" || e.Line == 0 || e.EndColumn <= e.Column {
				t.Fatalf("diagnostic: %+v", e)
			}
			if strings.Contains(e.Message, "Increment") && e.PreviousLine != 3 {
				t.Fatalf("missing receiver declaration: %+v", e)
			}
		}
	}
}

// The Sema guard also has an ID when a recovered/programmatically constructed
// extension AST bypasses the parser rejection. Rules: rules/declarations/interfaces.md §§4,12.
func TestInterfacePlacementSemaGuard(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/interface_diagnostics/extends_placement_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	for _, statement := range program.Statements {
		if impl, ok := statement.(*ast.ImplStatement); ok {
			impl.ConformanceRecovery = nil
		}
	}
	a := NewAnalyzer()
	errors := a.Analyze(program)
	found := false
	for _, e := range errors {
		if e.ID == diagnostics.InterfaceConformancePlacement && e.Help != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing Sema placement guard: %+v", errors)
	}
}
