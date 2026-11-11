package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lspserver "sec/internal/lsp/server"
)

// Range/on-type use current snapshots and project indentation rather than
// client preferences. Their applied edits match the shared canonical layout
// within the selected syntax unit, leaving every other byte untouched.
// Rules: rules/tooling/formatter.md §§4(22),5,9,12,25,28,29;
// rules/tooling/lsp.md Appendix A.8.
func TestSelectionFormattingProtocol(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/range_selection.sec")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("../../testdata/formatter/range_selection.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{2, 4, 6} {
		for _, method := range []string{"textDocument/rangeFormatting", "textDocument/onTypeFormatting"} {
			for _, newline := range []string{"\n", "\r\n"} {
				text := strings.ReplaceAll(string(data), "\n", newline)
				root := t.TempDir()
				if err := os.MkdirAll(filepath.Join(root, ".sec"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte(fmt.Sprintf("[format]\nindentation_width = %d\n", width)), 0600); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, "main.sec")
				// A differing disk version must never win over the live snapshot.
				if err := os.WriteFile(path, []byte("stale disk source"), 0600); err != nil {
					t.Fatal(err)
				}
				uri := uriFromPath(path)
				s := &server{documentSnapshots: lspserver.NewDocuments()}
				s.documentSnapshots.Open(uri, 1, text)
				params := map[string]any{"textDocument": map[string]any{"uri": uri}, "options": map[string]any{"tabSize": 9, "insertSpaces": false}}
				start := strings.Index(text, "return 1+2")
				if method == "textDocument/rangeFormatting" {
					params["range"] = lspRange{Start: offsetPosition(text, start), End: offsetPosition(text, start+len("return 1+2"))}
				} else {
					params["position"] = offsetPosition(text, start+len("return 1+2")+len(newline)+1)
					params["ch"] = "}"
				}
				edits := lspTestDecode[[]textEdit](t, lspTestRequest(t, s, method, params))
				if len(edits) != 1 {
					t.Fatal(method, width, newline, edits)
				}
				edit := edits[0]
				from := lineCharToOffset(text, edit.Range.Start.Line, edit.Range.Start.Character)
				to := lineCharToOffset(text, edit.Range.End.Line, edit.Range.End.Character)
				got := text[:from] + edit.NewText + text[to:]
				unitStart := strings.Index(text, "/** Focus")
				unitEnd := strings.Index(text, newline+newline+"fn After") + len(newline)
				canonical := string(golden)
				unit := canonical[strings.Index(canonical, "/** Focus") : strings.Index(canonical, "\n\nfn After")+1]
				unit = strings.ReplaceAll(unit, "\n    ", "\n"+strings.Repeat(" ", width))
				want := text[:unitStart] + unit + text[unitEnd:]
				if got != want {
					t.Fatalf("%s width %d:\n%q\nwant:\n%q", method, width, got, want)
				}
				s.documentSnapshots.Open(uri, 2, got)
				at := strings.Index(got, "return 1 + 2")
				params["range"] = lspRange{Start: offsetPosition(got, at), End: offsetPosition(got, at+1)}
				if again := lspTestDecode[[]textEdit](t, lspTestRequest(t, s, "textDocument/rangeFormatting", params)); len(again) != 0 {
					t.Fatal("second pass", again)
				}
				if disk, err := os.ReadFile(path); err != nil || string(disk) != "stale disk source" {
					t.Fatal("request wrote source", err)
				}
			}
		}
	}
}

// Protocol validation never clamps forged/out-of-range coordinates, splits a
// surrogate pair, or formats incomplete source on a trigger. Both local modes
// receive formatting priority without overtaking a document synchronization.
// Rules: rules/tooling/formatter.md §§25,28,29; rules/tooling/lsp.md Appendix A.8, snapshots.
func TestSelectionFormattingSafety(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/range_selection.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	s := &server{documentSnapshots: lspserver.NewDocuments()}
	s.documentSnapshots.Open(uri, 1, text)
	for _, pos := range []position{{Line: -1}, {Line: 999}, {Line: 1, Character: 999}, {Line: 6, Character: 15}} {
		// Line 6's column 15 is inside the emoji's UTF-16 surrogate pair.
		params := map[string]any{"textDocument": map[string]any{"uri": uri}, "range": lspRange{Start: pos, End: pos}}
		result := lspTestRequest(t, s, "textDocument/rangeFormatting", params)
		if result.Error == nil || result.Error.Code != -32602 {
			t.Fatal("invalid range admitted", pos, result)
		}
	}
	for _, method := range []string{"textDocument/rangeFormatting", "textDocument/onTypeFormatting"} {
		result := lspTestRequest(t, s, method, map[string]any{"textDocument": map[string]any{"uri": uri}})
		if result.Error == nil || result.Error.Code != -32602 {
			t.Fatal("missing position admitted", result)
		}
		items := []rpcMessage{queuedMessage("textDocument/hover", 1), queuedMessage(method, 2)}
		if nextRequestIndex(items) != 1 {
			t.Fatal("formatting not prioritized", method)
		}
		items = append(items[:1], queuedMessage("textDocument/didChange", 0), queuedMessage(method, 2))
		if nextRequestIndex(items) != 0 {
			t.Fatal("formatting overtook synchronization", method)
		}
	}
	start := strings.Index(text, "return 1+2")
	params := map[string]any{"textDocument": map[string]any{"uri": uri}, "position": offsetPosition(text, start+len("return 1+2")+1), "ch": "\n"}
	if edits := lspTestDecode[[]textEdit](t, lspTestRequest(t, s, "textDocument/onTypeFormatting", params)); len(edits) != 1 {
		t.Fatal("newline trigger", edits)
	}
	params["ch"] = ";"
	if edits := lspTestDecode[[]textEdit](t, lspTestRequest(t, s, "textDocument/onTypeFormatting", params)); len(edits) != 0 {
		t.Fatal("unsupported trigger", edits)
	}
	s.documentSnapshots.Open(uri, 2, text+"\nfn Incomplete() void {")
	params["ch"] = "\n"
	if edits := lspTestDecode[[]textEdit](t, lspTestRequest(t, s, "textDocument/onTypeFormatting", params)); len(edits) != 0 {
		t.Fatal("incomplete document edited", edits)
	}
}

// The advertised capabilities match the routed protocol methods and triggers.
// Rules: rules/tooling/lsp.md Appendix A.8.
func TestSelectionFormattingCapabilities(t *testing.T) {
	s := &server{}
	result := lspTestDecode[map[string]any](t, lspTestRequest(t, s, "initialize", map[string]any{}))
	caps := result["capabilities"].(map[string]any)
	if caps["documentRangeFormattingProvider"] != true {
		t.Fatal(caps)
	}
	onType, ok := caps["documentOnTypeFormattingProvider"].(map[string]any)
	if !ok || onType["firstTriggerCharacter"] != "}" || fmt.Sprint(onType["moreTriggerCharacter"]) != "[\n]" {
		t.Fatal(onType)
	}
}

// Literal/comment triggers cannot format nearby code; a real range edit after
// a non-BMP literal uses UTF-16 columns and never splits the scalar.
// Rules: rules/tooling/formatter.md §§5,12,25,29;
// rules/tooling/lsp.md Appendix A.8, protocol positions.
func TestSelectionFormattingOpaqueAndUTF16(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/range_opaque.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	s := &server{documentSnapshots: lspserver.NewDocuments()}
	s.documentSnapshots.Open(uri, 1, text)
	for _, at := range []int{strings.Index(text, "\"}\"") + 2, strings.Index(text, "// }") + 4} {
		params := map[string]any{"textDocument": map[string]any{"uri": uri}, "position": offsetPosition(text, at), "ch": "}"}
		if edits := lspTestDecode[[]textEdit](t, lspTestRequest(t, s, "textDocument/onTypeFormatting", params)); len(edits) != 0 {
			t.Fatal("opaque trigger formatted source", edits)
		}
	}
	at := strings.Index(text, "+\"x\"")
	params := map[string]any{"textDocument": map[string]any{"uri": uri}, "range": lspRange{Start: offsetPosition(text, at), End: offsetPosition(text, at+1)}}
	edits := lspTestDecode[[]textEdit](t, lspTestRequest(t, s, "textDocument/rangeFormatting", params))
	if len(edits) != 1 {
		t.Fatal(edits)
	}
	edit := edits[0]
	start := lineCharToOffset(text, edit.Range.Start.Line, edit.Range.Start.Character)
	end := lineCharToOffset(text, edit.Range.End.Line, edit.Range.End.Character)
	got := text[:start] + edit.NewText + text[end:]
	if !strings.Contains(got, "\"😀\" + \"x\"") || !strings.Contains(got, "\nreturn 1 // }\n") || offsetPosition(text, at).Character != 31 {
		t.Fatal("UTF-16 edit/context", edit, got, offsetPosition(text, at))
	}
}

// Range formatting shares adaptive-width resolution and disk fallback with
// document formatting. Invalid viewport/configuration input yields no edits.
// Rules: rules/tooling/formatter.md §§4(9–23),28;
// rules/tooling/lsp.md Appendix A.8.
func TestSelectionFormattingConfiguration(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/preferred_width.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/preferred_width.narrow.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".sec"), 0700); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, ".sec", "sec.toml")
	if err := os.WriteFile(manifest, []byte("[format]\nwidth_mode = \"adaptive\"\nadaptive_min_width = 64\nadaptive_max_width = 120\nadaptive_fallback_width = 120\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "main.sec")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(path)
	s := &server{documentSnapshots: lspserver.NewDocuments()}
	params := map[string]any{"textDocument": map[string]any{"uri": uri}, "range": lspRange{End: offsetPosition(string(data), len(data))}, "options": map[string]any{"secPresentationWidth": uint64(64)}}
	edits := lspTestDecode[[]textEdit](t, lspTestRequest(t, s, "textDocument/rangeFormatting", params))
	if len(edits) != 1 {
		t.Fatal(edits)
	}
	e := edits[0]
	start := lineCharToOffset(string(data), e.Range.Start.Line, e.Range.Start.Character)
	end := lineCharToOffset(string(data), e.Range.End.Line, e.Range.End.Character)
	if got := string(data[:start]) + e.NewText + string(data[end:]); got != string(want) {
		t.Fatal(got)
	}
	params["options"] = map[string]any{"secPresentationWidth": -1}
	if response := lspTestRequest(t, s, "textDocument/rangeFormatting", params); response.Error == nil || response.Error.Code != -32602 {
		t.Fatal(response)
	}
	delete(params, "options")
	if err := os.WriteFile(manifest, []byte("[format]\nindentation_width = 3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if response := lspTestRequest(t, s, "textDocument/rangeFormatting", params); response.Error == nil {
		t.Fatal("invalid project formatted source", response)
	}
}
