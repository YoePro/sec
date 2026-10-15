package sema

import (
	"regexp"
	"strings"
)

var compilerKnownTemplateParameter = regexp.MustCompile(`\b(T|K|V|E|N)\b`)

// instantiateCompilerKnownPresentation binds receiver parameters in registry
// signatures without changing semantic member IDs or method-local generics.
// Replacements visit template tokens once, preserving inserted nominal names.
// Rules: rules/compiler/compiler_known_members.md — "Synthetic definitions",
// "Documentation generation", "Named and related types".
func instantiateCompilerKnownPresentation(member CompilerKnownMember, receiver Type) CompilerKnownMember {
	typ := dereferenceType(receiver)
	bindings := map[string]string{}
	if (typ.Kind == ArrayType || typ.Kind == SliceType) && typ.Element != nil {
		bindings["T"] = typeDisplayName(*typ.Element)
		if length, fixed := FixedArrayLength(typ); fixed {
			bindings["N"] = length.String()
		}
	} else if typ.Name == "map" && len(typ.TypeArgs) == 2 {
		bindings["K"] = typeDisplayName(typ.TypeArgs[0])
		bindings["V"] = typeDisplayName(typ.TypeArgs[1])
	} else if typ.Kind == ResultType && len(typ.TypeArgs) == 2 {
		bindings["T"] = typeDisplayName(typ.TypeArgs[0])
		bindings["E"] = typeDisplayName(typ.TypeArgs[1])
	} else if len(typ.TypeArgs) == 1 && (typ.Name == "list" || typ.Name == "set" || typ.Kind == RawPtrType || typ.Kind == InterfaceType) {
		bindings["T"] = typeDisplayName(typ.TypeArgs[0])
	} else if isNumericType(typ) {
		bindings["T"] = typeDisplayName(typ)
	}
	if member.ID == "CKM-PTR-VALUE" && len(member.Result.TypeArgs) == 1 {
		bindings["T"] = typeDisplayName(member.Result.TypeArgs[0])
	}
	bind := func(text string) string {
		return compilerKnownTemplateParameter.ReplaceAllStringFunc(text, func(parameter string) string {
			if value, ok := bindings[parameter]; ok {
				return value
			}
			return parameter
		})
	}
	member.Signature = bind(member.Signature)
	member.Receiver = bind(member.Receiver)
	if len(member.CallerObligations) > 0 {
		member.CallerObligations = append([]string(nil), member.CallerObligations...)
		for index := range member.CallerObligations {
			member.CallerObligations[index] = bind(member.CallerObligations[index])
		}
	}
	member.BoundReceiver = typeDisplayName(receiver)
	return member
}

// CompilerKnownDocumentationForType emits the same read-only registry text
// used by navigation, in registry order, for an instance or associated surface.
// This is derived API documentation, not a normative source declaration.
// Rules: rules/compiler/compiler_known_members.md — "Documentation generation",
// "Synthetic definitions".
func CompilerKnownDocumentationForType(receiver Type, static bool) string {
	var text strings.Builder
	for _, member := range CompilerKnownMembersForType(receiver, static) {
		text.WriteString(CompilerKnownSyntheticDefinition(member))
		text.WriteByte('\n')
	}
	return text.String()
}
