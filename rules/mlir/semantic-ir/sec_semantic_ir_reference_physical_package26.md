# P26 Semantic IR Amendment — Physical Reference Plans

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-19
- **Last updated:** 2026-10-19
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/mlir/semantic-ir/sec_semantic_ir_reference_physical_package26.md` (installed 2026-10-20)
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P26`

---
## § 1. Preserve source reference facts

P15/Sema retains reference kind, Place root/projection, BorrowID/LifetimeID, StorageIdentity, InvalidationDomain, EpochDependency, provenance, access authority, spatial extent, initialization, address space and relocation/pin constraints. Do not re-run Sema in the builder.

## § 2. Resolved physical plan reference

Add or reuse `ResolvedReferencePhysicalPlanRef`/`ResolvedDescriptorMaterializationPlanRef` as read-only references to P24 selected layout/representation and guarantee-specific discharge requirements. Facts are immutable and plan-keyed.

## § 3. Guarantee obligations

Represent each required guarantee as an obligation with either `StaticProof`, `DynamicCheck`, or `TargetGuarantee` discharge. Missing obligation/discharge linkage is verifier failure. A generation proof is not an aliasing, bounds or lifetime proof.

## § 4. Storage invalidation events

Reuse `StorageEstablishDomainOp`, `StorageAdvanceEpochOp`, `StorageEndDomainOp`, `StorageReclaimOp` and canonical relocation events where implemented. Arena allocation leaves epoch unchanged; Reset advances; Release ends. No invented storage taxonomy.

## § 5. Reference checks and projection

Reuse P15/P16 reference creation, reborrow and derivation, P24 `CheckValidity`/`Project*`/`Discharge`, plus a minimal typed epoch/bounds check when lowering needs an explicit checked branch. Guards precede address derivation/dereference and retain exact source failure semantics.

## § 6. Descriptor materialization

P16 slices, P18 owning arrays, P23 strings and P19 Arenas consume P24 LayoutID/policy. Source and physical identities remain distinct. Descriptor moves/transports are not new owners or borrows. No unapproved source methods or universal descriptor layout.

## § 7. P25 ABI mapping

At an executable call boundary, attach exact CallABIPlanRef and preserve semantic parameter/result identity. Physical splitting consumes the immutable ABI classification without rewriting ownership or extending a borrow. If P25 is not executable, keep high-level callable IR or fail closed.

## § 8. Verifier and deterministic printer

Verify obligations, validation CFG, layout/plan IDs, provenance, source authority, storage epoch transitions and ABI correspondence. Print stable semantic identity and plan/proof references, never host addresses or unstable map iteration order.
