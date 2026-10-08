// Package temporal owns pure temporal semantic classifications. Analyzer owns
// resolved identity, source provenance, effects and diagnostics.
package temporal

// IsWallClockProperty classifies the three canonical UTC wall-clock properties
// using resolved intrinsic type identity. A user property named Now or Today
// cannot acquire clock semantics from its spelling.
// Rules: rules/types/temporal.md — §3 "UTC wall-clock access";
// rules/corrections/applied/temporal-now-correction-20260928.md — §§2.1,7.
func IsWallClockProperty(intrinsic bool, owner, property string) bool {
	if !intrinsic {
		return false
	}
	return (owner == "datetime" || owner == "time") && property == "Now" || owner == "date" && property == "Today"
}
