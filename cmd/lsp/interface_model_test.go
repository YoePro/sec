package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInterfaceLensConsumesCanonicalIdentity rejects prefix-name collisions
// and presents compiler-owned generic and inherited conformance outcomes.
// Rules: rules/compiler/semantic_ir.md — §109(1–4);
// rules/declarations/interfaces.md — §§5–6,9,12.
func TestInterfaceLensConsumesCanonicalIdentity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/interface_model_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	path := filepath.Join(t.TempDir(), "interfaces.sec")
	uri := uriFromPath(path)
	overlay := sourceOverlay{Sources: map[string]string{path: text}}
	summaries := crossTargetInterfaceConformance(uri, text, overlay)
	if len(summaries) != 5 {
		t.Fatalf("summaries: %+v", summaries)
	}
	byName := map[string]interfaceConformanceSummary{}
	for _, summary := range summaries {
		byName[summary.name] = summary
	}
	read := interfaceConformanceCommand(uri, byName["Read"])
	if read.Title != "✓ Good conforms on 1 target" {
		t.Fatalf("Read inherited ReadMore failures: %s", read.Title)
	}
	more := interfaceConformanceCommand(uri, byName["ReadMore"])
	if !strings.HasPrefix(more.Title, "✗ conforms on 0 of 1 targets") || !strings.Contains(more.Title, "Value") {
		t.Fatal(more.Title)
	}
	if len(byName["Sink"].targets[0].implementers) != 2 || len(byName["Sink"].targets[0].failures) != 1 || byName["Sink"].targets[0].failures[0].member != "Write" {
		t.Fatalf("generic conformance lost: %+v", byName["Sink"])
	}
	if title := interfaceConformanceCommand(uri, byName["Child"]).Title; title != "✓ Inherited conforms on 1 target" {
		t.Fatal(title)
	}
	if title := interfaceConformanceCommand(uri, byName["Unresolved"]).Title; !strings.Contains(title, "conformance unresolved") || strings.HasPrefix(title, "✓") {
		t.Fatal(title)
	}
}
