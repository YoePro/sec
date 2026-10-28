package semantic

import (
	"fmt"
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// Detached register values (MD-077 §§ 5–6, 9–10; semantic_ir.md § 21(4)–(5)).
// A register[N] value is a nominal fixed-width bit layout; its field reads
// and writes are the pure register.value.extract / register.value.update
// operations over the module's register-definition table. Hardware-bound
// instances are a different category and never reach these operations.

// registerLoweringPackage is the package that admits register values
// (P15-42/42a).
const registerLoweringPackage = 15

// RegisterFieldID is a field's declaration ordinal in its register.
type RegisterFieldID uint32

// RegisterFieldDefinition is one declared field: its bit slice in the
// register's logical numbering (bit 0 is the least-significant bit), its
// value carrier type, access contract and unit. Reserved `_` fields are kept
// so the layout stays complete; they are never extracted or updated.
type RegisterFieldDefinition struct {
	ID        RegisterFieldID
	Name      string
	BitOffset uint16
	Width     uint16
	Type      TypeID
	Access    string
	Shadow    string
	Unit      string
	Reserved  bool
	Location  Location
}

// RegisterDefinition is a register type's nominal identity and layout.
type RegisterDefinition struct {
	TypeID          TypeID
	SymbolID        SymbolID
	Name            string
	Width           uint16
	AllocationOrder string
	ByteOrder       string
	Fields          []RegisterFieldDefinition
	Location        Location
}

// RegisterWriteProof is Sema's proof that an updated field value fits the
// field without truncation (MD-077 § 6(5)): typed (bool, enum or nested
// register value), constant, or a proven interval.
type RegisterWriteProof struct {
	Basis string
	Low   *big.Int
	High  *big.Int
}

func (b *builder) internRegisterType(t sema.Type) (TypeID, error) {
	if b.maxPackage < registerLoweringPackage {
		return 0, &UnsupportedFeatureError{Feature: "register type " + t.Name, Package: b.maxPackage}
	}
	module, identity := b.semanticIdentity(t)
	typeID := b.module.Types.Intern(Type{Kind: TypeRegister, Name: t.Name, Module: module, Identity: identity, BitWidth: uint16(t.RegisterWidth)})
	for _, definition := range b.module.Registers {
		if definition.TypeID == typeID {
			return typeID, nil
		}
	}
	definition := RegisterDefinition{TypeID: typeID, SymbolID: SymbolID(identity), Name: t.Name, Width: uint16(t.RegisterWidth),
		AllocationOrder: t.RegisterAllocationOrder, ByteOrder: t.RegisterByteOrder}
	if definition.AllocationOrder == "" {
		definition.AllocationOrder = "lsb-first"
	}
	if token, ok := b.analyzer.ResolvedTypeDeclarationLocation(t); ok {
		definition.Location = location(token)
	}
	// Reserve the definition before interning field types so a nested
	// register that names this type again cannot recurse.
	index := len(b.module.Registers)
	b.module.Registers = append(b.module.Registers, definition)
	fields := make([]RegisterFieldDefinition, 0, len(t.RegisterFields))
	for ordinal, field := range t.RegisterFields {
		fieldType, err := b.internType(field.Type)
		if err != nil {
			return 0, err
		}
		access := string(field.Access)
		if access == "" {
			access = string(sema.RegisterReadWrite)
		}
		fields = append(fields, RegisterFieldDefinition{ID: RegisterFieldID(ordinal), Name: field.Name, BitOffset: uint16(field.BitOffset),
			Width: uint16(field.Width), Type: fieldType, Access: access, Shadow: string(field.Shadow), Unit: field.Unit, Reserved: field.Name == "_", Location: location(field.Token)})
	}
	b.module.Registers[index].Fields = fields
	return typeID, nil
}

func registerDefinition(module *Module, typeID TypeID) (RegisterDefinition, bool) {
	for _, definition := range module.Registers {
		if definition.TypeID == typeID {
			return definition, true
		}
	}
	return RegisterDefinition{}, false
}

// registerHardwareBoundary fails closed for a field step on a hardware
// instance: hardware operations are MD-077 § 18(4) work. A field of `self` in
// a register impl lowers as a software register field; the impl member's
// lowered body is its software-receiver variant, and a hardware instance
// never reaches it (its receiver type has no Semantic IR representation yet).
func (fb *functionBuilder) registerHardwareBoundary(plan sema.ResolvedRegisterFieldPlan, token lexer.Token) error {
	if plan.Root != sema.RegisterRootHardware {
		return nil
	}
	return fb.unsupported("hardware register operation register.hw.* on field "+plan.Register.Name+"."+plan.Field.Name+" (MD-077 § 18(4))", token)
}

// hardwareInstanceMemberBoundary fails closed for a member of impl T used
// through a HardwareInstance[T]: its realization needs register.hw.bind and
// the hardware operation family (MD-077 § 18(3)-(4)).
func (fb *functionBuilder) hardwareInstanceMemberBoundary(expr *ast.MemberExpression) error {
	object, ok := fb.owner.analyzer.TypeOf(expr.Object)
	if ok && object.Kind == sema.ReferenceType && object.Element != nil {
		object = *object.Element
	}
	if !ok || object.Kind != sema.HardwareInstanceType {
		return nil
	}
	return fb.unsupported("member "+expr.Property.Value+" of hardware register instance "+expr.Object.String()+" (register.hw.*, MD-077 § 18(3)-(4))", expr.Token)
}

// buildRegisterFieldRead lowers a detached field read to
// register.value.extract.
func (fb *functionBuilder) buildRegisterFieldRead(expr *ast.MemberExpression, plan sema.ResolvedRegisterFieldPlan) (builtValue, error) {
	if err := fb.registerHardwareBoundary(plan, expr.Token); err != nil {
		return builtValue{}, err
	}
	source, err := fb.buildExpr(expr.Object, 0)
	if err != nil {
		return builtValue{}, err
	}
	if typ, ok := fb.owner.module.Types.Lookup(source.typ); !ok || typ.Kind != TypeRegister {
		return builtValue{}, fb.unsupported("register field read through a non-value operand", expr.Token)
	}
	fieldType, err := fb.owner.internType(plan.Field.Type)
	if err != nil {
		return builtValue{}, err
	}
	return fb.result(Operation{Kind: OpRegisterValueExtract, Operands: []ValueID{source.id}, RegisterField: RegisterFieldID(plan.FieldIndex), Location: location(expr.Token)}, fieldType), nil
}

// registerAssignmentStep is one projection of a field assignment target:
// a stored struct field or a register field.
type registerAssignmentStep struct {
	register     bool
	structPlan   sema.ResolvedStructMemberPlan
	registerPlan sema.ResolvedRegisterFieldPlan
}

// buildRegisterFieldAssignment lowers `root.(field)*.Field = value` whose
// leaf is a detached register field: the value is evaluated and fitted to
// the field carrier first, then the root is loaded once, projected down,
// rebuilt leaf-to-root with register.value.update / struct.replace-field and
// stored once.
func (fb *functionBuilder) buildRegisterFieldAssignment(stmt *ast.AssignmentStatement, target *ast.MemberExpression) error {
	var reversed []registerAssignmentStep
	var root *ast.Identifier
	for expression := ast.Expression(target); root == nil; {
		switch current := expression.(type) {
		case *ast.MemberExpression:
			if plan, ok := fb.owner.analyzer.ResolvedRegisterFieldOf(current); ok {
				if err := fb.registerHardwareBoundary(plan, current.Token); err != nil {
					return err
				}
				reversed = append(reversed, registerAssignmentStep{register: true, registerPlan: plan})
			} else if plan, ok := fb.owner.analyzer.ResolvedStructMemberOf(current); ok && plan.Kind == sema.MemberStoredField && plan.Action == sema.StructFieldCopyTrivial {
				reversed = append(reversed, registerAssignmentStep{structPlan: plan})
			} else {
				return fb.unsupported("non-trivial or unresolved register field assignment path", current.Token)
			}
			expression = current.Object
		case *ast.Identifier:
			root = current
		default:
			return fb.unsupported("register field assignment through a non-local root", expressionToken(expression))
		}
	}
	steps := make([]registerAssignmentStep, len(reversed))
	for index := range reversed {
		steps[len(reversed)-1-index] = reversed[index]
	}
	proof, ok := fb.owner.analyzer.ResolvedRegisterWriteOf(stmt)
	if !ok {
		return fmt.Errorf("missing resolved register write proof")
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(root)
	if !ok {
		return fmt.Errorf("missing resolved register root binding")
	}
	bind, ok := fb.bindings[fact.ID]
	if !ok || bind.storage == 0 || !bind.mutable || !fb.storageAllowed(bind.typ) {
		return fb.unsupported("register field assignment without trivial mutable root storage", root.Token)
	}
	loc := location(stmt.Token)
	leaf := steps[len(steps)-1].registerPlan
	replacement, err := fb.buildRegisterFieldValue(stmt, leaf, proof)
	if err != nil {
		return err
	}
	current := fb.result(Operation{Kind: OpStorageLoad, Storage: bind.storage, Location: loc}, bind.typ)
	parents := make([]builtValue, len(steps))
	for index, step := range steps {
		parents[index] = current
		if index == len(steps)-1 {
			break
		}
		if step.register {
			fieldType, internErr := fb.owner.internType(step.registerPlan.Field.Type)
			if internErr != nil {
				return internErr
			}
			current = fb.result(Operation{Kind: OpRegisterValueExtract, Operands: []ValueID{current.id}, RegisterField: RegisterFieldID(step.registerPlan.FieldIndex), Location: loc}, fieldType)
			continue
		}
		fieldType, internErr := fb.owner.internType(step.structPlan.MemberType)
		if internErr != nil {
			return internErr
		}
		current = fb.result(Operation{Kind: OpStructExtractField, Operands: []ValueID{current.id}, StructField: StructFieldID(step.structPlan.FieldID), StructActions: []StructFieldAction{StructActionCopyTrivial}, Location: loc}, fieldType)
	}
	write := registerWriteProof(proof)
	for index := len(steps) - 1; index >= 0; index-- {
		step := steps[index]
		if step.register {
			replacement = fb.result(Operation{Kind: OpRegisterValueUpdate, Operands: []ValueID{parents[index].id, replacement.id}, RegisterField: RegisterFieldID(step.registerPlan.FieldIndex), RegisterWrite: write, Location: loc}, parents[index].typ)
			// An enclosing register receives the rebuilt nested value as a
			// value of the nested field's own register type.
			write = RegisterWriteProof{Basis: string(sema.RegisterWriteTyped)}
			continue
		}
		replacement = fb.result(Operation{Kind: OpStructReplaceField, Operands: []ValueID{parents[index].id, replacement.id}, StructField: StructFieldID(step.structPlan.FieldID), Location: loc}, parents[index].typ)
		write = RegisterWriteProof{Basis: string(sema.RegisterWriteTyped)}
	}
	fb.emit(Operation{Kind: OpStorageStore, Storage: bind.storage, Operands: []ValueID{replacement.id}, Location: loc})
	return nil
}

// buildRegisterFieldValue evaluates the written value once and fits it to the
// field's carrier: a one-bit field written with the integer 0 or 1 becomes
// the bool constant, and a differently typed integer proven to fit is
// converted exactly. No conversion ever truncates.
func (fb *functionBuilder) buildRegisterFieldValue(stmt *ast.AssignmentStatement, field sema.ResolvedRegisterFieldPlan, proof sema.ResolvedRegisterWriteProof) (builtValue, error) {
	loc := location(stmt.Token)
	fieldType, err := fb.owner.internType(field.Field.Type)
	if err != nil {
		return builtValue{}, err
	}
	carrier, _ := fb.owner.module.Types.Lookup(fieldType)
	if carrier.Kind == TypeBool && proof.Basis == sema.RegisterWriteConstant {
		set := proof.Low.Sign() != 0
		return fb.result(Operation{Kind: OpConstBool, Bool: &set, Location: loc}, fieldType), nil
	}
	if proof.Basis == sema.RegisterWriteTyped || proof.Basis == sema.RegisterWriteConstant {
		return fb.buildExpr(stmt.Value, fieldType)
	}
	value, err := fb.buildExpr(stmt.Value, 0)
	if err != nil {
		return builtValue{}, err
	}
	if value.typ == fieldType {
		return value, nil
	}
	source, _ := fb.owner.module.Types.Lookup(value.typ)
	if !isBuiltinIntegerType(fb.owner.module.Types, value.typ) || carrier.Kind != TypeUint {
		return builtValue{}, fb.unsupported("register field write of "+source.Name+" into "+carrier.Name, stmt.Token)
	}
	basis := registerFieldIntervalProof
	if source.Kind == TypeUint && !source.TargetSize && source.BitWidth <= carrier.BitWidth {
		basis = compilerWideningProof
	}
	return fb.result(Operation{Kind: OpNumericConvertExact, Operands: []ValueID{value.id}, ConversionProof: &basis, Location: loc}, fieldType), nil
}

// registerFieldIntervalProof is the exactness basis of a conversion into a
// field carrier justified by Sema's proven interval of the written value.
var registerFieldIntervalProof = ConversionProof{Basis: "register-field-interval"}

func registerWriteProof(proof sema.ResolvedRegisterWriteProof) RegisterWriteProof {
	write := RegisterWriteProof{Basis: string(proof.Basis)}
	if proof.Low != nil {
		write.Low, write.High = new(big.Int).Set(proof.Low), new(big.Int).Set(proof.High)
	}
	return write
}

// verifyRegisterDefinitions checks the one nominal module registry and exact
// declaration-order coverage before operations may refer to its field IDs.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a (MD-114 §§2–3,9).
func verifyRegisterDefinitions(module *Module) error {
	seen := map[TypeID]bool{}
	identities := map[string]bool{}
	for _, definition := range module.Registers {
		typ, ok := module.Types.Lookup(definition.TypeID)
		if !ok || typ.Kind != TypeRegister || typ.Identity == "" || definition.SymbolID != SymbolID(typ.Identity) ||
			definition.Name != typ.Name || definition.Width != typ.BitWidth || definition.Width == 0 || seen[definition.TypeID] {
			return fmt.Errorf("invalid register definition !%d", definition.TypeID)
		}
		if identities[typ.Identity] {
			return fmt.Errorf("duplicate register nominal identity %s", typ.Identity)
		}
		identities[typ.Identity] = true
		if definition.ByteOrder != "" && definition.ByteOrder != "little-endian" && definition.ByteOrder != "big-endian" {
			return fmt.Errorf("register has invalid byte order")
		}
		seen[definition.TypeID] = true
		if definition.AllocationOrder != "lsb-first" && definition.AllocationOrder != "msb-first" {
			return fmt.Errorf("register !%d has invalid allocation order %q", definition.TypeID, definition.AllocationOrder)
		}
		occupied := make([]bool, definition.Width)
		names := map[string]bool{}
		allocated := uint32(0)
		for index, field := range definition.Fields {
			if field.Name == "" || field.Reserved != (field.Name == "_") || !field.Reserved && names[field.Name] {
				return fmt.Errorf("register has invalid or duplicate field name/reserved flag")
			}
			names[field.Name] = true
			switch field.Access {
			case "read-write", "read-only", "write-only", "read-clear", "read-set", "write-one-clear", "write-one-set", "write-one-toggle", "write-zero-clear", "write-zero-set", "write-zero-toggle":
			default:
				return fmt.Errorf("register field has invalid access policy")
			}
			if allocated+uint32(field.Width) > uint32(definition.Width) {
				return fmt.Errorf("register fields exceed declared width")
			}
			offset := allocated
			if definition.AllocationOrder == "msb-first" {
				offset = uint32(definition.Width) - allocated - uint32(field.Width)
			}
			if uint32(field.BitOffset) != offset {
				return fmt.Errorf("register field offset disagrees with allocation order")
			}
			allocated += uint32(field.Width)
			if field.ID != RegisterFieldID(index) || field.Width == 0 || uint32(field.BitOffset)+uint32(field.Width) > uint32(definition.Width) {
				return fmt.Errorf("register !%d field #%d has an invalid bit slice", definition.TypeID, index)
			}
			for bit := field.BitOffset; bit < field.BitOffset+field.Width; bit++ {
				if occupied[bit] {
					return fmt.Errorf("register !%d field #%d overlaps another field", definition.TypeID, index)
				}
				occupied[bit] = true
			}
			if field.Shadow != string(sema.RegisterShadowed) && field.Shadow != string(sema.RegisterNoShadow) {
				return fmt.Errorf("register !%d field #%d has invalid shadow policy %q", definition.TypeID, index, field.Shadow)
			}
			if err := verifyRegisterFieldCarrier(module, field); err != nil {
				return fmt.Errorf("register !%d field #%d: %w", definition.TypeID, index, err)
			}
		}
		for _, used := range occupied {
			if !used {
				return fmt.Errorf("register !%d leaves bits unassigned", definition.TypeID)
			}
		}
	}
	for index, typ := range module.Types.All() {
		if typ.Kind == TypeRegister && !seen[TypeID(index+1)] {
			return fmt.Errorf("register type has no definition")
		}
	}
	state := map[TypeID]int{}
	var visit func(TypeID) error
	visit = func(id TypeID) error {
		if state[id] == 1 {
			return fmt.Errorf("recursive register layout")
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		definition, _ := registerDefinition(module, id)
		for _, field := range definition.Fields {
			typ, _ := module.Types.Lookup(field.Type)
			if typ.Kind == TypeRegister {
				if err := visit(field.Type); err != nil {
					return err
				}
			}
		}
		state[id] = 2
		return nil
	}
	for _, definition := range module.Registers {
		if err := visit(definition.TypeID); err != nil {
			return err
		}
	}
	return nil
}

// verifyRegisterFieldCarrier checks the MD-077 § 6 carrier of a field: bool
// for one bit, the smallest standard unsigned integer for bit[N], the
// bit-backed enum or nested register of exactly the field width.
func verifyRegisterFieldCarrier(module *Module, field RegisterFieldDefinition) error {
	carrier, ok := module.Types.Lookup(field.Type)
	if !ok {
		return fmt.Errorf("missing carrier type")
	}
	switch carrier.Kind {
	case TypeBool:
		if field.Width != 1 {
			return fmt.Errorf("bool carrier for a %d-bit field", field.Width)
		}
	case TypeUint:
		if carrier.TargetSize || carrier.BitWidth != smallestUnsignedCarrier(field.Width) {
			return fmt.Errorf("carrier %s is not the smallest unsigned carrier of bit[%d]", carrier.Name, field.Width)
		}
	case TypeEnum, TypeRegister:
		if carrier.BitWidth != field.Width {
			return fmt.Errorf("%s carrier width %d differs from field width %d", carrier.Kind, carrier.BitWidth, field.Width)
		}
	default:
		return fmt.Errorf("unsupported carrier kind %s", carrier.Kind)
	}
	return nil
}

func smallestUnsignedCarrier(width uint16) uint16 {
	for _, carrier := range []uint16{8, 16, 32, 64, 128, 256} {
		if width <= carrier {
			return carrier
		}
	}
	return 0
}

// verifyRegisterValueOperation checks a pure register.value.* operation
// against the register-definition table and, for an update, the write proof.
func verifyRegisterValueOperation(module *Module, op Operation, values map[ValueID]Value) error {
	wantOperands := 1
	if op.Kind == OpRegisterValueUpdate {
		wantOperands = 2
	}
	if len(op.Operands) != wantOperands || len(op.Results) != 1 {
		return fmt.Errorf("invalid %s", op.Kind)
	}
	definition, ok := registerDefinition(module, values[op.Operands[0]].Type)
	if !ok || int(op.RegisterField) >= len(definition.Fields) {
		return fmt.Errorf("%s has no register field #%d", op.Kind, op.RegisterField)
	}
	field := definition.Fields[op.RegisterField]
	if field.Reserved {
		return fmt.Errorf("%s accesses reserved field #%d", op.Kind, op.RegisterField)
	}
	if op.Kind == OpRegisterValueExtract {
		if op.Results[0].Type != field.Type || op.RegisterWrite.Basis != "" {
			return fmt.Errorf("register.value.extract field type mismatch")
		}
		return nil
	}
	if op.Results[0].Type != definition.TypeID || values[op.Operands[1]].Type != field.Type {
		return fmt.Errorf("register.value.update type mismatch")
	}
	carrier, _ := module.Types.Lookup(field.Type)
	switch op.RegisterWrite.Basis {
	case string(sema.RegisterWriteTyped):
		if carrier.Kind == TypeUint {
			return fmt.Errorf("register.value.update of bit[%d] needs a range proof", field.Width)
		}
		if op.RegisterWrite.Low != nil || op.RegisterWrite.High != nil {
			return fmt.Errorf("typed register.value.update carries an interval")
		}
	case string(sema.RegisterWriteConstant), string(sema.RegisterWriteInterval):
		low, high := op.RegisterWrite.Low, op.RegisterWrite.High
		maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(field.Width)), big.NewInt(1))
		if low == nil || high == nil || low.Sign() < 0 || low.Cmp(high) > 0 || high.Cmp(maximum) > 0 {
			return fmt.Errorf("register.value.update proof does not fit bit[%d]", field.Width)
		}
		if op.RegisterWrite.Basis == string(sema.RegisterWriteConstant) && low.Cmp(high) != 0 {
			return fmt.Errorf("register.value.update constant proof is not one value")
		}
	default:
		return fmt.Errorf("register.value.update has unknown proof basis %q", op.RegisterWrite.Basis)
	}
	return nil
}

func formatRegisters(out *strings.Builder, registers []RegisterDefinition) {
	if len(registers) == 0 {
		return
	}
	out.WriteString("\n  registers {\n")
	for _, definition := range registers {
		fmt.Fprintf(out, "    !%d %q width=%d order=%s", definition.TypeID, definition.SymbolID, definition.Width, definition.AllocationOrder)
		if definition.ByteOrder != "" {
			fmt.Fprintf(out, " byte-order=%s", definition.ByteOrder)
		}
		out.WriteString(" {\n")
		for _, field := range definition.Fields {
			fmt.Fprintf(out, "      #%d %s:!%d bits=[%d+%d] access=%s", field.ID, field.Name, field.Type, field.BitOffset, field.Width, field.Access)
			if field.Shadow == string(sema.RegisterShadowed) {
				out.WriteString(" shadow")
			}
			if field.Unit != "" {
				fmt.Fprintf(out, " unit=%q", field.Unit)
			}
			if field.Reserved {
				out.WriteString(" reserved")
			}
			fmt.Fprintf(out, " %s\n", formatLocation(field.Location))
		}
		out.WriteString("    }\n")
	}
	out.WriteString("  }\n")
}

func formatRegisterValueOperation(out *strings.Builder, op Operation) {
	if op.Kind == OpRegisterValueExtract {
		fmt.Fprintf(out, " %%%d field=#%d", op.Operands[0], op.RegisterField)
		return
	}
	fmt.Fprintf(out, " %%%d field=#%d value=%%%d proof=%s", op.Operands[0], op.RegisterField, op.Operands[1], op.RegisterWrite.Basis)
	if op.RegisterWrite.Low != nil {
		fmt.Fprintf(out, "[%s..%s]", op.RegisterWrite.Low, op.RegisterWrite.High)
	}
}

// receiverRegisterPath resolves expr as register field steps on the
// receiver of a register impl member: an implicit field name or `self`
// followed by field steps. On this software-receiver lowering they are
// ordinary software register fields (MD-077 §§ 1, 5).
func (fb *functionBuilder) receiverRegisterPath(expr ast.Expression) ([]sema.ResolvedRegisterFieldPlan, bool) {
	if fb.receiver.typ == 0 {
		return nil, false
	}
	var steps []*ast.MemberExpression
	current := expr
	for {
		member, ok := current.(*ast.MemberExpression)
		if !ok {
			break
		}
		steps = append([]*ast.MemberExpression{member}, steps...)
		current = member.Object
	}
	identifier, ok := current.(*ast.Identifier)
	if !ok {
		return nil, false
	}
	use, ok := fb.owner.analyzer.ResolvedImplicitReceiverOf(identifier)
	if !ok {
		return nil, false
	}
	var plans []sema.ResolvedRegisterFieldPlan
	switch {
	case use.RegisterField != nil:
		plans = append(plans, *use.RegisterField)
	case !use.Self:
		return nil, false
	}
	for _, step := range steps {
		plan, ok := fb.owner.analyzer.ResolvedRegisterFieldOf(step)
		if !ok {
			return nil, false
		}
		plans = append(plans, plan)
	}
	return plans, len(plans) > 0
}

// lowerReceiverRegisterRead reads the whole receiver register through its
// Place and extracts the field path.
func (fb *functionBuilder) lowerReceiverRegisterRead(expr ast.Expression, plans []sema.ResolvedRegisterFieldPlan) (builtValue, error) {
	loc := locationFromExpression(expr)
	root, _, err := fb.receiverRootPlace(loc)
	if err != nil {
		return builtValue{}, err
	}
	placeType, _ := fb.owner.module.Types.Lookup(root.typ)
	current := fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{root.id}, Location: loc}, placeType.Element)
	for _, plan := range plans {
		fieldType, err := fb.owner.internType(plan.Field.Type)
		if err != nil {
			return builtValue{}, err
		}
		current = fb.result(Operation{Kind: OpRegisterValueExtract, Operands: []ValueID{current.id}, RegisterField: RegisterFieldID(plan.FieldIndex), Location: loc}, fieldType)
	}
	return current, nil
}

// lowerReceiverRegisterWrite writes a field of a mutable receiver register:
// the destination Place first, then the fitted value, then one read of the
// whole register, the leaf-to-root updates and one write.
func (fb *functionBuilder) lowerReceiverRegisterWrite(stmt *ast.AssignmentStatement, plans []sema.ResolvedRegisterFieldPlan) error {
	loc := location(stmt.Token)
	if stmt.Operator != "=" {
		return fb.unsupported("compound assignment to a receiver register field", stmt.Token)
	}
	proof, ok := fb.owner.analyzer.ResolvedRegisterWriteOf(stmt)
	if !ok {
		return fmt.Errorf("missing resolved register write proof")
	}
	root, mutable, err := fb.receiverRootPlace(loc)
	if err != nil {
		return err
	}
	if !mutable {
		return fb.unsupported("write through a shared receiver", stmt.Token)
	}
	replacement, err := fb.buildRegisterFieldValue(stmt, plans[len(plans)-1], proof)
	if err != nil {
		return err
	}
	placeType, _ := fb.owner.module.Types.Lookup(root.typ)
	current := fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{root.id}, Location: loc}, placeType.Element)
	parents := make([]builtValue, len(plans))
	for index, plan := range plans {
		parents[index] = current
		if index == len(plans)-1 {
			break
		}
		fieldType, err := fb.owner.internType(plan.Field.Type)
		if err != nil {
			return err
		}
		current = fb.result(Operation{Kind: OpRegisterValueExtract, Operands: []ValueID{current.id}, RegisterField: RegisterFieldID(plan.FieldIndex), Location: loc}, fieldType)
	}
	write := registerWriteProof(proof)
	for index := len(plans) - 1; index >= 0; index-- {
		replacement = fb.result(Operation{Kind: OpRegisterValueUpdate, Operands: []ValueID{parents[index].id, replacement.id}, RegisterField: RegisterFieldID(plans[index].FieldIndex), RegisterWrite: write, Location: loc}, parents[index].typ)
		write = RegisterWriteProof{Basis: string(sema.RegisterWriteTyped)}
	}
	fb.emit(Operation{Kind: OpPlaceWrite, Operands: []ValueID{root.id, replacement.id}, Location: loc})
	return nil
}

// Register bit-pattern conversions (MD-110 §§ 3–5, 7; semantic_ir.md
// § 21(6)). register.value.fits-bits is the runtime fit predicate,
// register.value.from-bits the exact construction under a constant,
// source-domain, bit-extraction or guarded proof, and register.value.to-bits
// the total zero/sign-extending interpretation. All three are pure.
const (
	OpRegisterValueFitsBits OpKind = "register.value.fits-bits"
	OpRegisterValueFromBits OpKind = "register.value.from-bits"
	OpRegisterValueToBits   OpKind = "register.value.to-bits"
)

// RegisterConversionFacts are the destination register, the proof basis of
// a from-bits and the signed interpretation of a to-bits.
type RegisterConversionFacts struct {
	Register TypeID
	Basis    string
	Signed   bool
}

// buildRegisterConversion lowers an explicit `T(x)` or `IntegerType(r)` that
// Sema resolved. The checked form (guarded) needs the try failure lowering
// that checked integer conversions also wait for.
func (fb *functionBuilder) buildRegisterConversion(call *ast.CallExpression, conversion sema.ResolvedRegisterConversion) (builtValue, error) {
	loc := location(call.Token)
	registerType, err := fb.owner.internType(conversion.Register)
	if err != nil {
		return builtValue{}, err
	}
	integerType, err := fb.owner.internType(conversion.Integer)
	if err != nil {
		return builtValue{}, err
	}
	if conversion.Kind == sema.RegisterToBits {
		source, err := fb.buildExpr(call.Arguments[0], registerType)
		if err != nil {
			return builtValue{}, err
		}
		return fb.result(Operation{Kind: OpRegisterValueToBits, Operands: []ValueID{source.id},
			RegisterConversion: &RegisterConversionFacts{Register: registerType, Signed: conversion.Signed}, Location: loc}, integerType), nil
	}
	if conversion.Basis == sema.RegisterFitGuarded {
		return builtValue{}, fb.unsupported("checked integer-to-register conversion "+conversion.Register.Name+"(...) (needs try, as checked integer conversions)", call.Token)
	}
	value, err := fb.buildExpr(call.Arguments[0], integerType)
	if err != nil {
		return builtValue{}, err
	}
	return fb.result(Operation{Kind: OpRegisterValueFromBits, Operands: []ValueID{value.id},
		RegisterConversion: &RegisterConversionFacts{Register: registerType, Basis: string(conversion.Basis)}, Location: loc}, registerType), nil
}

// integerShape is the signedness and bit width of a Semantic IR integer.
func integerShape(module *Module, id TypeID) (signed bool, width uint16, ok bool) {
	typ, found := module.Types.Lookup(id)
	if !found {
		return false, 0, false
	}
	switch typ.Kind {
	case TypeByte:
		return false, 8, true
	case TypeInt, TypeUint:
		width = typ.BitWidth
		if typ.TargetSize {
			width = module.PointerWidthBits
		}
		// A target-sized integer without a CompilationPlan has width 0.
		return typ.Kind == TypeInt, width, true
	}
	return false, 0, false
}

// verifyRegisterConversionOperation checks one conversion's local contract.
func verifyRegisterConversionOperation(module *Module, op Operation, values map[ValueID]Value) error {
	facts := op.RegisterConversion
	if facts == nil || len(op.Operands) != 1 || len(op.Results) != 1 {
		return fmt.Errorf("%s requires one operand, one result and conversion facts", op.Kind)
	}
	definition, ok := registerDefinition(module, facts.Register)
	if !ok {
		return fmt.Errorf("%s names no register type", op.Kind)
	}
	operand, result := values[op.Operands[0]].Type, op.Results[0].Type
	switch op.Kind {
	case OpRegisterValueFitsBits, OpRegisterValueFromBits:
		if _, _, integer := integerShape(module, operand); !integer {
			return fmt.Errorf("%s operand must be a resolved primitive integer", op.Kind)
		}
		if op.Kind == OpRegisterValueFitsBits {
			if !typeHasKind(module.Types, result, TypeBool) || facts.Basis != "" {
				return fmt.Errorf("register.value.fits-bits returns bool and carries no proof")
			}
			return nil
		}
		if result != facts.Register {
			return fmt.Errorf("register.value.from-bits must produce its destination register")
		}
		switch facts.Basis {
		case string(sema.RegisterFitConstant), string(sema.RegisterFitSourceDomain), string(sema.RegisterFitBitExtraction), string(sema.RegisterFitGuarded):
		default:
			return fmt.Errorf("register.value.from-bits has unknown proof basis %q", facts.Basis)
		}
	case OpRegisterValueToBits:
		if values[op.Operands[0]].Type != facts.Register || facts.Basis != "" {
			return fmt.Errorf("register.value.to-bits takes its register and carries no proof")
		}
		signed, width, integer := integerShape(module, result)
		if !integer || width == 0 || signed != facts.Signed {
			return fmt.Errorf("register.value.to-bits result must be an integer of its recorded signedness")
		}
		if width < definition.Width {
			return fmt.Errorf("register.value.to-bits result width %d is narrower than register width %d", width, definition.Width)
		}
	}
	return nil
}

// registerBitsFit is the MD-110 § 3(2) fit rule.
func registerBitsFit(value *big.Int, width uint16) bool {
	if value.Sign() >= 0 {
		return value.BitLen() <= int(width)
	}
	minimum := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), uint(width-1)))
	return value.Cmp(minimum) >= 0
}

// verifyRegisterConversionProofs checks every from-bits proof against its
// evidence: a fitting constant, a fitting source domain, a bit extraction of
// at most N bits, or a dominating true edge of a fits-bits predicate on the
// same operand and register.
func verifyRegisterConversionProofs(module *Module, fn *Function, dom map[BlockID]map[BlockID]bool) error {
	definitions := map[ValueID]Operation{}
	values := map[ValueID]Value{}
	for _, parameter := range fn.Parameters {
		values[parameter.Value.ID] = parameter.Value
	}
	for _, block := range fn.Blocks {
		for _, parameter := range block.Parameters {
			values[parameter.ID] = parameter
		}
		for _, op := range block.Operations {
			for _, result := range op.Results {
				definitions[result.ID], values[result.ID] = op, result
			}
		}
	}
	// guardedBlocks are the blocks a true edge of fits-bits(operand, register) enters.
	type guardKey struct {
		operand  ValueID
		register TypeID
	}
	guards := map[guardKey][]BlockID{}
	predecessors := map[BlockID]int{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, successor := range op.Successors {
				predecessors[successor.Block]++
			}
		}
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind != OpCondBranch || len(op.Operands) != 1 || len(op.Successors) != 2 {
				continue
			}
			predicate, found := definitions[op.Operands[0]]
			if !found || predicate.Kind != OpRegisterValueFitsBits {
				continue
			}
			key := guardKey{predicate.Operands[0], predicate.RegisterConversion.Register}
			guards[key] = append(guards[key], op.Successors[0].Block)
		}
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind != OpRegisterValueFromBits {
				continue
			}
			definition, _ := registerDefinition(module, op.RegisterConversion.Register)
			operand := op.Operands[0]
			switch sema.RegisterFitBasis(op.RegisterConversion.Basis) {
			case sema.RegisterFitConstant:
				constant := definitions[operand]
				if constant.Kind != OpConstInt || constant.Integer == nil || !registerBitsFit(constant.Integer, definition.Width) {
					return fmt.Errorf("register.value.from-bits constant proof without a fitting constant operand")
				}
			case sema.RegisterFitSourceDomain:
				signed, width, _ := integerShape(module, values[operand].Type)
				// Every value of a width-W integer fits when W <= N (unsigned)
				// or W <= N (signed: its minimum is -2^(W-1) >= -2^(N-1)).
				if width == 0 || width > definition.Width {
					return fmt.Errorf("register.value.from-bits source-domain proof: %d-bit source (signed=%t) exceeds register width %d", width, signed, definition.Width)
				}
			case sema.RegisterFitBitExtraction:
				extraction := definitions[operand]
				if extraction.Kind != OpIntBitExtract || extraction.Integer == nil || extraction.Integer.Int64() > int64(definition.Width) {
					return fmt.Errorf("register.value.from-bits bit-extraction proof without an extraction of at most %d bits", definition.Width)
				}
			case sema.RegisterFitGuarded:
				guarded := false
				// The true edge proves the fit only where it is the sole way in.
				for _, entry := range guards[guardKey{operand, op.RegisterConversion.Register}] {
					if predecessors[entry] == 1 && (entry == block.ID || dom[block.ID][entry]) {
						guarded = true
					}
				}
				if !guarded {
					return fmt.Errorf("register.value.from-bits guarded proof without a dominating successful fits-bits on the same operand and register")
				}
			}
		}
	}
	return nil
}

func formatRegisterConversionOperation(out *strings.Builder, op Operation) {
	fmt.Fprintf(out, " %%%d", op.Operands[0])
	if facts := op.RegisterConversion; facts != nil {
		fmt.Fprintf(out, " register=!%d", facts.Register)
		if facts.Basis != "" {
			fmt.Fprintf(out, " proof=%s", facts.Basis)
		}
		if op.Kind == OpRegisterValueToBits {
			fmt.Fprintf(out, " signed=%t", facts.Signed)
		}
	}
}
