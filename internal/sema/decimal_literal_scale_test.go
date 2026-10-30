package sema

import "testing"

// A decimal literal fits its target when some exact legal scale gives a
// coefficient in the target width: written trailing zeros may move to a
// coarser (possibly negative) scale, but significant digits never round.
//
// Rules:
//   - rules/types/types.md — "Decimal scale"; rules/memory/layout.md — § 19
//   - rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §§ 3(4), 3(7), 14 DEC-02
func TestDecimalLiteralFitsTargetWidth(t *testing.T) {
	for name, test := range map[string]struct {
		body  string
		valid bool
	}{
		"zeros strip for decimal":     {"fn Read() decimal {\n    return 10000000000000000000000.0m\n}\n", true},
		"large exponent":              {"fn Read() decimal {\n    return 1e400m\n}\n", true},
		"significant digits too wide": {"fn Read() decimal {\n    return 12345678901234567890123.5m\n}\n", false},
		"decimal128 is wider":         {"fn Read() decimal128 {\n    return 12345678901234567890123.5m\n}\n", true},
		"minimum coefficient":         {"fn Read() decimal {\n    return -9223372036854775808m\n}\n", true},
		"one past maximum":            {"fn Read() decimal {\n    return 9223372036854775809m\n}\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n"+test.body)
			if test.valid != (len(errors) == 0) {
				t.Fatalf("valid=%t, errors=%v", test.valid, errors)
			}
		})
	}
}

// An unrepresentable decimal literal gets a diagnostic that names the
// literal and the representation it lacks, not "decimal versus decimal".
// Rules: rules/errors/errorhandling.md — § 30; MD-066 § 3(7).
func TestDecimalLiteralWidthDiagnostic(t *testing.T) {
	errors := analyzeSourceRaw(t, "module main\nfn Read() decimal {\n    let x: decimal := -12345678901234567890123.5m\n    return x\n}\n")
	if len(errors) != 1 || errors[0].Message != "decimal literal (-12345678901234567890123.5m) has no exact decimal representation" {
		t.Fatalf("got %v", errors)
	}
}
