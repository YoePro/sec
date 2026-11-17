package main

import (
	"strings"
	"testing"
)

// The report preserves transitive uncertainty beside independent known trust
// witnesses, including through recursive and deferred execution relationships.
// Rules: rules/memory/unsafe.md §14; rules/analysis/call_graph.md §§44–45.
func TestAnalyseTrustProvenanceUnknownPaths(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "../../testdata/analysis/trust_graph.sec")
	if code != 0 {
		t.Fatal(code, stdout, stderr)
	}
	for _, want := range []string{
		"OuterOpen: trust dependencies unknown via OuterOpen -> Open; open callable contract retained",
		"RecursiveOpen: trust dependencies unknown via RecursiveOpen -> Open",
		"DeferredOpen: trust dependencies unknown via DeferredOpen -> ",
		"Mixed: trust dependencies unknown via Mixed -> OuterOpen -> Open",
		"Mixed: implementation depends on trust via Mixed -> Wrapper",
		"CycleB: implementation depends on trust via CycleB -> CycleA -> Wrapper",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("missing %q:\n%s", want, stdout)
		}
	}
}

// Optional whole-program reports preserve safe wrapper boundaries, direct
// obligations, foreign trust and unresolved-target uncertainty from Sema.
// Rules: rules/memory/unsafe.md — §§3.4,14,52(1),54(7);
// rules/tooling/cli.md — §5.4(2–4).
func TestAnalyseTrustProvenance(t *testing.T) {
	const fixture = "../../testdata/analysis/trust_report.sec"
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", fixture)
	if code != 0 {
		t.Fatal(code, stdout, stderr)
	}
	start := strings.Index(stdout, "\ntrust provenance:\n")
	if start < 0 {
		t.Fatalf("missing optional trust report:\n%s", stdout)
	}
	text := stdout[start:]
	for _, want := range []string{
		"absence of a recorded dependency is not a trust-freedom proof",
		"Leaf: caller-facing safety unsafe at ",
		"Leaf: unsafe operation RawPtr.Read (raw-pointer-access) at ",
		"Leaf: operation obligation for RawPtr.Read: Ensure the pointer is non-null",
		"Wrapper: caller-facing safety safe at ",
		"Wrapper: implementation depends on trust via Wrapper -> Leaf",
		"Outer: implementation depends on trust via Outer -> Wrapper",
		"Foreign: calls trusted foreign foreign_add at ",
		"foreign_add: trusted foreign declaration at ",
		"Open: trust dependencies unknown beyond open callable target at ",
		"Plain: caller-facing safety safe at ",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Plain: implementation depends on trust") || strings.Contains(text, "Plain: unsafe operation") {
		t.Fatal("ordinary safe callable acquired invented trust dependency")
	}
	again, _, againCode := runCLIForDiagnostics(t, "analyse", "--verbose", fixture)
	if againCode != 0 || again != stdout {
		t.Fatal("trust report is not deterministic")
	}
	for _, mode := range []string{"--info", "--json"} {
		text, _, code := runCLIForDiagnostics(t, "analyse", mode, fixture)
		if code != 0 || strings.Contains(text, "trust provenance:") || strings.Contains(text, "caller-facing safety") {
			t.Fatalf("optional trust facts leaked into %s: %d %s", mode, code, text)
		}
	}
	text, _, code = runCLIForDiagnostics(t, "analyse", fixture)
	if code != 0 || strings.Contains(text, "trust provenance:") {
		t.Fatalf("optional facts leaked into default output: %d %s", code, text)
	}
	text, _, code = runCLIForDiagnostics(t, "analyse", "pitfalls", "--verbose", fixture)
	if code != 0 || strings.Contains(text, "trust provenance:") {
		t.Fatalf("selecting one analysis also produced unrelated trust facts: %d %s", code, text)
	}
	text, _, code = runCLIForDiagnostics(t, "help", "analyse")
	if code != 0 || !strings.Contains(text, "trust provenance") {
		t.Fatalf("analysis help omits the optional report: %d %s", code, text)
	}
}

// A function-focused report includes only its represented reachable callables;
// the broader report never turns missing indirect targets into trust freedom.
// Rules: rules/tooling/cli.md — §§5.3,5.4;
// rules/memory/unsafe.md — §14.
func TestAnalyseTrustProvenanceFunctionFocus(t *testing.T) {
	const fixture = "../../testdata/analysis/trust_report.sec"
	for _, name := range []string{"Outer", "Plain", "Open"} {
		stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "--function", name, fixture)
		if code != 0 {
			t.Fatal(code, stdout, stderr)
		}
		for _, excluded := range []string{"Foreign: calls trusted foreign", "foreign_add: trusted foreign declaration"} {
			if strings.Contains(stdout, excluded) {
				t.Fatalf("focus %s leaked unrelated trust: %s", name, stdout)
			}
		}
		if name == "Outer" && !strings.Contains(stdout, "Wrapper: implementation depends on trust via Wrapper -> Leaf") {
			t.Fatal("focused report omitted reachable trust dependency")
		}
		if name == "Plain" && strings.Contains(stdout, "caller-facing safety unsafe") {
			t.Fatal("plain callable focus included unrelated unsafe implementation")
		}
		if name == "Open" && !strings.Contains(stdout, "trust dependencies unknown beyond open callable target") {
			t.Fatal("open callable trust uncertainty disappeared")
		}
	}
}
