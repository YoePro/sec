# Analysis ownership and package history

This is an implementation-navigation map, maintained by
`governance.compiler-analysis` in [governance.yaml](governance.yaml). It records
existing owners; it neither defines language semantics nor duplicates their
checklists or completion claims.

The coordination authority is
[compiler_analysis.md](../rules/compiler/compiler_analysis.md) §§ 1–2, 6,
9–10, 14–15, 67 and 75. Detailed semantics belong to the specialized books
below. Current support and missing work belong to the linked fragment and
exact integration ID. An owner listed here can still be partial or planned.

## Specialized analysis owners

All ten books named by compiler_analysis.md § 1(3) are mapped here. The
fact families follow § 6; a consuming analysis does not acquire ownership
merely by depending on a fact.

| Rulebook | Canonical fact family | Existing implementation tracker |
| --- | --- | --- |
| [call_graph.md](../rules/analysis/call_graph.md) | Callable identity, reachability, resolved targets and call edges | [analysis.yaml](analysis.yaml): `analysis.call-graph` |
| [effect_analysis.md](../rules/analysis/effect_analysis.md) | Independent effect summaries and their transitive guarantees | Panic: [errors.yaml](errors.yaml), `analysis.panic`; allocation: [allocation.yaml](allocation.yaml), `analysis.allocation`; blocking: [concurrency_blocking.yaml](concurrency_blocking.yaml), `frontend.blocking-effects`. Call-target/SCC infrastructure: `analysis.call-graph` in [analysis.yaml](analysis.yaml). Attribute spelling and implication: `frontend.attributes-v2` in [attributes.yaml](attributes.yaml). These are distinct subscopes, not competing effect trackers. |
| [escape_analysis.md](../rules/analysis/escape_analysis.md) | Escape destinations, retention and causes | [analysis.yaml](analysis.yaml): `sema.escape-analysis` |
| [closure_analysis.md](../rules/analysis/closure_analysis.md) | Capture modes, environments, callable capabilities and target sets | [analysis.yaml](analysis.yaml): `sema.closure-analysis` |
| [parameter_usage_analysis.md](../rules/analysis/parameter_usage_analysis.md) | Parameter demand and recommendations derived from canonical semantic facts | [analysis.yaml](analysis.yaml): `sema.parameter-usage-analysis` |
| [pitfall_analysis.md](../rules/analysis/pitfall_analysis.md) | Advisory evidence and recommended corrections | [analysis.yaml](analysis.yaml): `sema.pitfall-analysis` |
| [stack_analysis.md](../rules/analysis/stack_analysis.md) | Stack demand, bounds, budgets and evidence | [analysis.yaml](analysis.yaml): `sema.stack-analysis` |
| [data_races.md](../rules/analysis/data_races.md) | Cross-execution conflict and data-race proof | [analysis.yaml](analysis.yaml): `sema.data-race-analysis` |
| [deadlock_analysis.md](../rules/analysis/deadlock_analysis.md) | Resource/wait dependencies and deadlock proof | [analysis.yaml](analysis.yaml): `sema.deadlock-analysis` |
| [isr_analysis.md](../rules/analysis/isr_analysis.md) | ISR validity consuming the required effect, stack, race and deadlock facts | [analysis.yaml](analysis.yaml): `sema.isr-analysis` |

The effect row intentionally links the existing independent effect verticals.
There is no new aggregate effect checklist here. In particular, an implemented
blocking or panic subset does not establish the complete ISR proof.

## Coordination, memory facts and consumers

| Responsibility | Existing owner |
| --- | --- |
| Scheduling, shared fixed-point driver and analysis execution provenance | [analysis.yaml](analysis.yaml): `analysis.compiler-analysis` |
| Interprocedural coordination and snapshot/incremental dependencies | [analysis.yaml](analysis.yaml): `analysis.interprocedural-coordination`, `analysis.incremental-coordination` |
| Local ownership and availability | [lifetime.yaml](lifetime.yaml): `frontend.ownership-v2` |
| Borrow compatibility and live ranges | [lifetime.yaml](lifetime.yaml): `frontend.borrowing` |
| Local lifetime/reference validity | [lifetime.yaml](lifetime.yaml): `frontend.lifetime-analysis` |
| Reference-return summaries and interprocedural lifetime dependencies | [analysis.yaml](analysis.yaml): `interprocedural.references`, `interprocedural.lifetime-analysis` |
| Post-analysis representation | [lowering.yaml](lowering.yaml): `frontend.semantic-ir-v2`, `semantic-ir.core-v2`, `semantic-ir.compiler-pipeline`; specialized `semantic-ir.*` entries remain in their domain fragments |
| Shared analysis presentation | [tooling.yaml](tooling.yaml): `tooling.compiler-analysis` |
| Command selection and reporting | [tooling.yaml](tooling.yaml): `tooling.cli`, under [cli.md](../rules/tooling/cli.md) and applied MD-017 |
| This navigation/authority reconciliation | [governance.yaml](governance.yaml): `governance.compiler-analysis` |

The implementation schedule is described in
[analysis-scheduling.md](../internal/sema/analysis-scheduling.md), with the
registered producer dependencies in
[analyzer_analysis_pipeline.go](../internal/sema/analyzer_analysis_pipeline.go).
The scheduler coordinates existing fact producers; it does not replace the
specialized analyses. In particular, a completed scheduled callback is not a
positive safety proof, and convergence may retain Unknown facts.

The shared fact/provenance API, complete semantic identities, explicit proof
state coordination, remaining duplicate fact derivation and CompilationPlan
summary identity stay tracked by `analysis.compiler-analysis`. Closing this
navigation task does not close those implementation requirements.

## P1–P14 are implementation history

Under compiler_analysis.md § 67 and § 75(10), the original package Markdown
and YAML files in [rules/mlir/packages/](../rules/mlir/packages/) retain their
sync commits, acceptance boundaries and historical completion claims. They
describe vertical milestones, not an alternative analysis architecture or a
set of current compiler passes.

The following index connects each historical package to the current ledger
without copying its checklist:

| Historical package | Scope at that milestone | Current status owner in [lowering.yaml](lowering.yaml) |
| --- | --- | --- |
| [P1](../rules/mlir/packages/sec-mlir-dialect_package1.md) | Dialect foundation | Foundation evidence remains in the original package record; current representation work is `frontend.semantic-ir-v2` and `semantic-ir.core-v2` |
| [P2](../rules/mlir/packages/sec-mlir-dialect_package2.md) | Semantic IR core foundation | `semantic-ir.core-v2`; historical acceptance remains in the original package record |
| [P3](../rules/mlir/packages/sec-mlir-dialect_package3.md) | Semantic IR executable core | `frontend.semantic-ir-v2`, `semantic-ir.core-v2`; historical acceptance remains in the original package record |
| [P4](../rules/mlir/packages/sec-mlir-dialect_package4.md) | First Semantic IR to Sec MLIR bridge | `lowering.sec-mlir-schema-2` |
| [P5](../rules/mlir/packages/sec-mlir-dialect_package5.md) | Trivial core lowering | `lowering.sec-mlir-trivial-core` |
| [P6](../rules/mlir/packages/sec-mlir-dialect_package6.md) | Scalar coverage and plan-aware resolution | `lowering.sec-mlir-scalar-layout` |
| [P7](../rules/mlir/packages/sec-mlir-dialect_package7.md) | Checked integer semantic operations | `lowering.sec-mlir-checked-integers` |
| [P8](../rules/mlir/packages/sec-mlir-dialect_package8.md) | Checked integer Arith lowering | `lowering.sec-mlir-checked-integer-arith` |
| [P9](../rules/mlir/packages/sec-mlir-dialect_package9.md) | Typed arithmetic failure flow | `lowering.sec-mlir-typed-arithmetic-failure` |
| [P10](../rules/mlir/packages/sec-mlir-dialect_package10.md) | Local try handlers and Result branching | `lowering.sec-mlir-local-try-handlers` |
| [P11](../rules/mlir/packages/sec-mlir-dialect_package11.md) | Enum and union semantic values | `lowering.sec-mlir-enum-union-values` |
| [P12](../rules/mlir/packages/sec-mlir-dialect_package12.md) | Enum/union/Result/Option match CFG | `lowering.sec-mlir-match-cfg` |
| [P13](../rules/mlir/packages/sec-mlir-dialect_package13.md) | Struct semantic values | `lowering.sec-mlir-package13` |
| [P14](../rules/mlir/packages/sec-mlir-dialect_package14.md) | Fixed-array semantic values | `lowering.sec-mlir-package14` |

When a current rule or correction extends an older package assumption, record
the remaining implementation in its existing domain owner. Preserve the older
acceptance boundary. Historical references to a legacy compilation path cannot
authorize another backend: current builds follow the single track defined by
[compiler_pipeline.md](../rules/compiler/compiler_pipeline.md) and MD-101.

Use the owning fragment's live checklist for progress. This map deliberately
contains no copied counts, statuses or remaining descriptions that could
diverge from that owner.
