package semantic

import (
	"strings"
	"testing"
)

// A bodyless try assignment through a `try set` property setter lowers in
// Semantic IR: the setter call yields Result[void, E], the success edge
// continues, and the failure edge propagates the error through the enclosing
// Result after running the function's cleanup (owned locals and defer bodies,
// exit=error-propagation); a handled form branches into its local handlers.
// Rules: rules/errors/errorhandling.md — §§ 15, 23;
// rules/compiler/semantic_ir.md — § 26(3).
func TestTryAssignmentThroughSetterCarriesCleanup(t *testing.T) {
	source := "module main\n\nenum SetterError error { Rejected, Busy }\n\ntype Vehicle struct { speed: int }\n\nimpl Vehicle {\n    property Speed: int {\n        get { return speed }\n        try set value SetterError { return Err(SetterError.Rejected) }\n    }\n}\n\nfn G() void {\n}\n\nfn Drive(vehicle: ref mut Vehicle) Result[uint, SetterError] {\n    let mut values: int[]\n    defer {\n        G()\n    }\n    try vehicle.Speed = 10\n    return Ok(values.Len)\n}\n\nfn Handled(vehicle: ref mut Vehicle) int {\n    try vehicle.Speed = 20 {\n        Err(error) => {\n            discard error\n            return 1\n        }\n    }\n    return 0\n}\n\nfn main() int {\n    return 0\n}\n"
	module, err := hostedModule(t, source, 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	drive := functionText(text, "main::Drive")
	for _, want := range []string{"Vehicle.Speed.try-set", "result.is-err", "cleanup.defer-register", "cleanup.track-owned", "result.err", "cleanup.run-function exit=error-propagation", "cleanup.run-function exit=return"} {
		if !strings.Contains(drive, want) {
			t.Fatalf("Drive lacks %q:\n%s", want, drive)
		}
	}
	if propagate := strings.Index(drive, "exit=error-propagation"); strings.LastIndex(drive[:propagate], "result.err") < 0 || !strings.Contains(drive[propagate:], "return") {
		t.Fatalf("the failure edge must build Err, run cleanup and return:\n%s", drive)
	}
	handled := functionText(text, "main::Handled")
	if !strings.Contains(handled, "Vehicle.Speed.try-set") || strings.Count(handled, "return") < 2 || strings.Contains(handled, "exit=error-propagation") {
		t.Fatalf("Handled must enter its local handler instead of propagating:\n%s", handled)
	}
}

// functionText is the formatted text of one function, up to the next one.
func functionText(text, prefix string) string {
	start := strings.Index(text, `func "`+prefix)
	if start < 0 {
		return ""
	}
	rest := text[start:]
	if next := strings.Index(rest[1:], `func "`); next >= 0 {
		return rest[:next+1]
	}
	return rest
}
