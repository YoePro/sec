package main

import (
	"os"
	"strings"
	"testing"
)

// TestChannelRevokeTypesLSP presents the exact compiler enum/union domains and
// concrete extracted payload type, without extra disposition variants.
// Rules: rules/concurrency/channels.md — §§13–14,62–63.
func TestChannelRevokeTypesLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/channel_revoke_types_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///channel-revoke/main.sec"
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	offset := strings.Index(source, "match disposition") + len("match ")
	hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "MessageDisposition") {
		t.Fatal(hover, ok)
	}
	for _, tc := range []struct {
		name     string
		variants []string
	}{
		{"MessageDisposition", []string{"Received", "Expired", "Discarded"}},
		{"ChannelRevokeResult", []string{"Revoked", "Unavailable"}},
	} {
		completion := source + "\nfn Complete() void {\n discard " + tc.name + ".\n}\n"
		offset := strings.LastIndex(completion, tc.name+".") + len(tc.name) + 1
		assertCompletionLabels(t, completeSource(uri, completion, offset), tc.variants)
	}
	invalid, err := os.ReadFile("../../testdata/sema/channel_revoke_types_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity == 1 {
			count++
		}
	}
	if count != 4 {
		t.Fatal(count)
	}
}
