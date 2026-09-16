package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// rules/errors/panic.md §§4 and 15.3 make a literal-false assertion a
// non-returning panic path, including when a non-void function needs no
// ordinary return on that path.
func TestAssertFalseTerminatesFrontendControlFlow(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/assert_false_termination_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	assertSemaErrors(t, analyzeSourceRaw(t, string(source)), nil)
}

func TestAssertFalseRejectsFollowingCodeButUnprovenAssertFallsThrough(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/assert_false_termination_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	if len(errors) != 2 {
		t.Fatalf("errors = %+v, want unreachable-code and missing-return diagnostics", errors)
	}
	joined := errors[0].Message + "\n" + errors[1].Message
	for _, want := range []string{"unreachable statement", "StillFallsThrough", "return"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("errors = %+v, want %q", errors, want)
		}
	}
	if errors[0].ID != diagnostics.UnreachableStatement && errors[1].ID != diagnostics.UnreachableStatement {
		t.Fatalf("missing S3001 for statement after assert false: %+v", errors)
	}
}
