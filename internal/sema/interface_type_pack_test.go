package sema

import "testing"

// MD-117 on interface owners: a type pack is a method-level generic
// parameter, and an interface requirement may not introduce one because
// interface dispatch must stay finite and Sec has no runtime generic or pack
// dispatch (interfaces.md § 9; functions.md § 38A(1), (9)(2)). The ordinary
// rule, not the S1177 unsupported-feature gate, rejects it, for shared,
// exclusive and owned packs alike.
func TestInterfaceRequirementRejectsTypePack(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

interface Tracer {
	fn Trace[...Types](values: ...ref Types) void
	mut fn Update[...Types](values: ...ref mut Types) void
	fn Consume[...Types](values: ...Types) void
}
`)
	assertSemaErrors(t, errors, []string{
		"interface method Trace cannot declare method-level generic parameters; use interface generic parameters instead at 5:14",
		"interface method Update cannot declare method-level generic parameters; use interface generic parameters instead at 6:19",
		"interface method Consume cannot declare method-level generic parameters; use interface generic parameters instead at 7:16",
	})
}
