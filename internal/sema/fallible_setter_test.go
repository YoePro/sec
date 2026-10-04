package sema

import (
	"os"
	"strings"
	"testing"
)

// A try set declares its error contract on the setter line, normal completion
// and bare return are success, return Err(error) is failure, and an interface
// requirement declares the same contract.
//
// Rules:
//   - rules/errors/errorhandling.md — §24 "Fallible property setters", §24.1 "Interfaces"
//   - rules/errors/errorhandling.md — §23 "Fallible assignment"
func TestFallibleSetterContractAccepted(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/fallible_setter_contract_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(input))
	if len(errors) != 0 {
		t.Fatalf("errors = %+v, want none", errors)
	}
	property, ok := lookupProperty(analyzer.types["Car"], "Speed")
	if !ok || property.Error == nil || property.Error.Name != "SpeedError" {
		t.Fatalf("Car.Speed contract = %+v", property)
	}
	if contract := analyzer.interfaceSetterErrors["Tunable.Speed"]; contract == nil || contract.Name != "SpeedError" {
		t.Fatalf("Tunable.Speed contract = %+v", contract)
	}
}

// Missing and non-error setter contracts, implementations whose contract does
// not satisfy the interface, return Ok(), and returned values are rejected.
//
// Rules:
//   - rules/errors/errorhandling.md — §24 "Fallible property setters", §24.1 "Interfaces"
func TestFallibleSetterContractDiagnostics(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/fallible_setter_contract_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(input))
	wants := []struct {
		line    int
		message string
	}{
		{31, "fallible setter Legacy must declare its error type after the value parameter: try set value ErrorType { ... }"},
		{46, "setter error type int is not an error type"},
		{24, "type Car property Speed setter error OtherError does not satisfy SpeedError required by interface Tunable"},
		{39, "return Ok() is invalid in a try set body"},
		{41, "a try set body returns no value"},
	}
	if len(errors) != len(wants) {
		t.Fatalf("errors = %+v, want %d", errors, len(wants))
	}
	for i, want := range wants {
		if errors[i].Line != want.line || !strings.Contains(errors[i].Message, want.message) {
			t.Fatalf("error %d = %+v, want %q at line %d", i, errors[i], want.message, want.line)
		}
	}
}
