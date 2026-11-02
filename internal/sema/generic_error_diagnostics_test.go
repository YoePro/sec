package sema

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// MD-120's three independent source errors retain registered mandatory IDs and
// repairs across concrete/generic channels and match/try selector forms.
// Rules: rules/declarations/generics.md — §15A; MD-120 §§4,6,10(7);
// rules/tooling/diagnostics.md — §§2(6–10),14–15.
func TestGenericErrorDiagnosticIdentities(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/generic_error_diagnostics/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	counts := map[string]int{}
	for _, e := range errors {
		counts[e.ID]++
		if e.Severity != diagnostics.SeverityError || e.Help == "" || e.Line == 0 || e.EndColumn <= e.Column {
			t.Errorf("incomplete diagnostic: %+v", e)
		}
		if strings.Contains(e.Message, "Result error type E") && !strings.Contains(e.Help, "[E: error]") {
			t.Errorf("missing constraint repair: %+v", e)
		}
		if e.ID == diagnostics.ErrorVariantRequiresConcreteChannel && strings.Contains(e.Message, "; E has") && (!strings.Contains(e.Help, "Err(cause)") || strings.Contains(e.Help, "qualify")) {
			t.Errorf("invented E variants: %+v", e)
		}
	}
	if len(errors) != 13 || counts[diagnostics.InvalidResultErrorChannel] != 2 || counts[diagnostics.GenericInterfaceArgumentCount] != 2 || counts[diagnostics.ErrorVariantRequiresConcreteChannel] != 9 {
		t.Fatalf("diagnostics: %+v", errors)
	}
	data, err = os.ReadFile("../../testdata/sema/generic_error_diagnostics/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(data)); len(errors) != 0 {
		t.Fatal(errors)
	}
}
