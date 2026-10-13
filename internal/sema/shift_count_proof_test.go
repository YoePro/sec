package sema

import (
	"strings"
	"testing"
)

// A shift whose count is a compile-time constant inside 0..<width cannot fail
// unless it is a signed left shift that may overflow, so it is no try failure
// point: Err(error) binds only the other failures of the protected
// expression, a try over the shift alone contains no failure, and the shift
// has no arithmetic panic effect. A variable count stays checked.
//
// Rules:
//   - rules/foundations/operators.md — "Shift count", "Unsigned left shift", "Right shift"
//   - rules/errors/runtime_checks.md — "What try converts"
func TestConstantShiftCountIsNotAFailurePoint(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type EncodeError union error {
    Allocation(CollectionError),
}

fn Digit(value: byte) byte {
    return value
}

fn Encode(current: byte, signed: int) Result[void, EncodeError] {
    let mut result: byte[]
    try result.Append(Digit((current >> 4) & 0x0F)) {
        Err(error) => return Err(EncodeError.Allocation(error))
    }
    try result.Append(Digit(current << 3)) {
        Err(error) => return Err(EncodeError.Allocation(error))
    }
    let shifted := signed >> 2
    let mut flags := current
    flags >>= 1
    return Ok()
}
`)
	if len(errors) != 0 {
		t.Fatalf("constant in-range shifts must not be failure points: %+v", errors)
	}

	errors = analyzeSourceRaw(t, `
module main

fn Shift(value: byte, count: int) Result[byte, ArithmeticError] {
    let checked := try value >> count
    let proven := try value >> 3
    return Ok(checked)
}
`)
	var messages []string
	for _, err := range errors {
		messages = append(messages, err.Message)
	}
	joined := strings.Join(messages, "\n")
	if len(errors) != 1 || !strings.Contains(joined, "value >> 3") {
		t.Fatalf("want only the proven constant shift rejected as containing no failure, got:\n%s", joined)
	}
}
