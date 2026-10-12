package semantic

import (
	"errors"
	"strings"
	"testing"
)

// A try assignment is consumed through Sema's ResolvedTryAssignment and
// rejected with its resolved edge, error channel and fallible step until
// Semantic IR represents property setters; it is never lowered as a plain
// store or reported as an unknown statement.
//
// Rules:
//   - rules/errors/errorhandling.md — §23 "Fallible assignment", §35
func TestTryAssignmentConsumesResolvedFact(t *testing.T) {
	fixture := `module main

enum SetterError error { Rejected }

type Vehicle struct { speed: int }

impl Vehicle {
	property Speed: int {
		get { return speed }
		try set value SetterError { return Err(SetterError.Rejected) }
	}
}
`
	for name, test := range map[string]struct{ source, want string }{
		"propagation": {fixture + `
fn Set(vehicle: Vehicle) Result[void, SetterError] {
	try vehicle.Speed = 10
	return Ok()
}
`, "try assignment through fallible property setter Speed (propagating to Result[void, SetterError] error SetterError)"},
		"handled": {fixture + `
fn Set(vehicle: Vehicle) void {
	try vehicle.Speed = 10 {
		Err(SetterError.Rejected) => {}
	}
}
`, "try assignment through fallible property setter Speed (with local handlers error SetterError)"},
	} {
		module, err := analyzedModule(t, test.source, 14)
		var unsupported *UnsupportedFeatureError
		if module != nil || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, test.want) {
			t.Errorf("%s: module=%t err=%v, want unsupported %q", name, module != nil, err, test.want)
		}
	}
}
