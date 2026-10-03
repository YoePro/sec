package main

import (
	"strings"
	"testing"

	"sec/internal/sema"
)

// Property hover and completion consume Sema's resolved accessor and error
// contract rather than presenting a fallible setter as an ordinary field.
//
// Rules:
//   - rules/declarations/properties.md — §§ 4–6
//   - rules/errors/errorhandling.md — §§ 24 and 36
//   - rules/tooling/lsp.md — "Hover" and "Completion"
func TestFalliblePropertyHoverAndCompletionShowSetterContract(t *testing.T) {
	source := `module main

enum SpeedError error {
    TooHigh,
}

type Vehicle struct {
    speed: int,
}

impl Vehicle {
    property TopSpeed: int {
        get {
            return self.speed
        }
        try set next SpeedError {
            self.speed = next
        }
    }
}

fn Read(vehicle: Vehicle) int {
    return vehicle.TopSpeed
}
`
	for name, offset := range map[string]int{
		"declaration": strings.Index(source, "TopSpeed: int"),
		"use":         strings.LastIndex(source, "TopSpeed"),
	} {
		t.Run(name, func(t *testing.T) {
			hover, ok := hoverForSource("", source, offsetPosition(source, offset+1))
			if !ok {
				t.Fatalf("missing property hover at %s", name)
			}
			for _, want := range []string{
				"property TopSpeed: int",
				"Accessors: `get`, `try set`",
				"Setter error: `SpeedError`",
				"Assignment requires: `try`",
			} {
				if !strings.Contains(hover.Contents.Value, want) {
					t.Fatalf("property hover at %s missing %q:\n%s", name, want, hover.Contents.Value)
				}
			}
		})
	}

	completionSource := strings.Replace(source, "return vehicle.TopSpeed", "vehicle.\n    return 0", 1)
	offset := strings.Index(completionSource, "vehicle.\n") + len("vehicle.")
	items := completeSource("", completionSource, offset)
	for _, item := range items {
		if item.Label != "TopSpeed" {
			continue
		}
		if item.Detail != "int (get, try set) error SpeedError" {
			t.Fatalf("TopSpeed completion detail = %q", item.Detail)
		}
		return
	}
	t.Fatalf("TopSpeed missing from completion: %+v", items)
}

func TestOrdinaryPropertyHoverDoesNotClaimFallibility(t *testing.T) {
	property := propertyHoverContents(sema.Property{
		Name: "Count", Type: sema.Type{Name: "int", Kind: sema.IntType}, HasGetter: true, HasSetter: true,
	})
	if !strings.Contains(property, "Accessors: `get`, `set`") || strings.Contains(property, "Setter error") || strings.Contains(property, "Assignment requires") {
		t.Fatalf("ordinary property hover = %s", property)
	}
}
