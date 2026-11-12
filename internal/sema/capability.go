package sema

import (
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// reportCapabilityUnavailable reports valid Sec that the current compiler
// implementation does not support yet. It is the one Sema producer of the
// generic implementation-capability boundary (S1239, family
// compiler-capability), so the boundary is never disguised as a source
// semantic error; boundaries with a dedicated identity use their own ID in
// the same family.
// Rules: rules/compiler/compiler_pipeline.md — § 32(2)–(4).
func (a *Analyzer) reportCapabilityUnavailable(token lexer.Token, help string, format string, args ...any) {
	a.addErrorAtTokenWithMetadata(token, diagnostics.CompilerCapabilityUnavailable, help, format, args...)
}

// IsImplementationCapability reports whether this diagnostic is an
// implementation-capability boundary rather than a source or target error.
func (e Error) IsImplementationCapability() bool {
	return diagnostics.IsImplementationCapability(e.ID)
}
