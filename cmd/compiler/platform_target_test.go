package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestPlatformTargetInspection: `sec platform target list` lists every
// registered target, and `info` reports one target's frozen plan from
// canonical facts: identity, data model, typed alignments, an unresolved
// atomic minimum as unresolved (never a guess), the 64-bit Arena epoch with
// its panic outcome on a 32-bit target, compiler stage support and the plan
// fingerprint, identically in human and JSON form; unknown targets fail.
// Rules: rules/tooling/cli.md — § 10; rules/compiler/compiler.md — § 64(3).
func TestPlatformTargetInspection(t *testing.T) {
	run := func(args ...string) (string, string, int) {
		var stdout, stderr bytes.Buffer
		code := runPlatformCommand(args, &stdout, &stderr)
		return stdout.String(), stderr.String(), code
	}
	out, _, code := run("target", "list")
	if code != 0 || !strings.Contains(out, "linux-amd64") || !strings.Contains(out, "x86_64-pc-linux-gnu") || !strings.Contains(out, "baremetal-cortex-m4") {
		t.Fatalf("list: %d\n%s", code, out)
	}
	out, _, code = run("target", "info", "--target", "linux-armv7")
	for _, want := range []string{"linux-armv7 (armv7-unknown-linux-gnueabihf)", "32 bits, little-endian", "int64:8", "atomic align       unresolved", "64-bit logical, panic on exhaustion", "fingerprint        plan "} {
		if code != 0 || !strings.Contains(out, want) {
			t.Fatalf("info lacks %q (%d):\n%s", want, code, out)
		}
	}
	out, _, code = run("target", "info", "--target", "linux-armv7", "--json")
	var info platformTargetInfo
	if code != 0 || json.Unmarshal([]byte(out), &info) != nil || info.PointerWidthBits != 32 || info.EpochBits != 64 || info.EpochExhaustion != "panic" ||
		len(info.AtomicAlignments) != 0 || info.Fingerprints.Plan == "" || !info.Stages["semantic validation"] {
		t.Fatalf("json info: %d %+v", code, info)
	}
	if _, stderr, code := run("target", "info", "--target", "plan9-mips"); code == 0 || !strings.Contains(stderr, "no registered target plan9-mips") {
		t.Fatalf("unknown target accepted: %d %s", code, stderr)
	}
	if _, stderr, code := run("target", "show"); code == 0 || !strings.Contains(stderr, "usage: sec platform target") {
		t.Fatalf("bad subcommand: %d %s", code, stderr)
	}
}
