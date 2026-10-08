package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	lspserver "sec/internal/lsp/server"
	"sec/internal/parser"
	"sec/internal/sema"
)

// pitfallLSPFixture loads source under the compiler's shared testdata owner.
// Rules: rules/analysis/pitfall_analysis.md — "Required incremental and LSP tests".
func pitfallLSPFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/sema/pitfall_lsp_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestPitfallLSPConfiguration exercises protocol policy reload, optional
// severity and code-action withdrawal without losing normative errors.
// Rules: rules/analysis/pitfall_analysis.md — "LSP configuration reload", "LSP presentation";
// rules/tooling/diagnostics.md — §18; rules/tooling/lsp.md — "Configuration".
func TestPitfallLSPConfiguration(t *testing.T) {
	var out bytes.Buffer
	s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), diagnosticDelay: time.Hour}
	defer s.stopDiagnosticTimers()
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	source := pitfallLSPFixture(t)
	s.documentSnapshots.Open(uri, 7, source)
	if err := s.handle(rpcMessage{Method: "initialize", ID: json.RawMessage(`1`), Params: json.RawMessage(`{"initializationOptions":{"diagnostics":{"rules":{"A2004":"off"}}}}`)}); err != nil {
		t.Fatal(err)
	}
	if s.pitfallSettingsSnapshot().Rules["A2004"] != "off" {
		t.Fatal("initial policy ignored")
	}

	for _, level := range []string{"info", "warning", "error", "off"} {
		if err := s.handle(rpcMessage{Method: "workspace/didChangeConfiguration", Params: json.RawMessage(`{"settings":{"sec":{"diagnostics":{"rules":{"A2004":"` + level + `","S1134":"off"}}}}}`)}); err != nil {
			t.Fatal(err)
		}
		out.Reset()
		if err := s.publishModuleDiagnostics(uri); err != nil {
			t.Fatal(err)
		}
		var publication struct {
			Params struct {
				Version     int
				Diagnostics []diagnostic
			}
		}
		raw := out.String()[strings.Index(out.String(), "{"):]
		if err := json.Unmarshal([]byte(raw), &publication); err != nil {
			t.Fatal(err)
		}
		if publication.Params.Version != 7 {
			t.Fatal(publication)
		}
		found := 0
		for _, item := range publication.Params.Diagnostics {
			if item.Code != diagnostics.PitfallAdvisory {
				t.Fatalf("valid fixture has unexpected diagnostic: %+v", item)
			}
			found++
			if item.Severity != map[string]int{"info": 3, "warning": 2, "error": 1}[level] || !strings.Contains(item.Message, "pitfall.") || len(item.RelatedInformation) == 0 {
				t.Fatal(item)
			}
		}
		if (found == 0) != (level == "off") {
			t.Fatalf("%s: %+v", level, publication)
		}
		out.Reset()
		params, err := json.Marshal(codeActionParams{TextDocument: textDocumentIdentifier{URI: uri}, Range: lspRange{End: endPosition(source)}, Context: codeActionContext{Diagnostics: []diagnostic{{Code: "A2004", Message: "obsolete client finding"}}}})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.handle(rpcMessage{Method: "textDocument/codeAction", ID: json.RawMessage(`2`), Params: params}); err != nil {
			t.Fatal(err)
		}
		var response struct{ Result []codeAction }
		if err := json.Unmarshal([]byte(out.String()[strings.Index(out.String(), "{"):]), &response); err != nil {
			t.Fatal(err)
		}
		actions := response.Result
		if (len(actions) == 0) != (level == "off") {
			t.Fatalf("%s: %+v", level, actions)
		}
	}
	before := s.pitfallSettingsSnapshot()
	detached := s.pitfallSettingsSnapshot()
	detached.Rules["A2004"] = "error"
	if s.pitfallSettingsSnapshot().Rules["A2004"] != before.Rules["A2004"] {
		t.Fatal("snapshot aliases settings")
	}

	s.updatePitfallSettings(json.RawMessage(`{"diagnostics":{"rules":{"A2004":true,"suspicious.pitfall":"invalid"}}}`))
	if s.pitfallSettingsSnapshot().Rules["A2004"] != before.Rules["A2004"] {
		t.Fatal("invalid update changed policy")
	}
	source = parameterInsightFixture(t, "pitfall_diagnostic_ownership_invalid")
	s.documentSnapshots.Open(uri, 8, source)
	out.Reset()
	if err := s.publishModuleDiagnostics(uri); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"code":"S1134"`) || strings.Contains(out.String(), `"code":"A2004"`) {
		t.Fatal(out.String())
	}
	mandatory := pitfallCodeActions(uri, source, lspRange{End: endPosition(source)}, sourceOverlay{}, s.pitfallSettingsSnapshot())
	if len(mandatory) == 0 {
		t.Fatal("off removed mandatory guidance")
	}
	for _, action := range mandatory {
		if len(action.Edit.Changes) != 0 || action.Disabled == nil {
			t.Fatal("intent edit became automatic", action)
		}
	}
}

// TestPitfallLSPFixes applies certified edits, checks delimiter/UTF-16 ranges,
// and keeps intent suggestions visibly distinct and non-applicable.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety", "Corrective actions";
// rules/tooling/lsp.md — "Safe fixes", "Snapshots".
func TestPitfallLSPFixes(t *testing.T) {
	source := pitfallLSPFixture(t)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	actions := pitfallCodeActions(uri, source, lspRange{End: endPosition(source)}, sourceOverlay{}, pitfallDiagnosticSettings{})
	fixed, suggested, grouped, unicode := 0, 0, false, false
	for _, action := range actions {
		if action.Disabled != nil {
			suggested++
			if !strings.HasPrefix(action.Title, "Suggested edit:") || len(action.Edit.Changes) != 0 {
				t.Fatal(action)
			}
			continue
		}
		if !strings.HasPrefix(action.Title, "Proven fix:") {
			t.Fatal(action)
		}
		edits := action.Edit.Changes[uri]
		if len(edits) != 1 {
			t.Fatal(action)
		}
		edit := edits[0]
		start := lineCharToOffset(source, edit.Range.Start.Line, edit.Range.Start.Character)
		end := lineCharToOffset(source, edit.Range.End.Line, edit.Range.End.Character)
		original := source[start:end]
		grouped = grouped || original == "(value > 10) && (value < 0)"
		unicode = unicode || original == "värde𐐀 > 10 && värde𐐀 < 0"
		rewritten := source[:start] + edit.NewText + source[end:]
		parsed := parser.New(lexer.NewWithFile(rewritten, pathFromURI(uri))).Parse()
		if len(parsed.Diagnostics) != 0 || parsed.Fatal {
			t.Fatal(original, rewritten, parsed.Diagnostics)
		}
		if errs := sema.NewAnalyzer().Analyze(parsed.Program); len(errs) != 0 {
			t.Fatal(original, errs)
		}
		fixed++
	}
	if fixed != 6 || suggested == 0 || !grouped || !unicode {
		t.Fatalf("fixed=%d suggested=%d grouped=%v unicode=%v actions=%+v", fixed, suggested, grouped, unicode, actions)
	}
	// Current text supersedes stale client diagnostics, and requests outside
	// a finding range cannot receive an unrelated edit.
	changed := strings.ReplaceAll(source, "flag == true", "flag")
	direct := lspRange{Start: position{Line: 1}, End: position{Line: 1, Character: 100}}
	if values := pitfallCodeActions(uri, changed, direct, sourceOverlay{}, pitfallDiagnosticSettings{}); len(values) != 0 {
		t.Fatal(values)
	}
}

// TestPitfallLSPProjectReload covers project severity and analysis-depth edits,
// watched-file refresh, unaffected document routing and obsolete job rejection.
// Rules: rules/analysis/pitfall_analysis.md — "LSP configuration reload";
// rules/tooling/lsp.md — "Configuration", "Snapshots".
func TestPitfallLSPProjectReload(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, ".sec", "sec.toml")
	uri := uriFromPath(filepath.Join(dir, "main.sec"))
	otherURI := uriFromPath(filepath.Join(t.TempDir(), "other.sec"))
	source := pitfallLSPFixture(t)
	var out bytes.Buffer
	s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), diagnosticDelay: time.Hour}
	defer s.stopDiagnosticTimers()
	s.documentSnapshots.Open(uri, 1, source)
	s.documentSnapshots.Open(otherURI, 2, source)
	s.updatePitfallSettings(json.RawMessage(`{"diagnostics":{"rules":{"A2004":"off"}}}`))
	for _, depth := range []string{"interactive", "deep", "interactive"} {
		if err := os.WriteFile(manifest, []byte("[analysis]\nlsp_depth = \""+depth+"\"\n[diagnostics.rules]\n\"suspicious.pitfall\" = \"warning\"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := s.handle(rpcMessage{Method: "workspace/didChangeWatchedFiles", Params: json.RawMessage(`{"changes":[]}`)}); err != nil {
			t.Fatal(err)
		}
		values := analyzeDiagnosticBatchWithPolicy(s.documentSnapshots.Snapshots(), s.sourceOverlay(), s.pitfallSettingsSnapshot())
		hasWidth := false
		for _, value := range values[uri] {
			if value.Severity != 2 {
				t.Fatal(value)
			}
			hasWidth = hasWidth || strings.Contains(value.Message, string(sema.PitfallMeaninglessComparison))
		}
		if hasWidth != (depth == "deep") || len(values[uri]) == 0 || len(values[otherURI]) != 0 {
			t.Fatal(depth, values)
		}
		actions := pitfallCodeActions(uri, source, lspRange{End: endPosition(source)}, sourceOverlay{}, s.pitfallSettingsSnapshot())
		widthFix := false
		for _, action := range actions {
			for _, d := range action.Diagnostics {
				widthFix = widthFix || strings.Contains(d.Message, string(sema.PitfallMeaninglessComparison))
			}
		}
		if widthFix != (depth == "deep") {
			t.Fatal(depth, actions)
		}
	}
	// A configuration notification must immediately invalidate the old job,
	// even before its replacement's debounce delay has elapsed.
	key := normalizedSourcePath(dir)
	s.timerMu.Lock()
	old := s.invalidateDiagnosticJob(key)
	s.timerMu.Unlock()
	s.updatePitfallSettings(json.RawMessage(`{"diagnostics":{"rules":{"A2004":"info"}}}`))
	out.Reset()
	if err := s.publishDiagnosticBatch(key, old); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatal("obsolete policy published", out.String())
	}
}
