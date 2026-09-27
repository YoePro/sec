# Correction — Scheduling v2 cross-rulebook synchronization

- **Status:** Applied
- **Created:** 2026-09-27
- **Last updated:** 2026-09-27
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `main-reviewed-2026-09-27`
- **Primary owning rulebook:** `rules/concurrency/scheduling.md`
- **Implementation governance:** `governance/concurrency_scheduling.yaml`
- **Classification:** Normative synchronization of scheduling, yield, task migration, and execution-context semantics

---

## 1. Canonical task yield

Add/synchronize:

```sec
impl Task {
    static fn Yield() void
}
```

`Task.Yield()`:
- acts on the current logical task;
- requires a real logical task context;
- keeps the task runnable on normal commit;
- gives the scheduler an opportunity to run another ready task;
- returns `void` after normal resumption;
- guarantees no fairness/context switch/other-task progress;
- creates no memory synchronization edge.

---

## 2. Task.Yield is a cancellation point

Synchronize `tasks.md`, `cancellation.md`, Sema, Semantic IR, lowering, runtime and tests.

`Task.Yield()` is a current-task cancellation point.

```text
cancellation commits first
    Yield does not return to following source code
    ordinary cancellation cleanup runs

normal yield commits first
    task remains runnable
    scheduler receives a scheduling opportunity
    task later resumes and Yield returns void
```

Do not both return normally and execute the pre-commit cancellation path for one yield.

---

## 3. Canonical thread yield

Keep/add:

```sec
impl Thread {
    static fn Yield() void
}
```

`Thread.Yield()`:
- acts on the current physical/native thread;
- is only a scheduler hint;
- is not a Sec cancellation point;
- does not inspect `CancelRequested`;
- creates no memory synchronization edge;
- is invalid in ISR context.

---

## 4. Remove target-dependent implicit root task

Delete legacy wording that program entry may or may not have a task context depending on target profile.

Canonical:

```text
ordinary main
    always has attached physical ThreadContext
    never implicitly has TaskContext
```

Backend use of executor/task machinery for `main` remains invisible to source semantics.

---

## 5. Current context validity

In ordinary `main`:

```sec
Thread.Current() // valid
Task.Current()   // compile-time error
Task.Yield()     // compile-time error
```

`Task.Current()` and `Task.Yield()` become valid only while executing in a real logical task context.

---

## 6. Task migration

Across permitted migration:
- `TaskContext` follows the logical task;
- task identity/cancellation/ownership/continuation state remain stable;
- `ThreadContext` may change;
- physical TLS instance may change.

Thread affinity does not imply task affinity.

---

## 7. Native preemption versus semantic migration

Backend/native preemption is not automatically a source-level task migration point.

If analysis has allowed a physical-thread-bound value/reference to remain live, the backend must preserve the physical-thread identity required by that proof.

Do not resume on another worker across an unmodeled point in a way that invalidates `ThreadLocal` borrow provenance, `ThreadAttachment` confinement, or another execution-affine capability.

---

## 8. Thread-local synchronization

Update `thread_local.md` so `Task.Yield()` is explicitly a migratable task scheduling point.

By default a live `ThreadLocal[T].Borrow()` or `BorrowMut()` reference may not cross it.

Physical-thread park/block alone does not invalidate the TLS borrow when same-thread identity is guaranteed.

---

## 9. Cancellation book

Add `Task.Yield()` to the canonical cancellation-point set.

Do not add `Thread.Yield()`.

```text
Task.Yield
    logical scheduling + cancellation point

Thread.Yield
    physical scheduling hint only
```

---

## 10. Blocking book

Ensure `blocking.md` distinguishes logical task suspension, physical executor-worker blocking, and direct physical-thread parking/blocking.

A task-context operation should suspend the task where target integration permits.

This must not fabricate `TaskContext` in ordinary physical main/thread code.

---

## 11. Select book

No select semantic change.

Preserve source-order priority and cancellation/commit semantics.

Scheduler/native event order must never override source-order branch priority.

---

## 12. Thread priority and affinity

Reuse the exact thread-owned declarations `ThreadPriority`, `CpuSet`, `ThreadSetting[ThreadPriority]`, and `ThreadSetting[CpuSet]`.

Keep explicit `Preferred(...)`, `Required(...)`, and `Default`.

Do not reintroduce implicit plain-value-to-Preferred conversion.

---

## 13. No portable runtime scheduling mutation API

Do not define portable:

```text
Thread[T].SetPriority
Thread[T].SetAffinity
ThreadSchedulingError
```

Target-specific runtime scheduling APIs require complete platform-owned declarations and errors.

Remove/internalize stale compiler-known `ThreadSchedulingError` unless another canonical API takes ownership.

---

## 14. Memory model

Both yield operations create no synchronization edge.

Do not lower either to a memory fence merely to create scheduler visibility.

---

## 15. Fairness and starvation

Do not promise universal task/thread fairness.

Profile-specific stronger guarantees are permitted only when explicitly declared.

Static analysis may diagnose cooperative loops without scheduling/cancellation points, worker-blocking starvation risk, priority/affinity conflicts, and source-order-select starvation risk.

---

## 16. Semantic IR

Preserve at least:
- logical versus physical execution kind;
- task-context availability;
- task scheduling points;
- task migration permission/restriction;
- `Task.Yield()` cancellation commit state;
- `Thread.Yield()` physical hint;
- execution-affine live values;
- target scheduler requirements;
- priority/affinity intent;
- physical blocking versus task suspension;
- source provenance.

Concrete opcode names remain owned by `semantic_ir.md`.

---

## 17. Lowering

`Task.Yield()` lowers to logical scheduler machinery preserving cancellation semantics.

`Thread.Yield()` lowers to a native physical-thread scheduler hint.

Do not lower `Task.Yield()` to `Thread.Yield()` when logical task semantics would be lost.

Do not create hidden `TaskContext` for `main`.

---

## 18. LSP/tooling

Hover on `Task.Yield()` documents logical task, scheduling point, cancellation point, and no synchronization.

Hover on `Thread.Yield()` documents physical thread, scheduler hint, not cancellation-aware, and no synchronization.

Diagnostics must explain that ordinary `main` has `ThreadContext` but no implicit `TaskContext`.

---

## 19. Structured concurrency reference cleanup

`structured_concurrency.md` is Covered rather than an active normative owner.

Remove cross-references that point to it only for lifecycle rules already owned by tasks/await/cancellation/ownership/borrowing/destruction.

Future supervision/task-group semantics belong to a separate dedicated feature/rulebook.

---

## 20. Governance

Populate `governance/concurrency_scheduling.yaml` with `concurrency.scheduling-v2`.

Do not create `implementation-status-scheduling.yaml` and do not duplicate the integration into root `implementation-status.yaml`.

---

## 21. language-rulebook-status.md

After synchronization, mark `concurrency/scheduling.md` synchronized at revision 2.0.

Suggested summary:

```text
Revision 2.0 defines Task.Yield as a cancellation-aware logical scheduling
point, Thread.Yield as a physical hint, explicit migration constraints,
and target-independent main/TaskContext semantics.
```

---

## 22. Required tests

Add/update tests for:
- `Task.Yield()` exact signature/context;
- Task.Yield cancellation first-commit race;
- normal Task.Yield return;
- no yield synchronization edge;
- `Thread.Yield()` exact signature and non-cancellation semantics;
- ISR rejection;
- main `ThreadContext` and no `TaskContext`;
- Task.Current/Task.Yield rejection in main;
- migration/TLS restrictions;
- native preemption preserving execution-affine constraints;
- physical-thread blocking versus task suspension;
- priority/affinity exactness;
- no ThreadSchedulingError;
- select source-order priority;
- Semantic IR distinction;
- LSP hover/diagnostics.
