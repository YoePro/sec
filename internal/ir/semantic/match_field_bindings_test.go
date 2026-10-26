package semantic

import (
	"strings"
	"testing"
)

const matchFieldSource = `
module main

type Figure union {
    Rect { Width: int32, Height: int32, Label: int32 }
    Dot(int32)
    Empty
}
`

// flowcontrol_match.md § 10: a shallow named-field copy reads only the bound
// fields with guarded union.unwrap-field on the proven variant path; omitted
// fields are neither read nor copied and no whole payload is built.
func TestMatchFieldPatternCopiesBoundFieldsOnly(t *testing.T) {
	module, err := buildPackage15Source(t, matchFieldSource+`
fn Area(s: Figure) int32 {
    return match s {
        Rect { Width, Height: h } where Width > 0 => h
        Figure.Rect { Height } => Height
        Dot(v) => v
        _ => 0
    }
}

fn main() int {
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"field=Width[copy-trivial]", "field=Height[copy-trivial]"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"field=Label", "struct.construct"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("omitted fields must not be read (%q):\n%s", unwanted, text)
		}
	}
}

// flowcontrol_match.md §§ 10, 13 and P15: a borrowed named field is
// place.union-payload under the arm's variant test, place.field and
// ref.borrow-*, ended on the arm's exits; the sibling field is untouched.
func TestMatchFieldPatternBorrowsFieldPlaces(t *testing.T) {
	module, err := buildPackage15Source(t, matchFieldSource+`
fn Grow(s: Figure) int32 {
    let mut f := s
    match f {
        Rect { Width: ref mut w, Height: ref h } => {
            *w = *h
        }
        _ => {}
    }
    return 0
}

fn main() int {
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"place.union-payload", "place.field", "field=#0", "field=#1", "ref.borrow-mut", "ref.borrow-shared", "ref.end-borrow", "variant-proven"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "field=#2") || strings.Contains(text, "union.unwrap-field") {
		t.Fatalf("borrowed fields must not copy the payload or touch Label:\n%s", text)
	}
}
