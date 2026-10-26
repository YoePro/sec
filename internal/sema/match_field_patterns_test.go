package sema

import (
	"strings"
	"testing"
)

const matchFieldUnions = `
module main

type Handle struct { id: int }
impl Handle { free {} }

type Figure union {
    Rect { Width: int32, Height: int32 }
    Owned { Label: int32, Item: Handle }
    Dot(int32)
    Empty
}
`

// flowcontrol_match.md §§ 10, 12, 13, 15: shallow named-field patterns bind
// copies of copyable fields, borrow any field, may be partial, renamed and
// qualified, and publish resolved field bindings with stable identities.
func TestShallowNamedFieldMatchPatterns(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, matchFieldUnions+`
fn Area(s: Figure) int32 {
    return match s {
        Rect { Width, Height: h } where Width > 0 => Width * h
        Figure.Rect { Height } => Height
        Owned { Label } => Label
        _ => 0
    }
}

fn Grow(s: Figure) int32 {
    match s {
        Rect { Width: ref mut w, Height: ref hh } => {
            *w = *w + *hh
        }
        Owned { Item: ref item } => {
            discard item
        }
        _ => {}
    }
    return 0
}
`)
	assertSemaErrors(t, errors, nil)
	actions := map[string]ResolvedMatchBindingAction{}
	for _, plan := range analyzer.resolvedMatchPlans {
		for _, arm := range plan.Arms {
			for _, field := range arm.FieldBindings {
				if _, ok := analyzer.ResolvedBindingOf(field.Identifier); !ok {
					t.Fatalf("field binding %s has no resolved identity", field.Identifier.Value)
				}
				actions[field.Field+"="+field.Identifier.Value] = field.Action
			}
		}
	}
	for key, want := range map[string]ResolvedMatchBindingAction{
		"Width=Width": MatchBindingCopyTrivial, "Height=h": MatchBindingCopyTrivial, "Height=Height": MatchBindingCopyTrivial,
		"Label=Label": MatchBindingCopyTrivial, "Width=w": MatchBindingBorrowMutable, "Height=hh": MatchBindingBorrowShared, "Item=item": MatchBindingBorrowShared,
	} {
		if want != "" && actions[key] != want {
			t.Fatalf("%s: action %q, want %q (all %v)", key, actions[key], want, actions)
		}
	}
}

// flowcontrol_match.md §§ 10, 12, 14, 15 and grammar.md: unknown, repeated
// and `_` fields, move-only by-value fields, borrows of temporaries or
// through immutable subjects, non-struct-like variants and bindings used
// outside their arm are rejected.
func TestShallowNamedFieldMatchPatternRejections(t *testing.T) {
	errors := analyzeSourceRaw(t, matchFieldUnions+`
fn Make() Figure {
    return Figure.Empty
}

fn Bad(s: Figure) int32 {
    let a := match s {
        Rect { Depth } => 0
        _ => 1
    }
    let b := match s {
        Rect { Width, Width: w } => w
        _ => 1
    }
    let c := match s {
        Rect { Width: _ } => 0
        _ => 1
    }
    let d := match s {
        Owned { Item } => 0
        _ => 1
    }
    let e := match Make() {
        Rect { Width: ref w } => 0
        _ => 1
    }
    let f := match s {
        Dot { Value } => 0
        _ => 1
    }
    let g := match s {
        Rect { Width: x, Height: x } => x
        _ => 1
    }
    let fixed := Make()
    match fixed {
        Rect { Width: ref mut w } => {
            *w = 1
        }
        _ => {}
    }
    return a + b + c + d + e + f + g + Width
}
`)
	joined := ""
	for _, err := range errors {
		joined += err.Error() + "\n"
	}
	for _, expected := range []string{
		"union variant Figure.Rect has no field Depth",
		"field Width is bound more than once in this pattern",
		"field pattern Width: _ is not a binding",
		"field Item of Figure.Owned is move-only; a named-field binding only copies",
		"cannot borrow union payload field from temporary match subject",
		"union variant Figure.Dot has no named fields to destructure",
		"name x is bound more than once in this pattern",
		"undefined variable Width",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in:\n%s", expected, joined)
		}
	}
	if !strings.Contains(joined, "cannot create mutable reference to immutable place fixed.<Rect>.Width") {
		t.Fatalf("ref mut field of an immutable subject must be rejected:\n%s", joined)
	}
}
