package sema

import (
	"strings"
	"testing"
)

// MD-068 § 9(2): decimal and decimal128 expose the read-only value
// predicates IsInteger, IsZero, IsPositive, IsNegative and Sign, each with
// its own stable identity per type, and no setters.
// Rules: rules/library/core-library.md §§ 10.2, 10.4; rules/compiler/compiler_known_members.md — Decimal types.
func TestDecimalValuePredicateProperties(t *testing.T) {
	for _, test := range []struct {
		typ    Type
		prefix string
	}{
		{builtinType("decimal"), "CKM-DECIMAL-"},
		{builtinType("decimal128"), "CKM-DECIMAL128-"},
	} {
		members := map[string]CompilerKnownMember{}
		for _, member := range CompilerKnownMembersForType(test.typ, false) {
			members[member.Name] = member
		}
		for name, want := range map[string]string{"IsInteger": "bool", "IsZero": "bool", "IsPositive": "bool", "IsNegative": "bool", "Sign": "int"} {
			member, ok := members[name]
			if !ok {
				t.Fatalf("%s lacks %s", test.typ.Name, name)
			}
			if member.ID != test.prefix+strings.ToUpper(name) || member.Kind != CompilerKnownProperty || member.Result.Name != want {
				t.Fatalf("%s.%s = %+v", test.typ.Name, name, member)
			}
		}
	}

	errs := analyzeSourceRaw(t, `
module main

fn Props(value: decimal, wide: decimal128) int {
    if value.IsInteger && !value.IsZero && value.IsPositive && !wide.IsNegative {
        return value.Sign + wide.Sign
    }
    return 0
}

fn Write(value: decimal) void {
    let mut copy := value
    copy.IsZero = true
}
`)
	if len(errs) != 1 {
		t.Fatalf("want only the read-only assignment rejected, got %v", errs)
	}
}
