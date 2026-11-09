package sema

import (
	"os"
	"strings"
	"testing"
)

// TestTicketOwnedRevoke checks ticket-owned authority, receiver consumption,
// partial moves, borrow conflicts and sender-independent source semantics.
// Rules: rules/concurrency/channels.md — §§18,30,57(6).
func TestTicketOwnedRevoke(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"ticket_revoke_valid.sec", 0}, {"ticket_revoke_invalid.sec", 6},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatal(tc.file, errors)
		}
		wantMoves := 5
		if tc.errors > 0 {
			wantMoves = 4
		}
		if len(a.receiverMoves) != wantMoves {
			t.Fatal("accepted receiver moves", tc.file, len(a.receiverMoves), wantMoves)
		}
		for call := range a.receiverMoves {
			place, ok := a.ResolvedReceiverMoveOf(call)
			if !ok || place.RootID == 0 {
				t.Fatal("missing canonical consumed receiver", place)
			}
		}
		if tc.errors > 0 {
			messages := ""
			for _, err := range errors {
				messages += err.Message + "\n"
			}
			for _, want := range []string{"owned ticket", "borrowed", "expects 0 arguments", "Sender.Revoke is not part"} {
				if !strings.Contains(messages, want) {
					t.Fatal(want, errors)
				}
			}
		}
	}
}
