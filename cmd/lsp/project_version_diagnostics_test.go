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

// Rules: rules/projects/projects.md — §48; rules/tooling/lsp.md — shared diagnostics and snapshots.
func TestLSPProjectVersionPublicationAndClearing(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".sec", "sec.toml")
	bad := strings.Replace(string(fixture), "build = 123", "build = -1", 1)
	if err := os.WriteFile(path, []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/projects/init/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "cmd", "sec", "main.sec")
	uri := uriFromPath(sourcePath)
	manifestURI := uriFromPath(path)
	var out bytes.Buffer
	s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), diagnosticDelay: time.Hour}
	defer s.stopDiagnosticTimers()
	s.documentSnapshots.Open(uri, 1, string(source))
	if err := s.publishModuleDiagnostics(uri); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), manifestURI) || !strings.Contains(out.String(), "S1150") {
		t.Fatal(out.String())
	}
	// Manifest diagnostics are published on the manifest, never the source URI.
	items := analyze(manifestURI, bad)
	if len(items) != 1 || items[0].Code != "S1150" || items[0].Range.Start.Line != 10 || items[0].Range.Start.Character != 0 {
		t.Fatal(items)
	}
	// An unsaved correction supersedes disk and clears the discovered error.
	out.Reset()
	s.documentSnapshots.Open(manifestURI, 1, string(fixture))
	if err := s.publishModuleDiagnostics(uri); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), manifestURI) || strings.Contains(out.String(), "S1150") {
		t.Fatal(out.String())
	}
	out.Reset()
	if err := s.publishModuleDiagnostics(manifestURI); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "P100") || strings.Contains(out.String(), "S1150") {
		t.Fatal("TOML parsed as Sec", out.String())
	}
	// Closing the correction restores the invalid disk snapshot. Deleting that
	// manifest then clears its retained error on the next source refresh.
	s.documentSnapshots.Close(manifestURI)
	if err := s.publishModuleDiagnostics(uri); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := s.publishModuleDiagnostics(uri); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), manifestURI) || strings.Contains(out.String(), "S1150") {
		t.Fatal("deleted manifest retained diagnostics", out.String())
	}
}
