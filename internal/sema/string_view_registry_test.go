package sema

import (
	"strings"
	"testing"
)

// The complete MD-126 registry is read-only and keeps fallible access,
// panic-capable access and explicit text materialization distinct.
// Rules: rules/compiler/compiler_known_members.md §21; rules/library/core-library.md §5.8.
func TestStringViewRegistryAndRemovedSlice(t *testing.T) {
	if _, errors := stringRangeFixture(t, "registry_valid"); len(errors) > 0 {
		t.Fatal(errors)
	}
	members := CompilerKnownMembersForType(builtinType("StringView"), false)
	want := map[string]CompilerKnownMemberKind{"Len": CompilerKnownProperty, "SizeOf": CompilerKnownProperty, "IsEmpty": CompilerKnownMethod, "GetRune": CompilerKnownMethod, "RuneAt": CompilerKnownMethod, "GetByte": CompilerKnownMethod, "ByteAt": CompilerKnownMethod, "CharAt": CompilerKnownMethod, "StartsWith": CompilerKnownMethod, "ToString": CompilerKnownMethod}
	if len(members) != len(want) {
		t.Fatal(members)
	}
	ids := map[string]bool{}
	for _, member := range members {
		kind, exists := want[member.Name]
		if !exists || kind != member.Kind || ids[member.ID] || member.ID == "" {
			t.Fatal(member)
		}
		ids[member.ID] = true
		if member.Name != "ToString" && member.AllocationBehavior != AllocationFree {
			t.Fatal(member)
		}
		if member.Name == "GetRune" || member.Name == "GetByte" {
			if member.Result.Kind != ResultType || member.Result.TypeArgs[1].Name != "IndexError" {
				t.Fatal(member)
			}
		}
	}
	for _, owner := range []Type{builtinType("string"), builtinType("StringView")} {
		for _, member := range CompilerKnownMembersForType(owner, false) {
			if member.Name == "Slice" {
				t.Fatal("retired member", member)
			}
		}
	}
	_, errors := stringRangeFixture(t, "slice_removed_invalid")
	if len(errors) != 5 {
		t.Fatal(errors)
	}
	for _, err := range errors {
		if err.ID == "" || !strings.Contains(err.Message, "string.Slice has been removed") || !strings.Contains(err.Help, "try text[a..<b]") || !strings.Contains(err.Help, "borrowed StringView") {
			t.Fatal(err)
		}
	}
	a, errors := stringRangeFixture(t, "valid")
	if len(errors) > 0 {
		t.Fatal(errors)
	}
	for expression := range a.stringRanges {
		plan, ok := a.ResolvedStringRangeOf(expression)
		if !ok || plan.OperationID != CompilerKnownStringRangeID {
			t.Fatal(plan)
		}
	}
}
