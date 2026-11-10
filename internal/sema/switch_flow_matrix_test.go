package sema

import "testing"

const switchMatrixPrelude = `
module main

type Res struct {
    data: int[],
}

type State union {
    Idle
    Busy
}

fn Check(r: Res) bool {
    return true
}

fn Use(r: Res) void {}
`

// Switch flow regression matrix: lazy source-order case tests, short-circuit
// comma alternatives, ownership and assignment state across fallthrough,
// destination tests skipped on the fallthrough edge, call-graph reachability
// after an always-matching alternative, and open bit-backed enum coverage.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §§ 12, 16, 19, 24, 32, 36–37
//   - rules/corrections/applied/correction24-20260823.md — "Architectural correction"
func TestSwitchFlowRegressionMatrix(t *testing.T) {
	tests := []struct {
		name, body string
		want       []string
	}{
		{"skipped later consuming case call leaves source available", `
fn A(flag: bool) void {
    let r := Res { data: [] }
    switch {
    case flag:
        Use(<-r)
    case Check(<-r):
        discard 0
    }
}`, nil},
		{"comma alternatives short-circuit frontend state", `
fn B(flag: bool) void {
    let r := Res { data: [] }
    switch {
    case flag, Check(<-r):
        Use(<-r)
    }
}`, []string{"r may no longer be available here; it was consumed by a call on one possible execution path at 23:15, ownership action on a possibly unavailable path at 22:24, source ownership established here at 20:9"}},
		{"move in fallthrough source reaches the destination", `
fn C(k: int) void {
    let r := Res { data: [] }
    switch k {
    case 1:
        Use(<-r)
        fallthrough
    case 2:
        Use(<-r)
    default:
        Use(<-r)
    }
}`, []string{"r may no longer be available here; it was consumed by a call on one possible execution path at 26:15, ownership action on a possibly unavailable path at 23:15, source ownership established here at 20:9"}},
		{"destination tests are skipped on the fallthrough edge", `
fn D(flag: bool) void {
    let r := Res { data: [] }
    switch {
    case flag:
        fallthrough
    case Check(<-r):
        Use(<-r)
    }
}`, []string{"r may no longer be available here; it was consumed by a call on one possible execution path at 25:15, ownership action on a possibly unavailable path at 24:18, source ownership established here at 20:9"}},
		{"assignment state merges across fallthrough", `
fn E(k: int) State {
    let mut s: State
    switch k {
    case 1:
        s = State.Idle
        fallthrough
    case 2:
        s = State.Busy
    default:
        s = State.Idle
    }
    return s
}`, nil},
		{"fallthrough-only assignment is not definite in the destination", `
fn E2(k: int) State {
    let mut s: State
    switch k {
    case 1:
        s = State.Idle
        fallthrough
    case 2:
        return s
    default:
        return State.Idle
    }
}`, []string{"variable s is unassigned at 26:16"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertSemaErrors(t, analyzeSourceRaw(t, switchMatrixPrelude+test.body), test.want)
		})
	}

	t.Run("skipped later case call does not register reachable effect", func(t *testing.T) {
		assertSemaErrors(t, analyzeSourceRaw(t, `
module main

fn Boom() bool {
    panic "x"
}

@noPanic
fn Safe() int {
    switch {
    case true:
        return 1
    case Boom():
        return 2
    }
    return 0
}
`), nil)
	})

	t.Run("open bit-backed declared members alone remain non-exhaustive", func(t *testing.T) {
		assertSemaErrors(t, analyzeSourceRaw(t, `
module main

enum Mode bit[2] {
    Off = 0,
    Low = 1,
    High = 2,
}

fn Pick(m: Mode) int {
    switch m {
    case Mode.Off:
        return 0
    case Mode.Low:
        return 1
    case Mode.High:
        return 2
    }
}
`), []string{"function Pick must return int at 10:4"})
	})
}
