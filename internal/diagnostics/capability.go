package diagnostics

// ImplementationCapability describes a producer-confirmed limitation of a
// compiler boundary. It deliberately carries no language-validity proof state.
// Missing source metadata is left absent, never inferred from error text.
// Rules: rules/compiler/compiler_pipeline.md — §32(1–5), §97(1).
type ImplementationCapability struct {
	Feature, Boundary, Callable string
	File                        string
	Line, Column                int
}

// CapabilityFailure exposes typed implementation limits to pipeline consumers.
// Semantic, target, verification and toolchain failures do not implement it
// merely because their messages contain words such as "unsupported".
// Rules: rules/compiler/compiler_pipeline.md — §32(2–4).
type CapabilityFailure interface {
	error
	ImplementationCapability() ImplementationCapability
}
