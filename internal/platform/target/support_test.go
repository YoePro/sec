package target

import (
	"errors"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// The compiler's per-stage support is a separate fact from the platform's
// semantic plan: a planned target has a valid semantic plan and supports
// checking, while emission is an implementation-capability boundary, never
// an invalid program or a missing platform capability.
// Rules: rules/platform/platform_model.md — §§ 13, 15; rules/compiler/compiler_pipeline.md — § 32.
func TestCompilerTargetSupportIsSeparateFromPlatformSemantics(t *testing.T) {
	implemented, ok := Find(Target{OS: "linux", Arch: "amd64"})
	if !ok {
		t.Fatal("linux-amd64 missing")
	}
	for _, stage := range []PipelineStage{StageSemanticValidation, StageAnalysis, StageSecMLIRLowering, StageObjectEmission, StageLinking} {
		if !implemented.CompilerSupport().Supports(stage) {
			t.Errorf("linux-amd64 lacks %s", stage)
		}
	}
	wasm, ok := Find(Target{OS: "unknown", Arch: "wasm32"})
	if !ok {
		t.Fatal("unknown-wasm32 missing")
	}
	if _, err := wasm.ScalarPlan(); err != nil {
		t.Fatalf("planned target has no semantic plan: %v", err)
	}
	support := wasm.CompilerSupport()
	if !support.Supports(StageSemanticValidation) || support.Supports(StageObjectEmission) {
		t.Errorf("wasm32 support = %+v", support)
	}
	err := support.Require(StageObjectEmission)
	var capability diagnostics.CapabilityFailure
	if !errors.As(err, &capability) || capability.ImplementationCapability().Feature != string(StageObjectEmission) ||
		!strings.Contains(err.Error(), "the compiler does not implement object emission for target unknown-wasm32 yet; the target's semantic plan is valid") {
		t.Errorf("emission error = %v", err)
	}
}
