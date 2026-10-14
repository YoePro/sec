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

// Rules: rules/projects/projects.md — §§4,48; rules/tooling/lsp.md — Snapshots.
func TestLSPProjectBoundaryOverlayPublicationAndClearing(t *testing.T) {
	root := t.TempDir()
	text, err := os.ReadFile("../../testdata/projects/boundary/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/projects/boundary/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	outer := filepath.Join(root, ".sec", "sec.toml")
	if err := os.MkdirAll(filepath.Dir(outer), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outer, text, 0600); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "unsaved", ".sec", "sec.toml")
	uri := uriFromPath(filepath.Join(root, "cmd", "sec", "main.sec"))
	childURI := uriFromPath(child)
	var out bytes.Buffer
	s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), diagnosticDelay: time.Hour}
	defer s.stopDiagnosticTimers()
	s.documentSnapshots.Open(uri, 1, string(source))
	s.documentSnapshots.Open(childURI, 1, string(text))
	if err := s.publishModuleDiagnostics(uri); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), childURI) || !strings.Contains(out.String(), "S1154") {
		t.Fatal(out.String())
	}
	items := analyze(childURI, string(text), s.sourceOverlay())
	if len(items) != 1 || items[0].Code != "S1154" || items[0].Range.Start.Line != 1 || items[0].Range.Start.Character != 0 || items[0].Range.End.Character != 9 {
		t.Fatal(items)
	}
	out.Reset()
	if err := s.publishModuleDiagnostics(childURI); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "S1154") {
		t.Fatal("standalone manifest bypassed discovery", out.String())
	}
	// Closing an unsaved child removes its boundary, clears the closed URI's
	// retained diagnostics and leaves the outer manifest contents unchanged.
	s.documentSnapshots.Close(childURI)
	out.Reset()
	if err := s.publishModuleDiagnostics(uri); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), childURI) || strings.Contains(out.String(), "S1154") {
		t.Fatal("stale nested boundary error", out.String())
	}
	data, err := os.ReadFile(outer)
	if err != nil || !bytes.Equal(data, text) {
		t.Fatal("discovery mutated outer manifest", err)
	}
	// A disk child is diagnosed even with no child source or manifest open.
	if err := os.MkdirAll(filepath.Dir(child), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, text, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := s.publishModuleDiagnostics(uri); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "S1154") {
		t.Fatal(out.String())
	}
	if err := os.Remove(child); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := s.publishModuleDiagnostics(uri); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), childURI) || strings.Contains(out.String(), "S1154") {
		t.Fatal(out.String())
	}
}

func TestLSPProjectBoundaryDependencyInvalidation(t *testing.T) {
	root := t.TempDir()
	dependencies := map[string]bool{filepath.Join(root, ".sec", "sec.toml"): true}
	for _, path := range []string{filepath.Join(root, "child", ".sec", "sec.toml"), filepath.Join(filepath.Dir(root), ".sec", "sec.toml")} {
		if !sourceDependencyAffected(dependencies, path) {
			t.Fatalf("new boundary %s did not invalidate project", path)
		}
	}
	if sourceDependencyAffected(dependencies, filepath.Join(root+"-other", ".sec", "sec.toml")) {
		t.Fatal("sibling project invalidated unrelated boundary")
	}
}
