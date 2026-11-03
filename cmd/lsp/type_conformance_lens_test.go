package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lspserver "sec/internal/lsp/server"
	"sec/internal/sema"
)

// Type lenses share the compiler-owned conformance report of interface lenses,
// preserve requirement identity and resolve lazily against current target files.
// Rules: rules/tooling/lsp.md — "Interface conformance code lens";
// rules/declarations/interfaces.md — §§4–6,12.
func TestTypeConformanceLens(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"common.sec", "linux.sec", "windows_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/lsp/type_conformance_lens/" + name)
		if err != nil {
			t.Fatal(err)
		}
		writeWorkspaceSource(t, root, name, string(data))
	}
	path := filepath.Join(root, "common.sec")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	uri, text := uriFromPath(path), string(data)
	s := &server{documentSnapshots: lspserver.NewDocuments()}
	s.documentSnapshots.Open(uri, 1, text)
	interfaceConformanceCache.Lock()
	interfaceConformanceCache.key = "type-lens-not-analyzed"
	interfaceConformanceCache.summary = nil
	interfaceConformanceCache.Unlock()
	lenses := lspTestDecode[[]codeLens](t, lspTestRequest(t, s, "textDocument/codeLens", codeLensParams{TextDocument: textDocumentIdentifier{URI: uri}}))
	if len(lenses) != 4 {
		t.Fatalf("candidates=%v", lenses)
	}
	interfaceConformanceCache.Lock()
	untouched := interfaceConformanceCache.key == "type-lens-not-analyzed"
	interfaceConformanceCache.Unlock()
	if !untouched {
		t.Fatal("listing type candidates ran analysis")
	}
	var handle, other codeLens
	for _, lens := range lenses {
		if lens.Data.Type == "Handle" {
			handle = lens
		}
		if lens.Data.Type == "Other" {
			other = lens
		}
	}
	if handle.Data == nil || other.Data == nil {
		t.Fatal(lenses)
	}
	want := offsetPosition(text, strings.Index(text, "type Handle")+len("type "))
	if handle.Range.Start != want {
		t.Fatalf("type lens range=%v want=%v", handle.Range, want)
	}
	handle.Command = &lspCommand{Title: "forged", Command: "forged"}
	resolved := lspTestDecode[codeLens](t, lspTestRequest(t, s, "codeLens/resolve", handle))
	if resolved.Command == nil || !strings.Contains(resolved.Command.Title, "Handle: ✗ conforms on 1 of 2 targets") || !strings.Contains(resolved.Command.Title, "windows-amd64: Native.Size") {
		t.Fatalf("type report=%+v", resolved.Command)
	}
	failures := resolved.Command.Arguments[2].([]any)
	if len(failures) != 1 || !strings.HasSuffix(failures[0].(map[string]any)["uri"].(string), "windows_invalid.sec") {
		t.Fatal(failures)
	}
	for _, kind := range []string{"both", "wrong-kind"} {
		forged := handle
		data := *handle.Data
		forged.Data = &data
		if kind == "both" {
			data.Interface = "Native"
		} else {
			data.Interface = data.Type
			data.Type = ""
		}
		response := lspTestRequest(t, s, "codeLens/resolve", forged)
		if response.Error == nil || response.Error.Code != -32602 {
			t.Fatalf("forged subject accepted: %v", response)
		}
	}
	unused := lspTestDecode[codeLens](t, lspTestRequest(t, s, "codeLens/resolve", other))
	if unused.Command != nil {
		t.Fatalf("nonimplementer acquired conformance: %+v", unused)
	}
	windows := filepath.Join(root, "windows_invalid.sec")
	contents, err := os.ReadFile(windows)
	if err != nil {
		t.Fatal(err)
	}
	fixed := strings.Replace(string(contents), "Size(extra: int)", "Size()", 1)
	if err := os.WriteFile(windows, []byte(fixed), 0644); err != nil {
		t.Fatal(err)
	}
	updated := lspTestDecode[codeLens](t, lspTestRequest(t, s, "codeLens/resolve", handle))
	if updated.Command == nil || updated.Command.Title != "✓ Handle (Native, Writable) conforms on 2 targets" {
		t.Fatalf("updated=%+v", updated.Command)
	}
	// One successful requirement must not mask a missing explicit conformance.
	missing := strings.Replace(fixed, "implements Native, Writable", "implements Writable", 1)
	if err := os.WriteFile(windows, []byte(missing), 0644); err != nil {
		t.Fatal(err)
	}
	incomplete := lspTestDecode[codeLens](t, lspTestRequest(t, s, "codeLens/resolve", handle))
	if incomplete.Command == nil || !strings.Contains(incomplete.Command.Title, "Native (no declared conformance)") || strings.HasPrefix(incomplete.Command.Title, "✓") {
		t.Fatalf("missing conformance reported successful: %+v", incomplete.Command)
	}
	for _, summary := range crossTargetInterfaceConformance(uri, text, s.sourceOverlay()) {
		if summary.typeSubject {
			t.Fatal("type leaked into interface-only projection")
		}
	}
}

// Equal type spellings in another module cannot contribute conformance to the
// active type. Interface views still require the exact declaration token.
// Rules: rules/declarations/interfaces.md — §§4–6;
// rules/foundations/names_scopes_visibility.md — §3.
func TestTypeConformanceLensModuleIdentity(t *testing.T) {
	subject := conformanceSubject{name: "Handle", isType: true}
	fact := sema.InterfaceConformanceFact{TypeName: "Handle", TypeModule: "foreign"}
	if subject.matches(fact, "storage") {
		t.Fatal("foreign same-name conformance accepted")
	}
	fact.TypeModule = "storage"
	if !subject.matches(fact, "storage") {
		t.Fatal("own canonical conformance missing")
	}
	fact.TypeName = "HandleExtra"
	if subject.matches(fact, "storage") {
		t.Fatal("prefix type-name collision accepted")
	}
}

// Parameterized-interface, enum and named-scalar conformance uses the same compiler facts as
// struct conformance; source declaration categories do not infer membership.
// Rules: rules/declarations/interfaces.md — §§4–6,9;
// rules/tooling/lsp.md — "Interface conformance code lens".
func TestTypeConformanceLensParameterizedInterfaceEnumAndNamed(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/type_conformance_lens/generic_enum.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	path := filepath.Join(t.TempDir(), "types.sec")
	uri := uriFromPath(path)
	overlay := sourceOverlay{Sources: map[string]string{path: text}}
	seen := map[string]string{}
	for _, summary := range crossTargetConformance(uri, text, overlay) {
		if summary.typeSubject {
			seen[summary.name] = interfaceConformanceCommand(uri, summary).Title
		}
	}
	for _, name := range []string{"IntSink", "Marker", "Count"} {
		if !strings.HasPrefix(seen[name], "✓ "+name+" (") {
			t.Fatalf("%s conformance=%v", name, seen)
		}
	}
	if !strings.Contains(seen["IntSink"], "Sink[int]") {
		t.Fatal(seen)
	}
}
