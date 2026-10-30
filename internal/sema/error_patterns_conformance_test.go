package sema

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Shared declarations of the MD-029 conformance tests: a closed union error
// with a move-only payload, a field payload and a fieldless variant.
const errorPatternConformancePrelude = `
module main

type Handle struct {
    id: int
}

impl Handle {
    free {
    }
}

type LoadError union error {
    Busy(Handle)
    Bad {
        Code: int,
        Text: string
    }
    Stop
}

fn Load() Result[int, LoadError] {
    return Ok(1)
}

fn Consume(h: Handle) bool {
    return true
}

fn Inspect(h: ref Handle) bool {
    return h.id > 0
}
`

func wantSemaMessages(t *testing.T, errs []Error, wants ...string) {
	t.Helper()
	if len(errs) != len(wants) {
		t.Fatalf("errors = %v, want %d: %q", errs, len(wants), wants)
	}
	for _, want := range wants {
		found := false
		for _, err := range errs {
			found = found || strings.Contains(err.Message, want)
		}
		if !found {
			t.Errorf("missing %q in %v", want, errs)
		}
	}
}

// M029-11: a payload pattern binds exactly one name, so a nested pattern is
// a focused parse error, and a binding mode the subject cannot grant (ref mut
// through an immutable Result) is rejected.
// Rules: rules/errors/errorhandling.md — §§ 15.3, 20;
// rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md — § 17 (M029-11).
func TestErrorPatternNestingAndBindingModes(t *testing.T) {
	parsed := parser.New(lexer.New(errorPatternConformancePrelude + `
fn Nested() int {
    match Load() {
        Ok(v) => return v
        Err(.Busy(Some(x))) => return 1
        Err(_) => return 0
    }
}
`))
	parsed.ParseProgram()
	if errors := strings.Join(parsed.Errors(), "\n"); !strings.Contains(errors, "an error variant payload pattern contains exactly one binding; nested patterns are not part of Sec 0.1") {
		t.Fatalf("nested pattern errors = %s", errors)
	}

	errs := analyzeSourceRaw(t, errorPatternConformancePrelude+`
fn ImmutableRefMut() int {
    let result := Load()
    match result {
        Ok(v) => return v
        Err(.Busy(ref mut handle)) => return 1
        Err(_) => return 0
    }
}
`)
	wantSemaMessages(t, errs, "cannot create mutable reference to immutable place result.<Err>.<Busy>")
}

// M029-12: a guard runs before its arm is selected, so it may inspect but
// not consume a payload or field binding; a false guard leaves the payload
// for the next arm, and the selected arm commits its transfer exactly once.
// Rules: rules/control-flow/flowcontrol_match.md — Guards;
// rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md — § 17 (M029-12).
func TestErrorPatternGuardOwnership(t *testing.T) {
	errs := analyzeSourceRaw(t, errorPatternConformancePrelude+`
fn GuardFalseThenMove() int {
    match Load() {
        Ok(v) => return v
        Err(.Busy(handle)) where handle.id > 0 => {
            discard Consume(<-handle)
            return 1
        }
        Err(.Busy(handle)) => {
            discard Consume(<-handle)
            return 2
        }
        Err(_) => return 0
    }
}

fn FieldGuard() int {
    match Load() {
        Ok(v) => return v
        Err(.Bad { Code: code }) where code > 0 => return code
        Err(.Bad { Code: code }) => return 0 - code
        Err(_) => return 0
    }
}
`)
	if len(errs) != 0 {
		t.Fatalf("guard-false continuation rejected: %v", errs)
	}

	errs = analyzeSourceRaw(t, errorPatternConformancePrelude+`
fn GuardConsumes() int {
    match Load() {
        Ok(v) => return v
        Err(.Busy(handle)) where Consume(<-handle) => return 1
        Err(_) => return 0
    }
}

fn MovesTwice() int {
    match Load() {
        Ok(v) => return v
        Err(.Busy(handle)) => {
            discard Consume(<-handle)
            discard Consume(<-handle)
            return 1
        }
        Err(_) => return 0
    }
}
`)
	if len(errs) != 2 || errs[0].Line != 37 || !strings.Contains(errs[0].Message, "match guard must not consume handle before the arm is selected") ||
		errs[1].Line != 47 || !strings.Contains(errs[1].Message, "handle") {
		t.Fatalf("errors = %v", errs)
	}
}

// M029-13: shared and mutable payload references follow the ordinary
// borrow and alias rules; a live ref mut payload binding blocks other use of
// the subject, and move-only payloads are never silently partially moved.
// Rules: rules/memory/borrowing.md — §§ 8, 33(3);
// rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md — § 17 (M029-13).
func TestErrorPatternReferenceBindings(t *testing.T) {
	errs := analyzeSourceRaw(t, errorPatternConformancePrelude+`
fn Shared() int {
    let result := Load()
    match result {
        Ok(v) => return v
        Err(.Busy(ref handle)) where Inspect(handle) => return handle.id
        Err(.Bad { Code: ref code, Text: ref text }) => return 2
        Err(_) => return 0
    }
}

fn Exclusive() int {
    let mut result := Load()
    match result {
        Ok(v) => return v
        Err(.Busy(ref mut handle)) => {
            handle.id = 3
            return handle.id
        }
        Err(.Bad { Text: ref mut text }) => return 4
        Err(_) => return 0
    }
}
`)
	if len(errs) != 0 {
		t.Fatalf("reference payload bindings rejected: %v", errs)
	}

	errs = analyzeSourceRaw(t, errorPatternConformancePrelude+`
fn Alias() int {
    let mut result := Load()
    match result {
        Ok(v) => return v
        Err(.Busy(ref mut handle)) => {
            let observed := result
            handle.id = 3
            return 1
        }
        Err(_) => return 0
    }
}

fn PartialMove() void {
    let result := Load()
    match result {
        Ok(v) => {}
        Err(.Busy(handle)) => {}
        Err(_) => {}
    }
}
`)
	wantSemaMessages(t, errs,
		"cannot read result while it is mutably borrowed",
		"the Handle payload of LoadError.Busy is move-only; binding it by value would partially move the Result")
}

// § 17(3): for the finite domain of a closed concrete error type, Sema's
// match coverage must agree with an independent enumeration for every subset
// of variant arms in every pattern spelling; on an open `error` channel no
// set of concrete arms is exhaustive, even when an unrelated type declares
// the same variant spellings.
// Rules: rules/control-flow/flowcontrol_match.md — § 5; rules/errors/errorhandling.md — § 27.2b;
// rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md — § 17(3).
func TestErrorPatternCoverageDifferential(t *testing.T) {
	const prelude = `
module main

type E union error {
    A
    B(int)
    C { X: int }
}

enum F error {
    A
    B
    C
}

fn Closed() Result[int, E] {
    return Ok(1)
}

fn Open() Result[int, error] {
    return Ok(1)
}
`
	domain := []string{"A", "B", "C"}
	spellings := map[string][]string{
		"qualified":  {"Err(E.A)", "Err(E.B)", "Err(E.C)"},
		"contextual": {"Err(.A)", "Err(.B)", "Err(.C)"},
		"payload":    {"Err(.A)", "Err(.B(value))", "Err(.C { X: value })"},
		"explicit":   {"Err(E.A)", "Err(E.B(_))", "Err(E.C {})"},
	}
	names := make([]string, 0, len(spellings))
	for name := range spellings {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, spelling := range names {
		patterns := spellings[spelling]
		for subset := 0; subset < 1<<len(domain); subset++ {
			arms := []string{"Ok(v) => {}"}
			var missing []string
			for index, variant := range domain {
				if subset&(1<<index) != 0 {
					arms = append(arms, patterns[index]+" => {}")
				} else {
					missing = append(missing, "Err(E."+variant+")")
				}
			}
			source := prelude + "\nfn Use() void {\n    match Closed() {\n        " + strings.Join(arms, "\n        ") + "\n    }\n}\n"
			errs := analyzeSourceRaw(t, source)
			label := fmt.Sprintf("%s subset %03b", spelling, subset)
			want := "missing " + strings.Join(missing, ", ")
			if subset == 0 {
				// No Err arm at all reports the whole Err case.
				want = "missing Err at"
			}
			if len(missing) == 0 {
				if len(errs) != 0 {
					t.Errorf("%s: complete coverage rejected: %v", label, errs)
				}
				continue
			}
			if len(errs) != 1 || !strings.Contains(errs[0].Error(), "non-exhaustive match for Result[int, E]: "+want) {
				t.Errorf("%s: want missing %v, got %v", label, missing, errs)
			}
		}
	}

	for subset := 0; subset < 1<<6; subset++ {
		arms := []string{"Ok(v) => {}"}
		for index, pattern := range []string{"Err(E.A)", "Err(E.B(_))", "Err(E.C {})", "Err(F.A)", "Err(F.B)", "Err(F.C)"} {
			if subset&(1<<index) != 0 {
				arms = append(arms, pattern+" => {}")
			}
		}
		source := prelude + "\nfn Use() void {\n    match Open() {\n        " + strings.Join(arms, "\n        ") + "\n    }\n}\n"
		errs := analyzeSourceRaw(t, source)
		if len(errs) != 1 || !strings.Contains(errs[0].Message, "non-exhaustive match for Result[int, error]: concrete error arms cannot cover the open error domain") {
			t.Errorf("open subset %06b: %v", subset, errs)
		}
		withFallback := strings.Replace(source, "\n    }\n}\n", "\n        Err(_) => {}\n    }\n}\n", 1)
		if errs := analyzeSourceRaw(t, withFallback); len(errs) != 0 {
			t.Errorf("open subset %06b with fallback: %v", subset, errs)
		}
	}
}
