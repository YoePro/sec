package sema

import (
	"strings"
	"testing"
)

// TestCompilerKnownSyntheticFriendlyDisplay protects all generated text,
// including fallback and secondary metadata, without exposing transport IDs.
// Rules: rules/compiler/compiler_known_members.md — "User-facing symbol names",
// "Synthetic definitions".
func TestCompilerKnownSyntheticFriendlyDisplay(t *testing.T) {
	list := builtinTypes()["list"]
	list.TypeArgs = []Type{builtinTypes()["int"]}
	members := CompilerKnownMembersForType(list, false)
	members = append(members, AtomicFenceMember(), CompilerKnownMember{
		ID: "CKV-UNKNOWN", Name: "CKV-UNKNOWN", Kind: CompilerKnownValueExpression,
		Documentation: "See CKM-UNKNOWN; diagnostic S1176 remains stable.",
	})
	for _, member := range members {
		text := CompilerKnownSyntheticDefinition(member)
		for _, prefix := range []string{"CKM-", "CKF-", "CKV-"} {
			if strings.Contains(text, prefix) {
				t.Fatalf("transport ID in generated text: %s", text)
			}
		}
		for _, want := range []string{"Synthetic read-only definition", "not source code", member.DisplayFacts().Label} {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q in generated definition", want)
			}
		}
		if member.ID == "CKV-UNKNOWN" && (!strings.Contains(text, "Signature unavailable") || !strings.Contains(text, "S1176")) {
			t.Fatal("fallback invented a signature or changed diagnostic identity", text)
		}
	}
}
