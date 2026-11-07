package sema

import (
	"strings"
	"testing"
)

// Union construction and payload forms that frontend.unions lists:
// explicit generic constructor qualification (`Option[int].Some(10)`,
// `Box[int].Full(3)`), the required `<-` when a reusable move-only source is
// consumed into a payload outside a terminal return, and shallow named-field
// payload destructuring with shorthand, rename, omission and field-level
// `ref`/`ref mut`.
//
// Rules:
//   - rules/memory/copy_move.md — §§ 9.2, 10.4
//   - rules/control-flow/flowcontrol_match.md — payload and field bindings
func TestUnionConstructionAndDestructuringForms(t *testing.T) {
	prelude := `module main

type Res struct {
    id: int,
}

impl Res {
    free {
    }
}

type Figure union {
    Named { res: Res, size: int }
    Pair { left: int, right: int }
    Empty
}

type Box[T] union {
    Full(T)
    Hollow
}

`
	valid := analyzeSourceRaw(t, prelude+`fn Qualified() int {
    let o := Option[int].Some(10)
    let b := Box[int].Full(3)
    return 0
}

fn Explicit() Option[Res] {
    let r := Res { id: 1 }
    let o := Some(<-r)
    return o
}

fn TerminalReturn() Option[Res] {
    let r := Res { id: 1 }
    return Some(r)
}

fn Destructure(f: Figure) int {
    match f {
        Named { res: ref r, size } => {
            return r.id + size
        }
        Pair { left: l } => {
            return l
        }
        Empty => {
            return 0
        }
    }
}

fn MutateField(f: ref mut Figure) void {
    match f {
        Named { res: ref mut r } => {
            r.id = 3
        }
        Pair { left } => {
        }
        Empty => {
        }
    }
}
`)
	if len(valid) != 0 {
		t.Fatalf("unexpected errors: %v", valid)
	}

	cases := []struct {
		name string
		body string
		want string
	}{
		{"Some payload", `fn F() int {
    let r := Res { id: 1 }
    let o := Some(r)
    return 0
}`, "cannot copy move-only value r"},
		{"user union payload", `fn F() int {
    let r := Res { id: 1 }
    let f := Figure.Named { res: r, size: 1 }
    return 0
}`, "cannot copy move-only value r"},
		{"Ok payload", `fn F() int {
    let r := Res { id: 1 }
    let o: Result[Res, error] := Ok(r)
    return 0
}`, "cannot copy move-only value r"},
		{"wildcard field binding", `fn F(f: Figure) int {
    match f {
        Pair { left, right: _ } => {
            return left
        }
        _ => {
            return 0
        }
    }
}`, "field pattern right: _ is not a binding"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, prelude+test.body+"\n")
			for _, err := range errors {
				if strings.Contains(err.Message, test.want) {
					return
				}
			}
			t.Fatalf("no diagnostic containing %q; got %v", test.want, errors)
		})
	}
}
