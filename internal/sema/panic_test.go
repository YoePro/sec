package sema

import "testing"

// rules/errors/panic.md § 15.2 requires bool exactly and defines no truthiness
// conversion for assertion conditions.
func TestAssertConditionMustBeBool(t *testing.T) {
	assertSemaErrors(t, analyzeSourceRaw(t, `
module main

fn Good(ready: bool) void {
	assert ready
	assert ready, "still ready"
}

fn Bad(value: int) void {
	assert value
}
`), []string{
		"assert condition must be bool, got int at 10:9",
	})
}
