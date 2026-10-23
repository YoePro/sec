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
//   - rules/declarations/registers.md — § 11.7
func registerAccessHover(plan sema.ResolvedRegisterFieldPlan) string {
	contents := registerFieldHover(plan.Field)
	if plan.Request == nil {
		return contents
	}
	request := string(plan.Request.Kind)
	if plan.Request.Effect != "" {
		request += " (" + plan.Request.Effect + ")"
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
// after `self.` inside the impl of a register type, the only place they are
// reachable (MD-077 § 14; registers.md § 11.7).
func registerOperationCompletionItems(receiver sema.Type, prefix string) []completionItem {
	if receiver.Kind == sema.ReferenceType && receiver.Element != nil {
		receiver = *receiver.Element
	}
	if receiver.Kind != sema.RegisterType {
		return nil
	}
	var items []completionItem
	for _, item := range []completionItem{
		{Label: "Read", Kind: 2, Detail: "fn Read() " + receiver.Name + " (hardware observation)"},
		{Label: "Write", Kind: 2, Detail: "fn Write() / Write(value: " + receiver.Name + ") (hardware write)"},
	} {
		if strings.HasPrefix(item.Label, prefix) {
			items = append(items, item)
		}
	}
	return items
}

func registerOperationHover(operation sema.ResolvedRegisterOperation) string {
	signature, meaning := "", ""
	switch operation.Kind {
	case sema.RegisterOperationRead:
		signature = fmt.Sprintf("fn Read() %s", operation.Register.Name)
		meaning = "One logical hardware observation returning a detached snapshot."
	case sema.RegisterOperationCommit:
		signature = "fn Write() void"
		meaning = "Commits the pending shadow write intents; performs no physical write when none is pending."
	case sema.RegisterOperationWriteWhole:
		signature = fmt.Sprintf("fn Write(value: %s) void", operation.Register.Name)
		meaning = "Requests one whole-register hardware write of a detached value."
	}
	return fmt.Sprintf("```sec\n%s\n```\n\n%s\n\nCompiler-known hardware register operation, reachable only inside `impl %s` on a hardware receiver.", signature, meaning, operation.Register.Name)
}
