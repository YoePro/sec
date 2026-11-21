package main

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// TestEnumRepresentabilityAcceptance checks closed-domain and known-value exact
// conversions alongside the canonical checked Result, try propagation and local
// handler. The exact subset executes natively; the checked subset preserves
// the public P18 unsupported-lowering boundary without publishing partial IR.
// Rules: rules/declarations/enums.md §12.1; rules/types/types.md §§25(11),25.1;
// rules/errors/errorhandling.md §§9–11.
func TestEnumRepresentabilityAcceptance(t *testing.T) {
	fixture := "../../testdata/build/native/enum_representability.sec"
	if _, stderr, code := runCLIForDiagnostics(t, "check", fixture); code != 0 {
		t.Fatalf("check: %d %s", code, stderr)
	}
	// The public inspection route is still capped at P18. Its separate
	// checked-lowering backlog must not silently publish an exact conversion.
	output, stderr, code := runCLIForDiagnostics(t, "emit-ir", fixture, "-o", "-")
	if code != 4 || output != "" || !strings.Contains(stderr, "fallible enum to integer conversion Wide to uint8") || !strings.Contains(stderr, "unsupported-lowering") {
		t.Fatalf("checked boundary: %d %s %s", code, output, stderr)
	}
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	var exit *exec.ExitError
	if err := runNativeProgram(t, "../../testdata/build/native/enum_representability_exact.sec"); !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("native result: %v; want 42 (other codes identify a failing case)", err)
	}
}
