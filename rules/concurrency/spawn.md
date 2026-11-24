# Spawn

- **Status:** Normative
- **Created:** 2026-07-26
- **Last updated:** 2026-11-24 (amendments MD-128 transactional creation ownership, MD-129 parent cancellation and detach, MD-133 spawn-block result type and MD-138 execution requirements; see the corresponding files in `rules/corrections/applied/`). Previously 2026-11-09 (modernization: execution-kind-specific semantics; see `rules/corrections/applied/spawn-normative-fix-20261109-20261109.md`)
- **Document revision:** 2.1
- **Sec language version:** 0.1 (MD-138 execution requirements: 0.1.1)
- **Canonical path:** `rules/concurrency/spawn.md`
- **Repository baseline reviewed:** `dce854ecb317eb62923c638c23a1baaac354be02`
- **Related governance:** `governance/concurrency_spawn.yaml` (cross-kind spawn boundary), `governance/concurrency_task.yaml`, `governance/concurrency_thread.yaml`, `governance/concurrency_process.yaml`, `governance/concurrency_model.yaml` (`concurrency.model-v2` umbrella)
- **Related rulebooks:** `rules/foundations/grammar.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/processes.md`, `rules/concurrency/await.md`, `rules/concurrency/concurrency.md`, `rules/concurrency/blocking.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/scheduling.md`, `rules/concurrency/concurrency_runtime_model.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/transferability.md`, `rules/analysis/escape_analysis.md`, `rules/compiler/semantic_ir.md`, `rules/platform/target_profiles.md`, `rules/platform/ffi.md`

---

## § 1 Purpose

**§ 1(1)** `spawn` starts an operation as a new execution entity.

**§ 1(2)** It is the explicit boundary between synchronous execution and concurrent
execution.

**§ 1(3)** A normal function call remains synchronous.

```sec
let value := Calculate()
```

**§ 1(4)** A spawned function call creates a task by default.

```sec
let task := try spawn Calculate()
```

**§ 1(5)** This is shorthand for:

```sec
let task := try spawn task Calculate()
```

## § 2 Syntax

```text
SpawnExpression
    ::= "spawn" [ SpawnKind ] Expression
      | "spawn" [ SpawnKind ] Block

SpawnKind
    ::= Contextual("task")
      | Contextual("thread")
      | Contextual("process")
```

**§ 2(1)** Typical use:

```sec
let worker := try spawn Work()
let taskWorker := try spawn task Work()
let threadWorker := try spawn thread Work()
let childProcess := try spawn process Program()
```

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

**§ 2(6)** Example:

```sec
let thread := "worker"
```

**§ 2(7)** The `thread` binding in the example at § 2(6) is subject to the normal name and scope rules; the contextual modifier role by itself does not prohibit that identifier.

**§ 2(8)** `spawn` is an expression whose static type is the *fallible creation result* for the selected execution kind. Its use in a larger expression does not discard the creation error or the lifecycle obligations of a successfully produced handle.

**§ 2(9)** In the absence of an explicit `SpawnKind`, the selected execution kind is `task`. Neither the return type of the callable nor the presence of physical worker threads changes this default.

## § 3 Result type

**§ 3(1)** If the spawned operation returns `T`, these forms have the following conceptual
result types:

```sec
spawn Work()          // Result[Task[T], TaskSpawnError]
spawn task Work()     // Result[Task[T], TaskSpawnError]
spawn thread Work()   // Result[Thread[T], ThreadSpawnError]
spawn process Work()  // Result[Process[T], ProcessSpawnError]
```

**§ 3(2)** `spawn expression` is exactly equivalent to `spawn task expression`.

**§ 3(3)** For every execution kind, `T` is the callable's complete declared normal return
type. A callable returning `Result[U, E]` therefore produces
`Process[Result[U, E]]`; process-creation failure is the outer
`ProcessSpawnError` and is never flattened with the callable result.

**§ 3(4)** Example:

```sec
fn Calculate() int {
    return 42
}

let calculation := try spawn Calculate()
```

**§ 3(5)** `calculation` has type `Task[int]`; the raw spawn expression has type
`Result[Task[int], TaskSpawnError]`.

**§ 3(6)** A function returning `void` through `spawn` or `spawn task` produces
`Result[Task[void], TaskSpawnError]`.

**§ 3(7)** A fallible function preserves its complete return type:

```sec
fn Load() Result[Image, IOError] {
}

let loading := try spawn Load()
```

**§ 3(8)** The successfully created task type is:

```sec
Task[Result[Image, IOError]]
```

**§ 3(9)** `spawn` does not unwrap or alter the function return type.

**§ 3(10)** Task creation failure occurs before a `Task[T]` exists and remains distinct
from every outcome of the successfully created task.

**§ 3(11)** `Task[T]`, `Thread[T]`, and `Process[T]` are distinct handle types.

**§ 3(12)** They are not interchangeable and must not be silently lowered across kinds.

**§ 3(13)** The following source explicitly requests a physical/native thread execution identity:

```sec
spawn thread Work()
```

**§ 3(14)** That request must not be lowered to an ordinary logical `Task[T]` when the selected target cannot support a physical thread. Equally, `spawn process` must not be substituted with task/thread execution: the three creation kinds remain semantically distinct.

**§ 3(15)** An unsupported physical-thread capability is diagnosed during compilation/target validation (for example, `target profile does not support physical threads`), not reported as a runtime `ThreadSpawnError` and not resolved by silently creating a task.

**§ 3(16)** Successful creation produces the appropriate owning handle `Task[T]`, `Thread[T]` or `Process[T]`. A task's eventual `TaskOutcome[T]`, a joined thread's terminal state and a process's completion state are governed by their distinct specialist rulebooks and do not replace the creation-result type.

**§ 3(17)** **Amendment MD-133 (2026-11-24, Sec 0.1 and 0.1.1, `rules/corrections/applied/MD-133-spawn-block-return-type-correction-2026-11-24-20261124.md`).** § 3(3) applies to a spawned call or explicitly typed callable. For a spawn block, `T` is inferred as defined in § 3A.

## § 3A Spawn-block result type (MD-133)

Integrated 2026-11-24 from `rules/corrections/applied/MD-133-spawn-block-return-type-correction-2026-11-24-20261124.md` (owner-approved). It resolves the result semantics of the existing block form for every execution kind; no grammar changes.

**§ 3A(1)** All three creation forms share one syntax and result-typing principle; a kind-specific restriction applies only where its owner defines one. Unqualified `spawn` and `spawn task` are identical. For a spawned call or explicitly typed callable, `T` is its complete declared normal return type (§ 3(3)); for a spawn block, `T` is inferred from the block's explicit return statements and ordinary control-flow analysis of the child body.

| Source form | Raw creation result | Successful payload |
|---|---|---|
| `spawn expression` or `spawn { ... }` | `Result[Task[T], TaskSpawnError]` | `Task[T]` |
| `spawn task expression` or `spawn task { ... }` | `Result[Task[T], TaskSpawnError]` | `Task[T]` |
| `spawn thread expression` or `spawn thread { ... }` | `Result[Thread[T], ThreadSpawnError]` | `Thread[T]` |
| `spawn process expression` or `spawn process { ... }` | `Result[Process[T], ProcessSpawnError]` | `Process[T]` |

**§ 3A(2)** A spawn block is the body of a child callable without declared parameters or return annotation and is a separate callable and control-flow boundary; closure, capture, transferability and execution-context rules apply, and no block annotation or implicit capture form exists. `try` around the spawn handles only the creation result; it changes neither the child's normal return type nor the handling of the child's own `Result`, `Option`, cancellation, panic or failure.

**§ 3A(3)** A value leaves a spawn block as its normal result only through an explicit value-bearing `return`. A final expression statement, including a call, is not an implicit return, and a bare `return` is a void return. A block without a value-bearing return has `T = void`. No tail-expression return rule exists.

**§ 3A(4)** Only returns of the spawn body itself contribute, not those of nested functions, lambdas or spawn bodies; an ordinary nested block, conditional, loop or `match` arm is not a callable boundary. The inferred `T` is the single complete type of the resolved value-bearing return expressions, keeping nominal identities and ordinary type and ownership rules; unused expression statements, local declarations, discarded results and values of nested callables are never candidates. Every reachable normal return path must have the same resolved `T`: no implicit numeric conversion, common supertype, `any` boxing, union synthesis, automatic `Result`/`Option` wrapping or new overload rule reconciles conflicting returns. Literal shaping may use a uniquely established return type. If no unique complete type results, the unresolved return contract is diagnosed with advice to type the returned value or to use an explicitly typed spawned lambda; an unresolved non-void return never defaults to `void`.

**§ 3A(5)** After inference the ordinary return and reachability rules apply (`functions.md` §§ 12–14, `lambda-functions.md` §§ 5, 35): a non-void body returns a compatible value on every reachable path that completes normally, so reachable fallthrough or a bare `return` there is an error; a void body may use a bare `return` or reach its end. A path that does not complete normally (`panic`, a valid `cancel`, proven divergence) needs no fabricated value. A body without a value-bearing return that never completes normally has `T = void`; there is no bottom type and no implied completion. Multiple return values stay unsupported, and a returned `Result[V, E]`, `Option[V]`, reference, named type or callable keeps its complete type subject to the kind's result validity and transferability rules. Inference does not depend on traversal order or optimization and is the same under cold and incremental analysis.

**§ 3A(6)** A `return` in a spawn body exits the child callable, not the spawning function; `break` and `continue` cannot cross the boundary, and defers and local cleanup belong to the child. A returned value follows ordinary return ownership (an owning local transfers without a mandatory `<-`) and must survive child cleanup; no inferred type legalizes an invalid reference or transfer. Explicit-capture rules stay mandatory: a bare spawn block acquires no enclosing local, and a needed capture or explicit result type uses the existing explicit-capture or typed-callable form. A naked `try` in the body is checked against the child's own established `Result` or `Option` channel, never the spawning function's (`errorhandling.md` § 12); no wrapper or error union is inferred from a `try`. The selected kind's result ownership and transfer rules, process result materialization included, MD-128 input ownership, MD-129 cancellation inheritance and the execution requirements of the actual child body (MD-138, § 8(6)) apply unchanged.

```sec
fn Calculate() int {
    return 42
}

let direct := try spawn Calculate()          // Task[int]
let explicit := try spawn {                  // Task[int]
    return Calculate()
}
let ignored := try spawn {                   // Task[void]: the final call result is discarded
    Calculate()
}
let stored := try spawn {                    // Task[void]: a binding is not a return
    let value := Calculate()
}
let computed := try spawn {                  // Task[int]
    return 40 + 2
}
let noResult := try spawn {                  // Task[void]
    return
}
let threadWorker := try spawn thread {       // Thread[int]
    return Calculate()
}
let processWorker := try spawn process {     // Process[int], if process result transfer is valid
    return Calculate()
}
```

**§ 3A(7)** The only intentional asymmetry with ordinary named functions and lambdas is this return-type inference of the existing block form. Ordinary functions and lambdas keep their explicit return type and explicit value-bearing `return`; spawn blocks create no general block expressions or inferred-return lambdas.

**§ 3A(8)** When an otherwise valid void spawn block ends with an ordinary standalone call whose non-void result is implicitly discarded, the existing advisory `A3001` / `ownership.implicit-discarded-result` (`diagnostics.md` § 25(8)) explains that the missing explicit `return` leaves the successful handle as `Task[void]`, `Thread[void]` or `Process[void]`; it keeps its identity, default `Information` severity, policy and deduplication, and no second advisory is emitted. The handle type it names is distinct from the raw creation result `Result[Task[void], TaskSpawnError]` (or its thread/process form). It is not emitted for a void call, an explicit discard, a consumed result, an explicit return, an unresolved or invalid call or a non-void block; a block with earlier value-bearing returns and reachable fallthrough gets its mandatory return-path diagnostic instead. Must-use and non-discardable rules take precedence: an unused `Result` or owning handle is not legalized by being last, and disabling `A3001` legalizes no discard. Inserting `return` is programmer-directed help that may change the handle type, lifecycle and process-transfer requirements, is not offered where the return would violate the kind's requirements, and is never applied by the formatter or a fix-all.

```text
The result of Calculate() (int) is discarded. This spawn block has no
value-bearing return and produces Task[void] on successful creation.

Help: Write 'return Calculate()' if the task should return the calculated
value, or explicitly discard the result if no value is intended.
```

## § 4 Thread configuration

**§ 4(1)** Thread configuration is owned by `threads.md`. A named `ThreadConfig` is
move-only and is consumed with the ordinary move marker:

```sec
let worker := try spawn thread <-config Work()
```

**§ 4(2)** A fresh inline configuration needs no synthetic move marker and uses the exact
CamelCase fields and explicit `ThreadSetting[T]` variants:

```sec
let worker := try spawn thread {
    Name: Preferred("worker")
    Stack: Required(65536)
    Affinity: Preferred(CpuSet { 2, 3 })
    Priority: Preferred(ThreadPriority.High)
    Start: ThreadStartMode.Deferred
} Work()
```

**§ 4(3)** Omitted fields use `ThreadConfig.init()` defaults. Plain values do not
implicitly become `Preferred(...)`.

**§ 4(4)** Invalid lowering:

```sec
spawn process Program()
```

**§ 4(5)** must not become a task or thread.

**§ 4(6)** Expected diagnostic:

```text
target profile does not support process execution
```

**§ 4(7)** The spawn IR must record the requested execution kind:

```text
Task
Thread
Process
```

**§ 4(8)** The backend must not infer execution kind from the called function.

**§ 4(9)** For `Process`, Semantic IR additionally preserves the concrete callable entry,
`Process[T]` and `ProcessSpawnError` result identities, transfer adapters for
arguments/captures/receiver, startup prepare/commit/rollback facts, and source
provenance. It must not retain ordinary cross-process borrow facts or lower the
operation as task/thread execution.

**§ 4(10)** A named `ThreadConfig` consumed in `spawn thread` obeys normal `<-` rules. Its `Start` member may select `ThreadStartMode.Deferred`, in which case a successfully created `Thread[T]` owns an unstarted thread until `Start()` commits; detailed configuration, storage-borrow and start guarantees belong to `threads.md` §§ 10–21. This does not introduce a deferred `Task[T]` or `Process[T]`.

## § 5 Common ownership

**§ 5(1)** All returned execution handles are move-only lifecycle owners unless another
rule explicitly defines a weaker observer type.

**§ 5(2)** A spawned execution entity must not be silently forgotten.

**§ 5(3)** Before the owning scope ends, each successfully created execution handle must have its lifecycle obligation resolved through an operation **valid for its concrete execution kind**: await where defined, join where defined, explicit detach, transfer of ownership, or another expressly permitted consuming/resolving operation. `await` is not thereby made valid for threads or processes.

**§ 5(4)** The operation's resolved operand type determines its lifecycle behavior. `Task[T]`, `Thread[T]` and `Process[T]` have distinct completion, join, observation, cancellation and ownership rules. Common-looking source syntax such as `join handle` or `detach handle` does not collapse their semantic identities.

## § 6 Creation and start semantics

**§ 6(1)** `spawn` creates the **selected execution kind**. Task creation and Sec-callable process creation are eager in Sec 0.1. Physical-thread creation is eager by default but may explicitly select `ThreadStartMode.Deferred`.

**§ 6(2)** On successful eager task creation, the task is eligible for scheduling and may be runnable, running, waiting or already terminal. Success does not guarantee it ran before the next source statement.

**§ 6(3)** On successful eager physical-thread creation, the user callable may begin immediately after creation commits and may already have terminated when the caller first observes the handle. On deferred creation, the thread exists in `ThreadStatus.Created` and the user callable must not run before successful `Start()` commit.

**§ 6(4)** On successful `spawn process`, process startup has committed according to `processes.md`: the process identity and child bootstrap are established, the required startup transfer is committed, and the source receives an owning `Process[T]`. The child callable may already have started or completed. No unstarted `Process[T]` or `Process.Start()` is introduced.

**§ 6(5)** The implementation does not guarantee that an eager task, thread or process runs before the next statement in the spawner. It must nevertheless preserve the exact execution-kind, commit, ordering and visibility guarantees of the relevant owner. Deferred threads are the explicit exception to callable start at creation.

**§ 6(6)** Successful creation is not successful completion. Later terminal outcomes, callable-returned `Err(E)`, execution failure, cancellation and panic remain separate from the creation-result error.

**§ 6(7)** **Amendment MD-128 (2026-11-24, Sec 0.1 and 0.1.1, `rules/corrections/applied/MD-128-spawn-transactional-ownership-correction-2026-11-24-20261124.md` § 2(5)–(6)).** Successful creation establishes the owning handle and a valid execution identity and commits the spawn's ownership transfers (§ 10(8)). The child may run before its creator examines `Ok`, but never before the logical creation commit and the required input publication. The commit is distinct from callable entry, callable completion and result observation, and is not delayed until `await`, `join` or a deferred thread's `Start()`:

| Execution kind | Raw result type | Ownership commit boundary |
|---|---|---|
| `spawn` / `spawn task` | `Result[Task[T], TaskSpawnError]` | successful task creation |
| `spawn thread` | `Result[Thread[T], ThreadSpawnError]` | successful thread creation, deferred creation included |
| `spawn process` | `Result[Process[T], ProcessSpawnError]` | the existing successful process-startup commit |

## § 7 Synchronous calls

**§ 7(1)** Function declarations do not become asynchronous by name or return type.

```sec
let value := CalculateAsync()
```

**§ 7(2)** The suffix `Async` has no language meaning.

**§ 7(3)** Only `spawn` creates a new task:

```sec
let task := try spawn CalculateAsync()
```

**§ 7(4)** Sec does not require `async fn` for functions that may be spawned.

**§ 7(5)** The same function may be called synchronously or spawned.

**§ 7(6)** A callable's ordinary, synchronous type and body are not rewritten into a different return type when spawned. The `spawn` boundary supplies the owning execution handle and fallible creation layer; the callable retains its declared return contract.

## § 8 Current execution context

**§ 8(1)** `spawn task` (including unqualified `spawn`) starts a new logical task context. Code executing as that task may use `Task.Current()` according to the canonical current-task and effect rules.

**§ 8(2)** For a spawned logical task, `Task.Current().CancelRequested` observes **that task's** cooperative cancellation request. The task context follows the logical task during permitted worker migration.

**§ 8(3)** `spawn thread` establishes a distinct physical/native thread execution identity. `Thread.Current()` refers to that physical thread; task context is not fabricated merely because the thread or runtime can execute tasks.

**§ 8(4)** `spawn process` establishes a separate process isolation domain. It does not imply a parent-process task context in the child, nor may a backend silently reuse the parent's `Task.Current()` or `Thread.Current()` identity.

**§ 8(5)** Ordinary calls within an execution context do not themselves spawn another execution entity. A nested `spawn` creates the specifically requested child execution kind; any logical parent-child task relationship is determined by the active task context and the structured-concurrency rules, not by an assumption that every execution context is a task.

**§ 8(6)** **Amendment MD-138 (2026-11-24, Sec 0.1.1, `rules/corrections/applied/MD-138-callable-execution-requirements-v0.1.1-correction-2026-11-24-20261124.md` § 8).** A spawned callable's execution requirements (`rules/concurrency/cancellation.md` § 24A) are checked against the new execution identity and the active target/profile, not propagated to the creator. `spawn` and `spawn task` satisfy `task` and `cancellable`; a `thread` requirement additionally needs proof of a valid physical thread context for the child. `spawn thread` satisfies `thread` and `cancellable`, not `task`. `spawn process` provides neither a logical task nor an eligible cancellable thread by itself, and a `thread` requirement must be proved from the child's bootstrap and target contracts, never from the creator's thread. Eager and deferred thread creation are checked alike. A missing proof is a compile-time diagnostic; no creation-failure error variant is added. Handle types, result types, ownership transfer, scheduling, parentage and cancellation propagation are unchanged.

## § 9 Parent and child tasks

**§ 9(1)** When a logical task spawns another logical task, the current logical task may establish the parent-child relationship defined by `tasks.md` § 8 and `concurrency.md` § 19. An ordinary `main`/native-thread execution context has **no implicit root `TaskContext`**, so a task spawned there does not acquire a fictitious current-task parent.

**§ 9(2)** Regardless of logical parenthood, the successful `Task[T]` handle has one owning lifecycle responsibility at the receiving expression, binding or transferred destination.

**§ 9(3)** Parent-child relationships do not duplicate, replace or discharge handle ownership. The same applies when a task is spawned by a callable currently executing on a physical thread.

**§ 9(4)** Every owner must resolve its task lifecycle through the permitted await, join, transfer or detach operations. Structured lifetime and cancellation relationships remain governed by their dedicated rulebooks; detachment does not imply retention of ordinary structured child ownership.

## § 10 Argument evaluation

**§ 10(1)** Spawn arguments are evaluated in normal Sec evaluation order before ownership is transferred to the new task.

**§ 10(2)** The compiler must preserve normal argument evaluation order.

**§ 10(3)** Successfully evaluated argument values are copied, moved or borrowed according to their types and parameter modes.

**§ 10(4)** The backend must not reorder argument evaluation across the task creation boundary when that changes observable behavior.

**§ 10(5)** For `spawn process`, arguments, captures, and instance-method receiver state
must satisfy the canonical `ProcessTransferable` proof or adapter from
`rules/memory/transferability.md`. Ordinary `ref T` and `ref mut T` never
cross the process boundary, and a numeric `RawPtr[T]` does not become a
child-valid pointer merely by transfer. Transferred receiver state becomes
child-local; method code is compiler/linker materialization rather than runtime
object payload.

**§ 10(6)** For `spawn process`, consumption of a reusable named source uses `<-` and follows the explicitly **transactional process-startup contract** of `processes.md` § 8: successful startup commits the transfer, whereas any pre-commit `ProcessSpawnError` leaves the named source owned by the parent. Existing effects and successful transfers performed while evaluating nested arguments are not retroactively undone.

From MD-128 (2026-11-24) the same transaction applies to every execution kind (§ 10(8)); `processes.md` §§ 8.6–8.8 remain its precedent and the process-specific bootstrap rules.

**§ 10(7)** For every execution kind, argument, receiver and capture evaluation occurs in the creating execution context in ordinary Sec left-to-right evaluation order before the child callable is permitted to access transferred inputs. Reusable sources are subject to the pending call-transfer reservation, borrow-conflict and explicit-move rules of `ownership.md` §§ 11–14.

**§ 10(8)** **Amendment MD-128 (2026-11-24, Sec 0.1 and 0.1.1, `rules/corrections/applied/MD-128-spawn-transactional-ownership-correction-2026-11-24-20261124.md` §§ 1–3); this paragraph replaces the earlier deferral of a common rule.** `spawn task`, `spawn thread` and `spawn process` share one transactional ownership semantics, and unqualified `spawn` is `spawn task`. A spawn operation has one logical creation transaction covering its pending transfers of reusable source Places and the preparation of owned values, borrowed inputs, receiver and environment state and creation resources; it is semantic, not a mandated runtime mechanism.

- A pending transfer of a reusable source does not change its ownership yet; its reservation prevents conflicting access (`ownership.md` §§ 11–14) and gives no permission to destroy, duplicate or independently use the source.
- Successful creation commits every transfer of the transaction exactly once: the consumed source Places become unavailable and the new execution owns the inputs. Permitted copies stay copies and borrows stay borrows with their boundary-specific lifetime and exclusivity obligations.
- `Err(...)` from creation means no commit: every pending transfer of a reusable caller-owned source is cancelled and that source stays owned and available to the caller. The transaction releases its own reservations and the temporary borrow obligations that would have belonged to the child, never unrelated or pre-existing loans.
- Creation-only temporary resources and caller-owned temporary arguments or environments get ordinary failure-path cleanup. No uncommitted reusable source is destroyed because creation failed, and rollback creates no second owner or duplicate destructor.
- A provisionally created native object, control structure or bootstrap child must not run user code with uncommitted inputs and is cleaned up on failure; a reported failure leaves no live unowned child, hidden lifecycle obligation, unreleased creation resource or competing owner (process bootstrap: `processes.md` § 8.8 in full).
- Creation failure is not a post-commit failure: panic, cancellation, native execution failure, a returned `Err(E)` or a failed `Start()` of an already created deferred thread never roll back the commit.
- Unsupported capabilities, statically invalid configuration, illegal transfers and invalid borrows stay compile-time diagnostics and are never converted into runtime spawn errors.

No backend shortcut may lose ownership, duplicate destruction or expose uninitialized inputs. A runtime strategy that cannot satisfy the transaction is not a valid implementation of that spawn form.

**§ 10(9)** Creation and publication must preserve the ordering and memory-visibility guarantees of `concurrency_memory_model.md`. Physical representation or target-specific startup mechanics cannot reorder observable argument evaluation across the source-level spawn boundary.

**§ 10(10)** **Amendment MD-128 (2026-11-24, Sec 0.1 and 0.1.1, `rules/corrections/applied/MD-128-spawn-transactional-ownership-correction-2026-11-24-20261124.md` § 4).** Only pending transfers of the current creation transaction are reversible. Independently completed nested calls, earlier moves, mutations, I/O and other observable effects are not undone. A resource already moved into an independently constructed owner (closure environment, receiver or configuration) is not restored to its earlier source, while a pending direct transfer of that owner into the new execution is cancelled on failure; temporary owners are cleaned up by their actual ownership state, and `ThreadConfig` failure cleanup applies only to configuration state owned by the failed creation machinery. When a later argument fails before creation is attempted, the outer-call preparation and failure-cleanup rules apply and that failure is never relabelled as a spawn creation error.

**§ 10(11)** **Amendment MD-128 (2026-11-24, Sec 0.1 and 0.1.1, `rules/corrections/applied/MD-128-spawn-transactional-ownership-correction-2026-11-24-20261124.md` § 6(1)–(2)).** A cancellation request already present in the parent is not by itself a creation error (MD-129); a genuine creation error still follows § 10(8). If existing cancellation rules stop an operation before the creation commit, cleanup follows the uncommitted path; after the commit the handle and committed inputs follow ordinary lifecycle and cancellation cleanup. `spawn` does not become a cancellation point.

## § 11 Ownership transfer

**§ 11(1)** Arguments passed to owned parameters follow normal copy and move rules.

```sec
fn Process(data: Data) void {
}

let data := Data.Create()
let worker := try spawn Process(<-data)
```

**§ 11(2)** If `Data` is move-only:

- ownership of `data` moves into the task
- the caller may not use `data` afterward
- the task owns the value until it transfers or destroys it

**§ 11(3)** If `Data` is copyable, the argument may be copied according to normal Sec rules.

**§ 11(4)** `spawn` must not silently clone move-only values.

**§ 11(5)** A reusable source with move-only ownership requires the normal explicit `<-` marker even when the transfer occurs through a callable receiver, an enclosing closure or a spawned block. A fresh temporary does not require a synthetic marker. A copyable value is copied only when the ordinary parameter contract permits that choice; an explicit consuming `->` parameter consumes even a copyable reusable source.

**§ 11(6)** Moving exclusive ownership across an execution boundary is not simultaneous sharing. It does not by itself establish that the destination is allowed to use a thread-affine, process-local, interrupt-bound or otherwise restricted resource. The selected kind's `transferability.md` proof remains mandatory.

**§ 11(7)** **Amendment MD-128 (2026-11-24, Sec 0.1 and 0.1.1, `rules/corrections/applied/MD-128-spawn-transactional-ownership-correction-2026-11-24-20261124.md` § 5).** The transfers described in § 11(2) take effect on successful creation. On a creation `Err` the source stays owned:

```sec
match spawn task Work(<-resource) {
    Ok(worker) => {
        // resource is unavailable here.
        let outcome := await worker
        HandleOutcome(outcome)
    }
    Err(error) => {
        // resource remains owned and available here.
        HandleCreationFailure(error)
        UseResource(ref resource)
    }
}
```

Thread and process creation change the handle, error and lifecycle resolution but not this conditional availability. With `let worker := try spawn Work(<-resource)` the success continuation has an unavailable `resource` and an owning handle; on a propagated creation failure `resource` stays with its owner and gets ordinary cleanup, and no synthetic value restores it. Where success and failure paths merge, ordinary ownership-state merging applies: a source consumed on one predecessor is not unconditionally available, and no implicit reinitialization or duplication exists.

## § 12 Borrowed arguments

**§ 12(1)** A spawned task may receive borrowed arguments when the compiler proves that the borrowed value remains valid until the task completes.

```sec
fn Inspect(data: ref Data) void {
}

fn Run() void {
    let data := Data.Create()
    let worker := try spawn Inspect(ref data)
    let outcome := await worker
    Handle(outcome)
}
```

**§ 12(2)** The borrow remains active while the task may use it.

**§ 12(3)** The owner may not be destroyed, moved, mutably borrowed in conflict or mutated in conflict until the task has completed.

**§ 12(4)** Borrowed arguments to a spawned physical thread must meet the thread-boundary transferability and lifetime rules of `threads.md` and `transferability.md`: referent lifetime, address stability, exclusive/shared access rights, storage cleanup and execution affinity must remain valid throughout the borrowed use.

**§ 12(5)** Ordinary `ref T` and `ref mut T` are not transferable as borrowed references into a distinct process. `spawn process` must use a valid process transfer adapter or explicit process-shared capability instead; it may not interpret the parent's address as a child-valid reference.

## § 13 Mutable borrows

**§ 13(1)** A `ref mut` argument gives the spawned task exclusive access for the duration of the borrow.

```sec
fn Update(data: ref mut Data) void {
}

let mut data := Data.Create()
let worker := try spawn Update(ref mut data)
```

**§ 13(2)** While the task may use the mutable borrow, the parent task must not access `data` directly or create another overlapping borrow.

**§ 13(3)** The compiler must track this borrow until task completion is established.

**§ 13(4)** `ref mut` transfer to a spawned thread has the same exclusivity and lifetime obligation, plus any physical-thread affinity and backing-storage restrictions. A thread implementation must not release an exclusive backing-storage borrow before the native cleanup boundary established by `threads.md`.

**§ 13(5)** **Amendment MD-128 (2026-11-24, Sec 0.1 and 0.1.1, `rules/corrections/applied/MD-128-spawn-transactional-ownership-correction-2026-11-24-20261124.md` §§ 2(4), 3(2)).** The borrow obligations of §§ 12–13 begin at successful creation. A failed creation releases the loans that belonged to the transaction and leaves unrelated and pre-existing loans unchanged; a successful one keeps its lifetime and exclusivity obligation until its canonical end.

**§ 13(5)** No form of ordinary mutable borrow crosses `spawn process`. A process-shared mutable capability, if permitted, is a different source-level transfer and synchronization contract governed by `processes.md`, `ipc.md` and `transferability.md`.

## § 14 Escaping tasks

**§ 14(1)** A task that borrows scope-owned data may not outlive that data.

**§ 14(2)** Invalid:

```sec
fn Invalid() Task[void] {
    let data := Data.Create()
    let worker := try spawn Inspect(ref data)
    return worker
}
```

**§ 14(3)** Expected diagnostic:

```text
task worker cannot escape while borrowing local value data
```

**§ 14(4)** The same rule applies to returning, storing, transferring or detaching the task.

**§ 14(5)** The escape proof also applies to captures, method receivers, temporaries and indirect referents. A returned or stored task handle may outlive its creator only if every live borrow and its required storage/affinity guarantees remain valid for the entire task use. Moving a handle does not extend the referenced value's lifetime.

## § 15 Detached tasks

**§ 15(1)** A detached task may not retain references to scope-owned data.

**§ 15(2)** Invalid:

```sec
fn Invalid() void {
    let data := Data.Create()
    let worker := try spawn Inspect(ref data)
    detach worker
}
```

**§ 15(3)** Expected diagnostic:

```text
detached task worker cannot retain reference to local value data
```

**§ 15(4)** A detached task should normally own its required data:

```sec
fn Valid() void {
    let data := Data.Create()
    let worker := try spawn Process(<-data)
    detach worker
}
```

**§ 15(5)** Static or program-owned references still require valid concurrency-safe access.

**§ 15(6)** Static lifetime alone does not make shared mutation safe.

**§ 15(7)** Detachment is not implicit dropping of a failed creation `Result`, and it cannot discard the obligation to handle spawn failure. Once a task, thread or process is successfully detached under its own rulebook, its runtime cleanup must not retain any source-scoped borrow that can become invalid before completion.

**§ 15(8)** Detachment of a value-returning entity follows the explicit result-discard rules where applicable. A successful detach does not prove that program shutdown will await execution completion; the relevant runtime and target contracts own shutdown behavior.

## § 16 Captures and callable values

**§ 16(1)** A spawned lambda, block or callable value follows the ordinary closure capture and callable typing rules before its selected execution-kind boundary is validated.

**§ 16(2)** Owned captures are copied or explicitly moved according to their types and the callable/parameter contract. Borrowed captures remain borrows and must not be silently changed into owned copies or heap-promoted to satisfy an otherwise invalid escape.

**§ 16(3)** Captures across a task or thread boundary require the complete boundary-specific transferability, lifetime, storage stability and execution-affinity proof. A moving task must not retain a physical-thread-affine capability across an unproven migration point.

**§ 16(4)** A `MutexGuard[T]` cannot be captured, moved or borrowed into another task contrary to the mutex/transferability restrictions; the restriction is not bypassed by wrapping the guard in a closure. A process-spawned callable environment must instead satisfy process materialization and process transferability, and may not carry ordinary parent-process borrows.

**§ 16(5)** `spawn process` may use a named function, eligible concrete generic specialization, eligible method, eligible lambda, or runtime-selected callable only when the compiler can establish the complete legal child-process entry and transfer representation defined by `processes.md`. An opaque address or foreign callable pointer does not automatically satisfy that contract.

**§ 16(6)** **Amendment MD-138 (2026-11-24, Sec 0.1.1, `rules/corrections/applied/MD-138-callable-execution-requirements-v0.1.1-correction-2026-11-24-20261124.md` § 8(4)).** Spawn arguments and callable and capture preparation evaluated in the creator keep their own execution requirements, which the creator must satisfy; the child's context cannot satisfy them. Operations inside a spawned block's own callable body belong to the child invocation.

**§ 16(7)** **Amendment MD-128 (2026-11-24, Sec 0.1 and 0.1.1, `rules/corrections/applied/MD-128-spawn-transactional-ownership-correction-2026-11-24-20261124.md` §§ 4(2), 5(5)).** Constructing a closure environment is an independently completed operation: a capture it consumed is not restored by a later failed spawn. When the spawn consumes a reusable closure value, that transfer is pending in the creation transaction like any other reusable owned input and is cancelled on failure, leaving the closure owner and its capture state intact.

```sec
let callback := capture(<-resource) fn() void {
    UseResource(ref resource)
}
// resource was consumed by closure construction; a failed spawn of callback cannot restore it.
```

## § 17 Spawned methods

**§ 17(1)** Methods may be spawned.

```sec
let worker := try spawn service.Run()
```

**§ 17(2)** Method receiver treatment is determined by the **resolved** receiver mode, the selected execution boundary and the applicable transferability contract.

**§ 17(3)** For task and thread spawn, a by-value receiver is copied or explicitly moved according to Sec ownership rules. Its nested references and thread-affine resources still require independent transferability proof.

**§ 17(4)** For task and thread spawn, a read-only receiver follows the ordinary shared-borrow contract and may cross the boundary only when lifetime, storage stability, aliasing and target constraints permit it.

**§ 17(5)** For task and thread spawn, a mutating receiver follows the ordinary exclusive mutable-borrow contract for the complete concurrent use. The caller may not access conflicting source places while that borrow is live.

**§ 17(6)** For `spawn process`, the complete receiver **value** must be materializable as child-local process state under `processes.md` § 7; the method's code is compiler/linker materialization, not a serialized parent object. No ordinary receiver borrow can refer into the parent process. A copyable receiver may be transferred by copy where permitted; a consumed named receiver requires the ordinary explicit move syntax.

## § 18 Task handle ownership

**§ 18(1)** The raw result of task `spawn` is
`Result[Task[T], TaskSpawnError]`. Its successful payload is a move-only
`Task[T]` lifecycle owner.

**§ 18(2)** The task handle must be stored, returned, passed to another owner, awaited, joined or detached explicitly.

**§ 18(3)** A bare task-spawn expression used only as a statement is invalid when it would silently drop an unresolved fallible creation result or its successful owning lifecycle handle:

```sec
spawn Work()
```

**§ 18(4)** Expected diagnostic:

```text
spawned task result must be owned, awaited, joined or detached
```

**§ 18(5)** The caller must explicitly handle the creation `Result` and resolve ownership of the successfully returned `Task[T]`. This prevents fire-and-forget behavior arising from a silently discarded creation result; explicit detachment remains available only through the rules for a successfully created handle.

## § 19 Spawn in expressions

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

## § 20 Spawn in loops

**§ 20(1)** Spawning inside loops is allowed.

**§ 20(2)** Each successfully created task retains its lifecycle obligation, including tasks created on different loop iterations. A loop may not silently lose owning handles.

**§ 20(3)** Resource amplification from repeated task creation is validated against the selected target/profile. If the target requires a proven static task-count or storage bound, an unprovable or violated requirement is a compile-time error. Otherwise, demonstrably risky patterns may receive nonmandatory analysis diagnostics; they are not automatically invalid merely because creation occurs in an unbounded loop.

**§ 20(4)** On hosted profiles that permit dynamic task creation, runtime exhaustion uses the exact `TaskSpawnError` creation-failure contract. Target limits and compiler-host resource limits are not interchangeable.

## § 21 Recursive spawn

**§ 21(1)** Recursive and indirect spawn cycles are permitted when compatible with the selected target's task-resource policy. Sec does not impose a universal static recursion/spawn-count bound on every hosted runtime.

**§ 21(2)** A cycle that creates two child tasks per invocation may cause exponential resource growth. This is an analysis example, **not by itself a universally invalid source construct**. Whether it is rejected depends on the selected profile's provable resource requirements; otherwise resource exhaustion remains a creation-time `TaskSpawnError` possibility.

**§ 21(3)** Diagnostics must distinguish a proven violation of required target resource policy from optional resource-amplification advice. When analysis reports a cycle, it should identify the responsible spawn path and explain the relevant resource constraint, without claiming that ordinary recursion is intrinsically prohibited.

## § 22 Scheduling

**§ 22(1)** Unqualified `spawn` and `spawn task` select a **logical task**, not a source-visible physical-thread identity. A target may realize tasks on physical worker threads without changing `Task[T]` semantics.

**§ 22(2)** `spawn thread` requests a physical/native thread identity according to `threads.md`; it may not silently become an ordinary task. `spawn process` requests a distinct process isolation domain and may not become an in-process thread or task.

**§ 22(3)** The target may choose the permitted scheduling and execution machinery for each kind, but it must preserve the specified `Task`, `Thread` or `Process` identity, lifecycle, cancellation, affinity and memory-publication semantics.

**§ 22(4)** If the selected target lacks the requested execution capability, compilation/plan validation rejects that request. The compiler must not run the callable synchronously or silently select another execution kind as a fallback.

## § 23 Blocking behavior

**§ 23(1)** Evaluating a `spawn` expression may execute ordinary argument/receiver/capture preparation and perform the required creation, validation, reservation and startup work for its selected execution kind. Such work may have the blocking effects permitted by the relevant target and blocking-effect contracts.

**§ 23(2)** `spawn` itself does not wait for the spawned callable to reach its terminal result. The explicit wait operations remain `await`, `join`, `select` and other kind-specific interfaces as defined by their owners. For a deferred thread, user code does not begin before explicit `Start()` commit.

**§ 23(3)** Runtime resource exhaustion before successful creation is reported using the kind-specific spawn error where the target supports that kind. Lack of the capability itself is a compile-time/target error.

**§ 23(4)** The compiler must not silently execute the entire callable synchronously as a substitute for unavailable task, thread or process creation, and it must not invent a source-visible lifecycle handle whose underlying execution was not successfully established.

## § 24 Cancellation inheritance

**§ 24(1)** Any parent-child task cancellation linkage follows `cancellation.md` § 55 and `concurrency.md` §§ 19 and 21. `spawn` does not independently define a second cancellation-propagation system.

**§ 24(2)** Task cancellation, physical-thread cancellation, explicit `Context` cancellation and process termination are distinct. Creating a thread from a task does not automatically give that thread the parent's task cancellation identity. Creating a process does not implicitly convert task cancellation into process termination.

**§ 24(3)** **Amendment MD-129 (2026-11-24, Sec 0.1 and later compatible versions, `rules/corrections/applied/MD-129-spawn-parent-cancellation-correction-2026-11-24-20261124.md`); this paragraph replaces the earlier deferral.** A cancellation request already present on the parent is not a creation error. A successfully created child with a structured parent-child cancellation link begins with its own request pending before it can observe its state; a child without such a link inherits nothing. A later detach breaks future propagation without revoking a received request. The canonical rules, including race ordering, are `cancellation.md` §§ 55(5)–(9).

**§ 24(4)** A spawn implementation must preserve any cancellation relationship required by the owning task or passed-context contract; it cannot silently clear, fabricate or merge distinct execution identities' cancellation state.

**§ 24(5)** Detailed request propagation, cancellation points, current-execution effects and `Context` semantics belong to `rules/concurrency/cancellation.md`; no reference to a superseded `concurrency.txt` is retained.

## § 25 Static analysis

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

**§ 25(3)** **Amendment MD-138 (2026-11-24, Sec 0.1.1, `rules/corrections/applied/MD-138-callable-execution-requirements-v0.1.1-correction-2026-11-24-20261124.md` §§ 8, 10(2)).** The "execution-context and cancellation requirements" of § 25(1) include the spawned callable's normalized `require` contract validated against the child identity (§ 8(6)) separately from the creator-side requirements of its arguments (§ 16(6)).

**§ 25(5)** **Amendment MD-133 (2026-11-24, Sec 0.1 and 0.1.1, `rules/corrections/applied/MD-133-spawn-block-return-type-correction-2026-11-24-20261124.md` § 6(1)–(2), (5)).** Parsing keeps the distinction between a spawn call and a spawn block and preserves return and expression-statement nodes, spans, the explicit kind and nested control flow; a final expression is never rewritten into a return during parsing or recovery. Sema resolves the child boundary, every return candidate and the complete `T` before forming the handle type, and validates kind, captures and transferability, effects, return paths, error channels and use of the raw creation result. LSP hover and semantic queries show the inferred child return type with the correct raw and handle types; the formatter never changes the presence or absence of returns.

**§ 25(4)** **Amendment MD-128 (2026-11-24, Sec 0.1 and 0.1.1, `rules/corrections/applied/MD-128-spawn-transactional-ownership-correction-2026-11-24-20261124.md` § 7(1), (5)).** Analysis distinguishes prepared transfers, committed transfers and independently completed moves, and establishes on each creation-result path the correct Place availability, owner identity, active loans and lifecycle obligation through `match`, `try` handlers, propagated errors and joins. Diagnostics and LSP ownership information distinguish a move pending on a creation result from a committed one, and never suggest that a creation `Err` destroyed a directly prepared reusable source or restored an earlier consumed capture.

## § 26 Semantic IR

**§ 26(1)** Semantic IR must explicitly preserve the source-level `spawn` operation and distinguish task creation, physical-thread creation and isolated process creation. It must not reconstruct the requested kind from backend calling convention or an inferred worker type.

**§ 26(2)** For every spawn, Semantic IR must retain the selected kind, callable identity, concrete `T` and fallible creation/result identities, evaluated arguments, captured environment, method receiver, ownership preparation/commit facts, retained borrows and transferability proofs, configuration/target-capability decisions, cancellation and execution-context constraints, diagnostic/source provenance and the newly created lifecycle owner. Process-spawn IR must additionally preserve the process-bootstrap and transactional child-local materialization facts of `processes.md`; deferred thread creation must preserve start-mode and unstarted-state facts from `threads.md`.

**§ 26(3)** Lowering must consume these validated facts rather than infer ownership, borrowing, process isolation or physical-thread identity from low-level loads, stores or calls. Any missing required proof at the lowering boundary is an internal compiler defect, not permission to weaken Sec's source semantics.

**§ 26(4)** **Amendment MD-138 (2026-11-24, Sec 0.1.1, `rules/corrections/applied/MD-138-callable-execution-requirements-v0.1.1-correction-2026-11-24-20261124.md` § 10(4)).** Spawn Semantic IR keeps the child's execution-requirement validation; the verifier rejects a spawn whose child contract is not satisfied by the selected kind.

**§ 26(5)** **Amendment MD-128 (2026-11-24, Sec 0.1 and 0.1.1, `rules/corrections/applied/MD-128-spawn-transactional-ownership-correction-2026-11-24-20261124.md` § 7(2)–(4)).** Spawn Semantic IR keeps the execution kind, prepared input Places, actual transfer/copy/borrow modes, the creation commit boundary, the success and failure paths, cleanup obligations, provenance and the handle owner; no opcode spelling is mandated. Verification rejects a success path that leaves a consumed source available, a failure path that loses an uncommitted reusable source, duplicate destruction, user entry before input commit, and a reported creation error with an unresolved child lifecycle. Lowering and runtime creation machinery implement the same transaction for each supported kind; process adapters and provisional native threads may differ in mechanism, not in the source contract.

**§ 26(7)** **Amendment MD-133 (2026-11-24, Sec 0.1 and 0.1.1, `rules/corrections/applied/MD-133-spawn-block-return-type-correction-2026-11-24-20261124.md` § 6(3)–(4)).** Semantic IR records for a spawn block the kind, the resolved child callable identity, the complete normal `T`, the raw creation result and handle identity, explicit return edges and distinct void fallthrough edges, with result ownership, cleanup, effect requirements and the creation-commit facts. An unused final expression keeps ordinary evaluation and disposal and is never a synthetic return. Lowering consumes these facts: it never infers `T` from backend call results, chooses a different `T` per kind, removes the evaluation or cleanup of a discarded final call or fabricates a normal payload for a non-returning path.

**§ 26(6)** **Amendment MD-129 (2026-11-24, Sec 0.1 and later compatible versions, `rules/corrections/applied/MD-129-spawn-parent-cancellation-correction-2026-11-24-20261124.md` § 8).** Analysis and Semantic IR keep the distinction between a real current task context, a physical-thread context and the absence of a logical parent, and record the structured cancellation relationship, the initialization of the child's own request state, the relationship removal at detach, the creation outcome and the lifecycle owner. Where the request is not statically known it stays runtime state and is never guessed; a statically known pending parent request never makes a creation configuration invalid. Verification and lowering reject replacing a logical task identity with a worker identity, an implicit detach, clearing a pending request at detach and encoding an existing request as a creation error. Tooling distinguishes a pending request, successful creation and terminal cancellation and never implies that detach resets a request.

## § 27 Diagnostics

**§ 27(1)** Examples:

```text
spawned task result must be owned, awaited, joined or detached
```

```text
task worker cannot escape while borrowing local value data
```

```text
detached task worker cannot retain reference to local value data
```

```text
cannot move value data into task because it is currently borrowed
```

```text
cannot spawn task with MutexGuard[State]; mutex guards are task-bound
```

```text
target profile does not provide task execution
```

```text
target profile does not support process execution
```

```text
process spawn consumes 'payload'; write <-payload to transfer ownership
```

```text
value of type 'T' cannot cross the process boundary
```

```text
unbounded recursive task creation detected: Bomb -> spawn Bomb
```

**§ 27(2)** Parser, semantic, ownership, transferability, execution-capability and runtime-creation failures must remain distinguishable. Stable diagnostic IDs, severity, help and source ranges belong to `rules/tooling/diagnostics.md` and its registered catalog, not to a locally invented `spawn` registry.

**§ 27(3)** A statically unsupported task/thread/process capability is reported as a compile-time target/profile violation. A supported execution kind that fails to create at runtime reports its exact typed spawn error. Neither condition may be silently converted into the other.

**§ 27(4)** Where failure-path ownership is involved, diagnostics must identify the reusable source Place, the required explicit `<-`, the selected execution kind and any proven borrow, lifetime or transferability restriction. Diagnostics must not claim an automatic transfer/rollback rule that is not established by the relevant owner.

**§ 27(5)** **Amendment MD-133 (2026-11-24, Sec 0.1 and 0.1.1, `rules/corrections/applied/MD-133-spawn-block-return-type-correction-2026-11-24-20261124.md` § 4).** The spawn-block discarded-final-call explanation is part of `A3001` as defined in § 3A(8); a non-void block's missing return path, conflicting return types and an underconstrained return type are mandatory diagnostics.

## § 28 Restrictions

**§ 28(1)** `spawn` must not:

- infer asynchronous behavior from a function name
- change the function's declared return type
- silently detach the execution entity
- silently discard the execution handle
- silently clone move-only arguments
- permit invalid references to escape
- transfer a mutex guard across execution-entity boundaries
- bypass ownership or borrowing rules
- silently fall back to synchronous execution

**§ 28(2)** `spawn` must not fabricate `Task.Current()` context for a physical thread or a process; infer a parent task when no current logical task exists; erase deferred thread start mode; interpret ordinary parent-process pointers as child-process references; flatten `TaskOutcome[T]` or callable-returned `Result[T,E]`; or use a common backend path to bypass kind-specific transfer, lifecycle and error guarantees.

**§ 28(3)** Unsafe/FFI implementation machinery cannot silently waive the source-level transferability, ownership or target-isolation requirements that still apply at the selected spawn boundary.

## § 29 Related rules

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
