package main

import (
	"strings"

	"sec/internal/sema"
)

// memberCompletionItems projects Sema's exact and eligible inherited member
// facts into LSP completion while retaining the static/instance namespace split.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Built-in type member lookup"
//   - rules/compiler/compiler_known_members.md — "Named and related types"
//   - rules/tooling/lsp.md — "Completion"
//
// When expected is non-nil the selector is the returned value, and only
// members whose value type matches the expected type are offered: fields,
// properties, and compiler-known members by their type, methods by any
// overload's return type, enum members and union variants by their owner.
//
// Rules:
//   - rules/tooling/lsp.md — "Completion", "Completion ranking" (expected type)
func memberCompletionItems(exprType sema.Type, analyzer *sema.Analyzer, prefix string, static bool, accessingModule string, expected *sema.Type) []completionItem {
	types, functions, symbols := analyzer.Types(), analyzer.Functions(), analyzer.Symbols()
	// Match Sema's field projection through a reference; this does not
	// introduce an implicit whole-value read of its referent.
	// rules/declarations/struct.md — §11;
	// rules/memory/references.md — §3.4(6).
	if !static && exprType.Kind == sema.ReferenceType && exprType.Element != nil {
		exprType = *exprType.Element
	}
	// Parameter and local facts may retain the compiler-known opaque fallback
	// captured before trusted core declarations refine a temporal identity.
	// Completion must project the analyzed declaration, including its private
	// backing fields, and leave visibility filtering to Sema below.
	//
	// Rules: rules/types/temporal.md — §2, compiler-owned representation
	if declared, ok := types[exprType.Name]; ok && exprType.Kind == sema.StructType && len(exprType.Fields) == 0 && len(declared.Fields) > 0 {
		exprType = declared
	}
	// MD-077: a hardware instance offers the public members of its register
	// type's impl, never the physical register fields.
	hardware := false
	if exprType.Kind == sema.ReferenceType && exprType.Element != nil && exprType.Element.Kind == sema.HardwareInstanceType {
		exprType = *exprType.Element
	}
	if exprType.Kind == sema.HardwareInstanceType && len(exprType.TypeArgs) == 1 {
		exprType, hardware = exprType.TypeArgs[0], true
		if declared, ok := types[exprType.Name]; ok {
			exprType = declared
		}
	}
	items := []completionItem{}
	seen := map[string]bool{}
	addTyped := func(item completionItem, valueTypes ...sema.Type) {
		if item.Label == "" || seen[item.Label] || !completionLabelMatches(item.Label, prefix) {
			return
		}
		if expected != nil {
			matched := false
			for _, valueType := range valueTypes {
				if completionTypeMatches(analyzer, *expected, valueType) {
					matched = true
					break
				}
			}
			if !matched {
				return
			}
		}
		seen[item.Label] = true
		items = append(items, item)
	}
	for _, member := range sema.CompilerKnownMembersForType(exprType, static) {
		if item, ok := compilerKnownCompletionItem(member); ok {
			addTyped(item, member.Result)
		}
	}
	for _, method := range interfaceCompletionMethods(exprType, static) {
		label, _ := interfaceMethodLabel(method)
		// Inherited requirements name their declaring interface
		// (interfaces.md § 5); the label carries the receiver capability.
		if owner, ok := analyzer.InterfaceRequirementOwnerAt(method.Token.File, method.Token.Line, method.Token.Column); ok && owner != exprTypeInterfaceName(exprType) {
			label += "\n\ninherited from " + owner
		}
		addTyped(completionItem{Label: method.Name, Kind: 2, Detail: lspTypeName(method.ReturnType), Documentation: label}, method.ReturnType)
	}

	// rules/declarations/enums.md section 16 and unions.md section 4 place
	// enum members and union variants in the declared type's namespace. Stored
	// struct/register fields remain instance members; offering them on Type.
	// would suggest an expression that Sema correctly rejects.
	switch exprType.Kind {
	case sema.StructType:
		if !static {
			for _, field := range exprType.Fields {
				if !analyzer.CanAccessStructFieldFromModule(exprType, field.Name, accessingModule) {
					continue
				}
				addTyped(completionItem{Label: field.Name, Kind: 5, Detail: lspTypeName(field.Type)}, field.Type)
			}
		}
	case sema.RegisterType:
		if !static && !hardware {
			for _, field := range exprType.RegisterFields {
				detail := lspTypeName(field.Type)
				if field.Access != "" {
					detail += " (" + string(field.Access) + ")"
				}
				addTyped(completionItem{Label: field.Name, Kind: 5, Detail: detail}, field.Type)
			}
		}
	case sema.EnumType:
		if static {
			for _, variant := range exprType.EnumValues {
				addTyped(completionItem{Label: variant, Kind: 20, Detail: lspTypeName(exprType)}, exprType)
			}
		}
	case sema.UnionType:
		if static {
			for _, variant := range exprType.UnionVariants {
				addTyped(completionItem{Label: variant.Name, Kind: 20, Detail: lspTypeName(exprType)}, exprType)
			}
		}
	}

	// rules/declarations/static.md, sections 8 and 11. Completion keeps the
	// static and instance member namespaces on their legal receiver forms.
	for _, property := range exprType.Properties {
		if property.Static == static {
			addTyped(completionItem{Label: property.Name, Kind: 10, Detail: propertyCompletionDetail(property)}, property.Type)
		}
	}
	if !static {
		for _, property := range analyzer.InheritedCoreProperties(exprType) {
			addTyped(completionItem{Label: property.Name, Kind: 10, Detail: lspTypeName(property.Type)}, property.Type)
		}
	}
	if !static {
		for _, event := range exprType.Events {
			addTyped(completionItem{Label: event.Name, Kind: 24, Detail: lspTypeName(event.Type)}, event.Type)
		}
	}

	for _, typeName := range memberCompletionOwnerNames(exprType, types) {
		methodPrefix := typeName + "."
		for name, overloads := range functions {
			if !strings.HasPrefix(name, methodPrefix) {
				continue
			}
			methodName := strings.TrimPrefix(name, methodPrefix)
			matching := make([]sema.Function, 0, len(overloads))
			returnTypes := make([]sema.Type, 0, len(overloads))
			for _, overload := range overloads {
				if overload.Static == static {
					matching = append(matching, overload)
					returnTypes = append(returnTypes, overload.ReturnType)
				}
			}
			if len(matching) > 0 {
				addTyped(completionItem{Label: methodName, Kind: 2, Detail: functionCompletionDetail(matching)}, returnTypes...)
			}
		}
		if static && typeName == exprType.Name {
			staticPrefix := typeName + "."
			for name, symbol := range symbols {
				if !strings.HasPrefix(name, staticPrefix) {
					continue
				}
				memberName := strings.TrimPrefix(name, staticPrefix)
				addTyped(completionItem{Label: memberName, Kind: 6, Detail: lspTypeName(symbol.Type)}, symbol.Type)
			}
		}
	}

	sortCompletionItems(items)
	return items
}

// memberCompletionOwnerNames preserves exact nominal members first and then
// exposes eligible members from the receiver's related underlying types.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Lookup order"
//   - rules/compiler/compiler_known_members.md — "Named and related types"
//   - rules/tooling/lsp.md — "Completion"
func memberCompletionOwnerNames(typ sema.Type, types map[string]sema.Type) []string {
	if typ.Name == "" {
		return nil
	}
	names := []string{typ.Name}
	if !typ.Named || typ.Kind != sema.StringType {
		return names
	}
	seen := map[string]bool{typ.Name: true}
	for typ.Underlying != "" && !seen[typ.Underlying] {
		seen[typ.Underlying] = true
		names = append(names, typ.Underlying)
		underlying, ok := types[typ.Underlying]
		if !ok {
			break
		}
		typ = underlying
	}
	return names
}
