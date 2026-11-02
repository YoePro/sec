package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

type implStorageConflict struct {
	Kind  string
	Token lexer.Token
}

// collectImplStorageConflicts joins the valid primary/extension member surface
// before registering associated storage, so a later method, property, event or
// nested declaration cannot be hidden by an earlier static binding. Fields and
// register fields already belong to that same namespace. Method overloads are
// not duplicates here; their own signature checks remain authoritative.
// Rules: rules/declarations/static.md — §6;
// rules/declarations/impl.md — §§22–23;
// rules/declarations/properties.md — §12.
func (a *Analyzer) collectImplStorageConflicts(program *ast.Program) map[*ast.LetStatement]implStorageConflict {
	names := map[string]map[string]implStorageConflict{}
	bindings := map[string][]*ast.LetStatement{}
	for _, statement := range program.Statements {
		impl, ok := statement.(*ast.ImplStatement)
		if !ok || impl == nil || impl.Target == nil || !a.validImplStatements[impl] || a.shadowedUnitImpls[impl] {
			continue
		}
		owner := impl.Target.Name
		if names[owner] == nil {
			names[owner] = map[string]implStorageConflict{}
			target := a.types[owner]
			for _, field := range target.Fields {
				names[owner][field.Name] = implStorageConflict{"field", field.Token}
			}
			for _, field := range target.RegisterFields {
				if field.Name != "_" {
					names[owner][field.Name] = implStorageConflict{"register field", field.Token}
				}
			}
		}
		for _, member := range impl.Members {
			var name string
			var collision implStorageConflict
			switch member := member.(type) {
			case *ast.LetStatement:
				if member.Static && member.Name != nil {
					bindings[owner] = append(bindings[owner], member)
				}
				continue
			case *ast.FunctionDeclaration:
				if member.Name != nil {
					name = member.Name.Value
					collision = implStorageConflict{"method", member.Name.Token}
				}
			case *ast.PropertyDeclaration:
				if member.Name != nil {
					name = member.Name.Value
					collision = implStorageConflict{"property", member.Name.Token}
				}
			case *ast.EventDeclaration:
				if member.Name != nil {
					name = member.Name.Value
					collision = implStorageConflict{"event", member.Name.Token}
				}
			case *ast.TypeDeclStatement:
				if member.Name != nil {
					name = member.Name.Value
					collision = implStorageConflict{"nested type", member.Name.Token}
				}
			case *ast.EnumDeclaration:
				if member.Name != nil {
					name = member.Name.Value
					collision = implStorageConflict{"nested enum", member.Name.Token}
				}
			case *ast.UnitDeclStatement:
				if member.Name != nil {
					name = member.Name.Value
					collision = implStorageConflict{"nested unit", member.Name.Token}
				}
			}
			if name != "" {
				if _, exists := names[owner][name]; !exists {
					names[owner][name] = collision
				}
			}
		}
	}
	conflicts := map[*ast.LetStatement]implStorageConflict{}
	for owner, lets := range bindings {
		for _, binding := range lets {
			if collision, exists := names[owner][binding.Name.Value]; exists {
				conflicts[binding] = collision
			}
		}
	}
	return conflicts
}

// rejectImplStorageConflict emits both declaration sites before a conflicting
// static binding can enter symbols, completion or initializer analysis.
// Rules: rules/declarations/static.md — §6; rules/declarations/impl.md — §23;
// rules/tooling/diagnostics.md — §§2,5–7.
func (a *Analyzer) rejectImplStorageConflict(owner string, binding *ast.LetStatement) bool {
	collision, exists := a.implStorageConflicts[binding]
	if !exists {
		return false
	}
	a.addErrorAtTokenWithPreviousMetadata(binding.Name.Token, collision.Token, diagnostics.AssociatedStorageConflict,
		"Rename one declaration; primary and extension implementations share a member namespace, and static storage cannot share a name with an instance or associated member.",
		"associated member %s.%s conflicts with %s %s in %s", owner, binding.Name.Value, collision.Kind, binding.Name.Value, owner)
	return true
}
