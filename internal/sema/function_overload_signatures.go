package sema

// sameFunctionSignature compares complete overload input signatures; return
// types and consuming mode do not distinguish declarations. Method parameter
// names are alpha-equivalent, while generic owner identities remain separate.
// Rules: rules/declarations/functions.md — §§11,19;
// rules/declarations/generics.md — §§9,29–30.
func sameFunctionSignature(left Function, right Function) bool {
	if len(left.Parameters) != len(right.Parameters) {
		return false
	}
	leftTypes, rightTypes := genericOverloadParameterTypes(left), genericOverloadParameterTypes(right)
	for index := range left.Parameters {
		if left.Parameters[index].Ref != right.Parameters[index].Ref || left.Parameters[index].MutableRef != right.Parameters[index].MutableRef || left.Parameters[index].Variadic != right.Parameters[index].Variadic {
			return false
		}
		if !sameConcreteType(leftTypes[index], rightTypes[index]) {
			return false
		}
	}
	return true
}
