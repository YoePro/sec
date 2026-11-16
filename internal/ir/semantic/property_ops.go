package semantic

import (
	"fmt"
	"strings"
)

// PropertyAccessKind names the property operation a call.direct performs.
// Semantic IR keeps a property access distinct from field access as a call of
// the accessor function annotated with its property operation, an expanded
// form that evaluates the receiver once (semantic_ir.md § 26(2)). A compound
// update is a getter call and a setter call sharing that receiver.
// Rules: rules/compiler/semantic_ir.md — § 26; rules/declarations/properties.md — § 15.
type PropertyAccessKind string

const (
	PropertyAccessRead          PropertyAccessKind = "read"
	PropertyAccessWrite         PropertyAccessKind = "write"
	PropertyAccessFallibleWrite PropertyAccessKind = "fallible-write"
	PropertyAccessCompoundRead  PropertyAccessKind = "compound-read"
	PropertyAccessCompoundWrite PropertyAccessKind = "compound-write"
)

// PropertyFacts identify the property operation of an accessor call: its kind,
// the owner type's canonical identity and the property name.
type PropertyFacts struct {
	Kind   PropertyAccessKind
	Owner  string
	Name   string
	Static bool
}

func formatPropertyFacts(facts *PropertyFacts) string {
	static := ""
	if facts.Static {
		static = " static"
	}
	return fmt.Sprintf(" property(%s%s %s.%s)", facts.Kind, static, facts.Owner, facts.Name)
}

// verifyPropertyFacts checks a property-annotated call against its accessor:
// a read or compound read calls a getter returning a value, a write or
// compound write a void setter, a fallible write a setter returning its
// Result, and an instance accessor receives its receiver reference first.
func verifyPropertyFacts(module *Module, op Operation, callee *Function) error {
	facts := op.Property
	if facts == nil {
		return nil
	}
	if op.Kind != OpDirectCall || facts.Owner == "" || facts.Name == "" {
		return fmt.Errorf("property facts require a direct accessor call with an owner and a name")
	}
	getter := facts.Kind == PropertyAccessRead || facts.Kind == PropertyAccessCompoundRead
	switch facts.Kind {
	case PropertyAccessRead, PropertyAccessCompoundRead, PropertyAccessWrite, PropertyAccessCompoundWrite, PropertyAccessFallibleWrite:
	default:
		return fmt.Errorf("property access has unknown kind %q", facts.Kind)
	}
	suffix := ".set"
	fallible := strings.HasSuffix(callee.Name, facts.Name+".try-set")
	if getter {
		suffix = ".get"
	} else if facts.Kind == PropertyAccessFallibleWrite || facts.Kind == PropertyAccessCompoundWrite && fallible {
		// A compound update of a `try set` property writes through it too.
		suffix = ".try-set"
	}
	if !strings.HasSuffix(callee.Name, facts.Name+suffix) {
		return fmt.Errorf("property %s %s calls %s, not its %s accessor", facts.Kind, facts.Name, callee.Name, strings.TrimPrefix(suffix, "."))
	}
	values := len(callee.Parameters)
	if !facts.Static {
		values--
		if len(callee.Parameters) == 0 {
			return fmt.Errorf("instance property %s accessor has no receiver", facts.Name)
		}
		if _, ok := referenceType(module.Types, callee.Parameters[0].Value.Type); !ok {
			return fmt.Errorf("instance property %s accessor does not take its receiver by reference", facts.Name)
		}
	}
	returnsVoid := typeHasKind(module.Types, callee.ReturnType, TypeVoid)
	switch {
	case getter && (values != 0 || returnsVoid):
		return fmt.Errorf("property %s getter must take no value and return one", facts.Name)
	case !getter && values != 1:
		return fmt.Errorf("property %s setter must take exactly one value", facts.Name)
	case (facts.Kind == PropertyAccessWrite || facts.Kind == PropertyAccessCompoundWrite && !fallible) && !returnsVoid:
		return fmt.Errorf("infallible property %s setter must return void", facts.Name)
	case (facts.Kind == PropertyAccessFallibleWrite || facts.Kind == PropertyAccessCompoundWrite && fallible) && !typeHasKind(module.Types, callee.ReturnType, TypeResult):
		return fmt.Errorf("fallible property %s setter must return its Result", facts.Name)
	}
	return nil
}
