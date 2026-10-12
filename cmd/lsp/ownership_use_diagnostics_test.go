package main

import (
	"os"
	"strings"
	"testing"
)

// TestOwnershipUseOriginLSP projects source/API ownership provenance directly
// from compiler diagnostics and offers no unproven ownership repair edits.
// Rules: rules/memory/copy_move.md — §§28.1,29(1);
// rules/memory/destruction.md — §33.2(1),§33.3(2).
func TestOwnershipUseOriginLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/ownership_use_origin_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := "file:///tmp/sec-ownership-use/main.sec"
	values := analyze(uri, string(data))
	if len(values) != 9 {
		t.Fatal(values)
	}
	for i, got := range values {
		if got.Severity != 1 || got.Code == "" || len(got.RelatedInformation) != 2 {
			t.Fatal(got)
		}
		links := got.RelatedInformation
		if links[0].Location.Range.Start.Line != []int{9, 15, 20, 25, 30, 35, 40, 44, 51}[i] || links[1].Location.Range.Start.Line != []int{8, 14, 19, 24, 29, 34, 39, 43, 50}[i] {
			t.Fatal(got)
		}
		for _, link := range links {
			if link.Location.URI != uri {
				t.Fatal(link)
			}
		}
		if i == 4 && !strings.Contains(got.Message, "OkRef") || i == 5 && !strings.Contains(got.Message, "ErrRef") || i == 6 && !strings.Contains(got.Message, "not proof that every path") {
			t.Fatal(got)
		}
	}
	if len(ownershipCodeActions(uri, string(data), values)) != 0 {
		t.Fatal("unproven automatic ownership changes")
	}
}
