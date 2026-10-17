package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// ipcErrorVariants defines the exact closed portable error sets shared by
// standalone Sema and trusted core declarations, independently of target IPC
// support. Unsupported target operations never become an extra error variant.
// Rules: rules/concurrency/ipc.md — §§6,27,44,55,68,79,124.
func ipcErrorVariants(name string) []string {
	switch name {
	case "IPCError":
		return []string{"Closed", "PeerClosed", "TransferFailed", "MaterializationFailed", "OutOfMemory", "ResourceLimit", "ProtocolFailure", "NativeFailure"}
	case "SharedMemoryError":
		return []string{"Closed", "InvalidSize", "PermissionDenied", "OutOfMemory", "ResourceLimit", "MappingFailed", "DuplicationFailed", "NativeFailure"}
	case "IPCSyncError":
		return []string{"Closed", "InvalidCount", "LimitExceeded", "OwnerDied", "PermissionDenied", "OutOfMemory", "ResourceLimit", "NativeFailure"}
	case "IPCSharedError":
		return []string{"Closed", "LayoutMismatch", "OwnerDied", "PermissionDenied", "OutOfMemory", "ResourceLimit", "MappingFailed", "DuplicationFailed", "NativeFailure"}
	case "IPCAtomicError":
		return []string{"PermissionDenied", "OutOfMemory", "ResourceLimit", "DuplicationFailed", "NativeFailure"}
	}
	return nil
}

// builtinIPCErrorType reserves one nominal core error identity and its ordinary
// enum default (first member). It supplies no operation or runtime transport.
// Rules: rules/concurrency/ipc.md — §§6,27,44,55,68,79;
// rules/types/default_values.md — "Enums".
func builtinIPCErrorType(name string) Type {
	values := ipcErrorVariants(name)
	return Type{Name: name, Module: "core", Kind: EnumType, Named: true, Declared: true,
		Intrinsic: true, ErrorAssignable: true, Underlying: "int", EnumValues: values,
		EnumConsts: builtinEnumConsts(values), EnumDefault: values[0]}
}

// commandIOModeVariants defines the exact input/output mode sets, preserving
// Resource as a distinct owned-capability mode rather than Pipe or a handle.
// Rules: rules/concurrency/processes.md — §§5.12–5.13;
// rules/concurrency/ipc.md — §94.
func commandIOModeVariants(name string) []string {
	switch name {
	case "CommandInputMode":
		return []string{"Inherit", "Closed", "Pipe", "Resource"}
	case "CommandOutputMode":
		return []string{"Inherit", "Discard", "Pipe", "Resource"}
	}
	return nil
}

// builtinCommandIOModeType reserves a normal, copyable non-error enum whose
// implicit default is Inherit. It does not establish any I/O resource binding.
// Rules: rules/concurrency/processes.md — §§5.12–5.13;
// rules/concurrency/ipc.md — §94; rules/types/default_values.md — Enums.
func builtinCommandIOModeType(name string) Type {
	values := commandIOModeVariants(name)
	return Type{Name: name, Module: "core", Kind: EnumType, Named: true, Declared: true,
		Intrinsic: true, Underlying: "int", EnumValues: values,
		EnumConsts: builtinEnumConsts(values), EnumDefault: "Inherit"}
}

// ipcCoreEnumType selects the canonical compiler-owned enum shape; project
// types cannot acquire these identities by spelling or structural similarity.
// Rules: rules/concurrency/ipc.md — §§6,27,44,55,68,79,94;
// rules/concurrency/processes.md — §§5.12–5.13.
func ipcCoreEnumType(name string) (Type, bool) {
	if len(ipcErrorVariants(name)) != 0 {
		return builtinIPCErrorType(name), true
	}
	if len(commandIOModeVariants(name)) != 0 {
		return builtinCommandIOModeType(name), true
	}
	return Type{}, false
}

// ipcCoreEnumFromDeclaration accepts only the exact canonical core enum,
// retaining real declaration/member locations for hover and navigation. On
// malformed trusted source the reserved fallback remains exact for recovery.
// Rules: rules/concurrency/ipc.md — §§6,27,44,55,68,79,94;
// rules/concurrency/processes.md — §§5.12–5.13;
// rules/library/core-library.md — compiler-known core declarations.
func (a *Analyzer) ipcCoreEnumFromDeclaration(enum *ast.EnumDeclaration) (Type, bool) {
	name := enum.Name.Value
	typ, known := ipcCoreEnumType(name)
	if !known || !a.isTrustedCoreBuiltinDeclaration(name, enum.Name.Token) {
		return Type{}, false
	}
	values := typ.EnumValues
	valid := enum.ErrorType == typ.ErrorAssignable && !enum.BitUnderlying && enum.UnderlyingType == nil && len(enum.Attributes) == 0 && len(enum.GenericParameters) == 0 && len(enum.Values) == len(values)
	if valid {
		for i, value := range enum.Values {
			if value == nil || value.Invalid || value.Name == nil || value.Name.Value != values[i] || value.Default || value.Initializer != nil || value.Payload != nil {
				valid = false
				break
			}
		}
	}
	if !valid {
		kind := "enum"
		if typ.ErrorAssignable {
			kind = "error enum"
		}
		a.addErrorAtToken(enum.Name.Token, "%s requires its exact canonical core %s declaration from rules/concurrency/ipc.md", name, kind)
		return typ, true
	}
	typ.DeclarationToken = enum.Name.Token
	for _, value := range enum.Values {
		member := typ.EnumConsts[value.Name.Value]
		member.Token = value.Name.Token
		typ.EnumConsts[value.Name.Value] = member
		a.recordDefinition(value.Name.Token)
	}
	return typ, true
}

// rejectIPCCoreEnumNonEnum prevents trusted core from replacing a portable enum
// identity with a scalar, struct, union or interface of the same spelling.
// Rules: rules/concurrency/ipc.md — §§6,27,44,55,68,79,94;
// rules/concurrency/processes.md — §§5.12–5.13.
func (a *Analyzer) rejectIPCCoreEnumNonEnum(name string, token lexer.Token) bool {
	if _, known := ipcCoreEnumType(name); !known || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "%s requires its canonical core enum declaration", name)
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}
