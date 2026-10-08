package sema

import (
	"os"
	"testing"

	"sec/internal/diagnostics"
)

// A regex contract is checked for applicability and a compile-time string
// pattern, then rejected with S1058 because the regular-expression syntax and
// engine are not yet normatively fixed. The contract is never dropped
// silently.
//
// Rules:
//   - rules/types/contracts.md — "Applicability"
//   - rules/types/contracts.md — "String and collection contracts"
//   - missing-decisions.yaml — MD-010
func TestRegexContractFixtureReportsFocusedDiagnostics(t *testing.T) {
	source, err := os.ReadFile("../../testdata/contracts/regex_contract_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	assertSemaErrors(t, errors, []string{
		"regex contract on Email cannot be validated: the Sec regular-expression syntax and engine are not yet defined at 6:19, type declaration at 6:6",
		"regex contract does not apply to int at 9:16, type declaration at 9:6",
		"regex contract pattern must be a compile-time string at 12:24, contract declaration at 12:18",
	})
	if errors[0].ID != diagnostics.RegexContractUnavailable || errors[0].Help == "" {
		t.Fatalf("regex diagnostic = %+v, want S1058 with help", errors[0])
	}
}

// The retained semantic contract carries the escape-decoded pattern so later
// validation can consume it once the regex syntax is decided.
//
// Rule: rules/types/contracts.md — "String and collection contracts".
func TestRegexContractRetainsDecodedPattern(t *testing.T) {
	analyzer, _ := analyzeSourceWithAnalyzerRaw(t, "module main\ntype Digits string regex \"\\\\d+\\n\"\nfn main() void {}\n")
	typ, ok := analyzer.types["Digits"]
	if !ok {
		t.Fatal("Digits type was not declared")
	}
	for _, contract := range typ.Contracts {
		if regex, ok := contract.(RegexContract); ok {
			if regex.Pattern != "\\d+\n" {
				t.Fatalf("pattern = %q, want %q", regex.Pattern, "\\d+\n")
			}
			return
		}
	}
	t.Fatalf("contracts = %#v, want RegexContract", typ.Contracts)
}
