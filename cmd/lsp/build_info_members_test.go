package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLSPBuildInfoMembers exercises source-resolved fields and the static
// operation, including incomplete selectors and an inferred Current result.
// Rules: rules/library/core-library.md — §28; rules/tooling/lsp.md — §4.1(5–7);
// rules/compiler/compiler_known_members.md — §113B.
func TestLSPBuildInfoMembers(t *testing.T) {
	enableAnalysisCache()
	t.Cleanup(disableAnalysisCache)
	data, err := os.ReadFile("../../testdata/tooling/build_info/members.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	root := t.TempDir()
	path := filepath.Join(root, "cmd", "worker", "main.sec")
	uri := uriFromPath(path)
	manifest := "[project]\nname='Worker Project'\nuuid='550e8400-e29b-41d4-a716-446655440000'\n[version]\nmajor=2\nminor=0\nrevision=1\nbuild=7\nformat='[major].[minor].[revision]+[build]'\n[build]\nprofile='release'\n[target.worker]\nkind='command'\nsource='cmd/worker'\nvariants=['linux']\n[variant.linux]\nos='linux'\narch='amd64'\n"
	if err := os.MkdirAll(filepath.Join(root, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	overlay := sourceOverlay{Sources: map[string]string{filepath.Join(root, ".sec", "sec.toml"): manifest}}
	analysis := analyzeLSPDocumentResult(uri, parseProgramForLSP(uri, text), overlay, lspAnalysisDepthWithOverlay(path, overlay))
	if _, ok := analysis.analyzer.BuildInfo(); !ok {
		t.Fatalf("fixture has no resolved BuildInfo plan: %v", analysis.semanticErrors)
	}
	if len(analysis.semanticErrors) != 0 {
		t.Fatalf("fixture diagnostics: %v", analysis.semanticErrors)
	}
	fields := []struct{ name, typ string }{
		{"ProjectUUID", "string"}, {"ProjectName", "string"}, {"TargetName", "string"}, {"VariantName", "string"},
		{"VersionMajor", "uint64"}, {"VersionMinor", "uint64"}, {"VersionRevision", "uint64"}, {"VersionBuild", "uint64"},
		{"VersionUserDefined", "Option[string]"}, {"VersionRendered", "string"},
		{"OperatingSystem", "string"}, {"Architecture", "string"}, {"BuildProfile", "string"}, {"TargetProfile", "string"},
		{"SourceRevision", "Option[string]"}, {"DirtySourceStatus", "Option[bool]"},
	}
	offset := strings.Index(text, "info.ProjectUUID") + len("info.")
	items := completeSource(uri, text, offset, overlay)
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			var item *completionItem
			for _, candidate := range items {
				if candidate.Label == field.name {
					item = &candidate
					break
				}
			}
			if item == nil || item.Kind != 5 || item.Detail != field.typ {
				t.Fatalf("field completion = %+v; want %s: %s", item, field.name, field.typ)
			}
			if !strings.Contains(item.Documentation, "field "+field.name+": "+field.typ) {
				t.Fatalf("completion lost field signature: %+v", item)
			}
			offset := strings.Index(text, "info."+field.name) + len("info.")
			hover, ok := hoverForSource(uri, text, offsetPosition(text, offset), overlay)
			if !ok || !strings.Contains(hover.Contents.Value, field.name+": "+field.typ) {
				t.Fatalf("field hover = %+v (%v)", hover, ok)
			}
			if hover.Range != offsetsRange(text, offset, offset+len(field.name)) {
				t.Fatalf("hover range = %+v", hover.Range)
			}
		})
	}
	for _, item := range items {
		if item.Label == "Current" || item.Label == "Profile" {
			t.Fatalf("non-instance member offered: %+v", item)
		}
	}
	current := strings.Index(text, "BuildInfo.Current") + len("BuildInfo.")
	hover, ok := hoverForSource(uri, text, offsetPosition(text, current), overlay)
	if !ok || !strings.Contains(hover.Contents.Value, "static fn Current() BuildInfo") || !strings.Contains(hover.Contents.Value, "resolved Target and Variant") {
		t.Fatalf("Current hover = %+v (%v)", hover, ok)
	}
	staticItems := completeSource(uri, text, current, overlay)
	foundCurrent := false
	for _, item := range staticItems {
		if item.Label == "Current" {
			foundCurrent = true
			if item.Detail != "BuildInfo" || item.Documentation != hover.Contents.Value {
				t.Fatalf("Current completion = %+v", item)
			}
		}
		for _, field := range fields {
			if item.Label == field.name {
				t.Fatalf("instance field offered on type: %+v", item)
			}
		}
	}
	if !foundCurrent {
		t.Fatal("Current missing from type completion", staticItems)
	}
	inferred := strings.LastIndex(text, "info.VersionBuild") + len("info.")
	hover, ok = hoverForSource(uri, text, offsetPosition(text, inferred), overlay)
	if !ok || !strings.Contains(hover.Contents.Value, "VersionBuild: uint64") {
		t.Fatalf("inferred instance hover = %+v (%v)", hover, ok)
	}
	assertCompletionLabels(t, completeSource(uri, text, inferred, overlay), []string{"VersionBuild"})
	// The stored manifest belongs only to this isolated temporary project.
	if current, err := os.ReadFile(filepath.Join(root, ".sec", "sec.toml")); err != nil || string(current) != manifest {
		t.Fatalf("editor mutated manifest: %v", err)
	}
	// A missing plan diagnoses the call but must not hide its API from editing.
	if h, ok := hoverForSource(uriFromPath(filepath.Join(t.TempDir(), "main.sec")), text, offsetPosition(text, current)); !ok || !strings.Contains(h.Contents.Value, "static fn Current() BuildInfo") {
		t.Fatalf("Current lost without plan: %+v (%v)", h, ok)
	}
}
