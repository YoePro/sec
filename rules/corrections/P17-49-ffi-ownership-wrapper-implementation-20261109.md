# P17-49 — FFI ownership boundary

- **Date:** 2026-11-09
- **Revision:** 1.0
- **Status:** Approved implementation direction; verification pending
- **Sec version:** 0.1
- **Repository baseline:** `YoePro/sec` main `dce854ecb317eb62923c638c23a1baaac354be02`
- **Governance owner:** `lowering.sec-mlir-package17`

## § 1. Decision and scope

**§ 1(1).** P17-49 uses existing Sec ownership semantics. Foreign resource ownership is managed by **ordinary Sec wrappers**. No new FFI ownership attributes or direct foreign-transfer metadata are required by P17.

**§ 1(2).** An `extern` call or `RawPtr[T]` never implies pointee ownership transfer. A wrapper must explicitly implement the foreign API's success/failure transfer protocol. Richer direct-FFI ownership metadata remains **outside P17** (§ 134 of the package rulebook).

## § 2. Implementation and acceptance

**§ 2(1).** Preserve P17's ordinary move, consuming-call, availability and cleanup actions on Sec wrapper values. Commit call-site ownership transfers only after successful argument preparation and call entry. A wrapper must not relinquish a foreign resource before its acceptance is established.

**§ 2(2).** Test successful wrapper-mediated transfer, rejected/failed transfer, owner availability and exactly-once cleanup. Do not infer foreign release or retention from raw pointer values or ABI signatures.

**§ 2(3).** Update `package17-todo.md` P17-49 and its acceptance row after verification. Classify direct-FFI ownership metadata as out of scope, not as unfinished P17 work. Do not mark the entire P17 complete from this decision alone.

## § 3. Existing authority

- `rules/platform/ffi.md` §§ 35–38 — wrappers and foreign transfer contracts.
- `rules/memory/ownership.md` § 30; `rules/memory/copy_move.md` § 20 — explicit FFI ownership.
- `rules/memory/destruction.md` §§ 9.2, 22 — transfer and destruction.
- `rules/declarations/functions.md` §§ 9–10, 18 — consuming calls and commit point.
- `rules/mlir/packages/sec-mlir-dialect_package17.md` § 134 — deferred FFI scope.

**Integration:** Use existing governance ownership; no new MD or duplicate governance item.
