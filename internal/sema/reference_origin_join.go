package sema

import (
	"fmt"
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// ReferenceOriginAlternative is one possible referent's validity dependency.
// Origin identity and epoch remain distinct; alternatives are not concurrent
// references and never assert that all paths select the same storage.
// Rules: rules/memory/arena.md — §§4.2,4.5,36,44;
// rules/memory/lifetime_analysis.md — §11.1–11.3.
type ReferenceOriginAlternative struct {
	Name               string
	Token              lexer.Token
	Local, MatchScoped bool
	Storage            StorageOrigin
	Generation         int
	DisplayName        string
}

// referenceOriginAlternativesForExpression propagates a holder's joined
// dependencies through derived references, slices and field projections.
// Rules: rules/memory/arena.md — §§38,44;
// rules/memory/lifetime_analysis.md — §8.1, §11.2.
func (a *Analyzer) referenceOriginAlternativesForExpression(expr ast.Expression) []ReferenceOriginAlternative {
	root, ok := borrowRootName(expr)
	if !ok {
		return nil
	}
	symbol, ok := a.symbols[root]
	if !ok || !typeCarriesReferenceOrigin(symbol.Type) {
		return nil
	}
	return append([]ReferenceOriginAlternative(nil), symbol.Type.ReferenceOriginAlternatives...)
}

// referenceOriginAlternatives reads the complete origin set, including the
// legacy single-origin representation, without exposing its mutable slice.
// Rules: rules/memory/lifetime_analysis.md — §11.1–11.3.
func referenceOriginAlternatives(typ Type) []ReferenceOriginAlternative {
	if len(typ.ReferenceOriginAlternatives) != 0 {
		return append([]ReferenceOriginAlternative(nil), typ.ReferenceOriginAlternatives...)
	}
	return []ReferenceOriginAlternative{{Name: typ.ReferenceOriginName, Token: typ.ReferenceOriginToken,
		Local: typ.ReferenceOriginLocal, MatchScoped: typ.ReferenceOriginMatchScoped,
		Storage: typ.ReferenceOriginStorage, Generation: typ.ReferenceOriginGeneration,
		DisplayName: typ.ReferenceOriginDisplayName}}
}

// joinedReferenceOrigin preserves all continuing origins in deterministic
// order. The strongest scalar representative remains for legacy consumers;
// validity consumers must inspect the complete set, not this representative.
// Rules: rules/memory/lifetime_analysis.md — Control-flow joins;
// rules/memory/arena.md — §§36,38,44.
func joinedReferenceOrigin(types ...Type) Type {
	result := types[0]
	seen := map[ReferenceOriginAlternative]bool{}
	var origins []ReferenceOriginAlternative
	for _, typ := range types {
		if strongerReferenceOrigin(typ, result) {
			result = typ
		}
		for _, origin := range referenceOriginAlternatives(typ) {
			if !seen[origin] {
				seen[origin] = true
				origins = append(origins, origin)
			}
		}
	}
	sort.Slice(origins, func(i, j int) bool {
		left, right := origins[i], origins[j]
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.Generation != right.Generation {
			return left.Generation < right.Generation
		}
		if left.Storage != right.Storage {
			return left.Storage < right.Storage
		}
		if left.Token.File != right.Token.File {
			return left.Token.File < right.Token.File
		}
		if left.Token.Line != right.Token.Line {
			return left.Token.Line < right.Token.Line
		}
		if left.Token.Column != right.Token.Column {
			return left.Token.Column < right.Token.Column
		}
		if left.Local != right.Local {
			return !left.Local
		}
		if left.MatchScoped != right.MatchScoped {
			return !left.MatchScoped
		}
		if left.Token != right.Token {
			return fmt.Sprintf("%#v", left.Token) < fmt.Sprintf("%#v", right.Token)
		}
		return left.DisplayName < right.DisplayName
	})
	result.ReferenceOriginAlternatives = nil
	if len(origins) > 1 {
		result.ReferenceOriginAlternatives = origins
	}
	return result
}

// captureReferenceOrigins snapshots the reference-origin type of every
// reference-carrying binding at the end of a branch. Branch analysis restores
// the outer symbol table, so without this snapshot an origin a branch gives
// an outer reference holder (for example a holder rebound to Arena storage)
// would be lost at the join.
// Rules: rules/memory/lifetime_analysis.md — §11.1–11.2.
func captureReferenceOrigins(symbols map[string]Symbol) map[string]Type {
	origins := map[string]Type{}
	for name, symbol := range symbols {
		if typeCarriesReferenceOrigin(symbol.Type) {
			origins[name] = symbol.Type
		}
	}
	return origins
}

// mergeContinuingReferenceOrigins joins the reference origins of the outer
// bindings over every branch that reaches the join. A branch without a
// snapshot (a fallthrough or omitted branch) keeps the pre-branch origin.
// When branches disagree every possible origin and expected generation is
// retained, so no Arena domain disappears from later validity checks.
//
// Rules:
//   - rules/memory/arena.md — §§36(1–4), 44(1–2) (dependencies across Reset/Release)
//   - rules/corrections/applied/correction25-20260823.md, correction28-20260823.md — corrected branch/switch/loop generation-state propagation
//   - rules/control-flow/flowcontrol_if.md — § 27 "Sema and flow-analysis requirements"
//   - rules/memory/lifetime_analysis.md — control-flow joins
func (a *Analyzer) mergeContinuingReferenceOrigins(branches ...branchAnalysis) {
	for name, symbol := range a.symbols {
		if !typeCarriesReferenceOrigin(symbol.Type) {
			continue
		}
		var candidates []Type
		for _, branch := range branches {
			if !branch.continues {
				continue
			}
			candidate := symbol.Type
			if branch.referenceOrigins != nil {
				if origin, ok := branch.referenceOrigins[name]; ok {
					candidate = origin
				}
			}
			candidates = append(candidates, candidate)
		}
		if len(candidates) == 0 {
			continue
		}
		chosen := joinedReferenceOrigin(candidates...)
		if sameReferenceOriginType(chosen, symbol.Type) {
			continue
		}
		symbol.Type = referenceTypeWithOrigin(symbol.Type, chosen)
		a.symbols[name] = symbol
	}
}

// referenceOriginRank selects the conservative legacy representative; the
// full set separately owns Arena validity.
// Rules: rules/memory/lifetime_analysis.md — §11.2.
func referenceOriginRank(typ Type) int {
	switch {
	case typ.ReferenceOriginStorage == StorageOriginArena && typ.ReferenceOriginName != "":
		return 3
	case typ.ReferenceOriginLocal || typ.ReferenceOriginMatchScoped:
		return 2
	default:
		return 1
	}
}

// strongerReferenceOrigin retains legacy representative ordering and oldest epochs.
// Rules: rules/memory/lifetime_analysis.md — §11.2; rules/memory/arena.md — §38.
func strongerReferenceOrigin(candidate, current Type) bool {
	candidateRank, currentRank := referenceOriginRank(candidate), referenceOriginRank(current)
	if candidateRank != currentRank {
		return candidateRank > currentRank
	}
	return candidateRank == 3 && candidate.ReferenceOriginName == current.ReferenceOriginName &&
		candidate.ReferenceOriginGeneration < current.ReferenceOriginGeneration
}

// sameReferenceOriginType detects convergence of both representative and alternatives.
// Rules: rules/memory/lifetime_analysis.md — §11.2–11.3.
func sameReferenceOriginType(left, right Type) bool {
	leftOrigins, rightOrigins := referenceOriginAlternatives(left), referenceOriginAlternatives(right)
	if len(leftOrigins) != len(rightOrigins) {
		return false
	}
	for i := range leftOrigins {
		if leftOrigins[i] != rightOrigins[i] {
			return false
		}
	}
	return left.ReferenceOriginName == right.ReferenceOriginName && left.ReferenceOriginLocal == right.ReferenceOriginLocal &&
		left.ReferenceOriginMatchScoped == right.ReferenceOriginMatchScoped && left.ReferenceOriginStorage == right.ReferenceOriginStorage &&
		left.ReferenceOriginGeneration == right.ReferenceOriginGeneration && left.ReferenceOriginToken == right.ReferenceOriginToken
}

// mergeLoopExitReferenceOrigins joins outer reference-holder origins at a
// loop exit: the zero-iteration entry (the restored pre-loop symbols), the end
// of the body (bodySymbols), and every break edge of the loop.
// Rules: rules/memory/lifetime_analysis.md — §11.3(1–4).
func (a *Analyzer) mergeLoopExitReferenceOrigins(bodySymbols map[string]Symbol, frame loopBreakFrame) {
	branches := []branchAnalysis{{continues: true}, {continues: true, referenceOrigins: captureReferenceOrigins(bodySymbols)}}
	for _, edge := range frame.breaks {
		branches = append(branches, branchAnalysis{continues: true, referenceOrigins: edge.referenceOrigins})
	}
	a.mergeContinuingReferenceOrigins(branches...)
}

// loopHeaderReferenceOrigins joins outer reference-holder origins at a loop
// header over the entry, the end of a body that falls through, and every
// continue edge. Only bindings whose joined origin differs from the entry are
// returned; an empty result means the header origins are already a fixed
// point.
// Rules: rules/memory/lifetime_analysis.md — §11.3(1–4).
func loopHeaderReferenceOrigins(entry map[string]Symbol, body map[string]Symbol, bodyFallsThrough bool, frame loopBreakFrame) map[string]Type {
	snapshots := []map[string]Type{}
	if bodyFallsThrough {
		snapshots = append(snapshots, captureReferenceOrigins(body))
	}
	for _, edge := range frame.continues {
		snapshots = append(snapshots, edge.referenceOrigins)
	}
	changed := map[string]Type{}
	for name, symbol := range entry {
		if !typeCarriesReferenceOrigin(symbol.Type) {
			continue
		}
		chosen := symbol.Type
		for _, snapshot := range snapshots {
			if origin, ok := snapshot[name]; ok {
				chosen = joinedReferenceOrigin(chosen, origin)
			}
		}
		if !sameReferenceOriginType(chosen, symbol.Type) {
			changed[name] = chosen
		}
	}
	return changed
}

// referenceTypeWithOrigin copies all represented dependencies onto a derived type.
// Rules: rules/memory/arena.md — §§38,44; rules/memory/lifetime_analysis.md — §8.1.
func referenceTypeWithOrigin(target Type, source Type) Type {
	target.ArenaDomainID = source.ArenaDomainID
	target.ReferenceOriginAlternatives = append([]ReferenceOriginAlternative(nil), source.ReferenceOriginAlternatives...)
	target.ReferenceOriginName = source.ReferenceOriginName
	target.ReferenceOriginToken = source.ReferenceOriginToken
	target.ReferenceOriginLocal = source.ReferenceOriginLocal
	target.ReferenceOriginMatchScoped = source.ReferenceOriginMatchScoped
	target.ReferenceOriginStorage = source.ReferenceOriginStorage
	target.ReferenceOriginGeneration = source.ReferenceOriginGeneration
	target.ReferenceOriginDisplayName = source.ReferenceOriginDisplayName
	return target
}

// referenceTypeWithOriginFromExpression retains holder dependencies on a projection.
// Rules: rules/memory/arena.md — §§38,44; rules/memory/lifetime_analysis.md — §8.1.
func (a *Analyzer) referenceTypeWithOriginFromExpression(target Type, expr ast.Expression) Type {
	target.ReferenceOriginAlternatives = a.referenceOriginAlternativesForExpression(expr)
	originName, originToken, originLocal, originStorage, generation := a.referenceOriginForExpression(expr)
	target.ReferenceOriginName = originName
	target.ReferenceOriginToken = originToken
	target.ReferenceOriginLocal = originLocal
	target.ReferenceOriginStorage = originStorage
	target.ReferenceOriginGeneration = generation
	target.ReferenceOriginMatchScoped = a.referenceOriginMatchScopedForExpression(expr)
	return target
}

// referenceOriginMatchScopedForExpression identifies represented match-local borrows.
// Rules: rules/memory/lifetime_analysis.md — §8.1.
func (a *Analyzer) referenceOriginMatchScopedForExpression(expr ast.Expression) bool {
	root, ok := borrowRootName(expr)
	if !ok {
		return false
	}
	symbol, ok := a.symbols[root]
	return ok && typeCarriesReferenceOrigin(symbol.Type) && symbol.Type.ReferenceOriginMatchScoped
}

// referenceOriginForExpression reads the legacy representative for scalar consumers.
// Arena validity checks separately consume referenceOriginAlternatives.
// Rules: rules/memory/reference_model.md — Provenance; rules/memory/arena.md — §§38,44.
func (a *Analyzer) referenceOriginForExpression(expr ast.Expression) (string, lexer.Token, bool, StorageOrigin, int) {
	root, ok := borrowRootName(expr)
	if !ok {
		return "", lexer.Token{}, false, StorageOriginUnknown, 0
	}
	symbol, ok := a.symbols[root]
	if !ok {
		return root, expressionToken(expr), true, StorageOriginUnknown, 0
	}
	// rules/memory/reference_model.md: parameter roots use the symbolic origin
	// seeded by seedParameterReferenceOrigin. The binding is local, while the
	// referenced storage remains caller-owned.
	if origin, exists := a.localRefContainers[root]; exists && !origin.Unknown {
		name, token := origin.Name, origin.Token
		if name == "" {
			name, token = root, symbol.Token
		}
		return name, token, origin.Local, symbol.Storage, a.arenaGenerations[name]
	}
	if typeCarriesReferenceOrigin(symbol.Type) && symbol.Type.ReferenceOriginName != "" {
		return symbol.Type.ReferenceOriginName, symbol.Type.ReferenceOriginToken, symbol.Type.ReferenceOriginLocal, symbol.Type.ReferenceOriginStorage, symbol.Type.ReferenceOriginGeneration
	}
	return symbol.Name, symbol.Token, symbol.Local, symbol.Storage, a.arenaGenerations[symbol.Name]
}

// typeCarriesReferenceOrigin identifies represented reference and view holders.
// Rules: rules/memory/lifetime_analysis.md — §8.1; rules/collections/collections.md — Slices.
func typeCarriesReferenceOrigin(typ Type) bool {
	return typ.Kind == ReferenceType || typ.Kind == SliceType
}

// seedParameterReferenceOrigin binds caller-owned provenance and a canonical
// symbolic ArenaDomain before aliases or moves can rename a parameter holder.
// Rules: rules/memory/reference_model.md — Provenance;
// rules/memory/arena.md — §§4.2,44,68(3)-(4).
func (a *Analyzer) seedParameterReferenceOrigin(function Function, param FunctionParameter) {
	if !typeCarriesReferenceOrigin(param.Type) {
		return
	}
	if isArenaReferenceType(param.Type) {
		symbol := a.symbols[param.Name]
		symbol.Type.ArenaDomainID = a.newArenaDomainID(param.Token, "parameter")
		a.symbols[param.Name] = symbol
	}
	placeType := param.Type
	if param.Type.Kind == ReferenceType && param.Type.Element != nil {
		placeType = *param.Type.Element
	}
	place := Place{
		Root:        param.Name,
		RootToken:   param.Token,
		Type:        placeType,
		Mutable:     param.Type.ReferenceMutable || param.MutableRef,
		Addressable: true,
	}
	a.localRefContainers[param.Name] = localOriginWithPlaces(localReferenceOrigin{
		Name:    param.Name,
		Token:   param.Token,
		Mutable: param.Type.ReferenceMutable,
	}, []Place{place})
}
