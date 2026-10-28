package semantic

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema"
)

// UnitMetadataFacts retains the provenance of a constant materialized from a
// unit metadata read: the unit declaration, the member, and for a decimal
// member its exact declared rational and whether the public decimal is a
// projection of it (MD-062 § 9(7)). A folded UnitDimension observation names
// the member it observes, such as "Dimension.AxisCount".
// Rules: rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — § 12(7).
type UnitMetadataFacts struct {
	Unit      string
	Member    string
	Exact     string
	Projected bool
}

// buildUnitMetadataMember lowers a `<U>.Member` metadata read, or a property
// of a direct `<U>.Dimension` read, to the typed constant Sema resolved. Every
// metadata value is known at compile time and its read has no effects, so it
// folds; a UnitDimension value that escapes into a runtime value has no
// representation yet and fails closed.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types", "Semantic IR and lowering"
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 7(12), 11(5), 12(7)–12(8)
func (fb *functionBuilder) buildUnitMetadataMember(expr *ast.MemberExpression, typeID TypeID) (builtValue, bool, error) {
	loc := locationFromExpression(expr)
	if read, ok := fb.owner.analyzer.ResolvedUnitMetadataReadOf(expr); ok {
		facts := &UnitMetadataFacts{Unit: read.Unit, Member: read.Member}
		switch read.Member {
		case "Name", "LongName", "Symbol":
			return fb.result(Operation{Kind: OpConstString, String: read.String, UnitMetadata: facts, Location: loc}, typeID), true, nil
		case "BaseUnit":
			value := read.Bool
			return fb.result(Operation{Kind: OpConstBool, Bool: &value, UnitMetadata: facts, Location: loc}, typeID), true, nil
		case "Kind", "System", "Origin", "Transform", "Status", "Category":
			if fb.owner.maxPackage < 11 {
				return builtValue{}, true, fb.unsupported("enum unit metadata read", expr.Property.Token)
			}
			return fb.result(Operation{Kind: OpEnumConstant, EnumCase: EnumCaseID(read.Enum.Ordinal), UnitMetadata: facts, Location: loc}, typeID), true, nil
		case "Scale", "Offset", "LogBase", "LogFactor", "Reference":
			decimal, err := metadataDecimalConstant(read.Decimal)
			if err != nil {
				return builtValue{}, true, err
			}
			facts.Exact, facts.Projected = read.Exact.RatString(), read.Projected
			return fb.result(Operation{Kind: OpConstDecimal, Decimal: &decimal, UnitMetadata: facts, Location: loc}, typeID), true, nil
		}
		return builtValue{}, true, fb.unsupported("UnitDimension value", expr.Property.Token)
	}
	if expr.Property == nil {
		return builtValue{}, false, nil
	}
	site := expr.Property.Token
	member, resolved := fb.owner.analyzer.CompilerKnownMemberAt(site.File, site.Line, site.Column)
	if !resolved || member.ID != "CKM-UNITDIMENSION-AXISCOUNT" && member.ID != "CKM-UNITDIMENSION-ISDIMENSIONLESS" {
		return builtValue{}, false, nil
	}
	dimension, constant := fb.owner.analyzer.ResolvedUnitDimensionOf(expr.Object)
	if !constant {
		return builtValue{}, true, fb.unsupported("runtime UnitDimension property", site)
	}
	facts := fb.unitDimensionFacts(expr.Object, "Dimension."+expr.Property.Value)
	if member.ID == "CKM-UNITDIMENSION-AXISCOUNT" {
		return fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(int64(len(dimension.Base))), UnitMetadata: facts, Location: loc}, typeID), true, nil
	}
	value := dimension.IsZero()
	return fb.result(Operation{Kind: OpConstBool, Bool: &value, UnitMetadata: facts, Location: loc}, typeID), true, nil
}

// buildUnitDimensionEquality folds `==` and `!=` between two direct
// `<U>.Dimension` reads by exact canonical axis-identity equality. Equality
// between UnitDimension runtime values fails closed until Sec MLIR has a
// representation for them.
// Rules: rules/foundations/operators.md — "Unit dimension equality";
// rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 7(7), 7(12), 12(8).
func (fb *functionBuilder) buildUnitDimensionEquality(expr ast.Expression, typeID TypeID) (builtValue, bool, error) {
	infix, ok := expr.(*ast.InfixExpression)
	if !ok || infix.Operator != "==" && infix.Operator != "!=" {
		return builtValue{}, false, nil
	}
	if typ, typed := fb.owner.analyzer.ResolvedTypeOf(infix.Left); !typed || typ.Kind != sema.UnitDimensionType {
		return builtValue{}, false, nil
	}
	left, leftConstant := fb.owner.analyzer.ResolvedUnitDimensionOf(infix.Left)
	right, rightConstant := fb.owner.analyzer.ResolvedUnitDimensionOf(infix.Right)
	if !leftConstant || !rightConstant {
		return builtValue{}, true, fb.unsupported("runtime UnitDimension equality", infix.Token)
	}
	value := left.Equal(right) == (infix.Operator == "==")
	other := fb.unitDimensionFacts(infix.Right, "Dimension")
	facts := fb.unitDimensionFacts(infix.Left, "Dimension"+infix.Operator+"<"+other.Unit+">.Dimension")
	return fb.result(Operation{Kind: OpConstBool, Bool: &value, UnitMetadata: facts, Location: locationFromExpression(expr)}, typeID), true, nil
}

// unitDimensionFacts names the unit whose Dimension a folded observation read.
func (fb *functionBuilder) unitDimensionFacts(object ast.Expression, member string) *UnitMetadataFacts {
	read, _ := fb.owner.analyzer.ResolvedUnitMetadataReadOf(object.(*ast.MemberExpression))
	return &UnitMetadataFacts{Unit: read.Unit, Member: member}
}

// formatUnitMetadata prints the provenance of a metadata constant.
func formatUnitMetadata(facts *UnitMetadataFacts) string {
	if facts == nil {
		return ""
	}
	text := fmt.Sprintf(" unit-metadata=<%s>.%s", facts.Unit, facts.Member)
	if facts.Exact != "" {
		text += " exact=" + facts.Exact
	}
	if facts.Projected {
		text += " projected"
	}
	return text
}

// verifyUnitMetadataOperation checks that metadata provenance annotates only
// a constant, and that an unprojected decimal equals its exact value.
// Rules: rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 9(7)–9(8), 12(7).
func verifyUnitMetadataOperation(op Operation) error {
	facts := op.UnitMetadata
	if facts == nil {
		return nil
	}
	switch op.Kind {
	case OpConstString, OpConstBool, OpConstInt, OpEnumConstant:
		if facts.Exact != "" || facts.Projected {
			return fmt.Errorf("%s unit metadata <%s>.%s has no decimal provenance", op.Kind, facts.Unit, facts.Member)
		}
	case OpConstDecimal:
		exact, ok := new(big.Rat).SetString(facts.Exact)
		if !ok || op.Decimal == nil {
			return fmt.Errorf("decimal unit metadata <%s>.%s has no exact value", facts.Unit, facts.Member)
		}
		value := decimalConstantValue(*op.Decimal)
		if (value.Cmp(exact) != 0) != facts.Projected || facts.Projected && facts.Member != "Scale" {
			return fmt.Errorf("decimal unit metadata <%s>.%s projection does not match its exact value %s", facts.Unit, facts.Member, facts.Exact)
		}
	default:
		return fmt.Errorf("%s cannot carry unit metadata provenance", op.Kind)
	}
	return nil
}

// metadataDecimalConstant builds the constant of a decimal metadata read from
// Sema's canonical text: a plain decimal, or `<coefficient>e<exponent>` for a
// negative scale (MD-066 makes the whole signed int32 scale domain legal).
// Rules: rules/types/types.md — "Decimal scale"; MD-062 § 9(7).
func metadataDecimalConstant(text string) (DecimalConstant, error) {
	coefficient, exponent, scientific := strings.Cut(text, "e")
	if !scientific {
		return parseDecimal(text)
	}
	c, ok := new(big.Int).SetString(coefficient, 10)
	e, err := strconv.ParseInt(exponent, 10, 32)
	if !ok || err != nil || e <= 0 {
		return DecimalConstant{}, fmt.Errorf("invalid decimal metadata %q", text)
	}
	return DecimalConstant{Coefficient: c, Scale: int32(-e), Lexeme: text}, nil
}

// decimalConstantValue is the exact value coefficient × 10^(-scale).
func decimalConstantValue(decimal DecimalConstant) *big.Rat {
	scale := int64(decimal.Scale)
	power := new(big.Int).Exp(big.NewInt(10), big.NewInt(max(scale, -scale)), nil)
	if scale >= 0 {
		return new(big.Rat).SetFrac(decimal.Coefficient, power)
	}
	return new(big.Rat).SetInt(new(big.Int).Mul(decimal.Coefficient, power))
}
