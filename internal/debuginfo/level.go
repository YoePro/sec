// Package debuginfo owns target-neutral compiler debug metadata contracts.
// It is independent from optimization, runtime policy and physical encodings.
// Rules: rules/compiler/debug_information.md — §§2,3.
package debuginfo

import "fmt"

// DebugInformationLevel is the canonical compiler-internal debug selection.
// The zero value is None; this does not strip symbols or discard diagnostic
// provenance. CLI and manifest spellings remain owned by project/build rules.
// Rules: rules/compiler/debug_information.md — §§3(1)-(4),4(1)-(4).
type DebugInformationLevel uint8

const (
	None DebugInformationLevel = iota
	LineTables
	Full
)

// Validate rejects values outside the three canonical levels. Validity of a
// level does not prove that a backend or target can fulfill its contract.
// Rules: rules/compiler/debug_information.md — §§3(1),(4),70(1).
func (level DebugInformationLevel) Validate() error {
	switch level {
	case None, LineTables, Full:
		return nil
	default:
		return fmt.Errorf("invalid DebugInformationLevel %d: expected None, LineTables or Full", uint8(level))
	}
}

// String renders the canonical internal name, preserving an invalid value as
// an explicit invalid identity rather than silently reporting None.
// Rules: rules/compiler/debug_information.md — §§3(1),(4),70.
func (level DebugInformationLevel) String() string {
	switch level {
	case None:
		return "None"
	case LineTables:
		return "LineTables"
	case Full:
		return "Full"
	default:
		return fmt.Sprintf("DebugInformationLevel(%d)", uint8(level))
	}
}

// ParseDebugInformationLevel reads canonical internal names only. This is not
// a CLI/manifest parser and neither adds legacy aliases nor chooses defaults.
// Rules: rules/compiler/debug_information.md — §3(1)-(4).
func ParseDebugInformationLevel(name string) (DebugInformationLevel, error) {
	switch name {
	case "None":
		return None, nil
	case "LineTables":
		return LineTables, nil
	case "Full":
		return Full, nil
	default:
		return None, fmt.Errorf("invalid DebugInformationLevel %q: expected None, LineTables or Full", name)
	}
}

// MarshalText validates internal debug-level metadata before encoding it.
// It does not establish an artifact format or backend support guarantee.
// Rules: rules/compiler/debug_information.md — §§3,70.
func (level DebugInformationLevel) MarshalText() ([]byte, error) {
	if err := level.Validate(); err != nil {
		return nil, err
	}
	return []byte(level.String()), nil
}

// UnmarshalText commits only a valid canonical level, so invalid metadata
// cannot silently erase or downgrade a previously selected contract.
// Rules: rules/compiler/debug_information.md — §§3,70.
func (level *DebugInformationLevel) UnmarshalText(text []byte) error {
	parsed, err := ParseDebugInformationLevel(string(text))
	if err != nil {
		return err
	}
	*level = parsed
	return nil
}
