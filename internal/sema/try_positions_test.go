package sema

import (
	"os"
	"testing"
)

// try is usable wherever its success type is valid: call arguments, if and
// while conditions, aggregate field initializers, array elements, nested and
// parenthesized operands, logical operands, and unary operands.
//
// Rules:
//   - rules/errors/errorhandling.md — try expression positions
//   - rules/control-flow/flowcontrol_if.md — §11; rules/control-flow/flowcontrol_while.md — §7
func TestTryExpressionPositionsFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/try_expression_positions_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	assertSemaErrors(t, analyzeSourceRaw(t, string(input)), nil)
}

// A bodyless try condition whose success value is not bool uses the focused
// rulebook wording.
func TestTryConditionMustProduceBool(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

enum ParseError error {
    Bad,
}

fn Parse() Result[int, ParseError] {
    return Ok(1)
}

fn Use() Result[int, ParseError] {
    if try Parse() {
        return Ok(1)
    }
    while try Parse() {
        break
    }
    return Ok(0)
}
`)
	assertSemaErrors(t, errors, []string{
		"try expression in if condition must produce bool, got int at 13:8",
		"try expression in while condition must produce bool, got int at 16:11",
	})
}
