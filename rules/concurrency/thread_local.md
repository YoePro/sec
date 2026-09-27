# Thread-Local Storage

- **Status:** Normative
- **Created:** 2026-09-26
- **Last updated:** 2026-09-27
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/thread_local.md`
- **Replaces:** Earlier legacy revision at the same canonical path
- **Repository baseline reviewed:** `main-reviewed-2026-09-26`
- **Implementation governance:** `governance/concurrency_thread_local.yaml`
- **Related rulebooks:** `rules/concurrency/threads.md`, `rules/concurrency/tasks.md`, `rules/concurrency/scheduling.md`, `rules/concurrency/blocking.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/copy_move.md`, `rules/memory/destruction.md`, `rules/memory/lifetime_analysis.md`, `rules/declarations/static.md`, `rules/declarations/functions.md`, `rules/declarations/properties.md`, `rules/declarations/impl.md`, `rules/platform/ffi.md`, `rules/compiler/semantic_ir.md`, `rules/platform/platform_model.md`, `rules/platform/target_profiles.md`, `rules/tooling/lsp.md`

---

## § 1. Purpose and authority

**Governance tags:** `concurrency.thread-local-v2`

§ 1(1) `ThreadLocal[T]` provides one logically independent `T` for each attached physical Sec thread.

§ 1(2) Thread-local state is distinct from:

- task-local state;
- ordinary static/global state;
- process-local state;
- CPU-core-local state;
- interrupt-local state.

§ 1(3) A logical task may migrate between physical threads. Thread-local storage therefore follows physical `Thread` identity, not logical `Task` identity.

§ 1(4) This rulebook owns the portable `ThreadLocal[T]` key/API, first-access initialization, thread-bound borrows, per-thread destruction, foreign-thread attachment, TLS-specific analysis, Semantic IR requirements, lowering obligations, and tooling requirements.

§ 1(5) `threads.md` owns physical thread identity, `ThreadContext`, `ThreadID`, thread creation, join, detach, and `ThreadSpawnError`.

§ 1(6) `ffi.md` owns foreign ABI/callback declaration mechanics.

§ 1(7) Mutable implementation status belongs only in `governance/concurrency_thread_local.yaml`.

---

## § 2. Core model

**Governance tags:** `concurrency.thread-local-v2`

§ 2(1) A `ThreadLocal[T]` value is a program-level TLS key plus initializer description.

§ 2(2) The key does not itself contain one shared mutable `T`.

§ 2(3) For each attached physical thread, the key identifies at most one current per-thread `T` instance.

§ 2(4) Two distinct physical threads accessing the same key access distinct `T` objects.

§ 2(5) Two logical tasks executing sequentially on the same physical thread access the same current per-thread instance.

§ 2(6) One logical task that migrates may therefore observe a different per-thread instance after migration.

---

## § 3. Canonical source-visible types

**Governance tags:** `concurrency.thread-local-v2`, `tooling.thread-local-v2`

§ 3(1) The Sec 0.1 source-visible/compiler-known types owned by this rulebook are:

```text
ThreadLocal[T]
ThreadAttachment
ThreadAttachError
```

§ 3(2) `ThreadContext` is reused from `threads.md`.

§ 3(3) This revision does not define `ThreadContextError`.

§ 3(4) Compiler-known status must correspond to real source-visible declarations and LSP-visible semantic identities.

---

## § 4. Exact `ThreadLocal[T]` declaration

**Governance tags:** `concurrency.thread-local-v2`, `frontend.thread-local-v2`

§ 4(1) The canonical type identity is:

```sec
@noCopy
type ThreadLocal[T]
```

§ 4(2) The exact public behavior surface is:

```sec
impl ThreadLocal[T] {
    init(initial: T)
    init(factory: fn() T)

    fn Borrow() ref T
    fn BorrowMut() ref mut T
    fn Replace(value: T) T
}
```

§ 4(3) `ThreadLocal[T]` is non-copyable.

§ 4(4) Ordinary user code does not construct movable per-thread slot objects directly.

§ 4(5) A `ThreadLocal[T]` key is normally owned by static storage.

---

## § 5. No `.Value` property

**Governance tags:** `concurrency.thread-local-v2`, `frontend.thread-local-v2`

§ 5(1) Sec 0.1 defines no `ThreadLocal[T].Value` or legacy `.value` property.

§ 5(2) The following legacy forms are non-canonical:

```sec
LastError.value = 5
let error := LastError.value
let current := ref Context.value
```

§ 5(3) TLS access is explicit through:

```text
Borrow
BorrowMut
Replace
```

§ 5(4) This avoids one property expression having context-dependent value/copy/reference/move semantics.

§ 5(5) The property system does not receive a TLS-specific exception.

---

## § 6. Slot always-initialized invariant

**Governance tags:** `concurrency.thread-local-v2`, `analysis.thread-local-v2`

§ 6(1) After successful first initialization, a live per-thread slot always contains one initialized `T`.

§ 6(2) Sec 0.1 defines no general `Take()` operation that leaves the slot uninitialized.

§ 6(3) `Replace(...)` exchanges the old value for a new value while preserving the initialized-slot invariant.

§ 6(4) Sema/runtime do not expose an ordinary source-level initialized/uninitialized state after successful TLS initialization.

§ 6(5) Hidden implementation state may still distinguish pre-initialization and initialization-in-progress.

---

## § 7. Copyable initial-value constructor

**Governance tags:** `concurrency.thread-local-v2`, `memory.copy-move-v2`

§ 7(1) The overload:

```sec
init(initial: T)
```

is available only when the concrete resolved `T` is implicitly copyable under `copy_move.md`.

§ 7(2) Sec 0.1 has no general source `Copy` interface solely for expressing this constructor restriction.

§ 7(3) Therefore constructor eligibility is a compiler-checked semantic constraint after concrete `T` resolution, not a user-written `T: Copy` generic constraint.

§ 7(4) The key owns the initializer template value.

§ 7(5) On first access by a physical thread, the template is copied into that thread's TLS instance.

§ 7(6) The copy uses ordinary implicit-copy semantics and must therefore be infallible, allocation-free, and free from copy-time observable side effects as required by the copy model.

---

## § 8. Factory constructor

**Governance tags:** `concurrency.thread-local-v2`, `analysis.effects-v1`

§ 8(1) The overload:

```sec
init(factory: fn() T)
```

stores an initializer callable that produces one owned `T`.

§ 8(2) The factory form is valid for copyable or move-only `T`.

§ 8(3) The factory executes exactly once per attached physical thread instance that first accesses the key.

§ 8(4) The factory returns plain `T`; TLS initialization introduces no separate typed initializer error channel.

§ 8(5) A factory that performs fallible work must handle that work within its own implementation and still either return `T` or terminate through ordinary panic/cancellation/runtime semantics permitted for that context.

§ 8(6) The factory callable/captures are owned by the static `ThreadLocal[T]` key according to ordinary callable ownership rules.

---

## § 9. Deterministic lazy initialization

**Governance tags:** `concurrency.thread-local-v2`

§ 9(1) Per-thread `T` initialization is lazy and occurs at first semantic access to that key on that physical thread.

§ 9(2) The following all count as first access:

```text
Borrow()
BorrowMut()
Replace(...)
```

§ 9(3) The runtime/backend must not eagerly execute the user factory at:

- program startup;
- thread creation;
- thread attachment;
- deferred `Thread.Start()`.

§ 9(4) This prohibition applies because factory execution may have observable effects.

§ 9(5) Backends may eagerly provision raw TLS metadata/storage where doing so does not execute user initialization or otherwise change observable semantics.

---

## § 10. First access with initial-value constructor

**Governance tags:** `concurrency.thread-local-v2`

§ 10(1) For `init(initial: T)`, first access:

1. locates/provisions the current physical thread's slot;
2. copies the template `T`;
3. marks that per-thread key successfully initialized;
4. registers that `T` in the thread's TLS destruction order;
5. continues the requested access operation.

§ 10(2) No other physical thread's instance is read or modified.

---

## § 11. First access with factory constructor

**Governance tags:** `concurrency.thread-local-v2`

§ 11(1) For `init(factory: fn() T)`, first access:

1. marks the current key/thread pair as initialization-in-progress;
2. invokes the factory exactly once;
3. on successful return, installs the produced owned `T`;
4. marks the slot initialized;
5. registers the instance in successful first-initialization order;
6. continues the requested access operation.

§ 11(2) The produced value is not copied merely because it becomes TLS-owned.

§ 11(3) Ownership moves directly into the current thread's per-key slot.

---

## § 12. Initialization recursion

**Governance tags:** `concurrency.thread-local-v2`, `analysis.thread-local-v2`

§ 12(1) A factory may access a different `ThreadLocal[U]` key.

§ 12(2) A factory must not recursively access the same key on the same physical thread while that key is initialization-in-progress.

§ 12(3) Statically provable same-key recursive initialization is a compile-time diagnostic.

§ 12(4) Dynamically encountered same-key recursive initialization is a canonical runtime-check failure governed by `runtime_checks.md`/panic policy.

§ 12(5) The implementation must not invoke the same factory recursively to "resolve" this condition.

---

## § 13. Factory panic/failure before successful initialization

**Governance tags:** `concurrency.thread-local-v2`, `errors.panic-v2`

§ 13(1) If the factory does not return successfully, no initialized `T` exists for that key/thread pair.

§ 13(2) No `T` destruction responsibility is registered for the incomplete initialization.

§ 13(3) The factory is not re-run again within the same physical thread instance after an initialization attempt has escaped through panic/terminal failure.

§ 13(4) If execution can later continue on that same physical thread under a future/target panic containment policy, subsequent access to the failed key is a runtime-check failure rather than an initializer retry.

§ 13(5) This preserves the exactly-once factory invocation rule.

---

## § 14. Initializer effects

**Governance tags:** `concurrency.thread-local-v2`, `analysis.effects-v1`

§ 14(1) A potentially first access through `Borrow`, `BorrowMut`, or `Replace` carries the effects that may be executed by the selected initializer form.

§ 14(2) The copyable template constructor contributes ordinary implicit-copy effects, which are infallible/allocation-free by definition.

§ 14(3) The factory constructor contributes the factory's inferred effects on a potentially first access.

§ 14(4) Effect analysis may remove first-initialization effects only where it proves the current physical thread/key pair already initialized on every reaching path.

§ 14(5) A no-allocation/no-blocking context may therefore reject access to a factory-backed TLS key whose factory has incompatible effects.

---

## § 15. `Borrow()`

**Governance tags:** `concurrency.thread-local-v2`, `analysis.borrowing`

§ 15(1) The exact operation is:

```sec
fn Borrow() ref T
```

§ 15(2) It accesses the current physical thread's instance.

§ 15(3) It triggers first initialization when necessary.

§ 15(4) It returns a shared borrow.

§ 15(5) It does not copy `T`.

§ 15(6) It does not change the slot's initialized state.

---

## § 16. `BorrowMut()`

**Governance tags:** `concurrency.thread-local-v2`, `analysis.borrowing`

§ 16(1) The exact operation is:

```sec
fn BorrowMut() ref mut T
```

§ 16(2) It accesses the current physical thread's instance.

§ 16(3) It triggers first initialization when necessary.

§ 16(4) It returns an exclusive mutable borrow.

§ 16(5) Ordinary `ref mut` exclusivity applies.

§ 16(6) The immutable static `ThreadLocal[T]` key does not make the per-thread `T` immutable.

---

## § 17. `Replace(...)`

**Governance tags:** `concurrency.thread-local-v2`, `analysis.transferability`

§ 17(1) The exact operation is:

```sec
fn Replace(value: T) T
```

§ 17(2) `Replace` triggers first initialization if necessary because it must return the previous `T`.

§ 17(3) It then atomically at the source semantic level:

1. moves/copies the new parameter into the current thread's slot according to ordinary argument semantics;
2. transfers ownership of the old `T` to the caller.

§ 17(4) The slot is initialized before and after replacement.

§ 17(5) The old value is not destroyed by `ThreadLocal[T]` after it has transferred to the caller.

§ 17(6) A reusable move-only source passed as `value` requires the ordinary call-site `<-` marker.

---

## § 18. Replace and aliasing

**Governance tags:** `concurrency.thread-local-v2`, `analysis.borrowing`

§ 18(1) `Replace` requires no live borrow of the current per-thread slot that conflicts with replacement.

§ 18(2) In particular, code must not keep a `Borrow()` or `BorrowMut()` reference live while replacing the same slot unless ordinary borrowing analysis proves the operation legal.

§ 18(3) A `BorrowMut()` reference and `Replace()` cannot coexist over the same slot.

---

## § 19. Example declaration

**Governance tags:** `concurrency.thread-local-v2`

§ 19(1) Copyable initial value:

```sec
static let LastError := ThreadLocal[int](0)
```

§ 19(2) Factory initialization:

```sec
static let Context := ThreadLocal[WorkerContext](
    fn() WorkerContext {
        return WorkerContext.Create()
    }
)
```

§ 19(3) Access:

```sec
let current := LastError.Borrow()
let mut context := Context.BorrowMut()
```

§ 19(4) Replacement:

```sec
let old := Context.Replace(CreateReplacement())
```

---

## § 20. Static-key requirement

**Governance tags:** `concurrency.thread-local-v2`, `declarations.static-v2`

§ 20(1) Portable `ThreadLocal[T]` keys are declared in static storage.

§ 20(2) A local/automatic `ThreadLocal[T]` key is invalid in Sec 0.1.

§ 20(3) The key identity must remain valid for every physical thread that may access it.

§ 20(4) Static-key construction initializes only key metadata/template/factory ownership; it does not initialize every physical thread's `T`.

---

## § 21. Key ownership and destruction

**Governance tags:** `concurrency.thread-local-v2`, `memory.destruction`

§ 21(1) The static key owns its initializer template or factory callable/captures.

§ 21(2) Static key destruction occurs according to canonical static destruction rules after physical-thread TLS cleanup obligations that may still require the initializer/key have ended.

§ 21(3) A backend must not destroy the key initializer while a future valid thread first access remains possible.

§ 21(4) Program termination paths that bypass static cleanup follow the general termination rules.

---

## § 22. Per-thread initialization identity

**Governance tags:** `concurrency.thread-local-v2`

§ 22(1) Initialization identity is the pair:

```text
(ThreadID generation, ThreadLocal key identity)
```

§ 22(2) Native thread-ID reuse must not cause a new physical thread to inherit a dead previous thread's TLS value.

§ 22(3) Thread attachment/generation metadata must distinguish reused native identities.

---

## § 23. Thread-bound borrow provenance

**Governance tags:** `concurrency.thread-local-v2`, `analysis.borrowing`

§ 23(1) Every `ref T`/`ref mut T` returned from TLS carries compiler-known thread-bound provenance.

§ 23(2) The provenance identifies at least:

- TLS key identity;
- physical `ThreadID` generation;
- borrow mutability;
- ordinary lifetime region.

§ 23(3) Thread-bound provenance remains semantic information until all suspension/migration/lifetime checks are complete.

§ 23(4) Backend pointer equality is not sufficient to represent this provenance.

---

## § 24. Task migration restriction

**Governance tags:** `concurrency.thread-local-v2`, `analysis.borrowing`, `concurrency.scheduling-v2`

§ 24(1) A TLS borrow used from logical task code must not remain live across any point where that task may resume on another physical thread.

§ 24(2) In Sec 0.1 this includes at least:

```text
await
task-context waiting join
Task.Yield()
waiting select
other task suspension/rescheduling points
```

§ 24(3) This rule protects identity, not merely address lifetime.

§ 24(4) An owned value copied/constructed independently from the borrowed `T` is no longer a TLS borrow merely because its data originated in TLS.

---

## § 25. Physical-thread waiting does not imply migration

**Governance tags:** `concurrency.thread-local-v2`, `concurrency.blocking-v1`

§ 25(1) A physical `Thread` execution that blocks/parks and later resumes as the same physical thread does not invalidate a TLS borrow solely because it waited.

§ 25(2) Ordinary lifetime/alias rules still apply.

§ 25(3) The compiler must distinguish logical task suspension from physical thread blocking where the execution model proves physical-thread identity stable.

---

## § 26. Task affinity exception

**Governance tags:** `concurrency.thread-local-v2`, `concurrency.scheduling-v2`

§ 26(1) A TLS borrow may cross a task scheduling point only if the compiler has an explicit canonical proof that the task cannot migrate during the complete borrow lifetime.

§ 26(2) Sec 0.1 provides no general public task-affinity guarantee.

§ 26(3) Therefore the default Sec 0.1 analysis rejects TLS borrows across task suspension.

§ 26(4) Backend optimism or observed scheduler behavior is not a proof.

---

## § 27. Access after task migration

**Governance tags:** `concurrency.thread-local-v2`

§ 27(1) A `ThreadLocal[T]` key itself may be accessed before and after task migration.

§ 27(2) Each access resolves against the physical thread current at that access point.

§ 27(3) Therefore:

```sec
let before := Counter.Borrow()
// task suspends and migrates
let after := Counter.Borrow()
```

may refer to two different per-thread `T` instances.

§ 27(4) Code requiring task-stable state must not use physical TLS for that purpose.

---

## § 28. Current-thread attachment requirement

**Governance tags:** `concurrency.thread-local-v2`, `concurrency.thread-v2`

§ 28(1) Every TLS access requires an attached Sec physical-thread context.

§ 28(2) The main thread and Sec-created threads are attached by runtime/startup semantics.

§ 28(3) A foreign physical thread entering Sec through FFI must be attached before executing a code path that requires TLS.

§ 28(4) The requirement is a semantic execution-context requirement, not an ordinary runtime `ThreadLocal` method result.

---

## § 29. Exact `ThreadAttachError`

**Governance tags:** `concurrency.thread-local-v2`, `tooling.thread-local-v2`

§ 29(1) The exact declaration is:

```sec
enum ThreadAttachError error {
    // The current physical thread already has an active Sec thread context.
    AlreadyAttached

    // Runtime resources needed to establish a Sec attachment are unavailable.
    ResourceUnavailable

    // A target/runtime attachment failure occurred and is not represented
    // by another ThreadAttachError variant.
    NativeFailure
}
```

§ 29(2) These three variants are the complete portable Sec 0.1 inventory.

§ 29(3) There is no `NotAttached` variant because ordinary TLS access does not return `ThreadAttachError`.

---

## § 30. Exact `ThreadAttachment`

**Governance tags:** `concurrency.thread-local-v2`, `analysis.transferability`

§ 30(1) The canonical owned attachment token is:

```sec
@noCopy
type ThreadAttachment
```

§ 30(2) Its exact public surface is:

```sec
impl ThreadAttachment {
    property Context: ThreadContext {
        get
    }

    free {
        // Compiler/runtime-owned detach and TLS cleanup.
    }
}
```

§ 30(3) `free` is a lifecycle member, not an ordinary callable `value.free()` method.

§ 30(4) `ThreadAttachment` owns the Sec attachment only; it does not own the foreign native thread itself.

§ 30(5) `Context` is a non-owning view of the currently attached physical thread.

---

## § 31. `Thread.AttachCurrent()`

**Governance tags:** `concurrency.thread-local-v2`, `concurrency.thread-v2`

§ 31(1) The exact portable attachment operation is:

```sec
impl Thread {
    static fn AttachCurrent() Result[ThreadAttachment, ThreadAttachError]
}
```

§ 31(2) It acts on the calling physical/native thread.

§ 31(3) On success it establishes a Sec `ThreadContext` and returns the unique owned attachment token.

§ 31(4) On the main thread or an already Sec-created/attached physical thread it returns:

```sec
Err(ThreadAttachError.AlreadyAttached)
```

§ 31(5) It performs no user `ThreadLocal[T]` factory initialization.

---

## § 32. Attachment ownership

**Governance tags:** `concurrency.thread-local-v2`, `analysis.transferability`

§ 32(1) `ThreadAttachment` may move only within the same physical thread execution identity.

§ 32(2) It must not be transferred to another physical thread.

§ 32(3) It must not cross a logical task suspension point where the task may migrate.

§ 32(4) It must not be returned to a scope that may destroy it on another physical thread.

§ 32(5) Destroying the token on the wrong physical thread is a compile-time error when provable and an invariant violation otherwise.

---

## § 33. Detachment by `ThreadAttachment` destruction

**Governance tags:** `concurrency.thread-local-v2`, `memory.destruction`

§ 33(1) Normal destruction of a `ThreadAttachment`:

1. destroys all initialized Sec TLS values owned by that attached physical thread in reverse successful first-initialization order;
2. releases attachment/runtime TLS metadata;
3. ends the Sec `ThreadContext` attachment.

§ 33(2) No other physical thread's TLS values are destroyed.

§ 33(3) The native foreign thread continues to exist; only its Sec attachment ends.

§ 33(4) After detach, Sec code requiring an attached physical-thread context must not execute until reattachment.

---

## § 34. Reattachment

**Governance tags:** `concurrency.thread-local-v2`

§ 34(1) A foreign native thread may attach again after a prior `ThreadAttachment` was fully destroyed.

§ 34(2) The new attachment is a new Sec physical-thread attachment generation.

§ 34(3) TLS values from the prior attachment do not survive into the new attachment.

§ 34(4) Lazy first initialization occurs again for keys accessed in the new attachment.

---

## § 35. Automatic FFI callback attachment

**Governance tags:** `concurrency.thread-local-v2`, `platform.ffi-v1`

§ 35(1) Generated foreign callback wrappers may attach the current foreign thread automatically when:

- the callback enters on an unattached foreign physical thread;
- the selected target/runtime supports attachment;
- callback semantics require attached Sec execution.

§ 35(2) A wrapper-created attachment is owned by that wrapper invocation/attachment scope.

§ 35(3) The wrapper destroys only an attachment it created.

§ 35(4) A nested callback on an already attached physical thread reuses the active attachment and must not detach it when the nested callback returns.

§ 35(5) Automatic attachment must not eagerly run user TLS factories.

---

## § 36. Foreign calls from Sec

**Governance tags:** `concurrency.thread-local-v2`, `platform.ffi-v1`

§ 36(1) Calling foreign code from an already attached Sec thread does not detach the current thread.

§ 36(2) Foreign code that calls back synchronously on the same native thread observes the same Sec attachment generation.

§ 36(3) Native TLS used by a foreign library is distinct from Sec `ThreadLocal[T]` unless a target-specific unsafe adapter explicitly maps them.

---

## § 37. `ThreadContextError` removal

**Governance tags:** `concurrency.thread-local-v2`

§ 37(1) The legacy type:

```text
ThreadContextError
```

is not part of the portable Sec 0.1 TLS API.

§ 37(2) Legacy variants:

```text
NotAttached
AlreadyAttached
ResourceUnavailable
NativeFailure
```

must not remain exposed under that old public identity merely for compatibility.

§ 37(3) Attachment failure uses `ThreadAttachError`.

§ 37(4) TLS access itself is governed by execution-context validity and does not return a context error.

---

## § 38. Sec-created thread initialization boundary

**Governance tags:** `concurrency.thread-local-v2`, `concurrency.thread-v2`

§ 38(1) Creating/starting a Sec thread establishes TLS runtime infrastructure required for the thread to support later lazy TLS access.

§ 38(2) It does not execute user `ThreadLocal[T]` initializers/factories.

§ 38(3) `ThreadSpawnError.ThreadLocalInitializationFailed` means required runtime/target TLS infrastructure failed before user callable execution could commit.

§ 38(4) It does not mean a user TLS factory returned an error or failed during first access.

§ 38(5) A deferred thread may perform required runtime TLS infrastructure setup at creation or `Start()` according to target mechanics, but user factory semantics remain lazy.

---

## § 39. Main thread

**Governance tags:** `concurrency.thread-local-v2`

§ 39(1) The program main physical thread has a Sec thread context before ordinary user main execution that may access TLS.

§ 39(2) User TLS values are still initialized lazily on first access.

§ 39(3) The main thread participates in the same successful first-initialization ordering and reverse destruction rules as other attached physical threads.

---

## § 40. Normal per-thread destruction

**Governance tags:** `concurrency.thread-local-v2`, `memory.destruction`

§ 40(1) Every successfully initialized per-thread `T` is destroyed exactly once on a normal cleanup path that ends that physical thread attachment.

§ 40(2) Destruction order is reverse successful first-initialization order for that physical thread.

§ 40(3) Declaration order does not determine per-thread destruction order.

§ 40(4) A key never accessed by a given physical thread has no per-thread `T` to destroy.

---

## § 41. Initialization/destruction ordering example

**Governance tags:** `concurrency.thread-local-v2`

§ 41(1) Suppose one physical thread first accesses:

```text
B
A
C
```

in that successful initialization order.

§ 41(2) Its normal TLS destruction order is:

```text
C
A
B
```

§ 41(3) Another physical thread may initialize the same keys in a different order and therefore has its own destruction order.

---

## § 42. Nested initialization ordering

**Governance tags:** `concurrency.thread-local-v2`

§ 42(1) If factory `A` first accesses key `B`, and `B` initializes successfully before `A` returns, successful initialization order is:

```text
B
A
```

§ 42(2) Normal destruction order is therefore:

```text
A
B
```

§ 42(3) A key is registered in destruction order only after its initialization succeeds.

---

## § 43. Thread exit cases

**Governance tags:** `concurrency.thread-local-v2`, `errors.panic-v2`

§ 43(1) TLS destruction runs for normal physical-thread cleanup after:

- normal callable return;
- cooperative cancellation;
- contained panic where thread cleanup is preserved;
- normal foreign `ThreadAttachment` destruction.

§ 43(2) Cleanup is not guaranteed after execution paths that canonically bypass ordinary destruction, such as:

- unsafe hard thread termination;
- process kill;
- hardware reset/power loss;
- fatal runtime/kernel failure.

§ 43(3) Target-specific termination rules may further qualify cleanup guarantees.

---

## § 44. Detached Sec threads

**Governance tags:** `concurrency.thread-local-v2`, `concurrency.thread-v2`

§ 44(1) Detaching a Sec-created physical thread does not detach its Sec thread context while the physical thread is still running.

§ 44(2) The detached lifecycle manager retains metadata required to destroy initialized TLS values at normal physical-thread exit.

§ 44(3) The former source owner has no right to access the detached thread's TLS instances.

§ 44(4) Detach does not convert TLS values into global state.

---

## § 45. `Replace` and destruction order

**Governance tags:** `concurrency.thread-local-v2`, `memory.destruction`

§ 45(1) Replacing a slot does not create a second TLS-key initialization-order registration.

§ 45(2) The key keeps its original first-successful-initialization position.

§ 45(3) Ownership of the old `T` leaves TLS and its later destruction belongs to the receiver of `Replace`.

§ 45(4) The new `T` is the value destroyed at thread TLS cleanup unless replaced again or ownership otherwise changes through a future canonical operation.

---

## § 46. No direct cross-thread TLS access

**Governance tags:** `concurrency.thread-local-v2`

§ 46(1) There is no portable operation:

```text
ThreadLocal[T].BorrowFor(thread)
ThreadLocal[T].Get(threadID)
ThreadLocal[T].SetFor(...)
```

§ 46(2) A thread accesses only its current physical thread's instance.

§ 46(3) Cross-thread communication uses channels, atomics, mutex-protected shared state, IPC where appropriate, or another canonical concurrency mechanism.

---

## § 47. ThreadLocal key references

**Governance tags:** `concurrency.thread-local-v2`

§ 47(1) Source code may reference a static `ThreadLocal[T]` key from any context that is permitted to access TLS.

§ 47(2) Referring to the key does not borrow any particular thread's `T`.

§ 47(3) Only `Borrow`, `BorrowMut`, and `Replace` resolve the current physical thread's slot.

---

## § 48. ISR restriction

**Governance tags:** `concurrency.thread-local-v2`, `analysis.isr-v1`

§ 48(1) Ordinary `ThreadLocal[T]` access is invalid in ISR context.

§ 48(2) An ISR interrupting a physical thread does not execute as ordinary code in that thread's TLS ownership context.

§ 48(3) ISR code must not call:

```text
Borrow
BorrowMut
Replace
Thread.AttachCurrent
```

§ 48(4) Per-core or interrupt-local storage requires a distinct canonical abstraction.

---

## § 49. FFI and raw native TLS

**Governance tags:** `concurrency.thread-local-v2`, `platform.ffi-v1`

§ 49(1) A backend may implement Sec TLS using native TLS primitives.

§ 49(2) This implementation choice does not expose a native TLS key through safe portable Sec.

§ 49(3) Raw native TLS keys/indices/slots belong to target/platform `unsafe` APIs.

§ 49(4) Mapping one native library TLS object to one Sec `ThreadLocal[T]` requires an explicit target/FFI adapter contract.

---

## § 50. Target capacity

**Governance tags:** `concurrency.thread-local-v2`, `compiler.platform-model`

§ 50(1) A selected target may impose limits on:

- TLS key count;
- static TLS bytes;
- alignment;
- dynamic attachment metadata;
- destructor registrations;
- TLS-capable foreign attachments.

§ 50(2) Statically computable limit violations are compile-time diagnostics.

§ 50(3) Runtime foreign attachment resource exhaustion returns:

```sec
Err(ThreadAttachError.ResourceUnavailable)
```

§ 50(4) A target must not silently alias two TLS keys or truncate `T` storage to fit a limit.

---

## § 51. Storage backend freedom

**Governance tags:** `concurrency.thread-local-v2`, `compiler.platform-model`

§ 51(1) Valid implementation strategies include:

```text
compiler-assigned static TLS offsets
ELF/platform TLS
Windows TLS/FLS
RTOS thread-local slots
fields in a Sec thread control block
fixed generation-indexed runtime tables
other equivalent target mechanisms
```

§ 51(2) The selected strategy must preserve all source semantics.

§ 51(3) Different targets may use different strategies without changing `ThreadLocal[T]` source identity.

---

## § 52. Backend invariants

**Governance tags:** `concurrency.thread-local-v2`

§ 52(1) Every backend must preserve at least:

- one per-thread instance per initialized key;
- generation-safe physical-thread identity;
- lazy user initialization;
- exactly-once factory invocation attempt;
- always-initialized state after success;
- thread-bound borrow provenance;
- reverse successful initialization destruction;
- foreign attachment ownership;
- no cross-thread accidental slot reuse.

§ 52(2) Optimizations may remove an unused key/slot only when doing so cannot remove observable initializer/destructor effects.

---

## § 53. Allocation

**Governance tags:** `concurrency.thread-local-v2`, `allocation.general-v2`

§ 53(1) The `ThreadLocal[T]` API itself has no general typed allocation error.

§ 53(2) Static/native TLS backing may be provisioned without dynamic allocation where the target permits.

§ 53(3) A factory may perform allocation only according to its own inferred effects and error handling.

§ 53(4) Foreign `Thread.AttachCurrent()` may fail with `ResourceUnavailable` when runtime attachment metadata cannot be obtained.

§ 53(5) A target/profile forbidding dynamic allocation may reject attachment or factory-backed access paths whose implementation/effects require forbidden allocation.

---

## § 54. Blocking and cancellation

**Governance tags:** `concurrency.thread-local-v2`, `concurrency.blocking-v1`, `concurrency.cancellation-v2`

§ 54(1) `Borrow`, `BorrowMut`, and `Replace` are not intrinsically blocking after successful initialization.

§ 54(2) A first access may inherit blocking/cancellation effects from a factory initializer.

§ 54(3) The TLS mechanism does not invent an independent cancellation result type.

§ 54(4) If current-execution cancellation/panic terminates factory execution before successful initialization, § 13 applies.

§ 54(5) Attachment/detachment runtime work follows the selected target's effect restrictions.

---

## § 55. Memory model

**Governance tags:** `concurrency.thread-local-v2`, `concurrency.memory-model-v2`

§ 55(1) Ordinary TLS access by one physical thread requires no inter-thread synchronization merely to access that thread's private `T`.

§ 55(2) A `T` may itself contain references/capabilities to shared data; those shared accesses remain governed by ordinary memory-model/data-race rules.

§ 55(3) Moving/copying data from TLS into another synchronization mechanism gives only the synchronization guarantees of that mechanism.

§ 55(4) TLS is not a substitute for an atomic, mutex, channel, or other inter-thread synchronization operation.

---

## § 56. Data-race analysis

**Governance tags:** `concurrency.thread-local-v2`, `analysis.data-races`

§ 56(1) Distinct physical threads' per-key `T` objects are distinct storage and do not race with each other solely because they use the same `ThreadLocal[T]` key.

§ 56(2) Shared data reachable from multiple TLS values may still race.

§ 56(3) A raw/unsafe backend representation must not trick analysis into treating distinct TLS instances as one shared object or vice versa.

---

## § 57. Escape analysis

**Governance tags:** `concurrency.thread-local-v2`, `analysis.escape`

§ 57(1) A TLS borrow must not escape to:

- another physical thread;
- another task that may execute elsewhere;
- heap/static storage with wider physical-thread-independent lifetime;
- a returned reference whose caller may use it after migration;
- a detached closure/task/thread that outlives physical-thread identity.

§ 57(2) An owned independent value derived from `T` follows its own ordinary escape rules.

---

## § 58. Attachment-context effect

**Governance tags:** `concurrency.thread-local-v2`, `analysis.effects-v1`

§ 58(1) Functions that directly or transitively access TLS require execution in an attached physical Sec thread context.

§ 58(2) This requirement is compiler-inferred and propagates through ordinary calls/indirect calls according to effect/call-graph rules.

§ 58(3) No source annotation is introduced solely for this requirement in Sec 0.1.

§ 58(4) Generated Sec thread/main entry paths satisfy the requirement by construction.

§ 58(5) FFI callback/export paths must establish attachment before entering code whose inferred requirements need it.

---

## § 59. Semantic analysis

**Governance tags:** `frontend.thread-local-v2`, `analysis.thread-local-v2`

§ 59(1) Sema must track at least:

- TLS key identity;
- concrete `T`;
- initializer kind;
- template/factory ownership;
- initializer effects;
- physical thread/attachment requirement;
- first-initialization possibility;
- initialization-in-progress recursion state where representable;
- successful initialization order semantics;
- TLS borrow provenance;
- borrow mutability;
- task migration/suspension;
- `Replace` ownership;
- attachment token ownership/thread binding;
- foreign callback attachment state;
- ISR context;
- target capacity/capabilities;
- destruction obligations.

§ 59(2) Sema no longer tracks ordinary source move-out/uninitialized TLS slot state because `.Value`/`Take()` do not exist.

§ 59(3) Sema must reject the initial-value constructor for non-copyable `T`.

---

## § 60. Semantic IR

**Governance tags:** `semantic-ir.thread-local-v2`

§ 60(1) Semantic IR must preserve TLS explicitly enough that lowering never reconstructs semantics from arbitrary calls/names.

§ 60(2) Required facts include:

- TLS key identity;
- concrete `T`;
- initializer kind/value/callable identity;
- per-thread generation identity;
- first-access initialization requirement;
- initialization-in-progress state;
- initializer effects;
- borrow/shared/mutable operation;
- replacement ownership transfer;
- thread-bound reference provenance;
- successful initialization-order registration;
- destruction operation;
- foreign attach/detach ownership;
- attachment generation;
- current physical thread context;
- source provenance;
- target TLS requirements.

§ 60(3) Concrete IR opcode names remain owned by `semantic_ir.md`.

§ 60(4) The legacy literal opcode list from the previous thread-local rulebook is not normative.

---

## § 61. IR verification

**Governance tags:** `semantic-ir.thread-local-v2`

§ 61(1) IR verification must reject states that imply:

- one key/thread pair has two simultaneously owned TLS `T` values;
- a successful TLS slot is uninitialized through an ordinary source path;
- a mutable TLS borrow coexists with a conflicting access;
- a TLS borrow changes physical-thread generation;
- one `ThreadAttachment` is destroyed on another physical thread;
- one successful per-thread `T` is destroyed twice;
- a user factory executes twice for one attachment/key pair.

---

## § 62. Lowering

**Governance tags:** `lowering.thread-local-v2`, `compiler.platform-model`

§ 62(1) Lowering consumes validated TLS Semantic IR plus the selected `CompilationPlan`.

§ 62(2) Lowering may select any valid backend in § 51.

§ 62(3) Lowering must preserve:

- lazy user initialization;
- exactly-once factory invocation attempt;
- thread-generation identity;
- always-initialized successful slot invariant;
- exact `Borrow`/`BorrowMut` alias behavior;
- `Replace` ownership;
- reverse successful-init destruction;
- attachment token cleanup;
- effect restrictions.

§ 62(4) Lowering must not reintroduce legacy `.value` semantics.

---

## § 63. LSP hover

**Governance tags:** `tooling.thread-local-v2`

§ 63(1) Hover for `ThreadLocal[T]` must show the exact constructors and methods.

§ 63(2) Hover must not advertise `.Value`/`.value`.

§ 63(3) Hover for `Borrow()` shows:

```sec
fn Borrow() ref T
```

with concrete `T`.

§ 63(4) Hover for `BorrowMut()` shows:

```sec
fn BorrowMut() ref mut T
```

§ 63(5) Hover for `Replace(...)` shows the exact ownership-return shape.

§ 63(6) Hover for `ThreadAttachError` shows exactly three variants with documentation before each variant.

§ 63(7) Hover/navigation for `ThreadAttachment.Context` resolves to canonical `ThreadContext`.

---

## § 64. Completion and navigation

**Governance tags:** `tooling.thread-local-v2`

§ 64(1) Completion on a `ThreadLocal[T]` key offers:

```text
Borrow
BorrowMut
Replace
```

§ 64(2) It does not offer:

```text
Value
value
Take
Get
Set
```

as canonical TLS operations.

§ 64(3) Completion on `Thread` static context may offer:

```text
AttachCurrent
```

where relevant.

§ 64(4) Navigation resolves compiler-known identities to their real canonical declarations.

---

## § 65. Diagnostics

**Governance tags:** `tooling.thread-local-v2`, `frontend.thread-local-v2`

§ 65(1) Suggested diagnostics include:

```text
ThreadLocal[State](initial) requires an implicitly copyable State; use a factory initializer
```

```text
ThreadLocal[T] has no member Value; use Borrow(), BorrowMut(), or Replace(...)
```

```text
reference to thread-local Context cannot remain live across await
```

```text
thread-local mutable borrow cannot remain live across Task.Yield()
```

```text
ThreadAttachment cannot cross a task suspension that may migrate physical threads
```

```text
current physical thread is already attached to Sec
```

```text
thread-local factory recursively accesses the same key during initialization
```

```text
ordinary ThreadLocal[T] access is not permitted in an interrupt routine
```

```text
thread-local storage exceeds selected target capacity
```

§ 65(2) Diagnostics should identify the TLS key, borrow creation point, and invalid suspension/escape point where relevant.

---

## § 66. Restrictions

**Governance tags:** `concurrency.thread-local-v2`

§ 66(1) Sec 0.1 `ThreadLocal[T]` must not:

- expose `.Value`/`.value`;
- return `T` sometimes and `ref T` other times from one property;
- leave a successful slot uninitialized through ordinary source operations;
- expose a general `Take()`;
- run user factories eagerly at thread creation/attachment;
- retry a failed/panicking factory on the same thread attachment;
- share one `T` object between physical threads as the TLS instance;
- let TLS borrows cross unproven task migration;
- allow cross-thread slot lookup through `ThreadID`;
- expose raw native TLS keys in safe portable Sec;
- operate in ISR context;
- use `ThreadContextError`;
- treat `ThreadAttachment` as owning the native thread;
- detach an attachment on a different physical thread;
- convert TLS into task-local state implicitly.

---

## § 67. Explicitly absent Sec 0.1 APIs

**Governance tags:** `concurrency.thread-local-v2`

§ 67(1) Sec 0.1 defines no:

```text
ThreadLocal[T].Value
ThreadLocal[T].value
ThreadLocal[T].Take()
ThreadLocal[T].Get()
ThreadLocal[T].Set(...)
cross-thread TLS lookup
task-local TLS alias
ThreadContextError
ThreadObserver TLS access
ISR-local ThreadLocal[T]
user-defined TLS storage backend
typed TLS initializer error channel
```

---

## § 68. Conformance scenarios

**Governance tags:** `concurrency.thread-local-v2`

§ 68(1) Conformance tests must include at least:

- compiler-known `ThreadLocal[T]` with exact generic arity;
- `@noCopy` ThreadLocal;
- static-key-only declaration;
- copyable template constructor accepted for copyable `T`;
- template constructor rejected for move-only/non-copyable `T`;
- factory constructor works for move-only `T`;
- no `.Value`/`.value`;
- exact Borrow/BorrowMut/Replace signatures;
- first Borrow lazily initializes once;
- first BorrowMut lazily initializes once;
- first Replace initializes then returns old value;
- slot remains initialized after Replace;
- same physical thread sees same slot;
- different physical threads see different slots;
- task migration may observe different slots;
- TLS borrow rejected across await/Task.Yield/waiting select by default;
- physical-thread blocking alone does not invalidate TLS borrow;
- same-key recursive initialization rejected/trapped;
- nested different-key initialization ordering;
- reverse successful initialization destruction;
- detached Sec thread TLS cleanup;
- exact ThreadAttachError variants;
- AttachCurrent AlreadyAttached behavior;
- ThreadAttachment same-thread confinement;
- attachment destruction performs reverse TLS cleanup;
- reattachment creates fresh TLS generation;
- nested FFI callback wrapper does not detach borrowed attachment;
- ThreadSpawnError.ThreadLocalInitializationFailed is infrastructure-only;
- foreign attachment ResourceUnavailable;
- ISR TLS access rejection;
- target TLS capacity rejection;
- LSP exact surface/no legacy value member;
- Semantic IR exactly-once factory/destruction verification.

---

## § 69. Cross-rulebook synchronization

**Governance tags:** `concurrency.thread-local-v2`

§ 69(1) `threads.md` must clarify that `ThreadSpawnError.ThreadLocalInitializationFailed` covers required TLS runtime infrastructure, not user lazy factory failure.

§ 69(2) `threads.md` must expose `Thread.AttachCurrent()` as the exact static operation owned by this rulebook and must not resurrect `ThreadContextError`.

§ 69(3) `scheduling.md` must preserve physical-thread-bound TLS borrow restrictions across task migration/suspension.

§ 69(4) `blocking.md` must distinguish physical thread waiting from migratable logical-task suspension for TLS reference validity.

§ 69(5) `ffi.md` must synchronize generated callback attachment ownership/nesting semantics.

§ 69(6) `static.md` must preserve the distinction between static TLS-key initialization and lazy per-thread `T` initialization.

§ 69(7) `properties.md` requires no TLS special case because `.Value` is removed.

§ 69(8) `borrowing.md`, `lifetime_analysis.md`, and escape analysis must preserve thread-bound provenance.

§ 69(9) `destruction.md` must preserve per-thread reverse successful first-initialization cleanup.

§ 69(10) `semantic_ir.md` remains owner of concrete IR vocabulary.

---

## § 70. Governance

**Governance tags:** `concurrency.thread-local-v2`, `frontend.thread-local-v2`, `analysis.thread-local-v2`, `semantic-ir.thread-local-v2`, `lowering.thread-local-v2`, `tooling.thread-local-v2`, `compiler.platform-model`

§ 70(1) `governance/concurrency_thread_local.yaml` is the sole canonical implementation-status owner for the portable TLS integration defined by this revision.

§ 70(2) Related thread/FFI/scheduling/borrowing/destruction work is cross-linked to its owning fragments rather than duplicated as competing integration entries.

§ 70(3) Root `implementation-status.yaml` is not a second canonical ledger.

§ 70(4) Cross-rulebook synchronization required by this revision is tracked by the accompanying thread-local-v2 correction.

§ 70(5) After synchronization is applied, the correction belongs under `rules/corrections/applied/`.
