package sema

import (
	"strings"
	"testing"
)

// MD-042 reachable-state proofs now also hold for mutable bindings (owned
// by-value parameters and `let mut`) and for Option subjects, including the
// parser's `is None` form and the early-exit complement (`if x is None
// { return }` proves Some). A mutable binding's fact is version-tracked: an
// assignment to it, any call (a mutable-receiver method or `ref mut` argument
// may change the variant) or loop entry invalidates it. Semantic IR keeps
// reachable-state proofs for declared unions only, so an Option plan carries
// no proof and its arms stay ordinary tested arms.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — § 5 "Reachable-state exhaustiveness (MD-042)", § 25A
//   - rules/declarations/unions.md — § 8.1 "Active variant test"
func TestReachableStateProofsForMutableBindingsAndOption(t *testing.T) {
	prelude := `module main

type Choice union {
    Yes(int)
    No
}

fn Log() void {
}

`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, prelude+`fn Parameter(c: Choice) int {
    if c is Yes {
        match c {
            Yes(v) => {
                return v
            }
        }
    }
    return 0
}

fn MutableLocal(o: Option[int]) int {
    let mut x := o
    if x is Some {
        match x {
            Some(v) => {
                return v
            }
        }
    }
    return 0
}

fn EarlyExitOption(o: Option[int]) int {
    if o is None {
        return 0
    }
    match o {
        Some(v) => {
            return v
        }
    }
}

fn EarlyExitUnion(c: Choice) int {
    if c is not Yes {
        return 0
    }
    match c {
        Yes(v) => {
            return v
        }
    }
}

fn AbsentOption(o: Option[int]) int {
    if o is not None {
        return 1
    }
    match o {
        None => {
            return 0
        }
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}
	count := 0
	for match, plan := range analyzer.resolvedMatchPlans {
		if match.OptionAbsenceTest {
			continue
		}
		count++
		if !plan.Exhaustive {
			t.Errorf("match at line %d is not exhaustive", match.Token.Line)
			continue
		}
		option := isOptionType(plan.SubjectType)
		if option && (plan.ProvenActive != nil || plan.ProvenActiveVariant != "") {
			t.Errorf("Option match at line %d carries an IR proof %+v", match.Token.Line, plan.ProvenActive)
		}
		if !option && plan.ProvenActive == nil {
			t.Errorf("union match at line %d has no proof", match.Token.Line)
		}
	}
	if count != 5 {
		t.Fatalf("found %d match plans, want 5", count)
	}

	cases := []struct {
		name string
		body string
	}{
		{"assignment invalidates", `fn F(c: Choice) int {
    if c is Yes {
        c = Choice.No
        match c {
            Yes(v) => {
                return v
            }
        }
    }
    return 0
}`},
		{"any call invalidates", `fn F(c: Choice) int {
    if c is Yes {
        Log()
        match c {
            Yes(v) => {
                return v
            }
        }
    }
    return 0
}`},
		{"ref mut argument invalidates", `fn Clear(c: ref mut Choice) void {
    *c = Choice.No
}

fn F(c: Choice) int {
    if c is Yes {
        Clear(ref mut c)
        match c {
            Yes(v) => {
                return v
            }
        }
    }
    return 0
}`},
		{"loop entry invalidates", `fn F(o: Option[int]) int {
    let mut x := o
    if x is Some {
        while true {
            match x {
                Some(v) => {
                    return v
                }
            }
            x = None
        }
    }
    return 0
}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, prelude+test.body+"\n")
			for _, err := range errors {
				if strings.Contains(err.Message, "non-exhaustive match") {
					return
				}
			}
			t.Fatalf("expected a non-exhaustive match; got %v", errors)
		})
	}
}
