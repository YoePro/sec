package diagnostics

import "testing"

// Rule-specific policy promotes or disables a configurable diagnostic by
// stable ID or symbolic name (ID first), while a mandatory diagnostic can be
// neither disabled nor demoted.
//
// Rules:
//   - rules/tooling/diagnostics.md — §§ 4(7), 18(1)-(4)
func TestRulePolicyResolvesConfigurableDiagnostics(t *testing.T) {
	policy := ParseRulePolicy(`
[project]
name = "x"

[diagnostics.rules]
"A2005" = "error"   # promote explicit reborrow information
"ownership.explicit-reborrow" = "off"
"suspicious.pitfall" = warning
"S1153" = "off"
"A2001" = "loud"
`)
	if severity, enabled := policy.Effective(ExplicitReborrow); !enabled || severity != SeverityError {
		t.Fatalf("A2005 = %q %v, want error (ID beats name)", severity, enabled)
	}
	if severity, enabled := policy.Effective(PitfallAdvisory); !enabled || severity != SeverityWarning {
		t.Fatalf("A2004 = %q %v, want warning", severity, enabled)
	}
	if severity, enabled := policy.Effective(DurationConversionInvalid); !enabled || severity != SeverityError {
		t.Fatalf("mandatory S1153 = %q %v, must stay an error", severity, enabled)
	}
	if severity, enabled := policy.Effective(LargeValueParameter); !enabled || severity != DefaultSeverity(LargeValueParameter) {
		t.Fatalf("unknown state must keep the default, got %q %v", severity, enabled)
	}
	if _, enabled := (RulePolicy{"suspicious.pitfall": "off"}).Effective(PitfallAdvisory); enabled {
		t.Fatal("off must disable a configurable diagnostic")
	}
}
