package secmlir

import (
	"fmt"

	"sec/internal/diagnostics"
	"sec/internal/ir/semantic"
)

// UnsupportedLoweringError marks valid Semantic IR that the current high-level
// Sec MLIR schema cannot represent. It is intentionally distinct from Semantic
// IR verification.
type UnsupportedLoweringError struct {
	Feature       string
	Function      semantic.FunctionID
	SchemaVersion uint32
}

// schemaVersion preserves an explicitly requested unsupported schema, while
// existing operation failures retain their established default schema boundary.
// Rules: rules/compiler/compiler_pipeline.md — §32(1–5), §35(4).
func (e *UnsupportedLoweringError) schemaVersion() uint32 {
	if e.SchemaVersion != 0 {
		return e.SchemaVersion
	}
	return dialectSchemaVersion
}

// ImplementationCapability preserves the emitter's exact feature, callable
// identity and schema boundary; a callable ID is not a source location.
// Rules: rules/compiler/compiler_pipeline.md — §32(1–5), §35(4).
func (e *UnsupportedLoweringError) ImplementationCapability() diagnostics.ImplementationCapability {
	return diagnostics.ImplementationCapability{Feature: e.Feature, Callable: string(e.Function), Boundary: fmt.Sprintf("Sec MLIR schema %d", e.schemaVersion())}
}

// Error names the exact requested schema when supplied, retaining the existing
// default diagnostic boundary for predecessor operation errors.
// Rules: rules/compiler/compiler_pipeline.md — §32(1–5), §35(4).
func (e *UnsupportedLoweringError) Error() string {
	if e.Function == "" {
		return fmt.Sprintf("Sec MLIR schema %d does not support %s", e.schemaVersion(), e.Feature)
	}
	return fmt.Sprintf("Sec MLIR schema %d does not support %s in %s", e.schemaVersion(), e.Feature, e.Function)
}

// AtomicCapabilityEvidenceError is the MD-095 missing-evidence boundary: an
// atomic operation whose capability is still an open obligation (Unknown) at
// executable emission. It is distinct from a source error, a proven
// unsupported capability (S1261, reported by Sema) and an unimplemented
// lowering (UnsupportedLoweringError); no output is produced.
// Rules: rules/concurrency/atomics.md — §§ 37A(12), 47(9), 48(4);
// rules/tooling/diagnostics.md — § 24(4l).
type AtomicCapabilityEvidenceError struct {
	Detail     string
	Obligation *semantic.AtomicObligation
}

// DiagnosticID separates an incomplete atomic layout (S1266) from other
// missing capability evidence (S1265).
// Rules: rules/tooling/diagnostics.md — § 24(4l); rules/concurrency/atomics.md — § 48(4).
func (e *AtomicCapabilityEvidenceError) DiagnosticID() string {
	if e.Obligation != nil && e.Obligation.LayoutIncomplete {
		return diagnostics.AtomicLayoutIncomplete
	}
	return diagnostics.AtomicCapabilityUnverified
}

// Help explains that the obligation is unproven, not a source error or a
// proven unsupported capability (atomics.md §§ 37A(12), 48(4)).
func (e *AtomicCapabilityEvidenceError) Help() string {
	if e.DiagnosticID() == diagnostics.AtomicLayoutIncomplete {
		return "The selected target does not resolve the atomic storage layout needed for this operation. This is missing target evidence, not a proven unsupported operation; the source stays valid."
	}
	return "The selected target has not proven this atomic operation for the required execution context. This is missing evidence, not a proven unsupported operation; executable emission waits for a verified capability fact."
}

// Error names the open obligation (rules/concurrency/atomics.md § 48(4)).
func (e *AtomicCapabilityEvidenceError) Error() string {
	return "insufficient atomic capability evidence for executable emission: " + e.Detail
}
