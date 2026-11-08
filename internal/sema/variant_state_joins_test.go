package sema

import (
	"strings"
	"testing"
)

// Variant proofs cross control-flow joins (MD-042): a variant proven at the
// end of every continuing branch of an if, on every reachable break edge and
// the condition-false exit of a while, or stored by a direct assignment of a
// variant constructor holds after the join. A path without the proof, or an
// intervening call, leaves the domain whole.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — § 5 "Reachable-state exhaustiveness (MD-042)"
//   - rules/control-flow/flowcontrol_while.md — § 28 "Sema and flow-analysis requirements"
//   - rules/control-flow/flowcontrol_if.md — branch joins
func TestVariantProofsCrossJoins(t *testing.T) {
	valid := analyzeSourceRaw(t, "module main\n"+`
fn B(o: Option[int]) int {
    let mut x := o
    if x is None {
        x = Some(2)
    }
    match x {
        Some(v) => {
            return v
        }
    }
}

fn C(o: Option[int]) int {
    let x := o
    while true {
        if x is Some {
            break
        }
    }
    match x {
        Some(v) => {
            return v
        }
    }
}

fn D(o: Option[int]) int {
    let mut x := o
    while x is None {
        x = Some(1)
    }
    match x {
        Some(v) => {
            return v
        }
    }
}

type Choice union {
    Yes(int)
    No
}

fn E(c: Choice) int {
    let mut x := c
    if x is No {
        x = Choice.Yes(4)
    } else {
        x = Choice.Yes(5)
    }
    match x {
        Yes(v) => {
            return v
        }
    }
}
`)
	if len(valid) != 0 {
		t.Fatalf("unexpected errors: %v", valid)
	}
	prelude := "module main\n\nfn Log() void {\n}\n\n"
	cases := map[string]string{
		"B1": `fn B1(o: Option[int], flag: bool) int {
    let mut x := o
    if x is None {
        x = Some(2)
    } else if flag {
        x = None
    }
    match x {
        Some(v) => {
            return v
        }
    }
}
`,
		"C1": `fn C1(o: Option[int], flag: bool) int {
    let x := o
    while true {
        if x is Some {
            break
        }
        if flag {
            break
        }
    }
    match x {
        Some(v) => {
            return v
        }
    }
}
`,
		"D1": `fn D1(o: Option[int], flag: bool) int {
    let mut x := o
    while x is None {
        if flag {
            break
        }
        x = Some(1)
    }
    match x {
        Some(v) => {
            return v
        }
    }
}
`,
		"F1": `fn F1(o: Option[int]) int {
    let mut x := o
    if x is None {
        x = Some(2)
    }
    Log()
    match x {
        Some(v) => {
            return v
        }
    }
}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, prelude+body+"\n")
			for _, err := range errors {
				if strings.Contains(err.Message, "non-exhaustive match") {
					return
				}
			}
			t.Fatalf("expected a non-exhaustive match; got %v", errors)
		})
	}
}
