package semantic

import (
	"fmt"
	"math/big"
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
		switch {
		case static.Initializer.Integer != nil:
			fmt.Fprintf(out, " init=%s", static.Initializer.Integer.String())
		case static.Initializer.Bool != nil:
			fmt.Fprintf(out, " init=%t", *static.Initializer.Bool)
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
		if static.Initializer.Integer != nil && static.Initializer.Bool != nil {
			return nil, fmt.Errorf("static @%d has conflicting initializers", static.ID)
		}
		switch {
		case static.Initializer.Integer != nil:
			if typ.Kind != TypeInt && typ.Kind != TypeUint && typ.Kind != TypeByte && typ.Kind != TypeChar && typ.Kind != TypeRune {
				return nil, fmt.Errorf("static @%d has an integer initializer for a non-integer type", static.ID)
			}
			if !staticIntegerFits(module, typ, static.Initializer.Integer) {
				return nil, fmt.Errorf("static @%d initializer is outside its scalar domain", static.ID)
			}

		case static.Initializer.Bool != nil:
			if typ.Kind != TypeBool {
				return nil, fmt.Errorf("static @%d has a bool initializer for a non-bool type", static.ID)
			}
		default:
			return nil, fmt.Errorf("static @%d has no compile-time initializer", static.ID)
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
	typ, _ := b.module.Types.Lookup(typeID)
	initializer, ok := resolvedStaticInitializerOf(static.Initializer, typ.Kind)
	if !ok {
		return 0, &UnsupportedFeatureError{Feature: "static " + static.Name + " whose type or initializer has no resolved scalar compile-time value", Package: b.maxPackage, Location: location(token)}
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
func resolvedStaticInitializerOf(value sema.ResolvedStaticInitializer, kind TypeKind) (StaticInitializer, bool) {
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
