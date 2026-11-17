package main

import (
	"strings"
	"testing"
)

// @noPanic on init and free covers every path, including paths after or
// beside an early return, and the compiler-generated cleanup of the remaining
// owned fields after a free body: a field whose type's free may panic makes
// the outer free panic-capable, and early returns cannot evade either
// obligation (MD-030 § 9(6)).
//
// Rules:
//   - rules/corrections/applied/md030-no-panic-attribute-placement-correction-20261017.md — §§ 9(4), 9(6)
//   - rules/memory/destruction.md — §§ 15.4, 15.5
//   - rules/errors/panic.md — § 21
func TestNoPanicEarlyReturnAndFieldCleanup(t *testing.T) {
	if _, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/no_panic_bodies_early_return_valid.sec"); code != 0 {
		t.Fatal(code, stderr)
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/no_panic_bodies_early_return_invalid.sec")
	if code == 0 {
		t.Fatal("an early return or field cleanup evaded @noPanic")
	}
	for _, want := range []string{
		"init Outer.init does not satisfy @noPanic: reachable may-panic-arithmetic",
		"init Plain.init does not satisfy @noPanic: reachable may-panic-arithmetic",
		"free Outer.free does not satisfy @noPanic: reachable may-panic-assertion effect via Outer.free -> Inner.free",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("missing %q in:\n%s", want, stderr)
		}
	}
	if count := strings.Count(stderr, "does not satisfy @noPanic"); count != 3 {
		t.Errorf("want exactly 3 failures, got %d:\n%s", count, stderr)
	}
}
