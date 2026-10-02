package sema

import (
	"math/big"
	"strings"
	"testing"
)

// rules/types/contracts.md — "String and collection contracts".
func TestLengthContractsAreRepresentedOnStringsAndCollections(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
type Label string minLen 2 maxLen 12
type Code string exactLen 4
type Values int[] minLen 1 maxLen 8
type Four int[4] exactLen 4
type Items list[int] minLen 1
type MoreItems Items maxLen 8
`)
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
		for _, contract := range analyzer.types[name].Contracts {
			if length, ok := contract.(LengthContract); ok {
				got = append(got, length.Name+"="+length.Value.String())
			}
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s length contracts = %v, want %v", name, got, want)
		}
	}
}

func TestLengthContractValuesAndApplicability(t *testing.T) {
	errors := analyzeSource(t, `
type Number int minLen 1
type Negative string minLen -1
type Dynamic string maxLen limit
`)
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
	errors := analyzeSource(t, `
type AtLeastFive string minLen 5
type ImpossibleRange AtLeastFive maxLen 4
type EmptyCode string exactLen 0 notEmpty
type ExactFour string exactLen 4
type ExactConflict ExactFour exactLen 5
type FixedFour int[4] maxLen 3
`)
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
	errors := analyzeSource(t, `
type ASCII string exactLen 4 default "test"
type UTF8 string exactLen 2 default "å"
type TooShort string minLen 4 default "abc"
type TooLong string maxLen 2 default "abc"
type WrongExact string exactLen 4 default "abc"
`)
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
	errors := analyzeSource(t, `
type Short string maxLen 4
type Present string notEmpty
type Answer string in ["yes", "no"]

type Holder struct {
    value: Short,
}

fn Accept(value: Short) void {}

let valid: Short := "åa"
let tooLong: Short := "hello"
let missing: Present := ""
let invalidAnswer: Answer := "maybe"
let converted := Short("hello")
let holder := Holder { value: "hello" }
let mut current: Short := "ok"

fn Call() void {
    Accept("hello")
    try current = "hello" { Err(error) => { discard error } }
}

fn ReturnTooLong() Short {
    return "hello"
}
`)
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
	errors := analyzeSource(t, `
type TwoToThree int[] minLen 2 maxLen 3
type ExactlyTwo int[] exactLen 2
type Present int[] notEmpty

let valid: TwoToThree := [1, 2]
let tooShort: TwoToThree := [1]
let tooLong: TwoToThree := [1, 2, 3, 4]
let wrongExact: ExactlyTwo := [1, 2, 3]
let empty: Present := []
let pair: int[2] := [1, 2]
let spreadTooLong: ExactlyTwo := [pair..., 3]
`)
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
	errors := analyzeSource(t, `
type UniqueValues int[] unique
type UniqueText string[] unique

let valid: UniqueValues := [1, 2, 3]
let duplicate: UniqueValues := [1, 2, 1]
let folded: UniqueValues := [1 + 1, 2]
let duplicateText: UniqueText := ["å", "å"]
`)
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
	intType := Type{Name: "int", Kind: IntType}

	required := NewDynamicArrayType(intType)
	required.Name, required.Named = "Required", true
	required.Contracts = []Contract{MarkerContract{Name: "notEmpty"}}
	if resolved := DefaultValueOf(required); resolved.Kind != NoDefault {
		t.Fatalf("notEmpty dynamic default = %#v", resolved)
	}
	if display, kind, ok := DefaultValuePreview(required, 8); ok || kind != NoDefault || display != "" {
		t.Fatalf("notEmpty dynamic preview = %q, %q, %v", display, kind, ok)
	}

	empty := NewDynamicArrayType(intType)
	empty.Name, empty.Named = "Empty", true
	empty.Contracts = []Contract{LengthContract{Name: "exactLen", Value: big.NewInt(0)}, MarkerContract{Name: "unique"}}
	if resolved := DefaultValueOf(empty); resolved.Kind != ArrayDefault {
		t.Fatalf("exact-empty unique default = %#v", resolved)
	}

	uniquePair := NewFixedArrayType(intType, big.NewInt(2))
	uniquePair.Name, uniquePair.Named = "UniquePair", true
	uniquePair.Contracts = []Contract{MarkerContract{Name: "unique"}}
	if resolved := DefaultValueOf(uniquePair); resolved.Kind != NoDefault {
		t.Fatalf("repeated unique fixed-array default = %#v", resolved)
	}

	uniqueSingle := NewFixedArrayType(intType, big.NewInt(1))
	uniqueSingle.Name, uniqueSingle.Named = "UniqueSingle", true
	uniqueSingle.Contracts = []Contract{MarkerContract{Name: "unique"}}
	if resolved := DefaultValueOf(uniqueSingle); resolved.Kind != ArrayDefault {
		t.Fatalf("single-element unique default = %#v", resolved)
	}

	errors := analyzeSource(t, `
type RequiredValues int[] minLen 1
type UniquePair int[2] unique

fn Test() void {
	let mut required: RequiredValues
	let mut pair: UniquePair
}
`)
	if len(errors) != 2 || !errorsContainMessage(errors, "variable required of type RequiredValues requires an initializer because the type has no default value") || !errorsContainMessage(errors, "variable pair of type UniquePair requires an initializer because the type has no default value") {
		t.Fatalf("contracted array default diagnostics = %v", errors)
	}
}
