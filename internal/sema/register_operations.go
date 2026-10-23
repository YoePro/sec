package sema

import (
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
// endpoint facts (MD-111).

type RegisterOperationKind string

const (
	RegisterOperationRead       RegisterOperationKind = "read-register"
	RegisterOperationCommit     RegisterOperationKind = "commit-shadow"
	RegisterOperationWriteWhole RegisterOperationKind = "write-register"
)

// ResolvedRegisterOperation is a compiler-known register operation call.
type ResolvedRegisterOperation struct {
	Kind     RegisterOperationKind
	Register Type
	Root     RegisterRootKind
	Member   string
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
	return name == "Read" || name == "Write"
}

// inferRegisterOperationCall resolves `x.Read()`, `x.Write()`,
// `x.Write(value)` and the implicit-self forms on a register.
func (a *Analyzer) inferRegisterOperationCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	var name string
	var receiver ast.Expression
	var register Type
	switch callee := expr.Callee.(type) {
	case *ast.MemberExpression:
		if callee.Property == nil || !isReservedRegisterOperationName(callee.Property.Value) {
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
		operation.Kind, result = RegisterOperationRead, register
	case len(expr.Arguments) == 0:
		if !registerHasWritableShadow(register) {
			a.addErrorAtTokenWithMetadata(token, "",
				"Declare the written fields `shadow` to collect intents for Write(), or write them directly; use Write(value) for a whole-register write.",
				"Write() commits pending shadow write intents, but register %s has no writable shadow field", register.Name)
			return invalid, display, true
		}
		operation.Kind = RegisterOperationCommit
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
		operation.Kind = RegisterOperationWriteWhole
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

// rejectReservedRegisterMember rejects an impl member of a register type
// named Read or Write (owner decision 2026-10-23).
func (a *Analyzer) rejectReservedRegisterMember(target Type, name string, token lexer.Token) bool {
	if target.Kind != RegisterType || !isReservedRegisterOperationName(name) {
		return false
	}
	a.addErrorAtTokenWithMetadata(token, "",
		"Publish the operation under another name, for example `fn Snapshot() "+target.Name+" { return self.Read() }`.",
		"%s is reserved in impl %s for the compiler-known hardware register operation %s()", name, target.Name, name)
	return true
}
