package sema

import (
	"math/big"
	"sec/internal/ast"
)

// genericRecursiveStorageKind classifies represented local by-value self-recursion.
// References and dynamic-storage descriptors terminate containment, including
// references to fixed arrays. The declaration layout graph separately handles
// containment through other source-declared types.
// Rules: rules/declarations/unions.md — §18;
// rules/memory/layout.md — §13(7), §25(1–5).
// rules/corrections/applied/correction8-20260823.md — Valid finite recursion, items 9–11.
func genericRecursiveStorageKind(owner string, parameters []string, ref *ast.TypeReference) string {
	if ref == nil || ref.Ref {
		return ""
	}

	if ref.ElementType != nil {
		length := big.NewInt(ref.ArrayLength)
		if ref.ArrayLengthExpression != nil {
			if exact, ok := constantIntegerValue(ref.ArrayLengthExpression); ok {
				length = exact
			}
		}
		if !ref.Slice && length.Sign() > 0 {
			return genericRecursiveStorageKind(owner, parameters, ref.ElementType)
		}
		return ""
	}

	if ref.Name != owner {
		return ""
	}

	if genericTypeArgsMatchParameters(ref.TypeArgs, parameters) {
		return "direct"
	}
	return "nonconverging"
}

// genericStructFieldHasDirectRecursiveStorage rechecks direct inline storage
// after substitution. Only fixed arrays embed their element by value;
// references, views, raw pointers and owning dynamic containers are boundaries.
// Rules: rules/declarations/unions.md — §18;
// rules/memory/layout.md — §13(7), §25(1–5);
// rules/collections/collections.md — §5.2 Layout, §6.1 Meaning.
func genericStructFieldHasDirectRecursiveStorage(owner Type, field Type) bool {
	if sameConcreteType(owner, field) {
		return true
	}
	switch field.Kind {
	case ArrayType:
		return arrayShapeOf(field) == ArrayShapeFixed && field.Element != nil && genericStructFieldHasDirectRecursiveStorage(owner, *field.Element)
	default:
		return false
	}
}
