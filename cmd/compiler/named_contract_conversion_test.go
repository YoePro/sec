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

func runNativeProgram(t *testing.T, source string) error {
	t.Helper()
	program := filepath.Join(t.TempDir(), "program")
	if _, stderr, code := runCLIForDiagnostics(t, "build", source, "-o", program); code != 0 {
		t.Fatalf("build %s: %d %s", source, code, stderr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, program).Run()
}

// Runtime conversions into constrained named integer types run natively:
// intrinsic OutOfRange first, then range (inclusive and exclusive), in, odd,
// even and multipleOf in declaration order, each violation reported as
// ConversionError.Contract(ContractError.Violation { Kind, DeclarationIndex }),
// including local handlers and proven constant conversions. Float carriers
// check range bounds shaped into the base type with ordered predicates (NaN
// fails) and finite.
// Rules: rules/types/contracts.md — §§ 12, 13.1; rules/mlir/packages/sec-mlir-dialect_package22.md — §§ 11–13, 18–19.
func TestNamedContractConversionRuns(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	for _, fixture := range []string{"named_contract_conversion", "named_contract_float"} {
		err := runNativeProgram(t, "../../testdata/build/native/"+fixture+".sec")
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 42 {
			t.Fatalf("%s: status = %v, want 42 (another status names the failing case)", fixture, err)
		}
	}
}

// Without try a fitting value converts and a violation is the ContractFailure
// panic, which the native track lowers to its non-returning trap endpoint.
// Rules: rules/types/contracts.md — § 12(2); rules/errors/panic.md — § 2.
func TestNamedContractConversionPanics(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	fixture := "../../testdata/build/native/named_contract_panic.sec"
	var exit *exec.ExitError
	if err := runNativeProgram(t, fixture); !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("fitting value: status = %v, want 42", err)
	}
	source, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	violating := filepath.Join(t.TempDir(), "violating.sec")
	if err := os.WriteFile(violating, []byte(strings.Replace(string(source), "Convert(42)", "Convert(420)", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runNativeProgram(t, violating); !errors.As(err, &exit) || exit.Exited() {
		t.Fatalf("violation did not trap: %v", err)
	}
}

// Semantic IR keeps the contract checks explicit before the nominal value and
// builds the canonical nested error payload.
// Rules: rules/types/contracts.md — § 13.1; rules/mlir/packages/sec-mlir-dialect_package22.md — §§ 18, 23.
func TestNamedContractConversionIR(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", "../../testdata/build/native/named_contract_conversion.sec", "-o", "-")
	if code != 0 {
		t.Fatalf("emit-ir: %d %s", code, stderr)
	}
	start := strings.Index(stdout, `func "main::ToSmall(int)"`)
	if start < 0 {
		t.Fatalf("missing ToSmall:\n%s", stdout)
	}
	body := stdout[start:]
	body = body[:strings.Index(body, "\n  }")]
	order := []string{"numeric.convert-checked", "int.compare lt", "int.compare ge", "int.bitwise and", "named.convert"}
	position := -1
	for _, want := range order {
		next := strings.Index(body, want)
		if next <= position {
			t.Fatalf("%q missing or out of order:\n%s", want, body)
		}
		position = next
	}
	if !strings.Contains(body, "Kind=") || !strings.Contains(body, "DeclarationIndex=") {
		t.Errorf("missing ContractError.Violation construction:\n%s", body)
	}
}
