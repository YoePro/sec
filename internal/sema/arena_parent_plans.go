package sema

import (
	"fmt"
	"sort"

	"sec/internal/ast"
)

// ArenaParentDependency freezes one possible parent dependency of a borrowed
// child. Alternatives retain separate origins/epochs; they never assert that
// multiple mutually exclusive parent allocations are simultaneously selected.
// The child borrows the parent's backing and ends this borrow on terminal
// Release/destruction, without reclaiming the parent allocation individually.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§49–50;
// rules/memory/arena.md — §51(1)–(7).
type ArenaParentDependency struct {
	Origin               ReferenceOriginAlternative
	InvalidationDomain   InvalidationDomainID
	EpochDependency      EpochDependencyID
	BackingRelation      BackingRelation
	ReclamationAuthority ReclamationAuthority
}

// arenaParentDependencies publishes only dependencies already resolved during
// successful body analysis. It reuses the same origin alternatives as the
// existing retained-borrow checker and P15 domain/epoch identities. It does not
// infer calls, select providers, invent storage identities or rerun liveness.
// Subsequent child operations inherit the creation snapshot by stable domain,
// so moving either owner and resetting the child cannot erase its parent loan.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§49–50,72–76;
// rules/memory/arena.md — §§4.2,51(2)–(7).
func (a *Analyzer) arenaParentDependencies(call *ast.CallExpression, operation ArenaOperationKind, constructor, domain string) []ArenaParentDependency {
	if operation != ArenaOperationCreate {
		for _, creation := range a.arenaOperationPlans {
			if creation.Operation == ArenaOperationCreate && creation.Domain == domain {
				return append([]ArenaParentDependency(nil), creation.Parents...)
			}
		}
		return nil
	}
	if constructor != "FromBuffer" || len(call.Arguments) != 1 {
		return nil
	}
	backing, known := a.expressionTypes[call.Arguments[0]]
	if !known {
		return nil
	}
	seen := map[ReferenceOriginAlternative]bool{}
	var parents []ArenaParentDependency
	for _, origin := range referenceOriginAlternatives(backing) {
		if origin.Storage != StorageOriginArena || origin.Name == "" || seen[origin] {
			continue
		}
		seen[origin] = true
		parents = append(parents, ArenaParentDependency{
			Origin:             origin,
			InvalidationDomain: a.placeIdentities.invalidationDomain("arena:" + origin.Name),
			EpochDependency:    a.placeIdentities.epochDependency(fmt.Sprintf("arena:%s@%d", origin.Name, origin.Generation)),
			BackingRelation:    BackingRelationBorrowed, ReclamationAuthority: ReclamationArena,
		})
	}
	sort.Slice(parents, func(i, j int) bool {
		left, right := parents[i].Origin, parents[j].Origin
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.Generation != right.Generation {
			return left.Generation < right.Generation
		}
		if left.Token.File != right.Token.File {
			return left.Token.File < right.Token.File
		}
		if left.Token.Line != right.Token.Line {
			return left.Token.Line < right.Token.Line
		}
		return left.Token.Column < right.Token.Column
	})
	return parents
}
