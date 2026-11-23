package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// declareAllocationContext gives a function that requires ambient
// allocation context (Sema's § 77 summary) its compiler-hidden first input,
// from package 19 on. A program or test entry root instead produces the
// target-provided context in its entry block (it returns the profile then);
// a task/thread entry does not inherit its spawner's context and an
// exported/foreign-ABI function cannot gain one, so both stay named
// boundaries. The source signature never changes.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 51-64, 107-109.
func (fb *functionBuilder) declareAllocationContext(decl *ast.FunctionDeclaration, resolved sema.Function) (string, error) {
	b := fb.owner
	if b.maxPackage < arenaLoweringPackage || decl.Name == nil || resolved.Extern || !b.analyzer.RequiresAllocationContextAt(decl.Name.Token) {
		return "", nil
	}
	token := decl.Name.Token
	switch b.analyzer.CallRootKindAt(token) {
	case sema.CallRootProgramEntry, sema.CallRootTestEntry:
		capabilities := b.analyzer.AllocationCapabilities()
		if !capabilities.HasActiveArenaContext() {
			return "", fb.unsupported("entry root requiring an allocation context without a target-provided context (profile "+capabilities.Profile+")", token)
		}
		return capabilities.Profile, nil
	case sema.CallRootTaskEntry, sema.CallRootThreadEntry:
		return "", fb.unsupported("task/thread entry requiring an allocation context (a spawned execution does not inherit its parent's context, P19 § 63)", token)
	}
	if resolved.ABI != "" {
		return "", fb.unsupported("exported function requiring an allocation context (needs a wrapper or Sec-aware ABI contract, P19 § 64)", token)
	}
	context := b.module.Types.InternAllocationContext()
	value := fb.newValue(context, OwnershipCompilerTemporary, location(token))
	fb.fn.Parameters = append(fb.fn.Parameters, Parameter{Name: "$context", Value: value, HiddenAllocationContext: true, Location: location(token)})
	fb.fn.RequiresAllocationContext = true
	fb.allocationContext = &AllocationContextFacts{Value: value.ID, Origin: ContextPropagated}
	return "", nil
}

// produceTargetAllocationContext creates the target-provided context of an
// entry root from its resolved profile (§ 60).
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 53, 60, 119.
func (fb *functionBuilder) produceTargetAllocationContext(profile string, decl *ast.FunctionDeclaration) {
	loc := location(decl.Name.Token)
	context := fb.owner.module.Types.InternAllocationContext()
	value := fb.newValue(context, OwnershipCompilerTemporary, loc)
	facts := &AllocationContextFacts{Value: value.ID, Origin: ContextTarget, OriginPlan: "target-provided:" + profile}
	fb.emit(Operation{Kind: OpAllocationContextTarget, Results: []Value{value}, AllocationContext: facts, Location: loc})
	fb.allocationContext = facts
}

// forwardAllocationContext passes the active context as the first operand
// of a direct call to a function that requires one; other calls get none.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 56, 58, 108.
func (fb *functionBuilder) forwardAllocationContext(op *Operation, resolved sema.ResolvedCall, call *ast.CallExpression) error {
	if fb.owner.maxPackage < arenaLoweringPackage || op.Kind != OpDirectCall || resolved.Function.Extern ||
		!fb.owner.analyzer.RequiresAllocationContextAt(resolved.Function.Token) {
		return nil
	}
	if fb.allocationContext == nil {
		return fb.unsupported("call to a function requiring an allocation context from a caller without one", call.Token)
	}
	facts := *fb.allocationContext
	op.Operands = append([]ValueID{facts.Value}, op.Operands...)
	op.ArgumentActions = append([]ArgumentAction{ArgumentAllocationContext}, op.ArgumentActions...)
	op.AllocationContext = &facts
	return nil
}
