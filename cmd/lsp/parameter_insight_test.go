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
	lspserver "sec/internal/lsp/server"
)

func parameterInsightFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/sema/" + name + ".sec")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Rules: rules/analysis/parameter_usage_analysis.md — "LSP presentation";
// rules/tooling/lsp.md — "Configuration".
func TestParameterInsightHoverUsesResolvedIdentity(t *testing.T) {
	source := parameterInsightFixture(t, "parameter_insight_valid")
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	checks := []struct{ marker, want, absent string }{
		{"Read(frame", "read access, call-only lifetime, borrow-sufficient", "unused by"},
		{"Unused(frame", "unused by this function", "read access"},
		{"frame.a", "read access, call-only lifetime, borrow-sufficient", "unused by"},
		{"ForwardRead(frame", "read access, call-only lifetime, borrow-sufficient", "unused by"},
	}
	for _, check := range checks {
		offset := strings.Index(source, check.marker)
		if offset < 0 {
			t.Fatal(check.marker)
		}
		if strings.Contains(check.marker, "(frame") {
			offset += strings.Index(check.marker, "frame")
		}
		pos := offsetPosition(source, offset+1)
		plain, _ := hoverForSource(uri, source, pos)
		if strings.Contains(plain.Contents.Value, "Parameter `") {
			t.Fatal("optional demand in default hover")
		}
		concise, ok := hoverForSourceWithParameterInsight(uri, source, pos, parameterInsightSettings{Hover: "concise"})
		if !ok || !strings.Contains(concise.Contents.Value, check.want) || strings.Contains(concise.Contents.Value, check.absent) {
			t.Fatalf("%s: %+v", check.marker, concise)
		}
		if strings.Contains(concise.Contents.Value, "Candidate") || strings.Contains(concise.Contents.Value, "storage=") {
			t.Fatal("concise hover dumps lattice")
		}
	}
	detailed, ok := hoverForSourceWithParameterInsight(uri, source, offsetPosition(source, strings.Index(source, "Read(frame")+1), parameterInsightSettings{Hover: "detailed"})
	for _, want := range []string{"precision=exact", "storage=", "Candidate `ref Frame`", "recommended", "strong", "AvoidCopyCost"} {
		if !ok || !strings.Contains(detailed.Contents.Value, want) {
			t.Fatalf("missing %s: %+v", want, detailed)
		}
	}
	blocked, _ := hoverForSourceWithParameterInsight(uri, source, offsetPosition(source, strings.Index(source, "ForwardSink(frame")+1), parameterInsightSettings{Hover: "detailed"})
	if !strings.Contains(blocked.Contents.Value, "blocked") || !strings.Contains(blocked.Contents.Value, "consumption-required") {
		t.Fatalf("missing blockers: %+v", blocked)
	}
}

// Rules: rules/tooling/lsp.md — "Safety and advisory diagnostics", "Configuration".
func TestParameterInsightConfigurationAndPublication(t *testing.T) {
	var out bytes.Buffer
	s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), diagnosticDelay: time.Hour}
	defer s.stopDiagnosticTimers()
	source := parameterInsightFixture(t, "parameter_insight_valid")
	dir := t.TempDir()
	uri := uriFromPath(filepath.Join(dir, "main.sec"))
	s.documentSnapshots.Open(uri, 1, source)
	initialize := json.RawMessage(`{"initializationOptions":{"analysis":{"parameters":{"hover":"concise","advisories":"info"}}}}`)
	if err := s.handle(rpcMessage{Method: "initialize", ID: json.RawMessage(`1`), Params: initialize}); err != nil {
		t.Fatal(err)
	}
	if got := s.parameterInsightSettings(); got.Hover != "concise" || got.Advisories != "info" {
		t.Fatal(got)
	}
	for _, level := range []string{"info", "off", "warning", "error"} {
		out.Reset()
		payload := json.RawMessage(`{"settings":{"sec":{"analysis":{"parameters":{"advisories":"` + level + `"}}}}}`)
		if err := s.handle(rpcMessage{Method: "workspace/didChangeConfiguration", Params: payload}); err != nil {
			t.Fatal(err)
		}
		if s.parameterInsightSettings().Hover != "concise" {
			t.Fatal("partial update lost hover choice")
		}
		if err := s.publishModuleDiagnostics(uri); err != nil {
			t.Fatal(err)
		}
		data := out.String()
		present := strings.Contains(data, `"code":"A2001"`)
		if present != (level != "off") {
			t.Fatalf("%s: %s", level, data)
		}
		if level != "off" {
			severity := map[string]int{"info": 3, "warning": 2, "error": 1}[level]
			var published struct {
				Params struct{ Diagnostics []diagnostic }
			}
			raw := data[strings.Index(data, "{"):]
			if err := json.Unmarshal([]byte(raw), &published); err != nil {
				t.Fatal(err)
			}
			for _, item := range published.Params.Diagnostics {
				if item.Code == diagnostics.LargeValueParameter && item.Severity != severity {
					t.Fatal(item)
				}
			}
		}
	}
	for _, mode := range []string{"detailed", "off", "concise"} {
		payload := json.RawMessage(`{"settings":{"sec":{"analysis":{"parameters":{"hover":"` + mode + `"}}}}}`)
		if err := s.handle(rpcMessage{Method: "workspace/didChangeConfiguration", Params: payload}); err != nil {
			t.Fatal(err)
		}
		out.Reset()
		params, err := json.Marshal(hoverParams{TextDocument: textDocumentIdentifier{URI: uri}, Position: offsetPosition(source, strings.Index(source, "Read(frame")+1)})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.handle(rpcMessage{Method: "textDocument/hover", ID: json.RawMessage(`2`), Params: params}); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "Parameter `frame`") != (mode != "off") {
			t.Fatalf("%s hover: %s", mode, out.String())
		}
		if strings.Contains(out.String(), "Candidate `ref Frame`") != (mode == "detailed") {
			t.Fatalf("%s details: %s", mode, out.String())
		}
	}
	before := s.parameterInsightSettings()
	s.updateParameterInsight(json.RawMessage(`{"analysis":{"parameters":{"hover":"invalid","advisories":"invalid"}}}`))
	if s.parameterInsightSettings() != before {
		t.Fatal("invalid settings replaced valid choices")
	}
	s.documentSnapshots.Open(uri, 2, parameterInsightFixture(t, "parameter_insight_invalid"))
	s.updateParameterInsight(json.RawMessage(`{"analysis":{"parameters":{"advisories":"off"}}}`))
	out.Reset()
	if err := s.publishModuleDiagnostics(uri); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"severity":1`) || !strings.Contains(out.String(), "function Wrong must return int, got bool") {
		t.Fatal("mandatory error hidden", out.String())
	}
}
