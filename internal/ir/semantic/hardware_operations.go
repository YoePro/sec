package semantic

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// Hardware register operations (MD-077 §§ 9–12; semantic_ir.md § 21). They
// are produced only from Sema's resolved logical requests and register
// operations, on a hardware-instance<T> authority value, and never on a
// Place. A use of a bound instance reaches impl T members through their
// hardware variant (`T.M[hardware]`), whose `self` is that authority. No
// operation chooses a physical access: the physical plan needs verified
// MD-111 operation plans, so every request records plan=unverified and Sec
// MLIR emission fails closed for it (§ 11(4)).
const (
	OpHardwareBind          OpKind = "register.hw.bind"
	OpHardwareObserve       OpKind = "register.hw.observe"
	OpHardwareRequestWrite  OpKind = "register.hw.request_write"
	OpHardwareShadowObserve OpKind = "register.hw.shadow_observe"
	OpHardwareShadowIntent  OpKind = "register.hw.shadow_intent"
	OpHardwareCommit        OpKind = "register.hw.commit"
	OpHardwareShadowKnown   OpKind = "register.hw.shadow_known"
)

// ArgumentHardwareInstance lends a hardware-instance authority to the
// hardware variant of an impl member for the duration of the call; it is
// neither a copy nor a move of the authority (MD-077 § 3(5)).
const ArgumentHardwareInstance ArgumentAction = "hardware-instance"

// HardwarePlanUnverified marks a request whose physical plan is not yet
// established; it is the only plan state Semantic IR admits today.
const HardwarePlanUnverified = "unverified"

// HardwareRequest is the logical request of one register.hw.* operation as
// Sema resolved it: the request kind (observe, destructive-observe,
// write-value, action, write-register, commit, shadow-known), the read effect
// or write action, the field's shadow policy, the MD-112 action mask and
// value dependence, whether it addresses the whole register, and the plan.
type HardwareRequest struct {
	Kind           string
	Effect         string
	Shadow         string
	ActionMask     string
	ValueDependent bool
	Whole          bool
	Plan           string
}

func hardwareRequestOf(request sema.ResolvedRegisterHardwareRequest) *HardwareRequest {
	return &HardwareRequest{Kind: string(request.Kind), Effect: request.Effect, Shadow: string(request.Shadow), ActionMask: request.ActionMask, ValueDependent: request.ValueDependent, Plan: HardwarePlanUnverified}
}

// hardwareVariantID names the hardware variant of an impl member.
func hardwareVariantID(id FunctionID) FunctionID {
	text := string(id)
	if open := strings.Index(text, "("); open >= 0 {
		return FunctionID(text[:open] + "[hardware]" + text[open:])
	}
	return FunctionID(text + "[hardware]")
}

// registerMemberKey is Sema's RegisterMemberRequirement key of a member.
func registerMemberKey(function sema.Function) string {
	return function.ImplTarget + "." + strings.TrimPrefix(function.Name, function.ImplTarget+".")
}

// demandHardwareVariant schedules, once, the hardware variant of an impl T
// member called on a hardware instance and returns its identity.
func (b *builder) demandHardwareVariant(function sema.Function, token lexer.Token) (FunctionID, error) {
	if hardwareOnly, reason, ok := b.analyzer.RegisterMemberRequirement(registerMemberKey(function)); ok && !hardwareOnly {
		return "", &UnsupportedFeatureError{Feature: "software-only register member " + function.Name + " on a hardware instance (" + reason + ")", Package: b.maxPackage, Location: location(token)}
	}
	if len(function.GenericSubstitution) != 0 || len(function.GenericParameters) != 0 {
		return "", &UnsupportedFeatureError{Feature: "generic member " + function.Name + " on a hardware instance", Package: b.maxPackage, Location: location(token)}
	}
	id := hardwareVariantID(functionID(function, b.module.Types))
	if b.hardwareVariants == nil {
		b.hardwareVariants = map[FunctionID]bool{}
	}
	if !b.hardwareVariants[id] {
		b.hardwareVariants[id] = true
		b.specializations.pending = append(b.specializations.pending, &specialization{id: id, function: function, token: token, hardware: true})
	}
	return id, nil
}

// addHardwareReceiverParameter makes `self` the hardware-instance<T>
// authority of a hardware variant.
func (fb *functionBuilder) addHardwareReceiverParameter(function sema.Function, decl *ast.FunctionDeclaration) error {
	facts, ok := fb.owner.analyzer.ReceiverReferenceFactsOf(function)
	if !ok || facts.Referent.Kind != sema.RegisterType {
		return fb.unsupported("hardware variant of a non-register impl member", decl.Token)
	}
	register, err := fb.owner.internType(facts.Referent)
	if err != nil {
		return locateUnsupportedType(err, decl.Token)
	}
	instance := fb.owner.module.Types.Intern(Type{Kind: TypeHardwareInstance, Element: register})
	value := fb.newValue(instance, OwnershipImmediate, location(decl.Token))
	fb.fn.Parameters = append(fb.fn.Parameters, Parameter{Name: "self", Value: value, Location: location(decl.Token)})
	fb.receiver = builtValue{id: value.ID, typ: instance}
	return nil
}

// hardwareReceiver reports whether the function being built is a hardware
// variant, whose receiver is an instance authority rather than a reference.
func (fb *functionBuilder) hardwareReceiver() bool {
	if fb.receiver.typ == 0 {
		return false
	}
	typ, ok := fb.owner.module.Types.Lookup(fb.receiver.typ)
	return ok && typ.Kind == TypeHardwareInstance
}

// bindHardwareInstance yields the authority of the binding an identifier
// names (register.hw.bind).
func (fb *functionBuilder) bindHardwareInstance(expression ast.Expression) (builtValue, bool) {
	binding, ok := fb.hardwareBindingRoot(expression)
	if !ok {
		return builtValue{}, false
	}
	return fb.result(Operation{Kind: OpHardwareBind, HardwareBinding: binding.ID, Location: locationFromExpression(expression)}, binding.Instance), true
}

// hardwareInstanceOperand resolves the instance an operation applies to:
// the hardware variant's own receiver for `self`, an implicit member or a
// bare call, or a bound instance named directly.
func (fb *functionBuilder) hardwareInstanceOperand(object ast.Expression) (builtValue, bool) {
	if object == nil || fb.isReceiverSelf(object) {
		if fb.hardwareReceiver() {
			return fb.receiver, true
		}
		return builtValue{}, false
	}
	return fb.bindHardwareInstance(object)
}

// hardwareReceiverArgument builds the instance argument of a member call on
// a hardware instance; false means an ordinary receiver.
func (fb *functionBuilder) hardwareReceiverArgument(call *ast.CallExpression) (builtValue, bool) {
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok {
		return fb.hardwareInstanceOperand(nil)
	}
	return fb.hardwareInstanceOperand(member.Object)
}

func (fb *functionBuilder) hardwareFieldPlan(plans []sema.ResolvedRegisterFieldPlan, token lexer.Token) (sema.ResolvedRegisterFieldPlan, error) {
	if len(plans) != 1 {
		return sema.ResolvedRegisterFieldPlan{}, fb.unsupported("nested register field path on a hardware instance", token)
	}
	if plans[0].Request == nil {
		return sema.ResolvedRegisterFieldPlan{}, fmt.Errorf("hardware field %s.%s has no resolved request", plans[0].Register.Name, plans[0].Field.Name)
	}
	return plans[0], nil
}

// lowerHardwareFieldRead observes one field (register.hw.observe) and, for
// a shadowed field, records the observation (register.hw.shadow_observe).
func (fb *functionBuilder) lowerHardwareFieldRead(instance builtValue, plans []sema.ResolvedRegisterFieldPlan, token lexer.Token) (builtValue, error) {
	plan, err := fb.hardwareFieldPlan(plans, token)
	if err != nil {
		return builtValue{}, err
	}
	fieldType, err := fb.owner.internType(plan.Field.Type)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(token)
	request := hardwareRequestOf(*plan.Request)
	value := fb.result(Operation{Kind: OpHardwareObserve, Operands: []ValueID{instance.id}, RegisterField: RegisterFieldID(plan.FieldIndex), HardwareRequest: request, Location: loc}, fieldType)
	if plan.Field.Shadow == sema.RegisterShadowed {
		fb.emit(Operation{Kind: OpHardwareShadowObserve, Operands: []ValueID{instance.id, value.id}, RegisterField: RegisterFieldID(plan.FieldIndex), HardwareRequest: request, Location: loc})
	}
	return value, nil
}

// lowerHardwareFieldWrite evaluates and fits the value, then requests the
// write: register.hw.shadow_intent for a shadowed field, otherwise
// register.hw.request_write. A no-op request emits no hardware operation
// (MD-112 § 3(1)).
func (fb *functionBuilder) lowerHardwareFieldWrite(instance builtValue, stmt *ast.AssignmentStatement, plans []sema.ResolvedRegisterFieldPlan) error {
	if stmt.Operator != "=" {
		return fb.unsupported("compound assignment to a hardware register field", stmt.Token)
	}
	plan, err := fb.hardwareFieldPlan(plans, stmt.Token)
	if err != nil {
		return err
	}
	proof, ok := fb.owner.analyzer.ResolvedRegisterWriteOf(stmt)
	if !ok {
		return fmt.Errorf("missing resolved register write proof")
	}
	value, err := fb.buildRegisterFieldValue(stmt, plan, proof)
	if err != nil {
		return err
	}
	if plan.Request.Kind == sema.RegisterRequestNoOp {
		return nil
	}
	kind := OpHardwareRequestWrite
	if plan.Field.Shadow == sema.RegisterShadowed {
		kind = OpHardwareShadowIntent
	}
	fb.emit(Operation{Kind: kind, Operands: []ValueID{instance.id, value.id}, RegisterField: RegisterFieldID(plan.FieldIndex), RegisterWrite: registerWriteProof(proof), HardwareRequest: hardwareRequestOf(*plan.Request), Location: location(stmt.Token)})
	return nil
}

// buildHardwareRegisterOperation lowers Read(), Write(), Write(value) and
// IsShadowKnown(T.Field) on a hardware instance.
func (fb *functionBuilder) buildHardwareRegisterOperation(call *ast.CallExpression, operation sema.ResolvedRegisterOperation) (builtValue, error) {
	var object ast.Expression
	if member, ok := call.Callee.(*ast.MemberExpression); ok {
		object = member.Object
	}
	instance, ok := fb.hardwareInstanceOperand(object)
	if !ok {
		return builtValue{}, fb.unsupported("hardware register operation "+string(operation.Kind)+" without a hardware instance receiver", call.Token)
	}
	register, err := fb.owner.internType(operation.Register)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(call.Token)
	switch operation.Kind {
	case sema.RegisterOperationRead:
		value := fb.result(Operation{Kind: OpHardwareObserve, Operands: []ValueID{instance.id}, HardwareRequest: &HardwareRequest{Kind: string(sema.RegisterRequestObserve), Whole: true, Plan: HardwarePlanUnverified}, Location: loc}, register)
		if registerHasShadow(operation.Register) {
			fb.emit(Operation{Kind: OpHardwareShadowObserve, Operands: []ValueID{instance.id, value.id}, HardwareRequest: &HardwareRequest{Kind: string(sema.RegisterRequestObserve), Whole: true, Plan: HardwarePlanUnverified}, Location: loc})
		}
		return value, nil
	case sema.RegisterOperationCommit:
		fb.emit(Operation{Kind: OpHardwareCommit, Operands: []ValueID{instance.id}, HardwareRequest: &HardwareRequest{Kind: "commit", Whole: true, Plan: HardwarePlanUnverified}, Location: loc})
		return builtValue{}, nil
	case sema.RegisterOperationWriteWhole:
		if len(call.Arguments) != 1 {
			return builtValue{}, fmt.Errorf("Write(value) without its value")
		}
		value, err := fb.buildExpr(call.Arguments[0], register)
		if err != nil {
			return builtValue{}, err
		}
		fb.emit(Operation{Kind: OpHardwareRequestWrite, Operands: []ValueID{instance.id, value.id}, HardwareRequest: &HardwareRequest{Kind: "write-register", Whole: true, Plan: HardwarePlanUnverified}, Location: loc})
		return builtValue{}, nil
	case sema.RegisterOperationShadowKnown:
		if len(operation.FieldPath) != 1 {
			return builtValue{}, fb.unsupported("IsShadowKnown on a nested register field path", call.Token)
		}
		index := -1
		for position, field := range operation.Register.RegisterFields {
			if field.Name == operation.FieldPath[0].Name {
				index = position
			}
		}
		if index < 0 {
			return builtValue{}, fmt.Errorf("IsShadowKnown field %s not in %s", operation.FieldPath[0].Name, operation.Register.Name)
		}
		boolType := fb.owner.module.Types.Intern(Type{Kind: TypeBool, Name: "bool", BitWidth: 1})
		return fb.result(Operation{Kind: OpHardwareShadowKnown, Operands: []ValueID{instance.id}, RegisterField: RegisterFieldID(index), HardwareRequest: &HardwareRequest{Kind: "shadow-known", Shadow: string(sema.RegisterShadowed), Plan: HardwarePlanUnverified}, Location: loc}, boolType), nil
	}
	return builtValue{}, fb.unsupported("hardware register operation "+string(operation.Kind), call.Token)
}

func registerHasShadow(register sema.Type) bool {
	for _, field := range register.RegisterFields {
		if field.Shadow == sema.RegisterShadowed {
			return true
		}
	}
	return false
}

func formatHardwareOperation(out *strings.Builder, op Operation) {
	if op.Kind == OpHardwareBind {
		fmt.Fprintf(out, " #%d", op.HardwareBinding)
		return
	}
	for index, operand := range op.Operands {
		if index > 0 {
			out.WriteString(",")
		}
		fmt.Fprintf(out, " %%%d", operand)
	}
	request := op.HardwareRequest
	if request == nil {
		return
	}
	if request.Whole {
		out.WriteString(" whole")
	} else {
		fmt.Fprintf(out, " field=#%d", op.RegisterField)
	}
	fmt.Fprintf(out, " request=%s", request.Kind)
	if request.Effect != "" {
		fmt.Fprintf(out, " effect=%s", request.Effect)
	}
	if request.Shadow != "" {
		fmt.Fprintf(out, " shadow=%s", request.Shadow)
	}
	if request.ActionMask != "" {
		fmt.Fprintf(out, " mask=%s", request.ActionMask)
	}
	if request.ValueDependent {
		out.WriteString(" value-dependent")
	}
	if op.RegisterWrite.Basis != "" {
		fmt.Fprintf(out, " proof=%s", op.RegisterWrite.Basis)
	}
	fmt.Fprintf(out, " plan=%s", request.Plan)
}

// verifyHardwareOperation checks a register.hw.* operation: its instance
// operand, the register field it names, its request and value/result types.
// It verifies preservation of Sema's facts, not a physical plan (§ 11(3)).
func verifyHardwareOperation(module *Module, op Operation, values map[ValueID]Value) error {
	if op.Kind == OpHardwareBind {
		if len(op.Operands) != 0 || len(op.Results) != 1 || op.HardwareBinding == 0 || int(op.HardwareBinding) > len(module.HardwareBindings) {
			return fmt.Errorf("invalid register.hw.bind")
		}
		if op.Results[0].Type != module.HardwareBindings[op.HardwareBinding-1].Instance {
			return fmt.Errorf("register.hw.bind result is not the binding's hardware instance")
		}
		return nil
	}
	if len(op.Operands) == 0 || op.HardwareRequest == nil || op.HardwareRequest.Plan != HardwarePlanUnverified {
		return fmt.Errorf("%s requires an instance operand and an admitted request plan", op.Kind)
	}
	instance, ok := module.Types.Lookup(values[op.Operands[0]].Type)
	if !ok || instance.Kind != TypeHardwareInstance {
		return fmt.Errorf("%s operand is not a hardware instance", op.Kind)
	}
	definition, ok := registerDefinition(module, instance.Element)
	if !ok {
		return fmt.Errorf("%s instance names no register definition", op.Kind)
	}
	request := op.HardwareRequest
	var field RegisterFieldDefinition
	if !request.Whole {
		if int(op.RegisterField) >= len(definition.Fields) || definition.Fields[op.RegisterField].Reserved {
			return fmt.Errorf("%s names no accessible register field #%d", op.Kind, op.RegisterField)
		}
		field = definition.Fields[op.RegisterField]
	}
	valueType := definition.TypeID
	if !request.Whole {
		valueType = field.Type
	}
	observe := request.Kind == string(sema.RegisterRequestObserve) || request.Kind == string(sema.RegisterRequestDestructiveObserve)
	write := request.Kind == string(sema.RegisterRequestWriteValue) || request.Kind == string(sema.RegisterRequestAction)
	switch op.Kind {
	case OpHardwareObserve:
		if len(op.Operands) != 1 || len(op.Results) != 1 || !observe || op.Results[0].Type != valueType {
			return fmt.Errorf("invalid register.hw.observe")
		}
	case OpHardwareShadowObserve:
		if len(op.Operands) != 2 || len(op.Results) != 0 || !observe || values[op.Operands[1]].Type != valueType || !request.Whole && field.Shadow != string(sema.RegisterShadowed) {
			return fmt.Errorf("invalid register.hw.shadow_observe")
		}
	case OpHardwareRequestWrite:
		whole := request.Whole && request.Kind == "write-register"
		if len(op.Operands) != 2 || len(op.Results) != 0 || !whole && (request.Whole || !write || field.Shadow == string(sema.RegisterShadowed)) || values[op.Operands[1]].Type != valueType {
			return fmt.Errorf("invalid register.hw.request_write")
		}
		if !request.Whole && field.Access == string(sema.RegisterReadOnly) {
			return fmt.Errorf("register.hw.request_write on read-only field #%d", op.RegisterField)
		}
	case OpHardwareShadowIntent:
		if len(op.Operands) != 2 || len(op.Results) != 0 || request.Whole || !write || field.Shadow != string(sema.RegisterShadowed) || values[op.Operands[1]].Type != valueType {
			return fmt.Errorf("invalid register.hw.shadow_intent")
		}
	case OpHardwareCommit:
		if len(op.Operands) != 1 || len(op.Results) != 0 || !request.Whole || request.Kind != "commit" {
			return fmt.Errorf("invalid register.hw.commit")
		}
	case OpHardwareShadowKnown:
		boolType, _ := module.Types.Lookup(op.Results[0].Type)
		if len(op.Operands) != 1 || len(op.Results) != 1 || request.Whole || request.Kind != "shadow-known" || boolType.Kind != TypeBool || field.Shadow != string(sema.RegisterShadowed) {
			return fmt.Errorf("invalid register.hw.shadow_known")
		}
	}
	return nil
}

func isHardwareOperation(kind OpKind) bool {
	switch kind {
	case OpHardwareBind, OpHardwareObserve, OpHardwareRequestWrite, OpHardwareShadowObserve, OpHardwareShadowIntent, OpHardwareCommit, OpHardwareShadowKnown:
		return true
	}
	return false
}
