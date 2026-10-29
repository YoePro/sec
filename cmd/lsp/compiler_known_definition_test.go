package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/sema"
)

// Receiver-specific documents must not grow process memory without a bound;
// eviction requires fresh navigation rather than an incorrect generic answer.
// Rules: rules/tooling/lsp.md — Snapshots;
// rules/compiler/compiler_known_members.md — "Synthetic definitions".
func TestSyntheticDefinitionCacheIsBounded(t *testing.T) {
	first := ""
	for index := 0; index <= 512; index++ {
		member := sema.CompilerKnownMember{ID: "CKM-LIST-APPEND", Name: "Append", BoundReceiver: fmt.Sprintf("list[Item%d]", index), Signature: fmt.Sprintf("fn Append(value: Item%d) Result[void, CollectionError]", index)}
		uri := compilerKnownDefinitionLocation(member).URI
		if index == 0 {
			first = uri
		}
	}
	syntheticDefinitionMembers.Lock()
	count := len(syntheticDefinitionMembers.entries)
	syntheticDefinitionMembers.Unlock()
	if count > 512 {
		t.Fatalf("cache entries = %d", count)
	}
	if _, ok := compilerKnownDefinitionText(first); ok {
		t.Fatal("oldest receiver instance was not evicted")
	}
}

// Two instantiations share a semantic operation ID but must retain different
// navigable documents, including after an intervening navigation request.
// Rules: rules/compiler/compiler_known_members.md — "Synthetic definitions",
// "Stable member identity", "Documentation generation".
func TestSyntheticDefinitionsKeepReceiverInstancesSeparate(t *testing.T) {
	list := sema.Type{Name: "list", Kind: sema.StructType}
	var locations []location
	var expected []string
	for _, element := range []sema.Type{{Name: "int", Kind: sema.IntType}, {Name: "string", Kind: sema.StringType}} {
		list.TypeArgs = []sema.Type{element}
		for _, member := range sema.CompilerKnownMembersForType(list, false) {
			if member.Name == "Append" {
				locations = append(locations, compilerKnownDefinitionLocation(member))
				expected = append(expected, sema.CompilerKnownSyntheticDefinition(member))
				if !strings.Contains(sema.CompilerKnownDocumentationForType(list, false), expected[len(expected)-1]) {
					t.Fatal("documentation and navigation disagree")
				}
			}
		}
	}
	if len(locations) != 2 || locations[0].URI == locations[1].URI {
		t.Fatal("receiver instances collided", locations)
	}
	for index, location := range locations {
		text, ok := compilerKnownDefinitionText(location.URI)
		if !ok || text != expected[index] {
			t.Fatal("later navigation replaced earlier definition", location.URI, text)
		}
	}
	if _, ok := compilerKnownDefinitionText("sec-compiler-known:/CKM-LIST-APPEND.sec?instance=unknown"); ok {
		t.Fatal("unresolved instance silently used generic metadata")
	}
}

func TestDefinitionOfCompilerKnownMemberNavigatesToSyntheticDefinition(t *testing.T) {
	source := "module main\n\nfn Fill(values: ref mut list[int]) Result[void, CollectionError] {\n    try values.Append(1)\n    discard values.RemoveAt(0)\n    return Ok()\n}\n"
	offset := strings.Index(source, "Append")
	pos := offsetPosition(source, offset)
	locations := definitionsForSource("file:///tmp/synthetic.sec", source, pos)
	if len(locations) != 1 || !friendlyDefinitionURI(locations[0].URI, "CKM-LIST-APPEND", "ref mut list[int].Append") {
		t.Fatalf("locations = %#v", locations)
	}
	text, ok := compilerKnownDefinitionText(locations[0].URI)
	if !ok {
		t.Fatal("no synthetic text")
	}
	for _, want := range []string{
		"Synthetic read-only definition",
		"not source code",
		"Member:     ref mut list[int].Append",
		"Rule:       rules/collections/collections.md — 13.5 Core list methods",
		"Receiver:   list[int] (mutable)",
		"Effects:    none",
		"Unsafe:     no",
		"Target:     none; available on every target plan",
		"Structural: changes the receiver's element count or storage",
		"fn Append(value: int) Result[void, CollectionError]",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("synthetic text lacks %q:\n%s", want, text)
		}
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if got := lines[locations[0].Range.Start.Line]; got != "fn Append(value: int) Result[void, CollectionError]" {
		t.Fatalf("location line = %q", got)
	}
}

func TestSyntheticDefinitionTextRendersFromIDAlone(t *testing.T) {
	text, ok := compilerKnownDefinitionText("sec-compiler-known:/CKM-DYNAMIC-ARRAY-REMOVEAT.sec")
	if !ok || !strings.Contains(text, "fn RemoveAt(index: uint) Option[T]") || !strings.Contains(text, "6.9 `RemoveAt`") {
		t.Fatalf("text = %q", text)
	}
	if _, ok := compilerKnownDefinitionText("file:///tmp/x.sec"); ok {
		t.Fatal("ordinary URIs are not synthetic definitions")
	}
}

func TestCompilerKnownMemberHoverShowsRegistrySignature(t *testing.T) {
	source := "module main\n\nfn Drop(values: ref mut list[int]) void {\n    discard values.RemoveAt(0)\n}\n"
	offset := strings.Index(source, "RemoveAt")
	hover, ok := hoverForSource("file:///tmp/hover.sec", source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "fn RemoveAt(index: uint) Option[int]") {
		t.Fatalf("hover = %#v", hover)
	}
}

// Rules: rules/declarations/interfaces.md §9.1;
// rules/compiler/compiler_known_members.md — Built-in type member lookup, LSP.
func TestCompilerInterfaceRequirementUsesSharedIdentity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/interface_catalog/iterator.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///tmp/interface-registry.sec"
	offset := strings.Index(source, "value.Next") + len("value.")
	pos := offsetPosition(source, offset)
	hover, ok := hoverForSource(uri, source, pos)
	if !ok || !strings.Contains(hover.Contents.Value, "Iterator[int].Next") || !strings.Contains(hover.Contents.Value, "Option[int]") {
		t.Fatalf("hover = %#v, %v", hover, ok)
	}
	locations := definitionsForSource(uri, source, pos)
	if len(locations) != 1 || !friendlyDefinitionURI(locations[0].URI, "CKM-ITERATOR-NEXT", "Iterator[int].Next") {
		t.Fatalf("locations = %#v", locations)
	}
	definition, ok := compilerKnownDefinitionText(locations[0].URI)
	if !ok || !strings.Contains(definition, "fn Next() Option[int]") || !strings.Contains(definition, "Iterator[int] (mutable)") {
		t.Fatalf("definition = %q", definition)
	}
	if _, err := prepareRenameForSource(uri, source, pos, sourceOverlay{}); err == nil {
		t.Fatal("compiler requirement was renamable")
	}
	items := completeSource(uri, source, offset)
	assertCompletionLabels(t, items, []string{"Next"})
	for _, item := range items {
		if item.Label == "Next" && item.Detail != "Option[int]" {
			t.Fatalf("completion: %#v", item)
		}
	}
	position := offsetPosition(source, offset)
	tokens := decodeSemanticTokens(semanticTokensForSource(uri, source))
	assertSemanticToken(t, tokens, position.Line, position.Character, len("Next"), "method")
	// The concrete implementation is a source declaration and keeps navigation
	// and rename semantics, even though it fulfills the compiler requirement.
	offset = strings.Index(source, "fn Next") + len("fn ")
	locations = definitionsForSource(uri, source, offsetPosition(source, offset))
	if len(locations) != 1 || locations[0].URI != uri {
		t.Fatalf("concrete definition = %#v", locations)
	}
	if _, err := prepareRenameForSource(uri, source, offsetPosition(source, offset), sourceOverlay{}); err != nil {
		t.Fatalf("ordinary implementation: %v", err)
	}
}

// Rules: rules/types/temporal.md §3; compiler_known_members.md — private _now, LSP.
func TestTemporalIntrinsicUsesSharedToolingFacts(t *testing.T) {
	root := t.TempDir()
	core := filepath.Join(root, "sec", "core")
	if err := os.MkdirAll(core, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sec", "stdlib"), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/sema/temporal/read_twice.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	path := filepath.Join(core, "read.sec")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(path)
	pos := offsetPosition(source, strings.Index(source, "_now"))
	hover, ok := hoverForSource(uri, source, pos)
	for _, want := range []string{"_now", "datetime", "may-use-nondeterministic-input"} {
		if !ok || !strings.Contains(hover.Contents.Value, want) {
			t.Fatalf("hover missing %s: %#v", want, hover)
		}
	}
	locations := definitionsForSource(uri, source, pos)
	if len(locations) != 1 || !friendlyDefinitionURI(locations[0].URI, "CKV-TEMPORAL-NOW", "_now") {
		t.Fatalf("locations = %#v", locations)
	}
	definition, ok := compilerKnownDefinitionText(locations[0].URI)
	if !ok || !strings.Contains(definition, "UTCWallClock must be Supported and Enabled") || !strings.Contains(definition, "loader-proven core") {
		t.Fatalf("definition = %q", definition)
	}
	if _, err := prepareRenameForSource(uri, source, pos, sourceOverlay{}); err == nil {
		t.Fatal("_now was renamable")
	}
	tokens := decodeSemanticTokens(semanticTokensForSource(uri, source))
	assertSemanticTokenWithModifier(t, tokens, pos.Line, pos.Character, len("_now"), "variable", "readonly")
	// Spelling and a core-looking module grant no intrinsic identity.
	userURI := uriFromPath(filepath.Join(root, "untrusted.sec"))
	hover, ok = hoverForSource(userURI, source, pos)
	if ok && strings.Contains(hover.Contents.Value, "Compiler-known value `_now`") {
		t.Fatalf("untrusted hover: %#v", hover)
	}
	if locations := definitionsForSource(userURI, source, pos); len(locations) != 0 {
		t.Fatalf("untrusted definition: %#v", locations)
	}
	assertNoCompletionLabel(t, completeSource(uri, source, strings.Index(source, "_now")+len("_n")), "_now")
}

// Rules: rules/compiler/compiler_known_members.md — Synthetic definitions.
func TestIntrinsicDefinitionRecoveredFromRegistry(t *testing.T) {
	for id, want := range map[string]string{
		"CKM-ITERATOR-NEXT": "fn Next() Option[T]",
	} {
		syntheticDefinitionMembers.Lock()
		delete(syntheticDefinitionMembers.entries, "sec-compiler-known:/"+id+".sec")
		syntheticDefinitionMembers.Unlock()
		text, ok := compilerKnownDefinitionText("sec-compiler-known:/" + id + ".sec")
		if !ok || !strings.Contains(text, want) {
			t.Fatalf("%s: %q", id, text)
		}
	}
}

// Rules: rules/compiler/compiler_known_members.md — Private core UTC wall-clock intrinsic.
func TestCoreOnlyDefinitionRequiresAuthorizedNavigation(t *testing.T) {
	syntheticDefinitionMembers.Lock()
	delete(syntheticDefinitionMembers.entries, "sec-compiler-known:/CKV-TEMPORAL-NOW.sec")
	syntheticDefinitionMembers.Unlock()
	if text, ok := compilerKnownDefinitionText("sec-compiler-known:/CKV-TEMPORAL-NOW.sec"); ok || text != "" {
		t.Fatalf("internal ID exposed without source authority: %q", text)
	}
}

// friendlyDefinitionURI checks the editor-visible basename independently from
// internal transport identity. URL decoding is the editor's path presentation.
// Rules: rules/compiler/compiler_known_members.md — "Synthetic definitions".
func friendlyDefinitionURI(uri, id, label string) bool {
	parsed, err := url.Parse(uri)
	return err == nil && parsed.Scheme == compilerKnownScheme && parsed.Path == "/"+label+".sec" && parsed.Query().Get("id") == id
}
