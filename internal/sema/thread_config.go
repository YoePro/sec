package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// builtinThreadConfigType builds the exact public configuration with selected
// scalar types. Generic payloads use ordinary substitution, with no implicit
// preference conversion or default policy added to ThreadSetting itself.
// Rules: rules/concurrency/threads.md — §§4,10(1–2),11(2–5).
func builtinThreadConfigType(types map[string]Type) Type {
	setting := func(payload Type) Type {
		typ := types["ThreadSetting"]
		typ.TypeArgs = []Type{payload}
		return substituteGenericType(typ, map[string]Type{"T": payload})
	}
	storage := types["ThreadStorage"]
	reference := Type{Kind: ReferenceType, Element: &storage, ReferenceMutable: true}
	option := types["Option"]
	option.TypeArgs = []Type{reference}
	option = substituteGenericType(option, map[string]Type{"T": reference})
	return Type{Name: "ThreadConfig", Module: "core", Kind: StructType,
		Named: true, Declared: true, Intrinsic: true, ExplicitlyNonCopyable: true,
		NoCopyPolicyOrigin: "ThreadConfig", Fields: []StructField{
			{Name: "Name", Type: setting(types["string"])},
			{Name: "Stack", Type: setting(types["uint"])},
			{Name: "Affinity", Type: setting(types["CpuSet"])},
			{Name: "Priority", Type: setting(types["ThreadPriority"])},
			{Name: "Start", Type: types["ThreadStartMode"]},
			{Name: "Storage", Type: option},
		}}
}

// threadConfigFieldDefault represents the explicit initializer, independently
// of each field type's ordinary defaultability. In particular Priority is
// Default, never a preferred native or portable scheduler priority.
// Rules: rules/concurrency/threads.md — §§10(1),11(3).
func threadConfigFieldDefault(owner Type, field StructField) DefaultResolution {
	if owner.Name == "ThreadConfig" && owner.Module == "core" && owner.Intrinsic {
		switch field.Name {
		case "Name", "Stack", "Affinity", "Priority":
			return DefaultResolution{Kind: UnionDefault, Variant: "Default"}
		case "Start":
			return DefaultResolution{Kind: EnumDefault, Value: DefaultConstant{Kind: EnumType, Lexeme: "Eager"}}
		case "Storage":
			return DefaultResolution{Kind: UnionDefault, Variant: "None"}
		}
	}
	return DefaultValueOf(field.Type)
}

// registerThreadConfigDeclaration validates the complete trusted public shape
// and retains real field definitions for navigation. The compiler supplies the
// exact allocation-free initializer rather than exposing runtime internals.
// Rules: rules/concurrency/threads.md — §10(1–2).
func (a *Analyzer) registerThreadConfigDeclaration(stmt *ast.TypeDeclStatement) bool {
	if stmt.Name.Value != "ThreadConfig" || !a.isTrustedCoreBuiltinDeclaration("ThreadConfig", stmt.Name.Token) {
		return false
	}
	typ := builtinThreadConfigType(a.types)
	valid := stmt.StructType != nil && stmt.BaseType == nil && stmt.AssignedType == nil && stmt.RegisterType == nil && !stmt.Union && !stmt.ErrorType && stmt.Contract == nil && stmt.Default == nil && len(stmt.Implements) == 0 && len(stmt.GenericParameters) == 0 && len(stmt.Attributes) == 1 && hasAttribute(stmt.Attributes, "noCopy")
	if valid {
		fields := stmt.StructType.Fields
		valid = len(fields) == len(typ.Fields)
		if valid {
			for index, field := range fields {
				if field == nil || field.Name == nil || field.Name.Value != typ.Fields[index].Name || !threadConfigFieldTypeMatches(field.Type, typ.Fields[index].Type) || field.Contract != nil || len(field.Tags) != 0 {
					valid = false
					break
				}
			}
		}
		if valid {
			for index, field := range fields {
				typ.Fields[index].Token = field.Name.Token
				a.recordDefinition(field.Name.Token)
			}
		}
	}
	if !valid {
		a.addErrorAtToken(stmt.Name.Token, "ThreadConfig requires its exact canonical non-generic @noCopy core struct with Name, Stack, Affinity, Priority, Start and Storage")
	}
	typ.DeclarationToken = stmt.Name.Token
	a.types["ThreadConfig"] = typ
	return true
}

// rejectThreadConfigNonStruct prevents alternate trusted declaration kinds.
// Rules: rules/concurrency/threads.md — §10(1).
func (a *Analyzer) rejectThreadConfigNonStruct(name string, token lexer.Token) bool {
	if name != "ThreadConfig" || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "ThreadConfig requires its exact canonical @noCopy core struct")
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}

// inferThreadConfigConstructor exposes only the exact zero-argument initializer.
// Its defaults share the same semantic resolutions as contextual literals.
// Rules: rules/concurrency/threads.md — §§10(1),11(2–3).
func (a *Analyzer) inferThreadConfigConstructor(expr *ast.CallExpression) (Type, expressionValue, bool) {
	display := expressionValue{Display: expr.String()}
	if !a.checkCompilerKnownCallArity(expr, "ThreadConfig", 0, 0) {
		return Type{Kind: InvalidType}, display, true
	}
	return a.types["ThreadConfig"], display, true
}

// threadConfigFieldTypeMatches compares the complete canonical nested type
// syntax, including the mutable storage reference; display names omit refs.
// Rules: rules/concurrency/threads.md — §10(1).
func threadConfigFieldTypeMatches(ref *ast.TypeReference, expected Type) bool {
	if ref == nil || ref.Invalid || ref.ElementType != nil || ref.Unit != "" || ref.UnitOnly || ref.EventCapacitySet || len(ref.ConstArgs) != 0 || ref.FunctionReturnType != nil || len(ref.FunctionParameterTypes) != 0 {
		return false
	}
	if expected.Kind == ReferenceType {
		return ref.Ref && ref.MutableRef == expected.ReferenceMutable && expected.Element != nil && ref.Name == expected.Element.Name && len(ref.TypeArgs) == 0
	}
	if ref.Ref || ref.MutableRef || ref.Name != expected.Name || len(ref.TypeArgs) != len(expected.TypeArgs) {
		return false
	}
	for index, argument := range ref.TypeArgs {
		if !threadConfigFieldTypeMatches(argument, expected.TypeArgs[index]) {
			return false
		}
	}
	return true
}
