package main

import (
	"strings"
	"testing"
)

const callFactHintSource = `module main

unit Meter physical
unit Milli physical

impl <Meter> {
	Dimension: [length^1]
	Kind: length
	Scale: 1
}

impl <Milli> {
	Dimension: [length^1]
	Kind: length
	Scale: 1 / 1000
}

type Percent int range 0..100

fn Describe(value: int) int { return value }
fn Describe(value: bool) int { return 1 }

fn Grow() Result[void, CollectionError] {
	let mut values: int[]
	try values.Append(1)
	return Ok()
}

@target(os: "linux")
fn Platform() int { return 1 }

fn Use(a: int64<Meter>, raw: int) Result[int, ConversionError] {
	let n := Describe(3)
	discard Grow()
	let p := try Percent(raw)
	discard Platform()
	if a >= -5 && a <= 5 {
		let b: int64<Milli> := a
		discard b
	}
	return Ok(n + int(p))
}
`

// TestInlayCallFactHints: the remaining § 17(7b) categories show facts the
// source does not spell, each switchable: the selected overload's parameter
// types, an implicit unit conversion with its exact factor, a constrained
// conversion's runtime checks, `allocates` for a callee Sema proves may
// allocate, and the @target selection of the called declaration.
// Rules: rules/tooling/lsp.md — § 17(1), § 17(7b).
func TestInlayCallFactHints(t *testing.T) {
	labels := func(settings inlayHintSettings) []string {
		var out []string
		for _, hint := range inlayHintsForSource("file:///tmp/call_facts.sec", callFactHintSource, allLines(), settings) {
			out = append(out, hint.Label)
		}
		return out
	}
	all := strings.Join(labels(defaultInlayHintSettings()), " | ")
	for _, want := range []string{"overload(int)", "Meter→Milli ×1000", "checks range 0..100", "allocates", `@target(os: "linux")`} {
		if !strings.Contains(all, want) {
			t.Fatalf("missing %q in %s", want, all)
		}
	}
	off := defaultInlayHintSettings()
	off.Overloads, off.Units, off.Contracts, off.Allocation, off.Targets = false, false, false, false, false
	none := strings.Join(labels(off), " | ")
	for _, gone := range []string{"overload(", "→", "checks ", "allocates", "@target"} {
		if strings.Contains(none, gone) {
			t.Fatalf("%q survived its switched-off setting: %s", gone, none)
		}
	}
	settings := inlayHintSettingsFrom(defaultInlayHintSettings(), []byte(`{"sec":{"inlayHints":{"allocation":false,"units":false}}}`))
	if settings.Allocation || settings.Units || !settings.Contracts || !settings.Targets || !settings.Overloads {
		t.Fatalf("configuration payload: %+v", settings)
	}
}
