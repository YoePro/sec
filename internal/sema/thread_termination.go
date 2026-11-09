package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// builtinThreadTerminationKindType defines portable classifications without
// inventing native status codes or platform-specific termination authority.
// Rules: rules/concurrency/threads.md — §23(1–4).
func builtinThreadTerminationKindType() Type {
	values := []string{"Requested", "External", "Fault", "Unknown"}
	return Type{Name: "ThreadTerminationKind", Module: "core", Kind: EnumType,
		Named: true, Declared: true, Intrinsic: true, Underlying: "int",
		EnumValues: values, EnumConsts: builtinEnumConsts(values), EnumDefault: values[0]}
}

// builtinThreadTerminationType exposes exactly the portable Kind payload.
// Terminal availability and native termination metadata remain separate facts.
// Rules: rules/concurrency/threads.md — §23(2–4).
func builtinThreadTerminationType() Type {
	return Type{Name: "ThreadTermination", Module: "core", Kind: StructType,
		Named: true, Declared: true, Intrinsic: true,
		Fields: []StructField{{Name: "Kind", Type: builtinThreadTerminationKindType()}}}
}

// registerThreadTerminationDeclaration accepts only the canonical trusted
// payload declaration; scalar/union replacements and extra native fields fail.
// Rules: rules/concurrency/threads.md — §23(1–4).
func (a *Analyzer) registerThreadTerminationDeclaration(stmt *ast.TypeDeclStatement) bool {
	name := stmt.Name.Value
	if (name != "ThreadTermination" && name != "ThreadTerminationKind") || !a.isTrustedCoreBuiltinDeclaration(name, stmt.Name.Token) {
		return false
	}
	if name == "ThreadTerminationKind" {
		a.addErrorAtToken(stmt.Name.Token, "ThreadTerminationKind requires its canonical core enum declaration")
		a.types[name] = builtinThreadTerminationKindType()
		return true
	}
	typ := builtinThreadTerminationType()
	valid := stmt.StructType != nil && stmt.BaseType == nil && stmt.AssignedType == nil && !stmt.Union && !stmt.ErrorType && stmt.RegisterType == nil && len(stmt.Attributes) == 0 && len(stmt.GenericParameters) == 0 && len(stmt.Implements) == 0 && stmt.Contract == nil && stmt.Default == nil
	if valid {
		fields := stmt.StructType.Fields
		valid = len(fields) == 1 && fields[0] != nil && fields[0].Name != nil && fields[0].Name.Value == "Kind" && fields[0].Type != nil
		if valid {
			field := fields[0]
			valid = !field.Type.Invalid && !field.Type.Ref && !field.Type.MutableRef && typeReferenceDisplayName(field.Type) == "ThreadTerminationKind" && field.Contract == nil && len(field.Tags) == 0
			if valid {
				typ.Fields[0].Token = field.Name.Token
				a.recordDefinition(field.Name.Token)
			}
		}
	}
	if !valid {
		a.addErrorAtToken(stmt.Name.Token, "ThreadTermination requires its exact canonical core struct with only Kind: ThreadTerminationKind")
	} else {
		typ.DeclarationToken = stmt.Name.Token
	}
	a.types[name] = typ
	return true
}

// rejectThreadTerminationNonStruct prevents enum/interface declarations from
// replacing the canonical portable metadata payload.
// Rules: rules/concurrency/threads.md — §23(2–4).
func (a *Analyzer) rejectThreadTerminationNonStruct(name string, token lexer.Token) bool {
	if name != "ThreadTermination" || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "ThreadTermination requires its canonical core struct declaration")
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}

// rejectThreadTerminationKindNonEnum prevents an interface declaration from
// replacing the exact termination classification enum in trusted core.
// Rules: rules/concurrency/threads.md — §23(1).
func (a *Analyzer) rejectThreadTerminationKindNonEnum(name string, token lexer.Token) bool {
	if name != "ThreadTerminationKind" || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "ThreadTerminationKind requires its canonical core enum declaration")
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}
