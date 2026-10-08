# Semantic analysis package boundaries

`sema` owns `Analyzer`, resolved types, source identities, provenance and the
shared semantic facts consumed by the compiler and LSP. Its files remain grouped
by responsibility while these components need package-private analyzer state.

`constant/` owns pure scalar constant operations: integer expression trees,
exact decimal arithmetic and representation, and target-width binary floating
arithmetic, range endpoints and exact unit-conversion proofs over binary
lattices. It imports AST definitions where needed and
never imports its parent `sema` package. Tests for these algorithms live beside
them; source-level contract/default and target tests remain in `sema`.

`unitexpr/` owns traversal of source unit factors, including groups and powers.
The parent interprets the names against the separate unit and generic type
namespaces and reports diagnostics; the child imports only the AST.

`collectionshape/` owns exact length proofs and intersections of resolved
length requirements. It depends only on arbitrary-precision arithmetic; the
parent retains named contracts and source diagnostics. Length-contract and
empty-list conformance tests live in the child's external test package and use
public Sema APIs, with Sec fixtures under `testdata/sema/collection_contracts/`.

`temporal/` owns pure classifications over resolved intrinsic temporal identity.
The parent owns clock availability, provenance, effects and compile-time context
outcomes. Temporal source conformance tests use public Analyzer APIs from the
child's external test package; Sec fixtures live in `testdata/sema/temporal/`.

`stackbound/` owns the immutable stack-resource proof values: stack bounds,
recursion-depth bounds and finite stack budgets with their explicit comparison.
It depends only on arbitrary-precision arithmetic. The parent composes bounds
over the call graph, owns summaries, evidence and contract stores, and reports
budget diagnostics; `stack_model.go` keeps the established Sema names
(`StackBound`, `NewExactStackBound`, `StackBudget`, ...) as aliases so CLI and
LSP consumers are unchanged.

New independent algorithms should follow this direction: the parent supplies
resolved inputs, the specialized package returns values or proof results, and
the parent supplies source diagnostics. Do not move Analyzer methods into
subdirectories without defining that boundary: Go treats each directory as a
separate package, and importing the parent from an imported child creates a
cycle. Physical layout remains in `internal/layout`, not a second Sema layout
implementation.

`interfaces/` owns source-level conformance tests for selected interface overload
facts, including inherited and named signatures, ownership and call-graph
contract parity. The tests consume public Sema APIs; Analyzer-owned requirement
resolution and fact publication stay in focused parent files to avoid a package
cycle. Sec fixtures live under `testdata/sema/interface_calls/`.
