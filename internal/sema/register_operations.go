package sema

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Compiler-known hardware register Read() / Write() (hardware-register-access
// .md §§ 7–8; MD-077 § 14; owner decision 2026-10-23: `Read` and `Write` are
// reserved member names in a register impl). They are reachable only inside
// the owning `impl T`, on a hardware receiver: `self` / implicit (which makes
// the member hardware-only) or a hardware binding. Read() is one logical
// observation returning a detached T; Write() commits pending shadow intents;
// Write(value) requests a whole-register write. Their physical plans need
// endpoint facts (MD-111). IsShadowKnown(T.Field...) (MD-113) asks whether
// software holds a complete shadow value for a statically selected field and
// never touches hardware.

type RegisterOperationKind string

const (
	RegisterOperationRead        RegisterOperationKind = "read-register"
	RegisterOperationCommit      RegisterOperationKind = "commit-shadow"
	RegisterOperationWriteWhole  RegisterOperationKind = "write-register"
	RegisterOperationShadowKnown RegisterOperationKind = "shadow-known"
)

// ResolvedRegisterOperation is a compiler-known register operation call. ID
// is its registry identity; FieldPath is the resolved field selector of an
// IsShadowKnown query, outermost field first.
type ResolvedRegisterOperation struct {
	Kind      RegisterOperationKind
	ID        string
	Register  Type
	Root      RegisterRootKind
	Member    string
	FieldPath []RegisterField
}

// CompilerKnownRegisterOperations is the registry of the reserved hardware
// register operations of register type T (compiler_known_members.md
// "Register hardware operations"; MD-113 § 9). They are reachable only inside
// the owning impl T on a hardware receiver, so they are not ordinary members
// of every register value.
func CompilerKnownRegisterOperations(register Type) []CompilerKnownMember {
	return []CompilerKnownMember{
		{ID: "CKM-REGISTER-READ", Name: "Read", Kind: CompilerKnownMethod, Result: register,
			Signature:     "fn Read() " + register.Name,
			Documentation: "One logical hardware observation returning a detached snapshot; establishes observed shadow knowledge per its postconditions."},
		{ID: "CKM-REGISTER-WRITE", Name: "Write", Kind: CompilerKnownMethod, Result: builtinType("void"),
			Signature:     "fn Write() void",
			Documentation: "Commits the pending shadow write intents; performs no physical write when none is pending."},
		{ID: "CKM-REGISTER-WRITE-VALUE", Name: "Write", Kind: CompilerKnownMethod, Result: builtinType("void"),
			Signature:     "fn Write(value: " + register.Name + ") void",
			Documentation: "Requests one whole-register hardware write of a detached value."},
		{ID: "CKM-REGISTER-IS-SHADOW-KNOWN", Name: "IsShadowKnown", Kind: CompilerKnownMethod, Result: builtinType("bool"),
			Signature:     "fn IsShadowKnown(" + register.Name + ".Field) bool",
			Documentation: "Reports whether software holds a complete shadow value for the selected shadow field. Performs no hardware access; a Known value is not current hardware state."},
	}
}

// ResolvedRegisterOperationOf returns the compiler-known register operation a
// call resolved to.
func (a *Analyzer) ResolvedRegisterOperationOf(call *ast.CallExpression) (ResolvedRegisterOperation, bool) {
	if a == nil || call == nil {
		return ResolvedRegisterOperation{}, false
	}
	operation, ok := a.resolvedRegisterOperations[call]
	return operation, ok
}

func isReservedRegisterOperationName(name string) bool {
	for _, member := range CompilerKnownRegisterOperations(Type{}) {
		if member.Name == name {
			return true
		}
	}
	return false
}

// inferRegisterOperationCall resolves `x.Read()`, `x.Write()`,
// `x.Write(value)` and the implicit-self forms on a register. A qualified
// ordinary function is resolved before probing any runtime register receiver.
// Rules: rules/compiler/compiler_known_members.md — "Lookup order";
// rules/declarations/registers.md — §11.7, reserved register operations.
func (a *Analyzer) inferRegisterOperationCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	var name string
	var receiver ast.Expression
	var register Type
	switch callee := expr.Callee.(type) {
	case *ast.MemberExpression:
		if callee.Property == nil || !isReservedRegisterOperationName(callee.Property.Value) {
			return Type{}, expressionValue{}, false
		}
		// Module aliases are not runtime values. Probing an already registered
		// qualified free function as a receiver would emit a spurious undefined
		// variable diagnostic for calls such as linux.Read(fd, buffer).
		if len(a.functions[callExpressionName(expr)]) != 0 {
			return Type{}, expressionValue{}, false
		}
		objectType, _ := a.inferPlaceBase(callee.Object)
		if objectType.Kind == InvalidType {
			return Type{}, expressionValue{}, false
		}
		register = dereferenceType(objectType)
		if register.Kind != RegisterType {
			return Type{}, expressionValue{}, false
		}
		name, receiver = callee.Property.Value, callee.Object
	case *ast.Identifier:
		if !isReservedRegisterOperationName(callee.Value) {
			return Type{}, expressionValue{}, false
		}
		target, ok := a.types[a.currentImplTarget]
		if !ok || target.Kind != RegisterType {
			return Type{}, expressionValue{}, false
		}
		if symbol, local := a.symbols[callee.Value]; local && !symbol.ImplicitMember {
			return Type{}, expressionValue{}, false
		}
		name, register = callee.Value, target
	default:
		return Type{}, expressionValue{}, false
	}
	invalid := Type{Kind: InvalidType}
	display := expressionValue{Display: expr.String()}
	token := expressionToken(expr.Callee)
	root := RegisterRootImplReceiver
	if receiver != nil {
		root = a.registerRootKind(receiver)
	}
	if a.currentImplTarget != register.Name {
		if root == RegisterRootHardware {
			a.addErrorAtTokenWithMetadata(token, "",
				"Expose the operation as a property or method of impl "+register.Name+".",
				"%s() of hardware instance %s is not public; it is reached only inside impl %s", name, receiver.String(), register.Name)
		} else {
			a.addErrorAtToken(token, "%s() is a hardware register operation; %s is a software %s value, whose fields are read and written directly", name, receiverDisplay(receiver), register.Name)
		}
		return invalid, display, true
	}
	switch root {
	case RegisterRootDetached:
		a.addErrorAtToken(token, "%s() is a hardware register operation; %s is a software %s value, whose fields are read and written directly", name, receiverDisplay(receiver), register.Name)
		return invalid, display, true
	case RegisterRootImplReceiver:
		a.markHardwareOnlyMember(token, "calls "+name+"()")
	}
	operation := ResolvedRegisterOperation{Register: register, Root: root, Member: a.currentRegisterMemberKey()}
	result := a.types["void"]
	switch {
	case name == "Read":
		if !a.checkCompilerKnownCallArity(expr, register.Name+".Read", 0, 0) {
			return invalid, display, true
		}
		operation.Kind, operation.ID, result = RegisterOperationRead, "CKM-REGISTER-READ", register
	case name == "IsShadowKnown":
		if !a.checkCompilerKnownCallArity(expr, register.Name+".IsShadowKnown", 1, 1) {
			return invalid, display, true
		}
		path, ok := a.resolveRegisterFieldSelector(expr.Arguments[0], register)
		if !ok {
			return invalid, display, true
		}
		operation.Kind, operation.ID, operation.FieldPath, result = RegisterOperationShadowKnown, "CKM-REGISTER-IS-SHADOW-KNOWN", path, builtinType("bool")
	case len(expr.Arguments) == 0:
		if !registerHasWritableShadow(register) {
			a.addErrorAtTokenWithMetadata(token, "",
				"Declare the written fields `shadow` to collect intents for Write(), or write them directly; use Write(value) for a whole-register write.",
				"Write() commits pending shadow write intents, but register %s has no writable shadow field", register.Name)
			return invalid, display, true
		}
		operation.Kind, operation.ID = RegisterOperationCommit, "CKM-REGISTER-WRITE"
	default:
		if !a.checkCompilerKnownCallArity(expr, register.Name+".Write", 0, 1) {
			return invalid, display, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], register)
		if valueType.Kind == InvalidType {
			return invalid, display, true
		}
		if !a.canInitialize(register, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "%s.Write value must be %s, got %s", register.Name, typeDisplayName(register), typeDisplayName(valueType))
			return invalid, display, true
		}
		operation.Kind, operation.ID = RegisterOperationWriteWhole, "CKM-REGISTER-WRITE-VALUE"
	}
	if a.resolvedRegisterOperations == nil {
		a.resolvedRegisterOperations = map[*ast.CallExpression]ResolvedRegisterOperation{}
	}
	a.resolvedRegisterOperations[expr] = operation
	return result, display, true
}

func receiverDisplay(receiver ast.Expression) string {
	if receiver == nil {
		return "self"
	}
	return receiver.String()
}

// registerHasWritableShadow reports a shadowed field that can hold a write
// intent (hardware-register-access.md §§ 8.1, 9.12).
func registerHasWritableShadow(register Type) bool {
	for _, field := range register.RegisterFields {
		if field.Name == "_" {
			continue
		}
		if field.Shadow == RegisterShadowed && field.Access != RegisterReadOnly {
			return true
		}
		if field.Type.Kind == RegisterType && registerHasWritableShadow(field.Type) {
			return true
		}
	}
	return false
}

// resolveRegisterFieldSelector resolves the static operand of IsShadowKnown:
// `T.Field(.Field)*` rooted in the receiver's register type T, resolved as a
// type and never evaluated. Every step but the last selects a nested
// register, and the last a named field with an effective shadow policy.
//
// Rules:
//   - rules/declarations/registers.md — § 11.7 (Amendment MD-113)
//   - rules/corrections/applied/MD-113-shadow-knowledge-across-calls-decision-20261030.md — §§ 4, 6, 11
func (a *Analyzer) resolveRegisterFieldSelector(argument ast.Expression, register Type) ([]RegisterField, bool) {
	token := expressionToken(argument)
	var steps []*ast.Identifier
	for current := argument; ; {
		switch node := current.(type) {
		case *ast.MemberExpression:
			if node.Property == nil {
				a.addErrorAtToken(token, "IsShadowKnown needs a field selector such as %s.Field", register.Name)
				return nil, false
			}
			steps = append([]*ast.Identifier{node.Property}, steps...)
			current = node.Object
			continue
		case *ast.Identifier:
			steps = append([]*ast.Identifier{node}, steps...)
		default:
			a.addErrorAtToken(token, "IsShadowKnown needs a static field selector such as %s.Field, not a runtime expression", register.Name)
			return nil, false
		}
		break
	}
	if len(steps) == 1 {
		a.addErrorAtTokenWithMetadata(token, "", "Write the selector with its register type, for example "+register.Name+"."+steps[0].Value+".",
			"IsShadowKnown needs the register-type-qualified field selector %s.%s", register.Name, steps[0].Value)
		return nil, false
	}
	rootLength := 0
	var root Type
	for length := len(steps) - 1; length >= 1 && steps[0].Value != "self"; length-- {
		names := make([]string, length)
		for index := range names {
			names[index] = steps[index].Value
		}
		if typ, ok := a.types[a.resolveTypeName(strings.Join(names, "."))]; ok {
			root, rootLength = typ, length
			break
		}
	}
	if rootLength == 0 {
		a.addErrorAtTokenWithMetadata(token, "", "Name the field through its register type, for example "+register.Name+"."+steps[len(steps)-1].Value+"; the argument identifies a field and is never evaluated.",
			"IsShadowKnown selects a field by its register type; %s is not a register type and the argument is not evaluated", steps[0].Value)
		return nil, false
	}
	if root.Kind != RegisterType || root.Name != register.Name || root.Module != register.Module {
		a.addErrorAtToken(steps[0].Token, "IsShadowKnown selector is rooted in %s, but the receiver is a %s hardware instance", typeDisplayName(root), register.Name)
		return nil, false
	}
	var path []RegisterField
	current := register
	for index, step := range steps[rootLength:] {
		if step.Value == "_" {
			a.addErrorAtToken(step.Token, "reserved register field _ cannot be selected")
			return nil, false
		}
		field, ok := lookupRegisterFieldInfo(current, step.Value)
		if !ok {
			a.addErrorAtToken(step.Token, "register %s has no field %s", current.Name, step.Value)
			return nil, false
		}
		a.bindDefinition(step.Token, field.Token)
		path = append(path, field)
		if index == len(steps[rootLength:])-1 {
			break
		}
		if field.Type.Kind != RegisterType {
			a.addErrorAtToken(step.Token, "field %s of register %s is not a nested register", field.Name, current.Name)
			return nil, false
		}
		current = field.Type
	}
	leaf := path[len(path)-1]
	if leaf.Shadow != RegisterShadowed {
		a.addErrorAtTokenWithMetadata(steps[len(steps)-1].Token, "",
			"Declare the field `shadow` to keep software knowledge of it, or read it with an explicit observation.",
			"IsShadowKnown applies only to shadow fields; %s is noshadow", leaf.Name)
		return nil, false
	}
	return path, true
}

// rejectReservedRegisterMember rejects an impl member of a register type
// named Read, Write or IsShadowKnown (owner decision 2026-10-23; MD-113 § 9).
func (a *Analyzer) rejectReservedRegisterMember(target Type, name string, token lexer.Token) bool {
	if target.Kind != RegisterType || !isReservedRegisterOperationName(name) {
		return false
	}
	a.addErrorAtTokenWithMetadata(token, "",
		"Publish the operation under another name, for example `fn Snapshot() "+target.Name+" { return self.Read() }`.",
		"%s is reserved in impl %s for the compiler-known hardware register operation %s()", name, target.Name, name)
	return true
}
