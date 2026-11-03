package semantic

import (
	"fmt"

	"sec/internal/sema"
)

// unresolvedConcreteStorage finds the first unresolved semantic storage fact,
// retaining the field/variant path. Check the supplied metadata before nominal
// cache reuse or IR allocation: an existing representation never proves that a
// later semantic instance is closed. No physical layout facts are required here.
// Rules: rules/compiler/generics_lowering.md — §§4,7–8;
// rules/declarations/generics.md — §§23,25;
// rules/compiler/monomorphization.md — §13.3.
func unresolvedConcreteStorage(typ sema.Type, path string) string {
	switch typ.Kind {
	case sema.InvalidType, sema.GenericType, sema.GenericTypePackType:
		return path
	}
	for index, argument := range typ.TypeArgs {
		if found := unresolvedConcreteStorage(argument, fmt.Sprintf("%s.argument[%d]", path, index)); found != "" {
			return found
		}
	}
	for _, nested := range []struct {
		typ    *sema.Type
		suffix string
	}{
		{typ.Element, ".element"}, {typ.NamedBase, ".base"}, {typ.FunctionReturnType, ".return"},
	} {
		if nested.typ != nil {
			if found := unresolvedConcreteStorage(*nested.typ, path+nested.suffix); found != "" {
				return found
			}
		}
	}
	for index, parameter := range typ.FunctionParameterTypes {
		if found := unresolvedConcreteStorage(parameter, fmt.Sprintf("%s.parameter[%d]", path, index)); found != "" {
			return found
		}
	}
	for _, field := range typ.Fields {
		if found := unresolvedConcreteStorage(field.Type, path+"."+field.Name); found != "" {
			return found
		}
	}
	for _, variant := range typ.UnionVariants {
		variantPath := path + "." + variant.Name
		if variant.Payload != nil {
			if found := unresolvedConcreteStorage(*variant.Payload, variantPath); found != "" {
				return found
			}
		}
		for _, field := range variant.PayloadFields {
			if found := unresolvedConcreteStorage(field.Type, variantPath+"."+field.Name); found != "" {
				return found
			}
		}
	}
	return ""
}
