# Correction — Thread-Local Storage v2 cross-rulebook synchronization

- **Status:** Applied
- **Created:** 2026-09-26
- **Last updated:** 2026-09-27
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `main-reviewed-2026-09-26`
- **Primary owning rulebook:** `rules/concurrency/thread_local.md`
- **Implementation governance:** `governance/concurrency_thread_local.yaml`
- **Classification:** Normative synchronization of TLS access, initialization, attachment, lifetime, and tooling

---

## 1. Remove legacy `.value`

Remove the legacy compiler-known property:

```text
ThreadLocal[T].value
```

and do not replace it with `Value`.

The exact Sec 0.1 access surface is:

```sec
impl ThreadLocal[T] {
    fn Borrow() ref T
    fn BorrowMut() ref mut T
    fn Replace(value: T) T
}
```

Do not preserve contextual behavior where one property access sometimes copies `T`, sometimes yields `ref T`, sometimes yields `ref mut T`, and sometimes moves `T` out.

Do not add `Take()`.

A successfully initialized TLS slot remains initialized until its attachment cleanup destroys it.

---

## 2. Copyable initial-value constructor

Keep:

```sec
init(initial: T)
```

only when concrete `T` is implicitly copyable.

Do not invent a public `Copy` interface solely to express this restriction.

The compiler resolves copyability after concrete generic substitution/monomorphization.

Each physical thread first access copies the stored initializer template.

---

## 3. Factory constructor

Keep:

```sec
init(factory: fn() T)
```

for copyable or move-only `T`.

Factory execution is:
- lazy;
- per physical-thread attachment;
- exactly once on first access;
- never eager merely because a thread was created/attached/started.

The factory has no separate TLS-specific typed error result.

Its normal inferred effects apply to potentially first TLS access.

---

## 4. User initialization is no longer target-timing-dependent

Remove legacy wording that permits user factories to execute either at thread attachment or first access depending on target profile.

Targets may eagerly prepare raw TLS metadata/backing storage.

Targets must not eagerly execute user initialization when that changes observable behavior.

---

## 5. First-access operations

The following trigger first initialization if needed:

```text
Borrow
BorrowMut
Replace
```

`Replace` must first obtain a valid old `T`, then exchange it for the new value and return old ownership.

The TLS slot remains initialized.

---

## 6. Recursive initialization

Track internal per-key/thread state sufficient to identify:

```text
Uninitialized
Initializing
Initialized
InitializationFailed
```

These are implementation states, not a public enum.

Same-key recursive access during `Initializing`:
- compile-time diagnostic when statically proven;
- canonical runtime-check failure otherwise.

Do not recursively call the factory again.

A failed/panicking factory is not retried in the same attachment generation.

---

## 7. Thread-bound borrows

References returned by:

```sec
Borrow()
BorrowMut()
```

carry physical-thread-generation provenance.

Reject them across possible task migration, including by default:
- `await`;
- task-context waiting `join`;
- `Task.Yield()`;
- waiting `select`;
- equivalent task suspension points.

Physical-thread blocking alone does not invalidate the reference when the same physical-thread identity is guaranteed.

---

## 8. Replace move-out tracking removal

Remove legacy Sema/IR state that models:

```text
move out .value
slot becomes uninitialized
later assignment reinitializes
```

That public state no longer exists.

`Replace` owns the exchange semantics instead.

---

## 9. Add exact `ThreadAttachError`

Add:

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

Do not add `NotAttached`.

---

## 10. Add exact `ThreadAttachment`

Add:

```sec
@noCopy
type ThreadAttachment
```

with:

```sec
impl ThreadAttachment {
    property Context: ThreadContext {
        get
    }

    free {
        // Runtime detach/TLS cleanup.
    }
}
```

`free` is the lifecycle member defined by `impl.md`; it is not called manually as `attachment.free()`.

The token owns the Sec attachment, not the native thread.

---

## 11. Replace `Thread.AttachCurrent()`

Replace:

```sec
Thread.AttachCurrent() -> Result[ThreadContext, ThreadContextError]
```

with:

```sec
Thread.AttachCurrent() -> Result[ThreadAttachment, ThreadAttachError]
```

Main and Sec-created threads are already attached and therefore return:

```sec
Err(ThreadAttachError.AlreadyAttached)
```

No user TLS factory is executed by attachment.

---

## 12. Remove `ThreadContextError`

Remove/intern legacy portable:

```sec
enum ThreadContextError {
    NotAttached
    AlreadyAttached
    ResourceUnavailable
    NativeFailure
}
```

Attachment errors now belong to `ThreadAttachError`.

TLS access does not return a context error.

Do not retain the old public identity solely because the compiler already has a symbol for it.

---

## 13. ThreadAttachment confinement

A `ThreadAttachment`:
- may move within the same physical thread;
- may not transfer to another physical thread;
- may not cross task suspension where the task may migrate;
- may not be destroyed on a different physical thread.

This requires ownership/borrow/effect/escape analysis support similar to other execution-affine capabilities.

---

## 14. Detachment cleanup

Destroying `ThreadAttachment`:
1. destroys that attachment generation's successfully initialized TLS values in reverse successful first-initialization order;
2. releases Sec attachment/TLS runtime metadata;
3. ends the current Sec `ThreadContext` attachment.

It does not terminate the native thread.

---

## 15. Reattachment

After normal attachment destruction, the same native foreign thread may attach again.

The new attachment is a new Sec thread-context/TLS generation.

No TLS value from the old generation survives.

---

## 16. FFI callback wrappers

Update FFI-generated callback wrapper semantics.

On callback entry:
- if the current physical thread is already attached, reuse it;
- otherwise auto-attach when target/profile policy supports it.

On callback exit:
- destroy/detach only an attachment created by that wrapper;
- never detach an inherited/nested attachment.

User TLS factories remain lazy even for auto-attached callbacks.

---

## 17. ThreadSpawnError synchronization

Clarify in `threads.md` that:

```sec
ThreadSpawnError.ThreadLocalInitializationFailed
```

means required TLS runtime/target infrastructure could not be established before user callable execution.

It does not mean:
- a `ThreadLocal[T]` user factory returned an error;
- a user factory panicked during later first access.

This keeps thread creation separate from lazy user TLS initialization.

---

## 18. Main and Sec-created threads

Main and `spawn thread` physical threads are attached by construction/runtime startup.

Their user TLS values are still lazy-first-access.

Do not run all registered TLS factories at:
- program startup;
- thread spawn;
- deferred thread Start.

---

## 19. Per-thread destruction order

Change any declaration-order wording to:

```text
reverse successful first-initialization order per physical-thread attachment
```

A never-accessed key has no per-thread value/destructor entry.

Nested initialization records the nested key when it succeeds, then the outer key when it succeeds.

Replacement does not create a second order registration.

---

## 20. Detached threads

Keep TLS metadata/runtime state alive for a detached Sec physical thread until normal detached terminal cleanup destroys that thread's initialized TLS values.

The former source owner gains no TLS access capability to the detached thread.

---

## 21. Static declarations

Synchronize `static.md`:

```sec
static let Counter := ThreadLocal[int](0)
```

creates/owns the TLS key and initializer description in static storage.

It does not eagerly create one `int` for every thread.

Local automatic ThreadLocal keys are not Sec 0.1.

---

## 22. Properties

No property-language change is needed.

`properties.md` must not add a TLS exception for dynamic getter result category.

The removal of `.value` is specifically intended to preserve the canonical property model.

---

## 23. Effects

Potential first access to a factory-backed key carries the factory's inferred effects.

Effect/call-graph analysis must propagate:
- attached-thread-context requirement;
- allocation;
- blocking;
- cancellation;
- panic/other existing inferred effects

as applicable.

A context that forbids an effect may reject TLS access when first initialization remains possible.

---

## 24. ISR

Keep ordinary TLS access invalid in ISR context.

Also reject:

```text
Thread.AttachCurrent()
```

in ISR context.

Do not map ISR execution to the interrupted thread's ordinary TLS semantics.

---

## 25. Target capacity

Target validation may account for:
- TLS key count;
- bytes/alignment;
- destructor registrations;
- runtime foreign attachment resources.

Static over-capacity is a compile-time diagnostic.

Dynamic foreign attachment exhaustion uses:

```sec
ThreadAttachError.ResourceUnavailable
```

---

## 26. Semantic IR

Replace the legacy hard-coded TLS opcode list as a normative source with semantic facts including:
- key identity;
- T;
- initializer kind/value/factory;
- per-thread attachment generation;
- lazy first-access requirement;
- initialization-in-progress state;
- initializer effects;
- Borrow/BorrowMut provenance;
- Replace ownership exchange;
- successful initialization-order registration;
- destruction;
- attachment ownership/generation;
- source/target facts.

Concrete IR names remain owned by `semantic_ir.md`.

---

## 27. LSP

Update hover/completion/navigation:
- show Borrow/BorrowMut/Replace;
- show both exact constructors;
- remove Value/value/Take/Get/Set suggestions;
- show ThreadAttachError exact variants;
- show ThreadAttachment.Context and lifecycle semantics;
- navigate ThreadContext to the canonical thread declaration.

---

## 28. Governance

Populate:

```text
governance/concurrency_thread_local.yaml
```

with:

```text
concurrency.thread-local-v2
```

Do not create a new `implementation-status-thread-local.yaml`.

Do not duplicate the integration into root `implementation-status.yaml`.

---

## 29. language-rulebook-status.md

After applying this correction, change:

```text
concurrency/thread_local.md | Written — sync required
```

to a synchronized revision 2.0 status.

Suggested note:

```text
Revision 2.0 defines explicit Borrow/BorrowMut/Replace TLS access,
deterministic lazy per-thread initialization, thread-bound references,
reverse initialization-order destruction, and owned foreign-thread attachment.
```

---

## 30. Required tests

Add/update tests for:
- no Value/value API;
- exact Borrow/BorrowMut/Replace;
- copyability-gated initial constructor;
- move-only factory constructor;
- lazy exactly-once first initialization;
- no eager factory at attachment/spawn;
- same-key recursive initialization;
- failed-factory no retry;
- slot always initialized after success/Replace;
- physical-thread identity/provenance;
- migration restrictions;
- ThreadAttachError;
- ThreadAttachment same-thread confinement;
- reverse TLS destruction at detach/thread exit;
- reattachment fresh generation;
- nested FFI callback ownership;
- ThreadSpawnError infrastructure-only TLS failure;
- ISR restriction;
- target capacity;
- IR verification;
- LSP surface.
