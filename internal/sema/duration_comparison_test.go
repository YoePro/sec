package sema

import (
	"strings"
	"testing"
)

// duration has the six comparisons between two durations and no arithmetic
// (owner decision 2026-11-08). The quantity-literal case (`a > 250<ms>`
// needs `duration(250<ms>)`) needs the unit library and is covered by the
// CLI probe recorded in governance.
//
// Rules:
//   - rules/types/temporal.md — § 5.8(4)–(5), § 5.5(1)–(2)
func TestDurationComparisonsOnly(t *testing.T) {
	valid := analyzeSourceRaw(t, `module main

fn Compare(a: duration, b: duration) bool {
    return a == b || a != b || a < b || a <= b || a > b || a >= b
}
`)
	if len(valid) != 0 {
		t.Fatalf("unexpected errors: %v", valid)
	}
	cases := map[string]struct{ body, want string }{
		"plain number": {"fn F(a: duration) bool {\n    return a < 5\n}", "cannot order duration and int"},
		"addition":     {"fn F(a: duration, b: duration) duration {\n    return a + b\n}", "operator + is not defined for duration"},
		"subtraction":  {"fn F(a: duration, b: duration) duration {\n    return a - b\n}", "operator - is not defined for duration"},
		"scaling":      {"fn F(a: duration) duration {\n    return a * 2\n}", "operator * is not defined for duration"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n\n"+test.body+"\n")
			for _, err := range errors {
				if strings.Contains(err.Message, test.want) {
					return
				}
			}
			t.Fatalf("no diagnostic containing %q; got %v", test.want, errors)
		})
	}
}
