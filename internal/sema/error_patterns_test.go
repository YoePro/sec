package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// MD-029 payload, field and contextual error patterns in Result match arms
// and try handlers (correction md029-md042-md102 § 17: M029-02/03/05/07/09/10).
//
// Rules:
//   - rules/errors/errorhandling.md — §§ 15.3, 20, 27.2b
//   - rules/foundations/grammar.md — "Try expression" (ErrPattern)
func TestErrorPayloadPatterns(t *testing.T) {
	analyzer, errs := analyzeSourceWithAnalyzerRaw(t, `
module main

type FileError union error {
    NotFound {
        Path: string,
        Code: int
    }
    PermissionDenied
    InvalidEncoding(uint)
}

fn Open(flag: bool) Result[int, FileError] {
    if flag {
        return Err(FileError.PermissionDenied)
    }
    return Ok(1)
}

fn OpenAny(flag: bool) Result[int, error] {
    return Ok(1)
}

fn ViaMatch(flag: bool) uint {
    match Open(flag) {
        Ok(value) => return 0
        Err(FileError.NotFound { Code: code }) where code > 0 => return 1
        Err(.NotFound {}) => return 2
        Err(.PermissionDenied) => return 3
        Err(FileError.InvalidEncoding(position)) => return position
    }
}

fn ViaTry(flag: bool) int {
    let value := try Open(flag) {
        Err(.NotFound { Path: path, Code }) => Code
        Err(FileError.InvalidEncoding(_)) => 2
        Err(.PermissionDenied) => 3
    }
    return value
}

fn Narrowed(flag: bool) int {
    match OpenAny(flag) {
        Ok(value) => return value
        Err(FileError.NotFound { Code: code }) => return code
        Err(other) => return 0
    }
}

fn Borrowed(flag: bool) int {
    let result := Open(flag)
    match result {
        Ok(value) => return value
        Err(.NotFound { Code: code, Path: ref path }) => return code
        Err(.InvalidEncoding(ref position)) => return 1
        Err(_) => return 0
    }
}
`)
	if len(errs) != 0 {
		t.Fatalf("valid MD-029 error patterns rejected: %v", errs)
	}
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.OmittedPayloadPattern {
			t.Fatalf("explicit or binding patterns must not report omitted payloads: %+v", warning)
		}
	}
}

// Focused rejections (M029-08/11/12/13): contextual selection on open or
// heterogeneous channels, unknown variants and fields, payload shapes the
// declared variant does not have, borrows from the consumed try Result and
// hidden partial moves out of a reusable Result.
func TestErrorPayloadPatternRejections(t *testing.T) {
	errs := analyzeSourceRaw(t, `
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

enum Small error {
    A
}

fn Load(flag: bool) Result[int, LoadError] {
    return Ok(1)
}

fn LoadAny(flag: bool) Result[int, error] {
    return Ok(1)
}

fn LoadSmall() Result[int, Small] {
    return Ok(1)
}

fn Contextual(flag: bool) int {
    match LoadAny(flag) {
        Ok(value) => return value
        Err(.Stop) => return 1
        Err(e) => return 2
    }
}

fn Shapes(flag: bool) int {
    match Load(flag) {
        Ok(value) => return value
        Err(.Missing) => return 1
        Err(.Bad { Size: s }) => return 2
        Err(.Bad { Code: a, Code: b }) => return 3
        Err(.Stop(x)) => return 4
        Err(_) => return 5
    }
}

fn EnumPayload() int {
    match LoadSmall() {
        Ok(value) => return value
        Err(.A(x)) => return 1
    }
}

fn TryBorrow(flag: bool) int {
    let value := try Load(flag) {
        Err(.Bad { Code: ref code }) => 1
        Err(_) => 0
    }
    return value
}

fn PartialMove(flag: bool) int {
    let result := Load(flag)
    match result {
        Ok(value) => return value
        Err(.Busy(handle)) => return 1
        Err(_) => return 0
    }
}
`)
	for _, want := range []string{
		"Err(.Stop) needs one closed concrete error type, but this failure channel is the open error channel",
		"LoadError has no variant Missing",
		"union variant LoadError.Bad has no field Size",
		"field Code is bound more than once in this pattern",
		"error variant LoadError.Stop carries no payload to bind",
		"enum error variant Small.A carries no payload to bind",
		"cannot borrow union payload field from temporary match subject",
		"the Handle payload of LoadError.Busy is move-only; binding it by value would partially move the Result",
	} {
		found := false
		for _, err := range errs {
			found = found || strings.Contains(err.Message, want)
		}
		if !found {
			t.Errorf("missing %q in %v", want, errs)
		}
	}
}

// A heterogeneous try failure set supplies no single concrete error type for
// `.Variant`, even when only one member declares the spelling (M029-08).
func TestContextualErrorVariantNeedsOneConcreteType(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

fn Use(values: int[], index: uint, amount: int) int {
    return try values[index] + amount {
        Err(.OutOfBounds) => 0
        Err(_) => 1
    }
}
`)
	found := false
	for _, err := range errs {
		found = found || strings.Contains(err.Message, "Err(.OutOfBounds) needs one closed concrete error type, but this failure channel is a heterogeneous failure set")
	}
	if !found {
		t.Fatalf("contextual selection over a heterogeneous failure set accepted: %v", errs)
	}
}
