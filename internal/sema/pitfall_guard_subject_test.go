package sema

import "testing"

func guardResults(t *testing.T, body string) []PitfallFinding {
	t.Helper()
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
fn Process(value: int) void {}
fn Log(message: string) void {}
fn Reset(index: ref mut uint) void {}
fn Check(left: ref int[], right: ref int[], leftIndex: uint, rightIndex: uint, enabled: bool) void {
    let mut index: uint := leftIndex
    `+body+`
}
`)
	if len(errors) != 0 {
		t.Fatalf("analysis errors: %v", errors)
	}
	return analyzer.PitfallAnalysis().Results()
}

func countRule(results []PitfallFinding, rule PitfallRuleID) (int, *PitfallFinding) {
	count := 0
	var first *PitfallFinding
	for _, result := range results {
		if result.Rule == rule {
			count++
			if first == nil {
				candidate := result
				first = &candidate
			}
		}
	}
	return count, first
}

// A strict guard that protects nothing in its block while another access is
// covered by no guard on its path is the copy/paste guard/use mismatch. Any
// block that uses the guarded pair, and any access covered by an enclosing if,
// while, or half-open loop guard, stays silent; `<=` guards belong to the
// ineffective-guard rule instead.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Guard checks the wrong value"
//   - rules/analysis/pitfall_analysis.md — "Required control-flow tests"
func TestPitfallWrongGuardSubject(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "wrong variable guarded", body: "if leftIndex < left.Len {\n Process(right[rightIndex])\n }", want: 1},
		{name: "wrong collection guarded", body: "if leftIndex < left.Len {\n Process(right[leftIndex])\n }", want: 1},
		{name: "mirrored guard", body: "if left.Len > leftIndex {\n Process(right[rightIndex])\n }", want: 1},
		{name: "guard correctly protects access", body: "if leftIndex < left.Len {\n Process(left[leftIndex])\n }"},
		{name: "guarded pair used beside another access", body: "if leftIndex < left.Len {\n Process(left[leftIndex])\n Process(right[leftIndex])\n }"},
		{name: "conjunction covers both", body: "if leftIndex < left.Len && rightIndex < right.Len {\n Process(right[rightIndex])\n }"},
		{name: "enclosing guard covers access", body: "if rightIndex < right.Len {\n if leftIndex < left.Len {\n Process(right[rightIndex])\n }\n }"},
		{name: "enclosing loop covers access", body: "for j in uint(0)..<right.Len {\n if leftIndex < left.Len {\n Process(right[j])\n }\n }"},
		{name: "enclosing while covers access", body: "while rightIndex < right.Len {\n if leftIndex < left.Len {\n Process(right[rightIndex])\n }\n break\n }"},
		{name: "guard misses equality boundary is another rule", body: "if leftIndex <= left.Len {\n Process(right[rightIndex])\n }"},
		{name: "nested control flow is correlated", body: "if leftIndex < left.Len {\n if enabled {\n Process(right[rightIndex])\n }\n }", want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			count, finding := countRule(guardResults(t, test.body), PitfallWrongGuardSubject)
			if count != test.want {
				t.Fatalf("wrong-guard findings = %d, want %d (%+v)", count, test.want, finding)
			}
			if finding != nil && (finding.Family != PitfallControlFlow || finding.Classification != PitfallLikelyMistake ||
				finding.Confidence != PitfallConfidenceHigh || len(finding.EvidenceFor) != 2 || finding.OwningRule != "bounds") {
				t.Fatalf("incomplete wrong-guard finding: %+v", *finding)
			}
		})
	}
}

// A check that detects an out-of-range index but neither leaves the path nor
// re-establishes the index does not protect the directly following access.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Safety check without control transfer"
func TestPitfallCheckWithoutTransfer(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "log only", body: "if index >= left.Len {\n Log(\"bad index\")\n }\n Process(left[index])", want: 1},
		{name: "mirrored strict check", body: "if left.Len < index {\n Log(\"bad index\")\n }\n Process(left[index])", want: 1},
		{name: "return transfers control", body: "if index >= left.Len {\n Log(\"bad index\")\n return\n }\n Process(left[index])"},
		{name: "panic terminates", body: "if index >= left.Len {\n panic \"bad index\"\n }\n Process(left[index])"},
		{name: "index re-established", body: "if index >= left.Len {\n index = 0\n }\n Process(left[index])"},
		{name: "nested re-establishment", body: "if index >= left.Len {\n if enabled {\n index = 0\n }\n }\n Process(left[index])"},
		{name: "mutable reference may re-establish", body: "if index >= left.Len {\n Reset(ref mut index)\n }\n Process(left[index])"},
		{name: "different collection accessed", body: "if index >= left.Len {\n Log(\"bad index\")\n }\n Process(right[index])"},
		{name: "access not directly after", body: "if index >= left.Len {\n Log(\"bad index\")\n }\n if enabled {\n Process(left[index])\n }", want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			count, finding := countRule(guardResults(t, test.body), PitfallCheckWithoutTransfer)
			if count != test.want {
				t.Fatalf("check-without-transfer findings = %d, want %d (%+v)", count, test.want, finding)
			}
			if finding != nil && (finding.Classification != PitfallLikelyMistake || len(finding.EvidenceFor) != 3 || len(finding.Actions) != 0) {
				t.Fatalf("incomplete check-without-transfer finding: %+v", *finding)
			}
		})
	}
}
