package main

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		{"explicit_return", 0, false},
		{"loop", 5, false},
		{"unsigned", 0, false},
		{"overflow_trap", 0, true},
		{"division_trap", 0, true},
		{"for_loops", 0, false},
		{"switch", 0, false},
		{"switch_partial_relations", 0, false},
		{"switch_loop_control", 0, false},
		{"logical_contexts", 0, false},
		{"overload_identity", 0, false},
		{"overload_specialization_identity", 0, false},
		{"overload_nominal_identity", 0, false},
		{"integer_conversions", 0, false},
		{"characters", 0, false},
		{"character_predicates", 0, false},
		{"enums", 0, false},
		{"enum_switch_classes", 0, false},
		{"range_steps", 0, false},
		{"bool_switch", 0, false},
		{"discard", 0, false},
		{"statics", 0, false},
		{"static_cte", 0, false},
		{"panic_trap", 0, true},
		{"assert_trap", 0, true},
		{"integer_enums", 0, false},
		{"enum_known_narrowing", 0, false},
		{"enum_known_bindings", 0, false},
		{"float_core", 0, false},
		{"floats", 0, false},
		{"float_literals", 0, false},
		{"float_operator_matrix", 0, false},
		{"float_core_matrix", 0, false},
		{"bit_extract", 0, false},
		{"tagged_values", 0, false},
		{"error_variant_patterns", 0, false},
		{"try_handler_forms", 0, false},
		{"named_scalars", 0, false},
		{"named_integer_storage", 0, false},
		{"named_integer_storage_overflow", 0, true},
		{"try_positions", 0, false},
		{"match_loop_control", 0, false},
		{"state_tests", 0, false},
		{"static_initialization", 0, false},
	} {
		t.Run(test.fixture, func(t *testing.T) {
			program := filepath.Join(t.TempDir(), test.fixture)
			_, stderr, code := runCLIForDiagnostics(t, "build", "../../testdata/build/native/"+test.fixture+".sec", "-o", program)
			if code != 0 {
				t.Fatalf("build: %d %s", code, stderr)
			}
			// A wrong loop-control target can otherwise hang this regression.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := exec.CommandContext(ctx, program).Run()
			if ctx.Err() != nil {
				t.Fatalf("program did not terminate: %v", ctx.Err())
			}
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

// Statics reach the artifact as compile-time initialized storage: no global
// constructor, no startup function and no runtime-initialization call exists,
// even when an initializer calls Sec functions, and a runtime-free program
// acquires no runtime startup.
// Rules: rules/compiler/initialization.md — §§ 4, 6, 7, 11, 45.2, 45.3.
func TestStaticInitializationHasNoHiddenStartup(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-llvm", "../../testdata/build/native/static_initialization.sec", "-o", "-")
	if code != 0 {
		t.Fatalf("emit-llvm: %d %s", code, stderr)
	}
	for _, want := range []string{"= internal constant i64 4096", "= internal constant i64 2048", "= internal global i64 4", "define i32 @main()"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("LLVM lacks %q:\n%s", want, stdout)
		}
	}
	for _, forbidden := range []string{"global_ctors", "sec_runtime_init"} {
		if strings.Contains(stdout, forbidden) {
			t.Errorf("LLVM contains hidden startup %q", forbidden)
		}
	}
}
