package sema

import (
	"os"
	"reflect"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// TestStringEnumInitializersUseCanonicalCTE checks composed decoded strings,
// immutable bindings, aliases, defaults and generic enum template constants.
// Rules: rules/declarations/enums.md — §§5,7,8;
// rules/compiler/compile_time_evaluation.md — SemanticCompileTimeRequiredContext.
func TestStringEnumInitializersUseCanonicalCTE(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/string_enum_cte/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	typ := a.Types()["Composed"]
	classes := EnumValueClasses(typ)
	if len(classes) != 3 || !reflect.DeepEqual(classes[0].Members, []string{"Primary", "Alias"}) || *classes[0].StringValue != "Alpha Beta" || *classes[1].StringValue != "" || *classes[2].StringValue != "å🙂" || typ.EnumDefault != "Empty" {
		t.Fatalf("wrong folded enum facts: %+v, default %s", classes, typ.EnumDefault)
	}
	generic := a.Types()["Phantom"]
	if value := generic.EnumConsts["Value"].StringValue; value == nil || *value != "Alpha Beta" {
		t.Fatalf("generic enum lost CTE: %+v", generic)
	}
}

// TestStringEnumCTERejections keeps unavailable CTE execution distinct from
// nonconstant/wrong-kind values and retains canonical alias coverage rejection.
// Rules: rules/declarations/enums.md — §§5,7; compile_time_evaluation.md — §8(5).
func TestStringEnumCTERejections(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/string_enum_cte/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	for _, want := range []string{"requires an explicit initializer", "iota is not available", "compile-time string constant", "requires compile-time execution", "unreachable", "not a declared value"} {
		found := false
		for _, e := range errors {
			found = found || strings.Contains(e.Message, want)
		}
		if !found {
			t.Errorf("missing %q: %v", want, errors)
		}
	}
	found := false
	for _, e := range errors {
		if e.ID == diagnostics.SemanticCompileTimeExecutionUnavailable {
			found = true
		}
	}
	if !found {
		t.Fatalf("unavailable execution lost S1101 identity: %v", errors)
	}
}
