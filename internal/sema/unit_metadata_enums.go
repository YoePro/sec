package sema

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// unitMetadataEnumVariants defines the exact closed member lists of the six
// core unit metadata enums, shared by standalone Sema and the trusted core
// declarations in sec/core/unit_metadata.sec.
// Rules: rules/library/core-library.md — §§ 27.1–27.4;
// rules/types/units.md — "Unit metadata value types" (3) (MD-062).
func unitMetadataEnumVariants(name string) []string {
	switch name {
	case "UnitKind":
		return []string{
			"Unknown", "Other",
			"Length", "Mass", "Time", "ElectricCurrent", "ThermodynamicTemperature", "AmountOfSubstance", "LuminousIntensity",
			"Count", "Ratio", "Probability", "MonetaryAmount", "InformationAmount", "InformationRate",
			"Area", "Volume", "PlaneAngle", "SolidAngle",
			"Speed", "Velocity", "Acceleration", "Jerk", "RotationalFrequency", "AngularVelocity", "AngularAcceleration",
			"Force", "Pressure", "Stress", "MassDensity", "Energy", "Work", "Heat", "Torque", "Power", "ApparentPower", "ReactivePower",
			"Frequency", "MassFlowRate", "VolumeFlowRate",
			"ElectricCharge", "ElectricPotential", "ElectromotiveForce", "ElectricFieldStrength", "ElectricalResistance",
			"ElectricalConductance", "ElectricalImpedance", "ElectricalReactance", "Capacitance", "Inductance",
			"MagneticFieldStrength", "MagneticFlux", "MagneticFluxDensity",
			"Entropy", "HeatCapacity", "SpecificHeatCapacity", "ThermalConductivity",
			"Concentration", "AmountConcentration", "CatalyticActivity",
			"LuminousFlux", "Illuminance", "Luminance",
			"Radioactivity", "AbsorbedDose", "DoseEquivalent",
		}
	case "UnitSystem":
		return []string{
			"Unspecified", "Other", "SI", "CGS", "CGSGaussian", "CGSElectrostatic", "CGSElectromagnetic", "IEC",
			"Imperial", "USCustomary", "Nautical", "Astronomical", "Natural", "Planck", "Atomic", "Currency", "Domain",
			"Historical",
		}
	case "UnitOrigin":
		return []string{"None", "Zero", "One", "AbsoluteZero"}
	case "UnitTransform":
		return []string{"Linear", "Affine", "Logarithmic"}
	case "UnitCategory":
		return []string{"Physical", "Currency", "Information", "Ratio", "Other"}
	case "UnitStatus":
		return []string{"Active", "Deprecated", "Obsolete"}
	}
	return nil
}

// builtinUnitMetadataEnumType reserves one nominal, copyable core enum
// identity whose default is its first member, like any ordinary enum.
// Rules: rules/library/core-library.md — § 27; rules/types/default_values.md — "Enums".
func builtinUnitMetadataEnumType(name string) Type {
	values := unitMetadataEnumVariants(name)
	return Type{Name: name, Module: "core", Kind: EnumType, Named: true, Declared: true,
		Intrinsic: true, Underlying: "int", EnumValues: values,
		EnumConsts: builtinEnumConsts(values), EnumDefault: values[0]}
}

// unitMetadataEnumFromDeclaration accepts only the exact canonical core
// declaration of a unit metadata enum, so Sema, Semantic IR and the LSP share
// one identity with the core source and no shadow enum exists.
// Rules: rules/library/core-library.md — § 27;
// rules/types/units.md — "Unit metadata value types" (3).
func (a *Analyzer) unitMetadataEnumFromDeclaration(enum *ast.EnumDeclaration) (Type, bool) {
	name := enum.Name.Value
	if len(unitMetadataEnumVariants(name)) == 0 || !a.isTrustedCoreBuiltinDeclaration(name, enum.Name.Token) {
		return Type{}, false
	}
	return a.exactCoreEnumDeclaration(enum, builtinUnitMetadataEnumType(name), "rules/library/core-library.md § 27"), true
}

// rejectUnitMetadataEnumNonEnum prevents trusted core from replacing a unit
// metadata enum identity with another kind of type of the same spelling.
// Rules: rules/library/core-library.md — § 27.
func (a *Analyzer) rejectUnitMetadataEnumNonEnum(name string, token lexer.Token) bool {
	if len(unitMetadataEnumVariants(name)) == 0 || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "%s requires its canonical core enum declaration", name)
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}

// unitMetadataEnumFields names the standard enum of each enum-backed
// metadata key (normalized spelling).
// Rules: rules/types/units.md — "Unit metadata value types" (1), (4).
var unitMetadataEnumFields = map[string]string{
	"kind": "UnitKind", "system": "UnitSystem", "origin": "UnitOrigin",
	"transform": "UnitTransform", "status": "UnitStatus",
}

// unitMetadataLegacyMember maps a legacy bare metadata value of key (normalized
// spelling) to its standard enum member, using the one canonical migration
// table: the snake_case spelling of every UnitKind, UnitOrigin, UnitTransform
// and UnitStatus member, the exact member spelling of every UnitSystem member
// (the form Sec 0.1 catalogs use), and `temperature` for
// UnitKind.ThermodynamicTemperature. Any other bare token has no mapping.
// Rules: rules/types/units.md — "Unit metadata value types" (5);
// rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — § 5(2)–5(3).
func unitMetadataLegacyMember(key, value string) (string, bool) {
	enum := unitMetadataEnumFields[key]
	if enum == "" {
		return "", false
	}
	if enum == "UnitKind" && value == "temperature" {
		return "ThermodynamicTemperature", true
	}
	for _, member := range unitMetadataEnumVariants(enum) {
		if enum == "UnitSystem" && value == member || enum != "UnitSystem" && value == snakeCaseMember(member) {
			return member, true
		}
	}
	return "", false
}

// snakeCaseMember spells a PascalCase enum member in the legacy lowercase
// underscore form, for example ElectricCurrent as electric_current.
func snakeCaseMember(member string) string {
	var out strings.Builder
	for i, r := range member {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out.WriteByte('_')
		}
		out.WriteRune(r)
	}
	return strings.ToLower(out.String())
}
