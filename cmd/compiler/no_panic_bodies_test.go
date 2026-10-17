package main

import (
	"os"
	"strings"
	"testing"

	secformatter "sec/internal/formatter"
)

// @noPanic on init, free, each property accessor and a directly initialized
// lambda invokes the existing transitive panic proof on exactly that body;
// an unannotated sibling accessor is not checked, a proof failure is a
// compile error naming the body and the introducing operation, and the
// formatter keeps every attribute on its target (MD-030).
//
// Rules:
//   - rules/foundations/attributes.md — "Callable-body attachment (MD-030)"
//   - rules/errors/panic.md — § 21
//   - rules/corrections/applied/md030-no-panic-attribute-placement-correction-20261017.md — §§ 6, 9
func TestNoPanicCallableBodies(t *testing.T) {
	valid := "../../testdata/sema/no_panic_bodies_valid.sec"
	if _, stderr, code := runCLIForDiagnostics(t, "check", valid); code != 0 {
		t.Fatal(code, stderr)
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/no_panic_bodies_proof_invalid.sec")
	if code == 0 {
		t.Fatal("panicking bodies were accepted")
	}
	for _, want := range []string{
		"init Counter.init does not satisfy @noPanic: reachable may-panic-arithmetic",
		"free Counter.free does not satisfy @noPanic: reachable may-panic-assertion",
		"property get accessor Counter.Doubled does not satisfy @noPanic",
		"property set accessor Counter.Tripled does not satisfy @noPanic",
		"lambda at ",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("missing %q in:\n%s", want, stderr)
		}
	}
	if count := strings.Count(stderr, "does not satisfy @noPanic"); count != 5 {
		t.Errorf("want exactly 5 failures (unannotated accessors are not checked), got %d:\n%s", count, stderr)
	}
	source, err := os.ReadFile(valid)
	if err != nil {
		t.Fatal(err)
	}
	var formatted strings.Builder
	if err := formatStdinWithOptions(strings.NewReader(string(source)), &formatted, secformatter.Options{}); err != nil || formatted.String() != string(source) {
		t.Fatalf("formatter moved or dropped an attribute: %v\n%s", err, formatted.String())
	}
}
