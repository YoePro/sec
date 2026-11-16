package sema

// compatibleInterfaceMethodSignature compares finite interface inputs without
// inferring or erasing additional method-local generic parameters. Owner
// substitutions must already be bound by the caller.
// Rules: rules/declarations/interfaces.md — §§6,9;
// rules/declarations/generics.md — §§9–10,14,22.
func compatibleInterfaceMethodSignature(method Function, required Function) bool {
	if len(method.GenericParameters) != 0 || method.GenericTypePack != "" || len(required.GenericParameters) != 0 || required.GenericTypePack != "" {
		return false
	}
	methodParams := explicitInterfaceComparableParameters(method.Parameters)
	requiredParams := explicitInterfaceComparableParameters(required.Parameters)
	if len(methodParams) != len(requiredParams) {
		return false
	}
	for i := range methodParams {
		if methodParams[i].Ref != requiredParams[i].Ref || methodParams[i].MutableRef != requiredParams[i].MutableRef || methodParams[i].Consuming != requiredParams[i].Consuming {
			return false
		}
		if !sameConcreteType(methodParams[i].Type, requiredParams[i].Type) {
			return false
		}
	}
	return true
}
