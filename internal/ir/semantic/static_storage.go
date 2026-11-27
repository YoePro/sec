package semantic

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema"
)

// StaticID identifies one static-duration storage location of a module.
type StaticID uint32

// Static is one static-duration storage location (semantic_ir.md § 28): a
// module-level binding or a function-local `static let`. It is initialized
// once from its compile-time constant Initializer — never by a hidden
// startup or lazy-initialization function (§ 28(5)) — and is read with
// static.load, written with static.store when Mutable, and addressed as a
// Place root with place.static-root, which marks it AddressTaken.
type Static struct {
	ID       StaticID
	Name     string
	Type     TypeID
	Mutable  bool
	Function string
	// Initializer is the exact compile-time constant value: Integer for an
	// integer-family type, Bool for bool.
	Initializer  StaticInitializer
	AddressTaken bool
	Location     Location
}

// StaticInitializer is a static's compile-time constant initial value.
type StaticInitializer struct {
	Integer *big.Int
	Bool    *bool
	// Components hold a struct's fields in declaration order or a fixed
	// array's elements in index order (compile_time_evaluation.md § 32).
	Aggregate  bool
	Components []StaticInitializer
	// Variant is a union, Option or Result value: its active variant and
	// payload (compile_time_evaluation.md § 32(3); SVV-2).
	Variant *StaticVariantInitializer
	// Enum names the case of an enum value; String is a string value.
	Enum   *string
	String *string
	// Float is a float value's canonical binary payload in its resolved
	// format (P20 §§ 74, 76); it is never a lexeme to reparse.
	Float *FloatConstant
}

// StaticVariantInitializer is the active variant of a static tagged value and
// its payload: none for an empty variant (or Ok of Result[void, E]), the
// payload for a single-payload variant, and an Aggregate of the fields in
// declaration order for a named-field variant.
type StaticVariantInitializer struct {
	Name    string
	Payload *StaticInitializer
}

func formatStaticInitializer(initializer StaticInitializer) string {
	switch {
	case initializer.Aggregate:
		parts := make([]string, len(initializer.Components))
		for index, component := range initializer.Components {
			parts[index] = formatStaticInitializer(component)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case initializer.Integer != nil:
		return initializer.Integer.String()
	case initializer.Bool != nil:
		return fmt.Sprintf("%t", *initializer.Bool)
	case initializer.Variant != nil:
		if initializer.Variant.Payload == nil {
			return initializer.Variant.Name
		}
		return initializer.Variant.Name + "(" + formatStaticInitializer(*initializer.Variant.Payload) + ")"
	case initializer.Enum != nil:
		return "." + *initializer.Enum
	case initializer.String != nil:
		return strconv.Quote(*initializer.String)
	case initializer.Float != nil:
		return fmt.Sprintf("float.bits=0x%x/%d", initializer.Float.Bits, initializer.Float.Format)
	}
	return "?"
}

const (
	OpStaticLoad      OpKind = "static.load"
	OpStaticStore     OpKind = "static.store"
	OpPlaceStaticRoot OpKind = "place.static-root"
)

func formatStatics(out *strings.Builder, statics []Static) {
	if len(statics) == 0 {
		return
	}
	out.WriteString("\n  statics {\n")
	for _, static := range statics {
		fmt.Fprintf(out, "    @%d %q: !%d mutable=%t", static.ID, static.Name, static.Type, static.Mutable)
		if static.Function != "" {
			fmt.Fprintf(out, " function=%q", static.Function)
		}
		if formatStaticInitializer(static.Initializer) != "?" {
			fmt.Fprintf(out, " init=%s", formatStaticInitializer(static.Initializer))
		}
		if static.AddressTaken {
			out.WriteString(" address-taken")
		}
		fmt.Fprintf(out, " %s\n", formatLocation(static.Location))
	}
	out.WriteString("  }\n")
}

// verifyStatics checks the module's static declarations: unique dense IDs,
// a known type and exactly one initializer in that scalar domain.
// Rules: rules/compiler/semantic_ir.md §28; rules/types/types.md — integer/char/rune domains.
func verifyStatics(module *Module) (map[StaticID]Static, error) {
	statics := map[StaticID]Static{}
	for index, static := range module.Statics {
		if static.ID != StaticID(index+1) || static.Name == "" {
			return nil, fmt.Errorf("static @%d is not the %dth static or has no name", static.ID, index+1)
		}
		typ, ok := module.Types.Lookup(static.Type)
		if !ok {
			return nil, fmt.Errorf("static @%d has an unknown type", static.ID)
		}
		_ = typ
		if err := verifyStaticInitializer(module, static.Type, static.Initializer); err != nil {
			return nil, fmt.Errorf("static @%d: %w", static.ID, err)
		}
		statics[static.ID] = static
	}
	return statics, nil
}

// verifyStaticOperation checks static.load, static.store and
// place.static-root against the static they name.
func verifyStaticOperation(module *Module, op Operation, values map[ValueID]Value) error {
	static, ok := staticOf(module, op.Static)
	if !ok {
		return fmt.Errorf("%s names unknown static @%d", op.Kind, op.Static)
	}
	switch op.Kind {
	case OpStaticLoad:
		if len(op.Operands) != 0 || len(op.Results) != 1 || op.Results[0].Type != static.Type {
			return fmt.Errorf("invalid static.load of @%d", static.ID)
		}
	case OpStaticStore:
		if len(op.Operands) != 1 || len(op.Results) != 0 || values[op.Operands[0]].Type != static.Type {
			return fmt.Errorf("invalid static.store to @%d", static.ID)
		}
		if !static.Mutable {
			return fmt.Errorf("static.store to immutable static @%d", static.ID)
		}
	case OpPlaceStaticRoot:
		place, ok := placeType(module.Types, resultTypeOf(op))
		if len(op.Operands) != 0 || !ok || place.Element != static.Type {
			return fmt.Errorf("invalid place.static-root of @%d", static.ID)
		}
		if place.Mutable && !static.Mutable {
			return fmt.Errorf("place.static-root authority exceeds static @%d mutability", static.ID)
		}
		if !static.AddressTaken {
			return fmt.Errorf("place.static-root addresses static @%d that is not marked address-taken", static.ID)
		}
		if op.Place == nil || op.Place.RootKind != PlaceRootStatic || op.Place.Root == 0 || !validAddressSpace(op.Place.AddressSpace) {
			return fmt.Errorf("place.static-root requires static Place root metadata")
		}
	}
	return nil
}

func staticOf(module *Module, id StaticID) (Static, bool) {
	if id == 0 || int(id) > len(module.Statics) {
		return Static{}, false
	}
	return module.Statics[id-1], true
}

func resultTypeOf(op Operation) TypeID {
	if len(op.Results) != 1 {
		return 0
	}
	return op.Results[0].Type
}

// staticRegistry maps Sema static bindings to module statics.
type staticRegistry struct {
	ids        map[sema.StaticBindingID]StaticID
	moduleLets map[*ast.LetStatement]bool
}

// registerStatic adds a Sema static binding to the module once, with its
// compile-time initializer. Module-level statics of other modules and
// statics whose type or initializer Semantic IR cannot represent yet are
// package boundaries.
func (b *builder) registerStatic(static sema.ResolvedStatic) (StaticID, error) {
	if id, ok := b.statics.ids[static.ID]; ok {
		return id, nil
	}
	declaration := static.Declaration
	// String holders enter the high-level materialization track with its
	// explicit allocation-context capability; retain earlier package gates.
	// Rules: rules/compiler/semantic_ir.md — §§17,28; MD-039.
	if static.Type.Kind == sema.StringType && b.maxPackage < arenaLoweringPackage {
		return 0, &UnsupportedFeatureError{Feature: "static " + static.Name + " whose type or initializer has no resolved compile-time value", Package: b.maxPackage, Location: location(declaration.Token)}
	}
	token := declaration.Name.Token
	if static.Function == "" && !b.statics.moduleLets[declaration] {
		return 0, &UnsupportedFeatureError{Feature: "static storage of another module", Package: b.maxPackage, Location: location(token)}
	}
	// MD-081: a static initialized with `ref S` is valid source, but a static
	// holding a reference needs a module-lifetime borrow in the P15 reference
	// model and reference lowering (P24); it stays an explicit boundary.
	if _, isReference := declaration.Value.(*ast.RefExpression); isReference {
		return 0, &UnsupportedFeatureError{Feature: "static " + static.Name + " initialized with a reference (static-held borrow materialization, P15-42/P24)", Package: b.maxPackage, Location: location(token)}
	}
	typeID, err := b.internType(static.Type)
	if err != nil {
		return 0, locateUnsupportedType(err, token)
	}
	initializer, ok := b.staticInitializerOf(static.Initializer, typeID)
	if !ok {
		return 0, &UnsupportedFeatureError{Feature: "static " + static.Name + " whose type or initializer has no resolved compile-time value", Package: b.maxPackage, Location: location(token)}
	}
	name := b.module.Identity + "::" + static.Name
	if static.Function != "" {
		name = b.module.Identity + "::" + static.Function + "::" + static.Name
	}
	id := StaticID(len(b.module.Statics) + 1)
	b.module.Statics = append(b.module.Statics, Static{
		ID: id, Name: name, Type: typeID, Mutable: static.Mutable, Function: static.Function,
		Initializer: initializer, Location: location(token),
	})
	if b.statics.ids == nil {
		b.statics.ids = map[sema.StaticBindingID]StaticID{}
	}
	b.statics.ids[static.ID] = id
	return id, nil
}

// resolvedStaticInitializerOf consumes Sema's immutable scalar CTE result. It
// does not reevaluate source expressions or read runtime static storage.
// Rules: rules/declarations/static.md §§15–16; rules/compiler/semantic_ir.md §28(5).
func (b *builder) staticInitializerOf(value sema.ResolvedStaticInitializer, typeID TypeID) (StaticInitializer, bool) {
	typ, ok := staticInitializerType(b.module.Types, typeID)
	if !ok {
		return StaticInitializer{}, false
	}
	kind := typ.Kind
	if value.Aggregate {
		// A struct's fields in declaration order; a fixed array's elements.
		var types []TypeID
		switch kind {
		case TypeStruct:
			definition, found := structDefinition(b.module, typeID)
			if !found {
				return StaticInitializer{}, false
			}
			for _, field := range definition.Fields {
				types = append(types, field.Type)
			}
		case TypeArray:
			for range value.Components {
				types = append(types, typ.Element)
			}
		default:
			return StaticInitializer{}, false
		}
		if len(types) != len(value.Components) {
			return StaticInitializer{}, false
		}
		result := StaticInitializer{Aggregate: true, Components: []StaticInitializer{}}
		for index, component := range value.Components {
			converted, ok := b.staticInitializerOf(component, types[index])
			if !ok {
				return StaticInitializer{}, false
			}
			result.Components = append(result.Components, converted)
		}
		return result, true
	}
	if value.Variant != nil {
		return b.staticVariantInitializerOf(*value.Variant, typeID)
	}
	if value.Scalar != nil {
		switch kind {
		case TypeEnum:
			if member := value.Scalar.Nominal.Member; member != "" {
				return StaticInitializer{Enum: &member}, true
			}
		case TypeString:
			text := value.Scalar.String
			return StaticInitializer{String: &text}, true
		case TypeFloat:
			if float, ok := b.staticFloatConstant(*value.Scalar, typeID); ok {
				return StaticInitializer{Float: float}, true
			}
		}
		return StaticInitializer{}, false
	}
	integer := kind == TypeInt || kind == TypeUint || kind == TypeByte || kind == TypeChar || kind == TypeRune
	if integer && value.Integer != nil && value.Bool == nil {
		return StaticInitializer{Integer: new(big.Int).Set(value.Integer)}, true
	}
	if kind == TypeBool && value.Bool != nil && value.Integer == nil {
		b := *value.Bool
		return StaticInitializer{Bool: &b}, true
	}
	return StaticInitializer{}, false
}

// staticOfIdentifier resolves an identifier naming static storage.
func (fb *functionBuilder) staticOfIdentifier(identifier *ast.Identifier) (Static, bool, error) {
	static, ok := fb.owner.analyzer.ResolvedStaticOf(identifier)
	if !ok {
		return Static{}, false, nil
	}
	id, err := fb.owner.registerStatic(static)
	if err != nil {
		return Static{}, true, err
	}
	return fb.owner.module.Statics[id-1], true, nil
}

// lowerStaticRootPlace forms the Place root of a static binding (P15-42,
// package15.md § 64): place.static-root with the static's authority, marking
// the static address-taken so it keeps one stable location. Fixed-address
// and non-ordinary address spaces stay unsupported.
func (fb *functionBuilder) lowerStaticRootPlace(reference *ast.RefExpression, plan sema.ResolvedReferencePlan, rootExpr ast.Expression) (builtValue, error) {
	if binding, ok := fb.hardwareBindingRoot(rootExpr); ok {
		// MD-077 § 12(2): a hardware binding is a register.hw.bind root, never
		// a Place; its uses need register.hw.* operations with verified plans.
		return builtValue{}, fb.unsupported("use of hardware register instance "+binding.Symbol+" (register.hw.observe/request_write need verified MD-111 operation plans, MD-077 § 18(4))", reference.Token)
	}
	if plan.Facts.AddressStability == sema.AddressStabilityFixed || plan.Facts.AddressSpace != sema.AddressSpaceOrdinary {
		return builtValue{}, fb.unsupported("addressed or non-ordinary Place root", reference.Token)
	}
	identifier, ok := rootExpr.(*ast.Identifier)
	if !ok {
		return builtValue{}, fb.unsupported("reference root expression", reference.Token)
	}
	static, isStatic, err := fb.staticOfIdentifier(identifier)
	if err != nil {
		return builtValue{}, err
	}
	if !isStatic {
		return builtValue{}, fmt.Errorf("static-provenance root %s has no static binding", identifier.Value)
	}
	if reference.Mutable && !static.Mutable {
		return builtValue{}, fb.unsupported("mutable borrow of an immutable static", reference.Token)
	}
	fb.owner.module.Statics[static.ID-1].AddressTaken = true
	facts := placeMetadataOf(plan.Place)
	facts.RootKind = PlaceRootStatic
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: static.Type, Mutable: static.Mutable})
	return fb.placeResult(Operation{Kind: OpPlaceStaticRoot, Static: static.ID, Place: facts, Location: location(identifier.Token)}, placeType), nil
}

// moduleStaticNamed returns the module-level static declared with name.
func (b *builder) moduleStaticNamed(name string) (Static, bool) {
	for _, static := range b.module.Statics {
		if static.Function == "" && static.Name == b.module.Identity+"::"+name {
			return static, true
		}
	}
	return Static{}, false
}

// buildStaticCompoundAssignment lowers `target op= value` on a mutable static
// as one load, the resolved underlying operator, and one store, exactly like
// a compound assignment to local storage (operators.md "Compound
// assignment"; static.md).
func (fb *functionBuilder) buildStaticCompoundAssignment(stmt *ast.AssignmentStatement, resolved sema.ResolvedOperator, static Static) error {
	if !static.Mutable {
		return fmt.Errorf("compound assignment to immutable static %s", static.Name)
	}
	rightType, err := fb.owner.internType(*resolved.RightType)
	if err != nil {
		return err
	}
	loc := location(stmt.Token)
	current := fb.result(Operation{Kind: OpStaticLoad, Static: static.ID, Location: loc}, static.Type)
	value, err := fb.buildExpr(stmt.Value, rightType)
	if err != nil {
		return err
	}
	op := Operation{Operands: []ValueID{current.id, value.id}, Location: loc, Operator: strings.TrimSuffix(stmt.Operator, "=")}
	if !semanticOperatorKind(resolved, &op) {
		return fb.unsupported("compound assignment "+stmt.Operator, stmt.Token)
	}
	updated := builtValue{}
	if resolved.RuntimeCheck || resolved.ShiftCountProven {
		if updated, err = fb.emitCheckedOperator(op, static.Type, nil, nil, nil); err != nil {
			return err
		}
	} else {
		updated = fb.result(op, static.Type)
	}
	fb.emit(Operation{Kind: OpStaticStore, Static: static.ID, Operands: []ValueID{updated.id}, Location: loc})
	return nil
}

// staticIntegerFits validates representability using explicit or selected widths,
// never host int; unresolved target widths remain high-level, not a range proof.
// Rules: rules/compiler/semantic_ir.md §28; rules/types/types.md — integer/char/rune domains.
func staticIntegerFits(module *Module, typ Type, value *big.Int) bool {
	if typ.Kind == TypeRune && (value.Sign() < 0 || value.Cmp(big.NewInt(0x10ffff)) > 0 || value.Cmp(big.NewInt(0xd800)) >= 0 && value.Cmp(big.NewInt(0xdfff)) <= 0) {
		return false
	}
	if !typ.Signed && value.Sign() < 0 {
		return false
	}
	bits := typ.BitWidth
	if typ.TargetSize {
		bits = module.PointerWidthBits
	}
	if bits == 0 {
		return true
	}
	limit := new(big.Int).Lsh(big.NewInt(1), uint(bits))
	if !typ.Signed {
		return value.Cmp(limit) < 0
	}
	half := new(big.Int).Rsh(limit, 1)
	return value.Cmp(new(big.Int).Neg(half)) >= 0 && value.Cmp(half) < 0
}

// verifyStaticInitializer checks a compile-time initializer against its type:
// a scalar in its domain, or a struct's fields and a fixed array's elements,
// each complete and in order (compile_time_evaluation.md § 32).
func verifyStaticInitializer(module *Module, typeID TypeID, initializer StaticInitializer) error {
	typ, ok := staticInitializerType(module.Types, typeID)
	if !ok {
		return fmt.Errorf("initializer of an unknown type")
	}
	set := 0
	for _, present := range []bool{initializer.Integer != nil, initializer.Bool != nil, initializer.Aggregate, initializer.Variant != nil, initializer.Enum != nil, initializer.String != nil, initializer.Float != nil} {
		if present {
			set++
		}
	}
	if set != 1 {
		return fmt.Errorf("needs exactly one compile-time initializer")
	}
	switch {
	case initializer.Integer != nil:
		if typ.Kind != TypeInt && typ.Kind != TypeUint && typ.Kind != TypeByte && typ.Kind != TypeChar && typ.Kind != TypeRune {
			return fmt.Errorf("integer initializer for a non-integer type")
		}
		if !staticIntegerFits(module, typ, initializer.Integer) {
			return fmt.Errorf("initializer is outside its scalar domain")
		}
	case initializer.Bool != nil:
		if typ.Kind != TypeBool {
			return fmt.Errorf("bool initializer for a non-bool type")
		}
	case initializer.Variant != nil:
		return verifyStaticVariant(module, typeID, *initializer.Variant)
	case initializer.Enum != nil:
		definition, found := enumDefinition(module, typeID)
		if !found {
			return fmt.Errorf("enum initializer for a non-enum type")
		}
		for _, enumCase := range definition.Cases {
			if enumCase.Name == *initializer.Enum {
				return nil
			}
		}
		return fmt.Errorf("enum initializer names unknown case %q", *initializer.Enum)
	case initializer.String != nil:
		if typ.Kind != TypeString {
			return fmt.Errorf("string initializer for a non-string type")
		}
	case initializer.Float != nil:
		if typ.Kind != TypeFloat || !initializer.Float.Canonical || initializer.Float.Format != 32 && initializer.Float.Format != 64 ||
			typ.BitWidth != 0 && uint16(typ.BitWidth) != initializer.Float.Format || initializer.Float.Format == 32 && initializer.Float.Bits>>32 != 0 {
			return fmt.Errorf("float initializer needs canonical bits of its type's binary format")
		}
	case typ.Kind == TypeStruct:
		definition, found := structDefinition(module, typeID)
		if !found || len(definition.Fields) != len(initializer.Components) {
			return fmt.Errorf("struct initializer does not list every field")
		}
		for index, field := range definition.Fields {
			if err := verifyStaticInitializer(module, field.Type, initializer.Components[index]); err != nil {
				return err
			}
		}
	case typ.Kind == TypeArray:
		length, ok := parseCanonicalArrayLength(typ.Length)
		if !ok || length.Cmp(big.NewInt(int64(len(initializer.Components)))) != 0 {
			return fmt.Errorf("array initializer does not list every element")
		}
		for _, component := range initializer.Components {
			if err := verifyStaticInitializer(module, typ.Element, component); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("aggregate initializer for a %s type", typ.Kind)
	}
	return nil
}

// staticFloatConstant converts Sema's resolved compile-time float into the
// canonical bit pattern of the static's binary format: the value is the
// shortest round-trip spelling of the already rounded binary value, so
// parsing it in that format is exact (signed zero, infinities and NaN
// included).
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 13-20, 74, 82.
func (b *builder) staticFloatConstant(value sema.DefaultConstant, typeID TypeID) (*FloatConstant, bool) {
	format, ok := b.floatFormat(typeID)
	if !ok || value.Kind != sema.FloatType || value.FloatBits != 0 && uint16(value.FloatBits) != format {
		return nil, false
	}
	number, err := strconv.ParseFloat(value.Lexeme, int(format))
	if err != nil {
		return nil, false
	}
	bits := math.Float64bits(number)
	if format == 32 {
		bits = uint64(math.Float32bits(float32(number)))
	}
	if math.IsNaN(number) {
		// The deterministic canonical quiet NaN, as T.NaN lowers (P20 § 28).
		bits = 0x7ff8000000000000
		if format == 32 {
			bits = 0x7fc00000
		}
	}
	return &FloatConstant{Bits: bits, Format: format, Canonical: true}, true
}
