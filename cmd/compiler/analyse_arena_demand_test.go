package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestAnalyseArenaDemand: `sec analyse arena-demand` presents Sema's
// byte-accurate peak demand per created Arena domain with its derivation and
// fixed backing (verbose), reports a constant peak above the backing as an
// advisory finding by default, and carries every domain in the JSON report.
// Rules: rules/memory/arena.md — § 114; rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79-81, 130.
func TestAnalyseArenaDemand(t *testing.T) {
	fixture := "../../testdata/sema/arena_demand_report.sec"
	out, stderr, code := runCLIForDiagnostics(t, "analyse", "arena-demand", "--verbose", fixture)
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, stderr)
	}
	for _, want := range []string{"arena demand:", "Fits: Arena domain #", "peak demand 19 bytes of a 64-byte backing", "Overflows: Arena domain #", "peak demand 30 bytes of a 16-byte backing"} {
		if !strings.Contains(out, want) {
			t.Fatalf("verbose report lacks %q:\n%s", want, out)
		}
	}
	out, _, _ = runCLIForDiagnostics(t, "analyse", "arena-demand", fixture)
	if !strings.Contains(out, "can need 30 bytes but its fixed backing has 16") || strings.Contains(out, "can need 19 bytes") {
		t.Fatalf("default report must flag only the overflowing domain:\n%s", out)
	}
	out, _, _ = runCLIForDiagnostics(t, "analyse", "arena-demand", "--json", fixture)
	var document struct {
		ArenaDemand []struct {
			Callable, Kind, Bytes, CapacityBytes string
		} `json:"arena_demand"`
	}
	if err := json.Unmarshal([]byte(out), &document); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(document.ArenaDemand) != 2 || document.ArenaDemand[0].Kind != "constant" || document.ArenaDemand[1].Bytes != "30" {
		t.Fatalf("JSON arena_demand: %+v", document.ArenaDemand)
	}
}
