package sema

import (
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema/constant"
)

// UnitMetadataRead is the resolved value of one `<U>.Member` metadata read:
// the unit declaration and member it observes, and its typed value. Exactly
// one value field applies, selected by Member. Decimal members keep the exact
// declared rational next to their public decimal spelling; Projected is true
// only for a Scale whose exact value is not a decimal (MD-062 § 9(7)).
// Defaulted marks a declarable member whose value is its omission default.
type UnitMetadataRead struct {
	Unit      string
	Module    string
	Member    string
	Type      Type
	Defaulted bool
	// Declaration is navigation provenance, not a renameable symbol identity.
	// Defaults/implicit values point to the unit; explicit values to their key.
	Declaration lexer.Token
	String      string
	Bool        bool
	Enum        ResolvedEnumCase
	Exact       *big.Rat
	Decimal     string
	Projected   bool
	Dimension   Dimension
}

// ResolvedUnitMetadataReadOf returns the metadata value read by a named-unit
// member expression `<U>.Member` that Sema accepted.
// Rules: rules/types/units.md — "Unit metadata value types";
// rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 11(4)–11(5), 12(7).
func (a *Analyzer) ResolvedUnitMetadataReadOf(expr *ast.MemberExpression) (UnitMetadataRead, bool) {
	if a == nil || expr == nil {
		return UnitMetadataRead{}, false
	}
	read, ok := a.unitMetadataReads[expr]
	return read, ok
}

// ResolvedUnitDimensionOf returns the canonical dimension of expr when it is
// a direct `<U>.Dimension` read, whose value is known at compile time.
// Rules: rules/types/units.md — "Unit metadata value types" (8);
// rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — § 7(12).
func (a *Analyzer) ResolvedUnitDimensionOf(expr ast.Expression) (Dimension, bool) {
	member, ok := expr.(*ast.MemberExpression)
	if !ok {
		return Dimension{}, false
	}
	read, ok := a.ResolvedUnitMetadataReadOf(member)
	if !ok || read.Member != "Dimension" {
		return Dimension{}, false
	}
	return read.Dimension, true
}

// recordUnitMetadataRead binds an accepted read to its resolved value, so
// later phases reuse the declaration facts instead of reinterpreting metadata
// tokens. A decimal member that cannot be materialized as `decimal` fails the
// read, not the unit declaration.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types" (1), (4), (7), (10)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 2, 9(7)–9(8), 10(1), 10(3), 11(4)
func (a *Analyzer) recordUnitMetadataRead(expr *ast.MemberExpression, reference *ast.UnitReferenceExpression, unit UnitDefinition, typ Type) (Type, bool) {
	read := UnitMetadataRead{Unit: reference.Unit, Module: unit.Module, Member: expr.Property.Value, Type: typ}
	if key, declarable := unitMetadataKeys[strings.ToLower(read.Member)]; declarable && key == read.Member {
		read.Defaulted = !unit.Declared[strings.ToLower(read.Member)]
	}
	enum := func(value UnitMetadataEnum, standard, absent string) {
		if value.Type == "" {
			value = UnitMetadataEnum{Type: standard, Member: absent}
		}
		read.Enum, _ = enumCaseNamed(a.types[value.Type], value.Member)
	}
	switch read.Member {
	case "Name":
		read.String = strings.TrimPrefix(unit.Name, unit.Module+".")
	case "LongName":
		read.String = unit.LongName
	case "Symbol":
		read.String = unit.Symbol
	case "BaseUnit":
		read.Bool = unit.IsBaseUnit
	case "Kind":
		enum(unit.KindEnum, "UnitKind", "Unknown")
	case "System":
		enum(unit.SystemEnum, "UnitSystem", "Unspecified")
	case "Origin":
		enum(unit.OriginEnum, "UnitOrigin", "None")
	case "Transform":
		enum(UnitMetadataEnum{}, "UnitTransform", capitalizedUnitFact(string(unit.Transform), "Linear"))
	case "Status":
		enum(UnitMetadataEnum{}, "UnitStatus", capitalizedUnitFact(string(unit.Status), "Active"))
	case "Category":
		enum(UnitMetadataEnum{}, "UnitCategory", capitalizedUnitFact(string(unit.Category), "Other"))
	case "Dimension":
		read.Dimension = unit.Dimension
	case "Scale":
		read.Exact = unit.ScaleValue
		if read.Exact == nil {
			read.Exact = big.NewRat(1, 1)
		}
	case "Offset":
		read.Exact = unit.OffsetValue
		if read.Exact == nil {
			read.Exact = new(big.Rat)
		}
	case "LogBase":
		read.Exact = unit.LogBaseValue
	case "LogFactor":
		read.Exact = unit.LogFactorValue
	case "Reference":
		read.Exact = unit.ReferenceValue
	}
	if read.Exact != nil {
		text, projected, ok := constant.NearestDecimalText(read.Exact, 64)
		if ok && projected && read.Member != "Scale" {
			ok = false
		}
		if !ok {
			a.addErrorAtTokenWithMetadata(expr.Property.Token, "",
				"only a Scale read rounds to the nearest decimal; other metadata must be exactly representable as decimal",
				"%s metadata %s of unit %s cannot be represented as decimal", read.Member, read.Exact.RatString(), reference.String())
			return Type{Kind: InvalidType}, false
		}
		read.Decimal, read.Projected = text, projected
	}
	if a.unitMetadataReads == nil {
		a.unitMetadataReads = map[*ast.MemberExpression]UnitMetadataRead{}
	}
	// Declared values navigate to their accepted metadata key; omission
	// defaults and declaration-derived members navigate to the owning unit.
	// Rules: rules/types/units.md — "LSP requirements"; MD-062 §12(9).
	read.Declaration = unit.Token
	if declared, ok := unit.MetadataTokens[normalizeUnitMetadataName(read.Member)]; ok {
		read.Declaration = declared
		a.setDefinitions(expr.Property.Token, declared)
	}
	a.unitMetadataReads[expr] = read
	return typ, true
}

// capitalizedUnitFact maps an internal lower-case category, status or
// transform fact to its core enum member, or to the default when absent.
// Rules: rules/library/core-library.md — §§ 27.3–27.4.
func capitalizedUnitFact(value, absent string) string {
	if value == "" {
		return absent
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

// UnitMetadataReadAt returns the metadata read whose member name starts at
// the given source position, for editor hover.
// Rules: rules/tooling/lsp.md — "Hover"; rules/types/units.md — "Unit metadata value types" (11).
func (a *Analyzer) UnitMetadataReadAt(file string, line, column int) (UnitMetadataRead, bool) {
	if a == nil {
		return UnitMetadataRead{}, false
	}
	for expr, read := range a.unitMetadataReads {
		if token := expr.Property.Token; token.File == file && token.Line == line && token.Column == column {
			return read, true
		}
	}
	return UnitMetadataRead{}, false
}

// UnitMetadataMember is one of the 16 public metadata members of a named
// unit as editor tooling presents it: its exact type, or the reason reading
// it is a compile-time error.
type UnitMetadataMember struct {
	Name    string
	Type    Type
	Problem string
}

// unitMetadataMemberNames lists the 16 public members in rulebook order.
var unitMetadataMemberNames = []string{
	"Name", "Category", "LongName", "Symbol", "BaseUnit", "Status", "Dimension", "Kind",
	"Scale", "System", "Transform", "Offset", "Origin", "LogBase", "LogFactor", "Reference",
}

// UnitMetadataMembers lists the metadata members of the named unit with the
// same types and read errors that Sema applies to `<U>.Member`.
// Rules: rules/tooling/lsp.md — completions; rules/types/units.md — "Unit metadata value types" (1), (11);
// rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — § 12(9).
func (a *Analyzer) UnitMetadataMembers(unit string) ([]UnitMetadataMember, bool) {
	if a == nil {
		return nil, false
	}
	definition, ok := a.units[unit]
	if !ok {
		return nil, false
	}
	members := make([]UnitMetadataMember, 0, len(unitMetadataMemberNames))
	for _, name := range unitMetadataMemberNames {
		typ, problem, _ := a.unitMetadataMemberFact("<"+unit+">", definition, name, definition.Token)
		members = append(members, UnitMetadataMember{Name: name, Type: typ, Problem: problem.message})
	}
	return members, true
}

// IsUnitMetadataSymbolAt identifies fixed metadata keys/reads for refactoring
// guards. Navigation provenance does not authorize renaming these compiler-
// defined property names, or treating a default property as the unit's name.
// Rules: rules/types/units.md — "Unit metadata value types" (1), (6), (11);
// rules/tooling/lsp.md — "Rename"; MD-062 §12(9).
func (a *Analyzer) IsUnitMetadataSymbolAt(file string, line, column int) bool {
	if a == nil {
		return false
	}
	if _, ok := a.UnitMetadataReadAt(file, line, column); ok {
		return true
	}
	for _, unit := range a.units {
		for _, declaration := range unit.MetadataTokens {
			if declaration.File == file && declaration.Line == line && declaration.Column == column {
				return true
			}
		}
	}
	return false
}
