package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestIfConformanceConditionsAndAssignment covers all non-boolean families
// and the true/false/terminated continuation assignment merge. Union empty
// state is used so default initialization of scalar int cannot mask a missing
// assignment. Branch locals must not escape their lexical scope.
// Rules: rules/control-flow/flowcontrol_if.md — §§3,17–20,27–28;
// rules/declarations/unions.md — §7.3, empty local initialization state.
func TestIfConformanceConditionsAndAssignment(t *testing.T) {
	for _, name := range []string{"truthiness_invalid", "assignment", "assignment_invalid", "comparison_invalid"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/control_flow/if_conformance/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			errors := analyzeSourceRaw(t, string(source))
			switch name {
			case "truthiness_invalid":
				if len(errors) != 9 {
					t.Fatal(errors)
				}
				for i, e := range errors {
					if e.ID != diagnostics.IfConditionNotBool || e.Severity != diagnostics.SeverityError || e.Help == "" || e.Line != 10+i || e.EndColumn <= e.Column {
						t.Fatalf("missing registered condition occurrence: %+v", e)
					}
				}
			case "assignment":
				if len(errors) != 0 {
					t.Fatal(errors)
				}
			case "assignment_invalid":
				if len(errors) != 2 || !strings.Contains(errors[0].Message, "variable state is unassigned") || errors[0].Line != 12 || !strings.Contains(errors[1].Message, "branchOnly") || errors[1].Line != 19 {
					t.Fatalf("want missing false-path assignment and inaccessible branch local: %+v", errors)
				}
			case "comparison_invalid":
				if len(errors) != 1 || !strings.Contains(errors[0].Message, "comparison chaining is not supported") || errors[0].Line != 4 {
					t.Fatalf("want focused comparison-chain rejection: %+v", errors)
				}
			}
		})
	}
}
