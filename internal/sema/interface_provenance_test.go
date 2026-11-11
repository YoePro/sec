package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// A missing or incompatible inherited requirement names the interface that
// declares it, and conformance facts record each implemented member with
// its declaring interface and receiver capabilities, own requirements first.
// Rules: rules/declarations/interfaces.md — §§ 3, 5, 6.
func TestInterfaceInheritedRequirementProvenance(t *testing.T) {
	source := `module main

interface Counter {
    fn Value() int
    mut fn Increment() void
}

interface Resettable implements Counter {
    mut fn Reset() void
}

type Partial struct {
    count: int,
}

impl Partial implements Resettable {
    fn Value() int {
        return self.count
    }
    fn Reset() void {
        self.count = 0
    }
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, source)
	if len(errors) != 1 || errors[0].ID != diagnostics.InterfaceMemberMissing || !strings.HasPrefix(errors[0].Help, "Resettable inherits Increment from Counter.") {
		t.Fatalf("missing inherited member = %v", errors)
	}
	facts := analyzer.InterfaceConformances()
	if len(facts) != 1 || len(facts[0].Members) != 2 {
		t.Fatalf("conformance facts = %+v", facts)
	}
	reset, value := facts[0].Members[0], facts[0].Members[1]
	if value.Name != "Value" || value.DeclaredIn != "Counter" || value.RequirementMutable || value.Kind != "method" {
		t.Errorf("Value member = %+v", value)
	}
	if reset.Name != "Reset" || reset.DeclaredIn != "Resettable" || !reset.RequirementMutable || !reset.ImplementationMutable {
		t.Errorf("Reset member = %+v", reset)
	}
}
