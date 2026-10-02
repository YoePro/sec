package formatter

import (
	"os"
	"testing"
)

// Every canonical contract family formats with single-space header, contract,
// and default separation, and a contiguous simple named-type group aligns its
// base-type and first contract anchors. The result is a fixed point.
//
// Rules:
//   - rules/tooling/formatter.md — §3 canonical model, §6 basic whitespace
//   - rules/tooling/formatter.md — §7(6) and §9(1–2) named-type group alignment
//   - rules/foundations/grammar.md — "Type contracts" and "Default clause"
//   - rules/types/contracts.md — "Applicability"
func TestFormatAllContractFamiliesWithNamedTypeAlignment(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/contract_families.sec")
	if err != nil {
		t.Fatal(err)
	}
	want := `module main

type Percent      int      range 0..100
type Port         int      range 1..<65536 default 8080
type Open         int      range 1..
type Role         string   in ["admin", "user", "guest"] default "user"
type PageOffset   int      multipleOf 4096
type PositiveEven int      range 1..100 even
type OddValue     int      odd
type Name         string   minLen 1 maxLen 64
type Code         string   exactLen 4
type Tags         string[] notEmpty unique
type Finite       float    finite
type Email        string   regex "^[a-z]+@[a-z]+$" minLen 3
`
	got := Format(Source{Text: string(input)}, Options{}).Text
	if got != want {
		t.Fatalf("contract families formatted as:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("contract family formatting is not idempotent:\n%s", again)
	}
}

// Blank lines and comments end a named-type alignment group, generic and
// aggregate headers get single spaces without joining the group, and a
// postfix sequence base type keeps its attached brackets.
//
// Rules:
//   - rules/tooling/formatter.md — §7(9), §9(8) group boundaries
//   - rules/tooling/formatter.md — §15 type declarations and aggregate syntax
func TestFormatNamedTypeGroupBoundariesAndHeaders(t *testing.T) {
	input := "module main\n\ntype   A int   range 0..1\ntype LongerName string\n\ntype B   int\n// divider\ntype CC int even\ntype   Box[T]   struct {\n    value: T,\n}\ntype Failure   union   error {\n    Broken,\n}\ntype Tags string[]   unique\n"
	want := "module main\n\ntype A          int range 0..1\ntype LongerName string\n\ntype B int\n// divider\ntype CC int even\ntype Box[T] struct {\n    value: T,\n}\ntype Failure union error {\n    Broken,\n}\ntype Tags string[] unique\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("named-type headers formatted as:\n%s\nwant:\n%s", got, want)
	}
}
