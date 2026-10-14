package debuginfo

import (
	"encoding/json"
	"testing"
)

// TestDebugPlacementDomain ensures that physical format names, extra placement
// values and malformed metadata cannot become a valid placement silently.
// Rules: rules/compiler/debug_information.md — §§49(2)-(4),70(3).
func TestDebugPlacementDomain(t *testing.T) {
	for value := 0; value <= 255; value++ {
		placement := DebugInformationPlacement(value)
		valid := placement == Embedded || placement == Separate
		if err := placement.Validate(); (err == nil) != valid {
			t.Fatalf("placement %d: %v", value, err)
		}
		if !valid {
			if data, err := json.Marshal(placement); err == nil || data != nil {
				t.Fatalf("invalid placement encoded: %d %s %v", value, data, err)
			}
			if placement.String() == Embedded.String() || placement.String() == Separate.String() {
				t.Fatalf("invalid placement rendered as valid: %d", value)
			}
		}
	}
	for _, text := range []string{"", "embedded", "separate", "Split", "DWARF", "CodeView", "Separate ", " Embedded", "0", "1"} {
		placement := Separate
		if err := placement.UnmarshalText([]byte(text)); err == nil || placement != Separate {
			t.Fatalf("%q: %v, %v", text, err, placement)
		}
	}
}

// TestDebugPlacementIndependentFromLevel preserves all six selections through
// internal metadata round trips, including None without inferred stripping and
// Full without an inferred placement or optimization policy.
// Rules: rules/compiler/debug_information.md — §§2(3)-(5),4(1)-(4),49(1)-(5).
func TestDebugPlacementIndependentFromLevel(t *testing.T) {
	for _, level := range []DebugInformationLevel{None, LineTables, Full} {
		for _, placement := range []DebugInformationPlacement{Embedded, Separate} {
			selection := Selection{Level: level, Placement: placement}
			if err := selection.Validate(); err != nil {
				t.Fatal(selection, err)
			}
			data, err := json.Marshal(selection)
			if err != nil {
				t.Fatal(err)
			}
			var decoded Selection
			if err := json.Unmarshal(data, &decoded); err != nil || decoded != selection {
				t.Fatal(selection, decoded, err)
			}
		}
	}
	for _, selection := range []Selection{
		{Level: DebugInformationLevel(255), Placement: Separate},
		{Level: Full, Placement: DebugInformationPlacement(255)},
	} {
		if err := selection.Validate(); err == nil {
			t.Fatal("invalid selection accepted", selection)
		}
	}
}
