package secmlir

import (
	"fmt"

	"sec/internal/ir/semantic"
)

// UnsupportedLoweringError marks valid Semantic IR that schema 2 cannot
// represent. It is intentionally distinct from Semantic IR verification.
type UnsupportedLoweringError struct {
	Feature  string
	Function semantic.FunctionID
}

func (e *UnsupportedLoweringError) Error() string {
	if e.Function == "" {
		return fmt.Sprintf("Sec MLIR schema 2 does not support %s", e.Feature)
	}
	return fmt.Sprintf("Sec MLIR schema 2 does not support %s in %s", e.Feature, e.Function)
}
