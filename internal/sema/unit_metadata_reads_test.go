package sema

import (
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

const unitMetadataReadsSource = `module main

enum LaboratoryKind {
    ParticleFlux
    ReactionRate
}

enum OtherKind {
    ParticleFlux
}

unit W physical
impl <W> {
    Dimension: [mass^1, length^2, time^-3]
}

unit m physical
impl <m> {
    LongName: "Metre"
    Symbol: "m"
    BaseUnit: true
    Status: UnitStatus.Active
    Dimension: [length^1]
    Kind: UnitKind.Length
    Scale: 1
    System: UnitSystem.SI
    Transform: UnitTransform.Linear
}

unit aln physical
impl <aln> {
    Dimension: [length^1]
    Kind: length
    Scale: 0.5938
    System: Historical
    Status: active
}

unit dBm ratio
impl <dBm> {
    Dimension: []
    Kind: UnitKind.Ratio
    Transform: UnitTransform.Logarithmic
    LogBase: 10
    LogFactor: 10
    Reference: 0.001<W>
}

unit dB ratio
impl <dB> {
    Transform: logarithmic
    LogBase: 10
    LogFactor: 10
    Reference: 1
}

unit Flux uint other
impl <Flux> {
    Kind: LaboratoryKind.ParticleFlux
}

unit Item uint other
`

// Every MD-062 metadata member of a named unit reads with its exact type:
// strings, bool, decimal and decimal<V>, the closed core enums, and the
// concrete user enum of a custom Kind. Omitted keys read their typed
// defaults; legacy bare names (`Kind: length`, `System: Historical`) read
// as their standard members.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types" (1)–(6), (9)–(11)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 2, 4, 5, 6, 9, 10, 13(1)–13(3), 13(5)
func TestUnitMetadataReadsHaveExactTypes(t *testing.T) {
	source := unitMetadataReadsSource + `
fn Use() void {
    let name: string := <Item>.Name
    let category: UnitCategory := <Item>.Category
    let longName: string := <m>.LongName
    let symbol: string := <m>.Symbol
    let base: bool := <m>.BaseUnit
    let defaultBase: bool := <Item>.BaseUnit
    let status: UnitStatus := <m>.Status
    let defaultStatus: UnitStatus := <Item>.Status
    let kind: UnitKind := <m>.Kind
    let legacyKind: UnitKind := <aln>.Kind
    let defaultKind: UnitKind := <Item>.Kind
    let custom: LaboratoryKind := <Flux>.Kind
    let system: UnitSystem := <aln>.System
    let defaultSystem: UnitSystem := <Item>.System
    let origin: UnitOrigin := <Item>.Origin
    let transform: UnitTransform := <dB>.Transform
    let scale: decimal := <aln>.Scale
    let defaultScale: decimal := <Item>.Scale
    let offset: decimal := <Item>.Offset
    let logBase: decimal := <dB>.LogBase
    let logFactor: decimal := <dBm>.LogFactor
    let reference: decimal := <dB>.Reference
    let power: decimal<W> := <dBm>.Reference
    let active := <aln>.Status == UnitStatus.Active
    let historical := <aln>.System == UnitSystem.Historical
    let sameKind := <Flux>.Kind == LaboratoryKind.ParticleFlux
}
`
	analyzer, errors := analyzeSourceWithAnalyzer(t, source)
	assertSemaErrors(t, errors, nil)
	if aln := analyzer.units["aln"]; aln.Kind != "UnitKind.Length" || aln.System != "UnitSystem.Historical" || aln.Status != StatusActive {
		t.Fatalf("legacy aln metadata = %+v", aln)
	}
	if flux := analyzer.units["Flux"]; flux.Kind != "LaboratoryKind.ParticleFlux" || flux.KindEnum != (UnitMetadataEnum{Type: "LaboratoryKind", Member: "ParticleFlux"}) {
		t.Fatalf("custom Flux Kind = %+v", flux)
	}
	if dBm := analyzer.units["dBm"]; dBm.ReferenceUnit != "W" || dBm.ReferenceValue.RatString() != "1/1000" {
		t.Fatalf("dBm Reference = %+v", dBm)
	}
}

// A custom Kind keeps its exact nominal enum type: it is not a UnitKind and
// does not compare with a same-spelled member of another enum. Transform and
// Status accept only their closed core enums.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types" (4), (11)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 4(2), 4(6), 14(3)
func TestUnitMetadataCustomEnumsStayNominal(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"standard type", "let kind: UnitKind := <Flux>.Kind", "LaboratoryKind"},
		{"other enum", "let same := <Flux>.Kind == OtherKind.ParticleFlux", "OtherKind"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errors := analyzeSource(t, unitMetadataReadsSource+"\nfn Use() void {\n    "+tc.body+"\n}\n")
			if !semaErrorsContain(errors, tc.want) {
				t.Fatalf("want an error naming %s, got %+v", tc.want, errors)
			}
		})
	}
	for _, tc := range []struct{ key, value, want string }{
		{"Transform", "LaboratoryKind.ParticleFlux", "Transform metadata of unit Bad accepts only UnitTransform members, not LaboratoryKind"},
		{"Status", "LaboratoryKind.ReactionRate", "Status metadata of unit Bad accepts only UnitStatus members, not LaboratoryKind"},
		{"Kind", "UnitKind.Gravity", "unknown enum value: UnitKind.Gravity has never been declared"},
		{"System", "Missing.Value", "System metadata of unit Bad must name a member of a declared enum; Missing is not an enum"},
		{"Kind", "widget", "Kind metadata widget of unit Bad has no standard UnitKind member"},
		{"Origin", "ParticleFlux", "Origin metadata ParticleFlux of unit Bad has no standard UnitOrigin member"},
		{"Status", "Active", "Status metadata Active of unit Bad has no standard UnitStatus member"},
	} {
		t.Run(tc.key+" "+tc.value, func(t *testing.T) {
			source := unitMetadataReadsSource + "\nunit Bad other\nimpl <Bad> {\n    " + tc.key + ": " + tc.value + "\n}\n"
			errors := analyzeSource(t, source)
			if len(errors) != 1 || !strings.Contains(errors[0].Message, tc.want) {
				t.Fatalf("want exactly %q, got %+v", tc.want, errors)
			}
		})
	}
}

// Absent metadata without a default, named-only members on structural unit
// expressions, unapproved members and not-yet-implemented UnitDimension reads
// are compile-time diagnostics.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types" (2), (7), (9), (10)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 10(2), 10(4), 14(1), 14(5)
func TestUnitMetadataReadDiagnostics(t *testing.T) {
	for _, tc := range []struct{ expression, want string }{
		{"<Item>.LongName", "unit <Item> declares no LongName metadata"},
		{"<Item>.Symbol", "unit <Item> declares no Symbol metadata"},
		{"<Item>.LogBase", "unit <Item> declares no LogBase metadata"},
		{"<m>.LogFactor", "unit <m> declares no LogFactor metadata"},
		{"<m>.Reference", "unit <m> declares no Reference metadata"},
		{"<dB>.Scale", "unit <dB> declares no Scale metadata"},
		{"<dB>.Offset", "unit <dB> declares no Offset metadata"},
		{"<m/s>.Name", "<m/s> is a structural unit expression and has no Name metadata"},
		{"<m/s>.Dimension", "<m/s> is a structural unit expression and has no Dimension metadata"},
		{"<Item>.DefaultNumericType", "unit <Item> has no member DefaultNumericType"},
		{"<Item>.IsBaseunit", "unit <Item> has no member IsBaseunit"},
		{"<p>.Dimension", "unit <p> has no established Dimension"},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			source := unitMetadataReadsSource + "unit s physical\nimpl <s> {\n    Dimension: [time^1]\n}\nunit p physical\nfn Use() void {\n    let value := " + tc.expression + "\n}\n"
			errors := analyzeSource(t, source)
			if len(errors) != 1 || !strings.Contains(errors[0].Message, tc.want) {
				t.Fatalf("want exactly %q, got %+v", tc.want, errors)
			}
		})
	}
	// Metadata is read-only: an assignment has no `<U>.Member` target form.
	p := parser.New(lexer.New(unitMetadataReadsSource + "fn Use() void {\n    <m>.Kind = UnitKind.Mass\n}\n"))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("assignment to unit metadata was accepted")
	}
}

// A metadata key is declared at most once per unit, also through migration
// spellings and across several impl blocks; both declarations are named.
// Name, Category and IsBaseunit cannot be declared.
//
// Rules:
//   - rules/types/units.md — "Unit metadata", "Unit metadata value types" (6)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 5(5)–5(6), 6(3)–6(4), 14(2)
func TestUnitMetadataDeclarationsAreUnique(t *testing.T) {
	errors := analyzeSource(t, `module main
unit u other
impl <u> {
    Kind: UnitKind.Count
    long_name: "One"
    LongName: "Two"
}
impl extends <u> {
    kind: UnitKind.Ratio
}
`)
	if len(errors) != 2 {
		t.Fatalf("want two duplicate diagnostics, got %+v", errors)
	}
	for _, want := range []string{"duplicate unit metadata LongName for unit <u>", "duplicate unit metadata Kind for unit <u>"} {
		if !semaErrorsContain(errors, want) {
			t.Errorf("missing %q in %+v", want, errors)
		}
	}
	for _, err := range errors {
		if err.PreviousLine == 0 {
			t.Errorf("duplicate does not name the first declaration: %+v", err)
		}
	}
	for _, tc := range []struct{ line, want string }{
		{"Name: \"u\"", "Name is an implicit member of a unit, derived from its unit declaration; it cannot be declared in impl <u>"},
		{"Category: physical", "Category is an implicit member of a unit, derived from its unit declaration; it cannot be declared in impl <u>"},
		{"IsBaseunit: true", "IsBaseunit is not unit metadata; the canonical key is BaseUnit"},
		{"IsBaseUnit: true", "IsBaseUnit is not unit metadata; the canonical key is BaseUnit"},
	} {
		errors := analyzeSource(t, "module main\nunit u other\nimpl <u> {\n    "+tc.line+"\n}\n")
		if !semaErrorsContain(errors, tc.want) {
			t.Errorf("%s: want %q, got %+v", tc.line, tc.want, errors)
		}
	}
}

// The migration table maps exactly the legacy spellings of the standard
// members: snake_case for Kind, Origin, Transform and Status, the exact member
// spelling for System, and temperature for ThermodynamicTemperature.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types" (5)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 5(2)–5(3), 13(8)
func TestUnitMetadataLegacyMigrationTable(t *testing.T) {
	for key, enum := range unitMetadataEnumFields {
		for _, member := range unitMetadataEnumVariants(enum) {
			legacy := snakeCaseMember(member)
			if enum == "UnitSystem" {
				legacy = member
			}
			if got, ok := unitMetadataLegacyMember(key, legacy); !ok || got != member {
				t.Errorf("%s %s -> %q %v, want %s", key, legacy, got, ok, member)
			}
		}
	}
	for _, tc := range [][3]string{
		{"kind", "temperature", "ThermodynamicTemperature"},
		{"kind", "electric_potential", "ElectricPotential"},
		{"origin", "absolute_zero", "AbsoluteZero"},
		{"transform", "logarithmic", "Logarithmic"},
		{"status", "obsolete", "Obsolete"},
		{"system", "USCustomary", "USCustomary"},
	} {
		if got, ok := unitMetadataLegacyMember(tc[0], tc[1]); !ok || got != tc[2] {
			t.Errorf("%s %s -> %q %v, want %s", tc[0], tc[1], got, ok, tc[2])
		}
	}
	for _, tc := range [][2]string{{"kind", "Temperature"}, {"kind", "Length"}, {"system", "si"}, {"status", "Active"}, {"kind", "duration"}, {"symbol", "m"}} {
		if got, ok := unitMetadataLegacyMember(tc[0], tc[1]); ok {
			t.Errorf("%s %s unexpectedly maps to %s", tc[0], tc[1], got)
		}
	}
}

// The trusted core declarations in sec/core/unit_metadata.sec are the exact
// canonical enums and share one identity with the compiler fallback.
//
// Rules:
//   - rules/library/core-library.md — §§ 27.1–27.4
//   - rules/types/units.md — "Unit metadata value types" (3)
func TestCoreUnitMetadataEnums(t *testing.T) {
	program := parseIPCErrorFixture(t, "../../sec/core/unit_metadata.sec")
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	count := 0
	for _, statement := range program.Statements {
		enum, ok := statement.(*ast.EnumDeclaration)
		if !ok {
			continue
		}
		count++
		name := enum.Name.Value
		typ, fallback := a.Types()[name], builtinType(name)
		want := unitMetadataEnumVariants(name)
		if len(want) == 0 || !typ.Intrinsic || typ.Module != "core" || typ.Kind != EnumType || typ.ErrorAssignable || !sameConcreteType(typ, fallback) || !reflect.DeepEqual(typ.EnumValues, want) {
			t.Fatal(name, typ, fallback)
		}
		if resolution := DefaultValueOf(typ); resolution.Kind != EnumDefault || resolution.Value.Lexeme != want[0] {
			t.Fatal(name, resolution)
		}
	}
	if count != 6 || len(unitMetadataEnumVariants("UnitKind")) != 66 || len(unitMetadataEnumVariants("UnitSystem")) != 18 {
		t.Fatalf("enums=%d UnitKind=%d UnitSystem=%d", count, len(unitMetadataEnumVariants("UnitKind")), len(unitMetadataEnumVariants("UnitSystem")))
	}
	errors := analyzeSource(t, "module main\nenum UnitStatus {\n    Active\n}\n")
	if len(errors) == 0 {
		t.Fatal("a project redeclared the core UnitStatus enum")
	}
}

func semaErrorsContain(errors []Error, want string) bool {
	for _, err := range errors {
		if strings.Contains(err.Message, want) {
			return true
		}
	}
	return false
}
