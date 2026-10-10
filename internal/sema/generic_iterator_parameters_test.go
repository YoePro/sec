package sema

import (
	"strings"
	"testing"
)

const genericIteratorPrelude = `module main

type Words struct {
    index: int,
}

impl Words implements Iterator[string] {
    fn Next() Option[string] {
        if self.index >= 2 {
            return None
        }
        self.index += 1
        return Some("w")
    }
}

type Plain struct {
    index: int,
}
`

// A generic parameter constrained by the compiler-known Iterator[T] is a for
// source: the element type and Next requirement come from the constraint and
// each specialization resolves the concrete Next statically. A ref mut I
// parameter infers I from an owned mutable argument through the implicit
// call-site borrow.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §37 "Compiler-known `Iterator[T]`"
//   - rules/declarations/generics.md — §15 "Operations available on generic parameters"
//   - rules/memory/borrowing.md — §15.2 "Call-site borrow creation"
func TestGenericIteratorParametersAreForSources(t *testing.T) {
	source := genericIteratorPrelude + `
fn Count[I: Iterator[string]](values: I) int {
    let mut total := 0
    for value in values {
        let text: string := value
        discard text
        total += 1
    }
    return total
}

fn Drain[I: Iterator[string]](values: ref mut I) int {
    let mut total := 0
    for value in values {
        discard value
        total += 1
    }
    return total
}

fn Main() int {
    let mut words := Words { index: 0 }
    let drained := Drain(words)
    discard drained
    return Count(Words { index: 0 })
}
`
	if errors := analyzeSourceRaw(t, source); len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}
}

func TestGenericIteratorParameterDiagnostics(t *testing.T) {
	source := genericIteratorPrelude + `
fn Drain[I: Iterator[string]](values: ref mut I) int {
    for value in values {
        let wrong: int := value
        discard wrong
    }
    return 0
}

fn Count[I: Iterator[string]](values: I) int {
    return 0
}

fn Main() int {
    let words := Words { index: 0 }
    let plain := Plain { index: 0 }
    discard Count(plain)
    return Drain(words)
}
`
	errors := analyzeSourceRaw(t, source)
	var messages []string
	for _, err := range errors {
		messages = append(messages, err.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{
		"cannot initialize int with string",
		"type Plain does not satisfy constraint Iterator[string] for I",
		// The return position selects the generic through its expected type;
		// it must still check the implicit mutable borrow.
		"cannot create mutable reference to immutable variable words",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
}
