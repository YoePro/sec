package sema

import (
	"strings"
	"testing"
)

// Predecessor access values[i - 1] needs a proof that i >= 1 (pitfall
// analysis § 25.7(6)): dominating guards, an else branch, an early exit and a
// range loop starting at 1 suppress it; a reassignment ends the proof; a
// zero-based loop neighbor stays with the proven lower-neighbor finding; the
// explanation distinguishes checked unsigned underflow from a signed bounds
// violation.
func TestPitfallPredecessorNeedsPositiveIndex(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

fn Last(values: int[]) int {
    return values[values.Len - 1]
}

fn Pred(values: int[], i: uint) int {
    return values[i - 1]
}

fn PredGuarded(values: int[], i: uint) int {
    if i > 0 {
        return values[i - 1]
    }
    return 0
}

fn PredLoop(values: int[]) int {
    let mut total := 0
    for i in 1u..<values.Len {
        total += values[i] - values[i - 1]
    }
    return total
}

fn PredLoopZero(values: int[]) int {
    let mut total := 0
    for i in 0u..<values.Len {
        total += values[i - 1]
    }
    return total
}

fn Signed(values: int[], i: int) int {
    return values[i - 1]
}

fn EarlyExit(values: int[], i: uint) int {
    if i == 0 {
        return 0
    }
    return values[i - 1]
}

fn Reassigned(values: int[], i: uint) int {
    let mut j := i
    if j > 0 {
        j = 0
        return values[j - 1]
    }
    return 0
}

fn ElseBranch(values: int[], i: uint) int {
    if i == 0 {
        return 0
    } else {
        return values[i - 1]
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}
	findings := map[int]PitfallFinding{}
	lowerNeighbor := map[int]bool{}
	for _, finding := range analyzer.PitfallAnalysis().Findings() {
		switch finding.Rule {
		case PitfallPredecessorNeedsPositiveIndex:
			findings[finding.Subject.Source.Line] = finding
		case PitfallLowerNeighborIndex:
			lowerNeighbor[finding.Subject.Source.Line] = true
		}
	}
	if len(findings) != 3 || !lowerNeighbor[29] || findings[29].Rule != "" {
		t.Fatalf("predecessor findings at lines %v, lower-neighbor %v; want 8, 35 and 49", keysOfFindings(findings), lowerNeighbor)
	}
	want := map[int]string{8: "checked unsigned underflow", 35: "is -1, a bounds violation", 49: "checked unsigned underflow"}
	for line, hazard := range want {
		finding, ok := findings[line]
		if !ok {
			t.Fatalf("no finding at line %d", line)
		}
		found := false
		for _, evidence := range finding.EvidenceFor {
			found = found || strings.Contains(evidence.Fact, hazard)
		}
		if !found {
			t.Errorf("line %d: missing hazard %q in %+v", line, hazard, finding.EvidenceFor)
		}
	}
}

func keysOfFindings(findings map[int]PitfallFinding) []int {
	keys := []int{}
	for key := range findings {
		keys = append(keys, key)
	}
	return keys
}

// Reverse and stepped loops (§ 25.7(6)): a descending range proves its binding
// >= 1 from its end bound (inclusive end >= 1, exclusive end >= 0), an
// ascending stepped range from its start, and a while condition at every body
// entry; a descending range that reaches 0 keeps the finding.
func TestPitfallPredecessorInReverseAndSteppedLoops(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

fn DownToZero(values: int[], n: int) int {
    let mut total := 0
    for i in n..0 step -1 {
        total += values[i - 1]
    }
    return total
}

fn DownToOne(values: int[], n: int) int {
    let mut total := 0
    for i in n..1 step -1 {
        total += values[i - 1]
    }
    return total
}

fn DownExclusive(values: int[], n: int) int {
    let mut total := 0
    for i in n..<0 step -1 {
        total += values[i - 1]
    }
    return total
}

fn UpStepped(values: int[], n: int) int {
    let mut total := 0
    for i in 1..<n step 2 {
        total += values[i - 1]
    }
    return total
}

fn WhileDown(values: int[], n: uint) int {
    let mut total := 0
    let mut i := n
    while i > 0 {
        total += values[i - 1]
        i -= 1
    }
    return total
}
`)
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}
	lines := []int{}
	for _, finding := range analyzer.PitfallAnalysis().Findings() {
		if finding.Rule == PitfallPredecessorNeedsPositiveIndex {
			lines = append(lines, finding.Subject.Source.Line)
		}
	}
	if len(lines) != 1 || lines[0] != 6 {
		t.Fatalf("predecessor findings at lines %v, want only 6 (descending to 0)", lines)
	}
}
