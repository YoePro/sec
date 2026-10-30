package main

import (
	"fmt"
	"strings"

	"sec/internal/sema"
)

// registerAccessHover presents Sema's logical hardware request of a register
// field step and the compiler-known Read()/Write() operations, so tooling
// shows what the field syntax means on hardware without re-deriving it.
//
// Rules:
//   - rules/platform/hardware-register-access.md — §§ 4, 7–9, 13.1, 33.3
//   - rules/declarations/registers.md — §§ 11.3, 11.7
func registerAccessHover(plan sema.ResolvedRegisterFieldPlan) string {
	contents := registerFieldHover(plan.Field)
	if plan.Request == nil {
		return contents
	}
	request := string(plan.Request.Kind)
	if plan.Request.Effect != "" {
		request += " (" + plan.Request.Effect + ")"
	}
	// MD-112: the logical per-bit action mask, or its value dependence.
	switch {
	case plan.Request.ActionMask != "":
		request += " mask 0b" + plan.Request.ActionMask
	case plan.Request.ValueDependent:
		request += " mask from the written value"
	}
	switch plan.Root {
	case sema.RegisterRootHardware:
		contents += fmt.Sprintf("\n\nHardware request: `%s`", request)
	case sema.RegisterRootImplReceiver:
		contents += fmt.Sprintf("\n\nOn a hardware instance: `%s`. On a software value: an ordinary field.", request)
	}
	return contents
}

// registerOperationCompletionItems offers the compiler-known Read()/Write()
// and IsShadowKnown() after `self.` inside the impl of a register type, the
// only place they are reachable, from the compiler-known registry (MD-077
// § 14; MD-113 § 9(7); registers.md § 11.7).
func registerOperationCompletionItems(receiver sema.Type, prefix string) []completionItem {
	if receiver.Kind == sema.ReferenceType && receiver.Element != nil {
		receiver = *receiver.Element
	}
	if receiver.Kind != sema.RegisterType {
		return nil
	}
	details := map[string][]string{}
	var order []string
	for _, member := range sema.CompilerKnownRegisterOperations(receiver) {
		if _, seen := details[member.Name]; !seen {
			order = append(order, member.Name)
		}
		details[member.Name] = append(details[member.Name], member.Signature)
	}
	var items []completionItem
	for _, name := range order {
		if strings.HasPrefix(name, prefix) {
			items = append(items, completionItem{Label: name, Kind: 2, Detail: strings.Join(details[name], " / ")})
		}
	}
	return items
}

// registerOperationHover presents the registry entry of a resolved register
// operation, and the resolved field path of an IsShadowKnown query.
func registerOperationHover(operation sema.ResolvedRegisterOperation) string {
	signature, meaning := "", ""
	for _, member := range sema.CompilerKnownRegisterOperations(operation.Register) {
		if member.ID == operation.ID {
			signature, meaning = member.Signature, member.Documentation
		}
	}
	if operation.Kind == sema.RegisterOperationShadowKnown && len(operation.FieldPath) > 0 {
		names := make([]string, 0, len(operation.FieldPath))
		for _, field := range operation.FieldPath {
			names = append(names, field.Name)
		}
		signature = fmt.Sprintf("fn IsShadowKnown(%s.%s) bool", operation.Register.Name, strings.Join(names, "."))
		leaf := operation.FieldPath[len(operation.FieldPath)-1]
		meaning += fmt.Sprintf(" Selected field: `%s` (%s, %s).", leaf.Name, leaf.Access, leaf.Shadow)
	}
	return fmt.Sprintf("```sec\n%s\n```\n\n%s\n\nCompiler-known hardware register operation, reachable only inside `impl %s` on a hardware receiver.", signature, meaning, operation.Register.Name)
}
