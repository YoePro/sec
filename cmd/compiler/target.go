package main

import (
	"fmt"

	"sec/internal/layout"
	platformtarget "sec/internal/platform/target"
)

type TargetStatus = platformtarget.Status

const (
	TargetImplemented  = platformtarget.Implemented
	TargetExperimental = platformtarget.Experimental
	TargetPlanned      = platformtarget.Planned
)

type TargetDefinition struct {
	platformtarget.Definition
}

var targets = compilerTargetDefinitions()

type CompilerTarget = platformtarget.Target

func compilerTargetDefinitions() []TargetDefinition {
	definitions := platformtarget.Definitions()
	result := make([]TargetDefinition, len(definitions))
	for index, definition := range definitions {
		result[index] = TargetDefinition{Definition: definition}
	}
	return result
}

func hostCompilerTarget() CompilerTarget {
	return platformtarget.Host()
}

func normalizeTargetOS(osName string) string {
	return platformtarget.NormalizeOS(osName)
}

func normalizeTargetArch(arch string) string {
	return platformtarget.NormalizeArch(arch)
}

func parseCompilerTarget(value string) (CompilerTarget, bool) {
	return platformtarget.Parse(value)
}

func findTargetDefinition(target CompilerTarget) (TargetDefinition, bool) {
	definition, ok := platformtarget.Find(target)
	return TargetDefinition{Definition: definition}, ok
}

func (definition TargetDefinition) scalarPlan() (layout.ResolvedScalarPlan, error) {
	return definition.ScalarPlan()
}

func requireTargetCanEmitLLVM(target CompilerTarget) (TargetDefinition, error) {
	definition, ok := findTargetDefinition(target)
	if !ok {
		return TargetDefinition{}, fmt.Errorf("unsupported target %s", target.String())
	}
	if !definition.CanEmitLLVM || definition.LLVMTriple == "" {
		return TargetDefinition{}, fmt.Errorf("target %s cannot emit LLVM yet", target.String())
	}
	return definition, nil
}

func requireTargetCanLink(target CompilerTarget) (TargetDefinition, error) {
	// rules/compiler/linking.md section 8: CanLink is only a coarse compiler
	// support gate. It does not define target linkage or replace LinkEnvironment.
	definition, err := requireTargetCanEmitLLVM(target)
	if err != nil {
		return TargetDefinition{}, err
	}
	if !definition.CanLink {
		return TargetDefinition{}, fmt.Errorf("target %s cannot link yet", target.String())
	}
	return definition, nil
}
