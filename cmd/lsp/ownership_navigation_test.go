package main

import (
	"os"
	"strings"
	"testing"
)

// TestOwnershipNavigationLSP projects only compiler locations: custom free,
// transferred borrow holders, and the origin from a previous loop iteration.
// Rules: rules/memory/destruction.md — §33.2(2);
// rules/memory/borrowing.md — §§32(4,8),33(1,3).
func TestOwnershipNavigationLSP(t *testing.T) {
	for _, test := range []struct {
		file                   string
		uses, origins, holders []int
	}{
		{"custom_free_partial_move_invalid.sec", []int{10}, []int{6}, nil},
		{"borrow_holder_transfer_invalid.sec", []int{6, 14}, []int{4, 15}, []int{5, 11}},
	} {
		data, err := os.ReadFile("../../testdata/tooling/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		uri := "file:///tmp/sec-ownership-navigation/" + test.file
		diagnostics := analyze(uri, string(data))
		if len(diagnostics) != len(test.uses) {
			t.Fatal(diagnostics)
		}
		for i, got := range diagnostics {
			count := 1
			if test.holders != nil {
				count = 2
			}
			links := got.RelatedInformation
			if got.Severity != 1 || got.Range.Start.Line != test.uses[i]-1 || len(links) != count || links[0].Location.Range.Start.Line != test.origins[i]-1 {
				t.Fatal(got)
			}
			for _, link := range links {
				if link.Location.URI != uri {
					t.Fatal(link)
				}
			}
			if test.holders == nil {
				if got.Code != "S1085" || links[0].Message != "custom free requires complete self" || !strings.Contains(got.Message, "requires complete self") || !strings.Contains(got.Message, "redesign ownership") {
					t.Fatal(got)
				}
			} else if links[1].Location.Range.Start.Line != test.holders[i]-1 || !strings.Contains(links[1].Message, "keeps authority live") || (i == 1 && !strings.Contains(links[0].Message, "previous loop iteration")) {
				t.Fatal(got)
			}
		}
	}
}
