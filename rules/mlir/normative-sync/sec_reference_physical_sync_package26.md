# P26 Normative Synchronization — Physical References and Descriptors

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-19
- **Last updated:** 2026-10-19
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/mlir/normative-sync/sec_reference_physical_sync_package26.md` (installed 2026-10-20)
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P26`

---
## § 1. Normative source owners

This amendment synchronizes with `rules/memory/reference_model.md` (safe-reference guarantees, selected representation and generations), `rules/memory/storage.md` (storage identity, invalidation and relocation), `rules/memory/layout.md` (P24 canonical ResolvedLayout), `rules/platform/abi.md` (P25 CallABIPlan), `rules/memory/raw_pointers.md` and current Sema/ownership/borrowing/lifetime rulebooks. No source syntax or new nominal type is introduced.

## § 2. No universal fat pointer

A selected physical reference may be address-only or include length/epoch/capability/slot metadata as required. The representation contract is plan-keyed and must preserve all source guarantees. No default to `(ptr,len,epoch)` for every reference. No raw-pointer reinterpret of safe refs.

## § 3. Logical epochs and checks

Default logical epoch width 64 bits on 32-bit and 64-bit general-purpose targets. Physical checks must not silently truncate, tear or wrap. Dynamic stale ordinary-reference failure uses `panic.invalid-reference-generation` / `InvalidReferenceGeneration`; not Result, not UB. A chosen compact generation needs a proven retirement/exhaustion strategy.

## § 4. Guarantee-specific proof discharge

Static, runtime and target guarantees are distinct. Every erased runtime guard must be justified by a proof covering its exact validity requirement, path, storage incarnation, target and potential invalidations. Layout, borrow validity, bounds, provenance and synchronization cannot be inferred from only one successful epoch check.

## § 5. Descriptor families

P16 slice, P18 `T[]`, P23 string, P19 Arena and P15 reference descriptors materialize only through selected P24 policies. View descriptors do not own backing; owning descriptors remain single P17 values. `.Len` vs `.SizeOf` and internal capacity remain source-rule owned. Do not guess C compatibility or physical descriptor shape.

## § 6. Foreign boundary and MD-070

No publicly callable `RawPtr` ↔ safe reference conversion form is added while MD-070 remains open. (Note 2026-10-22: MD-070 was applied on 2026-10-21. `RawPtr[T].AsRef()` / `AsRefMut()` are the source spellings (`raw_pointers.md` § 18A), and they lower only once the distinct Semantic IR raw-to-safe operation of § 32(7) exists.) Existing P25-approved call-bounded internal foreign reference adapter may lower with retained proof. Foreign raw return cannot be promoted automatically into a safe reference.

## § 7. Concurrency and hardening

Concurrency validity requires a selected synchronization/reclamation protocol in addition to generation equality. Profiles may retain extra hardening checks, while statically proven short-lived refs can omit runtime metadata. Never impose global handle/generation management on profiles that do not need it.

## § 8. Preserve applied decisions

Honor MD-061 ref-holder reborrow semantics and already-applied October reference/string/array decisions. Do not reopen MD-050/MD-071/MD-072 as P26 questions; existing MD-019/MD-070/MD-073/MD-109 remain explicit gates where relevant.

## § 9. Sync application

Merge these paragraphs additively with existing stable section numbering. Implementer must re-evaluate locally uncommitted `missing-decisions.yaml` and corrected P15–P25 packages. Never overwrite local approved corrections with this draft.
