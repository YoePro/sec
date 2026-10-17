package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// TestNativeTrackBuildsAndRuns builds programs through the single compilation
// track (Semantic IR -> Sec dialect -> standard MLIR -> LLVM dialect -> LLVM
// IR -> clang) and runs them: command status, loops, short-circuit logic,
// unsigned division, and checked-arithmetic failures reaching the minimal
// panic endpoint (MD-101).
//
// Rules:
//   - rules/compiler/compiler_pipeline.md — §§ 32–34
//   - rules/compiler/initialization.md — §§ 21, 23, 28
//   - rules/errors/panic.md — §§ 8.1, 12(2)
func TestNativeTrackBuildsAndRuns(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	for _, test := range []struct {
		fixture string
		status  int
		trap    bool
	}{
		{"add", 5, false},
		{"loop", 5, false},
		{"unsigned", 0, false},
		{"overflow_trap", 0, true},
		{"division_trap", 0, true},
		{"for_loops", 0, false},
		{"switch", 0, false},
		{"integer_conversions", 0, false},
		{"characters", 0, false},
		{"integer_enums", 0, false},
	} {
		t.Run(test.fixture, func(t *testing.T) {
			program := filepath.Join(t.TempDir(), test.fixture)
			_, stderr, code := runCLIForDiagnostics(t, "build", "../../testdata/build/native/"+test.fixture+".sec", "-o", program)
			if code != 0 {
				t.Fatalf("build: %d %s", code, stderr)
			}
			err := exec.Command(program).Run()
			var exit *exec.ExitError
			switch {
			case test.trap:
				if !errors.As(err, &exit) || exit.Exited() {
					t.Fatalf("checked failure did not trap: %v", err)
				}
			case test.status == 0:
				if err != nil {
					t.Fatalf("status: %v", err)
				}
			default:
				if !errors.As(err, &exit) || exit.ExitCode() != test.status {
					t.Fatalf("status = %v, want %d", err, test.status)
				}
			}
		})
	}
}

// TestNativeTrackNamesUnloweredConstructs keeps the track honest: a valid
// program using a construct the track does not lower yet fails as an
// implementation capability limit naming the Sec operation, with no output
// and no fallback backend.
func TestNativeTrackNamesUnloweredConstructs(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	program := filepath.Join(t.TempDir(), "union")
	stdout, stderr, code := runCLIForDiagnostics(t, "build", "../../testdata/build/native/unsupported_union.sec", "-o", program)
	if code != 4 || stdout != "" || !strings.Contains(stderr, "[unsupported-lowering]") ||
		!strings.Contains(stderr, "does not lower 'sec.union.is_variant' yet at ../../testdata/build/native/unsupported_union.sec:10:9") {
		t.Fatalf("%d %s %s", code, stdout, stderr)
	}
	if _, err := exec.LookPath(program); err == nil {
		t.Fatal("a rejected build produced a program")
	}
}
