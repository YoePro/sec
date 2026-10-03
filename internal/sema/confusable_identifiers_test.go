package sema

import (
	"strings"
	"testing"
)

// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 2.5–2.10, 8.10
//   - rules/foundations/lexical_structure.md — "Visually confusable identifiers"
func TestConfusableIdentifiersInOneDeclarationDomain(t *testing.T) {
	locals := analyzeSourceRaw(t, "module main\n\n"+
		"fn First() int {\n    let admin := 1\n    let \u0430dmin := 2\n    return admin\n}\n\n"+
		"fn Second() int {\n    let \u0430dmin := 2\n    return \u0430dmin\n}\n\n"+
		"fn Third() int {\n    let admin := 1\n    return admin\n}\n")
	if len(locals) != 1 || locals[0].ID != "S1099" || locals[0].Line != 5 || locals[0].PreviousLine != 4 ||
		!strings.Contains(locals[0].Message, "UTS #39 confusable forms collide") {
		t.Fatalf("local errors = %+v, want one S1099 at line 5 naming line 4; unrelated functions stay legal", locals)
	}
	module := analyzeSourceRaw(t, "module main\n\nfn total() int {\n    return 1\n}\n\nfn totaI() int {\n    return 2\n}\n")
	if len(module) != 1 || module[0].ID != "S1099" || module[0].Line != 7 || module[0].PreviousLine != 3 {
		t.Fatalf("module errors = %+v, want one S1099 at line 7 naming line 3", module)
	}
}

func TestConfusableCheckSkipsUnitSymbolNamespace(t *testing.T) {
	errors := analyzeSourceRaw(t, "module main\n\nunit μs float physical\n\nfn Use() int {\n    let µs := 3\n    return µs\n}\n")
	for _, err := range errors {
		if err.ID == "S1099" {
			t.Fatalf("unit symbol compared with an ordinary identifier: %+v", errors)
		}
	}
}
