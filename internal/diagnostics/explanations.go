package diagnostics

// Explanation is the extended, registry-owned documentation of one diagnostic
// identity that `sec diagnostics <ID>` and the JSON catalog publish next to
// the Definition (rules/tooling/diagnostics.md § 13(6)). It is presentation
// only: it never changes identity, severity, mandatory status or emission.
// Fields are optional; a diagnostic without an entry has no extended text.
type Explanation struct {
	// Description is the one-sentence short description.
	Description string `json:"description,omitempty"`
	// Details is the extended explanation of the rule and its proof basis.
	Details string `json:"explanation,omitempty"`
	// Causes lists common source situations that produce the diagnostic.
	Causes []string `json:"common_causes,omitempty"`
	// Alternatives lists valid ways to resolve it.
	Alternatives []string `json:"valid_alternatives,omitempty"`
	// Example is a short Sec source example that triggers the diagnostic.
	Example string `json:"example,omitempty"`
	// Related names related registered diagnostic IDs.
	Related []string `json:"related,omitempty"`
	// Since names the language version the rule belongs to.
	Since string `json:"since,omitempty"`
}

// explanations holds the extended texts by diagnostic ID. Every key must be
// a registered ID and every Related entry must be registered; the registry
// conformance test enforces both.
var explanations = map[string]Explanation{
	UnreachableStatement: {
		Description: "A statement can never execute because control flow proven by the compiler cannot reach it.",
		Details: "Sec rejects code that no valid execution can reach (rules/tooling/diagnostics.md § 21). " +
			"S3001 is reported on the first unreachable statement only: statements after a return, break, continue, panic or other " +
			"terminating statement in the same block; the first statement of an if branch, while body or switch clause whose condition " +
			"is decided by a literal, a folded constant or the facts of dominating conditions on the same path; and code after a loop " +
			"that never exits. The proof covers only facts the compiler can establish soundly; anything it cannot prove stays reachable " +
			"and gets no diagnostic. A switch clause entered by fallthrough is reachable even when its own items are impossible.",
		Causes: []string{
			"a statement written after return, break, continue or panic in the same block",
			"a branch guarded by a condition that an earlier condition on the same path already decided",
			"a while true loop without a reachable break followed by more statements",
			"a switch clause whose values or ranges dominating facts exclude",
		},
		Alternatives: []string{
			"remove the unreachable statement",
			"move it before the terminating statement",
			"change the condition or control flow so the path can actually execute",
			"delete the branch whose condition is already decided by an enclosing check",
		},
		Example: "fn F() int {\n    return 1\n    let x := 2   // S3001\n}",
		Related: []string{UnreachableTryHandler, InactiveMatchArm},
		Since:   "Sec 0.1",
	},
}

// Explain returns the extended explanation of a registered diagnostic, if one
// exists. The returned value is a detached copy.
func Explain(id string) (Explanation, bool) {
	explanation, ok := explanations[id]
	if !ok {
		return Explanation{}, false
	}
	explanation.Causes = append([]string(nil), explanation.Causes...)
	explanation.Alternatives = append([]string(nil), explanation.Alternatives...)
	explanation.Related = append([]string(nil), explanation.Related...)
	return explanation, true
}

// ExplainedIDs lists the IDs that carry an extended explanation.
func ExplainedIDs() []string {
	ids := make([]string, 0, len(explanations))
	for id := range explanations {
		ids = append(ids, id)
	}
	return ids
}
