package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestChannelCapabilityLSP projects the canonical enum and its source comments
// into completion, hover, diagnostics and navigation, without offering Priority.
// Rules: rules/concurrency/channels.md — §§4–5,62(1),(7),63.
func TestChannelCapabilityLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/channel_capability_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "capability.sec"))
	for _, diagnostic := range analyze(uri, source) {
		if diagnostic.Severity == 1 {
			t.Fatal(diagnostic)
		}
	}
	position := offsetPosition(source, strings.Index(source, "ChannelCapability.Statistics")+len("ChannelCapability."))
	hover, ok := hoverForSource(uri, source, position)
	if !ok || !strings.Contains(hover.Contents.Value, "Statistics snapshots") {
		t.Fatal(hover, ok)
	}
	definitions := definitionsForSource(uri, source, position)
	if len(definitions) != 1 || !strings.HasSuffix(definitions[0].URI, "/sec/core/channel.sec") {
		t.Fatal(definitions)
	}
	position = offsetPosition(source, strings.Index(source, "ChannelCapability"))
	hover, ok = hoverForSource(uri, source, position)
	if !ok || !strings.Contains(hover.Contents.Value, "enum ChannelCapability {\n    Revocation\n    Expiration\n    ISRSafeSend\n    Statistics\n}") || strings.Contains(hover.Contents.Value, "Priority") {
		t.Fatal(hover, ok)
	}
	completion := source + "\nfn Complete() void {\n discard ChannelCapability.\n}\n"
	offset := strings.LastIndex(completion, "ChannelCapability.\n") + len("ChannelCapability.")
	assertCompletionLabels(t, completeSource(uri, completion, offset), []string{"Revocation", "Expiration", "ISRSafeSend", "Statistics"})
	invalid := strings.Replace(source, "ChannelCapability.Statistics", "ChannelCapability.Priority", 1)
	count := 0
	for _, diagnostic := range analyze(uri, invalid) {
		if diagnostic.Severity != 1 {
			continue
		}
		count++
		if !strings.Contains(diagnostic.Message, "ChannelCapability.Priority has never been declared") {
			t.Fatal(diagnostic)
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
}
