package main

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"sec/internal/sema"
)

// TestFriendlySyntheticNavigationIdentity separates public filenames from
// operation identity and receiver/context-specific document content.
// Rules: rules/compiler/compiler_known_members.md — "Synthetic definitions",
// "User-facing symbol names", "Stable member identity".
func TestFriendlySyntheticNavigationIdentity(t *testing.T) {
	base := sema.CompilerKnownMember{ID: "CKM-LIST-APPEND", Name: "Append", BoundReceiver: "list[Customer]",
		Signature: "fn Append(value: Customer) Result[void, CollectionError]"}
	variants := []sema.CompilerKnownMember{base, base, base, base}
	variants[1].BoundReceiver = "list[Invoice]"
	variants[1].Signature = "fn Append(value: Invoice) Result[void, CollectionError]"
	variants[2].ID = "CKM-OTHER-APPEND"
	variants[3].Documentation = "Different resolved call context."
	seen := map[string]bool{}
	for _, member := range variants {
		location := compilerKnownDefinitionLocation(member)
		if seen[location.URI] {
			t.Fatal("distinct metadata collided", location.URI)
		}
		seen[location.URI] = true
		parsed, err := url.Parse(location.URI)
		if err != nil {
			t.Fatal(err)
		}
		assertNoCompilerKnownDisplayIDs(t, parsed.Path)
		if !friendlyDefinitionURI(location.URI, member.ID, member.DisplayFacts().Label) {
			t.Fatal("lost public filename or transport identity", location.URI)
		}
		// Editors can normalize escaping and query order. Content lookup must
		// still find precisely the navigated receiver instance.
		parts := strings.Split(parsed.RawQuery, "&")
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		escaped := compilerKnownScheme + ":/" + url.PathEscape(strings.TrimPrefix(parsed.Path, "/")) + "?" + strings.Join(parts, "&")
		text, ok := compilerKnownDefinitionText(escaped)
		if !ok || text != sema.CompilerKnownSyntheticDefinition(member) {
			t.Fatal("editor normalization lost the instance", escaped)
		}
		assertNoCompilerKnownDisplayIDs(t, text)
		lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
		if lines[location.Range.Start.Line] != member.Signature {
			t.Fatal("navigation missed signature", location.Range)
		}
	}
}

// TestFriendlySyntheticColdRequests preserves legacy public-ID navigation and
// rejects missing instances, forged authority and malformed transport data.
// Rules: rules/compiler/compiler_known_members.md — "Synthetic definitions",
// "Private core UTC wall-clock intrinsic", "Internal core string materialization".
func TestFriendlySyntheticColdRequests(t *testing.T) {
	for _, uri := range []string{
		"sec-compiler-known:/Iterator.Next.sec?id=CKM-ITERATOR-NEXT",
		"sec-compiler-known:/CKM-ITERATOR-NEXT.sec",
	} {
		text, ok := compilerKnownDefinitionText(uri)
		if !ok || !strings.Contains(text, "fn Next() Option[T]") {
			t.Fatal("public registry recovery failed", uri)
		}
		assertNoCompilerKnownDisplayIDs(t, text)
	}
	for _, function := range sema.CompilerKnownFunctions() {
		if function.Internal {
			for _, uri := range []string{
				"sec-compiler-known:/" + function.ID + ".sec",
				"sec-compiler-known:/unvisited-private.sec?id=" + url.QueryEscape(function.ID),
			} {
				if _, ok := compilerKnownDefinitionText(uri); ok {
					t.Fatal("cold request granted core authority", function.Name)
				}
			}
		}
	}
	for _, uri := range []string{
		"sec-compiler-known:/unvisited-value.sec?id=CKV-TEMPORAL-NOW",
		"sec-compiler-known:/list[int].Append.sec?id=CKM-LIST-APPEND&instance=unknown",
		"sec-compiler-known:/Next.sec?id=CKM-ITERATOR-NEXT&id=CKV-TEMPORAL-NOW",
		"sec-compiler-known:/Next.sec?id=%ZZ",
		"sec-compiler-known://foreign/Next.sec?id=CKM-ITERATOR-NEXT",
		"sec-compiler-known:/Next.sec?id=CKM-ITERATOR-NEXT#fragment",
		"sec-compiler-known:/Next.sec?id=CKM-ITERATOR-NEXT&unknown=true",
	} {
		if _, ok := compilerKnownDefinitionText(uri); ok {
			t.Error("invalid/cold instance request succeeded", uri)
		}
	}
}

// TestFriendlyFenceDefinitionContexts preserves established order information
// without presenting a MemoryOrder argument as atomic.Fence's receiver.
// Rules: rules/concurrency/atomics.md — §§26,40;
// rules/compiler/compiler_known_members.md — "Synthetic definitions".
func TestFriendlyFenceDefinitionContexts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/compiler_known_display/fences.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	const uri = "file:///tmp/friendly-display/fences.sec"
	var previous string
	for _, order := range []string{"Acquire", "Release"} {
		offset := strings.Index(source, "Fence(MemoryOrder."+order)
		locations := definitionsForSource(uri, source, offsetPosition(source, offset))
		if len(locations) != 1 || !friendlyDefinitionURI(locations[0].URI, "CKF-ATOMIC-FENCE", "atomic.Fence") {
			t.Fatalf("lost function name: %+v", locations)
		}
		if locations[0].URI == previous {
			t.Fatal("resolved order contexts collided")
		}
		previous = locations[0].URI
		text, ok := compilerKnownDefinitionText(previous)
		if !ok || !strings.Contains(text, "This call establishes MemoryOrder."+order) {
			t.Fatal("lost resolved order", text)
		}
		assertNoCompilerKnownDisplayIDs(t, text)
	}
}
