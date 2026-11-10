package main

import (
	"os"
	"strings"
	"testing"
)

// TestBlockingHoverCanonicalEvidence covers existing may-block summaries,
// retaining conservative unknown/foreign evidence beside the selected path.
// Rules: rules/analysis/effect_analysis.md — §§5,30;
// rules/foundations/attributes.md — §81(1); rules/tooling/lsp.md — §4.1(7).
func TestBlockingHoverCanonicalEvidence(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/blocking_hover.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := "file:///blocking-hover/main.sec"
	for _, issue := range analyze(uri, text) {
		if issue.Severity == 1 {
			t.Fatalf("fixture must type-check: %+v", issue)
		}
	}
	a := analyzeNavigationSource(uri, text)
	graph := a.CallGraph()
	seen := map[string]bool{}
	for _, node := range graph.Nodes() {
		if node.Declaration.File != pathFromURI(uri) || node.Name == "External" {
			continue
		}
		summary := graph.BlockSummary(node.ID)
		offset := strings.Index(text, "fn "+node.Name+"(") + 3
		if offset < 3 {
			continue
		}
		result, ok := hoverForSource(uri, text, offsetPosition(text, offset))
		if !ok {
			t.Fatal("missing hover", node.Name)
		}
		seen[node.Name] = true
		if node.Name == "Pure" {
			if !strings.Contains(result.Contents.Value, "May block: `no`") {
				t.Fatal(result)
			}
			continue
		}
		if !summary.MayBlock || !strings.Contains(result.Contents.Value, blockingHoverLine(graph, node.ID)) {
			t.Fatal("compiler/editor summary mismatch", node.Name, result)
		}
		for _, id := range summary.BlockPath {
			step, _ := graph.Node(id)
			if !strings.Contains(result.Contents.Value, "`"+step.Name+"`") {
				t.Fatal("missing selected path", result)
			}
		}
		switch node.Name {
		case "Foreign":
			if !strings.Contains(result.Contents.Value, "unverified foreign call") {
				t.Fatal(result)
			}
		case "Unknown":
			if !strings.Contains(result.Contents.Value, "unresolved call target") {
				t.Fatal(result)
			}
		case "Mixed":
			if !strings.Contains(result.Contents.Value, "Mutex.Lock") || !strings.Contains(result.Contents.Value, "unverified foreign call") {
				t.Fatal(result)
			}
		}
	}
	// A resolved call occurrence must present the callee's synchronous path,
	// rather than accidentally selecting its caller's or the external leaf.
	callOffset := strings.LastIndex(text, "Nested(lock)")
	callHover, ok := hoverForSource(uri, text, offsetPosition(text, callOffset))
	if !ok || !strings.Contains(callHover.Contents.Value, "May block: `yes` via `Nested` -> `Direct`") {
		t.Fatal("wrong call-site path", callHover)
	}
	for _, name := range []string{"Pure", "Direct", "Nested", "Foreign", "Unknown", "Mixed", "Deferred", "Recursive", "Calls"} {
		if !seen[name] {
			t.Fatal("missing callable", name)
		}
	}
}
