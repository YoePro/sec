# MD-113 — Shadow knowledge across calls, field selectors, and compiler-known inspection

- **Decision:** MD-113 — `hardware-shadow-knowledge-across-calls`
- **Status:** **APPLIED 2026-10-30** — normatively integrated (hardware-register-access.md §§ 9.8, 9.13, 9.14, 30.4; registers.md § 11.7; impl.md § 23; compiler_known_members.md; semantic_ir.md § 21(9); modules.md § 2.4; lsp.md; dialect v11.41.1c / lowering v11.32); Sema implements the registry, IsShadowKnown selector and reservation, the static shadow-knowledge analysis with member summaries, and LSP contract hover. Semantic IR/Sec MLIR (with the MD-077 register.hw.* operations, which Semantic IR rejects today) and ModuleSurface publication for separate compilation remain ordinary governance items.
- **Created:** 2026-10-30
- **Last updated:** 2026-10-30
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Reference repository:** `YoePro/sec`, `main` at `aa59b4c18c91ed04eef69c7dd0028e41630067f5` (verified 2026-10-30)
- **Primary normative owners:** `rules/platform/hardware-register-access.md` §§ 7–10, 13, 27, 30–33; `rules/declarations/registers.md` §§ 11.5–11.7, 13–14
- **Related owners:** `rules/compiler/compiler_known_members.md`; `rules/declarations/impl.md` §§ 8, 22–23; `rules/compiler/semantic_ir.md` § 21 and applicable dataflow/call contracts; `rules/mlir/sec_mlir_dialect.md`; `rules/mlir/sec_mlir_lowering.md`; `rules/projects/modules.md`; `rules/tooling/lsp.md`
- **Tracking:** `missing-decisions.yaml` MD-113; existing `governance/platform.yaml` `platform.hardware-register-access`; additional implementation owners only if needed after reconciliation
- **Dependencies:** MD-077 (hardware instances, impl exposure, shadow/IR), MD-112 (non-trigger and multibit actions; approved separately), MD-111 (endpoint facts and physical access plans)
- **Decision authority:** Approved by the Sec language owner, 2026-10-30, including the review choices resolved in § 14.

> **Integration contract.** The source and semantic decisions in this file are approved, including `Control.Mode` field-selector spelling, status-only inspection of write-only shadow fields, and conditional knowledge summaries. This decision does **not** claim that the compiler, canonical rulebooks, governance, or MLIR implementation have been synchronized. Integrate with additive amendments to existing stable sections; do not renumber established paragraphs or create duplicate governance entries. Maintain MD-113 as implementation-pending until the existing tracking policy permits its removal.

---

## § 1. Scope and already-agreed constraints

**§ 1(1).** MD-113 defines when software can read a shadowed register field, how the compiler proves its availability across method calls and modules, how a program asks whether a shadow value is available at runtime, and how the required source/member identities are represented and documented.

**§ 1(2). Normative constraints (approved):**

1. Shadow knowledge is **not** present hardware state. Hardware may change independently.
2. Register hardware is touched **only** to realize a programmer-requested, contract-valid logical hardware operation. The compiler never reads merely to make shadow knowledge available.
3. Shadow availability is analyzed **statically and interprocedurally**; no implicit runtime fault, panic, automatic refresh, or hidden hardware read is introduced for an unknown shadow read.
4. A program may query whether a software shadow value is known **without** physical I/O.
5. Such a query is available through a tightly scoped compiler-known operation that the owning register `impl` may wrap in ordinary public methods.
6. `IsShadowKnown` is the agreed name for the new compiler-known operation. This name, along with existing `Read` and `Write`, must be reserved and non-overloadable in register `impl` surfaces.
7. A pending ordinary write may provide a readable **intended** shadow value before commit. A pending specialized clear/set/toggle action, by itself, never fabricates an observed or intended readable value.
8. When a readable observed value is already known, pending specialized actions do not silently replace it with a predicted post-commit value.
9. No dedicated public `ShadowValue[T]` or `ObservedValue[T]` wrapper type is introduced.

**§ 1(3). Approval scope:** The field-selector grammar, operand/availability restrictions, status-only write-only queries, conditional interprocedural summaries, and normative diagnostic/verification obligations specified below are approved together with the principles in § 1(2). Concrete compiler representation choices left explicitly implementation-defined in §§ 9–10 remain implementation decisions, not gaps in the approved source contract.

**§ 1(4). Non-goals:** No general reflection, runtime hardware introspection, automatic MMIO reads, new public field access outside `impl`, new shadow manager, or change to detached `register[N]` value semantics. MD-113 does not decide MD-111 physical endpoint construction or MD-112 action-mask meaning.

---

## § 2. Four distinct concepts, not one cached register

**§ 2(1). Hardware state** means the device's physical state at a particular instant. Unless a target contract establishes stronger guarantees, Sec does **not** claim it remains equal to the last observed or requested value.

**§ 2(2). Observed knowledge** is software information established by a permitted, coherent hardware observation. Its state for a relevant field or field subpart is `Unknown` or `Known(value)`. It describes the value observed **then**, not necessarily the value in hardware **now**.

**§ 2(3). Intended ordinary value** is a value that software has selected for an ordinary writable shadow field but has not necessarily committed to hardware. A successful software assignment may make the shadow field readable as that intended value. This does not imply that a physical write has occurred.

**§ 2(4). Pending specialized action** is a requested effect such as W1C/W0C/W1S/W0S/W1T/W0T with a per-bit action mask (MD-112). It is **not** an ordinary new field value and cannot be substituted for one.

**§ 2(5). Readable shadow view.** For ordinary writable fields, the most recent valid intended value is selected before a previous observed value, provided the write has established a well-typed intended value. Otherwise the previously observed value can remain readable. For specialized action fields, a pending action never overrides the last valid observed shadow value merely because it appears to imply a final value. A field without either qualifying source remains unreadable/unknown.

**§ 2(6). Query meaning.** `IsShadowKnown(path)` means precisely **software possesses a complete defined shadow value for the selected field at this point**, from a valid observation or ordinary intended assignment. This is an availability/knowledge question, independent of source read permission. It does *not* mean that the caller is permitted to read a `write-only` field, that a physical register read is safe, that a write succeeded, that the device is up to date, or that the device's physical bits are known.

**§ 2(7). Access permissions remain separate.** Read permissions still govern whether a program may obtain the field value. A shadow-status query does not grant permission to read a `write-only` field or bypass a register implementation boundary. Status-only queries on `write-only shadow` fields are permitted under § 6(7); ordinary field reads remain forbidden.

---

## § 3. Compiler-known source surface

**§ 3(1). Existing reserved hardware operations (unchanged):**

| Source form inside owning register `impl T` | Semantic operation | Result | Hardware effect |
|---|---|---|---|
| `self.Read()` | One explicit logical hardware observation | Detached `T` | Required observation; may have documented read side effects |
| `self.Write()` | Commit pending shadow logical intents | `void` | Only if intents require a physical transaction; no pending intent means no I/O |
| `self.Write(value)` | Explicit whole-register write of detached `T` | `void` | Explicit write requiring a verified physical plan |

The current compiler implementation resolves both `Write` forms to `void` and `Read` to detached `T`. Target faults and explicit checked-access APIs remain governed by the existing hardware rulebooks; MD-113 does **not** introduce an implicit `Result` return or new fault handler.

**§ 3(2). One new source-visible compiler-known operation:**

| Source form | Result | Physical I/O | Allocations | Side effects |
|---|---|---|---|---|
| `self.IsShadowKnown(Control.Mode)` | `bool` | None | None | Reads software shadow-availability metadata only; does not alter observed values, pending intents, or hardware |

**§ 3(3).** The call above is **compiler-owned syntax with one specially interpreted field-selector operand**. `Control.Mode` is not evaluated as an expression and is not a value of any public Sec type. No fictional `FieldPath` parameter type or user-callable function declaration is implied by this table. A normative integration must document the compiler-known operation in the registry's category and semantic schema rather than pretending a normal Sec declaration can express its selector operand.

**§ 3(4).** Calls are legal **only inside the owning `impl Control` or its same-module `impl extends Control` fragments**, with a receiver that resolves to `HardwareInstance[Control]` for hardware execution. External callers obtain the capability only through normal public members of the owning `impl`; they may not call `device.IsShadowKnown(...)` directly.

**§ 3(5).** A call on a detached `Control` register value is invalid as a hardware/compiler-known shadow operation. Detached registers already have pure ordinary value semantics and no hardware shadow-state contract. This is consistent with the receiver-dependent source behavior in `registers.md` § 11.7.

**§ 3(6).** Implicit receiver spelling, if admitted by the canonical existing register-op resolver, is equivalent to `self.IsShadowKnown(Control.Mode)`; the recommended documentation form always spells `self` explicitly. No new implicit receiver rule is added in MD-113.

**§ 3(7).** `IsShadowKnown` does not itself fetch or return the field value. Code reads the field separately under the ordinary shadow-read and permission rules. No new compiler-known `GetShadow`, `ReadShadow`, `RefreshShadow`, `ObserveNow`, or `GetHardwareState` member is introduced.

---

## § 4. Field-selector grammar and resolution

**§ 4(1). Context-specific input form:**

```ebnf
ShadowKnownCall       ::= HardwareReceiver "." "IsShadowKnown" "(" RegisterFieldSelector ")"
RegisterFieldSelector ::= RegisterTypeName "." FieldName ("." FieldName)*
```

This is a **semantic specialization at compiler-known call resolution**, not a new general expression grammar, token, or runtime field-path object. `RegisterTypeName` is an actual resolved nominal register type. The first selected field is declared directly in that type; every intermediate path component must be a nested-register field; the final component must be a declared, named field eligible for `shadow` inspection.

**§ 4(2). Simple path:**

```sec
type Control register[8] {
    Mode: bit[3] shadow,
    _: bit[5],
}

impl Control {
    fn HasKnownMode() bool {
        return self.IsShadowKnown(Control.Mode)
    }
}
```

The expression `Control.Mode` in this one argument position identifies the declared register field; it does not read either a detached value or a hardware register.

**§ 4(3). Nested path:**

```sec
type InterruptBits register[4] {
    Pending: bit[4] write-one-clear shadow,
}

type Control register[8] {
    IRQ: InterruptBits,
    _: bit[4],
}

impl Control {
    fn HasObservedPending() bool {
        return self.IsShadowKnown(Control.IRQ.Pending)
    }
}
```

The full path resolves against the register definitions and their canonical field identities; the nested path must not be guessed by width, offset, field spelling alone, or an inferred backing integer.

**§ 4(4). Root typing.** The resolved register-type identity must match the type `T` of the receiving `HardwareInstance[T]`. A selector rooted in some other nominal register type is invalid, even if both types have identical width and field spellings. The selector cannot be rooted directly in the nested type (`InterruptBits.Pending`) when the receiver is `HardwareInstance[Control]` and the path is `Control.IRQ.Pending`.

**§ 4(5). Name resolution.** Resolve the root as a **type**, not a local variable, property, instance field, value, or module at runtime. A local variable named `Control` does not turn the selector into a runtime expression. If canonical Sec type lookup cannot uniquely identify the owning type, report ambiguity instead of evaluating an expression. The exact spelling of module-qualified types follows `rules/projects/modules.md`; do not introduce `::` or arbitrary dotted module paths as ad hoc exceptions.

**§ 4(6). Selector restrictions.** The following are compile-time errors for `IsShadowKnown`:

```sec
self.IsShadowKnown(Mode)                  // Missing nominal root.
self.IsShadowKnown(self.Mode)             // Value/instance member access is not a selector.
self.IsShadowKnown(other.Mode)            // Value/instance member access is not a selector.
self.IsShadowKnown(Control.Unknown)       // No such declared field.
self.IsShadowKnown(Control._)             // Reserved, unnamed field.
self.IsShadowKnown(Control.Mode + 1)      // Runtime expression is not a selector.
```

**§ 4(7).** A selector cannot be stored, passed to an ordinary function, returned, obtained by dereferencing a pointer, or constructed dynamically. It has no runtime representation. Its compiler identity includes the canonical `RegisterTypeID` and the complete resolved nested `RegisterFieldID` sequence.

**§ 4(8).** The formatter must preserve the explicit root and complete path. LSP completion, hover, and go-to-definition should use the same Sema resolution, including a link to the final field and information about the entire path. No second LSP-only parser is permitted.

**§ 4(9). Approved spelling.** The explicit register-type-qualified selector is canonical for `IsShadowKnown` in Sec 0.1. Alternative spellings are not admitted by MD-113. Any future extension requires an explicit compatible language decision; the current operation always resolves a static field identity without evaluating a field access.

---

## § 5. Initial shadow state, availability, and state transitions

**§ 5(1). Initial state.** A newly bound hardware instance starts with `Unknown` software shadow knowledge for every shadowed field, unless an existing, separately specified and compiler-proven binding/init contract explicitly establishes valid software knowledge. Binding alone performs no hardware read and fabricates no zero, reset value, or hardware state.

**§ 5(2). Successful legal observation.** A physical observation requested by ordinary source semantics can establish `Known(observed_value)` only for fields and portions actually established by the coherent observation. Destructive read effects remain observable and must be preserved; `Known` remains historical knowledge.

**§ 5(3). Ordinary shadow assignment.** A legal assignment to an ordinary writable shadow field may establish `Known(intended_value)` before commit, even if no previous observation existed. The pending logical write intent is tracked separately; no hardware write occurs unless and until a defined operation requires it.

**§ 5(4). Specialized shadow assignment.** W1C/W0C/W1S/W0S/W1T/W0T create pending *actions* per MD-112. If previously `Unknown`, shadow availability remains `Unknown` solely from the action. If previously readable `Known(observed_value)`, that view remains readable as the last established view; the pending action must not be assumed committed or used to fabricate a predicted new value.

**§ 5(5). Commit.** A permitted `Write()` commits pending intents through a verified hardware plan. Following success or recoverable partial failure, the compiler updates, retains, or invalidates shadow knowledge **only** according to actual guaranteed postconditions and the applicable failure contract. An external device capable of changing bits independently does not grant the compiler a stronger current-state assertion.

**§ 5(6). Unknown read.** A direct shadow field read with no statically proven available value is invalid at the source-analysis boundary unless guarded by a proven condition as described in §§ 7–8. It must never silently issue a hardware read, produce an uninitialized value, turn into an implicit `Option`/`Result`, or panic at runtime.

**§ 5(7). Fine-grained knowledge.** The compiler may track availability at field or bit/subfield granularity internally. `IsShadowKnown(Control.Mode)` is `true` only if the **complete logical value** of `Mode` is present in software shadow knowledge or a qualifying intended value; the permission to read it is checked separately. Partial knowledge insufficient for the complete field returns `false`. MD-113 introduces no partial-value API.

---

## § 6. Availability rules and permissions

**§ 6(1).** A valid selector requires an effective `shadow` policy on the final selected field. Selecting a `noshadow` field is a compile-time error, not a false status result: `noshadow` is a different access model, not a shadow field in `Unknown` state.

**§ 6(2).** `IsShadowKnown` remains a software metadata question even if the field has `read-clear`, `read-set`, or a special write behavior. Querying status does **not** perform the field's hardware operation.

**§ 6(3).** Selecting a nested field validates each nesting step and resolves to its final leaf or complete declared field identity. Register-wide `shadow` defaults and field-level `noshadow` overrides are applied as defined by existing register rules.

**§ 6(4).** The field selector does not bypass the type-owned implementation's public exposure policy. Code outside the owning `impl` cannot access the compiler-known status query directly.

**§ 6(5).** No implicit conversion from an integer or string to a field selector is allowed. The selector is not a first-class value and has no generic field-type parameter.

**§ 6(6).** `IsShadowKnown` requires a valid software shadow instance and does not grant any hardware authority that the receiver lacks. It is legal solely as the already constrained hardware-receiver compiler-known operation, not as an unguarded method on arbitrary pointers.

**§ 6(7). Write-only shadow fields.** **Status-only** inspection of a `write-only shadow` field is permitted. The boolean reveals only whether software holds a complete known software shadow value, **not** the value itself or any hardware state. `self.WriteOnlyField` remains a compile-time read-permission error regardless of the boolean result. The status query never grants read permission.

---

## § 7. Static interprocedural analysis and public method contracts

**§ 7(1).** A compiler-inferred method summary for a register `impl` carries at least the following *semantic categories*; they are not source-level effect attributes or user-defined types:

```text
receiver_kind
required_shadow_knowledge
ensured_shadow_knowledge
possible_shadow_invalidations
pending_intent_effects
hardware_observations
hardware_writes
hardware_read_side_effects
hardware_ordering_and_failure_effects
conditional_result_refinements
```

**§ 7(2).** A method that reads `self.Mode` without establishing it locally may be accepted with an inferred precondition `RequiresKnown(Control.Mode)`. Every caller must prove that precondition or be rejected. Merely compiling the method body need not fail because knowledge is supplied by a valid caller; an exported precondition remains part of the method's public semantic contract.

**§ 7(3).** A method that establishes a coherent observation, or an ordinary intended value, may export `EnsuresKnown(Control.Mode)` only if all normal return paths actually establish it. Exceptional/failed/partial paths carry their own conservative postconditions.

**§ 7(4).** Every call through a public member or imported module consumes the **same resolved semantic summary** as a local call. A downstream compiler must not need the implementation body to validate a call, but the producer must publish trustworthy summaries in its canonical `ModuleSurface`/compilation artifacts. Semantic changes to these summaries affect module compatibility and incremental invalidation.

**§ 7(5).** Method summaries must remain conservative for recursion, indirect calls, interface dispatch, missing/opaque bodies, cross-module boundaries, and control-flow joins. Unknown effects are not assumed pure or knowledge-preserving. A summary is valid only for the receiver and field identities to which it applies.

**§ 7(6).** The compiler preserves physical hardware effects independently of observed software knowledge. A method can read hardware destructively even if its return value is discarded; a shadow-only getter must not be mislabeled as a physical observation.

**§ 7(7).** LSP/docs must display the published semantic contract, including whether a returned value comes from shadow or a hardware observation and whether the method may touch external hardware. This is **static API information**, not something a runtime program automatically reflects upon. Runtime inspection of *whether shadow currently has a value* uses `IsShadowKnown` through the owning `impl`.

---

## § 8. Runtime branch refinement and useful public wrappers

**§ 8(1).** `IsShadowKnown(Control.Mode)` returns a runtime `bool` computed from existing compiler-managed software availability metadata (or a proven constant where possible). Its evaluation performs no hardware access, no allocation, and no change to pending intent or observed knowledge.

**§ 8(2).** On the `true` branch of a valid status check, Sema may establish `ShadowKnown(Control.Mode)` for the **same instance**, until an intervening possible invalidation, conflicting mutation, alias, call, or unknown external effect destroys that proof. `false` means no qualifying shadow value is available in the software view at that point. Availability is not equivalent to current-hardware truth.

**§ 8(3).** To make the public-API model useful across modules, a public wrapper returning the status must be able to carry a **conditional postcondition** when the compiler proves it: `return == true => ShadowKnown(Control.Mode)` for the same receiver. Otherwise `if device.HasKnownMode() { device.GetMode() }` would not be analyzable even though the program explicitly asked the question. A wrapper that transforms the result or may invalidate the knowledge after checking must not claim that implication.

**§ 8(4). Canonical source example** (canonical Sec methods have compiler-provided implicit `self`; do not write `ref self` in method signatures):

```sec
type Control register[8] {
    Mode: bit[3] shadow,
    _: bit[5],
}

impl Control {
    fn HasKnownMode() bool {
        return self.IsShadowKnown(Control.Mode)
    }

    fn GetMode() uint8 {
        return self.Mode
    }

    fn TryGetMode() Option[uint8] {
        if self.IsShadowKnown(Control.Mode) {
            return Some(self.Mode)
        }
        return None()
    }

    fn Refresh() Control {
        return self.Read()
    }
}
```

`GetMode()` has a statically checked `RequiresKnown(Control.Mode)` precondition. `TryGetMode()` represents unknown software status explicitly and does not access hardware. `Refresh()` returns the detached observed snapshot and is **not** a shadow-only operation: it explicitly requests hardware observation with all effects and physical-plan restrictions of `Read()`. The example assumes the existing `Read()` contract establishes knowledge for `Mode` on successful normal completion; do not assume that for every endpoint/projection.

**§ 8(5).** A different module may call `HasKnownMode`, `GetMode`, and `TryGetMode` as ordinary published members. It may never directly invoke the private compiler-owned `IsShadowKnown` on the hardware binding. A method that directly calls `IsShadowKnown` cannot be invoked on detached values as though detached register values carried hardware shadow metadata.

**§ 8(6).** Flow refinement must be sound under concurrency and aliasing. If another permitted execution context can mutate the *software shadow metadata* between check and use, the compiler must prove exclusion/synchronization, preserve a coherent guarded access, or reject the proof. Hardware changing by itself does not invalidate the **historical software-knowledge** classification, but does mean the value must not be represented as the current physical state.

**§ 8(7).** If a conditional proof cannot be exported for a wrapper because the method returns a nontrivial transformed result, the wrapper can instead return an explicit `Option[T]` from inside a checked branch. The compiler must never manufacture an implicit fallible getter as a fallback.

---

## § 9. Compiler-known reservation, lookup, and registry

**§ 9(1).** The register-`impl` reserved names in Sec 0.1 are exactly:

```text
Read
Write
IsShadowKnown
```

`Read` and `Write` are **already reserved** by `registers.md` § 11.7 and `internal/sema/register_operations.go`. Adding `IsShadowKnown` extends that policy without global reservation of the same spelling on unrelated user-defined types.

**§ 9(2).** For every nominal `register[N]` type, a declaration of a method, any overload, property, static member, nested associated declaration, or any other user-owned member under one of these names is invalid **in both the primary `impl` and any `impl extends` fragment**. Detect at member registration, independent of calls and before overload resolution. A different parameter list, `static`, different return type, or different receiver assumptions cannot bypass the reservation.

**§ 9(3).** An ordinary non-register type can continue to define `Read`, `Write`, or `IsShadowKnown` where otherwise legal. The reservation is receiver-category-scoped, not a global reserved keyword, and adds no lexer keyword.

**§ 9(4).** The new operation belongs to the canonical single `CompilerKnownMember` registry (`compiler_known_members.md`), not a collection of independent string comparisons in Sema/LSP/IR. The existing ad hoc reservation/resolution in `internal/sema/register_operations.go` must be reconciled into that owner without changing the meaning of existing `Read` and `Write`.

**§ 9(5). Approved semantic registry contract** (descriptive schema, **not** a Sec declaration):

```text
ID: CKM-REGISTER-IS-SHADOW-KNOWN
CanonicalName: IsShadowKnown
Category: CompilerKnownOperation
Receiver: HardwareInstance[T] (inside owning impl T only)
Operand: one static RegisterFieldSelector rooted in T
Result: bool
IsUnsafe: false
IsOverridable: false
IsInheritable: false
RequiresAllocation: false
HardwareRead: never
HardwareWrite: never
SoftwareEffects: reads compiler-managed per-instance shadow availability
SemanticOperation: RegisterShadowIsKnown (illustrative internal name; stable semantic identity required)
```

The registry must also have **stable, distinct entries** for existing `Read()`, `Write()`, and `Write(value)` operations. Their precise registry IDs are assigned by the canonical registry implementation, not by overloading a user declaration.

**§ 9(6).** `IsShadowKnown` requires no public field-selector type. The compiler-only selector is validated and resolved before Semantic IR construction, and its semantic ID and receiver instance identity are carried in the resulting operation.

**§ 9(7).** The LSP must derive name availability, reserved-name diagnostics, completion, hover, definitions and semantic documentation from the same registry/member model used by Sema. No alternate LSP-owned intrinsic namespace.

---

## § 10. Effects, optimization, Semantic IR, and lowering

**§ 10(1).** Introduce one explicit Semantic IR operation or canonical intrinsic-call identity for the software shadow availability query. It carries: concrete hardware-implementation instance identity, `RegisterTypeID`, complete canonical `RegisterFieldID` path, effective shadow policy, source location, and the relevant availability-state analysis/guard fact. No physical endpoint access plan is needed to **read software availability metadata**, although constructing the hardware instance remains subject to the existing binding contract.

**§ 10(2).** `IsShadowKnown` is not `register.hw.observe` and not a read of hardware. It is a software-state query; it must not be lowered to a volatile MMIO read or to the field's special `read-clear`/`read-set` effect. Any optimizer may fold a proven constant only when it preserves shadow-state semantics and all other source-visible effects.

**§ 10(3).** The compiler must preserve a dataflow relationship between the query result and the selected instance/field's availability knowledge, including interprocedural conditional summaries and invalidations. Lowering must not treat a previously checked `true` as permanently true after an operation that can invalidate knowledge.

**§ 10(4).** `IsShadowKnown` does not expose a general runtime register description, reflection table, raw hardware pointer, or direct access to compiler-managed backing storage. Its implementation may be a constant, a bounded known-mask test, or another equivalent local software representation, depending on validated lifetime and aliasing rules.

**§ 10(5).** The existing `register.hw.shadow_observe`, `register.hw.shadow_intent`, `register.hw.shadow_invalidate`, and `register.hw.commit` remain separate from the `IsShadowKnown` status query. MD-113 must not erase the MD-112 distinction between regular intended values and special pending action masks.

**§ 10(6).** Sec MLIR must have either a verifier-checked shadow-query operation with field and instance identity or an explicitly justified already-canonical equivalent. Its exact spelling, schema admission, verifier attributes, and lowering shall be selected and specified in the existing normative MLIR document/version system as implementation-integration details; **MD-113 does not prescribe a new dialect schema number or operation spelling**. Physical hardware paths still fail closed without the MD-111 verified plan.

**§ 10(7).** Runtime shadow metadata is ordinary compiler-managed per-instance software state and does not imply garbage collection, a background hardware manager, or hidden periodic polling. It must obey existing lifetime, borrowing, sharing and destruction rules.

---

## § 11. Required diagnostics

**§ 11(1).** Diagnostics must state the invalid operation, the relevant rule, and a safe correction. Required categories include:

| Invalid case | Required diagnostic direction |
|---|---|
| `self.Mode` when shadow availability cannot be proven | Explain `Mode` may be unknown; establish a valid observation/intended value or guard with the software-status query |
| `self.IsShadowKnown(Mode)` | Require the explicit register-type-qualified field selector |
| `self.IsShadowKnown(self.Mode)` | Explain that the argument must identify a field and must not evaluate `self.Mode` |
| `self.IsShadowKnown(Control.Other)` with nonexistent field | Identify the missing field/path component |
| `self.IsShadowKnown(OtherRegister.Mode)` | Identify mismatch between receiver's nominal register type and selector root |
| `self.IsShadowKnown(Control.Ready)` with `noshadow` | Explain that status inspection applies only to shadow fields |
| `self.IsShadowKnown(Control._)` | Explain reserved/unnamed fields cannot be selected |
| `device.IsShadowKnown(Control.Mode)` outside owning `impl` | Explain compiler-only restriction and suggest a published ordinary wrapper |
| `self.IsShadowKnown(Control.Mode)` on detached value | Explain absence of hardware-instance shadow state |
| User declares `fn IsShadowKnown(...)` or a same-named property in register `impl` | Report reserved non-overloadable compiler-known member; include `impl extends` conflicts |
| An imported method's `RequiresKnown` cannot be proven | Identify the originating method and field-precondition, not an invented hardware read |
| `if HasKnownMode()` used after a possible invalidating effect | Explain why the previous knowledge proof no longer holds |
| Endpoint lacks a legal `Read()` plan in an example that calls `Read()` | Report no safe hardware observation plan, never substitute a shadow query or unsafe read |

**§ 11(2).** A status query for a valid field may return `false`; that is a runtime value, **not** a compiler error. A read that requires unavailable knowledge **is** a compile-time error when not adequately guarded or supplied by a caller contract.

---

## § 12. Mandatory positive and negative tests

**§ 12(1). Register and selector tests:** Simple leaf, nested leaf, inherited register-level `shadow`, field-level `noshadow` override, same-width distinct nominal roots, shadowed bit-backed enum, reserved `_`, nonexistent step, local variable name collision, qualified type lookup, expression arguments rejected, zero runtime field-path allocation.

**§ 12(2). State tests:** Fresh binding unknown without I/O; explicit successful observation makes the observed projection known; read-clear shadow records historical observation without a second read; ordinary intended value available before commit; pending W1C alone does not establish known; pending W1C preserves prior observed view; all-neutral MD-112 action does not manufacture status; commit success/failure/partial write respects proven postconditions; unknown getter without guard rejected.

**§ 12(3). Interprocedural tests:** Local and cross-module `RequiresKnown` / `EnsuresKnown`; conditional `return true => Known` wrapper followed by guarded getter; `Option` wrapper usable without requiring prior knowledge; CFG merges, recursion, indirect calls, invalidating calls, and interface dispatch conservatively handled; import summaries preserve field identity and source diagnostics; no dependence on LSP to enforce semantics.

**§ 12(4). Effects and code generation tests:** Status check emits no MMIO/CSR/port I/O instruction; no destructive read is introduced; no physical write triggered by metadata query; constant-folding of provably known status correct; software known-mask dynamic lowering correct where permitted; verified query and shadow transitions survive Semantic IR/MLIR; missing hardware plan still fails closed for true hardware operations.

**§ 12(5). Reservation tests:** User declarations of `Read`, `Write`, and `IsShadowKnown` rejected across ordinary methods, overloads, properties, static members and `impl extends` in register types; names remain available for unrelated ordinary types; LSP and formatter agree with Sema, and the compiler-known registry is canonical.

---

## § 13. Integration instructions, without duplicate governance

**§ 13(1).** Amend `rules/platform/hardware-register-access.md` § 9 to define initial availability, intended versus observed values, action intents, status query and result refinement. Amend § 30 and associated ordering/dataflow sections only where needed to preserve the canonical operation across lowering.

**§ 13(2).** Amend `rules/declarations/registers.md` § 11.7 to add the new reserved operation and cross-reference its field-selector semantics; keep detached-register behavior unchanged. Amend `rules/declarations/impl.md` §§ 8 and 22–23 only to clarify reservation in the combined primary/extended `impl` surface; never reintroduce explicit receiver parameters.

**§ 13(3).** Amend `rules/compiler/compiler_known_members.md` to give the complete registry entry, visibility, operand category, naming/reservation, effects, Semantic IR identity and LSP obligations. The current `Read`/`Write` implementation must be reconciled into the single registry rather than duplicated.

**§ 13(4).** Add only the necessary compatible Semantic IR and Sec MLIR normative amendments, using the currently active schema rules. Do not guess accepted dialect schema versions or claim physical register lowering supported on every target.

**§ 13(5).** Reconcile `missing-decisions.yaml` MD-113 with this approved decision, recording decision-complete / implementation-pending as appropriate. Existing `governance/platform.yaml` `platform.hardware-register-access` is the implementation owner. Extend its existing acceptance/tests/remaining records by stable integration IDs when necessary; do not create a second competing platform register owner. Keep the decision tracked while the project's current implementation-pending policy requires it.

**§ 13(6).** Update the manual, compiler-known documentation, generated API/LSP source descriptions and test fixtures together. Runtime availability queries and static effect summaries must be documented separately so that a reader does not mistake a `Known` shadow value for current hardware truth.

---

## § 14. Approved resolution of review items

**§ 14(1). Field-selector source spelling.** `RegisterType.Field[.NestedField]`, admitted only as the static selector operand of the compiler-known `IsShadowKnown` call, is the canonical Sec 0.1 spelling (§ 4). It is never an evaluated hardware field read and does not introduce general-purpose reflection or a first-class selector type.

**§ 14(2). Write-only status inspection.** The status-only query is permitted for an effective `write-only shadow` field, subject to the owning `impl` boundary; the field's value remains unreadable through ordinary source field access (§ 6(7)).

**§ 14(3). Conditional public summaries.** A correctly proven public wrapper must export `return == true => ShadowKnown(field)` for the same instance and field, so that a guarded caller may use the proof across module boundaries (§ 8(3)). If the wrapper does not semantically establish that implication, it must not claim it.

**§ 14(4). Initial state.** At newly established hardware binding, every shadow field starts `Unknown`, unless an independently defined, explicit and compiler-proven binding/init contract has established actual valid *software* knowledge without fabricated hardware state (§ 5(1)). No implicit register read, inferred reset value, or synthetic zero is permitted.

**§ 14(5). Guard and concurrency soundness.** A known-mask or equivalent software representation may be materialized only where needed; check-then-read proofs must remain sound under aliases, calls, concurrent access and mutation/invalidation (§§ 8(2), 8(6), 10(4)). Representation remains compiler-owned, not part of the public Sec type system.

**§ 14(6). Static public effect/knowledge contracts.** Method summaries are compiler-inferred, serialized as part of the module's semantic API, and enforced at call sites. They are not merely LSP hover descriptions, runtime method-reflection metadata, or manually declared Sec 0.1 effect annotations (§ 7).

**§ 14(7). Registry/IR/MLIR integration.** The approved compiler-known operation has a stable semantic identity and the semantics of §§ 3, 9–10. Specific internal implementation names, Sec MLIR operation spelling, verifier encoding and schema/version placement are delegated to controlled normative integration. They must preserve this decision and cannot change Sec source semantics. Any newly discovered *semantic* incompatibility requires a separate explicit decision rather than an unannounced implementation choice.

**§ 14(8). Approval.** The Sec language owner approved this complete MD-113 design on 2026-10-30. The former review alternatives in the revision 0.1 draft are resolved by §§ 14(1)–14(7). Status is **decision finalized; integration and implementation pending**. No compiler implementation or test completion is asserted.

---

## Appendix A. Provenance of existing contracts

- `rules/platform/hardware-register-access.md` §§ 7–9: explicit `Read`/`Write`, `Unknown`/`Known`, pending operation intents, no fabricated initialization, no automatic refresh/write-back.
- `rules/declarations/registers.md` § 11.7: detached vs hardware receiver, type-owned `impl`, register fields private to `impl`, compiler-reserved `Read` and `Write`.
- `rules/declarations/impl.md` § 8: compiler-provided implicit `self`; `fn Method(ref self)` is no longer canonical; §§ 22–23 combined `impl` member namespace and collision rules.
- `rules/compiler/compiler_known_members.md` §§ Registry, Stable member identity, Reserved member names, Effects: single canonical registry, non-overridable members by receiver category, explicit effect and Semantic IR identity.
- `rules/projects/modules.md` §§ 9–12: source-file-local import qualifiers; no arbitrary path or implicit re-export semantics.
- `rules/corrections/applied/md077-register-semantic-ir-final-decision-20261022.md` §§ 7–11, 19: hardware semantic operations and software knowledge versus external effects.
- `MD-112-specialized-register-write-non-trigger-and-multibit-correction.md` (owner-approved local proposal, 2026-10-30): per-bit specialized actions, runtime masks, neutral logical no-ops and shadow action-intent distinction.
- `missing-decisions.yaml` MD-113, baseline `aa59b4c`: four unresolved aspects enumerated in the implementation ledger; this decision resolves them with binding source and compiler contracts.

**End of MD-113 — decision approved 2026-10-30; normative integration and implementation pending.**
