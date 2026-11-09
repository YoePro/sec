package main

import (
	"os"
	"strings"
	"testing"
)

// Hover/completion use the same Channel field table as Sema, with exact
// endpoint types and canonical names; legacy/ownership errors remain visible.
// Rules: rules/concurrency/channels.md §§15,21,62(2),63(1).
func TestCanonicalChannelEndpointsLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/channel_endpoints_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///channel-endpoints/main.sec"
	offset := strings.Index(source, "channel.Tx") + len("channel.")
	labels := map[string]bool{}
	for _, item := range completeSource(uri, source, offset) {
		labels[item.Label] = true
	}
	if !labels["Tx"] || !labels["Rx"] || labels["tx"] || labels["rx"] {
		t.Fatal(labels)
	}
	for _, tc := range []struct{ field, typ string }{{"Tx", "Sender[int]"}, {"Rx", "Receiver[int]"}} {
		offset := strings.Index(source, "channel."+tc.field) + len("channel.")
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, tc.typ) {
			t.Fatal(tc.field, hover)
		}
	}
	invalid, err := os.ReadFile("../../testdata/sema/channel_endpoints_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity == 1 {
			count++
		}
	}
	if count != 7 {
		t.Fatal(count)
	}
}
