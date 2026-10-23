package sema

import (
	"math/big"
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Logical hardware register requests (hardware-register-access.md §§ 4, 6,
// 13.1; MD-077 §§ 7, 18(4)). A field step on a hardware instance, or on the
// receiver of a register impl member, is first a logical request derived from
// the field's declared access contract alone: an observation (destructive for
// read-clear / read-set), a value write, or a clear/set/toggle action. The
// physical plan that realizes it needs endpoint facts (MD-111) and is not
// chosen here. Requests that are invalid on hardware are rejected on a
// hardware instance and make an impl member software-only.

type RegisterHardwareRequestKind string

const (
	RegisterRequestObserve            RegisterHardwareRequestKind = "observe"
	RegisterRequestDestructiveObserve RegisterHardwareRequestKind = "destructive-observe"
	RegisterRequestWriteValue         RegisterHardwareRequestKind = "write-value"
	RegisterRequestAction             RegisterHardwareRequestKind = "action"
)

// ResolvedRegisterHardwareRequest is one logical request. Effect is the
// destructive read effect ("clear", "set") or the write action ("clear",
// "set", "toggle"); Shadow is the field's effective shadow policy.
type ResolvedRegisterHardwareRequest struct {
	Kind   RegisterHardwareRequestKind
	Effect string
	Shadow RegisterShadowPolicy
}

func (a *Analyzer) recordRegisterWriteTarget(target ast.Expression, stmt *ast.AssignmentStatement) {
	if a.registerWriteTargets == nil {
		a.registerWriteTargets = map[ast.Expression]*ast.AssignmentStatement{}
	}
	a.registerWriteTargets[target] = stmt
}

// implicitRegisterFieldOf reports an implicit `Field` of the register impl
// being analyzed.
func (a *Analyzer) implicitRegisterFieldOf(target *ast.Identifier, symbol Symbol) (RegisterField, bool) {
	if !symbol.ImplicitMember {
		return RegisterField{}, false
	}
	register, ok := a.types[a.currentImplTarget]
	if !ok || register.Kind != RegisterType {
		return RegisterField{}, false
	}
	return lookupRegisterFieldInfo(register, target.Value)
}

// checkImplicitRegisterFieldWrite validates `Field = value` inside a register
// impl exactly like `self.Field = value` and records the write proof.
func (a *Analyzer) checkImplicitRegisterFieldWrite(stmt *ast.AssignmentStatement, field RegisterField, valueType Type) bool {
	if field.Width == 1 && field.Type.Kind == BoolType && a.isZeroOrOneIntegerConstant(stmt.Value) {
		value, _ := a.integerConstantValue(stmt.Value)
		a.recordRegisterWrite(stmt, ResolvedRegisterWriteProof{Basis: RegisterWriteConstant, Low: value, High: new(big.Int).Set(value)})
		return true
	}
	if field.Type.Kind == UintType {
		return a.checkRegisterFieldWrite(stmt, field, valueType)
	}
	if !a.canInitialize(field.Type, valueType, stmt.Value) {
		a.addErrorAtToken(expressionToken(stmt.Value), "cannot assign %s to %s", typeDisplayName(valueType), typeDisplayName(field.Type))
		return false
	}
	a.recordRegisterWrite(stmt, ResolvedRegisterWriteProof{Basis: RegisterWriteTyped})
	return true
}

// registerAccessStep is one leaf field access awaiting request resolution.
type registerAccessStep struct {
	expr  ast.Expression
	token lexer.Token
	plan  *ResolvedRegisterFieldPlan
	// collateral lists the root register's other destructive fields.
	register Type
}

// resolveRegisterHardwareRequests classifies every hardware or impl-receiver
// leaf field step as a read or write request and validates it.
func (a *Analyzer) resolveRegisterHardwareRequests() {
	projections := map[ast.Expression]bool{}
	for member := range a.resolvedRegisterFields {
		if object, ok := member.Object.(*ast.MemberExpression); ok {
			if _, nested := a.resolvedRegisterFields[object]; nested {
				projections[object] = true
			}
		}
	}
	var steps []registerAccessStep
	for member, plan := range a.resolvedRegisterFields {
		if plan.Root == RegisterRootDetached || projections[member] {
			continue
		}
		stored := plan
		steps = append(steps, registerAccessStep{expr: member, token: member.Property.Token, plan: &stored, register: a.rootRegisterOf(member)})
	}
	for identifier, use := range a.implicitReceiverUses {
		if use.RegisterField == nil {
			continue
		}
		steps = append(steps, registerAccessStep{expr: identifier, token: identifier.Token, plan: use.RegisterField, register: use.RegisterField.Register})
	}
	sort.Slice(steps, func(i, j int) bool { return tokenBefore(steps[i].token, steps[j].token) })
	for _, step := range steps {
		if stmt, write := a.registerWriteTargets[step.expr]; write {
			step.plan.Request = a.registerWriteRequest(step, stmt)
		} else {
			step.plan.Request = a.registerReadRequest(step)
		}
		if member, ok := step.expr.(*ast.MemberExpression); ok {
			a.resolvedRegisterFields[member] = *step.plan
		}
	}
}

// rootRegisterOf is the register type of the outermost field step of a
// nested path: the physical register a hardware request reaches.
func (a *Analyzer) rootRegisterOf(member *ast.MemberExpression) Type {
	register := a.resolvedRegisterFields[member].Register
	for {
		object, ok := member.Object.(*ast.MemberExpression)
		if !ok {
			return register
		}
		plan, nested := a.resolvedRegisterFields[object]
		if !nested {
			return register
		}
		register, member = plan.Register, object
	}
}

// registerHardwareFailure rejects a request on a hardware instance, or makes
// the enclosing impl member software-only.
func (a *Analyzer) registerHardwareFailure(step registerAccessStep, help, reason string) {
	if step.plan.Root == RegisterRootImplReceiver {
		a.markSoftwareOnlyMemberKey(step.plan.Member, step.token, reason)
		return
	}
	a.addErrorAtTokenWithMetadata(step.token, "", help, "hardware register access %s", reason)
}

// registerReadRequest builds an observation and enforces the safe implicit
// read rule (§§ 6.2–6.3): with no target fact providing a selective
// observation, observing a field also observes every other field of the
// physical register, so another read-clear / read-set field makes the
// observation carry a collateral destructive effect.
func (a *Analyzer) registerReadRequest(step registerAccessStep) *ResolvedRegisterHardwareRequest {
	field := step.plan.Field
	request := &ResolvedRegisterHardwareRequest{Kind: RegisterRequestObserve, Shadow: field.Shadow}
	switch field.Access {
	case RegisterReadClear:
		request.Kind, request.Effect = RegisterRequestDestructiveObserve, "clear"
	case RegisterReadSet:
		request.Kind, request.Effect = RegisterRequestDestructiveObserve, "set"
	}
	if collateral, access, found := collateralDestructiveField(step.register, step.plan.Register, field.Name); found {
		a.registerHardwareFailure(step,
			"No target fact provides a selective observation of "+field.Name+"; read the whole register once with an explicit observation in impl "+step.register.Name+" instead.",
			"observes "+field.Name+", which would also perform the destructive "+string(access)+" read of "+collateral)
	}
	return request
}

// collateralDestructiveField finds a read-clear / read-set field of the
// physical register other than the observed one (or one inside it).
func collateralDestructiveField(root, owner Type, observed string) (string, RegisterFieldAccess, bool) {
	var walk func(register Type, prefix string, inside bool) (string, RegisterFieldAccess, bool)
	walk = func(register Type, prefix string, inside bool) (string, RegisterFieldAccess, bool) {
		for _, field := range register.RegisterFields {
			path := prefix + field.Name
			requested := inside || (register.Name == owner.Name && register.Module == owner.Module && field.Name == observed)
			if !requested && (field.Access == RegisterReadClear || field.Access == RegisterReadSet) {
				return path, field.Access, true
			}
			if field.Type.Kind == RegisterType {
				if found, access, ok := walk(field.Type, path+".", requested); ok {
					return found, access, true
				}
			}
		}
		return "", "", false
	}
	return walk(root, "", false)
}

// registerWriteRequest builds a value write or a clear/set/toggle action.
// Only the trigger constant of a write-one/write-zero field is defined on
// hardware; other values wait for MD-112.
func (a *Analyzer) registerWriteRequest(step registerAccessStep, stmt *ast.AssignmentStatement) *ResolvedRegisterHardwareRequest {
	field := step.plan.Field
	request := &ResolvedRegisterHardwareRequest{Kind: RegisterRequestWriteValue, Shadow: field.Shadow}
	trigger, effect, special := registerWriteTrigger(field.Access)
	if !special {
		return request
	}
	request.Kind, request.Effect = RegisterRequestAction, effect
	value, constant := a.registerBitConstant(stmt.Value)
	if field.Width == 1 && constant && value == trigger {
		return request
	}
	triggerSpelling := "false"
	if trigger {
		triggerSpelling = "true"
	}
	a.registerHardwareFailure(step,
		"Write the trigger value "+triggerSpelling+" to request the "+effect+" action, guarded by an if when it is conditional; other values are undecided (MD-112).",
		"writes a value other than the trigger constant to "+string(field.Access)+" register field "+field.Name)
	return request
}

// registerWriteTrigger returns the trigger value and action of a
// write-one/write-zero access.
func registerWriteTrigger(access RegisterFieldAccess) (bool, string, bool) {
	switch access {
	case RegisterWriteOneClear:
		return true, "clear", true
	case RegisterWriteOneSet:
		return true, "set", true
	case RegisterWriteOneToggle:
		return true, "toggle", true
	case RegisterWriteZeroClear:
		return false, "clear", true
	case RegisterWriteZeroSet:
		return false, "set", true
	case RegisterWriteZeroToggle:
		return false, "toggle", true
	}
	return false, "", false
}

// registerBitConstant reads a one-bit constant: a boolean constant or the
// integer 0 or 1.
func (a *Analyzer) registerBitConstant(expr ast.Expression) (bool, bool) {
	if value, known := a.constantBooleanValue(expr); known {
		return value, true
	}
	if a.isZeroOrOneIntegerConstant(expr) {
		value, _ := a.integerConstantValue(expr)
		return value.Sign() != 0, true
	}
	return false, false
}

// RegisterFieldAccessAt returns the field plan, with its logical hardware
// request when it has one, of the register field step at a source position
// (`x.Field` property token or implicit `Field`), for tooling.
func (a *Analyzer) RegisterFieldAccessAt(file string, line, column int) (ResolvedRegisterFieldPlan, bool) {
	if a == nil {
		return ResolvedRegisterFieldPlan{}, false
	}
	at := func(token lexer.Token) bool {
		return token.File == file && token.Line == line && token.Column == column
	}
	for member, plan := range a.resolvedRegisterFields {
		if member.Property != nil && at(member.Property.Token) {
			return plan, true
		}
	}
	for identifier, use := range a.implicitReceiverUses {
		if use.RegisterField != nil && at(identifier.Token) {
			return *use.RegisterField, true
		}
	}
	return ResolvedRegisterFieldPlan{}, false
}

// RegisterOperationAt returns the compiler-known register operation whose
// name is at a source position, for tooling.
func (a *Analyzer) RegisterOperationAt(file string, line, column int) (ResolvedRegisterOperation, bool) {
	if a == nil {
		return ResolvedRegisterOperation{}, false
	}
	for call, operation := range a.resolvedRegisterOperations {
		token := expressionToken(call.Callee)
		if member, ok := call.Callee.(*ast.MemberExpression); ok && member.Property != nil {
			token = member.Property.Token
		}
		if token.File == file && token.Line == line && token.Column == column {
			return operation, true
		}
	}
	return ResolvedRegisterOperation{}, false
}

// RegisterMemberRequirement reports whether an instance member of a register
// impl ("T.Member", or "T.Property.get" / ".set") runs only on a hardware
// instance or only on a software value, and why.
func (a *Analyzer) RegisterMemberRequirement(key string) (hardwareOnly bool, reason string, ok bool) {
	if a == nil {
		return false, "", false
	}
	if why, found := a.hardwareOnlyRegisterMembers[key]; found {
		return true, why.reason, true
	}
	if why, found := a.softwareOnlyRegisterMembers[key]; found {
		return false, why.reason, true
	}
	return false, "", false
}

// ResolvedRegisterHardwareRequestOf returns the logical hardware request of a
// field step, if it has one.
func (a *Analyzer) ResolvedRegisterHardwareRequestOf(expr ast.Expression) (ResolvedRegisterHardwareRequest, bool) {
	if a == nil {
		return ResolvedRegisterHardwareRequest{}, false
	}
	switch expr := expr.(type) {
	case *ast.MemberExpression:
		if plan, ok := a.resolvedRegisterFields[expr]; ok && plan.Request != nil {
			return *plan.Request, true
		}
	case *ast.Identifier:
		if use, ok := a.implicitReceiverUses[expr]; ok && use.RegisterField != nil && use.RegisterField.Request != nil {
			return *use.RegisterField.Request, true
		}
	}
	return ResolvedRegisterHardwareRequest{}, false
}
