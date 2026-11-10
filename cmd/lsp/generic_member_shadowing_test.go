package main

import (
	"os"
	"testing"
)

// TestGenericMemberShadowingLSP preserves mandatory compiler identities and
// links each hidden declaration from an unsaved generic method header.
// Rules: rules/foundations/names_scopes_visibility.md — §§8,15,20;
// rules/declarations/generics.md — §9.
func TestGenericMemberShadowingLSP(t *testing.T) {
	for _, tc := range []struct {
		file  string
		count int
	}{{"valid.sec", 0}, {"invalid.sec", 8}} {
		data, err := os.ReadFile("../../testdata/names/generic_member_shadowing/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		uri := "file:///generic-shadowing/main.sec"
		count := 0
		for _, issue := range analyze(uri, string(data)) {
			if issue.Severity != 1 {
				continue
			}
			count++
			if issue.Code != "S1105" || len(issue.RelatedInformation) == 0 || issue.RelatedInformation[0].Location.URI != uri {
				t.Fatal(issue)
			}
		}
		if count != tc.count {
			t.Fatal(tc.file, count, tc.count)
		}
	}
}
