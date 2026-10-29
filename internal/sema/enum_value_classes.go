package sema

import "math/big"

// EnumValueClass is one resolved underlying value with its declaration-ordered
// member names. Aliases share a class; returned slices and values are detached.
type EnumValueClass struct {
	Members      []string
	IntegerValue *big.Int
	StringValue  *string
}

// EnumValueClasses exposes the same canonical identities used for enum match
// coverage. It performs no evaluation or source lookup, and preserves the
// declaration order of classes and aliases for diagnostics and tooling.
// For open enums these are only declared classes, not the complete runtime domain.
// Rules: rules/declarations/enums.md — §§7,14,18;
// rules/tooling/lsp.md — "Hover".
func EnumValueClasses(typ Type) []EnumValueClass {
	if typ.Kind != EnumType {
		return nil
	}
	classes := []EnumValueClass{}
	indices := map[string]int{}
	for _, name := range typ.EnumValues {
		value, exists := typ.EnumConsts[name]
		if !exists {
			continue
		}
		key, resolved := enumValueClassKey(value)
		if !resolved {
			continue
		}
		index, known := indices[key]
		if !known {
			index = len(classes)
			indices[key] = index
			class := EnumValueClass{}
			if value.Value != nil {
				class.IntegerValue = new(big.Int).Set(value.Value)
			}
			if value.StringValue != nil {
				text := *value.StringValue
				class.StringValue = &text
			}
			classes = append(classes, class)
		}
		classes[index].Members = append(classes[index].Members, name)
	}
	return classes
}
