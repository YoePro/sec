# Sec `spawn` — Normative Modernization Correction

| Field | Value |
| --- | --- |
| **Status** | Normative correction for integration; not yet applied to the canonical rulebook |
| **Created** | 2026-11-09 |
| **Last updated** | 2026-11-09 |
| **Document revision** | 1.0 |
| **Sec language version** | 0.1 |
| **Repository** | `YoePro/sec` |
| **Verified `main` baseline** | `dce854ecb317eb62923c638c23a1baaac354be02` |
| **Correction target** | `rules/concurrency/spawn.md` (existing document revision 1; proposed next revision 2.0) |
| **Correction path** | `rules/corrections/spawn-normative-fix-20261109.md` |
| **Related canonical rulebooks** | `rules/foundations/grammar.md`; `rules/concurrency/tasks.md`; `rules/concurrency/threads.md`; `rules/concurrency/processes.md`; `rules/concurrency/concurrency.md`; `rules/concurrency/await.md`; `rules/concurrency/cancellation.md`; `rules/concurrency/scheduling.md`; `rules/concurrency/concurrency_memory_model.md`; `rules/memory/ownership.md`; `rules/memory/borrowing.md`; `rules/memory/transferability.md`; `rules/compiler/semantic_ir.md`; `rules/platform/target_profiles.md` |
| **Implementation governance** | `governance/concurrency_task.yaml`; `governance/concurrency_thread.yaml`; `governance/concurrency_process.yaml`; `governance/concurrency_model.yaml`; proposed `governance/concurrency_spawn.yaml` |

---

## § 1 Purpose, precedence and integration contract

**§ 1(1)** This correction contains **replacement normative prose**, not a list of defects. Integrate its target clauses directly into `rules/concurrency/spawn.md` using the existing chapter and paragraph identifiers. All target clauses not mentioned here remain unchanged unless a later canonical rule independently supersedes them.

**§ 1(2)** **Never renumber or repurpose any existing target paragraph.** The replacement of `§ N(k)` is placed at `§ N(k)`; an addition receives a previously unused identifier at the end of its existing chapter. Keep existing headings `§ 1`–`§ 29` in their current order, including chapters whose titles are updated for accuracy. Existing source citations must remain valid.

**§ 1(3)** Where the earlier `spawn.md` conflicts with the later specialist language model, the canonical owners are: `grammar.md` § 89 (spawn grammar), `tasks.md` (task creation and ownership), `threads.md` (physical thread creation and deferred start), `processes.md` (isolated process creation and transactional process startup), `cancellation.md` (cancellation), `scheduling.md` (execution and migration), and the canonical ownership/transferability books. `spawn.md` owns the *common spawn boundary*, not duplicate specialist APIs.

**§ 1(4)** Implementation status, code paths, tests, unfinished stages, verification history and work assignments do **not** belong in the updated `spawn.md`. They must be tracked in split governance. No old implementation assumption becomes normative merely because it appeared in document revision 1.

**§ 1(5)** This correction intentionally **does not invent a common failed-creation ownership-rollback guarantee** for task, thread and process. The *process* contract explicitly guarantees transactional startup. Task/thread argument evaluation follows existing ownership rules and the specific task/thread creation contracts. Any remaining ambiguous failure path requires its own separately approved decision rather than an assertion inserted during document cleanup.

---

## § 2 Replacement: target § 2 — Syntax

Replace the unnumbered `spawn-expression` grammar block under target § 2 with the following grammar. This is the grammar-owned model of `rules/foundations/grammar.md` § 89, not a claim about currently completed implementation.

```text
SpawnExpression
    ::= "spawn" [ SpawnKind ] Expression
      | "spawn" [ SpawnKind ] Block

SpawnKind
    ::= Contextual("task")
      | Contextual("thread")
      | Contextual("process")
```

Replace the following existing clauses in target § 2:

**§ 2(2)** A spawn operand may use the canonical expression or block form. When the operand is a block, it denotes the task/thread/process callable body or callable environment as defined by the grammar and the selected execution kind. The resulting callable, its captures and the execution target must satisfy the applicable type, lifetime and transferability rules. No parser or runtime implementation limitation changes this grammar.

**§ 2(3)** Examples of source forms permitted by the spawn grammar, subject to the selected execution-kind semantics:

```sec
spawn Work()
spawn Process(value)
spawn service.Run()
spawn callback(value)
spawn {
    Work()
}
```

**§ 2(4)** `task`, `thread` and `process` are contextual execution-kind modifiers immediately following `spawn`. In this grammar position they select one of the three distinct execution kinds; outside this position, their spelling is interpreted under the ordinary lexical and name-resolution rules.

**§ 2(5)** These modifiers do not introduce three independent general-purpose `spawn` statements, and they are not implicitly recognized as modifiers at unrelated grammar positions.

**§ 2(7)** The `thread` binding in the example at § 2(6) is subject to the normal name and scope rules; the contextual modifier role by itself does not prohibit that identifier.

**§ 2(8)** `spawn` is an expression whose static type is the *fallible creation result* for the selected execution kind. Its use in a larger expression does not discard the creation error or the lifecycle obligations of a successfully produced handle.

Add to target § 2:

**§ 2(9)** In the absence of an explicit `SpawnKind`, the selected execution kind is `task`. Neither the return type of the callable nor the presence of physical worker threads changes this default.

---

## § 3 Replacement: target § 3 — Result type and execution identity

Keep existing § 3(1)–§ 3(12), including the exact raw creation types and the rule that a callable's complete return type `T` remains nested inside the handle.

Replace § 3(13)–§ 3(15) with wording that preserves their original thread-capability focus:

**§ 3(13)** The following source explicitly requests a physical/native thread execution identity:

```sec
spawn thread Work()
```

**§ 3(14)** That request must not be lowered to an ordinary logical `Task[T]` when the selected target cannot support a physical thread. Equally, `spawn process` must not be substituted with task/thread execution: the three creation kinds remain semantically distinct.

**§ 3(15)** An unsupported physical-thread capability is diagnosed during compilation/target validation (for example, `target profile does not support physical threads`), not reported as a runtime `ThreadSpawnError` and not resolved by silently creating a task.

Add to target § 3:

**§ 3(16)** Successful creation produces the appropriate owning handle `Task[T]`, `Thread[T]` or `Process[T]`. A task's eventual `TaskOutcome[T]`, a joined thread's terminal state and a process's completion state are governed by their distinct specialist rulebooks and do not replace the creation-result type.

---

## § 4 Addition: target § 4 — Thread configuration and process distinction

Preserve **all existing** target clauses § 4(1)–§ 4(9). Their original identities already express valid contracts: explicit and inline thread configuration (§ 4(1)–§ 4(3)); rejection of silent `spawn process` substitution (§ 4(4)–§ 4(6)); preservation of execution-kind and process-startup facts in Semantic IR (§ 4(7)–§ 4(9)). Do not move or repurpose those clauses merely because the mixed chapter organization is older.

Add to target § 4:

**§ 4(10)** A named `ThreadConfig` consumed in `spawn thread` obeys normal `<-` rules. Its `Start` member may select `ThreadStartMode.Deferred`, in which case a successfully created `Thread[T]` owns an unstarted thread until `Start()` commits; detailed configuration, storage-borrow and start guarantees belong to `threads.md` §§ 10–21. This does not introduce a deferred `Task[T]` or `Process[T]`.

---

## § 5 Replacement: target § 5 — Common lifecycle ownership

Replace target § 5(3)–§ 5(4):

**§ 5(3)** Before the owning scope ends, each successfully created execution handle must have its lifecycle obligation resolved through an operation **valid for its concrete execution kind**: await where defined, join where defined, explicit detach, transfer of ownership, or another expressly permitted consuming/resolving operation. `await` is not thereby made valid for threads or processes.

**§ 5(4)** The operation's resolved operand type determines its lifecycle behavior. `Task[T]`, `Thread[T]` and `Process[T]` have distinct completion, join, observation, cancellation and ownership rules. Common-looking source syntax such as `join handle` or `detach handle` does not collapse their semantic identities.

---

## § 6 Replacement: target § 6 — Execution start and scheduling

Rename the existing target heading `§ 6 Eager scheduling` to **`§ 6 Creation and start semantics`** without changing its number. Replace § 6(1)–§ 6(5):

**§ 6(1)** `spawn` creates the **selected execution kind**. Task creation and Sec-callable process creation are eager in Sec 0.1. Physical-thread creation is eager by default but may explicitly select `ThreadStartMode.Deferred`.

**§ 6(2)** On successful eager task creation, the task is eligible for scheduling and may be runnable, running, waiting or already terminal. Success does not guarantee it ran before the next source statement.

**§ 6(3)** On successful eager physical-thread creation, the user callable may begin immediately after creation commits and may already have terminated when the caller first observes the handle. On deferred creation, the thread exists in `ThreadStatus.Created` and the user callable must not run before successful `Start()` commit.

**§ 6(4)** On successful `spawn process`, process startup has committed according to `processes.md`: the process identity and child bootstrap are established, the required startup transfer is committed, and the source receives an owning `Process[T]`. The child callable may already have started or completed. No unstarted `Process[T]` or `Process.Start()` is introduced.

**§ 6(5)** The implementation does not guarantee that an eager task, thread or process runs before the next statement in the spawner. It must nevertheless preserve the exact execution-kind, commit, ordering and visibility guarantees of the relevant owner. Deferred threads are the explicit exception to callable start at creation.

Add to target § 6:

**§ 6(6)** Successful creation is not successful completion. Later terminal outcomes, callable-returned `Err(E)`, execution failure, cancellation and panic remain separate from the creation-result error.

---

## § 7 Replacement: target § 7 — Synchronous calls

Preserve target § 7(1)–§ 7(5). Add:

**§ 7(6)** A callable's ordinary, synchronous type and body are not rewritten into a different return type when spawned. The `spawn` boundary supplies the owning execution handle and fallible creation layer; the callable retains its declared return contract.

---

## § 8 Replacement: target § 8 — Current execution context

Rename the target heading `§ 8 Current task context` to **`§ 8 Current execution context`** without renumbering. Replace § 8(1)–§ 8(5):

**§ 8(1)** `spawn task` (including unqualified `spawn`) starts a new logical task context. Code executing as that task may use `Task.Current()` according to the canonical current-task and effect rules.

**§ 8(2)** For a spawned logical task, `Task.Current().CancelRequested` observes **that task's** cooperative cancellation request. The task context follows the logical task during permitted worker migration.

**§ 8(3)** `spawn thread` establishes a distinct physical/native thread execution identity. `Thread.Current()` refers to that physical thread; task context is not fabricated merely because the thread or runtime can execute tasks.

**§ 8(4)** `spawn process` establishes a separate process isolation domain. It does not imply a parent-process task context in the child, nor may a backend silently reuse the parent's `Task.Current()` or `Thread.Current()` identity.

**§ 8(5)** Ordinary calls within an execution context do not themselves spawn another execution entity. A nested `spawn` creates the specifically requested child execution kind; any logical parent-child task relationship is determined by the active task context and the structured-concurrency rules, not by an assumption that every execution context is a task.

---

## § 9 Replacement: target § 9 — Parent and child tasks

Replace § 9(1)–§ 9(4):

**§ 9(1)** When a logical task spawns another logical task, the current logical task may establish the parent-child relationship defined by `tasks.md` § 8 and `concurrency.md` § 19. An ordinary `main`/native-thread execution context has **no implicit root `TaskContext`**, so a task spawned there does not acquire a fictitious current-task parent.

**§ 9(2)** Regardless of logical parenthood, the successful `Task[T]` handle has one owning lifecycle responsibility at the receiving expression, binding or transferred destination.

**§ 9(3)** Parent-child relationships do not duplicate, replace or discharge handle ownership. The same applies when a task is spawned by a callable currently executing on a physical thread.

**§ 9(4)** Every owner must resolve its task lifecycle through the permitted await, join, transfer or detach operations. Structured lifetime and cancellation relationships remain governed by their dedicated rulebooks; detachment does not imply retention of ordinary structured child ownership.

---

## § 10 Replacement: target § 10 — Argument evaluation and transfer boundaries

Preserve § 10(1)–§ 10(5). Replace § 10(6) and add § 10(7)–§ 10(9):

**§ 10(6)** For `spawn process`, consumption of a reusable named source uses `<-` and follows the explicitly **transactional process-startup contract** of `processes.md` § 8: successful startup commits the transfer, whereas any pre-commit `ProcessSpawnError` leaves the named source owned by the parent. Existing effects and successful transfers performed while evaluating nested arguments are not retroactively undone.

**§ 10(7)** For every execution kind, argument, receiver and capture evaluation occurs in the creating execution context in ordinary Sec left-to-right evaluation order before the child callable is permitted to access transferred inputs. Reusable sources are subject to the pending call-transfer reservation, borrow-conflict and explicit-move rules of `ownership.md` §§ 11–14.

**§ 10(8)** Task and thread creation must respect their specialist creation-commit and failure-cleanup contracts; they may not use an undocumented backend shortcut that loses ownership, duplicates destruction, or exposes uninitialized inputs. This paragraph does not add a new, universally identical task/thread/process rollback rule.

**§ 10(9)** Creation and publication must preserve the ordering and memory-visibility guarantees of `concurrency_memory_model.md`. Physical representation or target-specific startup mechanics cannot reorder observable argument evaluation across the source-level spawn boundary.

---

## § 11 Replacement: target § 11 — Ownership transfer

Preserve target § 11(1)–§ 11(4). Add:

**§ 11(5)** A reusable source with move-only ownership requires the normal explicit `<-` marker even when the transfer occurs through a callable receiver, an enclosing closure or a spawned block. A fresh temporary does not require a synthetic marker. A copyable value is copied only when the ordinary parameter contract permits that choice; an explicit consuming `->` parameter consumes even a copyable reusable source.

**§ 11(6)** Moving exclusive ownership across an execution boundary is not simultaneous sharing. It does not by itself establish that the destination is allowed to use a thread-affine, process-local, interrupt-bound or otherwise restricted resource. The selected kind's `transferability.md` proof remains mandatory.

---

## § 12 Replacement: target § 12 — Borrowed arguments

Keep § 12(1)–§ 12(3) as task examples. Add:

**§ 12(4)** Borrowed arguments to a spawned physical thread must meet the thread-boundary transferability and lifetime rules of `threads.md` and `transferability.md`: referent lifetime, address stability, exclusive/shared access rights, storage cleanup and execution affinity must remain valid throughout the borrowed use.

**§ 12(5)** Ordinary `ref T` and `ref mut T` are not transferable as borrowed references into a distinct process. `spawn process` must use a valid process transfer adapter or explicit process-shared capability instead; it may not interpret the parent's address as a child-valid reference.

---

## § 13 Replacement: target § 13 — Mutable borrows

Preserve § 13(1)–§ 13(3) for task borrowing. Add:

**§ 13(4)** `ref mut` transfer to a spawned thread has the same exclusivity and lifetime obligation, plus any physical-thread affinity and backing-storage restrictions. A thread implementation must not release an exclusive backing-storage borrow before the native cleanup boundary established by `threads.md`.

**§ 13(5)** No form of ordinary mutable borrow crosses `spawn process`. A process-shared mutable capability, if permitted, is a different source-level transfer and synchronization contract governed by `processes.md`, `ipc.md` and `transferability.md`.

---

## § 14 Replacement: target § 14 — Escaping tasks

Preserve § 14(1)–§ 14(4). Add:

**§ 14(5)** The escape proof also applies to captures, method receivers, temporaries and indirect referents. A returned or stored task handle may outlive its creator only if every live borrow and its required storage/affinity guarantees remain valid for the entire task use. Moving a handle does not extend the referenced value's lifetime.

---

## § 15 Replacement: target § 15 — Detached tasks

Preserve § 15(1)–§ 15(6). Add:

**§ 15(7)** Detachment is not implicit dropping of a failed creation `Result`, and it cannot discard the obligation to handle spawn failure. Once a task, thread or process is successfully detached under its own rulebook, its runtime cleanup must not retain any source-scoped borrow that can become invalid before completion.

**§ 15(8)** Detachment of a value-returning entity follows the explicit result-discard rules where applicable. A successful detach does not prove that program shutdown will await execution completion; the relevant runtime and target contracts own shutdown behavior.

---

## § 16 Replacement: target § 16 — Captures and callable values

Replace § 16(1)–§ 16(4):

**§ 16(1)** A spawned lambda, block or callable value follows the ordinary closure capture and callable typing rules before its selected execution-kind boundary is validated.

**§ 16(2)** Owned captures are copied or explicitly moved according to their types and the callable/parameter contract. Borrowed captures remain borrows and must not be silently changed into owned copies or heap-promoted to satisfy an otherwise invalid escape.

**§ 16(3)** Captures across a task or thread boundary require the complete boundary-specific transferability, lifetime, storage stability and execution-affinity proof. A moving task must not retain a physical-thread-affine capability across an unproven migration point.

**§ 16(4)** A `MutexGuard[T]` cannot be captured, moved or borrowed into another task contrary to the mutex/transferability restrictions; the restriction is not bypassed by wrapping the guard in a closure. A process-spawned callable environment must instead satisfy process materialization and process transferability, and may not carry ordinary parent-process borrows.

Add:

**§ 16(5)** `spawn process` may use a named function, eligible concrete generic specialization, eligible method, eligible lambda, or runtime-selected callable only when the compiler can establish the complete legal child-process entry and transfer representation defined by `processes.md`. An opaque address or foreign callable pointer does not automatically satisfy that contract.

---

## § 17 Replacement: target § 17 — Spawned methods

Replace § 17(2)–§ 17(6):

**§ 17(2)** Method receiver treatment is determined by the **resolved** receiver mode, the selected execution boundary and the applicable transferability contract.

**§ 17(3)** For task and thread spawn, a by-value receiver is copied or explicitly moved according to Sec ownership rules. Its nested references and thread-affine resources still require independent transferability proof.

**§ 17(4)** For task and thread spawn, a read-only receiver follows the ordinary shared-borrow contract and may cross the boundary only when lifetime, storage stability, aliasing and target constraints permit it.

**§ 17(5)** For task and thread spawn, a mutating receiver follows the ordinary exclusive mutable-borrow contract for the complete concurrent use. The caller may not access conflicting source places while that borrow is live.

**§ 17(6)** For `spawn process`, the complete receiver **value** must be materializable as child-local process state under `processes.md` § 7; the method's code is compiler/linker materialization, not a serialized parent object. No ordinary receiver borrow can refer into the parent process. A copyable receiver may be transferred by copy where permitted; a consumed named receiver requires the ordinary explicit move syntax.

---

## § 18 Replacement: target § 18 — Task handle ownership

Preserve § 18(1), § 18(2) and § 18(4). Replace § 18(3) and § 18(5):

**§ 18(3)** A bare task-spawn expression used only as a statement is invalid when it would silently drop an unresolved fallible creation result or its successful owning lifecycle handle:

```sec
spawn Work()
```

**§ 18(5)** The caller must explicitly handle the creation `Result` and resolve ownership of the successfully returned `Task[T]`. This prevents fire-and-forget behavior arising from a silently discarded creation result; explicit detachment remains available only through the rules for a successfully created handle.

---

## § 19 Replacement: target § 19 — Spawn in expressions

Replace § 19(1)–§ 19(3):

**§ 19(1)** `spawn` may occur in a typed expression context when the enclosing type and error-handling contract accept the *raw creation result* or explicitly handle it. `try spawn Work()` propagates the creation error through the ordinary `try` rules and yields the owning handle on success. This does not consume or flatten the callable's own return type.

```sec
let first := try spawn First()
let second: Task[int] := try spawn Second()
```

**§ 19(2)** A successfully created task may subsequently be awaited through the canonical task-owning operation; its result is `TaskOutcome[T]`, never an automatically flattened bare `T`:

```sec
let task := try spawn Calculate()
let outcome := await task
```

**§ 19(3)** The compiler may optimize representation and execution mechanics, but no optimization may change creation failure, handle ownership, execution-kind identity, argument/capture transfer, cancellation, scheduling, synchronization or observable completion semantics.

---

## § 20 Replacement: target § 20 — Spawn in loops

Replace § 20(2)–§ 20(4):

**§ 20(2)** Each successfully created task retains its lifecycle obligation, including tasks created on different loop iterations. A loop may not silently lose owning handles.

**§ 20(3)** Resource amplification from repeated task creation is validated against the selected target/profile. If the target requires a proven static task-count or storage bound, an unprovable or violated requirement is a compile-time error. Otherwise, demonstrably risky patterns may receive nonmandatory analysis diagnostics; they are not automatically invalid merely because creation occurs in an unbounded loop.

**§ 20(4)** On hosted profiles that permit dynamic task creation, runtime exhaustion uses the exact `TaskSpawnError` creation-failure contract. Target limits and compiler-host resource limits are not interchangeable.

---

## § 21 Replacement: target § 21 — Recursive spawn

Replace § 21(1)–§ 21(3):

**§ 21(1)** Recursive and indirect spawn cycles are permitted when compatible with the selected target's task-resource policy. Sec does not impose a universal static recursion/spawn-count bound on every hosted runtime.

**§ 21(2)** A cycle that creates two child tasks per invocation may cause exponential resource growth. This is an analysis example, **not by itself a universally invalid source construct**. Whether it is rejected depends on the selected profile's provable resource requirements; otherwise resource exhaustion remains a creation-time `TaskSpawnError` possibility.

**§ 21(3)** Diagnostics must distinguish a proven violation of required target resource policy from optional resource-amplification advice. When analysis reports a cycle, it should identify the responsible spawn path and explain the relevant resource constraint, without claiming that ordinary recursion is intrinsically prohibited.

---

## § 22 Replacement: target § 22 — Scheduling

Replace § 22(1)–§ 22(4):

**§ 22(1)** Unqualified `spawn` and `spawn task` select a **logical task**, not a source-visible physical-thread identity. A target may realize tasks on physical worker threads without changing `Task[T]` semantics.

**§ 22(2)** `spawn thread` requests a physical/native thread identity according to `threads.md`; it may not silently become an ordinary task. `spawn process` requests a distinct process isolation domain and may not become an in-process thread or task.

**§ 22(3)** The target may choose the permitted scheduling and execution machinery for each kind, but it must preserve the specified `Task`, `Thread` or `Process` identity, lifecycle, cancellation, affinity and memory-publication semantics.

**§ 22(4)** If the selected target lacks the requested execution capability, compilation/plan validation rejects that request. The compiler must not run the callable synchronously or silently select another execution kind as a fallback.

---

## § 23 Replacement: target § 23 — Blocking behavior

Replace § 23(1)–§ 23(4):

**§ 23(1)** Evaluating a `spawn` expression may execute ordinary argument/receiver/capture preparation and perform the required creation, validation, reservation and startup work for its selected execution kind. Such work may have the blocking effects permitted by the relevant target and blocking-effect contracts.

**§ 23(2)** `spawn` itself does not wait for the spawned callable to reach its terminal result. The explicit wait operations remain `await`, `join`, `select` and other kind-specific interfaces as defined by their owners. For a deferred thread, user code does not begin before explicit `Start()` commit.

**§ 23(3)** Runtime resource exhaustion before successful creation is reported using the kind-specific spawn error where the target supports that kind. Lack of the capability itself is a compile-time/target error.

**§ 23(4)** The compiler must not silently execute the entire callable synchronously as a substitute for unavailable task, thread or process creation, and it must not invent a source-visible lifecycle handle whose underlying execution was not successfully established.

---

## § 24 Replacement: target § 24 — Cancellation inheritance

Replace § 24(1)–§ 24(5):

**§ 24(1)** Any parent-child task cancellation linkage follows `cancellation.md` § 55 and `concurrency.md` §§ 19 and 21. `spawn` does not independently define a second cancellation-propagation system.

**§ 24(2)** Task cancellation, physical-thread cancellation, explicit `Context` cancellation and process termination are distinct. Creating a thread from a task does not automatically give that thread the parent's task cancellation identity. Creating a process does not implicitly convert task cancellation into process termination.

**§ 24(3)** A cancellation request already present before task creation is governed by the canonical structured task and cancellation relationship, including any explicitly passed `Context`. This rulebook does not independently invent a guarantee that the child either must or must not begin in a requested-cancellation state.

**§ 24(4)** A spawn implementation must preserve any cancellation relationship required by the owning task or passed-context contract; it cannot silently clear, fabricate or merge distinct execution identities' cancellation state.

**§ 24(5)** Detailed request propagation, cancellation points, current-execution effects and `Context` semantics belong to `rules/concurrency/cancellation.md`; no reference to a superseded `concurrency.txt` is retained.

---

## § 25 Replacement: target § 25 — Static analysis

Replace § 25(1)–§ 25(2):

**§ 25(1)** The frontend must establish, as applicable to each `spawn` expression:

```text
selected Task / Thread / Process execution kind
resolved callable entry and complete declared return type T
raw Result[Handle[T], SpawnError] type for the selected kind
ordinary operand and argument evaluation order
owned, copied, explicitly moved and borrowed input places
closure captures and method receiver state
pending ownership transfers and applicable commit boundaries
lifetime, address-stability, transferability and affinity proof facts
selected CompilationPlan execution capability and configuration
logical task parenthood only when a real current task context exists
execution-context and cancellation requirements
spawn result owner and its unresolved lifecycle obligation
process materialization and child startup-transfer facts where applicable
source and diagnostic provenance
```

**§ 25(2)** Analysis must preserve each proof only for the validated execution kind and source path. The resulting handle, active borrows, conditional ownership and cancellation effects remain valid only until the applicable completion, join, detach, transfer, failure or cleanup boundary proved by the owning rules. Unknown facts are not silently assumed safe.

---

## § 26 Replacement: target § 26 — Semantic IR

Replace § 26(1)–§ 26(3):

**§ 26(1)** Semantic IR must explicitly preserve the source-level `spawn` operation and distinguish task creation, physical-thread creation and isolated process creation. It must not reconstruct the requested kind from backend calling convention or an inferred worker type.

**§ 26(2)** For every spawn, Semantic IR must retain the selected kind, callable identity, concrete `T` and fallible creation/result identities, evaluated arguments, captured environment, method receiver, ownership preparation/commit facts, retained borrows and transferability proofs, configuration/target-capability decisions, cancellation and execution-context constraints, diagnostic/source provenance and the newly created lifecycle owner. Process-spawn IR must additionally preserve the process-bootstrap and transactional child-local materialization facts of `processes.md`; deferred thread creation must preserve start-mode and unstarted-state facts from `threads.md`.

**§ 26(3)** Lowering must consume these validated facts rather than infer ownership, borrowing, process isolation or physical-thread identity from low-level loads, stores or calls. Any missing required proof at the lowering boundary is an internal compiler defect, not permission to weaken Sec's source semantics.

---

## § 27 Replacement: target § 27 — Diagnostics

Preserve the existing *mentor-oriented* diagnostic examples without treating their exact text as a new diagnostic catalog. Add:

**§ 27(2)** Parser, semantic, ownership, transferability, execution-capability and runtime-creation failures must remain distinguishable. Stable diagnostic IDs, severity, help and source ranges belong to `rules/tooling/diagnostics.md` and its registered catalog, not to a locally invented `spawn` registry.

**§ 27(3)** A statically unsupported task/thread/process capability is reported as a compile-time target/profile violation. A supported execution kind that fails to create at runtime reports its exact typed spawn error. Neither condition may be silently converted into the other.

**§ 27(4)** Where failure-path ownership is involved, diagnostics must identify the reusable source Place, the required explicit `<-`, the selected execution kind and any proven borrow, lifetime or transferability restriction. Diagnostics must not claim an automatic transfer/rollback rule that is not established by the relevant owner.

---

## § 28 Replacement: target § 28 — Restrictions

Preserve § 28(1), including its existing restrictions. Add:

**§ 28(2)** `spawn` must not fabricate `Task.Current()` context for a physical thread or a process; infer a parent task when no current logical task exists; erase deferred thread start mode; interpret ordinary parent-process pointers as child-process references; flatten `TaskOutcome[T]` or callable-returned `Result[T,E]`; or use a common backend path to bypass kind-specific transfer, lifecycle and error guarantees.

**§ 28(3)** Unsafe/FFI implementation machinery cannot silently waive the source-level transferability, ownership or target-isolation requirements that still apply at the selected spawn boundary.

---

## § 29 Replacement: target § 29 — Canonical related-rule ownership

Replace the target § 29(1) block with:

**§ 29(1)** Detailed normative rules are defined by the following canonical owners, each for its own subject:

```text
rules/foundations/grammar.md                    Spawn source grammar
rules/concurrency/tasks.md                      Logical tasks and Task[T]
rules/concurrency/threads.md                    Physical threads, Thread[T], ThreadConfig
rules/concurrency/processes.md                  Process[T], process startup and isolation
rules/concurrency/await.md                      Consuming task await and TaskOutcome[T]
rules/concurrency/concurrency.md                Structured task lifecycle and concurrency invariants
rules/concurrency/cancellation.md               Cancellation, contexts and propagation
rules/concurrency/scheduling.md                 Scheduling, migration and deferred thread start
rules/concurrency/blocking.md                   Suspension and blocking effects
rules/concurrency/concurrency_memory_model.md   Memory publication and completion ordering
rules/memory/ownership.md                       Explicit moves and transfer reservations
rules/memory/borrowing.md                       Borrows, live ranges and alias safety
rules/memory/transferability.md                 Execution-boundary transfer proofs
rules/declarations/static.md                    Static storage and initialization
rules/compiler/semantic_ir.md                   Validated concurrency facts and IR
rules/platform/target_profiles.md               Execution-capability validation
rules/tooling/diagnostics.md                    Diagnostic registry and severities
```

---

## § 30 Governance-only extraction and application instructions

**§ 30(1)** **This entire § 30 is integration guidance for this correction file; do not copy it into `spawn.md`.** The revised normative `spawn.md` must contain neither a current implementation inventory nor instructions to an implementer. Its existing § 2(2) implementation note is replaced above by a grammar contract.

**§ 30(2)** Keep detailed task, thread and process feature status in the already registered integration owners `concurrency.tasks-v2` (`governance/concurrency_task.yaml`), `concurrency.thread-v2` (`governance/concurrency_thread.yaml`) and `concurrency.processes-v2` (`governance/concurrency_process.yaml`). Do not create duplicate copies of their remaining tasks.

**§ 30(3)** The proposed cross-kind fragment `governance/concurrency_spawn.yaml`, if added, must be registered exactly once in `governance/index.yaml` and own only common spawn frontend typing, contextual kind dispatch, operand/block handling, input evaluation, shared transfer-boundary integration, common Semantic IR provenance and cross-kind conformance. If a responsibility already belongs to a specialist fragment, link to it rather than duplicate it.

**§ 30(4)** Preserve the documented **actual implementation status**. For example, `governance/concurrency_task.yaml` still records that raw spawn typing has older direct-`Task[T]` paths, and `governance/concurrency_process.yaml` still records unsupported process execution before complete transfer/materialization support. A normative revision must not silently mark those as implemented.

**§ 30(5)** Update the `spawn.md` header when integrated: set `Last updated` to the actual integration date, advance its document revision, record the verified repository baseline, and link to the owning governance fragments. Keep all original paragraph numbers from `spawn.md` and all existing headings § 1–§ 29; § 30 here is not a new chapter for that book.

**§ 30(6)** Check resulting text against `grammar.md` § 89, `tasks.md` §§ 4–8/23/27, `threads.md` §§ 10–21, `processes.md` §§ 7–8, `cancellation.md` § 55, `scheduling.md` §§ 5–6/21–24, `ownership.md` §§ 11–14 and `transferability.md` §§ 8–10/17–18/24. Replace `.txt` rulebook references with exact canonical `.md` paths; do not renumber stable rule references.

**§ 30(7)** Verify documentation and governance parsing, source-conformance expectations, test coverage, diagnostics and lowering separately. Do not report any implementation as passing merely because this correction has been written. An unresolved semantic choice is recorded as an MD rather than guessed during editing.

---

## § 31 Scope boundary: ownership on failed task/thread creation

**§ 31(1)** **This § 31 is a correction-file boundary note, not a new clause for `spawn.md`.** Process startup transactional transfer is already normative in `processes.md` § 8 and is restated at target § 10(6) above. The general outer-call preparation/commit model is normative in `ownership.md` § 14. `threads.md` § 20 also requires failed thread creation to preserve or clean up ownership according to the canonical transfer transaction.

**§ 31(2)** This correction does not decide whether all pre-commit task/thread creation failures must return **every named spawn input** to the caller under an identical kind-independent rollback policy. Applying that stronger universal guarantee needs an explicit owner decision and coordinated amendments to the task, thread, ownership and transferability rules. Until then, integrators must not infer such a guarantee from the wording in this file.

**§ 31(3)** No other newly unresolved normative choice was required to perform the specified modernization. All other replacement clauses follow already published specialist rules or preserve the existing `spawn.md` source-level guarantees.
