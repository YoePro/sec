# Correction — Atomics and concurrency memory model v2 cross-rulebook synchronization

- **Status:** Pending synchronization
- **Created:** 2026-09-06
- **Last updated:** 2026-09-06
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `0f5027d`
- **Primary owning rulebooks:** `rules/concurrency/atomics.md`, `rules/concurrency/concurrency_memory_model.md`
- **Governance fragments:** `implementation-status-atomics.yaml`, `implementation-status-concurrency-memory-model.yaml`
- **Classification:** Normative synchronization of decided atomic and memory-model semantics

---

## 1. Canonical decisions to synchronize

The following declarations are normative and must be represented as real source-visible core declarations:

```sec
enum MemoryOrder {
    Relaxed
    Acquire
    Release
    AcqRel
    SeqCst
}

enum CompareExchangeResult[T] {
    Exchanged
    NotExchanged(T)
}
```

The public atomic API uses CamelCase, including:

```text
Load
Store
Swap
CompareExchange
FetchAdd
FetchSub
FetchAnd
FetchOr
FetchXor
atomic.Fence
```

The key memory-model decisions are:

- `SeqCst` is the default for ordinary atomic object operations.
- `atomic.Fence(order: MemoryOrder) void` is part of Sec 0.1.
- Fence order is mandatory.
- `MemoryOrder.Relaxed` is invalid for a fence.
- every atomic object has a per-object total modification order;
- successful `CompareExchange` is RMW;
- failed `CompareExchange` is read-only;
- release sequences continue only through contiguous RMW modifications on the same atomic object;
- a plain store breaks the preceding release sequence;
- failure order may be `Relaxed`, `Acquire`, or `SeqCst`;
- failure order need not be weaker than success order.

---

## 2. `sec/core/` source-visible declarations

Create or update an appropriate core source file under `sec/core/`.

Exact filename is organizational in Sec 0.1. The semantic requirement is that the declarations exist in module `core` and are available to ordinary source tooling.

Required declarations:

```sec
module core

enum MemoryOrder {
    Relaxed
    Acquire
    Release
    AcqRel
    SeqCst
}

enum CompareExchangeResult[T] {
    Exchanged
    NotExchanged(T)
}
```

Add source comments matching the owning rulebook so hover/documentation can explain each variant.

Do not implement these types only as internal compiler enum/name tables.

Compiler-known identity and the core source declaration must resolve to one canonical semantic symbol.

For `Atomic[T]`, retain compiler-known storage/layout semantics but expose the complete canonical public member surface through compiler/core semantic registration so hover, completion, Sema and generated documentation agree on all method names/signatures.

Do not model `Atomic[T]` as a public ordinary struct whose contained `T` is directly accessible.

---

## 3. `rules/types/types.md`

Update the predeclared uppercase core/compiler-known type overview to include the atomic family where that overview enumerates nominal core types:

```text
Atomic[T]
MemoryOrder
CompareExchangeResult[T]
```

Do not duplicate the detailed semantics from `atomics.md`.

The type overview should state that exact atomic eligibility and operation semantics belong to `rules/concurrency/atomics.md`.

Do not make `Atomic` lowercase merely because it is compiler-known.

---

## 4. `rules/concurrency/concurrency.md`

Synchronize all atomic examples and summaries to public CamelCase:

```sec
Ready.Load(...)
Ready.Store(...)
Counter.FetchAdd(...)
State.CompareExchange(...)
atomic.Fence(...)
```

Remove lowercase public spellings.

Where the umbrella summarizes atomics, preserve the language/CompilationPlan split:

```text
Atomic[T] language eligibility
    !=
concrete operation support for selected CompilationPlan
```

Do not reintroduce a hard-coded width list into the umbrella.

Where release sequences or fences are mentioned, defer exact definitions to `concurrency_memory_model.md`.

---

## 5. `rules/concurrency/mutex.md`, channels, task/thread and other examples

Audit examples that use atomic public operations.

Replace stale lowercase spellings with the canonical CamelCase surface.

`mutex.md` itself also contains legacy lowercase public method spellings. Synchronize the already-defined public naming rule so the relevant mutex surface is:

```sec
impl Mutex[T] {
    fn Lock() MutexGuard[T]
    fn TryLock() Option[MutexGuard[T]]
}
```

Update normative examples from `.lock()` / `.tryLock()` to `.Lock()` / `.TryLock()`.

This is a naming synchronization, not a redesign of mutex acquisition semantics. Any still-unresolved timeout/context overload design remains owned by the mutex rewrite and must not be completed by this correction.

Ensure compiler Sema and LSP hover/completion expose `Mutex[T]`, `MutexGuard[T]`, `Lock`, and `TryLock` as the same canonical compiler-known/source-visible surface used by the rulebook.

Do not change the synchronization ownership of those books.

Where those books describe their own happens-before edges, keep them consistent with `concurrency_memory_model.md`.

---

## 6. `rules/compiler/semantic_ir.md`

Ensure the canonical Semantic IR model can preserve:

- atomic storage identity;
- resolved `Atomic[T]`;
- atomic load/store/RMW kind;
- successful versus failed `CompareExchange`;
- success and failure memory orders;
- explicit fence and fence order;
- source provenance;
- target capability requirements;
- synchronization/publication facts required by later analyses.

Do not require `concurrency_memory_model.md` or `atomics.md` to own a second list of hard-coded Semantic IR opcode names.

Successful `CompareExchange` must be represented as a modification/RMW.

Failed `CompareExchange` must be represented as read-only.

---

## 7. `rules/analysis/data_races.md`

Keep `data_races.md` as the owner of race pairing/proof classification.

Ensure it consumes the following facts from the memory model:

- per-atomic identity;
- atomic versus ordinary access;
- program order;
- synchronization;
- happens-before;
- modification/RMW classification where relevant;
- release-sequence communication;
- fence synchronization;
- task/thread completion publication;
- FFI/platform synchronization contracts.

Do not add an independent alternate definition of `MemoryOrder`.

---

## 8. Platform and `CompilationPlan`

Update target-profile/platform governance to make the separation explicit:

```text
language:
    Atomic[T] eligibility

CompilationPlan:
    concrete T layout/alignment
    operation capability
    requested order capability
    native/emulated implementation
    blocking/nonblocking property
    interrupt-safety property
```

Do not hard-code language eligibility from a target word size.

Do not use compiler-host capabilities.

A semantically valid type such as `Atomic[uint64]` may be accepted by the language and later rejected for a concrete operation on a selected target that cannot provide a conforming implementation.

---

## 9. LSP

LSP hover must expose the exact source-visible declarations:

```sec
enum MemoryOrder {
    Relaxed
    Acquire
    Release
    AcqRel
    SeqCst
}

enum CompareExchangeResult[T] {
    Exchanged
    NotExchanged(T)
}
```

Hover must include variant comments/documentation.

For `Atomic[T]`, hover must expose the resolved type argument and canonical public method signatures applicable to that `T`.

Completion must use CamelCase public names only.

Completion for `MemoryOrder.` must show exactly five variants.

Pattern completion for `CompareExchangeResult[T]` must show `Exchanged` and `NotExchanged(T)`.

Navigation must treat the core declaration and compiler-known identity as one semantic symbol.

The LSP must not use an independent hard-coded surface that can drift from compiler Sema.

---

## 10. Formatter and documentation

Update formatter fixtures and generated documentation examples to use canonical public CamelCase names.

The formatter must not insert explicit `MemoryOrder.SeqCst` arguments when an ordinary atomic operation omitted the order.

The formatter must not remove the mandatory explicit order from `atomic.Fence`.

Generated documentation must include generic arity, enum variants, variant payloads and public method signatures.

---

## 11. Tests

Add or synchronize tests for:

- exact `MemoryOrder` variants;
- exact `CompareExchangeResult[T]` variants and payload typing;
- LSP hover of both core enums;
- CamelCase member completion;
- rejection of lowercase legacy public names;
- atomic-compatible type set;
- float/decimal/composite rejection;
- language eligibility versus target capability;
- valid/invalid load/store/RMW/failure orders;
- success/failure order independence;
- `atomic.Fence` mandatory explicit order;
- relaxed fence rejection;
- per-object modification order facts where exposed to analysis tests;
- release sequence through relaxed RMW;
- release-sequence break by plain store;
- successful CAS extending release sequence;
- failed CAS not extending release sequence;
- fence/atomic, atomic/fence and fence/fence communication patterns;
- no volatile-as-synchronization regression.

---

## 12. Non-decisions

Do not invent the following while applying this correction:

- float atomic support;
- decimal atomic support;
- arbitrary struct atomic support;
- weak compare-exchange;
- atomic wait/notify;
- a public lock-free query API;
- tagged-pointer APIs;
- reclamation APIs;
- new boolean/byte fetch operations beyond the normative v0.1 surface;
- a new maximum atomic bit width in the language rule.

Any desire to add these requires a separate explicit language-design decision.
