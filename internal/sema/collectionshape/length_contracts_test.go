package collectionshape_test

import (
	"math/big"
	"strings"
	"testing"

	"sec/internal/sema"
)

// rules/types/contracts.md — "String and collection contracts".
func TestLengthContractsAreRepresentedOnStringsAndCollections(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, fixture(t, "length_contracts_1.sec"))
	assertSemaErrors(t, errors, nil)

	for name, want := range map[string][]string{
		"Label":     {"minLen=2", "maxLen=12"},
		"Code":      {"exactLen=4"},
		"Values":    {"minLen=1", "maxLen=8"},
		"Four":      {"exactLen=4"},
		"Items":     {"minLen=1"},
		"MoreItems": {"minLen=1", "maxLen=8"},
	} {
		var got []string
		for _, contract := range analyzer.Types()[name].Contracts {
			if length, ok := contract.(sema.LengthContract); ok {
				got = append(got, length.Name+"="+length.Value.String())
			}
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s length contracts = %v, want %v", name, got, want)
		}
	}
}

func TestLengthContractValuesAndApplicability(t *testing.T) {
	errors := analyzeSource(t, fixture(t, "length_contracts_2.sec"))
	for _, want := range []string{
		"minLen contract does not apply to int",
		"minLen contract value must not be negative",
		"maxLen contract value must be a compile-time integer",
	} {
		if !errorsContainMessage(errors, want) {
			t.Errorf("errors %v do not contain %q", errors, want)
		}
	}
	if len(errors) != 3 {
		t.Fatalf("length contract errors = %v, want 3", errors)
	}
}

func TestLengthContractSetConsistencyIncludesInheritanceNotEmptyAndFixedArrays(t *testing.T) {
	errors := analyzeSource(t, fixture(t, "length_contracts_3.sec"))
	for _, name := range []string{"ImpossibleRange", "EmptyCode", "ExactConflict", "FixedFour"} {
		if !errorsContainMessage(errors, "length contracts cannot be satisfied together for "+name) {
			t.Errorf("errors %v do not contain inconsistency for %s", errors, name)
		}
	}
	if len(errors) != 4 {
		t.Fatalf("length consistency errors = %v, want 4", errors)
	}
}

func TestStringLengthContractsValidateExplicitDefaultsInCanonicalByteUnit(t *testing.T) {
	errors := analyzeSource(t, fixture(t, "length_contracts_4.sec"))
	for _, name := range []string{"TooShort", "TooLong", "WrongExact"} {
		if !errorsContainMessage(errors, "invalid for "+name) {
			t.Errorf("errors %v do not contain invalid default for %s", errors, name)
		}
	}
	if len(errors) != 3 {
		t.Fatalf("length default errors = %v, want 3", errors)
	}
}

func TestStringContractLiteralsAreCheckedAtInitializationBoundaries(t *testing.T) {
	errors := analyzeSource(t, fixture(t, "length_contracts_5.sec"))
	for _, want := range []string{
		`string value "hello" violates maxLen contract Short 4`,
		`string value "" violates notEmpty contract Present`,
		`string value "maybe" violates in contract Answer`,
	} {
		if !errorsContainMessage(errors, want) {
			t.Errorf("errors %v do not contain %q", errors, want)
		}
	}
	if len(errors) != 8 {
		t.Fatalf("string literal contract errors = %v, want 8", errors)
	}
}

// rules/types/contracts.md — exact collection-literal lengths are compile-time
// proofs for minLen, maxLen, exactLen, and notEmpty at typed boundaries.
func TestArrayLiteralLengthContractsUseExactCompactLength(t *testing.T) {
	errors := analyzeSource(t, fixture(t, "length_contracts_6.sec"))
	for _, want := range []string{
		"array literal length 1 violates minLen contract TwoToThree 2",
		"array literal length 4 violates maxLen contract TwoToThree 3",
		"array literal length 3 violates exactLen contract ExactlyTwo 2",
		"array literal length 0 violates notEmpty contract Present",
	} {
		if !errorsContainMessage(errors, want) {
			t.Errorf("errors %v do not contain %q", errors, want)
		}
	}
	if len(errors) != 5 {
		t.Fatalf("array literal length errors = %v, want 5", errors)
	}
}

// rules/types/contracts.md — unique compares direct elements using compile-time
// semantic equality and reports the first source-ordered proven duplicate.
func TestArrayLiteralUniqueContractRejectsProvenDirectDuplicates(t *testing.T) {
	errors := analyzeSource(t, fixture(t, "length_contracts_7.sec"))
	for _, want := range []string{
		"array literal element 3 duplicates element 1 under unique contract UniqueValues",
		"array literal element 2 duplicates element 1 under unique contract UniqueValues",
		"array literal element 2 duplicates element 1 under unique contract UniqueText",
	} {
		if !errorsContainMessage(errors, want) {
			t.Errorf("errors %v do not contain %q", errors, want)
		}
	}
	if len(errors) != 3 {
		t.Fatalf("array literal unique errors = %v, want 3", errors)
	}
}

// rules/types/default_values.md — array defaults must satisfy the complete
// named collection contract set before implicit materialization or preview.
func TestArrayDefaultsRespectLengthNotEmptyAndUniqueContracts(t *testing.T) {
	intType := sema.Type{Name: "int", Kind: sema.IntType}

	required := sema.NewDynamicArrayType(intType)
	required.Name, required.Named = "Required", true
	required.Contracts = []sema.Contract{sema.MarkerContract{Name: "notEmpty"}}
	if resolved := sema.DefaultValueOf(required); resolved.Kind != sema.NoDefault {
		t.Fatalf("notEmpty dynamic default = %#v", resolved)
	}
	if display, kind, ok := sema.DefaultValuePreview(required, 8); ok || kind != sema.NoDefault || display != "" {
		t.Fatalf("notEmpty dynamic preview = %q, %q, %v", display, kind, ok)
	}

	empty := sema.NewDynamicArrayType(intType)
	empty.Name, empty.Named = "Empty", true
	empty.Contracts = []sema.Contract{sema.LengthContract{Name: "exactLen", Value: big.NewInt(0)}, sema.MarkerContract{Name: "unique"}}
	if resolved := sema.DefaultValueOf(empty); resolved.Kind != sema.ArrayDefault {
		t.Fatalf("exact-empty unique default = %#v", resolved)
	}

	uniquePair := sema.NewFixedArrayType(intType, big.NewInt(2))
	uniquePair.Name, uniquePair.Named = "UniquePair", true
	uniquePair.Contracts = []sema.Contract{sema.MarkerContract{Name: "unique"}}
	if resolved := sema.DefaultValueOf(uniquePair); resolved.Kind != sema.NoDefault {
		t.Fatalf("repeated unique fixed-array default = %#v", resolved)
	}

	uniqueSingle := sema.NewFixedArrayType(intType, big.NewInt(1))
	uniqueSingle.Name, uniqueSingle.Named = "UniqueSingle", true
	uniqueSingle.Contracts = []sema.Contract{sema.MarkerContract{Name: "unique"}}
	if resolved := sema.DefaultValueOf(uniqueSingle); resolved.Kind != sema.ArrayDefault {
		t.Fatalf("single-element unique default = %#v", resolved)
	}

	errors := analyzeSource(t, fixture(t, "length_contracts_8.sec"))
	if len(errors) != 2 || !errorsContainMessage(errors, "variable required of type RequiredValues requires an initializer because the type has no default value") || !errorsContainMessage(errors, "variable pair of type UniquePair requires an initializer because the type has no default value") {
		t.Fatalf("contracted array default diagnostics = %v", errors)
	}
}
