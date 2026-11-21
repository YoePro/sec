package sema

import (
	"os"
	"strings"
	"testing"
)

// A fixed-array spread has a compile-time count and element type, so its
// indexed copies infer owned heterogeneous pack positions like separate
// arguments, mixed with ordinary arguments, other spreads and a fixed prefix.
// Rules: rules/declarations/spread.md — §§ 4, 4.1-4.3; rules/declarations/functions.md — § 38A(4)(2), (9).
func TestTypePackAcceptsFixedArraySpread(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/fixed_array_spread.sec")
	if err != nil {
		t.Fatal(err)
	}
	assertSemaErrors(t, analyzeSourceRaw(t, string(data)), nil)
}

// A borrowed pack position needs per-element borrow provenance (S1239, as for
// a fixed-array spread into ordinary ref parameters); move-only elements are
// never copied out; a runtime-length source cannot infer a heterogeneous pack
// and is reported at the spread with that reason.
// Rules: rules/declarations/spread.md — § 4.3; rules/declarations/functions.md — § 38A(4)(3), (7)(1), (10)(1);
// rules/memory/borrowing.md — § 15.4.
func TestTypePackFixedArraySpreadRejections(t *testing.T) {
	prefix := `module main

@noCopy
type Handle struct {
	id: int,
}

fn Take[...Types](values: ...Types) uint {
	return values.Len
}

fn Count[...Types](values: ...ref Types) uint {
	return values.Len
}
`
	cases := []struct{ name, body, want string }{
		{"borrowed pack", "fn F() uint {\n\tlet numbers: int[2] := [1, 2]\n\treturn Count(numbers...)\n}\n", "spread argument requires per-element borrow or reference provenance"},
		{"move-only elements", "fn F() uint {\n\tlet handles: Handle[1] := [Handle { id: 1 }]\n\treturn Take(handles...)\n}\n", "indexed expansion would require unsupported consuming element reads"},
		{"runtime-length source", "fn F(view: ref int[]) uint {\n\treturn Take(view...)\n}\n", "a runtime-length ref int[] cannot be spread into the heterogeneous type pack Types"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, prefix+test.body)
			if len(errors) != 1 || !strings.Contains(errors[0].Message, test.want) {
				t.Fatalf("want one %q, got %v", test.want, errors)
			}
		})
	}
}
