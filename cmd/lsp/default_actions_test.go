package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/formatter"
	"sec/internal/lexer"
	lspserver "sec/internal/lsp/server"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestDefaultCodeActions publishes only independently valid source edits while
// retaining default values, field evaluation order and written comments.
// Rules: rules/types/default_values.md — "LSP", "Formatter", "Struct spread and defaults".
func TestDefaultCodeActions(t *testing.T) {
	data, err := os.ReadFile("../../testdata/defaults/code_actions.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	baseline := parser.New(lexer.New(source))
	program := baseline.ParseProgram()
	if len(baseline.Errors()) != 0 {
		t.Fatal(baseline.Errors())
	}
	if errors := sema.NewAnalyzer().Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, tc := range []struct{ needle, title, want string }{
		{"type Port", "Declare explicit default for Port", " default 1"},
		{"type Role", "Declare explicit default for Role", ` default "..."`},
		{"let mut unicode", "Insert explicit default for unicode", " := 0"},
		{"Settings { port: PortValue() }", "Expand defaulted fields", `, label: "...", flags: [false, false], `},
		{"let mut number", "Insert explicit default for number", " := 0"},
		{"let mut port", "Insert explicit default for port", " := 1"},
		{"let mut text", "Insert explicit default for text", ` := "..."`},
		{"let mut flags", "Insert explicit default for flags", " := [false, false]"},
		{"let mut dynamic", "Insert explicit default for dynamic", " := []"},
		{"let mut listValue", "Insert explicit default for listValue", " := list[int] {}"},
		{"let mut namedList", "Insert explicit default for namedList", " := Empty(list[int] {})"},
		{"let mut bounded", "Insert explicit default for bounded", " := list[int, 4] {}"},
		{"let mut boxed", "Insert explicit default for boxed", " := Box[int] {}"},
		{"let mut state", "Insert explicit default for state", " := State.SECOND"},
		{"let mut settings", "Insert explicit default for settings", " := Settings {}"},
		{"Settings {}", "Expand defaulted fields", ` port: 1, label: "...", flags: [false, false], `},
		{"Settings { port: 12 }", "Expand defaulted fields", `, label: "...", flags: [false, false], `},
		{"Settings {\n", "Expand defaulted fields", ` label: "...", flags: [false, false], `},
		{"Settings { port: 12, }", "Expand defaulted fields", ` label: "...", flags: [false, false], `},
	} {
		t.Run(tc.needle, func(t *testing.T) {
			at := strings.Index(source, tc.needle)
			if at < 0 {
				t.Fatal("missing selection")
			}
			position := offsetPosition(source, at)
			actions := defaultCodeActions("", source, lspRange{Start: position, End: position}, sourceOverlay{})
			if len(actions) != 1 || actions[0].Title != tc.title || actions[0].Kind != "refactor.rewrite" {
				t.Fatalf("actions = %+v", actions)
			}
			edits := actions[0].Edit.Changes[""]
			if len(edits) != 1 || edits[0].NewText != tc.want {
				t.Fatalf("edits = %+v, want %q", edits, tc.want)
			}
			edited := applyTextEdits(source, edits)
			p := parser.New(lexer.New(edited))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			if errors := sema.NewAnalyzer().Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			if !strings.Contains(edited, "// keep this comment") || !strings.Contains(edited, "// preserve field comment") {
				t.Fatal("comment lost")
			}
		})
	}
	for _, needle := range []string{"let mut huge", "let completeSettings", "let explicit", "let spreadSettings"} {
		position := offsetPosition(source, strings.Index(source, needle))
		if actions := defaultCodeActions("", source, lspRange{Start: position, End: position}, sourceOverlay{}); len(actions) != 0 {
			t.Fatalf("%s actions = %+v", needle, actions)
		}
	}
	formatted := formatter.Format(formatter.Source{Text: source}, formatter.Options{}).Text
	if strings.Contains(formatted, "let mut number: int :=") || strings.Contains(formatted, "type Port int range 1..65535 default") {
		t.Fatal("ordinary formatting expanded defaults")
	}
}

// TestDefaultCodeActionsCRLF pins UTF-16 edits following a supplementary Unicode
// character and rejects diagnostic-bearing programs and unrelated selections.
// Rules: rules/types/default_values.md — "LSP";
// rules/tooling/lsp.md — "Snapshots", "Safe fixes".
func TestDefaultCodeActionsCRLF(t *testing.T) {
	data, err := os.ReadFile("../../testdata/defaults/code_actions.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ReplaceAll(string(data), "\n", "\r\n")
	at := strings.Index(source, "let mut unicode")
	pos := offsetPosition(source, at)
	actions := defaultCodeActions("", source, lspRange{Start: pos, End: pos}, sourceOverlay{})
	if len(actions) != 1 {
		t.Fatalf("actions = %+v", actions)
	}
	edited := applyTextEdits(source, actions[0].Edit.Changes[""])
	if !strings.Contains(edited, "/* 😀 */ let mut unicode: int := 0\r\n") {
		t.Fatal("edit damaged Unicode or CRLF")
	}
	if actions := defaultCodeActions("", source, lspRange{}, sourceOverlay{}); len(actions) != 0 {
		t.Fatalf("unrelated actions = %+v", actions)
	}
	invalid, err := os.ReadFile("../../testdata/defaults/default_structs_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	all := lspRange{End: offsetPosition(string(invalid), len(invalid))}
	if actions := defaultCodeActions("", string(invalid), all, sourceOverlay{}); len(actions) != 0 {
		t.Fatalf("invalid actions = %+v", actions)
	}
}

// TestDefaultCodeActionsProtocol uses the open-document module overlay instead
// of stale on-disk type contracts when publishing a real LSP workspace edit.
// Rules: rules/types/default_values.md — "LSP", "Defaults and contracts";
// rules/tooling/lsp.md — "Snapshots", "Target-aware analysis".
func TestDefaultCodeActionsProtocol(t *testing.T) {
	data, err := os.ReadFile("../../testdata/defaults/code_actions.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	split := strings.Index(source, "fn PortValue")
	declarations, body := source[:split], "module main\n"+source[split:]
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "stdlib"), 0755); err != nil {
		t.Fatal(err)
	}
	declarationsPath := filepath.Join(root, "types.sec")
	if err := os.WriteFile(declarationsPath, []byte(declarations), 0644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(root, "main.sec")
	if err := os.WriteFile(mainPath, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(mainPath)
	var output bytes.Buffer
	server := &server{out: &output, documentSnapshots: lspserver.NewDocuments()}
	server.documentSnapshots.Open(uri, 3, body)
	server.documentSnapshots.Open(uriFromPath(declarationsPath), 4, strings.Replace(declarations, "range 1..65535", "range 5..65535", 1))
	position := offsetPosition(body, strings.Index(body, "let mut port"))
	params, err := json.Marshal(codeActionParams{TextDocument: textDocumentIdentifier{URI: uri}, Range: lspRange{Start: position, End: position}})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.handle(rpcMessage{Method: "textDocument/codeAction", ID: json.RawMessage("1"), Params: params}); err != nil {
		t.Fatal(err)
	}
	var response struct{ Result []codeAction }
	if err := json.Unmarshal([]byte(output.String()[strings.Index(output.String(), "{"):]), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Result) != 1 || response.Result[0].Title != "Insert explicit default for port" {
		t.Fatalf("response = %+v", response)
	}
	edits := response.Result[0].Edit.Changes[uri]
	if len(edits) != 1 || edits[0].NewText != " := 5" {
		t.Fatalf("ignored unsaved contract: %+v", edits)
	}
}
