package semantic

import (
	"fmt"
	"sec/internal/diagnostics"
)

type UnsupportedFeatureError struct {
	Feature  string
	Package  uint8
	Location Location
}

// ImplementationCapability publishes the selected package limitation without
// reclassifying it as source invalidity or inventing a missing source location.
// Rules: rules/compiler/compiler_pipeline.md — §32(1–5), §58(3).
func (e *UnsupportedFeatureError) ImplementationCapability() diagnostics.ImplementationCapability {
	pkg := e.Package
	if pkg == 0 {
		pkg = 3
	}
	return diagnostics.ImplementationCapability{Feature: e.Feature, Boundary: fmt.Sprintf("semantic IR package %d", pkg), File: e.Location.File, Line: e.Location.Line, Column: e.Location.Column}
}

func (e *UnsupportedFeatureError) Error() string {
	pkg := e.Package
	if pkg == 0 {
		pkg = 3
	}
	where := ""
	if e.Location.Line > 0 {
		where = fmt.Sprintf(" at %s:%d:%d", e.Location.File, e.Location.Line, e.Location.Column)
	}
	return fmt.Sprintf("semantic IR feature not implemented in package %d: %s%s", pkg, e.Feature, where)
}
