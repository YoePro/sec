package main

import (
	"errors"
	"os/exec"
	"testing"

	"sec/internal/testsupport"
)

// P20 §§ 74, 76: module-level float statics (plain float, float32 and a
// float64 negative zero) lower with their canonical bit patterns into static
// storage and run natively with exact binary32 rounding and signed zero.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 13-20, 74, 76.
func TestFloatStaticsRun(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	var exit *exec.ExitError
	if err := runNativeProgram(t, "../../testdata/build/native/float_statics.sec"); !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("status = %v, want 42 (another status names the failing check)", err)
	}
}

// P20-19 (§§ 13-18, 82): compile-time folded float statics equal the same
// computation at run time — per-operation rounding in binary64 and binary32
// (0.1 + 0.2, a chain at 2^24), the minimum subnormal, division and
// signed-zero comparisons.
func TestFloatCompileTimeParity(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	var exit *exec.ExitError
	if err := runNativeProgram(t, "../../testdata/build/native/float_cte_parity.sec"); !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("status = %v, want 42 (another status names the failing check)", err)
	}
}

// P20 §§ 28, 68, 82: float type properties and non-finite values fold at
// compile time with runtime parity (infinities, overflow, NaN unordered).
func TestFloatCompileTimeNonFiniteParity(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	var exit *exec.ExitError
	if err := runNativeProgram(t, "../../testdata/build/native/float_cte_nonfinite.sec"); !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("status = %v, want 42 (another status names the failing check)", err)
	}
}

// P20 §§ 20, 44–55, 59: exact conversions folded by CTE match the runtime.
func TestFloatCompileTimeConversionParity(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	var exit *exec.ExitError
	if err := runNativeProgram(t, "../../testdata/build/native/float_cte_conversions.sec"); !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("status = %v, want 42 (another status names the failing check)", err)
	}
}

// P20 §§ 29–36, 82: float core methods folded by CTE match the runtime.
func TestFloatCompileTimeCoreMethodParity(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	var exit *exec.ExitError
	if err := runNativeProgram(t, "../../testdata/build/native/float_cte_core.sec"); !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("status = %v, want 42 (another status names the failing check)", err)
	}
}
