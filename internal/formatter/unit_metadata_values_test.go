package formatter

import "testing"

// Typed unit metadata values, legacy catalog names, exact written numeric
// metadata, `BaseUnit` and metadata reads keep their spelling and resolved
// meaning; formatting them is idempotent.
//
// Rules:
//   - rules/types/units.md — "Formatter requirements"
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 5(4), 12(10), 13(8)
func TestFormatKeepsUnitMetadataValues(t *testing.T) {
	source := "module main\n\nunit J physical\n\nimpl <J> {\n    LongName: \"joule\"\n    Dimension: [mass^1, length^2, time^-2]\n" +
		"    Kind: UnitKind.Energy\n    System: UnitSystem.SI\n    BaseUnit: false\n    Status: UnitStatus.Active\n}\n\n" +
		"unit minute physical\n\nimpl <minute> {\n    Dimension: [time^1]\n    Kind: time\n    System: SI\n    Scale: 1 / 60\n}\n\n" +
		"fn Same() bool {\n    return <J>.Kind == UnitKind.Energy && <minute>.Dimension != <J>.Dimension\n}\n"
	assertFormat(t, source, source)
}
