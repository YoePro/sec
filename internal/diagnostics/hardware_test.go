package diagnostics

import "testing"

// Hardware failures stay mandatory; compatible region overlap is configurable
// and never masquerades as binding alias evidence (MD-111 §§10(6),13(2)).
func TestHardwareDiagnosticRegistryAndPolicy(t *testing.T) {
	for _, id := range []string{HardwareMemberType, HardwareMemberSchema, HardwareEnumCase, HardwareUnknownDomain, HardwareAddressExtent, HardwareSymbolResolution, HardwareRequiredCapability, HardwareRegionConflict, HardwareSchemaUnavailable, HardwareContextUnavailable} {
		definition, ok := Lookup(id)
		if !ok || !definition.Mandatory || definition.DefaultSeverity != SeverityError {
			t.Fatalf("hardware diagnostic %s: %+v", id, definition)
		}
		if severity, enabled := (RulePolicy{id: "off"}).Effective(id); !enabled || severity != SeverityError {
			t.Fatalf("disabled mandatory %s", id)
		}
	}
	definition, ok := Lookup(HardwareRegionOverlap)
	if !ok || definition.Mandatory || definition.DefaultSeverity != SeverityWarning {
		t.Fatalf("overlap diagnostic %+v", definition)
	}
	if _, enabled := (RulePolicy{HardwareRegionOverlap: "off"}).Effective(HardwareRegionOverlap); enabled {
		t.Fatal("overlap warning cannot be disabled")
	}
	if severity, enabled := (RulePolicy{HardwareRegionOverlap: "error"}).Effective(HardwareRegionOverlap); !enabled || severity != SeverityError {
		t.Fatal("overlap warning cannot be promoted")
	}
}
