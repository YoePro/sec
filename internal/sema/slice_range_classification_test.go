package sema

import (
	"math/big"
	"strings"
	"testing"
)

// The canonical slice-range classification applies the RangeError
// precedence — negative endpoint, then start after end, then source bound —
// with exact arithmetic beyond int64 and never fails on unknown facts
// (P16-13).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 14–19
func TestClassifySliceRangePrecedence(t *testing.T) {
	n := func(value int64) *big.Int { return big.NewInt(value) }
	huge, _ := new(big.Int).SetString("18446744073709551616", 10)
	beyond := new(big.Int).Add(huge, big.NewInt(1))
	tests := []struct {
		name  string
		r     SliceRangeEndpoints
		want  SliceRangeFailure
		cause SliceRangeCause
	}{
		{"valid exclusive", SliceRangeEndpoints{Start: n(1), End: n(3), Length: n(4)}, SliceRangeValid, SliceRangeCauseNone},
		{"empty at end", SliceRangeEndpoints{Start: n(4), End: n(4), Length: n(4)}, SliceRangeValid, SliceRangeCauseNone},
		{"inclusive last", SliceRangeEndpoints{Start: n(0), End: n(3), Inclusive: true, Length: n(4)}, SliceRangeValid, SliceRangeCauseNone},
		{"open range", SliceRangeEndpoints{StartOmitted: true, EndOmitted: true, Length: n(4)}, SliceRangeValid, SliceRangeCauseNone},
		{"negative start beats start-after-end and bounds", SliceRangeEndpoints{Start: n(-1), End: n(-5), Length: n(4)}, SliceRangeInvalidRange, SliceRangeCauseNegativeStart},
		{"negative end beats start-after-end", SliceRangeEndpoints{Start: n(3), End: n(-1), Length: n(4)}, SliceRangeInvalidRange, SliceRangeCauseNegativeEnd},
		{"start after end beats bounds", SliceRangeEndpoints{Start: n(9), End: n(7), Length: n(4)}, SliceRangeStartAfterEnd, SliceRangeCauseStartAfterEnd},
		{"start after end without length", SliceRangeEndpoints{Start: n(3), End: n(1)}, SliceRangeStartAfterEnd, SliceRangeCauseStartAfterEnd},
		{"inclusive descending", SliceRangeEndpoints{Start: n(3), End: n(2), Inclusive: true}, SliceRangeStartAfterEnd, SliceRangeCauseStartAfterEnd},
		{"start past length", SliceRangeEndpoints{Start: n(5), EndOmitted: true, Length: n(4)}, SliceRangeOutOfBounds, SliceRangeCauseStartPastLength},
		{"exclusive end past length", SliceRangeEndpoints{Start: n(0), End: n(5), Length: n(4)}, SliceRangeOutOfBounds, SliceRangeCauseExclusiveEndPast},
		{"inclusive end at length", SliceRangeEndpoints{StartOmitted: true, End: n(4), Inclusive: true, Length: n(4)}, SliceRangeOutOfBounds, SliceRangeCauseInclusiveEndAtLen},
		{"unknown endpoints never fail", SliceRangeEndpoints{Length: n(4)}, SliceRangeValid, SliceRangeCauseNone},
		{"beyond int64 valid", SliceRangeEndpoints{Start: new(big.Int).Set(huge), End: new(big.Int).Set(beyond), Length: new(big.Int).Set(beyond)}, SliceRangeValid, SliceRangeCauseNone},
		{"beyond int64 inclusive at length", SliceRangeEndpoints{Start: n(0), End: new(big.Int).Set(beyond), Inclusive: true, Length: new(big.Int).Set(beyond)}, SliceRangeOutOfBounds, SliceRangeCauseInclusiveEndAtLen},
	}
	for _, test := range tests {
		got := ClassifySliceRange(test.r)
		if got.Failure != test.want || got.Cause != test.cause {
			t.Errorf("%s: got %+v, want %s/%s", test.name, got, test.want, test.cause)
		}
	}
}

// A compile-time-provable invalid slice range reports exactly one error, the
// first failing condition under the canonical precedence, and start after end
// is diagnosed for sources whose length is not known (P16-13).
func TestConstantSliceRangeDiagnosticsFollowPrecedence(t *testing.T) {
	tests := []struct {
		name, slice, want string
	}{
		{"negative start first", "fixed[-1..<9]", "slice lower bound must be non-negative"},
		{"start after end before bounds", "fixed[9..<7]", "descending slice range 9..<7 is invalid"},
		{"exclusive end past length", "fixed[0..<5]", "exclusive slice upper bound 5 exceeds length 4"},
		{"inclusive end at length", "fixed[..4]", "inclusive slice upper bound 4 exceeds final index 3"},
		{"start past length", "fixed[5..]", "slice lower bound 5 exceeds length 4"},
		{"dynamic source descending", "dynamic[3..<1]", "descending slice range 3..<1 is invalid"},
		{"dynamic source negative start first", "dynamic[-1..<-3]", "slice lower bound must be non-negative"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "module main\n\nfn Use(fixed: ref int[4], dynamic: ref int[]) void {\n    let view := ref " + test.slice + "\n}\n"
			errors := analyzeSourceRaw(t, source)
			if len(errors) != 1 || !strings.Contains(errors[0].Message, test.want) {
				t.Fatalf("want exactly %q, got %v", test.want, errors)
			}
		})
	}
	if errors := analyzeSourceRaw(t, "module main\n\nfn Use(fixed: ref int[4]) void {\n    let view := ref fixed[1..<4]\n    let empty := ref fixed[4..<4]\n}\n"); len(errors) != 0 {
		t.Fatalf("valid ranges rejected: %v", errors)
	}
}
