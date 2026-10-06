# Semantic analysis scheduling

This document describes implementation, not language semantics. The governing
rules are `rules/compiler/compiler_analysis.md` §§9–10 and §§14–15.

Declaration registration, type resolution, layout-cycle checking and interface
conformance establish the `declaration-facts` phase boundary in `Analyze`.
`runSemanticAnalysisPipeline` then declares producers and their dependencies.
`planAnalysisSchedule` validates the complete graph before executing callbacks.
Every dependency must have one registered producer or be an explicitly supplied
phase-boundary fact. Duplicate identities/dependencies and uncoordinated cycles
are compiler configuration errors, not source diagnostics.

Ready producers run by priority, then identity. Registration order and map
iteration order cannot change execution. Priorities preserve the existing
source-diagnostic presentation order among independent ready validators.
Consumers are released only after their prerequisites have completed.

The current scheduled producers are:

| Producer | Consumed facts |
| --- | --- |
| reference-summaries | declaration-facts |
| module-bodies | reference-summaries |
| callable-bodies | module-bodies, reference-summaries |
| string-materializations | callable-bodies |
| no-panic, no-alloc, no-block | callable-bodies, string-materializations |
| parameter-demand | callable-bodies and the three guarantee validators |
| parameter-advice | parameter-demand |
| pitfalls | callable-bodies, guarantee validators, parameter-demand, parameter-advice |

`callable-bodies` owns final local semantic traversal and publishes the existing
ownership, borrow, reference, escape, capture, execution and call-graph facts.
Domain-specific graph queries, effect propagation and loop-flow solvers remain
owned by their existing implementations inside this phase and its consumers.
The schedule coordinates producers; it does not duplicate those fact derivations.
The final Semantic IR/CompilationPlan readiness gate is a separate responsibility.

A mutually dependent domain is represented as one coordinated solver pass.
`runAnalysisFixedPoint` drives both reference-return inference and parameter
call-demand propagation. The domains retain their previous iteration limits,
join operations, canonical call-graph SCC ordering, and conservative widening.
Each round compares semantic facts or joins monotone demands. An unchanged round
reports convergence. Exhaustion invokes the domain's widening callback before
publishing any facts; it reports `Widened`, never `Converged`. Reference origins
become Unknown and caller parameter capabilities become Unknown, preserving the
existing rejection/advisory behavior. A producer that returns neither convergence
nor conservative publication cannot release its consumers.

`AnalysisSchedule()` returns detached per-run dependency and convergence records.
Analyze resets these records on every invocation. These are execution provenance:
a converged analysis may contain Unknown facts or source errors, and a completed
advisory pass may still have budget-limited coverage. The records do not certify
source validity, lowering readiness, or cross-snapshot cache compatibility.
