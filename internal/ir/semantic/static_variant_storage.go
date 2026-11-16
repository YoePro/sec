package semantic

import (
	"fmt"

	"sec/internal/sema"
)

// Static union, Option and Result values (SVV-2): Semantic IR keeps Sema's
// active variant and its payload as an initializer tree, checked against the
// union definition or the Result's success and error types. No startup code
// constructs them.
//
// Rules:
//   - rules/compiler/compile_time_evaluation.md — §§ 32(3), 36
//   - rules/declarations/static.md — §§ 15–16
//   - rules/compiler/semantic_ir.md — § 28

// staticVariantInitializerOf converts Sema's active variant of a static
// tagged value; a payload it cannot represent leaves the static a boundary.
func (b *builder) staticVariantInitializerOf(variant sema.ResolvedStaticVariant, typeID TypeID) (StaticInitializer, bool) {
	payloadType, fields, ok := staticVariantPayload(b.module, typeID, variant.Name)
	if !ok {
		return StaticInitializer{}, false
	}
	result := &StaticVariantInitializer{Name: variant.Name}
	switch {
	case fields != nil:
		if variant.Payload == nil || !variant.Payload.Aggregate || len(variant.Payload.Components) != len(fields) {
			return StaticInitializer{}, false
		}
		payload := StaticInitializer{Aggregate: true, Components: []StaticInitializer{}}
		for index, field := range fields {
			converted, ok := b.staticInitializerOf(variant.Payload.Components[index], field.Type)
			if !ok {
				return StaticInitializer{}, false
			}
			payload.Components = append(payload.Components, converted)
		}
		result.Payload = &payload
	case payloadType != 0:
		if variant.Payload == nil {
			return StaticInitializer{}, false
		}
		converted, ok := b.staticInitializerOf(*variant.Payload, payloadType)
		if !ok {
			return StaticInitializer{}, false
		}
		result.Payload = &converted
	case variant.Payload != nil:
		return StaticInitializer{}, false
	}
	return StaticInitializer{Variant: result}, true
}

// staticVariantPayload is the payload shape of a named variant: a payload
// type, named fields, or neither for an empty variant and Ok of a
// void-success Result.
func staticVariantPayload(module *Module, typeID TypeID, name string) (TypeID, []UnionPayloadField, bool) {
	typ, ok := module.Types.Lookup(typeID)
	if !ok {
		return 0, nil, false
	}
	if typ.Kind == TypeResult {
		var payload TypeID
		switch name {
		case "Ok":
			payload = typ.Success
		case "Err":
			payload = typ.Error
		default:
			return 0, nil, false
		}
		if successType, known := module.Types.Lookup(payload); known && successType.Kind == TypeVoid {
			payload = 0
		}
		return payload, nil, true
	}
	definition, found := unionDefinitionOf(module, typeID)
	if !found {
		return 0, nil, false
	}
	for _, variant := range definition.Variants {
		if variant.Name != name {
			continue
		}
		switch variant.Kind {
		case UnionVariantEmpty:
			return 0, nil, true
		case UnionVariantSingle:
			return variant.Payload, nil, true
		case UnionVariantFields:
			return 0, append([]UnionPayloadField{}, variant.PayloadFields...), true
		}
	}
	return 0, nil, false
}

// verifyStaticVariant checks a static variant against its type: the variant
// exists, and its payload matches the variant's payload shape.
func verifyStaticVariant(module *Module, typeID TypeID, variant StaticVariantInitializer) error {
	payloadType, fields, ok := staticVariantPayload(module, typeID, variant.Name)
	if !ok {
		return fmt.Errorf("variant initializer names unknown variant %q", variant.Name)
	}
	switch {
	case fields != nil:
		if variant.Payload == nil || !variant.Payload.Aggregate || len(variant.Payload.Components) != len(fields) {
			return fmt.Errorf("variant %s initializer does not list every payload field", variant.Name)
		}
		for index, field := range fields {
			if err := verifyStaticInitializer(module, field.Type, variant.Payload.Components[index]); err != nil {
				return err
			}
		}
	case payloadType != 0:
		if variant.Payload == nil {
			return fmt.Errorf("variant %s initializer has no payload", variant.Name)
		}
		return verifyStaticInitializer(module, payloadType, *variant.Payload)
	case variant.Payload != nil:
		return fmt.Errorf("variant %s initializer has a payload its variant does not", variant.Name)
	}
	return nil
}

func unionDefinitionOf(module *Module, typeID TypeID) (UnionDefinition, bool) {
	for _, definition := range module.Unions {
		if definition.TypeID == typeID {
			return definition, true
		}
	}
	return UnionDefinition{}, false
}
