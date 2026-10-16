# P25 Normative Synchronization — ABI and FFI Boundaries

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec_callable_abi_sync_package25.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P25`

---

## § 1. Authority and no-new-language-rule principle

P25 is an **implementation synchronization** for existing normative `rules/platform/abi.md` and `rules/platform/ffi.md`. ABI owns physical transport; FFI owns legality, trust and foreign source declarations; P24 layout owns representation. Do not renumber or replace their existing paragraphs. Source facts are cited in the package, not silently revised.

## § 2. Calling conventions and ordinary extern

Sec recognizes `Sec`, `C`, `system` families. Per current FFI §3, ordinary `extern` is a compiler-unverified foreign trust boundary but **does not automatically require an unsafe caller context**. `unsafe extern` requires one. General export of Sec functions as named C symbols is outside Sec 0.1; `c::callback` private thunks remain supported.

## § 3. Lowercase C prefix and binding environment

`c::` is canonical, not `C::`. Fundamental C ABI types are compiler-known. Target binding environment selection/serialization remains MD-021; P25 must consume existing valid resolved environments, not invent metadata or use host fallback.

## § 4. Types and legality

Position-sensitive foreign legality precedes ABI classification. Native Sec descriptor values are not automatically C-compatible. An opaque C struct is not passed by value. C struct, union, enum, bitfield and flexible arrays use the selected C ABI. `c::fn` is distinct from native `fn`. Call-bounded foreign refs require non-retention, are argument-only; foreign raw return may not become safe ref.

## § 5. Foreign varargs and callbacks

Final C varargs are `unsafe extern "C"` and require already-C-typed arguments before target default promotion; native spread/typed variadics are distinct. `c::callback` permits environment-free, non-capturing reusable functions of exact C-facing signature, creating only a private C ABI thunk, not a general export.

## § 6. Unwind and ownership

No unwind/nonlocal jump crosses FFI in either direction. Raw pointers do not grant ownership. ABI bit transport never creates semantic Sec copy, move, borrow or owner. Foreign error conversion belongs to explicit wrapper code, not ABI lowering.

## § 7. Whole-call planning

The selected `ABIModel` from concrete `CompilationPlan` classifies return first, then formal arguments, C varargs, hidden args and stack assignment. Canonical `CallABIPlan`, `ABISignature` and `ABIFingerprint` must be deterministic and checked across callers/callees and persisted artifacts.

## § 8. Open decisions and sync policy

Public `missing-decisions.yaml` updated 2026-10-15 may lag user's local edits. Reuse MD-019 (explicit layout syntax), MD-021 (binding producer), MD-045 (external callable/stack contract metadata), MD-052 (privileged low-level bodies where relevant). Do not recreate applied MD-020/MD-023/MD-024 as new P25-D#. No newly discovered ABI core normative decision is asserted.

## § 9. Governance and implementation boundaries

Find existing ABI/FFI/platform/MLIR implementation IDs before writing governance. Update under existing IDs with observed tests and partial states; no invented complete flag or duplicate fragment. Source FFI semantics cannot be declared implemented merely because ABI call materialization is added.
