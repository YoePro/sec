package secmlir

import (
	"fmt"

	"sec/internal/ir/semantic"
)

// UnsupportedLoweringError marks valid Semantic IR that the current high-level
// Sec MLIR schema cannot represent. It is intentionally distinct from Semantic
// IR verification.
type UnsupportedLoweringError struct {
	Feature  string
	Function semantic.FunctionID
}

func (e *UnsupportedLoweringError) Error() string {
	if e.Function == "" {
		return fmt.Sprintf("Sec MLIR schema %d does not support %s", dialectSchemaVersion, e.Feature)
	}
	return fmt.Sprintf("Sec MLIR schema %d does not support %s in %s", dialectSchemaVersion, e.Feature, e.Function)
}
