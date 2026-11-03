package main

import (
	"sec/internal/lexer"
	"sec/internal/sema"
)

// owner attributes source findings to their innermost containing named callable,
// using parser-owned endpoints rather than the preceding declaration. Closures
// and defers retain their enclosing named callable for report attribution.
// Concrete generic instances can share a source range: prefer a reachable
// identity, then canonical ID order, without expanding the graph's target set.
// Rules: rules/tooling/cli.md — §5.3; rules/analysis/call_graph.md — "Callable node";
// rules/corrections/applied/md017-sec-analyse-cli-and-help-correction-20261017.md — §§3,11(4–5).
func (s *analyseScope) owner(token lexer.Token) (sema.CallableID, bool) {
	var owner sema.CallableNode
	found := false
	for _, node := range s.byFile[token.File] {
		if node.Kind != sema.CallableBodyNamedFunction || !analyseCallableContains(node, token) {
			continue
		}
		later := sourcePositionBefore(owner.Declaration, node.Declaration)
		equal := owner.Declaration.Line == node.Declaration.Line && owner.Declaration.Column == node.Declaration.Column
		_, reachable := s.reachable[node.ID]
		_, previousReachable := s.reachable[owner.ID]
		if !found || later || equal && (reachable && !previousReachable || reachable == previousReachable && node.ID < owner.ID) {
			owner, found = node, true
		}
	}
	return owner.ID, found
}

// analyseCallableContains excludes module-level and following-declaration tokens.
// A node without a known endpoint can own only its exact declaration token; no
// body extent is manufactured from later declarations or source spelling.
// Rules: rules/tooling/cli.md — §5.3; rules/compiler/compiler_analysis.md — §55.
func analyseCallableContains(node sema.CallableNode, token lexer.Token) bool {
	if node.Declaration.File != token.File || sourcePositionBefore(token, node.Declaration) {
		return false
	}
	end := node.DeclarationEnd
	if end.Line <= 0 || end.File != token.File {
		end = node.Declaration
	}
	return !sourcePositionBefore(end, token)
}

// sourcePositionBefore compares canonical one-based token positions within the
// caller-validated source file. Same-line functions remain distinct.
// Rules: rules/tooling/cli.md — §5.3 (source attribution).
func sourcePositionBefore(left, right lexer.Token) bool {
	return left.Line < right.Line || left.Line == right.Line && left.Column < right.Column
}

// addFocusOpenTargetLimitations preserves open-world dependencies in focused
// output, including silent/default and JSON modes. It preserves the retained
// contract rather than treating an empty concrete target set as effect freedom.
// Rules: rules/tooling/cli.md — §§5.3(3),5.4(7),6(3);
// MD-017 correction — §11(5).
func (r *analyseReport) addFocusOpenTargetLimitations(graph *sema.CallGraph) {
	if r.scope == nil {
		return
	}
	for _, node := range graph.Nodes() {
		if _, reachable := r.scope.reachable[node.ID]; !reachable {
			continue
		}
		for _, site := range graph.Outgoing(node.ID) {
			if site.TargetSet.HasOpenContract {
				r.limitation("function focus: %s has an open callable target at %s; declared contract is retained, additional concrete callees and their effects remain unknown", r.names[node.ID], analysePosition(site.Source))
			}
		}
	}
}
