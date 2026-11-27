package sema

import (
	"os"
	"sec/internal/diagnostics"
	"testing"
)

// A2012 publishes detached numeric facts, including capacity reduction and
// a related range location, without parsing its human explanation.
// Rules: rules/tooling/diagnostics.md — §31A; MD-049 — §15(1).
func TestDecimalScaleDiagnosticArguments(t *testing.T) {
	for _, test := range []struct{ source, width, preferred, selected, reduced, limit string }{
		{"ordinary.sec", "64", "3", "3", "false", ""},
		{"reduced.sec", "64", "3", "-1", "true", "10000000000000000000"},
		{"wide.sec", "128", "3", "3", "false", ""},
	} {
		source, err := os.ReadFile("../../testdata/sema/decimal_scale_diagnostics/" + test.source)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeScaleTypes(t, string(source))
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		warnings := a.Warnings()
		if len(warnings) != 1 || warnings[0].ID != diagnostics.InferredDecimalScale {
			t.Fatal(warnings)
		}
		args := warnings[0].Arguments
		if args["type"] == "" || args["carrier_width"] != test.width || args["preferred_scale"] != test.preferred || args["selected_scale"] != test.selected || args["capacity_reduced"] != test.reduced || args["limiting_bound"] != test.limit {
			t.Fatal(args)
		}
		if test.reduced == "true" && len(warnings[0].RelatedLocations()) != 1 {
			t.Fatal("missing limiting range location")
		}
		args["selected_scale"] = "changed"
		if a.Warnings()[0].Arguments["selected_scale"] != test.selected {
			t.Fatal("mutable diagnostic arguments escaped")
		}
	}
}
