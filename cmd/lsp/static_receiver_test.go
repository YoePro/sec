package main

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// LSP publishes missing receiver diagnostics from Sema; explicit instance
// access in a static function remains valid.
// Rules: rules/declarations/static.md — §§9,9.1;
// rules/tooling/diagnostics.md — §§2(6–10),15.
func TestStaticReceiverIsolationLSP(t *testing.T) {
	for _, name := range []string{"receiver_valid", "receiver_invalid"} {
		data, err := os.ReadFile("../../testdata/sema/static_receiver/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		values := analyze("file:///tmp/sec-static-receiver/main.sec", string(data))
		errors, guards := 0, 0
		for _, d := range values {
			if d.Severity != 1 {
				continue
			}
			errors++
			if d.Code == diagnostics.StaticInstanceReceiverRequired {
				guards++
				if !strings.Contains(d.Message, "help:") || d.Range.End.Character <= d.Range.Start.Character {
					t.Fatalf("receiver diagnostic: %+v", d)
				}
			}
		}
		if name == "receiver_valid" && errors != 0 || name == "receiver_invalid" && (errors != 13 || guards != 7) {
			t.Fatalf("%s: %+v", name, values)
		}
	}
}
