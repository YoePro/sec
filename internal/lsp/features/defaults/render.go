package defaults

import (
	"strconv"
	"strings"

	"sec/internal/sema"
)

// sourceDefault emits complete allocation-free construction syntax from Sema's
// resolution. Bounded arrays never leak preview ellipses into source edits.
// Rules: rules/types/default_values.md — "Compile-time resolution", "LSP",
// "List defaults", "Empty struct literal".
func sourceDefault(typ sema.Type) (string, bool) {
	resolution := sema.DefaultValueOf(typ)
	switch resolution.Kind {
	case sema.NoDefault:
		return "", false
	case sema.StructDefault:
		return sourceTypeName(typ) + " {}", true
	case sema.CollectionDefault:
		base := typ
		base.Name = "list"
		value := sourceTypeName(base) + " {}"
		if typ.Named {
			value = typ.Name + "(" + value + ")"
		}
		return value, true
	case sema.ArrayDefault:
		length := len(resolution.Elements)
		if resolution.ArrayLengthDecimal != "" {
			n, err := strconv.ParseUint(resolution.ArrayLengthDecimal, 10, 64)
			if err != nil || n > 16 {
				return "", false
			}
			length = int(n)
		}
		if length == 0 {
			return "[]", true
		}
		if typ.Element == nil {
			return "", false
		}
		value, ok := sourceDefault(*typ.Element)
		if !ok {
			return "", false
		}
		if len(value) > 8192/length-2 {
			return "", false
		}
		values := make([]string, length)
		for i := range values {
			values[i] = value
		}
		return "[" + strings.Join(values, ", ") + "]", true
	default:
		value, _, ok := sema.DefaultValueDisplay(typ)
		return value, ok && value != ""
	}
}

// sourceTypeName retains mixed generic arguments in explicit constructors.
// Rules: rules/types/types.md — "Named types", "Generic types";
// rules/types/default_values.md — "List defaults", "Generic instances".
func sourceTypeName(typ sema.Type) string {
	name := typ.Name
	if len(typ.TypeArgs) == 0 && len(typ.ConstArgs) == 0 {
		return name
	}
	var args []string
	for _, arg := range typ.TypeArgs {
		args = append(args, sourceTypeName(arg))
	}
	for _, arg := range typ.ConstArgs {
		args = append(args, strconv.FormatInt(arg, 10))
	}
	return name + "[" + strings.Join(args, ", ") + "]"
}
