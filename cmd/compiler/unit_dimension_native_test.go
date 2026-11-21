package main

import (
	"errors"
	"os/exec"
	"testing"

	"sec/internal/testsupport"
)

// UnitDimension values run natively: equality of parameters and returned
// values is exact structural equality of the canonical mappings (equal
// dimensions from different units and constants compare equal, distinct ones
// unequal), AxisCount and IsDimensionless read the materialized value, and
// the dimensionless mapping works. The track selects schema 13 only for such
// modules.
// Rules: rules/corrections/applied/md062-unit-metadata-value-types-correction-20261027-20261121.md — §§ 7(5)-(7), 7(12).
func TestUnitDimensionValuesRun(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	var exit *exec.ExitError
	if err := runNativeProgram(t, "../../testdata/build/native/unit_dimension_values.sec"); !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("status = %v, want 42 (another status names the failing check)", err)
	}
}
