# Sec `spawn.md` — modernization audit and revision contract

- **Status:** Review; normative rewrite pending approval of genuinely open semantics
- **Created:** 2026-11-09
- **Last updated:** 2026-11-09
- **Document revision:** 1.0
- **Reviewed source:** `rules/concurrency/spawn.md`, revision 1, Git main `dce854ecb317eb62923c638c23a1baaac354be02`
- **Scope:** Preserve all extant section and paragraph identities. Do not renumber or repurpose existing paragraphs.
- **Proposed normative output:** `rules/concurrency/spawn.md` revision 2 (same canonical path)
- **Proposed integration fragment:** `governance/concurrency_spawn.yaml` (register in `governance/index.yaml`)
- **Authority:** Current Sec canonical rulebooks, not the old text of `spawn.md` when superseded by later normative decisions.

---

## § 1 Editorial and normative safeguards

**§ 1(1)** Keep section headings `§ 1` through `§ 29` in place. Retain every existing paragraph identifier, including any now-obsolete paragraph: rewrite its statement to say what is normatively correct, or explicitly state a deprecation/no-longer-applicable boundary. Insert new paragraphs only with new identifiers; do not shift any existing citations.

**§ 1(2)** Source-language semantics and compiler obligations belong in the normative rulebook. Implementation status, missing coverage, source-file locations, test plans, ownership of integration tasks and verification history belong in governance only.

**§ 1(3)** Exact APIs, type declarations and variant sets remain owned by `tasks.md`, `threads.md`, `processes.md`, `cancellation.md`, `transferability.md` and their specialist rulebooks. `spawn.md` owns the common source-level spawn boundary, classification, evaluation and dispatch, not duplicate API inventories.

**§ 1(4)** Preserve the distinction between *documented canonical behavior* and *currently implemented frontend behavior*. In `governance/concurrency_task.yaml`, task spawn typing is explicitly still tracked as a partially implemented old direct-`Task[T]` path. This does not change the normative `Result[Task[T], TaskSpawnError]` type.

---

## § 2 Section-by-section review

| Existing section | Finding | Required modernization |
| --- | --- | --- |
| §1 Purpose | Mostly consistent. | Preserve synchronous vs spawned distinction and fallible task-creation examples. |
| §2 Syntax | `§2(2)` states an initial implementation limitation; the text grammar omits the block alternative. | Follow `grammar.md` §89: `spawn [kind] Expression` **or** `spawn [kind] Block`; move implementation limitations into governance. Validate contextual modifier spelling against the canonical lexical rules. |
| §3 Result type | Correct three-way fallible `Result` type and preservation of nested callable returns. | Retain. Distinguish typed `T`, early creation failure and later execution outcome; use specialist books as authority. |
| §4 Thread configuration | Largely synchronized with canonical `ThreadConfig`. | Retain exact named `<-config` and fresh inline config semantics. Preserve `ThreadStartMode.Deferred`; avoid defining duplicate thread configuration APIs. |
| §5 Common ownership | Correct mandatory lifecycle ownership. | Make operation choice dependent on kind, not syntactic keyword alone; do not imply `await` for threads/processes. |
| §6 Eager scheduling | `§6(1)` unconditionally makes every `spawn` eager; contradictory to deferred physical threads. | Define eager default for task/process, eager-by-default or explicit deferred for thread. A successful deferred thread exists but its callable has not started; do not introduce deferred task/process. |
| §7 Synchronous calls | Correct: function names and suffixes do not change execution mode. | Retain; `async fn` is not required. |
| §8 Current task context | Erroneously gives every spawned execution a current `TaskContext`. | Only task spawn creates a new logical task context; thread spawn creates a physical thread execution identity, and process spawn creates a distinct process isolation domain. No fabricated task identity. |
| §9 Parent and child tasks | Says the currently executing task is always the parent. | Relate a new task to a current logical parent *if one exists*; no implicit root task in ordinary main/native thread context. Preserve owning-handle obligations. |
| §10 Argument evaluation | Correct left-to-right evaluation, but the ownership-commit rules need precision. | Preserve ordinary argument order, transferability and process transaction. Do not claim all execution kinds have the same failure-path transfer without a canonical decision. |
| §11 Ownership transfer | Correct explicit `<-` and no implicit clone. | Cross-link `ownership.md` §§11–14 and `transferability.md`; distinguish preparation/commit/failure when the relevant spawn rules define it. |
| §12 Borrowed arguments | Valid for tasks with proof. | Add target-kind qualification: borrowed task/thread values require lifetime, storage stability, transferability and affinity proofs; ordinary `ref`/`ref mut` cannot cross a process boundary. |
| §13 Mutable borrows | Correct exclusivity while child may use a mutable borrow. | Include task/thread lifetime and execution-affinity constraints; process boundary disallows ordinary borrows. |
| §14 Escaping tasks | Correct: borrowed state cannot outlive its source. | Preserve section scope; cross-link thread/process-specific escape rules without duplicating them. |
| §15 Detached tasks | Correct local-borrow escape and shared-mutation restrictions. | Retain; clarify detachment is kind-specific and distinct from dropping a failed spawn result. |
| §16 Captures and callable values | Omits kind-specific capture transfer and process materialization requirements. | Apply `transferability.md` closure and process boundary facts; do not infer copying from a captured reference or allow implicit cloning of resource-owning closures. |
| §17 Spawned methods | General borrow receiver description is false for process transfer. | Task/thread receivers obey their resolved modes; `spawn process` transfers whole receiver state to child-local storage under the process adapter contract. |
| §18 Task handle ownership | Correct for task handles; bare spawn is invalid as an unhandled owning lifecycle result. | Retain; do not conflate raw `Result` discardability and a successful `Task[T]` owner's lifecycle obligation. |
| §19 Spawn in expressions | Correct raw/handled result distinction. | Retain; examples must use compatible enclosing error channels. |
| §20 Spawn in loops | Unconditional unbounded-work warning is too strong. | Separate statically proven target-policy violations from advice/warnings about possible resource amplification. Do not reject valid hosted dynamic creation solely for lacking a static bound. |
| §21 Recursive spawn | Existing wording requires resource creation to be 'bounded' even on hosted targets. | Follow `tasks.md` §27 and `concurrency.md` §27: target-specific constraints; hosted targets may permit dynamic work and fail with `TaskSpawnError.ResourceLimit`. |
| §22 Scheduling | `§22(1)` says `spawn` does not select a physical thread; false for `spawn thread`. | Narrow to `spawn task`; make task/thread/process semantics explicit. |
| §23 Blocking behavior | Creation may do creation work but must not wait for worker completion. | Retain with kind-specific creation/start distinctions and explicit error-channel behavior. |
| §24 Cancellation inheritance | Uses conditional/provisional phrasing; links to deleted `concurrency.txt`. | Refer to `cancellation.md` §55 and `concurrency.md` §§19, 21, and distinguish task cancellation from explicit `Context`, physical thread and process. The precise already-cancelled-parent case needs confirmation, not silent invention. |
| §25 Static analysis | Mostly task-only inventory. | Add canonical selected execution kind, fallible creation type, source provenance and per-kind transfer/cancellation/capability facts, without duplicating deeper analysis ownership. |
| §26 Semantic IR | Assumes the resulting object is always `Task[T]`. | Require IR to preserve distinct task/thread/process and failure/transaction boundaries, deferring detailed semantics to the canonical Semantic IR and specialist rules. |
| §27 Diagnostics | Useful mentor examples, no stable ID contract. | Align to registered IDs where applicable and clarify compile-time unsupported-target versus runtime creation errors. Registry owns actual IDs/severity. |
| §28 Restrictions | Mostly correct; generic wording hides process isolation. | Retain no implicit fallbacks/detach/cloning; explicitly forbid forging a task context for thread/process spawn. |
| §29 Related rules | Stale `await.txt`, `concurrency.txt`, `static.txt`. | Replace with canonical `.md` paths and precise owner references. |

---

## § 3 Verified cross-book authority

**§ 3(1)** `rules/foundations/grammar.md` §89 includes both expression and block spawn alternatives and treats `task`, `thread`, `process` as contextual spawn modifiers. Its §89(3) calls `spawn process` deferred in current parser implementation; the normative process contract is already specified by `rules/concurrency/processes.md`, while actual implementation status belongs in `governance/concurrency_process.yaml`.

**§ 3(2)** `rules/concurrency/tasks.md` §§4–5 define `Result[Task[T], TaskSpawnError]` and its exact error identity. §27 permits target-dependent resource bounds, including hosted runtime creation failures.

**§ 3(3)** `rules/concurrency/threads.md` §§10–17 define exact `ThreadConfig`, `ThreadStartMode.Eager` and `.Deferred`, and start-once behavior; §20 defines task/thread transfer commit constraints.

**§ 3(4)** `rules/concurrency/processes.md` §§7–8 define eager typed process creation, process-callable materialization, transferability and *transactional process startup ownership commit*. Process errors preserve named source ownership when startup has not committed.

**§ 3(5)** `rules/concurrency/scheduling.md` §§5–6 distinguish eager task creation, default eager thread creation, and deferred-start threads; §§21–22 specify that ordinary main does not have an implicit root task.

**§ 3(6)** `rules/concurrency/cancellation.md` §§4–5, 55 distinguish task/thread identity cancellation and explicit Context propagation; they do not clearly lock what creating a task from an already-cancelled parent must do.

**§ 3(7)** `rules/memory/ownership.md` §14 defines call-argument transfer reservation and commit at call entry. This is not by itself an unambiguous spawn creation-failure contract, since task creation can fail before the user callable starts. Process spawn resolves this explicitly; task/thread spawn needs an explicit cross-kind consistency decision or authoritative reference.

---

## § 4 Genuine design checks, not assumed changes

**§ 4(1) Spawn failure and ownership.** For `match spawn task Work(<-resource)`, if creation returns `Err(TaskSpawnError)`, must `resource` remain owned and usable by the caller (as already specified for `spawn process`), or may ownership be irreversibly committed to creation machinery and cleaned up on failure? The `TaskSpawnError` return type carries no payload to restore. The answer determines source availability, cleanup obligations and IR commit facts. Do not choose silently.

**§ 4(2) Parent already cancelled.** `spawn.md` §24(3) suggests permitting a child task to be created with cancellation already requested. `cancellation.md` §55 delegates to task relationship rules without fully resolving this case. A separate question may be needed after §4(1), unless another authoritative source already locks it.

---

## § 5 Governance extraction

**§ 5(1)** Create `governance/concurrency_spawn.yaml` to own cross-kind `spawn` syntax, semantic dispatch, general input evaluation and shared failure/ownership boundary implementation. Register it in `governance/index.yaml` exactly once.

**§ 5(2)** Keep specialist task, thread and process implementations in their existing owner fragments: `concurrency_task.yaml`, `concurrency_thread.yaml`, `concurrency_process.yaml`. Do not duplicate their detailed implementation status in the spawn fragment.

**§ 5(3)** For root `spawn.md`, remove the source statement about "initial implementation" (§2(2)). Document a normative grammar instead. Keep implementation status, tests, blockers and rollout in governance only.

**§ 5(4)** Before closing the revision, check all 29 sections and every original paragraph number against the previous version, plus YAML syntax/registry parity and source-level conformance fixtures. The present audit is **not** a claim that code was changed or tests were run.
