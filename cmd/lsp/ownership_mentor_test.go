package main

import (
	"os"
	"strings"
	"testing"
)

// TestOwnershipMentorLSP retains compiler-owned explanations and source links
// and never turns API redesign advice into an automatic ownership edit.
// Rules: rules/memory/copy_move.md — §§8.1,28,29;
// rules/memory/borrowing.md — §§32,33(1,3,5).
func TestOwnershipMentorLSP(t *testing.T) {
	for _, file := range []string{"ownership_call_mentor_invalid.sec", "borrow_overlap_mentor_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/tooling/" + file)
		if err != nil {
			t.Fatal(err)
		}
		uri := "file:///tmp/sec-ownership-mentor/" + file
		values := analyze(uri, string(data))
		count := 10
		if strings.HasPrefix(file, "borrow") {
			count = 5
		}
		if len(values) != count {
			t.Fatal(values)
		}
		for i, got := range values {
			if got.Severity != 1 || len(got.RelatedInformation) == 0 {
				t.Fatal(got)
			}
			for _, link := range got.RelatedInformation {
				if link.Location.URI != uri {
					t.Fatal(link)
				}
			}
			if count == 10 {
				if got.Code != "S1007" {
					t.Fatal(got)
				}
				if i == 2 {
					if !strings.Contains(got.Message, "copies its argument") || !strings.Contains(got.Message, "`<-resource`") || !strings.Contains(got.Message, "To require transfer at every call") {
						t.Fatal(got)
					}
				} else if !strings.Contains(got.Message, "Write `<-") {
					t.Fatal(got)
				}
			} else if !strings.Contains(got.Message, "storage") && !strings.Contains(got.Message, "contains") {
				t.Fatal(got)
			}
		}
		if count == 10 && len(ownershipCodeActions(uri, string(data), values)) != 0 {
			t.Fatal("unproven automatic ownership edit")
		}
	}
}
