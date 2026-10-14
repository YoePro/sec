package debuginfo

import "fmt"

// DebugInformationPlacement describes where the primary source-debug payload
// belongs. Separate may still retain image-correlation metadata in the image.
// These are internal build concepts, not source declarations or format names.
// Rules: rules/compiler/debug_information.md — §49(1)-(5).
type DebugInformationPlacement uint8

const (
	Embedded DebugInformationPlacement = iota
	Separate
)

// Validate rejects unknown placement values without replacing the requested
// location. Target support is a separate planning obligation.
// Rules: rules/compiler/debug_information.md — §§49(2),(4),70(1),(3).
func (placement DebugInformationPlacement) Validate() error {
	switch placement {
	case Embedded, Separate:
		return nil
	default:
		return fmt.Errorf("invalid DebugInformationPlacement %d: expected Embedded or Separate", uint8(placement))
	}
}

// String renders the internal placement name; invalid values retain their
// identity rather than being rendered as an apparently valid default.
// Rules: rules/compiler/debug_information.md — §§49(2),70(3).
func (placement DebugInformationPlacement) String() string {
	switch placement {
	case Embedded:
		return "Embedded"
	case Separate:
		return "Separate"
	default:
		return fmt.Sprintf("DebugInformationPlacement(%d)", uint8(placement))
	}
}

// ParseDebugInformationPlacement accepts canonical compiler-internal names.
// CLI/manifest spelling and target physical formats are not decided here.
// Rules: rules/compiler/debug_information.md — §§3(3),49(2)-(4).
func ParseDebugInformationPlacement(name string) (DebugInformationPlacement, error) {
	switch name {
	case "Embedded":
		return Embedded, nil
	case "Separate":
		return Separate, nil
	default:
		return Embedded, fmt.Errorf("invalid DebugInformationPlacement %q: expected Embedded or Separate", name)
	}
}

// MarshalText validates a placement before writing internal metadata, without
// asserting that an artifact has been generated or the target supports it.
// Rules: rules/compiler/debug_information.md — §§49,70(1),(3).
func (placement DebugInformationPlacement) MarshalText() ([]byte, error) {
	if err := placement.Validate(); err != nil {
		return nil, err
	}
	return []byte(placement.String()), nil
}

// UnmarshalText commits only a valid canonical placement; a failed read leaves
// the previous selection intact rather than falling back to Embedded.
// Rules: rules/compiler/debug_information.md — §§49(2),70(3).
func (placement *DebugInformationPlacement) UnmarshalText(text []byte) error {
	parsed, err := ParseDebugInformationPlacement(string(text))
	if err != nil {
		return err
	}
	*placement = parsed
	return nil
}

// Selection carries the independent debug-level and placement axes. None with
// Separate remains a valid internal selection, even though None requires no
// source-debug payload; neither axis silently rewrites the other. Validation
// alone does not establish target support, emission or companion correlation.
// Rules: rules/compiler/debug_information.md — §§4(1),49(1)-(5),70(1)-(3).
type Selection struct {
	Level     DebugInformationLevel
	Placement DebugInformationPlacement
}

// Validate checks each independent selection domain without deriving level
// from placement or placement from level.
// Rules: rules/compiler/debug_information.md — §§3(1),49(1)-(2),70(2)-(3).
func (selection Selection) Validate() error {
	if err := selection.Level.Validate(); err != nil {
		return err
	}
	return selection.Placement.Validate()
}
