package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sec/internal/testsupport"
)

// P20 typed float conversions run on the native track: proven-exact
// widening, same-format and integer-to-float conversions, float-to-bool, and
// checked narrowing, float-to-integer and integer-to-float conversions under
// try whose NonFinite, OutOfRange and PrecisionLoss failures reach the
// matching ConversionError variant. rune_conversion_checked covers the
// integer-to-rune InvalidScalar boundaries (0xD7FF, 0xD800, 0xDFFF, 0xE000,
// 0x10FFFF, 0x110000) and rune-to-char OutOfRange. Each fixture returns 42 only
// when every case matches.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 41, 44–59, 70–72, 79.
func TestFloatConversionsBuildAndRun(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	for _, fixture := range []string{"float_conversion_exact", "float_conversion_checked", "rune_conversion_checked"} {
		t.Run(fixture, func(t *testing.T) {
			program := filepath.Join(t.TempDir(), fixture)
			if _, stderr, code := runCLIForDiagnostics(t, "build", "../../testdata/build/native/"+fixture+".sec", "-o", program); code != 0 {
				t.Fatalf("build: %d %s", code, stderr)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := exec.CommandContext(ctx, program).Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 42 {
				t.Fatalf("status = %v, want 42 (another status names the failing case)", err)
			}
		})
	}
}

// The emitted Sec MLIR carries the exact conversions with their proofs, the
// total float-to-bool and the checked form with its ordered failures.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 55, 70–73.
func TestFloatConversionSecMLIR(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	for fixture, wants := range map[string][]string{
		"float_conversion_exact": {
			`"sec.numeric.convert_exact"(%v0) {sec.conversion_proof = {basis = "source-domain"`, `: (f32) -> f64`,
			`"sec.float.to_bool"(%v0) : (f64) -> i1`, `basis = "value-interval"`, `: (si16) -> f32`,
		},
		"float_conversion_checked": {
			`<{failures = ["OutOfRange", "PrecisionLoss"]}> : (f64) -> (f32, i1, i1, i1)`,
			`<{failures = ["NonFinite", "OutOfRange", "PrecisionLoss"]}> : (f64) -> (si32, i1, i1, i1, i1)`,
			`<{failures = ["PrecisionLoss"]}> : (si64) -> (f64, i1, i1)`,
		},
	} {
		stdout, stderr, code := runCLIForDiagnostics(t, "emit-sec-mlir", "../../testdata/build/native/"+fixture+".sec", "-o", "-")
		if code != 0 {
			t.Fatalf("%s: %d %s", fixture, code, stderr)
		}
		for _, want := range wants {
			if !strings.Contains(stdout, want) {
				t.Errorf("%s lacks %q:\n%s", fixture, want, stdout)
			}
		}
	}
}

// A checked float conversion's failure is exactly ConversionError: a bodyless
// try in a function with another error type is rejected, and so is mixing it
// with an ArithmeticError failure, because no hidden error union is formed.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 39–40, 57, 89.
func TestFloatConversionErrorChannel(t *testing.T) {
	for name, check := range map[string]struct{ source, want string }{
		"wrong error type": {
			"module main\n\nfn Narrow(x: float64) Result[float32, ArithmeticError] {\n    let y := try float32(x)\n    return Ok(y)\n}\n\nfn main() int {\n    return 0\n}\n",
			"propagates ConversionError from float32(x)",
		},
		"no hidden union": {
			"module main\n\nfn Both(x: float64, a: int, b: int) Result[float32, ConversionError] {\n    let q := try a / b\n    let y := try float32(x)\n    return Ok(y)\n}\n\nfn main() int {\n    return 0\n}\n",
			"propagates ArithmeticError",
		},
	} {
		file := filepath.Join(t.TempDir(), "main.sec")
		if err := os.WriteFile(file, []byte(check.source), 0o644); err != nil {
			t.Fatal(err)
		}
		_, stderr, code := runCLIForDiagnostics(t, "check", file)
		if code == 0 || !strings.Contains(stderr, "S1093") || !strings.Contains(stderr, check.want) {
			t.Errorf("%s: %d %s", name, code, stderr)
		}
	}
}
