package sema

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/lexer"
	"sort"
)

// ForBindingResolution is a detached view of a successfully checked loop
// binding. A mode describes access authority, not a promise of copy or move.
// Rules: rules/control-flow/flowcontrol_for.md — §§4–10;
// rules/tooling/lsp.md — Hover; rules/compiler/compiler_analysis.md — §18.
type ForBindingResolution struct {
	Loop, Binding, ModeToken lexer.Token
	Name, Mode, Type         string
	Discard                  bool
}

// recordForBindingModes publishes facts only after the complete header passes
// Sema; invalid ref categories and source authority never become editor facts.
// Rules: rules/control-flow/flowcontrol_for.md — §§4–10,12–13.
func (a *Analyzer) recordForBindingModes(stmt *ast.ForStatement, types []Type) {
	if a.summaryPass {
		return
	}
	if a.forBindingModes == nil {
		a.forBindingModes = map[sourceTokenKey]ForBindingResolution{}
	}
	for i, binding := range stmt.Bindings {
		mode := string(binding.Mode)
		if mode == "" {
			mode = "value"
		}
		if binding.Discard {
			mode = "discard"
		}
		a.forBindingModes[sourceTokenLocation(binding.Token)] = ForBindingResolution{
			Loop: stmt.Token, Binding: binding.Token, ModeToken: binding.ModeToken,
			Name: binding.Name, Mode: mode, Type: typeDisplayName(types[i]), Discard: binding.Discard,
		}
	}
}

// ForBindingModes returns deterministic, detached facts from this analysis,
// without inferring types or rediscovering loop-binding syntax in the editor.
// Rules: rules/control-flow/flowcontrol_for.md — §§4–10;
// rules/compiler/compiler_analysis.md — §18.
func (a *Analyzer) ForBindingModes() []ForBindingResolution {
	if a == nil {
		return nil
	}
	facts := make([]ForBindingResolution, 0, len(a.forBindingModes))
	for _, fact := range a.forBindingModes {
		facts = append(facts, fact)
	}
	sort.Slice(facts, func(i, j int) bool {
		a, b := facts[i].Binding, facts[j].Binding
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
	return facts
}

// Display names the validated access mode separately from the resolved type.
// Rules: rules/control-flow/flowcontrol_for.md — §§4–10.
func (fact ForBindingResolution) Display() string {
	return fmt.Sprintf("Loop binding `%s`: `%s`; binding mode: `%s`", fact.Name, fact.Type, fact.Mode)
}
