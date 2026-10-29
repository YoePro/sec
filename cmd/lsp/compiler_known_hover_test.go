package main

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/sema"
)

// TestCompilerKnownFriendlyPresentation verifies the resolved public label in
// all shared renderers while retaining signatures, authority and unsafe facts.
// Rules: rules/compiler/compiler_known_members.md — "LSP", "User-facing symbol names".
func TestCompilerKnownFriendlyPresentation(t *testing.T) {
	integer := sema.Type{Name: "int", Kind: sema.IntType}
	str := sema.Type{Name: "string", Kind: sema.StringType}
	for _, test := range []struct {
		receiver    sema.Type
		name, label string
	}{
		{sema.Type{Name: "list", Kind: sema.StructType, TypeArgs: []sema.Type{integer}}, "Append", "list[int].Append"},
		{sema.Type{Name: "map", Kind: sema.StructType, TypeArgs: []sema.Type{str, integer}}, "ContainsKey", "map[string, int].ContainsKey"},
		{str, "Len", "string.Len"},
		{sema.Type{Name: "RawPtr", Kind: sema.RawPtrType, TypeArgs: []sema.Type{integer}}, "Read", "RawPtr[int].Read"},
	} {
		t.Run(test.label, func(t *testing.T) {
			var selected *sema.CompilerKnownMember
			for _, member := range sema.CompilerKnownMembersForType(test.receiver, false) {
				if member.Name == test.name {
					selected = &member
					break
				}
			}
			if selected == nil {
				t.Fatalf("no registry member %s", test.label)
			}
			member := *selected
			before := member
			hover := compilerKnownMemberHover(lspRange{}, member)
			item, ok := compilerKnownCompletionItem(member)
			if !ok || item.Label != test.name || item.Documentation != hover.Contents.Value {
				t.Fatalf("completion diverges from hover: %+v", item)
			}
			facts := member.DisplayFacts()
			if !strings.Contains(hover.Contents.Value, test.label) || !strings.Contains(hover.Contents.Value, facts.Signature) {
				t.Fatalf("lost resolved label/signature: %s", hover.Contents.Value)
			}
			if compilerKnownSignatureLabel(member) != facts.Signature {
				t.Fatal("signature label diverges from shared display facts")
			}
			assertNoCompilerKnownDisplayIDs(t, hover.Contents.Value, item.Label, item.Detail, item.Documentation, compilerKnownSignatureLabel(member))
			if member.Unsafe && !strings.Contains(item.Documentation, "Requires an `unsafe` context.") {
				t.Fatal("unsafe requirement lost")
			}
			if !reflect.DeepEqual(before, member) {
				t.Fatal("rendering changed registry metadata")
			}
		})
	}
}

// TestCompilerKnownFriendlyFallbacks covers unavailable names/signatures and
// transport-only secondary prose without hiding stable diagnostic codes.
// Rules: rules/compiler/compiler_known_members.md — "User-facing symbol names".
func TestCompilerKnownFriendlyFallbacks(t *testing.T) {
	for _, id := range []string{"CKM-UNKNOWN", "CKF-UNKNOWN", "CKV-UNKNOWN"} {
		member := sema.CompilerKnownMember{ID: id, Name: id, Kind: sema.CompilerKnownMethod,
			Signature: id, Documentation: "See " + id + "; diagnostic S1176.",
			Rule: id, TargetRestriction: "Requires " + id, Unsafe: true,
			CallerObligations: []string{"Prove lifetime; see " + id}}
		hover := compilerKnownMemberHover(lspRange{}, member)
		assertNoCompilerKnownDisplayIDs(t, hover.Contents.Value, compilerKnownSignatureLabel(member))
		for _, want := range []string{"Signature unavailable", "context-dependent", "S1176", "Requires an `unsafe` context.", "Prove lifetime", "Availability:"} {
			if !strings.Contains(hover.Contents.Value, want) {
				t.Errorf("missing %q in %s", want, hover.Contents.Value)
			}
		}
		if _, ok := compilerKnownCompletionItem(member); ok {
			t.Fatal("completion inserted an unavailable source name")
		}
	}
}

// TestCompilerKnownFriendlySourceUI exercises actual source resolution rather
// than fabricating a registry entry, including the completion metadata path.
// Rules: rules/tooling/lsp.md — "Hover", "Completion";
// rules/compiler/compiler_known_members.md — "User-facing symbol names".
func TestCompilerKnownFriendlySourceUI(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/compiler_known_display/members.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	const uri = "file:///tmp/friendly-display/members.sec"
	for _, test := range []struct{ use, name, label string }{
		{"values.RemoveAt", "RemoveAt", "list[int].RemoveAt"},
		{"text.Len", "Len", "string.Len"},
	} {
		offset := strings.Index(source, test.use) + len(test.use) - len(test.name)
		hover, found := hoverForSource(uri, source, offsetPosition(source, offset))
		if !found || !strings.Contains(hover.Contents.Value, test.label) {
			t.Fatalf("missing friendly source hover: %+v", hover)
		}
		foundItem := false
		for _, item := range completeSource(uri, source, offset) {
			if item.Label == test.name {
				foundItem = true
				assertNoCompilerKnownDisplayIDs(t, item.Label, item.Detail, item.Documentation)
				if !strings.Contains(item.Documentation, test.label) {
					t.Fatalf("missing resolved completion documentation: %+v", item)
				}
			}
		}
		if !foundItem {
			t.Fatalf("missing completion for %s", test.name)
		}
		assertNoCompilerKnownDisplayIDs(t, hover.Contents.Value)
	}
}

// assertNoCompilerKnownDisplayIDs checks ordinary UI strings only; transport
// URIs and synthetic document identity remain outside this renderer contract.
// Rules: rules/compiler/compiler_known_members.md — "User-facing symbol names".
func assertNoCompilerKnownDisplayIDs(t *testing.T, texts ...string) {
	t.Helper()
	for _, text := range texts {
		for _, prefix := range []string{"CKM-", "CKF-", "CKV-"} {
			if strings.Contains(text, prefix) {
				t.Fatalf("transport ID in ordinary UI: %s", text)
			}
		}
	}
}
