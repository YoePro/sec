package formatter

import "testing"

func TestFixMissingListSeparatorsInsertsProvenCommas(t *testing.T) {
	input := `module main

type Point struct {
    x: int // first
    y: int
    z: int
}

fn Sum(
    a: int
    b: int,
) int {
    return a + b
}

fn main() void {
    let total := Sum(
        1
        2,
    )
    let values := [
        1
        2,
    ]
}
`
	want := `module main

type Point struct {
    x: int, // first
    y: int,
    z: int,
}

fn Sum(
    a: int,
    b: int,
) int {
    return a + b
}

fn main() void {
    let total := Sum(
        1,
        2,
    )
    let values := [
        1,
        2,
    ]
}
`
	got := Format(Source{Text: input}, Options{Fix: true}).Text
	if got != want {
		t.Fatalf("corrected source mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{Fix: true}).Text; again != got {
		t.Fatalf("correction is not idempotent\n%s", again)
	}
}

func TestFixMissingListSeparatorsLeavesUnprovenAndLineSeparatedListsAlone(t *testing.T) {
	input := `module main

type Color enum {
    Red
    Green
}

type State union {
    Idle
    Running
}

type Pair struct {
    left: int right: int
}

fn main() void {
    let pair := Pair{
        left: 1
        right: 2
    }
}
`
	got := Format(Source{Text: input}, Options{Fix: true}).Text
	if got != Format(Source{Text: input}, Options{}).Text {
		t.Fatalf("line-separated or same-line lists must not gain commas\n%s", got)
	}
}

func TestMissingListSeparatorsStayUncorrectedWithoutLanguageCorrections(t *testing.T) {
	input := "module main\n\ntype Point struct {\n    x: int\n    y: int,\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != input {
		t.Fatalf("ordinary formatting changed source\n%s", got)
	}
}
