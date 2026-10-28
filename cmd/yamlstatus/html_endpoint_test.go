package main

import (
	"bytes"
	"html"
	"strings"
	"testing"
)

func TestHTMLReportCLIIsACompleteDocument(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "status.yaml", "integrations:\n  - id: sample\n    status: partial\n    implemented: [one]\n    remaining: [two, three]\n")
	var output, errors bytes.Buffer
	if err := runCLI([]string{"-html", dir}, &output, &errors); err != nil {
		t.Fatal(err, errors.String())
	}
	text := output.String()
	if !strings.HasPrefix(text, "<!doctype html>") || !strings.HasSuffix(strings.TrimSpace(text), "</html>") {
		t.Fatal("HTML output contains text outside the document", text)
	}
	for _, want := range []string{`class="layout"`, `class="sidebar"`, `class="content"`, "https://sec-lang.com/manual.css", "33.3%", `value="1" max="3"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Count(text, `class="site-header"`) != 1 || strings.Contains(text, "%!") || strings.Contains(text, "Governance directory:") {
		t.Fatal("duplicate header or corrupted HTML output", text)
	}
}

func TestHTMLReportEscapesGovernanceTextAndKeepsLibraryScope(t *testing.T) {
	unsafe := `<script>alert("status")</script>`
	text := createHTMLOutput(Stats{
		Files: 1, TotalObjects: 1, TotalImplemented: 1,
		StatusCounts:   map[string]int{unsafe: 1},
		FieldCounts:    map[string]int{"zebra": 2, "alpha": 1, unsafe: 3},
		LibraryObjects: 4, LibraryFiles: []string{unsafe + ".yaml"},
	}, true)
	if strings.Contains(text, unsafe) || !strings.Contains(text, html.EscapeString(unsafe)) {
		t.Fatal("governance text was not escaped")
	}
	if !strings.Contains(text, "4 library integrations are not counted as compiler implementation") || !strings.Contains(text, "1 of 1 checklist items") {
		t.Fatal("library scope changed report totals")
	}
	if strings.Index(text, "<code>alpha</code>") > strings.Index(text, "<code>zebra</code>") {
		t.Fatal("metadata order is unstable")
	}
}

func TestHTMLReportEmptyStates(t *testing.T) {
	for _, test := range []struct {
		stats Stats
		want  string
	}{
		{Stats{}, "No YAML files found."},
		{Stats{Files: 1}, "No integrations found."},
		{Stats{Files: 1, TotalObjects: 1}, "No checklist items found."},
	} {
		text := createHTMLOutput(test.stats, false)
		if !strings.Contains(text, test.want) || strings.Contains(text, "NaN") || strings.Contains(text, "<progress") {
			t.Fatalf("incorrect empty state: %s", text)
		}
	}
}
