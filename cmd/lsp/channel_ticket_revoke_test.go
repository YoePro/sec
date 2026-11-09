package main

import (
	"os"
	"strings"
	"testing"
)

// TestTicketOwnedRevokeLSP presents the consuming receiver and concrete result
// from compiler-owned member facts, without offering revoke on Sender[T].
// Rules: rules/concurrency/channels.md — §§18,30,62(5),63.
func TestTicketOwnedRevokeLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/ticket_revoke_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///ticket-revoke/main.sec"
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	offset := strings.Index(source, "ticket.Revoke") + len("ticket.")
	hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "Consumes this ticket") || !strings.Contains(hover.Contents.Value, "ChannelRevokeResult[int]") {
		t.Fatal(hover, ok)
	}
	for _, receiver := range []string{"ticket", "sender"} {
		completion := source + "\nfn Complete(ticket: MessageTicket[int], sender: Sender[int]) void {\n discard " + receiver + ".\n}\n"
		offset := strings.LastIndex(completion, receiver+".") + len(receiver) + 1
		found := false
		for _, item := range completeSource(uri, completion, offset) {
			if item.Label == "Revoke" {
				found = true
			}
		}
		if found != (receiver == "ticket") {
			t.Fatal(receiver, found)
		}
	}
	invalid, err := os.ReadFile("../../testdata/sema/ticket_revoke_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity == 1 {
			count++
		}
	}
	if count != 6 {
		t.Fatal(count)
	}
}
