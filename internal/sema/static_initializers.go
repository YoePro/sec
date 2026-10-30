package sema

import (
	"math/big"
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// ResolvedStaticInitializer is an immutable scalar materialization result from
// Sema, separate from a mutable static's runtime value. Empty means the shared
// evaluator cannot yet establish a represented scalar initializer.
// Rules: rules/declarations/static.md §§15–16; rules/compiler/semantic_ir.md §28.
type ResolvedStaticInitializer struct {
	Integer *big.Int
	Bool    *bool
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
		switch binding.Type.Kind {
		case IntType, UintType, CharType, RuneType, BoolType:
		default:
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
		switch binding.Type.Kind {
		case IntType, UintType, CharType, RuneType:
			if value.Integer != nil {
				binding.Initializer.Integer = new(big.Int).Set(value.Integer)
			}
		case BoolType:
			if value.Kind == BoolType {
				b := value.Bool
				binding.Initializer.Bool = &b
			}
		}
		a.staticBindings[key] = binding
	}
}

// cloneResolvedStaticInitializer protects published exact values from clients.
// Rules: rules/compiler/compiler_pipeline.md — immutable resolved facts; rules/declarations/static.md §§15–16.
func cloneResolvedStaticInitializer(binding ResolvedStatic) ResolvedStatic {
	binding.Initializer.Integer = clonePlaceConstantIndex(binding.Initializer.Integer)
	if binding.Initializer.Bool != nil {
		value := *binding.Initializer.Bool
		binding.Initializer.Bool = &value
	}
	return binding
}
