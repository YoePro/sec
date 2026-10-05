package sema

import (
	"fmt"
	"strings"
	"testing"
)

// Compiler-known attributes whose meaning is not implemented yet are rejected
// on functions, methods, and nominal types instead of being accepted without
// their guarantee or selection.
//
// Rules:
//   - rules/foundations/attributes.md — "Verified guarantee attributes", "Selection attributes", "Target-binding attributes"
func TestUnimplementedAttributesAreRejected(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

@isr
fn A() void {
}

@interrupt(vector: 3)
@noPanic
fn B() void {
}

@target(os: "linux")
type G int

type R struct {}

impl R {
    @interruptSafe
    fn M() void {
    }
}
`)
	assertSemaErrors(t, errors, []string{
		"attribute @isr is not implemented yet at 3:1",
		"attribute @interrupt is not implemented yet at 7:1",
		"attribute @target is not implemented yet at 12:1",
		"attribute @interruptSafe is not implemented yet at 18:5",
	})
}

// Attribute arguments must be known while the compilation is planned and have
// the value form their attribute defines; @address values keep their
// register-binding check.
//
// Rules:
//   - rules/foundations/attributes.md — "Compile-time arguments", "Selector values", "Address argument",
//     "Interrupt vector argument", "@when condition language"
func TestAttributeArgumentsArePlanTimeValues(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn Base() int {
    return 3
}

@interrupt(vector: Base())
fn A() void {
}

@target(os: Name)
fn B() void {
}

@when(config.audit && target.os == "linux")
fn C() void {
}

@interrupt(vector: Interrupt.Timer0)
@when(!config.telemetry && (config.audit || config.debug == true))
fn D() void {
}
`)
	ids := map[string]int{}
	messages := []string{}
	for _, err := range errors {
		ids[err.ID]++
		messages = append(messages, fmt.Sprintf("%s at %d:%d", err.Message, err.Line, err.Column))
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{
		"argument of @interrupt must be known while the compilation is planned; a function call needs run-time execution at 7:20",
		"@target selector values must be string literals at 11:9",
		"@when conditions use config.<name>, true, false, !, &&, ||, ==, and != only at 15:7",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in\n%s", want, joined)
		}
	}
	if ids["S1118"] != 1 || ids["S1114"] != 2 {
		t.Errorf("diagnostic IDs = %v; D's canonical arguments must be accepted\n%s", ids, joined)
	}
}
