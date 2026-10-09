package stringcontract_test

import (
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/sema/stringcontract"
)

// Rules: rules/types/contracts.md — String and collection contracts (revision 2.1).
func TestRuntimePlanFirstViolation(t *testing.T) {
	requirements := []stringcontract.Requirement{{Kind: "minLen", Bound: "1", DeclarationIndex: 2}, {Kind: "maxLen", Bound: "2", DeclarationIndex: 3}, {Kind: "maxByteLen", Bound: "4", DeclarationIndex: 4}}
	plan, err := stringcontract.New(requirements)
	if err != nil {
		t.Fatal(err)
	}
	requirements[0].Bound = "100"
	for _, test := range []struct {
		text  string
		kind  string
		index uint64
	}{
		{"éΩ", "", 0}, {"😀", "", 0}, {"a\x00", "", 0}, {"é", "", 0},
		{"", "minLen", 2}, {"abc", "maxLen", 3}, {"😀é", "maxByteLen", 4},
	} {
		violation, valid := plan.Validate(test.text)
		if valid != (test.kind == "") || violation.Kind != test.kind || violation.DeclarationIndex != test.index {
			t.Fatal(test, violation, valid)
		}
	}
}

// Rules: rules/types/contracts.md — Composition and string length units.
func TestRuneByteConsistency(t *testing.T) {
	for _, test := range []struct {
		rules []stringcontract.Requirement
		valid bool
	}{
		{[]stringcontract.Requirement{{Kind: "exactLen", Bound: "2"}, {Kind: "exactByteLen", Bound: "8"}}, true},
		{[]stringcontract.Requirement{{Kind: "exactLen", Bound: "2"}, {Kind: "maxByteLen", Bound: "1"}}, false},
		{[]stringcontract.Requirement{{Kind: "exactLen", Bound: "1"}, {Kind: "minByteLen", Bound: "5"}}, false},
		{[]stringcontract.Requirement{{Kind: "notEmpty"}, {Kind: "exactByteLen", Bound: "0"}}, false},
		{[]stringcontract.Requirement{{Kind: "exactLen", Bound: "0"}, {Kind: "exactByteLen", Bound: "0"}}, true},
		{[]stringcontract.Requirement{{Kind: "minLen", Bound: "18446744073709551616"}, {Kind: "maxByteLen", Bound: "18446744073709551615"}}, false},
	} {
		plan, err := stringcontract.New(test.rules)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Satisfiable() != test.valid {
			t.Fatal(test)
		}
	}
}

// Rules: rules/types/contracts.md — Applicability, Explicit defaults, Diagnostics.
func TestStringUnitsFrontend(t *testing.T) {
	for _, name := range []string{"units_valid", "units_invalid"} {
		data, err := os.ReadFile("../../../testdata/sema/string_contracts/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		result := parser.New(lexer.NewWithFile(string(data), name+".sec")).Parse()
		if result.HasErrors {
			t.Fatal(result.Diagnostics)
		}
		errors := sema.NewAnalyzer().Analyze(result.Program)
		if name == "units_valid" && len(errors) != 0 {
			t.Fatal(errors)
		}
		if name == "units_invalid" {
			if len(errors) != 8 {
				t.Fatalf("got %d: %v", len(errors), errors)
			}
			ids := map[string]int{}
			for _, e := range errors {
				ids[e.ID]++
			}
			if ids["S1065"] != 3 || ids["S1059"] != 2 || ids["S1064"] != 1 || ids["S1066"] != 1 || ids["S1028"] != 1 {
				t.Fatal(ids, errors)
			}
		}
	}
}
