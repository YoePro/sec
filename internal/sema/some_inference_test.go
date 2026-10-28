package sema

import (
	"strings"
	"testing"
)

// MD-102: Some(value) eliminates overloads by its payload, supplies T for
// Option[T], and has the standalone type Option[T] when its payload is
// independently typeable (correction md029-md042-md102 §§ 11–14, M102-01..13).
func TestSomePayloadSelectsOverloadAndInfersGenerics(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

fn Store(value: Option[int]) int {
    return 1
}

fn Store(value: Option[string]) int {
    return 2
}

fn Process[T](value: Option[T]) void {
}

fn Unwrap[T](value: Option[T], fallback: T) T {
    return match value {
        Some(item) => item
        None => fallback
    }
}

fn Calls(text: string) int {
    discard Store(Some(42))
    discard Store(Some(text))
    Process(Some(42))
    Process(Some("text"))
    let fromFallback := Unwrap(None, 0)
    let fromPayload := Unwrap(Some(42), 0)
    return fromFallback + fromPayload
}
`)
	if len(errs) != 0 {
		t.Fatalf("valid Some calls rejected: %v", errs)
	}
}

func TestStandaloneSomeInfersOptionType(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

fn Use(text: string) void {
    let number := Some(42)
    let name := Some(text)
    let nested := Some(Some(1))
    let wide: Option[int64] := Some(42)
    let typedNested: Option[Option[int]] := Some(None)
    let numberCheck: Option[int] := number
    let nameCheck: Option[string] := name
    let nestedCheck: Option[Option[int]] <- nested
    discard numberCheck
    discard nameCheck
    discard nestedCheck
    discard wide
    discard typedNested
}
`)
	if len(errs) != 0 {
		t.Fatalf("standalone Some rejected: %v", errs)
	}
	mismatch := analyzeSourceRaw(t, `
module main

fn Use() void {
    let number := Some(42)
    let wrong: Option[string] := number
    discard wrong
}
`)
	if len(mismatch) == 0 {
		t.Fatalf("Some(42) is Option[int] and must not initialize Option[string]")
	}
}

func TestSomeInferenceRejections(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

fn Store(value: Option[string]) void {
}

fn Store(value: Option[bool]) void {
}

fn Narrow(value: Option[int32]) void {
}

fn Narrow(value: Option[int64]) void {
}

fn Use() void {
    let missing := Some(None)
    let small: Option[uint8] := Some(300)
    Store(Some(42))
    Narrow(Some(42))
}
`)
	for _, want := range []string{
		"cannot infer the Option type of Some(None)",
		"300",
		"Option[string].Some payload must be string, got int",
		"ambiguous call to Narrow: Some(42) could be Option[int32] or Option[int64]",
	} {
		found := false
		for _, err := range errs {
			found = found || strings.Contains(err.Error(), want)
		}
		if !found {
			t.Errorf("missing %q in %v", want, errs)
		}
	}
}

// A visible user callable named Some is an ordinary call, never an Option
// constructor (M102-11).
func TestUserSomeFunctionIsNotOptionConstruction(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

fn Some(value: int) int {
    return value
}

fn Use() int {
    let plain := Some(3)
    return plain + 1
}
`)
	if len(errs) != 0 {
		t.Fatalf("user Some function rejected: %v", errs)
	}
}
