package sema

import (
	"strings"
	"testing"
)

// An early exit guarded by char/rune comparisons against code-point literals
// (`48r`, 'A') leaves the negated bound as a path fact, so the integer
// conversion of the guarded value needs no try (types.md "Explicit
// conversions"); a bound that still admits a negative result keeps S1139.
func TestRuneGuardBoundsIntegerConversion(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Digits(text: string) uint {
    let mut total: uint := 0
    for character in text {
        if character < 48r || character > 57r {
            break
        }
        let digit := uint(int(character) - 48)
        total = digit
    }
    return total
}

fn Upper(character: rune) uint {
    if character < 'A' || character > 'Z' {
        return 0
    }
    return uint(int(character) - 65)
}

fn StillBad(character: rune) uint {
    if character < 48r {
        return 0
    }
    return uint(int(character) - 49)
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Error(), "converting int to uint may fail with ConversionError.OutOfRange") || !strings.Contains(errors[0].Error(), "27:12") {
		t.Fatalf("only StillBad's conversion should require try, got: %v", errors)
	}
}
