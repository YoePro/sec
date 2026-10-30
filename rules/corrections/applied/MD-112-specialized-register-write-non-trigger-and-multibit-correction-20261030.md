# MD-112 — Non-trigger values, runtime masks, and multibit specialized register writes

- **Decision:** MD-112 — `hardware-special-write-non-trigger-values`
- **Status:** **APPLIED 2026-10-30** — normatively integrated (registers.md §§ 11.3, 11.7; hardware-register-access.md §§ 4.4–4.9, 8.2, 9.13, 13.5, 16.4, 30.4, 31.3; semantic_ir.md § 21(8); sec_mlir_dialect.md v11.41.1b / lowering v11.31; P15-42 amendment). Sema implements per-bit action masks, all-neutral no-op requests and value-dependent runtime requests; LSP hover shows the mask. Semantic IR request/intent/commit with masks (with the MD-077 register.hw.* operations, which Semantic IR rejects today), the Sec MLIR #sec.register_write_action implementation and MD-111 physical plans remain items of governance/platform.yaml platform.hardware-register-access.
- **Created:** 2026-10-30
- **Last updated:** 2026-10-30
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Reference repository:** `YoePro/sec`, `main` at `aa59b4c18c91ed04eef69c7dd0028e41630067f5` (verified 2026-10-30)
- **Archived:** `rules/corrections/applied/MD-112-specialized-register-write-non-trigger-and-multibit-correction-20261030.md`
- **Primary normative owners:** `rules/declarations/registers.md` § 11.3; `rules/platform/hardware-register-access.md` §§ 4.4–4.9, 8–9, 13, 16, 30–31
- **Related existing decisions:** MD-077 §§ 5–11, 14, 19; MD-110 for detached register conversions; MD-111 for resolved endpoint facts/physical plans; MD-113 for shadow knowledge across calls
- **Implementation tracking:** `missing-decisions.yaml` MD-112; existing `governance/platform.yaml` integration `platform.hardware-register-access`; relevant existing Semantic IR, Sema, and lowering work
- **Decision authority:** Accepted by the Sec language owner on 2026-10-30 following review of C++26 saturating arithmetic and its separation from register write effects

> **Integration contract.** This document resolves MD-112 only. Integrate it as amendments to the existing canonical sections without renumbering paragraphs. Do not replace MD-077 operations, invent a new register-field API, change detached register semantics, or report physical lowering complete while MD-111 endpoint/plan facts remain unavailable. Update MD-112 tracking to decided/resolved when integrated, while retaining implementation tasks under the existing governance owner.

---

## § 1. Scope

**§ 1(1).** This decision applies to **hardware-bound** register-field writes using exactly the six existing specialized write behaviors:

```text
write-one-clear
write-one-set
write-one-toggle
write-zero-clear
write-zero-set
write-zero-toggle
```

**§ 1(2).** The six behaviors denote **logical per-bit actions**, not assignment of an ordinary stored value. For a field of width `N`, each bit is independently triggering or non-triggering under its declared behavior.

**§ 1(3).** Detached `register[N]` values retain ordinary, pure bit-value semantics, even when their declared fields carry hardware write modifiers. They do not execute clear/set/toggle actions when modified as software values (MD-077 § 5; `registers.md` § 11.7).

**§ 1(4).** This decision does not introduce saturating arithmetic, saturating conversion, silent clamping, wrapping, new source syntax, or register-read rules. Saturating numerical operations are a separate future language/library design topic.

## § 2. Trigger and neutral values

**§ 2(1).** The trigger and neutral bit values are normative:

| Declared behavior | Triggering bit | Neutral bit | Logical action on triggering bit |
|---|---:|---:|---|
| `write-one-clear` | `1` | `0` | Clear the corresponding bit |
| `write-one-set` | `1` | `0` | Set the corresponding bit |
| `write-one-toggle` | `1` | `0` | Toggle the corresponding bit |
| `write-zero-clear` | `0` | `1` | Clear the corresponding bit |
| `write-zero-set` | `0` | `1` | Set the corresponding bit |
| `write-zero-toggle` | `0` | `1` | Toggle the corresponding bit |

**§ 2(2).** A neutral bit requests **no logical action on that bit**. It is not an invalid source value and must not be reinterpreted as a request to store that bit in the hardware state.

**§ 2(3).** For a width-`N` field and a validated width-`N` supplied bit pattern `v`, the logical action mask is:

```text
write-one-* : action_mask = v
write-zero-*: action_mask = (~v) & ((1 << N) - 1)
```

The formula is conceptual and must not imply native-width overflow for large `N`. `action_mask` denotes a width-`N` logical bit vector, not an instruction to truncate an invalid source value. Existing field range/conversion rules apply **before** calculating the mask.

**§ 2(4).** Only selected bits in `action_mask` request the respective clear, set, or toggle action. Remaining field bits are semantically untouched. The compiler may not infer active-high/active-low signal meaning or invert device-level signal polarity from these modifiers.

## § 3. Constant, runtime, and all-neutral requests

**§ 3(1).** A compile-time-known neutral value is legal for a specialized hardware field. An ordinary field assignment whose complete action mask is zero represents a **logical no-op**. By itself it creates no pending specialized write intent and requires **no physical I/O**.

**§ 3(2).** An ordinary field assignment with a runtime-known-only value is legal at the source-semantic level when the value meets existing type and field-range requirements. The compiler must preserve the value-dependent action mask. It must not require a literal trigger value.

**§ 3(3).** For a runtime mask, the physical operation is conditional on a nonzero action mask unless the validated endpoint contract provides an alternative realization proven equivalent, including all observable effects. A runtime mask of zero must not silently cause an otherwise unrequested observable hardware write.

**§ 3(4).** A nonzero action mask remains a requested hardware effect even when software believes the target bits already have the requested final state. The compiler may not drop clear/set/toggle actions based on a stale observation, shadow value, or presumed current register bits. A toggle request, in particular, must retain its effect.

**§ 3(5).** Constant-zero mask elimination is a **source-semantic no-op determination**, not an optimization that removes a required volatile physical transaction. It does not authorize removal of explicit `Write(value)`, ordinary `read-write` field writes, or other operations whose hardware contracts make a physical transaction observable.

## § 4. Multibit example

**§ 4(1).** Consider a hardware-bound type:

```sec
type InterruptStatus register[8] {
    Pending: bit[4] write-one-clear,
    _: bit[4],
}

impl InterruptStatus {
    fn ClearSelected(ref mut self) void {
        self.Pending = 0b0101
    }

    fn ClearNone(ref mut self) void {
        self.Pending = 0b0000
    }
}
```

**§ 4(2).** `ClearSelected` requests clearing `Pending` bits 0 and 2 and leaves bits 1 and 3 logically untouched. `ClearNone` requests no clear action and causes no hardware transaction solely by virtue of this field assignment.

**§ 4(3).** A `bit[4] write-zero-clear` field given `0b1010` requests clearing bits 0 and 2, because those written positions contain zero. Other positions remain neutral. This example is a logical request; actual physical writes still require verified endpoint capabilities.

**§ 4(4).** Runtime values use the same per-bit semantics. There is no implicit conversion or truncation merely because a field is four bits wide; source values must satisfy the existing carrier, range, and explicit extraction rules.

## § 5. Physical transaction planning

**§ 5(1).** A logical action mask is **not** itself a physical bus transfer pattern. The existing register-operation planner must choose a semantics-preserving plan using canonical `HardwareResourceID`, `HardwareEndpointID`, footprints, aliases, access widths, reserved-bit requirements, ordering, completion, authority, and failure contract.

**§ 5(2).** If one physical transaction necessarily writes other positions, each non-requested position must receive a physically non-triggering value **when that value is documented and sufficient**, or be excluded from the write footprint through an explicitly supported mask, strobe, alias, or equivalent operation.

**§ 5(3).** The compiler must not assume that neutral bits make an entire bus transaction harmless. Ordinary `read-write` fields, command fields, reserved bits, and stronger endpoint/device effects may still respond to that physical transaction.

**§ 5(4).** A read-modify-write sequence is not automatically safe. Reads may be destructive or incoherent; writing back observed ones or zeros can trigger unrelated specialized actions. Such a sequence requires the same existing proof as any other register plan.

**§ 5(5).** If no valid physical plan realizes the requested action mask without forbidden collateral effects, compilation fails closed. `unsafe` does not grant permission to invent hardware capabilities or bypass a known contradictory endpoint contract.

**§ 5(6).** Explicit whole-register `Write(value)` retains its separately defined semantics. It is not reclassified as an all-neutral field request; a required whole-register physical write cannot be erased merely because some specialized field bits are neutral. The existing mixed-semantics plan check remains mandatory.

## § 6. Shadow state and operation identity

**§ 6(1).** For specialized fields, pending shadow state records the **logical action requested**, including the selected bit mask, not a stored image of the written source pattern.

**§ 6(2).** An all-neutral field assignment adds no pending specialized action. `Write()` with no pending intent remains a physical no-op under `hardware-register-access.md` § 8.2 and MD-077 § 8(3).

**§ 6(3).** Pending actions may be combined or removed only where the compiler proves that doing so preserves semantics, operation order, hardware effects, failure behavior, and external observability. Do not assume that clear/set/toggle requests can always be commuted, collapsed, or canceled merely because their mathematical masks appear compatible.

**§ 6(4).** Shadow observed knowledge (`Unknown`/`Known(value)`) remains separate from pending action intent and does not establish current hardware truth. MD-113 separately owns cross-call shadow knowledge and initialization questions.

## § 7. Semantic IR and MLIR requirements

**§ 7(1).** Sema resolves field behavior, carrier and range validity, per-bit trigger mask semantics, and the distinction between an all-neutral request and a potentially effectful hardware action.

**§ 7(2).** Existing MD-077 Semantic IR operations remain canonical: `register.hw.request_write`, `register.hw.shadow_intent`, `register.hw.commit`, and the appropriate verification/physical-plan facts. No second public register-operation API and no competing IR family are introduced by MD-112.

**§ 7(3).** For a runtime-valued request, Semantic IR must preserve its **conditional action-mask semantics** and the condition under which a physical effect is permitted. The lowering may use control flow or an equivalent verified target mechanism; the choice is not new source-language syntax.

**§ 7(4).** Semantic IR and Sec MLIR verification must not conflate a logical no-op request with a physical write, infer endpoint write masks from register width, or recalculate source-semantic behavior from raw integer patterns after Sema has resolved it.

**§ 7(5).** Physical lowering requires a verified plan under MD-111 and the existing hardware-access rulebooks. Until a suitable plan exists, valid source-level semantics do not imply that executable hardware code can be emitted.

## § 8. Diagnostics and regression requirements

**§ 8(1).** Retire the current blanket Sema rejection of non-trigger constants, runtime-valued specialized writes, and multibit specialized writes in hardware context. Retain existing type, range, permission, impl-access, and no-safe-plan rejections.

**§ 8(2).** Required semantic/IR tests include:

1. All six behaviors for 1-bit trigger and neutral constants, including legal all-neutral no-ops.
2. All six behaviors for width-`N` fields with mixed trigger/neutral bit patterns and correct masks.
3. Runtime zero/nonzero masks with value-dependent action formation, without speculative hardware effects.
4. Detached register field writes remain pure bit updates, with no hardware actions.
5. Shadow pending-intent representation and a commit with no pending intent.
6. Ordinary `read-write` field writes and explicit `Write(value)` are not incorrectly optimized as specialized neutral no-ops.

**§ 8(3).** Required hardware-plan and negative tests include:

1. Safe selective alias/mask/strobe operations where the endpoint provides a proven plan.
2. Required neutral fill bits for collateral positions where physical writes touch more than the requested field.
3. Rejection of unsafe RMW, undocumented selective-write capability, illegal access width or alignment, or collateral destructive/command effects.
4. Preservation of effect and order for repeated toggles and other non-idempotent actions.
5. Fail-closed lowering where canonical endpoint facts or a valid action-mask-dependent plan are missing.

**§ 8(4).** Diagnostics for a rejected plan should identify the requested field and action mask, the affected endpoint/physical footprint, the conflicting hardware effect or missing capability, and a concrete way to make the program valid when one is available. A neutral constant is not itself a semantic error.

## § 9. Controlled integration

**§ 9(1).** Amend `rules/declarations/registers.md` § 11.3 with the bitwise trigger/neutral definitions and §§ 2–4 of this correction. Preserve its existing numbering and detached register distinction.

**§ 9(2).** Amend `rules/platform/hardware-register-access.md` §§ 4.4–4.9 with the trigger/neutral contract; §§ 8–9 with the shadow/no-op separation; and §§ 13, 16, 30–31 with runtime-mask, plan, verifier and physical-effect requirements. Preserve all existing numbering and MD-077 citations.

**§ 9(3).** Update `missing-decisions.yaml` MD-112 from `open` to the repository's established resolved/decided status at the normative integration commit, retaining provenance. Do not mark MD-111 or MD-113 resolved through this change.

**§ 9(4).** Extend the existing `governance/platform.yaml` owner `platform.hardware-register-access` with narrowly identified implementation and test items; do not duplicate an integration ID or claim completion without tests. Integrate the change into the current Sema request resolver, Semantic IR and MLIR verifier/lowering paths using their established operations.

**§ 9(5).** Record the actual repository baseline/commit, run focused positive and negative compiler/IR tests, update LSP/formatter diagnostics only where affected, and archive this correction once all normative targets are synchronized. Implementation status must remain separate from the language decision.
