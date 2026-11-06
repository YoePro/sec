package diagnostics

import "testing"

// Cleanup errors are mandatory diagnostics shared by the compiler and editor.
// Rules: rules/control-flow/defer.md — §31; rules/errors/panic.md — §28.
func TestInvalidDeferredCleanupDefinition(t *testing.T) {
	d, ok := Lookup(InvalidDeferredCleanup)
	if !ok || d.Name != "cleanup.invalid-deferred-control-flow" || d.Family != "cleanup" || !d.Mandatory || d.DefaultSeverity != SeverityError {
		t.Fatal(d)
	}
}
