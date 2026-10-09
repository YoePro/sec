package membership_test

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// analyzeFixture exercises the canonical parser and completed Sema on source
// fixtures, without inventing membership facts in test metadata.
// Rules: rules/types/contracts.md — Ordered membership and Diagnostics.
func analyzeFixture(t *testing.T, name string) (*sema.Analyzer, *ast.Program, []sema.Error) {
	t.Helper()
	data, err := os.ReadFile("../../../testdata/sema/membership/" + name + ".sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), name+".sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	return a, program, a.Analyze(program)
}

// TestNominalMembershipSupportsUnionValues covers semantic equality, immutable
// CTE dependencies, named inheritance, explicit defaults and first-member order.
// Rules: rules/types/contracts.md — Ordered membership, Composition;
// rules/declarations/unions.md — §14 Equality; types/default_values.md — Named types.
func TestNominalMembershipSupportsUnionValues(t *testing.T) {
	a, _, errors := analyzeFixture(t, "valid")
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, name := range []string{"Allowed", "Alias", "Explicit", "FromBindings"} {
		typ := a.Types()[name]
		value := sema.DefaultValueOf(typ)
		if value.Value.Nominal.Owner != "State" || value.Value.Nominal.Member != "Ready" || !sema.IsDefaultable(typ) {
			t.Fatalf("%s: %+v", name, value)
		}
		if display, _, ok := sema.DefaultValueDisplay(typ); !ok || display != name+".Ready" {
			t.Fatal(name, display, ok)
		}
	}
	for _, name := range []string{"GenericAllowed", "GenericAlias"} {
		value := sema.DefaultValueOf(a.Types()[name]).Value
		if value.Nominal.Owner != "Choice[int]" || value.Nominal.Member != "Yes" {
			t.Fatal(name, value)
		}
		if display, _, ok := sema.DefaultValueDisplay(a.Types()[name]); !ok || display != name+".Yes" {
			t.Fatal(name, display, ok)
		}
	}
}

// TestNominalMembershipRejectsInvalidValues checks source-linked duplicates,
// distinct nominal identities, unknown members, inherited conjunction, static
// invalid values and the separate deferred-executor outcome for ordinary calls.
// Rules: rules/types/contracts.md — Ordered membership and Diagnostics;
// rules/declarations/enums.md — §7 Value aliases; unions.md — §14 Equality.
func TestNominalMembershipRejectsInvalidValues(t *testing.T) {
	_, _, errors := analyzeFixture(t, "rejections_invalid")
	if len(errors) == 0 {
		t.Fatal("invalid fixture passed")
	}
	byID := map[string]int{}
	for _, err := range errors {
		byID[err.ID]++
		if err.ID == diagnostics.DuplicateContractMembershipValue && err.PreviousLine == 0 {
			t.Fatal("duplicate lost first-member source", err)
		}
		if strings.Contains(err.Message, "membership value State.Missing") && err.ID == diagnostics.SemanticCompileTimeExecutionUnavailable {
			t.Fatal("unknown variant classified as getter execution", err)
		}
	}
	for _, id := range []string{diagnostics.DuplicateContractMembershipValue, diagnostics.IncompatibleMembershipValue, diagnostics.InvalidContractArgument, diagnostics.EmptyContractMembership, diagnostics.DefaultViolatesContract, diagnostics.InvalidMembershipValue, diagnostics.ValueViolatesContract, diagnostics.InapplicableContract, diagnostics.SemanticCompileTimeExecutionUnavailable} {
		if byID[id] == 0 {
			t.Errorf("missing %s: %+v", id, errors)
		}
	}
	if byID[diagnostics.DuplicateContractMembershipValue] != 3 {
		t.Fatal("enum aliases or repeated union variant not treated as semantic duplicates", errors)
	}
}
