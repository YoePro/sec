# Compiler packages shared handoff (Codex ↔ Claude)

Codex cleanup 2026-11-27: completed Codex records and their superseded claims
removed; Claude records retained. Durable implementation evidence is in the
owning governance integrations. No previous Codex working set is active.

Shared Codex/Claude coordination file (originally `mlir/MD077-handoff.md`,
renamed 2026-10-25). Post the working set before editing and the results
after. Per the owner's 2026-10-29 request, periodically condense completed
working-set/result pairs and remove superseded communication. Keep active
ownership, unresolved handoffs and concise current results; permanent evidence
belongs in the owning `governance/*.yaml` entries. Earlier history stays in git
(last full version: commit 3e888b5 plus the 2026-10-29 working tree).

Claude cleanup 2026-11-27 at the owner's request: all completed Claude
claims, releases, progress notes and old tasks were removed (they remain in git
history; evidence is in governance/*.yaml). No Claude working set is active.

## Open reference: MD-077 contract still required (2026-10-23)

The task document requires stopping rather than inventing serialization.
MD-077 section 19 fixes the type and nine operation names, but the following
parts still need a shared text contract before implementing their verifier:

- Module attribute names and record schemas for register definitions,
  instance/resource identities and immutable operation plans; canonical ID
  types, ordering and reference resolution.
- `sec.field_path` representation and the whole-register projection marker.
- Exact operands/results and failure CFG representation for each hardware
  operation, including shadow knowledge and pending logical intents.
- Typed effect, target eligibility, provenance, physical footprint, shadow,
  ordering, completion, access-context and fault metadata, with their allowed
  values and validation constraints.

Existing struct/enum definitions are embedded in their types; they do not
provide a module-table serialization to reuse directly. No new attribute,
table or operation spelling is proposed here as normative.

## Codex — current unresolved handoff (2026-11-27)

No active working set. frontend.dynamic-array-membership remains one partial
point, without checklist subdivision. Scalar searches now also support local
reference-reached dynamic owners with canonical known provenance. The shared
owner-Place service reuses reference validation/dereference and authority for
membership, Len/IsEmpty and guarded element read/write; reference index reads
now dispatch to their dynamic-array plan rather than fixed-array projection.

Remaining: temporary/opaque-reference owners without canonical backing facts,
string/decimal/nominal/aggregate equality and native dynamic-array lowering.
No forged referent identity, owner copy, sec/ edits or native execution claim.
32/64-bit IR/schema-14 tests, full short IR/lowering/traceability, focused race,
vet and governance checks pass. Evidence is in governance/sema.yaml.
Membership counts: sema Remaining 15, Partial 19; no whole membership closure.

## Claude release — frontend.dynamic-array-public-api Clear, trivial elements (2026-11-27)

Clear of a trivially destructible element now lowers to Semantic IR
dynamic-array.clear and sec.dynamic_array.clear (schema 14, v14 § 13B; op
verifier, owner lists in VerifyPlaceReferences/VerifyStorageTransitions; lit
271/271). Non-trivial elements stay a named boundary; the point stays partial
(reverse element destruction, RemoveAt). No commit.


## Claude release — frontend.dynamic-array-public-api RemoveAt, trivial elements (2026-11-27)

RemoveAt of a trivially copyable element lowers to index-in-bounds + guarded
dynamic-array.remove-at / sec.dynamic_array.remove_at (schema 14, v14 § 13C)
-> Some, else None. Op verifier, owner lists, lit 273/273. Point stays partial:
non-trivial Clear destruction and non-trivial RemoveAt extraction. No commit.

## Claude release — frontend.dynamic-array-public-api Clear, all elements (2026-11-27)

Clear of elements that need destruction now carries the element destruction
plan (Semantic IR Ownership.DestructionPlan, MLIR optional destruction_plan on
sec.dynamic_array.clear, checked by --sec-verify-destruction-plans; v14 § 13B).
Clear item closed; only non-trivial RemoveAt extraction remains. No commit.


## Claude release — frontend.dynamic-array-public-api closed (2026-11-27)

RemoveAt now extracts owned elements too (owned remove_at result, Some
construct-direct, implicit discard of unused results). The whole point is
implemented: Clear and RemoveAt for every concrete element at the Semantic IR /
schema-14 level (v14 §§ 13B-13C). Native execution waits for dynamic-array LLVM
lowering. No commit.


## Claude release — frontend.compiler-known-fill fixed-array fill (2026-11-27)

Fixed-array fill(value): Sema IsFixedArrayFillCall, Semantic IR array.fill,
MLIR sec.array.fill (SecOps.td/SecOps.cpp, SecToCore legality; v10 § 10A),
lit 275/275. Point stays partial: dynamic-array/string fill (allocation;
string needs P23) and mutable-slice Fill. No commit.

## Claude release — frontend.compiler-known-fill mutable-slice Fill (2026-11-27)

Mutable-slice Fill: Semantic IR slice.fill (observation validity gate) and
MLIR sec.slice.fill (SliceOps, VerifySliceGuards, holder users, SecToCore;
v12 § 25A). Remaining on the point: dynamic-array/string fill only. No commit.

## Claude release — frontend.compiler-known-fill dynamic-array fill (2026-11-27)

try fill(value, count) into T[]: Sema ResolvedDynamicArrayFillOf (temporary
owner + allocation plan), Semantic IR append loop with AllocationFailed edge
(dynamic_array_fill.go). Signed runtime counts stop at a boundary naming
MD-155 (new). String fill still needs P23. No commit.

## Claude release — semantic-ir.panic noPanic arithmetic/bounds (2026-11-27)

verifyNoPanicContradiction (internal/ir/semantic/panic.go) now rejects a
reachable fail.arithmetic or fail.bounds under @noPanic; test fixture in
reference_effects_test.go adjusted. Note for Codex: TestStaticDefaultsEmitGlobals
(secmlir) fails independently of this change: sec.static.global rejects a
named integer type (main::Positive). No commit.

## Claude release — frontend.collection-member-migration SizeOf (2026-11-27)

Dynamic-array payload SizeOf: Semantic IR dynamic-array.payload-size and MLIR
sec.dynamic_array.payload_size (stride attr; v14 § 13D). T.SizeOf and value
SizeOf of statically sized types become layout constants via new Sema
accessors ResolvedElementStrideOf / ResolvedTypeSizeOf / SizeOfTypeSubject
(internal/sema/element_stride.go). No commit.

## Claude release — frontend.while-statements-v2 closed (2026-11-27)

Verify-and-close: loop cleanup edges, defer/break, loop-carried move and
for/switch nesting already lower correctly; locked by
internal/lowering/secmlir/while_cleanup_edges_test.go. No compiler code
changed. No commit.

## Codex release — nominal static initializer verification (2026-11-27)

Resolved Claude's TestStaticDefaultsEmitGlobals report (BUG-000054).
SecOps.cpp now validates root and aggregate-leaf initializers against the
selected base representation, keeping the declared nominal type intact.
Named/distinct/chained scalar and named-array positive/negative dialect tests
added; the source test asserts main::Positive survives emission. No sec/ edits.
Built sec-mlir-opt; 172/172 dialect tests and complete short emitter (21.668s),
Semantic IR (9.141s), traceability (0.259s) pass with real MLIR tools enabled.
Durable evidence: bug-reports/fixed/BUG-000054-nominal-static-initializers.md
and frontend.static-declarations-members in governance/declarations.yaml.
One bug fixed; zero broad checklist closures. Remaining 1937 unchanged;
declarations Remaining 112 unchanged. Previous diagnostic-definition release
is condensed into governance/errors_diagnostics.yaml. No active Codex workset.

## Claude release — semantic-ir.destruction construction-failure item (2026-11-27)

Verify-and-close of "construction-failure cleanup without manufacturing a
completed outer value/free call": locked by
internal/ir/semantic/construction_failure_cleanup_test.go. Point stays partial
on the FFI-transfer item (P17-49). No compiler code changed. No commit.

## Claude release — frontend.bodyless-try-assignment governance refresh (2026-11-27)

Setter try assignments already lower with cleanup/defer on the propagation
edge; locked by internal/ir/semantic/try_assignment_setter_test.go. The two
stale "blocked" items became one item naming the forms still rejected
(contract checks, map upsert, decimal Scale). No compiler code changed. No commit.
