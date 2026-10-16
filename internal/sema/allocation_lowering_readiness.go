package sema

import (
	"fmt"
	"reflect"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// AllocationLoweringError retains the exact semantic selection at an unsupported
// representation boundary. CreatedDomain is never substituted for backing.
// Rules: rules/memory/allocation.md — §§25(2),(5),26(1)–(3).
type AllocationLoweringError struct {
	Fact   AllocationContextFact
	Reason string
}

// Error explains why lowering cannot preserve the compiler-selected allocation.
// Rules: rules/memory/allocation.md — §§26(1)–(3),28(4).
func (e *AllocationLoweringError) Error() string {
	message := fmt.Sprintf("allocation lowering is unsupported: %s (%s)", e.Reason, AllocationContextDescription(e.Fact))
	if e.Fact.Source.Line > 0 {
		message += fmt.Sprintf(" at %s:%d:%d", e.Fact.Source.File, e.Fact.Source.Line, e.Fact.Source.Column)
	}
	return message
}

// ImplementationCapability keeps an unsupported backend distinct from invalid
// source and publishes the original allocation operation's location.
// Rules: rules/compiler/compiler_pipeline.md — §§32(1–5),58(3).
func (e *AllocationLoweringError) ImplementationCapability() diagnostics.ImplementationCapability {
	return diagnostics.ImplementationCapability{Feature: e.Reason + "; " + AllocationContextDescription(e.Fact), Boundary: "allocation lowering", File: e.Fact.Source.File, Line: e.Fact.Source.Line, Column: e.Fact.Source.Column}
}

// ValidateAllocationLoweringReadiness prevents maintained IR and analyzed legacy
// backends from erasing or replacing primitive allocation context/domain facts.
// No current backend represents these operations with their selected allocator,
// so they fail before producing output, even when selection is known. Source
// calls to foreign/user functions do not themselves choose a backend allocator.
// Folded strings and proven non-growing operations have no allocation effect.
// An empty module selects all non-core output modules; a named module selects
// only its own operations. Detached AST subsets may reuse original statements.
// Rules: rules/memory/allocation.md — §§19,25(1)–(5),26(1)–(3);
// rules/compiler/compiler_pipeline.md — §§32–34.
func (a *Analyzer) ValidateAllocationLoweringReadiness(program *ast.Program, module string) error {
	if a == nil || program == nil || a.iteratorLoweringProgram == nil {
		return &AllocationLoweringError{Reason: "completed allocation facts for this program are required"}
	}
	owned := map[ast.Statement]bool{}
	for _, statement := range a.iteratorLoweringProgram.Statements {
		owned[statement] = true
	}
	for _, statement := range program.Statements {
		if !owned[statement] {
			return &AllocationLoweringError{Reason: "allocation facts belong to a different program snapshot"}
		}
	}
	selected := map[CallableID]bool{}
	walkASTValue(reflect.ValueOf(program), func(value any) {
		var token lexer.Token
		switch value := value.(type) {
		case *ast.FunctionDeclaration:
			if value.Name != nil {
				token = value.Name.Token
			}
		case *ast.LambdaExpression:
			token = value.Token
		case *ast.DeferStatement:
			token = value.Token
		case *ast.FreeDeclaration:
			token = value.Token
		case *ast.TestDeclaration:
			token = value.Token
		default:
			return
		}
		for _, node := range a.callGraph.NodesForDeclaration(token) {
			selected[node.ID] = true
		}
	})
	for _, fact := range a.AllocationContextFacts() {
		node, exists := a.callGraph.Node(fact.Callable)
		if !exists || !selected[node.ID] || module != "" && node.Module != module || a.iteratorLoweringProgram.SourceProvenance[fact.Source.File] == ast.SourceCore {
			continue
		}
		switch fact.Kind {
		case ArenaEffectAllocate, ArenaEffectCollectionGrowth, ArenaEffectCreateOwned, ArenaEffectCreateGrowable:
		default:
			continue
		}
		reason := "the selected allocation context/domain has no preserving backend representation"
		if !fact.ContextKnown && fact.SelectedDomain == "" {
			reason = "the selected allocation context/domain is unresolved"
			if fact.CreatedDomain != "" {
				reason = "the backing allocation context/domain is unresolved; a created Arena domain does not select its backing allocator"
			}
		}
		return &AllocationLoweringError{Fact: fact, Reason: reason}
	}
	return nil
}
