package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Canonical fields and endpoint partial moves use the ordinary compiler frontend;
// lowercase names and invalid ownership cannot produce an inspection artifact.
// Rules: rules/concurrency/channels.md §§3(3),15(3–5),21(1–6),57(3).
func TestCanonicalChannelEndpointsCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/channel_endpoints_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "out")
		args := []string{command, "../../testdata/sema/channel_endpoints_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", artifact)
		}
		_, output, code := runCLIForDiagnostics(t, args...)
		if code != 3 {
			t.Fatal(code, output)
		}
		doc := decodeOccurrenceDocument(t, output)
		if doc.Summary.Errors != 7 || !strings.Contains(output, "channel.Tx") || !strings.Contains(output, "channel.Rx") {
			t.Fatal(output)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid source emitted artifact", err)
		}
	}
}
