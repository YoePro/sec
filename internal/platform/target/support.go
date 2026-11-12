package target

import (
	"fmt"

	"sec/internal/diagnostics"
)

// PipelineStage is one compiler pipeline capability tracked per target
// (platform_model.md § 15).
type PipelineStage string

const (
	StageSemanticValidation PipelineStage = "semantic validation"
	StageAnalysis           PipelineStage = "analysis"
	StageSecMLIRLowering    PipelineStage = "Sec MLIR lowering"
	StageObjectEmission     PipelineStage = "object emission"
	StageLinking            PipelineStage = "linking"
	StageExecution          PipelineStage = "execution"
)

// PipelineStages lists the stages in pipeline order.
func PipelineStages() []PipelineStage {
	return []PipelineStage{StageSemanticValidation, StageAnalysis, StageSecMLIRLowering, StageObjectEmission, StageLinking, StageExecution}
}

// CompilerTargetSupport is the current compiler implementation's support
// for a target, per pipeline stage. It is separate from the platform's
// semantic capabilities (the resolved plan) and from toolchain availability
// on the host, which a valid plan does not require (platform_model.md
// §§ 13, 15).
type CompilerTargetSupport struct {
	Target Target
	stages map[PipelineStage]bool
}

// CompilerSupport derives the compiler's per-stage support for the target
// from its registry entry.
// Rules: rules/platform/platform_model.md — § 15.
func (definition Definition) CompilerSupport() CompilerTargetSupport {
	support := CompilerTargetSupport{Target: Target{OS: definition.OS, Arch: definition.Arch}, stages: map[PipelineStage]bool{}}
	support.stages[StageSemanticValidation] = definition.CanCheck
	support.stages[StageAnalysis] = definition.CanCheck
	support.stages[StageSecMLIRLowering] = definition.CanEmitLLVM && definition.LLVMTriple != ""
	support.stages[StageObjectEmission] = definition.CanEmitLLVM && definition.LLVMTriple != ""
	support.stages[StageLinking] = definition.CanLink
	support.stages[StageExecution] = definition.CanRun
	return support
}

// Supports reports whether the compiler implements a stage for the target.
func (support CompilerTargetSupport) Supports(stage PipelineStage) bool {
	return support.stages[stage]
}

// Require returns an UnsupportedStageError when the compiler does not
// implement the stage, or nil.
func (support CompilerTargetSupport) Require(stage PipelineStage) error {
	if support.Supports(stage) {
		return nil
	}
	return &UnsupportedStageError{Target: support.Target, Stage: stage}
}

// UnsupportedStageError reports a missing compiler pipeline stage for a
// target. It is an implementation-capability boundary: the program's
// semantic validity and the platform's capabilities are unaffected.
type UnsupportedStageError struct {
	Target Target
	Stage  PipelineStage
}

func (e *UnsupportedStageError) Error() string {
	return fmt.Sprintf("the compiler does not implement %s for target %s-%s yet; the target's semantic plan is valid, so check and analyse can still use it", e.Stage, e.Target.OS, e.Target.Arch)
}

// ImplementationCapability exposes the boundary to pipeline consumers.
// Rules: rules/compiler/compiler_pipeline.md — § 32(2)–(4).
func (e *UnsupportedStageError) ImplementationCapability() diagnostics.ImplementationCapability {
	return diagnostics.ImplementationCapability{Feature: string(e.Stage), Boundary: "compiler target support " + e.Target.OS + "-" + e.Target.Arch}
}
