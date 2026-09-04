# Applied Correction — Tasks v2 cross-rulebook synchronization

- **Status:** Applied synchronization
- **Created:** 2026-09-04
- **Last updated:** 2026-09-04
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `777beb8`
- **Primary owning rulebook:** `rules/concurrency/tasks.md`
- **Intended correction area:** concurrency, panic/error typing, Semantic IR, core task types, cross-rulebook references
- **Classification:** Normative synchronization of already-decided task semantics
- **Governance fragment:** `implementation-status-tasks.yaml`

---

## 1. Purpose

This correction synchronizes adjacent rulebooks with Tasks revision 2.

It does not introduce new task-language design beyond decisions already made in the owning rulebooks and the Tasks revision 2 rewrite.

The required synchronization is centered on these canonical rules:

```sec
spawn Work()
    // Result[Task[T], TaskSpawnError]

spawn task Work()
    // Result[Task[T], TaskSpawnError]

await worker
    // TaskOutcome[T]
```

and:

```sec
type TaskOutcome[T] union {
    Completed(T)
    Cancelled
    Panicked(PanicInfo)
    Failed(TaskError)
}
```

`TaskError` is the umbrella error type for execution failure of an already-created task.

The complete `TaskError` variant inventory is intentionally not fixed by this correction. Codex must not invent variants.

---

## 2. `rules/concurrency/await.md`

Synchronize all task-specific await rules to the owning task rulebook.

Required changes:

1. Replace any rule stating or implying:

   ```sec
   await Task[T] -> T
   ```

   with:

   ```sec
   await Task[T] -> TaskOutcome[T]
   ```

2. Remove any rule that allows the compiler to change the static result type of the same task-await syntax from `TaskOutcome[T]` to `T` because normal completion can be proven.

3. State explicitly that proof of normal completion may optimize representation or branches but does not alter the static Sec type.

4. State that:

   ```sec
   await Task[void] -> TaskOutcome[void]
   ```

   rather than creating a special direct-`void` rule.

5. Synchronize the task terminal outcome model to:

   ```sec
   Completed(T)
   Cancelled
   Panicked(PanicInfo)
   Failed(TaskError)
   ```

6. Preserve the distinction between task execution outcome and the callable's own return type.

7. Add or preserve the canonical nested example:

   ```sec
   Task[Result[Image, IOError]]
       -> await
       -> TaskOutcome[Result[Image, IOError]]
   ```

8. State explicitly that:

   ```sec
   Completed(Err(IOError.InvalidValue))
   ```

   is normal task completion and is not `Failed(TaskError)`.

9. Update task-creation examples used by the await book to handle fallible spawn, normally:

   ```sec
   let worker := try spawn Work()
   ```

10. Preserve the already-defined distinction between `join` and `await`: task await consumes the owning task handle and transfers the terminal `TaskOutcome[T]`; join preserves the terminal handle for inspection.

11. Update references from `rules/concurrency/tasks.txt` to `rules/concurrency/tasks.md`.

---

## 3. `rules/concurrency/spawn.md`

Synchronize task spawn typing and examples.

Required changes:

1. Both default task spawn and explicit task spawn have raw type:

   ```sec
   Result[Task[T], TaskSpawnError]
   ```

2. Replace examples that directly bind a task handle from spawn without handling the creation result.

   Prefer:

   ```sec
   let worker := try spawn Work()
   ```

   when the surrounding function can propagate `TaskSpawnError`.

3. Helper functions that return a newly spawned task must expose task creation failure rather than pretending task creation is infallible.

4. Preserve the existing rule that lack of task capability on the selected target is a compile-time/target diagnostic rather than a runtime `TaskSpawnError`.

5. Update reusable move-only spawn arguments and captures to ownership-v2 explicit move syntax where transfer is intended:

   ```sec
   let worker := try spawn Use(<-resource)
   ```

6. Do not add an explicit move marker to fresh temporaries when ordinary ownership rules define direct fresh-value transfer.

7. Do not invent a new ownership rule for a move-only argument when spawn evaluation fails before task creation. The exact transfer commit behavior must remain consistent with `ownership.md` and `transferability.md` and should be stated in the owning spawn/ownership rules if further precision is needed.

8. Update references from `rules/concurrency/tasks.txt` to `rules/concurrency/tasks.md`.

---

## 4. `rules/concurrency/cancellation.md`

Synchronize cancellation with the exact task terminal outcome.

Required changes:

1. State that successful cooperative task cancellation produces:

   ```sec
   TaskOutcome[T].Cancelled
   ```

2. Cancellation carries no fabricated `T`.

3. A cancellation request is not itself terminal completion.

4. Preserve the distinction among:

   - normal return,
   - returned `Err(E)`,
   - cancellation,
   - panic,
   - task execution failure.

5. Use the complete task outcome model:

   ```sec
   Completed(T)
   Cancelled
   Panicked(PanicInfo)
   Failed(TaskError)
   ```

6. State that `Completed(Err(E))` is not cancellation and not task failure.

7. Remove any wording that leaves the task-await terminal result type implementation-specific now that `TaskOutcome[T]` is canonical.

8. Update references from `rules/concurrency/tasks.txt` to `rules/concurrency/tasks.md`.

---

## 5. `rules/errors/panic.md`

Synchronize task-boundary panic examples with the complete task outcome.

Required changes:

1. Where task outcome is shown conceptually, use the canonical task shape:

   ```sec
   type TaskOutcome[T] union {
       Completed(T)
       Cancelled
       Panicked(PanicInfo)
       Failed(TaskError)
   }
   ```

2. Preserve `panic.md` as the owner of panic semantics and `PanicInfo`.

3. Preserve target-specific panic policies, including policies where the target cannot recover a task-local panic into an outcome because a harder termination policy applies.

4. Do not merge panic with `TaskError`.

5. Do not merge panic with returned Sec errors.

6. Point the exact task outcome naming and await semantics to `rules/concurrency/tasks.md`.

---

## 6. `rules/concurrency/concurrency_runtime_model.md`

Synchronize core runtime task types with Tasks revision 2.

Required changes:

1. Preserve the already-canonical fallible task-spawn model:

   ```sec
   Result[Task[T], TaskSpawnError]
   ```

2. Add the canonical task terminal outcome:

   ```sec
   type TaskOutcome[T] union {
       Completed(T)
       Cancelled
       Panicked(PanicInfo)
       Failed(TaskError)
   }
   ```

3. State that task result storage preserves the complete declared return type `T`.

4. State that runtime task state must separately preserve normal completion, cancellation, panic, and task execution failure.

5. State that an inner `Result[T, E]` is payload data under `Completed`, not runtime task failure.

6. Keep `TaskSpawnError` for failure before task existence.

7. Add `TaskError` as the named Sec error type for execution failure after successful task creation.

8. Do not invent a complete `TaskError` variant list. Variant design remains open until separately specified.

9. Synchronize error-type syntax with the canonical error rulebook. Concrete task error enums/unions must be declared as Sec error types rather than ordinary non-error enums/unions.

10. Update references from `rules/concurrency/tasks.txt` to `rules/concurrency/tasks.md`.

---

## 7. Core task declarations and `core/errors.sec`

Synchronize the core declarations required by the task semantics.

Required changes:

1. Ensure `TaskSpawnError` is represented as a Sec error type with the already-defined variants:

   ```sec
   enum TaskSpawnError error {
       OutOfMemory
       ResourceLimit
       ExecutorUnavailable
       InvalidConfiguration
       NativeFailure
   }
   ```

2. Ensure the core type surface can represent the canonical generic task outcome:

   ```sec
   type TaskOutcome[T] union {
       Completed(T)
       Cancelled
       Panicked(PanicInfo)
       Failed(TaskError)
   }
   ```

3. Provide a named Sec error type `TaskError` suitable for `Failed(TaskError)`.

4. **Do not invent `TaskError` variants.** If a concrete declaration cannot be completed without at least one variant under the current grammar, record that as an explicit follow-up design/implementation dependency rather than choosing variants on behalf of the language design.

5. Keep `TaskError` distinct from `TaskSpawnError`.

6. Keep both task error types distinct from the `E` inside a task function returning `Result[T, E]`.

---

## 8. `rules/compiler/semantic_ir.md`

Synchronize task semantics in Semantic IR.

Required changes:

1. Task creation must preserve the fallible creation result.

2. Task await must semantically produce:

   ```sec
   TaskOutcome[T]
   ```

   and never a flow-sensitive direct `T`.

3. Semantic IR must preserve distinct terminal categories for:

   - normal completion with `T`,
   - cancellation,
   - panic with `PanicInfo`,
   - execution failure with `TaskError`.

4. Preserve:

   ```sec
   Completed(Err(E))
   ```

   as semantically different from:

   ```sec
   Failed(TaskError)
   ```

5. Preserve ownership transfer of the terminal payload.

6. Preserve task-handle consumption on await and handle preservation on join.

7. Preserve cross-task transferability facts for arguments and captures.

8. Concrete Semantic IR operation naming remains owned by `semantic_ir.md`; do not create a second competing operation vocabulary merely to apply this correction.

---

## 9. `rules/concurrency/concurrency.md` and overview material

Where task semantics are summarized:

1. Show task spawn as fallible.
2. Show task await as `TaskOutcome[T]`.
3. Preserve task/function-result separation.
4. Include panic and execution failure as distinct terminal categories when the overview enumerates task outcomes.
5. Update the canonical task rulebook path to `rules/concurrency/tasks.md`.

Overview text may remain shorter than the owning task rulebook but must not contradict it.

---

## 10. `rules/concurrency/scheduling.md`

No scheduling redesign is required.

Synchronize examples and references only where needed:

1. Task creation examples must remain fallible.
2. Scheduler optimizations must not imply a different source-level await type.
3. Migration must preserve logical task identity and task-local state.
4. Update `tasks.txt` references to `tasks.md`.

---

## 11. `rules/memory/transferability.md`

No transferability redesign is required.

Synchronize task references only where needed:

1. Update the canonical task rulebook path.
2. Ensure task-boundary examples use ownership-v2 explicit moves for reusable move-only sources.
3. Preserve the existing distinction between exclusive ownership transfer and simultaneous shared access.
4. Preserve borrow lifetime, migration, address-stability, and detach restrictions.
5. Do not infer that `TaskOutcome[T]` itself changes the transferability of `T`; normal union/payload ownership rules apply.

---

## 12. `rules/control-flow/discard.md`

No discard redesign is required.

Synchronize the task-specific cross-reference:

1. Replace `rules/concurrency/tasks.txt` with `rules/concurrency/tasks.md`.
2. Preserve the canonical distinction:

   ```sec
   detach worker
   ```

   for the permitted no-result case, and:

   ```sec
   detach worker discard
   ```

   where explicit result disposal is required.

3. Make clear that detach/discard occurs only after successful task creation; it does not discard `TaskSpawnError`.

4. Do not add a special implicit-discard exemption for `TaskOutcome[T]` through this correction.

---

## 13. `rules/concurrency/threads.md`

No thread semantic redesign is required by this correction.

Only reconcile cross-references where the task book is cited.

Keep the existing execution-kind separation:

- `TaskID` and `ThreadID` remain distinct,
- harmonized handle member names do not imply identical types,
- thread spawn/join semantics remain owned by `threads.md`.

---

## 14. Governance synchronization

The accompanying YAML fragment must be reconciled into `implementation-status.yaml`.

Required governance work:

1. Introduce/update integration tag:

   ```text
   concurrency.tasks-v2
   ```

2. Replace canonical rulebook references from:

   ```text
   rules/concurrency/tasks.txt
   ```

   to:

   ```text
   rules/concurrency/tasks.md
   ```

   in task-related integrations.

3. Synchronize the task rulebook status entry from the replaced `.txt` path to the `.md` path.

4. Record current implementation mismatches as implementation debt rather than weakening the normative rule.

5. In particular, governance must explicitly distinguish current implementation from these normative requirements:

   ```text
   spawn task -> Result[Task[T], TaskSpawnError]
   await Task[T] -> TaskOutcome[T]
   ```

6. Do not mark TaskOutcome-v2 complete until frontend, Semantic IR, lowering/runtime, diagnostics, and tests agree on the four terminal categories.

---

## 15. Explicit non-decisions

The following are **not** decided by this correction and must not be invented during synchronization:

1. The complete `TaskError` variant inventory.
2. Any new portable scheduler-state enum beyond the terminal categories required by `TaskOutcome[T]`.
3. New task priority syntax.
4. New task affinity syntax.
5. A new failed-spawn ownership-transfer rule not already derivable from the owning ownership/spawn/transferability rules.
6. New observer privileges.
7. Any flow-sensitive change of the static await type.
8. Any implicit exception mechanism for task failure.

If implementation work exposes a genuine language-design dependency in one of these areas, it must be returned for an explicit design decision.

---

## 16. Out of scope

`rules/concurrency/processes.txt` is not part of this correction.

Its status and rewrite should be handled when the process rulebook itself is taken up. This correction must not classify process semantics as intentionally deferred merely because the current legacy status text does so.
