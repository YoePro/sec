# MD-059 — Unsafe Callable Values and the Foreign Trust Boundary

- **Status:** Applied 2026-10-16 — integrated into `rules/memory/unsafe.md` §§ 39(6)–(7), 40(6), 41(5), the `rules/foundations/grammar.md` FunctionType note and `rules/platform/ffi.md` §§ 25, 42; parser diagnostic for `unsafe fn(...)` types and S1141 help updated; tests `TestMD059NoUnsafeFunctionType`; governance `analysis.yaml`; MD-059 closed. Follow-ups: MD-078 (callback trusted-fact encoding), MD-079 (`unsafe Call()` example form); `c::callback`/`c::fn` remain parser-unsupported (known hole).
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Proposed canonical path:** `rules/corrections/pending/md059-ffi-unsafe-callables-correction-20261016.md`
- **Decision ID:** `MD-059`
- **Primary rulebooks:** `rules/memory/unsafe.md`, `rules/foundations/grammar.md`, `rules/platform/ffi.md`
- **Related rulebooks:** `rules/declarations/functions.md`, `rules/declarations/interfaces.md`, `rules/declarations/generics.md`, `rules/platform/abi.md`, compiler Semantic IR, diagnostics, formatter, and testing
- **Replaces:** No existing stable numbered paragraphs. Amend existing rules in place, preserving their identifiers.
- **Governance:** Reconcile existing per-book governance fragments and the authoritative `missing-decisions.yaml`. Do not infer implementation or validation merely from this design decision.

---

## § 1. Scope and rationale

**§ 1(1).** Sec's ordinary safe callable model shall not acquire general first-class `unsafe fn(...)` types in Sec 0.1. This decision resolves the ordinary-function-type question in MD-059 **without** making foreign callbacks impossible.

**§ 1(2).** `unsafe` denotes caller obligations that cannot be fully established from ordinary Sec typing alone; it is not an invitation to reproduce C's unrestricted pointer and lifetime conventions throughout Sec.

**§ 1(3).** Hardware register declarations, platform knowledge, borrowing, ownership, and other compiler-verifiable language mechanisms remain preferred to unsafe raw access when their contracts apply. No new raw-memory language capability is introduced here.

**§ 1(4).** Foreign interoperability includes C and non-C boundaries, in particular operating-system system calls, system ABI interfaces, and other target-specific foreign execution boundaries. FFI does not mean only C.

## § 2. Native function-type grammar is unchanged

**§ 2(1).** The existing native `FunctionType` grammar remains authoritative in Sec 0.1:

```text
FunctionType
    ::= [ CallableCapability ] "fn"
        "(" [ CallableParameterType { "," CallableParameterType } [ "," ] ] ")"
        TypeReference

CallableCapability
    ::= "mut"
      | "->"
```

**§ 2(2).** In particular, the following is **not** a native Sec 0.1 type:

```sec
// Invalid in Sec 0.1: no general unsafe callable type.
let operation: unsafe fn() void
```

**§ 2(3).** An `unsafe fn` **declaration** remains supported. Rejecting the general unsafe function-*type* syntax must not reject existing unsafe function declarations or their explicitly authorized direct calls.

```sec
unsafe fn PerformUnchecked() void {
    // Caller obligations are documented by the function's contract.
}

unsafe PerformUnchecked()
```

**§ 2(4).** An unsafe Sec function must not be converted, implicitly or by a plain type assertion, into an ordinary safe `fn(...)`, `mut fn(...)`, or `-> fn(...)` value. Entering an `unsafe` context does not change the source function's callable safety classification.

**§ 2(5).** No spelling, ordering rule, formatter form, or implicit coercion is introduced for a general native `unsafe fn(...)` function type, including when the callable is referenced through generic type parameters. This is an intentional Sec 0.1 restriction, not merely a missing parser feature.

## § 3. Interfaces and generics

**§ 3(1).** Existing named unsafe methods and their interface safety requirements remain valid wherever Sec's declaration rules permit them. An unsafe interface requirement is **not** itself a first-class unsafe function-value type.

**§ 3(2).** A safe interface requirement cannot be satisfied by an implementation that adds unsafe caller obligations. Dynamic dispatch may not discard a requirement's unsafe status.

**§ 3(3).** Generic code must preserve the safety contracts of named functions and methods it invokes through established constraints. No general unsafe callable-type syntax is inferred or synthesized as part of generic substitution.

## § 4. Existing FFI callable forms remain separate

**§ 4(1).** The existing C ABI function pointer type `c::fn(Parameters) ReturnType` is **not** a native Sec `fn(...)` value, and it does not become one by implicit conversion.

```sec
// Existing C ABI function-pointer type.
c::fn(c::int) void
```

**§ 4(2).** The existing `c::callback(callable)` is the explicit C callback adaptation mechanism. Existing restrictions on reusable, environment-free, non-capturing, ABI-compatible source callables remain in force. Callback adaptation is not general Sec function export.

```sec
fn OnForeignValue(value: c::int) void {
    ProcessForeignValue(value)
}

let callback := c::callback(OnForeignValue)
```

**§ 4(3).** `extern "C"`, `extern "system"`, and other supported foreign calling conventions remain distinct. The existence of `c::fn` does **not** imply that all system-ABI indirect calls have the C ABI. Where system-ABI function pointers are needed, their precise representation belongs to the FFI/ABI specification, not to native `FunctionType`.

**§ 4(4).** Direct system calls through a declared `extern "system"` function remain governed by its ordinary FFI and `unsafe extern` contracts; they do not require a new general unsafe callable type.

## § 5. Safe callbacks and unsafe foreign operations

**§ 5(1).** A safe Sec callback may invoke unsafe native or foreign operations internally only after **all** corresponding caller obligations have been established by the wrapper and/or a valid trusted foreign contract. Merely placing the call inside `unsafe` is not evidence that the obligations were satisfied.

**§ 5(2).** A safe wrapper must not leave unexpressed safety obligations for a future foreign caller. A wrapper that only forwards an unchecked raw pointer without establishing its validity does not become sound solely because its own declaration omits `unsafe`.

**§ 5(3).** The external contract may establish facts that Sec cannot independently derive, such as the lifetime and readable extent of a callback buffer, allowable callback threads, exclusivity, retention policy, or callback invocation sequencing. Such facts are **trusted assumptions**, not compiler proofs about a foreign binary.

**§ 5(4).** Foreign contract facts must retain provenance through the binding, callback adaptation, Semantic IR, lowering, and relevant auditing/tooling. Changing the foreign binding or target must invalidate or trigger reconsideration of any dependent trusted assumptions according to existing compiler dependency rules.

**§ 5(5).** If a required safety fact is absent, conflicting, invalid for the target/ABI, or not adequately represented by the supported contract model, the compiler and binding must **fail closed**. The compiler must not infer validity from the spelling of a foreign symbol or from the mere presence of `unsafe`.

## § 6. Adapting an unsafe Sec function directly

**§ 6(1).** `c::callback(UnsafeSecFunction)` is **not** unconditionally permitted. Its use must never implicitly launder an unsafe callable into a safe callback.

**§ 6(2).** An FFI-specific adaptation of an unsafe Sec callable may be specified only where the foreign callback registration or invocation contract explicitly covers **every** required caller obligation on **every possible invocation**. The ABI, callback lifetime, thread attachment/affinity, argument validity, ownership, retained pointers, effects, and failure handling must remain correct.

**§ 6(3).** Sec 0.1 does **not** acquire a blanket new source syntax or implicit automatic acceptance of this adaptation from MD-059. Until an existing or separately specified FFI contract form can represent and validate the relevant obligations, direct adaptation of the unsafe Sec callable must be rejected. A safe adapter remains possible only when it really discharges the obligations.

**§ 6(4).** The concrete binding-language representation of callback-specific trusted facts, including how they are associated with callback registration, remains **an FFI integration detail** to be specified in the owning FFI rulebook. It shall not be invented here as a general Sec callable modifier. Record any genuinely unresolved implementation fork separately in the missing-decisions workflow rather than silently relaxing these requirements.

## § 7. Established foreign boundary requirements remain active

**§ 7(1).** An unsafe callback or trusted ABI boundary does not waive borrow checking, ownership, object lifetime, nullability, pointer extent and alignment, synchronization, target validation, or effect constraints.

**§ 7(2).** No foreign exception, `longjmp`, Sec unwind, or other non-local control transfer may cross the active Sec/FFI boundary contrary to `ffi.md` §41. A Sec callback must translate fallible behavior to the foreign protocol where the contract requires this.

**§ 7(3).** All existing constraints for `c::callback`, including closure capture restrictions and relevant thread-attachment behavior, remain normative.

## § 8. Diagnostics and tooling

**§ 8(1).** Sema shall continue to reject use of unsafe Sec functions as ordinary safe function values. The existing `S1141 unsafe.function-value-mismatch` diagnostic must remain meaningful; entering `unsafe` alone is not a repair.

**§ 8(2).** Diagnostics should explain the distinction between an unsafe *declaration*, a safe native *function type*, and a foreign ABI *function pointer*, and direct the programmer toward a valid safe wrapper or an explicitly supported FFI binding contract.

**§ 8(3).** Missing foreign callback safety conditions are errors when those conditions are required to make the operation legal, not optional stylistic warnings. Non-failing explanatory tooling may emit INFO diagnostics under Sec's separately configured severity policy.

**§ 8(4).** Formatter, LSP, parser, AST, Semantic IR, and backend must agree that the ordinary native `FunctionType` grammar has no Sec 0.1 unsafe prefix.

## § 9. Mandatory validation and regression cases

**§ 9(1).** Parser/Sema reject a native `unsafe fn(...)` type and reject unsafe-function-to-native-safe-function conversions, including inside an `unsafe` block. Unsafe named declarations and valid direct calls continue to work.

**§ 9(2).** A valid environment-free safe Sec callback adapted through `c::callback` continues to work with exact ABI types, and is not treated as a native callable pointer.

**§ 9(3).** A callback wrapper with fully established safety preconditions may call an unsafe foreign function internally; tests must also reject the superficially similar wrapper lacking a necessary pointer-lifetime, extent, or thread guarantee.

**§ 9(4).** A direct unsafe callback adaptation is rejected when its needed external guarantee is unknown, unsupported, or contradicted. A trusted effect claim by itself is **not** proof of unrelated pointer/lifetime safety conditions.

**§ 9(5).** Regression cases cover system ABI versus C ABI, callback retention after registration, foreign-thread callback entry, callback failure translation, and prohibited foreign unwind. Check both frontend and backend behavior on relevant targets.

**§ 9(6).** Compare outcomes across optimization settings. No ABI lowering or optimization may erase unsafe/foreign trust provenance before all legality checks and required uses have been completed.

## § 10. Integration and closure

**§ 10(1).** Amend `rules/memory/unsafe.md` §39–§41 in place to retain their no-safety-laundering requirements while expressly removing any implication that Sec 0.1 provides a general native unsafe function-value type.

**§ 10(2).** Retain `rules/foundations/grammar.md`'s native `FunctionType` grammar. Add an explanatory note that the omission of a general `unsafe` prefix is intentional. No alternative native syntax is introduced.

**§ 10(3).** Clarify `rules/platform/ffi.md` §§24–25 and §§41–42 about the distinction between ABI adaptation, foreign trust claims, and callback-specific caller obligations. Integrate into existing paragraphs rather than renumbering stable references.

**§ 10(4).** Reconcile the matching governance items and test status without duplicating identifiers. This correction settles the **language-surface policy** of MD-059; the exact supported encoding of foreign callback preconditions and its implementation/validation must be tracked separately wherever incomplete.

**§ 10(5).** MD-059 may be considered **design-decided**, but not closed as implemented/validated merely because this document exists. Closure requires the canonical rulebooks, compiler behavior, diagnostics, tests, and governance to agree with the locked design.
