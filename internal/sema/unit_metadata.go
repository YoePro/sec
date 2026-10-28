package sema

import (
	"math/big"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// UnitMetadataEnum is the resolved value of an enum-backed unit metadata key:
// the a.types key of its nominal enum and the member name. The zero value
// means the key was not declared.
type UnitMetadataEnum struct {
	Type   string
	Member string
}

// Identity is the canonical "Type.Member" spelling used by unit semantics to
// compare Kind and Origin facts, or "" when the key was not declared.
func (value UnitMetadataEnum) Identity() string {
	if value.Type == "" {
		return ""
	}
	return value.Type + "." + value.Member
}

// unitMetadataKeys lists the canonical spellings of the 14 declarable unit
// metadata keys by normalized spelling.
// Rules: rules/types/units.md — "Unit metadata".
var unitMetadataKeys = map[string]string{
	"longname": "LongName", "symbol": "Symbol", "baseunit": "BaseUnit", "status": "Status",
	"dimension": "Dimension", "kind": "Kind", "scale": "Scale", "system": "System",
	"transform": "Transform", "offset": "Offset", "origin": "Origin", "logbase": "LogBase",
	"logfactor": "LogFactor", "reference": "Reference",
}

// analyzeUnitMetadata binds the metadata of every `impl <U>` to its unit:
// each key at most once per unit across all of its impl blocks, enum-backed
// keys to nominal enum members, and numeric keys to exact rationals. Transform
// requirements are validated once per unit, whether or not anything reads
// the metadata.
//
// Rules:
//   - rules/types/units.md — "Unit metadata", "Unit metadata value types" (4), (5), (10), "Validation of metadata"
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 4, 5(5)–5(6), 10(5)
func (a *Analyzer) analyzeUnitMetadata(program *ast.Program) {
	declared := map[string]map[string]lexer.Token{}
	var order []string
	targets := map[string]lexer.Token{}
	scope := a.collectUnitAxes(program)
	for _, stmt := range program.Statements {
		if isNilStatement(stmt) {
			continue
		}
		if module, ok := stmt.(*ast.ModuleStatement); ok {
			scope.module = module.Path
			continue
		}
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok || !a.validImplStatements[impl] || impl.Target == nil {
			continue
		}
		// MD-040: only `impl <Unit>` holds unit metadata.
		if !impl.Target.UnitOnly || !a.shadowedUnitImpls[impl] {
			continue
		}
		name := impl.Target.Name
		unit, ok := a.units[name]
		if !ok {
			continue
		}
		if _, seen := targets[name]; !seen {
			targets[name] = impl.Target.Token
			order = append(order, name)
			declared[name] = map[string]lexer.Token{}
		}
		for _, member := range impl.Members {
			metadata, ok := member.(*ast.UnitMetadataDeclaration)
			if !ok {
				continue
			}
			key := normalizeUnitMetadataName(metadata.Name)
			if previous, duplicate := declared[name][key]; duplicate {
				a.addErrorAtTokenWithPrevious(metadata.Token, previous, "duplicate unit metadata %s for unit <%s>", unitMetadataKeys[key], name)
				continue
			}
			declared[name][key] = metadata.Token
			if a.applyUnitMetadata(name, &unit, key, metadata, scope) {
				if unit.Declared == nil {
					unit.Declared = map[string]bool{}
				}
				unit.Declared[key] = true
				if unit.MetadataTokens == nil {
					unit.MetadataTokens = map[string]lexer.Token{}
				}
				unit.MetadataTokens[key] = metadata.Token
				a.recordDefinition(metadata.Token)
			}
		}
		a.units[name] = unit
	}
	for _, name := range order {
		unit := a.units[name]
		if unit.Transform == "" {
			unit.Transform = LinearUnitTransform
		}
		a.validateUnitTransformMetadata(name, targets[name], unit)
		a.units[name] = unit
		if len(unit.Declared) == 0 {
			continue
		}
		typ := a.unitTypeEntry(name)
		if typ.Kind != InvalidType {
			typ.Dimension = unit.Dimension
			if semantics, dimension, err := resolveNamedUnitSemantics(name, a.units); err == nil {
				typ.UnitSemantics = semantics
				typ.Dimension = dimension
			}
			a.storeUnitTypeEntry(name, typ)
		}
	}
}

// applyUnitMetadata validates one metadata declaration and records it on
// unit, reporting whether it was valid. A malformed value is diagnosed and
// never replaced by the key's omission default.
//
// Rules:
//   - rules/types/units.md — "Unit metadata", "Unit metadata value types" (4), (5), (9), (10)
func (a *Analyzer) applyUnitMetadata(name string, unit *UnitDefinition, key string, metadata *ast.UnitMetadataDeclaration, scope unitAxisScope) bool {
	value := unitMetadataValueTokens(metadata.Value)
	switch key {
	case "dimension":
		spelled, ok := parseUnitMetadataDimension(value)
		if !ok {
			a.addErrorAtToken(metadata.Token, "invalid dimension metadata for unit %s", name)
			return false
		}
		dimension, ok := a.canonicalUnitDimension(name, metadata.Token, spelled, scope.forFile(metadata.Token.File))
		if !ok {
			return false
		}
		unit.Dimension = dimension
		unit.DimensionEstablished = true
	case "scale":
		// MD-062 § 9(2): a linear Scale is any non-zero exact value; a
		// negative one reverses coordinate direction. Zero is not invertible.
		scale, ok := exactUnitMetadataValue(value)
		if !ok {
			a.addErrorAtToken(metadata.Token, "invalid scale metadata for unit %s", name)
			return false
		}
		if scale.Sign() == 0 {
			a.addErrorAtTokenWithMetadata(metadata.Token, "",
				"a zero scale maps every value to one coordinate and cannot be inverted; use a non-zero scale",
				"invalid scale metadata for unit %s: Scale must be non-zero", name)
			return false
		}
		unit.Scale = tokensDisplay(value)
		unit.ScaleValue = scale
	case "longname", "symbol":
		text, ok := parseUnitMetadataString(value)
		if !ok {
			a.addErrorAtToken(metadata.Token, "invalid %s metadata for unit %s", unitMetadataKeys[key], name)
			return false
		}
		if key == "longname" {
			unit.LongName = text
		} else {
			unit.Symbol = text
		}
	case "baseunit":
		baseUnit, ok := parseUnitMetadataBool(value)
		if !ok {
			a.addErrorAtToken(metadata.Token, "invalid BaseUnit metadata for unit %s", name)
			return false
		}
		unit.IsBaseUnit = baseUnit
	case "status", "transform", "kind", "system", "origin":
		enum, ok := a.resolveUnitMetadataEnum(name, key, metadata.Token, value, scope.forFile(metadata.Token.File))
		if !ok {
			return false
		}
		switch key {
		case "status":
			unit.Status = UnitStatus(snakeCaseMember(enum.Member))
		case "transform":
			unit.Transform = UnitTransform(snakeCaseMember(enum.Member))
		case "kind":
			unit.KindEnum = enum
			// UnitKind.Unknown is an absence marker, never a compatibility fact.
			if enum.Identity() != "UnitKind.Unknown" {
				unit.Kind = enum.Identity()
			}
		case "system":
			unit.SystemEnum = enum
			unit.System = enum.Identity()
		case "origin":
			unit.OriginEnum = enum
			// UnitOrigin.None never satisfies a required origin.
			if enum.Identity() != "UnitOrigin.None" {
				unit.Origin = enum.Identity()
			}
		}
	case "offset":
		offset, exact := exactUnitMetadataValue(value)
		if len(value) == 0 || !exact {
			a.addErrorAtToken(metadata.Token, "invalid Offset metadata for unit %s", name)
			return false
		}
		unit.Offset = tokensDisplay(value)
		unit.OffsetValue = offset
	case "logbase":
		logBase, exact := exactUnitMetadataValue(value)
		if len(value) == 0 || !exact || logBase.Sign() <= 0 || logBase.Cmp(big.NewRat(1, 1)) == 0 {
			a.addErrorAtToken(metadata.Token, "invalid LogBase metadata for unit %s", name)
			return false
		}
		unit.LogBase = tokensDisplay(value)
		unit.LogBaseValue = logBase
	case "logfactor":
		logFactor, exact := exactUnitMetadataValue(value)
		if len(value) == 0 || !exact || logFactor.Sign() == 0 {
			a.addErrorAtToken(metadata.Token, "invalid LogFactor metadata for unit %s", name)
			return false
		}
		unit.LogFactor = tokensDisplay(value)
		unit.LogFactorValue = logFactor
	case "reference":
		reference, referenceUnit, ok := a.parseUnitMetadataReference(value)
		if !ok {
			a.addErrorAtToken(metadata.Token, "invalid Reference metadata for unit %s; expected a compile-time number such as 1 or a quantity such as 0.001<W>", name)
			return false
		}
		unit.Reference = tokensDisplay(value)
		unit.ReferenceValue = reference
		unit.ReferenceUnit = referenceUnit
	default:
		return false
	}
	return true
}

// resolveUnitMetadataEnum resolves an enum-backed metadata value: a qualified
// member `Enum.Member` of a declared enum (only the core enum for Transform
// and Status), or a legacy bare name from the migration table. A bare name
// without a mapping is a migration diagnostic; user enums are never searched
// by member name.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types" (3)–(5)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 4(1)–4(6), 5(1)–5(4)
func (a *Analyzer) resolveUnitMetadataEnum(name, key string, token lexer.Token, value []lexer.Token, scope unitAxisScope) (UnitMetadataEnum, bool) {
	standard := unitMetadataEnumFields[key]
	field := unitMetadataKeys[key]
	example := standard + "." + unitMetadataEnumVariants(standard)[0]
	if len(value) == 1 && value[0].Type == lexer.IDENT {
		member, ok := unitMetadataLegacyMember(key, value[0].Lexeme)
		if !ok {
			a.addErrorAtTokenWithMetadata(value[0], "",
				"write a qualified enum member such as "+example+" or CustomEnum.Member; legacy bare metadata names are accepted only when they map to one standard member",
				"%s metadata %s of unit %s has no standard %s member", field, value[0].Lexeme, name, standard)
			return UnitMetadataEnum{}, false
		}
		return UnitMetadataEnum{Type: standard, Member: member}, true
	}
	if len(value) < 3 || len(value)%2 == 0 {
		a.addErrorAtToken(token, "invalid %s metadata for unit %s; expected an enum member such as %s", field, name, example)
		return UnitMetadataEnum{}, false
	}
	for i, part := range value {
		if i%2 == 0 && part.Type != lexer.IDENT || i%2 == 1 && part.Type != lexer.DOT {
			a.addErrorAtToken(token, "invalid %s metadata for unit %s; expected an enum member such as %s", field, name, example)
			return UnitMetadataEnum{}, false
		}
	}
	member := value[len(value)-1]
	typeName := tokensJoined(value[:len(value)-2])
	// A metadata value belongs to its declaring unit's module/file imports,
	// independent of the currently edited caller or a same-spelled root enum.
	// Rules: rules/types/units.md — "Unit metadata value types" (4), (11);
	// rules/projects/modules.md — §§7–9.
	if dot := strings.Index(typeName, "."); dot >= 0 {
		if module, imported := scope.file[typeName[:dot]]; imported {
			typeName = module + typeName[dot:]
		}
	} else if unit, known := a.units[name]; known {
		if _, exists := a.types[unit.Module+"."+typeName]; exists && unit.Module != "" {
			typeName = unit.Module + "." + typeName
		} else if typ, exists := a.types[typeName]; exists && typ.Module != "" && typ.Module != "core" && typ.Module != unit.Module {
			typeName = unit.Module + "." + typeName
		}
	}
	typeName = a.resolveTypeName(typeName)
	typ, declared := a.types[typeName]
	if !declared || typ.Kind != EnumType || len(typ.GenericParameters) > 0 {
		a.addErrorAtToken(value[0], "%s metadata of unit %s must name a member of a declared enum; %s is not an enum", field, name, typeName)
		return UnitMetadataEnum{}, false
	}
	if (key == "transform" || key == "status") && typeName != standard {
		a.addErrorAtTokenWithMetadata(value[0], "",
			"Transform and Status are closed: use a member of "+standard,
			"%s metadata of unit %s accepts only %s members, not %s", field, name, standard, typeName)
		return UnitMetadataEnum{}, false
	}
	if definition, exists := a.typeDefinitionTokens[typeName]; exists {
		a.bindDefinition(value[0], definition)
	}
	constant, exists := typ.EnumConsts[member.Lexeme]
	if !exists {
		a.addErrorAtToken(member, "unknown enum value: %s.%s has never been declared", typeName, member.Lexeme)
		return UnitMetadataEnum{}, false
	}
	a.bindDefinition(member, constant.Token)
	return UnitMetadataEnum{Type: typeName, Member: member.Lexeme}, true
}

// parseUnitMetadataReference evaluates a Reference value: an exact
// dimensionless number, or a number followed by one named unit `<V>`.
// Rules: rules/types/units.md — "Unit metadata value types" (9).
func (a *Analyzer) parseUnitMetadataReference(tokens []lexer.Token) (*big.Rat, string, bool) {
	unit := ""
	numeric := tokens
	if count := len(tokens); count >= 4 && tokens[count-1].Type == lexer.GT && tokens[count-3].Type == lexer.LT && tokens[count-2].Type == lexer.IDENT {
		unit = tokens[count-2].Lexeme
		if _, known := a.units[unit]; !known {
			return nil, "", false
		}
		numeric = tokens[:count-3]
	}
	value, ok := exactUnitMetadataValue(numeric)
	if len(numeric) == 0 || !ok {
		return nil, "", false
	}
	return value, unit, true
}

// unitMetadataMemberType types the read-only metadata value `<U>.Member` of a
// named unit: one of the 16 members with its exact type, or a compile-time
// diagnostic when the value is absent and has no default. It reports false
// for any other member name.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types" (1), (2), (6), (9), (10), (11)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 2, 6, 9, 10
func (a *Analyzer) unitMetadataMemberType(reference *ast.UnitReferenceExpression, unit UnitDefinition, property *ast.Identifier) (Type, bool) {
	typ, problem, metadata := a.unitMetadataMemberFact(reference.String(), unit, property.Value, property.Token)
	if problem.message != "" {
		a.addErrorAtTokenWithMetadata(property.Token, "", problem.help, "%s", problem.message)
		return Type{Kind: InvalidType}, true
	}
	return typ, metadata
}

// unitMetadataProblem explains why a metadata member cannot be read.
type unitMetadataProblem struct {
	message, help string
}

// unitMetadataMemberFact is the diagnostic-free typing of `<U>.Member`
// shared by Sema and editor tooling: the member's type, or the problem that
// makes reading it an error. It reports false for any other member name.
// Rules: rules/types/units.md — "Unit metadata value types" (1), (2), (6), (9), (10), (11).
func (a *Analyzer) unitMetadataMemberFact(reference string, unit UnitDefinition, member string, token lexer.Token) (Type, unitMetadataProblem, bool) {
	missing := func(key, help string) (Type, unitMetadataProblem, bool) {
		return Type{Kind: InvalidType}, unitMetadataProblem{message: "unit " + reference + " declares no " + key + " metadata", help: help}, true
	}
	logarithmic := unit.Transform == LogarithmicUnitTransform
	decimal := a.types["decimal"]
	switch member {
	case "Name":
		return Type{Name: "string", Kind: StringType}, unitMetadataProblem{}, true
	case "Category":
		return a.types["UnitCategory"], unitMetadataProblem{}, true
	case "LongName", "Symbol":
		if !unit.Declared[strings.ToLower(member)] {
			return missing(member, "declare `"+member+": \"...\"` in impl "+reference+"; a missing display name is not read as an empty string")
		}
		return Type{Name: "string", Kind: StringType}, unitMetadataProblem{}, true
	case "BaseUnit":
		return a.types["bool"], unitMetadataProblem{}, true
	case "Status":
		return a.types["UnitStatus"], unitMetadataProblem{}, true
	case "Transform":
		return a.types["UnitTransform"], unitMetadataProblem{}, true
	case "Kind":
		return a.unitMetadataEnumType(unit.KindEnum, "UnitKind"), unitMetadataProblem{}, true
	case "System":
		return a.unitMetadataEnumType(unit.SystemEnum, "UnitSystem"), unitMetadataProblem{}, true
	case "Origin":
		return a.unitMetadataEnumType(unit.OriginEnum, "UnitOrigin"), unitMetadataProblem{}, true
	case "Scale", "Offset":
		if logarithmic && !unit.Declared[strings.ToLower(member)] {
			return missing(member, "a logarithmic unit has no linear "+member+"; its omission default applies only to linear units")
		}
		return decimal, unitMetadataProblem{}, true
	case "LogBase", "LogFactor":
		if !unit.Declared[strings.ToLower(member)] {
			return missing(member, member+" has no default; declare it in impl "+reference+" for a logarithmic unit")
		}
		return decimal, unitMetadataProblem{}, true
	case "Reference":
		if !unit.Declared["reference"] {
			return missing("Reference", "Reference has no default; declare it in impl "+reference+" for a logarithmic unit")
		}
		if unit.ReferenceUnit == "" {
			return decimal, unitMetadataProblem{}, true
		}
		typ, ok := a.resolveType(&ast.TypeReference{Token: token, Name: "decimal", Unit: unit.ReferenceUnit,
			UnitExpression: &ast.UnitExpression{Token: token, Kind: ast.UnitExpressionName, Name: unit.ReferenceUnit}})
		if !ok {
			return Type{Kind: InvalidType}, unitMetadataProblem{}, true
		}
		return typ, unitMetadataProblem{}, true
	case "Dimension":
		// An unresolved physical dimension is not a UnitDimension value.
		if !unit.DimensionEstablished {
			return Type{Kind: InvalidType}, unitMetadataProblem{
				message: "unit " + reference + " has no established Dimension",
				help:    "declare `Dimension: [axis^exponent, ...]` in impl " + reference + "; a physical unit without one has an unknown dimension, which is neither dimensionless nor a value",
			}, true
		}
		return a.types["UnitDimension"], unitMetadataProblem{}, true
	}
	return Type{}, unitMetadataProblem{}, false
}

// unitMetadataEnumType is the exact nominal type of an enum-backed read: the
// declared enum when the key was given, otherwise the standard enum whose
// default member is the omission value.
// Rules: rules/types/units.md — "Unit metadata value types" (1), (4).
func (a *Analyzer) unitMetadataEnumType(value UnitMetadataEnum, standard string) Type {
	if value.Type != "" {
		return a.types[value.Type]
	}
	return a.types[standard]
}

// isUnitMetadataMember reports whether name is one of the 16 public unit
// metadata members.
// Rules: rules/types/units.md — "Unit metadata value types" (1).
func isUnitMetadataMember(name string) bool {
	if name == "Name" || name == "Category" {
		return true
	}
	return unitMetadataKeys[strings.ToLower(name)] == name
}

func tokensJoined(tokens []lexer.Token) string {
	var out strings.Builder
	for _, token := range tokens {
		out.WriteString(token.Lexeme)
	}
	return out.String()
}

func unitMetadataValueTokens(tokens []lexer.Token) []lexer.Token {
	out := make([]lexer.Token, 0, len(tokens))
	for _, token := range tokens {
		if token.Type == lexer.COMMENT {
			continue
		}
		out = append(out, token)
	}
	return out
}

func normalizeUnitMetadataName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "_", "")
}

// parseUnitMetadataString decodes quoted metadata with Sec's escape grammar
// while retaining the identifier form accepted by unit metadata declarations.
//
// Rules:
//   - rules/types/units.md — unit metadata
//   - rules/foundations/lexical_structure.md — §15 "Escapes"
func parseUnitMetadataString(tokens []lexer.Token) (string, bool) {
	if len(tokens) != 1 {
		return "", false
	}
	switch tokens[0].Type {
	case lexer.STRING:
		value, ok := lexer.DecodeStringLiteral(tokens[0].Lexeme)
		if !ok {
			return "", false
		}
		return value, true
	case lexer.IDENT:
		return tokens[0].Lexeme, true
	default:
		return "", false
	}
}

func parseUnitMetadataBool(tokens []lexer.Token) (bool, bool) {
	if len(tokens) != 1 {
		return false, false
	}
	switch tokens[0].Type {
	case lexer.TRUE:
		return true, true
	case lexer.FALSE:
		return false, true
	default:
		return false, false
	}
}

// validateUnitTransformMetadata checks the metadata each transform requires,
// whether or not it is read. A negative Scale is defined only for linear
// units; its affine domain is undecided (MD-048), so it is rejected there.
// Rules: rules/types/units.md — "Validation of metadata", "Unit metadata
// value types" (10); rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — § 9(2).
func (a *Analyzer) validateUnitTransformMetadata(name string, token lexer.Token, unit UnitDefinition) {
	switch unit.Transform {
	case AffineUnitTransform:
		if unit.Scale == "" || unit.Offset == "" || unit.Origin == "" {
			a.addErrorAtToken(token, "affine unit %s requires Scale, Offset, and Origin metadata", name)
		}
		if unit.ScaleValue != nil && unit.ScaleValue.Sign() < 0 {
			a.addErrorAtTokenWithMetadata(token, "",
				"negative Scale is defined for linear units; the domain of a negative affine scale is not decided yet (missing-decisions.yaml MD-048)",
				"affine unit %s cannot have a negative Scale", name)
		}
	case LogarithmicUnitTransform:
		if unit.LogBase == "" || unit.LogFactor == "" || unit.Reference == "" {
			a.addErrorAtToken(token, "logarithmic unit %s requires LogBase, LogFactor, and Reference metadata", name)
		}
	}
}

func tokensDisplay(tokens []lexer.Token) string {
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		parts = append(parts, token.Lexeme)
	}
	return strings.Join(parts, " ")
}

func parseUnitMetadataDimension(tokens []lexer.Token) (Dimension, bool) {
	if len(tokens) < 2 || tokens[0].Type != lexer.LBRACKET || tokens[len(tokens)-1].Type != lexer.RBRACKET {
		return Dimension{}, false
	}
	dimension := Dimension{Base: map[string]int{}}
	i := 1
	for i < len(tokens)-1 {
		if tokens[i].Type != lexer.IDENT {
			return Dimension{}, false
		}
		axis := tokens[i].Lexeme
		i++
		for i+1 < len(tokens)-1 && tokens[i].Type == lexer.DOT && tokens[i+1].Type == lexer.IDENT {
			axis += "." + tokens[i+1].Lexeme
			i += 2
		}
		if i >= len(tokens)-1 || tokens[i].Type != lexer.BIT_XOR {
			return Dimension{}, false
		}
		i++
		sign := 1
		if i < len(tokens)-1 && tokens[i].Type == lexer.MINUS {
			sign = -1
			i++
		}
		if i >= len(tokens)-1 || tokens[i].Type != lexer.INT {
			return Dimension{}, false
		}
		exponent, ok := parseSmallInt(tokens[i].Lexeme)
		if !ok || exponent == 0 {
			return Dimension{}, false
		}
		if _, exists := dimension.Base[axis]; exists {
			return Dimension{}, false
		}
		dimension.Base[axis] = sign * exponent
		i++
		if i == len(tokens)-1 {
			break
		}
		if tokens[i].Type != lexer.COMMA {
			return Dimension{}, false
		}
		i++
	}
	return dimension, true
}

// maxUnitDimensionExponent bounds a declared exponent so dimension algebra
// never wraps into a different valid dimension.
// Rules: rules/types/units.md — "Unit metadata value types" (7).
const maxUnitDimensionExponent = 1<<15 - 1

func parseSmallInt(value string) (int, bool) {
	n := 0
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
		if n > maxUnitDimensionExponent {
			return 0, false
		}
	}
	return n, true
}

// compilerKnownUnitAxes are the axes with fixed shared compiler-known
// identities; every other axis belongs to its defining module.
// Rules: rules/types/units.md — "Unit metadata value types" (7).
var compilerKnownUnitAxes = map[string]bool{
	"length": true, "mass": true, "time": true, "electric_current": true,
	"thermodynamic_temperature": true, "amount_of_substance": true,
	"luminous_intensity": true, "information": true,
}

// unitAxisKey is the canonical identity of axis as written in module: the
// bare name of a compiler-known axis, otherwise "module::axis", so equal
// spellings in different modules are distinct axes.
// Rules: rules/types/units.md — "Unit metadata value types" (7).
func unitAxisKey(module, axis string) string {
	if compilerKnownUnitAxes[axis] {
		return axis
	}
	return module + "::" + axis
}

// unitAxisLocalName is the declared spelling of a canonical axis key.
func unitAxisLocalName(key string) string {
	if index := strings.LastIndex(key, "::"); index >= 0 {
		return key[index+2:]
	}
	return key
}

// unitAxisScope resolves the axes written in one module's unit metadata:
// the module itself, the import aliases of each file (alias to module name),
// and every axis key declared by any unit of the program.
type unitAxisScope struct {
	module   string
	imports  map[string]map[string]string
	file     map[string]string
	declared map[string]bool
}

func (scope unitAxisScope) forFile(file string) unitAxisScope {
	scope.file = scope.imports[file]
	return scope
}

// collectUnitAxes gathers the domain axes declared by the program's units:
// the named-identity axis of every currency and other unit and every
// unqualified non-compiler-known axis written in a Dimension, keyed by its
// defining module, plus each file's import aliases.
// Rules: rules/types/units.md — "Unit metadata value types" (7)–(8).
func (a *Analyzer) collectUnitAxes(program *ast.Program) unitAxisScope {
	scope := unitAxisScope{imports: map[string]map[string]string{}, declared: map[string]bool{}}
	for _, unit := range a.units {
		if unit.Category == CurrencyUnit || unit.Category == OtherUnit {
			scope.declared[unitAxisKey(unit.Module, unit.Name)] = true
		}
	}
	module := ""
	for _, stmt := range program.Statements {
		switch stmt := stmt.(type) {
		case *ast.ModuleStatement:
			if stmt != nil {
				module = stmt.Path
			}
		case *ast.ImportStatement:
			if stmt == nil {
				continue
			}
			target := stmt.Path
			if index := strings.LastIndex(target, "/"); index >= 0 {
				target = target[index+1:]
			}
			alias := stmt.Alias
			if alias == "" {
				alias = target
			}
			if scope.imports[stmt.Token.File] == nil {
				scope.imports[stmt.Token.File] = map[string]string{}
			}
			scope.imports[stmt.Token.File][alias] = target
		case *ast.ImplStatement:
			if stmt == nil || stmt.Target == nil || !stmt.Target.UnitOnly {
				continue
			}
			for _, member := range stmt.Members {
				metadata, ok := member.(*ast.UnitMetadataDeclaration)
				if !ok || normalizeUnitMetadataName(metadata.Name) != "dimension" {
					continue
				}
				spelled, ok := parseUnitMetadataDimension(unitMetadataValueTokens(metadata.Value))
				if !ok {
					continue
				}
				for axis := range spelled.Base {
					if !strings.Contains(axis, ".") {
						scope.declared[unitAxisKey(module, axis)] = true
					}
				}
			}
		}
	}
	return scope
}

// canonicalUnitDimension rekeys a parsed Dimension by axis identity. A
// qualified axis `Q.axis` names the axis declared in the module imported as Q
// (or the module named Q) and must exist there; two spellings of one resolved
// axis are a duplicate.
// Rules: rules/types/units.md — "Unit metadata value types" (7);
// rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 7(9)–7(11), 14(4).
func (a *Analyzer) canonicalUnitDimension(name string, token lexer.Token, spelled Dimension, scope unitAxisScope) (Dimension, bool) {
	axes := make([]string, 0, len(spelled.Base))
	for axis := range spelled.Base {
		axes = append(axes, axis)
	}
	sort.Strings(axes)
	dimension := Dimension{Base: map[string]int{}}
	written := map[string]string{}
	for _, axis := range axes {
		key := unitAxisKey(scope.module, axis)
		if index := strings.LastIndex(axis, "."); index >= 0 {
			qualifier, local := axis[:index], axis[index+1:]
			target := qualifier
			if imported, ok := scope.file[qualifier]; ok {
				target = imported
			}
			key = unitAxisKey(target, local)
			if !compilerKnownUnitAxes[local] && !scope.declared[key] {
				a.addErrorAtToken(token, "unknown dimension axis %s for unit %s: no unit metadata of module %s declares axis %s", axis, name, target, local)
				return Dimension{}, false
			}
		}
		if previous, duplicate := written[key]; duplicate {
			a.addErrorAtToken(token, "duplicate dimension axis for unit %s: %s and %s name the same axis", name, previous, axis)
			return Dimension{}, false
		}
		written[key] = axis
		dimension.Base[key] = spelled.Base[axis]
	}
	return dimension, true
}
