package sema

import (
	"fmt"
	"strings"
	"testing"

	"sec/internal/ast"
)

// Range segments expand inside fixed-array literals: the length is the sum of
// the segment counts, `..` includes and `..<` excludes the upper bound, the
// plan keeps one entry per source segment, and Append on an owning dynamic
// array accepts a range.
//
// Rules:
//   - rules/collections/collections.md — § 5.6a "Range segments in array literals", § 6.7 "Append"
func TestArrayLiteralRangeSegments(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
fn IsLower(value: rune) bool {
	let r := [224r..246r, 248r..255r, 257r..383r, 945r..969r]
	return value in r
}

fn Digits() int {
	let digits: uint8[13] := [0..<10, 42, 7..8]
	let empty: int[1] := [5..<5, 9]
	discard digits
	discard empty
	return 0
}

fn Grow() Result[void, CollectionError] {
	let mut values: rune[]
	try values.Append(0r..127r)
	try values.Append(65r)
	return Ok()
}
`)
	if len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}
	plans := map[string]string{}
	for literal, plan := range analyzer.resolvedArrayLiteralPlans {
		parts := []string{}
		for _, entry := range plan.Entries {
			part := string(entry.Kind) + ":" + entry.Length.String()
			if entry.Kind == ArrayLiteralRange {
				part += fmt.Sprintf("[%s..%s]", entry.RangeLower, entry.RangeUpper)
			}
			parts = append(parts, part)
		}
		plans[literal.String()] = plan.Length.String() + " " + typeDisplayName(plan.ElementType) + " " + strings.Join(parts, ",")
	}
	for literal, want := range map[string]string{
		"[224r..246r, 248r..255r, 257r..383r, 945r..969r]": "183 rune range:23[224..246],range:8[248..255],range:127[257..383],range:25[945..969]",
		"[0..<10, 42, 7..8]": "13 uint8 range:10[0..9],element:1,range:2[7..8]",
		"[5..<5, 9]":         "1 int range:0[5..4],element:1",
	} {
		if plans[literal] != want {
			t.Errorf("plan of %s = %q, want %q (all %v)", literal, plans[literal], want, plans)
		}
	}
	appended := 0
	for call, entry := range analyzer.resolvedRangeAppends {
		appended++
		if entry.Length.Int64() != 128 || entry.Type.Kind != RuneType || !strings.Contains(call.String(), "Append") {
			t.Errorf("Append range = %+v", entry)
		}
	}
	if appended != 1 {
		t.Errorf("Append range facts = %d, want 1", appended)
	}
	_ = ast.RangeExpression{}
}

// Invalid range segments are rejected with S1119: runtime bounds, descending
// bounds, surrogate runes, non-integer element types, bounds outside the
// element type, a dynamic-array literal target, Append on a list, and a
// length that does not match a fixed target.
func TestArrayLiteralRangeSegmentErrors(t *testing.T) {
	for _, test := range []struct {
		name, body, want string
	}{
		{"runtime bound", "fn F(n: int) int {\n\tlet values := [0..n]\n\treturn 0\n}", "range segment bounds must be compile-time constants"},
		{"descending", "fn F() int {\n\tlet values := [9..1]\n\treturn 0\n}", "range segment lower bound 9 exceeds upper bound 1"},
		{"surrogates", "fn F() int {\n\tlet values := [55290r..57400r]\n\treturn 0\n}", "covers surrogate code points"},
		{"float", "fn F() int {\n\tlet values := [1.0..2.0]\n\treturn 0\n}", "range segment element type must be an integer type or rune"},
		{"out of range", "fn F() int {\n\tlet values: uint8[2] := [255..256]\n\treturn 0\n}", "range segment bound 256 does not fit uint8"},
		{"dynamic target", "fn F() int {\n\tlet mut values: rune[] := [1r..3r]\n\treturn 0\n}", "range segment cannot initialize owning dynamic array rune[]"},
		{"list append", "fn F() Result[void, CollectionError] {\n\tlet mut values: list[int] := list[int] {}\n\ttry values.Append(1..3)\n\treturn Ok()\n}", "list[int].Append does not accept a range"},
		{"ordinary argument", "fn Take(value: int) int {\n\treturn value\n}\n\nfn F() int {\n\treturn Take(1..3)\n}", "a range is not a value in Sec 0.1"},
		{"length mismatch", "fn F() int {\n\tlet values: int[3] := [1..4]\n\treturn 0\n}", "array literal has 4 elements, expected 3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, errors := analyzeSourceWithAnalyzer(t, test.body)
			found := false
			for _, err := range errors {
				if strings.Contains(err.Message, test.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("errors = %v, want %q", errors, test.want)
			}
		})
	}
}
