package sema

import (
	"strings"
	"testing"
)

// MD-066/067/068 § 14(2): the frontend rejects removed `As*` members,
// infallible Scale assignment, wrongly typed scales, invalid rounding modes,
// bad format arguments and a `try` around a total decimal method.
// Rules: rules/library/core-library.md — §§ 10.1-10.7;
// rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — § 14(2).
func TestDecimalNegativeMatrix(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

enum RoundingMode {
    NearestEven
}

enum PrecisionError error {
    PrecisionLoss
}

fn Bad(value: decimal, wide: decimal128) Result[void, PrecisionError] {
    let a := value.AsInt()
    let mut m: decimal := 1.5m
    m.Scale = 3
    let b := try value.Rescale(3u)
    let c := try value.RoundTo(2, 7)
    let e := try wide.Floor()
    let f := try value.Rescale(int64(3))
    return Ok()
}
`)
	wants := []string{
		"value.AsInt",
		"assigning fallible property Scale requires try",
		"decimal.Rescale argument 1 must be int32, got uint",
		"decimal.RoundTo argument 2 must be RoundingMode, got int",
		"wide.Floor() contains none",
		"decimal.Rescale argument 1 must be int32, got int64",
	}
	joined := ""
	for _, err := range errs {
		joined += err.Message + "\n"
	}
	for _, want := range wants {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in\n%s", want, joined)
		}
	}
}
