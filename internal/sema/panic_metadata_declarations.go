package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// registerPanicMetadataDeclaration attaches exact trusted core source declarations
// to the reserved panic identities, keeping standalone fallback facts canonical.
// Neither a core-looking path nor a module spelling grants declaration authority.
// Rules: rules/errors/panic.md §13(2–7); rules/library/core-library.md —
// compiler-known core declarations; rules/concurrency/tasks.md §11.
func (a *Analyzer) registerPanicMetadataDeclaration(statement ast.Statement) bool {
	var name *ast.Identifier
	declaration, isType := statement.(*ast.TypeDeclStatement)
	switch stmt := statement.(type) {
	case *ast.TypeDeclStatement:
		name = stmt.Name
	case *ast.EnumDeclaration:
		name = stmt.Name
	case *ast.InterfaceDeclaration:
		name = stmt.Name
	}
	if name == nil || (name.Value != "PanicID" && name.Value != "PanicInfo") || !a.isTrustedCoreBuiltinDeclaration(name.Value, name.Token) {
		return false
	}
	typ := a.types[name.Value]
	valid := isType && a.currentModule == "core" && declaration.AssignedType == nil && declaration.RegisterType == nil && !declaration.Union && !declaration.ErrorType && declaration.Contract == nil && declaration.Default == nil && len(declaration.Implements) == 0 && len(declaration.GenericParameters) == 0 && len(declaration.Attributes) == 0
	if valid && name.Value == "PanicID" {
		valid = declaration.StructType == nil && declaration.BaseType != nil && typeReferenceDisplayName(declaration.BaseType) == "uint32"
	} else if valid {
		valid = declaration.BaseType == nil && declaration.StructType != nil && len(declaration.StructType.Fields) == len(typ.Fields)
		if valid {
			for i, field := range declaration.StructType.Fields {
				expected := typ.Fields[i]
				if field == nil || field.Name == nil || field.Name.Value != expected.Name || field.Type == nil || typeReferenceDisplayName(field.Type) != expected.Type.Name || field.Contract != nil || len(field.Tags) != 0 {
					valid = false
					break
				}
			}
		}
	}
	if !valid {
		a.addErrorAtTokenWithID(name.Token, diagnostics.ReservedDeclarationName, "%s is compiler-known and requires its exact canonical core declaration from rules/errors/panic.md section 13", name.Value)
		a.invalidTypeDeclarations[sourceTokenLocation(name.Token)] = true
		return true
	}
	typ.Declared = true
	typ.DeclarationToken = name.Token
	if name.Value == "PanicInfo" {
		typ.Fields = append([]StructField(nil), typ.Fields...)
		for i, field := range declaration.StructType.Fields {
			typ.Fields[i].Type = a.types[typ.Fields[i].Type.Name]
			typ.Fields[i].Token = field.Name.Token
			a.recordDefinition(field.Name.Token)
		}
	}
	a.types[name.Value] = typ
	a.registerTypeDefinition(name.Value, name.Token)
	return true
}
