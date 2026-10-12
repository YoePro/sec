package main

import (
	"os"
	"strings"
	"testing"
)

// TestTryAssignmentFlowPresentation checks prepared/incoming/outgoing hierarchy
// and callable flow hover against the compiler-owned assignment plans.
// Rules: rules/errors/errorhandling.md — §§23,36; rules/tooling/lsp.md — "Hover", "Call hierarchy".
func TestTryAssignmentFlowPresentation(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/try_assignment_flow_valid_test.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := "file:///tmp/sec-try-assignment-flow/main_test.sec"
	for _, name := range []string{"Propagate", "Handle", "Partial", "Terminate"} {
		pos := offsetPosition(text, strings.Index(text, "fn "+name)+3)
		items := callHierarchyItemsForSource(uri, text, pos)
		wantDetail := "fallible assignments: 0 propagate, 1 local handlers"
		if name == "Propagate" {
			wantDetail = "fallible assignments: 1 propagate, 0 local handlers"
		}
		if name == "Partial" {
			wantDetail = "fallible assignments: 1 propagate, 1 local handlers"
		}
		if len(items) != 1 || !strings.Contains(items[0].Detail, wantDetail) {
			t.Fatal(name, items)
		}
		hover, ok := hoverForSource(uri, text, pos)
		if !ok || !strings.Contains(hover.Contents.Value, "Fallible assignment at /tmp/sec-try-assignment-flow/main_test.sec:") || !strings.Contains(hover.Contents.Value, "success continues") {
			t.Fatal(name, hover)
		}
		want := "failure propagates to `Result[void, SetterError]`"
		if name == "Handle" {
			want = "exhaustive local handlers; handled failure may continue"
		}
		if name == "Partial" {
			want = "partial local handlers; handled failure may continue"
		}
		if name == "Terminate" {
			want = "selected handlers leave control flow (`returns`)"
		}
		if !strings.Contains(hover.Contents.Value, want) {
			t.Fatal(name, hover.Contents.Value)
		}
		incoming := callHierarchyIncomingCallsForSource(uri, text, items[0].Data.NodeID)
		if len(incoming) != 1 || incoming[0].From.Name != "Caller" || !strings.Contains(incoming[0].From.Detail, "fallible assignments: 0 propagate, 1 local handlers") {
			t.Fatal(name, incoming)
		}
	}
	caller := callHierarchyItemsForSource(uri, text, offsetPosition(text, strings.Index(text, "fn Caller")+3))
	if len(caller) != 1 {
		t.Fatal(caller)
	}
	seen := map[string]bool{}
	for _, outgoing := range callHierarchyOutgoingCallsForSource(uri, text, caller[0].Data.NodeID) {
		switch outgoing.To.Name {
		case "Propagate", "Handle", "Partial", "Terminate":
			seen[outgoing.To.Name] = true
		default:
			continue
		}
		if !strings.Contains(outgoing.To.Detail, "fallible assignments:") {
			t.Fatal(outgoing)
		}
	}
	if len(seen) != 4 {
		t.Fatal("missing outgoing callees", seen)
	}
	// Incoming caller items carry their own resolved assignments.
	propagate := callHierarchyItemsForSource(uri, text, offsetPosition(text, strings.Index(text, "fn Propagate")+3))[0]
	for _, outgoing := range callHierarchyOutgoingCallsForSource(uri, text, propagate.Data.NodeID) {
		for _, incoming := range callHierarchyIncomingCallsForSource(uri, text, outgoing.To.Data.NodeID) {
			if incoming.From.Name == "Propagate" && !strings.Contains(incoming.From.Detail, "1 propagate") {
				t.Fatal(incoming)
			}
		}
	}
}

// TestInvalidTryAssignmentHasNoFlowPresentation rejects syntax-derived plans.
// Rules: rules/errors/errorhandling.md — §§23,36.
func TestInvalidTryAssignmentHasNoFlowPresentation(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/try_assignment_flow_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := "file:///tmp/sec-try-assignment-flow/invalid.sec"
	pos := offsetPosition(text, strings.Index(text, "fn Invalid")+3)
	for _, item := range callHierarchyItemsForSource(uri, text, pos) {
		if strings.Contains(item.Detail, "fallible assignments:") {
			t.Fatal(item)
		}
	}
	if hover, ok := hoverForSource(uri, text, pos); ok && strings.Contains(hover.Contents.Value, "Fallible assignment at") {
		t.Fatal(hover)
	}
}
