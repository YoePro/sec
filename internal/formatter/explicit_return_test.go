package formatter

import "testing"

// Preserve the optional return marker, attach its operand and remain
// idempotent (rules/tooling/formatter.md — §21(5), §21(7)).
func TestFormatExplicitReturn(t *testing.T) {
	assertFormat(t,
		"fn Sum(value: int) int {\nreturn   <-   value+1\n}\n",
		"fn Sum(value: int) int {\n    return <-value + 1\n}\n")
	assertFormat(t,
		"fn Sum(value: int) int {\nreturn value+1\n}\n",
		"fn Sum(value: int) int {\n    return value + 1\n}\n")
}
