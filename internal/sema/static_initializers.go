package sema

import (
	"math/big"
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// ResolvedStaticInitializer is an immutable materialization result from Sema,
// separate from a mutable static's runtime value. Empty means no represented
// initializer. Variant and Scalar retain semantic facts for later lowering;
// their presence does not claim IR/schema support for those representations.
// Rules: rules/declarations/static.md §§15–16; rules/compiler/semantic_ir.md §28.
type ResolvedStaticInitializer struct {
	Integer *big.Int
	Bool    *bool
	Variant *ResolvedStaticVariant
	// Scalar retains other canonical CTE scalar values in a tagged payload.
	Scalar     *DefaultConstant
	ScalarType Type
	// Aggregate marks a struct or fixed-array value; Components are its
	// fields in declaration order or its elements in index order
	// (compile_time_evaluation.md § 32).
	Aggregate  bool
	Components []ResolvedStaticInitializer
}

// freezeStaticScalarInitializers resolves declaration dependencies by binding
// identity through the shared semantic CTE evaluator, not through runtime flow
// state or a name-based IR evaluator. No function execution/startup is invented.
// Rules: rules/declarations/static.md §§15–16; rules/compiler/compile_time_evaluation.md §§2(3),47.
func (a *Analyzer) freezeStaticScalarInitializers() {
	if len(a.staticBindings) == 0 {
		return
	}
	// Promoting optional guidance must not change mandatory compiler facts.
	for _, err := range a.errors {
		if definition, known := diagnostics.Lookup(err.ID); !known || definition.Mandatory {
			return
		}
	}
	values := map[StaticBindingID]DefaultConstant{}
	visiting := map[StaticBindingID]bool{}
	settled := map[StaticBindingID]bool{}
	var evaluate func(ResolvedStatic) (DefaultConstant, compileTimeOutcome)
	evaluate = func(binding ResolvedStatic) (DefaultConstant, compileTimeOutcome) {
		if settled[binding.ID] {
			if v, ok := values[binding.ID]; ok {
				return v, compileTimeEvaluated
			}
			return DefaultConstant{}, compileTimeNotConstant
		}
		if visiting[binding.ID] || binding.Declaration == nil {
			return DefaultConstant{}, compileTimeNotConstant
		}
		if !staticMaterializable(binding.Type) {
			return DefaultConstant{}, compileTimeNotConstant
		}
		visiting[binding.ID] = true
		defer delete(visiting, binding.ID)
		defer func() { settled[binding.ID] = true }()
		module := a.currentModule
		a.currentModule = binding.Module
		defer func() { a.currentModule = module }()
		resolve := func(identifier *ast.Identifier) (DefaultConstant, compileTimeOutcome) {
			source, ok := a.ResolvedStaticOf(identifier)
			if !ok {
				return DefaultConstant{}, compileTimeNotConstant
			}
			// An initializer depends on the source declaration's initial value, never
			// on its later runtime stores. Definition identities handle lexical shadowing.
			return evaluate(source)
		}
		value, outcome := a.semanticCompileTimeConstantVisiting(binding.Declaration.Value, map[string]bool{}, binding.Type, resolve)
		if outcome == compileTimeEvaluated {
			values[binding.ID] = value
		}
		return value, outcome
	}
	for _, binding := range a.ResolvedStaticDeclarations() {
		evaluate(binding)
	}
	for key, binding := range a.staticBindings {
		value, known := values[binding.ID]
		if !known {
			continue
		}
		if initializer, ok := staticInitializerOf(binding.Type, value); ok {
			binding.Initializer = initializer
		}
		a.staticBindings[key] = binding
	}
}

// cloneResolvedStaticInitializer protects published exact values from clients.
// Rules: rules/compiler/compiler_pipeline.md — immutable resolved facts; rules/declarations/static.md §§15–16.
func cloneResolvedStaticInitializer(binding ResolvedStatic) ResolvedStatic {
	binding.Initializer = cloneStaticInitializer(binding.Initializer)
	return binding
}

// cloneStaticInitializer recursively detaches published scalar, variant and
// aggregate initializer facts, independently of mutable runtime static values.
// Rules: rules/compiler/compiler_pipeline.md — immutable resolved facts;
// rules/compiler/compile_time_evaluation.md — §32(3).
func cloneStaticInitializer(initializer ResolvedStaticInitializer) ResolvedStaticInitializer {
	initializer.Variant = cloneStaticVariant(initializer.Variant)
	if initializer.Scalar != nil {
		value := cloneStaticScalar(*initializer.Scalar)
		initializer.Scalar = &value
		initializer.ScalarType = snapshotStaticVariantType(initializer.ScalarType)
	}
	initializer.Integer = clonePlaceConstantIndex(initializer.Integer)
	if initializer.Bool != nil {
		value := *initializer.Bool
		initializer.Bool = &value
	}
	if initializer.Components != nil {
		components := make([]ResolvedStaticInitializer, len(initializer.Components))
		for index, component := range initializer.Components {
			components[index] = cloneStaticInitializer(component)
		}
		initializer.Components = components
	}
	return initializer
}

// staticMaterializable reports a static type whose compile-time value
// Sema can attempt to publish. Tagged values validate their active payload
// separately. This predicate does not certify Semantic IR/schema support.
// Rules: rules/compiler/compile_time_evaluation.md — §32(1–4).
func staticMaterializable(typ Type) bool {
	if typ.Kind == UnionType || typ.Kind == ResultType {
		// Only the active payload is stored; its representation is checked
		// by the shared CTE boundary and staticVariantInitializerOf.
		return !typ.CustomFree
	}
	switch typ.Kind {
	case IntType, UintType, CharType, RuneType, BoolType:
		return true
	case FloatType:
		// P20 § 74: the resolved binary value is static data (bits leaf).
		return true
	case StructType:
		if typ.CustomFree || len(typ.Fields) == 0 {
			return false
		}
		for _, field := range typ.Fields {
			if !staticMaterializable(field.Type) {
				return false
			}
		}
		return true
	case ArrayType:
		return arrayShapeOf(typ) == ArrayShapeFixed && typ.Element != nil && staticMaterializable(*typ.Element)
	}
	return false
}

// staticInitializerOf converts a compile-time value into the published
// initializer tree, in declaration or index order.
// Rules: rules/compiler/compile_time_evaluation.md — §§31–32,36,47.
func staticInitializerOf(typ Type, value DefaultConstant) (ResolvedStaticInitializer, bool) {
	if typ.Kind == UnionType || typ.Kind == ResultType {
		return staticVariantInitializerOf(typ, value)
	}
	switch typ.Kind {
	case FloatType, DecimalType, StringType, EnumType, UnitDimensionType:
		if value.Aggregate != nil {
			return ResolvedStaticInitializer{}, false
		}
		constant := cloneStaticScalar(value)
		return ResolvedStaticInitializer{Scalar: &constant, ScalarType: snapshotStaticVariantType(typ)}, true
	case IntType, UintType, CharType, RuneType:
		if value.Integer == nil {
			return ResolvedStaticInitializer{}, false
		}
		return ResolvedStaticInitializer{Integer: new(big.Int).Set(value.Integer)}, true
	case BoolType:
		if value.Kind != BoolType {
			return ResolvedStaticInitializer{}, false
		}
		b := value.Bool
		return ResolvedStaticInitializer{Bool: &b}, true
	case StructType:
		if value.Aggregate == nil {
			return ResolvedStaticInitializer{}, false
		}
		result := ResolvedStaticInitializer{Aggregate: true}
		for _, field := range typ.Fields {
			component, known := value.Aggregate.Fields[field.Name]
			if !known {
				return ResolvedStaticInitializer{}, false
			}
			converted, ok := staticInitializerOf(field.Type, component)
			if !ok {
				return ResolvedStaticInitializer{}, false
			}
			result.Components = append(result.Components, converted)
		}
		return result, true
	case ArrayType:
		if value.Aggregate == nil || typ.Element == nil {
			return ResolvedStaticInitializer{}, false
		}
		result := ResolvedStaticInitializer{Aggregate: true, Components: []ResolvedStaticInitializer{}}
		for _, element := range value.Aggregate.Elements {
			converted, ok := staticInitializerOf(*typ.Element, element)
			if !ok {
				return ResolvedStaticInitializer{}, false
			}
			result.Components = append(result.Components, converted)
		}
		return result, true
	}
	return ResolvedStaticInitializer{}, false
}
