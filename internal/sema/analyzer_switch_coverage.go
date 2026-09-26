package sema

import (
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

type switchCoverageTracker struct {
	subjectType  Type
	values       map[string]lexer.Token
	ranges       []switchConstRange
	boolValues   map[bool]lexer.Token
	stringValues map[string]lexer.Token
	// enumValues is keyed by canonical integer/string value class, not declaration name.
	enumValues map[string]lexer.Token
}

type switchConstRange struct {
	min          *big.Int
	minExclusive bool
	max          *big.Int
	maxExclusive bool
	token        lexer.Token
	relational   bool
}

func newSwitchCoverageTracker() *switchCoverageTracker {
	return &switchCoverageTracker{
		values:       map[string]lexer.Token{},
		boolValues:   map[bool]lexer.Token{},
		stringValues: map[string]lexer.Token{},
		enumValues:   map[string]lexer.Token{},
	}
}

func (a *Analyzer) analyzeSwitchRangeCase(item *ast.SwitchRangeCase, subjectType Type) {
	if !isOrderedSwitchType(subjectType) {
		a.addErrorAtToken(item.Token, "switch range requires ordered subject type")
		return
	}
	if item.Range == nil {
		return
	}
	if item.Range.Start != nil {
		startType, _ := a.inferExpression(item.Range.Start)
		if startType.Kind != InvalidType && !canRangeBoundType(subjectType, startType, item.Range.Start) {
			a.addErrorAtToken(expressionToken(item.Range.Start), "switch range must be compatible with subject type %s, got %s", typeDisplayName(subjectType), typeDisplayName(startType))
		}
	}
	if item.Range.End != nil {
		endType, _ := a.inferExpression(item.Range.End)
		if endType.Kind != InvalidType && !canRangeBoundType(subjectType, endType, item.Range.End) {
			a.addErrorAtToken(expressionToken(item.Range.End), "switch range must be compatible with subject type %s, got %s", typeDisplayName(subjectType), typeDisplayName(endType))
		}
	}
}

// checkSwitchValueCoverage rejects duplicate bool, string, integer, and
// nominal enum value classes, normalizing declared aliases to their shared
// runtime value class and retaining the first equivalent case as provenance.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §20 "Duplicate compile-time values"
//   - rules/declarations/enums.md — §7 "Value aliases"
//   - rules/corrections/applied/correction24-20260823.md — "Bug 3 — enum aliases bypass duplicate switch-case detection"
func (a *Analyzer) checkSwitchValueCoverage(expr ast.Expression, tracker *switchCoverageTracker) {
	if tracker == nil {
		return
	}
	if literal, ok := expr.(*ast.BooleanLiteral); ok && tracker.subjectType.Kind == BoolType {
		if _, exists := tracker.boolValues[literal.Value]; exists {
			a.addErrorAtToken(expressionToken(expr), "duplicate switch case value %t", literal.Value)
			return
		}
		tracker.boolValues[literal.Value] = expressionToken(expr)
		return
	}
	if literal, ok := expr.(*ast.StringLiteral); ok && tracker.subjectType.Kind == StringType {
		if _, exists := tracker.stringValues[literal.Value]; exists {
			a.addErrorAtTokenWithMetadata(
				expressionToken(expr),
				diagnostics.DuplicateSwitchCase,
				"Remove the duplicate case or combine its body with the first case for this string value.",
				"duplicate switch case value %q",
				literal.Value,
			)
			return
		}
		tracker.stringValues[literal.Value] = expressionToken(expr)
		return
	}
	if _, key, ok := a.switchEnumCaseVariant(expr, tracker.subjectType); ok {
		if previous, exists := tracker.enumValues[key]; exists {
			a.addErrorAtTokenWithPreviousID(
				expressionToken(expr),
				previous,
				diagnostics.DuplicateSwitchCase,
				"duplicate switch case enum underlying value",
			)
			return
		}
		tracker.enumValues[key] = expressionToken(expr)
		return
	}
	value, ok := constantIntegerValue(expr)
	if !ok {
		return
	}
	key := value.String()
	if _, exists := tracker.values[key]; exists {
		a.addErrorAtToken(expressionToken(expr), "duplicate switch case value %s", key)
		return
	}
	for _, previous := range tracker.ranges {
		if previous.contains(value) {
			a.addErrorAtToken(expressionToken(expr), "switch case value %s is already covered by previous case", key)
			return
		}
	}
	tracker.values[key] = expressionToken(expr)
}

// switchEnumCaseVariant resolves a type-qualified enum member to its canonical
// integer/string value-class key for switch coverage.
//
// Rules:
//   - rules/declarations/enums.md — §7 "Value aliases"
//   - rules/declarations/enums.md — §15 "switch"
func (a *Analyzer) switchEnumCaseVariant(expr ast.Expression, subjectType Type) (string, string, bool) {
	if subjectType.Kind != EnumType {
		return "", "", false
	}
	member, ok := expr.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return "", "", false
	}
	typeName, ok := typePathFromExpression(member.Object)
	if !ok {
		return "", "", false
	}
	typeName = a.resolveTypeName(typeName)
	typ, ok := a.types[typeName]
	if !ok || typ.Kind != EnumType || !sameConcreteType(subjectType, typ) {
		return "", "", false
	}
	value, ok := typ.EnumConsts[member.Property.Value]
	key, ok := enumValueClassKey(value)
	if !ok {
		return "", "", false
	}
	return member.Property.Value, key, true
}

func (a *Analyzer) warnIncompleteEnumSwitch(stmt *ast.SwitchStatement, tracker *switchCoverageTracker) {
	if stmt == nil || tracker == nil || tracker.subjectType.Kind != EnumType {
		return
	}
	if tracker.subjectType.BitWidth > 0 {
		return
	}
	missing := []string{}
	seenValues := map[string]bool{}
	for _, name := range tracker.subjectType.EnumValues {
		value, ok := tracker.subjectType.EnumConsts[name]
		if !ok {
			continue
		}
		key, ok := enumValueClassKey(value)
		if !ok || seenValues[key] {
			continue
		}
		seenValues[key] = true
		if _, covered := tracker.enumValues[key]; !covered {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return
	}
	a.addWarningAtTokenWithMetadata(
		expressionToken(stmt.Subject),
		diagnostics.IncompleteEnumSwitch,
		"Handle the missing values, add default, or use match when exhaustive variant handling is required.",
		"switch over %s omits known values: %s",
		typeDisplayName(tracker.subjectType),
		strings.Join(missing, ", "),
	)
}

func (t *switchCoverageTracker) isExhaustive() bool {
	if t == nil {
		return false
	}
	if t.subjectType.Kind == BoolType {
		return len(t.boolValues) == 2
	}
	if t.subjectType.Kind != EnumType || t.subjectType.BitWidth > 0 {
		return false
	}
	classes := map[string]bool{}
	for _, value := range t.subjectType.EnumConsts {
		if key, ok := enumValueClassKey(value); ok {
			classes[key] = true
		}
	}
	return len(classes) > 0 && len(t.enumValues) == len(classes)
}

func (a *Analyzer) checkSwitchRangeCoverage(expr *ast.RangeExpression, tracker *switchCoverageTracker) {
	if tracker == nil || expr == nil {
		return
	}
	current, ok := switchConstRangeFromExpression(expr)
	if !ok {
		return
	}
	for _, previous := range tracker.ranges {
		if previous.relational && !current.coveredBy(previous) {
			continue
		}
		if current.overlaps(previous) {
			a.addErrorAtToken(expr.Token, "%s", switchCoverageOverlapMessage(current, previous))
			return
		}
	}
	for key := range tracker.values {
		value, ok := new(big.Int).SetString(key, 10)
		if ok && current.contains(value) {
			a.addErrorAtToken(expr.Token, "switch case range overlaps previous case")
			return
		}
	}
	tracker.ranges = append(tracker.ranges, current)
}

func (a *Analyzer) checkSwitchRelationalCoverage(item *ast.SwitchRelationalCase, tracker *switchCoverageTracker) {
	if tracker == nil || item == nil {
		return
	}
	current, ok := switchConstRangeFromRelationalCase(item)
	if !ok {
		return
	}
	for _, previous := range tracker.ranges {
		if previous.relational {
			if current.coveredBy(previous) {
				a.addErrorAtToken(item.Token, "unreachable switch case; previous case already covers this condition")
				return
			}
			continue
		}
		if current.overlaps(previous) {
			a.addErrorAtToken(item.Token, "unreachable switch case; previous case already covers this condition")
			return
		}
	}
	tracker.ranges = append(tracker.ranges, current)
}

func switchCoverageOverlapMessage(current switchConstRange, previous switchConstRange) string {
	if current.relational || previous.relational {
		return "unreachable switch case; previous case already covers this condition"
	}
	return "switch case range overlaps previous case"
}

func switchConstRangeFromExpression(expr *ast.RangeExpression) (switchConstRange, bool) {
	out := switchConstRange{token: expr.Token, maxExclusive: expr.Exclusive}
	if expr.Start != nil {
		value, ok := constantIntegerValue(expr.Start)
		if !ok {
			return switchConstRange{}, false
		}
		out.min = value
	}
	if expr.End != nil {
		value, ok := constantIntegerValue(expr.End)
		if !ok {
			return switchConstRange{}, false
		}
		out.max = value
	}
	out.normalizeBounds()
	return out, true
}

func switchConstRangeFromRelationalCase(item *ast.SwitchRelationalCase) (switchConstRange, bool) {
	value, ok := constantIntegerValue(item.Value)
	if !ok {
		return switchConstRange{}, false
	}
	out := switchConstRange{token: item.Token, relational: true}
	switch item.Operator {
	case "<":
		out.max = value
		out.maxExclusive = true
	case "<=":
		out.max = value
	case ">":
		out.min = value
		out.minExclusive = true
	case ">=":
		out.min = value
	default:
		return switchConstRange{}, false
	}
	return out, true
}

func (r switchConstRange) contains(value *big.Int) bool {
	if r.min != nil {
		cmp := value.Cmp(r.min)
		if cmp < 0 || (cmp == 0 && r.minExclusive) {
			return false
		}
	}
	if r.max != nil {
		cmp := value.Cmp(r.max)
		if cmp > 0 || (cmp == 0 && r.maxExclusive) {
			return false
		}
	}
	return true
}

func (r switchConstRange) overlaps(other switchConstRange) bool {
	if r.max != nil && other.min != nil {
		cmp := r.max.Cmp(other.min)
		if cmp < 0 || (cmp == 0 && (r.maxExclusive || other.minExclusive)) {
			return false
		}
	}
	if other.max != nil && r.min != nil {
		cmp := other.max.Cmp(r.min)
		if cmp < 0 || (cmp == 0 && (other.maxExclusive || r.minExclusive)) {
			return false
		}
	}
	return true
}

func (r *switchConstRange) normalizeBounds() {
	if r == nil || r.min == nil || r.max == nil || r.min.Cmp(r.max) <= 0 {
		return
	}
	r.min, r.max = r.max, r.min
	r.minExclusive, r.maxExclusive = r.maxExclusive, r.minExclusive
}

func (r switchConstRange) coveredBy(other switchConstRange) bool {
	return lowerBoundCovers(other, r) && upperBoundCovers(other, r)
}

func lowerBoundCovers(outer switchConstRange, inner switchConstRange) bool {
	if outer.min == nil {
		return true
	}
	if inner.min == nil {
		return false
	}
	cmp := outer.min.Cmp(inner.min)
	if cmp < 0 {
		return true
	}
	if cmp > 0 {
		return false
	}
	return !outer.minExclusive || inner.minExclusive
}

func upperBoundCovers(outer switchConstRange, inner switchConstRange) bool {
	if outer.max == nil {
		return true
	}
	if inner.max == nil {
		return false
	}
	cmp := outer.max.Cmp(inner.max)
	if cmp > 0 {
		return true
	}
	if cmp < 0 {
		return false
	}
	return !outer.maxExclusive || inner.maxExclusive
}
