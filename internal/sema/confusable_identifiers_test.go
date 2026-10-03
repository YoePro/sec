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

// Confusable detection covers every conflicting declaration domain: the
// parameters of one callable (functions and lambdas), the fields of one
// struct, the variants of one enum or union, and the members of one type
// across fields, methods, properties, and events, including methods from an
// earlier impl block. The same spellings in unrelated domains stay legal.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 2.5–2.10
//   - rules/foundations/lexical_structure.md — "Visually confusable identifiers"
func TestConfusableIdentifiersInMemberParameterAndVariantDomains(t *testing.T) {
	const confusable = "аdmin" // Cyrillic a
	tests := []struct {
		name   string
		source string
	}{
		{name: "function parameters", source: "fn Use(admin: int, " + confusable + ": int) int {\n    return admin\n}\n"},
		{name: "lambda parameters", source: "fn Use() int {\n    let pick := fn(admin: int, " + confusable + ": int) int {\n        return admin\n    }\n    return pick(1, 2)\n}\n"},
		{name: "struct fields", source: "type Account struct {\n    admin: int,\n    " + confusable + ": int,\n}\n"},
		{name: "enum values", source: "enum Role {\n    admin,\n    " + confusable + ",\n}\n"},
		{name: "union variants", source: "type Role union {\n    admin\n    " + confusable + "\n}\n"},
		{name: "field and method", source: "type Account struct {\n    admin: int,\n}\n\nimpl Account {\n    fn " + confusable + "() int {\n        return 1\n    }\n}\n"},
		{name: "methods across impl blocks", source: "type Account struct {\n    id: int,\n}\n\nimpl Account {\n    fn admin() int {\n        return 1\n    }\n}\n\nimpl extends Account {\n    fn " + confusable + "() int {\n        return 2\n    }\n}\n"},
		{name: "impl event and method", source: "type Data struct {\n    value: int,\n}\n\ntype Button struct {\n    storage: EventStorage[Data, 8],\n}\n\nimpl Button {\n    fn admin() int {\n        return 1\n    }\n\n    event " + confusable + " using storage\n}\n"},
		{name: "interface methods", source: "interface Accountable {\n    fn admin() int\n    fn " + confusable + "() int\n}\n"},
		{name: "interface method and property", source: "interface Accountable {\n    fn admin() int\n    property " + confusable + ": int { get }\n}\n"},
		{name: "interface property and event", source: "interface Accountable {\n    property admin: int { get }\n    event " + confusable + "[int]\n}\n"},
		{name: "generic parameters", source: "fn Pick[Admin, \u0410dmin](left: Admin, right: \u0410dmin) int {\n    return 0\n}\n"},
		{name: "field and property", source: "type Account struct {\n    admin: int,\n}\n\nimpl Account {\n    property " + confusable + ": int {\n        get {\n            return 1\n        }\n    }\n}\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n\n"+test.source)
			count := 0
			for _, err := range errors {
				if err.ID == "S1099" {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("errors = %+v, want exactly one S1099", errors)
			}
		})
	}

	unrelated := analyzeSourceRaw(t, "module main\n\n"+
		"fn First(admin: int) int {\n    return admin\n}\n\n"+
		"fn Second("+confusable+": int) int {\n    return "+confusable+"\n}\n\n"+
		"type Left struct {\n    admin: int,\n}\n\n"+
		"type Right struct {\n    "+confusable+": int,\n}\n\n"+
		"enum Role {\n    admin,\n}\n\n"+
		"enum Kind {\n    "+confusable+",\n}\n")
	for _, err := range unrelated {
		if err.ID == "S1099" {
			t.Fatalf("unrelated domains compared: %+v", unrelated)
		}
	}
}
