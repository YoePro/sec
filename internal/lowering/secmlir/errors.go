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
