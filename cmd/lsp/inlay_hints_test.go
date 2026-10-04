package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const inlayHintSource = `module main

type Point struct {
    x: int,
    y: int,
}

fn Area(width: int, height: int) int {
    return width * height
}

fn Scale(value: int) int {
    return value * 2
}

fn main() void {
    let width := 3
    let area := Area(width, 4)
    let scaled := Scale(2)
    let point := Point{ x: 1, y: 2 }
    let label: string := "x"
}
`

func inlayHintLabels(hints []inlayHint, text string) []string {
	lines := strings.Split(text, "\n")
	labels := []string{}
	for _, hint := range hints {
		line := lines[hint.Position.Line]
		labels = append(labels, strings.TrimSpace(line[:hint.Position.Character])+" ⟨"+hint.Label+"⟩")
	}
	return labels
}

func allLines() lspRange {
	return lspRange{Start: position{Line: 0}, End: position{Line: 1000}}
}

func TestInlayHintsShowInferredTypesAndParameterNames(t *testing.T) {
	hints := inlayHintsForSource("file:///tmp/hints.sec", inlayHintSource, allLines(), defaultInlayHintSettings())
	got := strings.Join(inlayHintLabels(hints, inlayHintSource), "\n")
	want := strings.Join([]string{
		"let width ⟨: int⟩",
		"let area ⟨: int⟩",
		"let area := Area(width, ⟨height:⟩",
		"let scaled ⟨: int⟩",
	}, "\n")
	if got != want {
		t.Fatalf("hints:\n%s\nwant:\n%s", got, want)
	}
	for _, hint := range hints {
		if strings.HasPrefix(hint.Label, ":") && hint.Kind != inlayHintKindType || strings.HasSuffix(hint.Label, ":") && hint.Kind != inlayHintKindParameter {
			t.Fatalf("hint kind = %#v", hint)
		}
	}
}

func TestInlayHintsRespectRangeAndSettings(t *testing.T) {
	areaLine := strings.Count(inlayHintSource[:strings.Index(inlayHintSource, "let area")], "\n")
	hints := inlayHintsForSource("file:///tmp/hints.sec", inlayHintSource, lspRange{Start: position{Line: areaLine}, End: position{Line: areaLine, Character: 200}}, defaultInlayHintSettings())
	if len(hints) != 2 {
		t.Fatalf("range hints = %#v", hints)
	}
	typesOnly := inlayHintSettingsFrom(defaultInlayHintSettings(), json.RawMessage(`{"sec":{"inlayHints":{"parameters":false}}}`))
	for _, hint := range inlayHintsForSource("file:///tmp/hints.sec", inlayHintSource, allLines(), typesOnly) {
		if hint.Kind == inlayHintKindParameter {
			t.Fatalf("parameter hint despite setting: %#v", hint)
		}
	}
	none := inlayHintSettingsFrom(typesOnly, json.RawMessage(`{"inlayHints":{"types":false}}`))
	if none.Types || none.Parameters {
		t.Fatalf("settings = %#v", none)
	}
	if hints := inlayHintsForSource("file:///tmp/hints.sec", inlayHintSource, allLines(), none); len(hints) != 0 {
		t.Fatalf("disabled hints = %#v", hints)
	}
}

const ownershipHintSource = `module main

@noCopy
type Handle struct {
    id: int,
}

type Figure union {
    Owned(Handle),
    Count(int),
    Empty,
}

fn Inspect(value: ref string) int {
    return 1
}

fn Grow(values: ref mut list[int]) void {
}

fn Take(figure: Figure) int {
    return match figure {
        Owned(handle) => handle.id
        Count(count) => count
        Empty => 0
    }
}

fn Use(numbers: ref mut list[int]) int {
    let name := "sec"
    Grow(numbers)
    return Inspect(name) + Inspect(ref name)
}
`

func TestOwnershipInlayHintsShowImplicitBorrowsAndMatchMoves(t *testing.T) {
	hints := inlayHintsForSource("file:///tmp/ownership.sec", ownershipHintSource, allLines(), inlayHintSettings{Ownership: true})
	got := strings.Join(inlayHintLabels(hints, ownershipHintSource), "\n")
	want := strings.Join([]string{
		"Owned(handle ⟨moves if selected⟩",
		"return Inspect( ⟨ref⟩",
	}, "\n")
	if got != want {
		t.Fatalf("ownership hints:\n%s\nwant:\n%s", got, want)
	}
	for _, hint := range hints {
		if hint.Kind != 0 {
			t.Fatalf("ownership hints carry no type/parameter kind: %#v", hint)
		}
	}
}

func TestOwnershipInlayHintsMarkImplicitMutableBorrowOfOwnedPlace(t *testing.T) {
	source := strings.Replace(ownershipHintSource, "fn Use(numbers: ref mut list[int]) int {", "fn Use(numbers: list[int]) int {\n    let mut owned := numbers", 1)
	source = strings.Replace(source, "Grow(numbers)", "Grow(owned)", 1)
	got := strings.Join(inlayHintLabels(inlayHintsForSource("file:///tmp/ownership.sec", source, allLines(), inlayHintSettings{Ownership: true}), source), "\n")
	if !strings.Contains(got, "Grow( ⟨ref mut⟩") {
		t.Fatalf("missing implicit mutable borrow hint:\n%s", got)
	}
	if hints := inlayHintsForSource("file:///tmp/ownership.sec", source, allLines(), inlayHintSettingsFrom(defaultInlayHintSettings(), []byte(`{"inlayHints":{"ownership":false,"types":false,"parameters":false}}`))); len(hints) != 0 {
		t.Fatalf("disabled ownership hints = %#v", hints)
	}
}
