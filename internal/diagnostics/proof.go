package diagnostics

// ProofState retains the owner's validity result independently of severity.
// Empty means that the producer supplied no proof classification; transport
// must never infer one from an error's text, severity or numeric identifier.
// Rules: rules/compiler/compiler_analysis.md — §7(1–8), §58(3).
type ProofState string

const (
	ProofValid    ProofState = "Valid"
	ProofInvalid  ProofState = "Invalid"
	ProofUnproven ProofState = "Unproven"
)
