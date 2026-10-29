package main

import (
	"os"
	"strings"
	"testing"
)

// TestQuantityAndDurationHover projects exact Sema facts for custom units,
// contextual carrier shaping, constant/bounded/checked conversion and rounding.
// Rules: rules/types/temporal.md — §§5.2–5.7;
// rules/types/units.md — "Quantity literals", "LSP requirements".
func TestQuantityAndDurationHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/duration_hover.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, diagnostic := range analyze("", source) {
		if diagnostic.Severity == 1 {
			t.Fatalf("invalid fixture: %+v", diagnostic)
		}
	}
	for _, test := range []struct {
		marker, selected string
		want             []string
		noPlan           bool
	}{
		{"7<tick>", "7", []string{"quantity: decimal<tick>", "time^1", "exact scale `1/1000`"}, true},
		{"250<tick>", "tick", []string{"quantity: int16<tick>"}, true},
		{"duration(5<tick>)", "duration", []string{"quantity: decimal<tick>", "Duration conversion: `explicit`", "Proof: `constant`", "Nanoseconds per unit: `1000000`", "Nanoseconds: `5000000`", "Result: `duration`"}, false},
		{"contextual: duration := 2<tick>", "2", []string{"Duration conversion: `contextual`", "Nanoseconds: `2000000`"}, false},
		{"negative: duration := -5<tick>", "-", []string{"Duration conversion: `contextual`", "Nanoseconds: `-5000000`"}, false},
		{"duration(500<tiny>, RoundingMode.Ceil)", "duration", []string{"Nanoseconds per unit: `1/1000`", "Rounding: `RoundingMode.Ceil`", "Nanoseconds: `1`"}, false},
		{"duration(count)", "duration", []string{"Proof: `bounded-domain`", "Nanosecond range: `-32768000000 ..= 32767000000`"}, false},
		{"duration(dynamic)", "duration", []string{"Proof: `checked`", "Result: `Result[duration, ConversionError]`", "First possible failure: `ConversionError.PrecisionLoss`", "Failure handling:"}, false},
		{"duration(dynamic, RoundingMode.Ceil)", "duration", []string{"Proof: `checked`", "First possible failure: `ConversionError.OutOfRange`", "Rounding: `RoundingMode.Ceil`"}, false},
		{"duration(elapsed)", "duration", []string{"Proof: `constant`", "Nanoseconds: `3000000`"}, false},
		{"Wait(25<tick>)", "25", []string{"Duration conversion: `contextual`", "Nanoseconds: `25000000`"}, false},
		{"Choose(30<tick>)", "30", []string{"quantity: decimal<tick>"}, true},
	} {
		t.Run(test.marker, func(t *testing.T) {
			offset := strings.Index(source, test.marker) + strings.Index(test.marker, test.selected)
			hover, ok := hoverForSource("", source, offsetPosition(source, offset))
			for _, want := range test.want {
				if !ok || !strings.Contains(hover.Contents.Value, want) {
					t.Fatalf("missing %q: %+v, found=%v", want, hover, ok)
				}
			}
			if test.noPlan && strings.Contains(hover.Contents.Value, "Duration conversion") {
				t.Fatalf("bare quantity implicitly became duration: %+v", hover)
			}
			if test.selected == "duration" && hover.Range != offsetsRange(source, offset, offset+len("duration")) {
				t.Fatalf("conversion hover claimed its argument: %+v", hover.Range)
			}
		})
	}
	for _, marker := range []string{"7<tick>", "250<tick>"} {
		start := strings.Index(source, marker)
		for offset := start; offset < start+len(marker); offset++ {
			hover, ok := hoverForSource("", source, offsetPosition(source, offset))
			if !ok || hover.Range != offsetsRange(source, start, start+len(marker)) {
				t.Fatalf("incomplete quantity range at %d: %+v, %v", offset, hover, ok)
			}
		}
	}
}

// TestInvalidDurationHoverDoesNotInventPlan keeps invalid conversions and
// unresolved units out of successful conversion presentation.
// Rules: rules/types/temporal.md — §§5.4(3),5.7;
// rules/tooling/lsp.md — "Hover".
func TestInvalidDurationHoverDoesNotInventPlan(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/duration_hover_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	hasError := false
	for _, diagnostic := range analyze("", source) {
		hasError = hasError || diagnostic.Severity == 1
	}
	if !hasError {
		t.Fatal("invalid fixture unexpectedly accepted")
	}
	for _, marker := range []string{"duration(1<metre>)", "2<missing>"} {
		hover, _ := hoverForSource("", source, offsetPosition(source, strings.Index(source, marker)))
		if strings.Contains(hover.Contents.Value, "Duration conversion") || strings.Contains(hover.Contents.Value, "Nanoseconds:") {
			t.Fatalf("invented successful duration plan: %+v", hover)
		}
	}
}
