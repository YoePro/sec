package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lspserver "sec/internal/lsp/server"
)

// TestProductionTestTargetLSP publishes on the manifest source key, including
// source-only discovery and an unsaved correction that clears the stale error.
// Rules: rules/tooling/testing.md — §§8.1,26.5;
// rules/projects/projects.md — §48; rules/tooling/lsp.md — snapshots.
func TestProductionTestTargetLSP(t *testing.T) {
	root := t.TempDir()
	data, err := os.ReadFile("../../testdata/projects/production_test_target/manifest.toml")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".sec", "sec.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(path)
	items := analyze(uri, string(data))
	if len(items) != 1 || items[0].Code != "S1145" || items[0].Range.Start.Line != 13 || items[0].Range.Start.Character != 0 {
		t.Fatal(items)
	}
	var out bytes.Buffer
	s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), diagnosticDelay: time.Hour}
	defer s.stopDiagnosticTimers()
	source, err := os.ReadFile("../../testdata/projects/production_test_target/tests/product/product.sec")
	if err != nil {
		t.Fatal(err)
	}
	sourceURI := uriFromPath(filepath.Join(root, "tests", "product", "product.sec"))
	s.documentSnapshots.Open(sourceURI, 1, string(source))
	if err := s.publishModuleDiagnostics(sourceURI); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), uri) || !strings.Contains(out.String(), "S1145") {
		t.Fatal(out.String())
	}
	corrected := strings.Replace(string(data), `kind = "command"`, `kind = "test"`, 1)
	out.Reset()
	s.documentSnapshots.Open(uri, 2, corrected)
	if err := s.publishModuleDiagnostics(sourceURI); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), uri) || strings.Contains(out.String(), "S1145") {
		t.Fatal(out.String())
	}
	if issues := analyze(uri, corrected); len(issues) != 0 {
		t.Fatal(issues)
	}
}
