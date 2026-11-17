# Sec Events — Normative Modernization Correction

| Field | Value |
| --- | --- |
| **Status** | Superseded 2026-11-17 by `events-normative-fix-20261113-decisions-20261117.md` (revision 1.1), which completed §§ 10–11 after the MD-134 decision; independent clauses were applied 2026-11-13 |
| **Created** | 2026-11-13 |
| **Last updated** | 2026-11-13 |
| **Document revision** | 1.0 (correction document) |
| **Target rulebook** | `rules/concurrency/events.md` revision 1 → 2.0 |
| **Sec language version** | 0.1 |
| **Reviewed repository** | `YoePro/sec` |
| **Repository baseline** | `ee75e5d036d6611519a1b34d570bb6778002acfd` |
| **Governance integration** | `governance/concurrency_events.yaml` (`concurrency.events-v2`), registered in `governance/index.yaml` on 2026-11-13 |
| **Open normative dependency** | **Proposed MD-134**: by-value handler signature versus borrowed publication payload (see separate decision file) |

## § 0 Application contract

**§ 0(1)** This document supplies **normative replacement text**. It is not a defect list. For each identified original paragraph, replace that paragraph **in situ** with the exact text below. For a new paragraph, append the new ID at the end of its existing section. For a whole-section replacement, retain the existing section number. Preserve all original section numbers **§ 1–§ 36**, original paragraph IDs and existing unaffected normative sentences. No paragraph may be renumbered, deleted, re-used with a different meaning, or silently merged with another paragraph.

**§ 0(2)** Update the original rulebook metadata to **Status: Normative**, **Last updated: 2026-11-13**, **Document revision: 2.0**, **Repository baseline reviewed: `ee75e5d036d6611519a1b34d570bb6778002acfd`**; add a link to `governance/concurrency_events.yaml`, this correction, `rules/foundations/grammar.md`, `rules/declarations/lambda-functions.md`, `rules/concurrency/concurrency.md`, `rules/memory/transferability.md`, `rules/memory/destruction.md`, and `rules/platform/target_profiles.md`. Keep the original creation date.

**§ 0(3)** Sections not explicitly amended remain valid unless the MD-134 decision later changes their semantics. **Do not choose an event payload/handler calling convention by implication.** The complete `fn(T) void` versus borrowed-payload conflict in §§ 10–11 is isolated in the attached proposed decision MD-134. Integration as a completely synchronized revision 2.0 requires the owner to decide MD-134 and then resolve those paragraphs, along with any dependent tests and API declarations. All other sections here are independent of that decision.

**§ 0(4)** Mutable compiler status, implemented/remaining work, paths of implementation files, code-generation chores and conformance test inventories belong exclusively to `governance/concurrency_events.yaml`. Never migrate them back into this rulebook. Preserve original normative guarantees, source examples, and diagnostic expectations where not superseded below.

---

## § 1 Purpose — replacement

Replace **§ 1(1)**:

> **§ 1(1)** A Sec event is an instance-owned, typed, synchronous publish/subscribe member for software notifications. In Sec 0.1 its storage is part of the owning value, either through a short-form `Event[T]` field or through an explicit `EventStorage[T, Capacity]` field exposed by its owning `impl`. A future type-level/static event form is not implicitly enabled by this rule.

Retain § 1(2)–(3).

## § 2 Core rule — addition

Append **§ 2(5)**:

> **§ 2(5)** The only Sec 0.1 `impl` event declaration is `event Name using existingStorage`, with no body. It exposes an existing instance-storage field and does not allocate or synthesize another field. `grammar.md` § 31 owns the grammar; a trailing policy block is invalid syntax.

## § 3 Default policy — replacements

Replace **§ 3(1)** with:

> **§ 3(1)** For Sec 0.1, `Event[T]` has the following exact default properties:
>
> ```text
> subscriber capacity: 4
> dispatch: synchronous, in the publisher's current execution context
> dispatch order: subscription order
> publication: only from the declaring type's valid owning impl context
> subscription: according to event-member visibility
> storage: fixed-capacity inline instance storage
> dynamic allocation: none
> implicit concurrent thread safety: none
> ordinary ISR publication: not permitted
> ```
>
> These are language-visible guarantees, not permission for a backend to silently select an allocating, asynchronous or thread-safe substitute. The payload argument and handler-parameter ownership model remain subject to MD-134.

Retain § 3(2)–(3).

## § 4 Data and behavior — addition

Append **§ 4(4)**:

> **§ 4(4)** `Subscribe` and `Publish` operate on compiler-managed event state. Their availability on event members must not be generalized into a source-level right to mutate arbitrary fields through a shared reference. The compiler must prove the event member, receiver, operation, lifetime, and applicable concurrent-access policy. Subscription operations are usable on an accessible event through an ordinary `ref` owner where the owning event contract allows them; this does not make the default event thread-safe.

## § 5 Publication rights — replacement and addition

Replace **§ 5(1)**:

> **§ 5(1)** Publication authority belongs to the `impl` of the event's declaring owner type and its valid same-owner extensions under the ordinary `impl` coherence rules. The compiler validates declaration identity and receiver authority; it must not infer publisher rights solely from visibility, field name, an interface requirement, or access to the underlying storage representation.

Append **§ 5(5)**:

> **§ 5(5)** Sec 0.1 has no public event-publication modifier or general external-publisher capability. Only already-defined owner publication is legal; the presence of a readable or subscribable event member grants no right to invoke `Publish` externally.

## § 6 Subscription — replacements and additions

Replace **§ 6(1)**:

> **§ 6(1)** `Subscribe(handler)` returns the exact closed `EventSubscribeResult` union with `Subscribed(Subscription)` or `Full`. Only `Subscribed` contains an owning `Subscription` value. That value is move-only, represents one registration, and must not be implicitly copied or abandoned as if it carried no cleanup obligation.

Replace **§ 6(2)**:

> **§ 6(2)** `Subscription.Close()` is a **consuming** operation on the subscription owner. A successful close unregisters the associated handler, releases the subscription's lifecycle obligation, and leaves the consumed source handle unavailable. A second `Close()` through that source is invalid.

Replace **§ 6(3)**:

> **§ 6(3)** Ordinary deterministic destruction of a still-owned `Subscription` performs the same unregistration and resource cleanup as `Close()`. A moved subscription is cleaned up only by its current owner. No subscription registration or captured handler resource may be destroyed twice.

Replace **§ 6(4)**:

> **§ 6(4)** A registered handler environment must not retain captures or references whose lifetime, provenance, ownership authority or execution affinity expires before its last possible invocation. Handler storage and captures are subject to the canonical callable-value, borrowing, transferability and destruction rules; registering a closure does not extend an invalid borrow or silently clone move-only captures.

Append **§ 6(5)**:

> **§ 6(5)** A live subscription may not refer to event backing storage that has been destroyed, invalidated or relocated in a way forbidden by the selected event-storage contract. Destruction, replacement and movement of the event owner must account for every remaining subscription. Compiler analysis may accept an operation only when the required lifetime and storage-stability proof is established; a missing proof is not permission to leave a dangling subscription.

Append **§ 6(6)**:

> **§ 6(6)** Removing a subscription during its own currently executing synchronous callback is legal under § 14. Unregistration prevents subsequent invocations but must not destroy callable/capture state still required by the active invocation. Destruction of that state occurs after its final required use.

## § 7 Capacity — replacements

Replace **§ 7(2)**:

> **§ 7(2)** When the subscription capacity is exhausted, `Subscribe` returns `EventSubscribeResult.Full` with **no** `Subscription` owner created. This is an ordinary closed-union outcome, not a `Result` error or a hidden allocation request.

Replace **§ 7(5)**:

> **§ 7(5)** The exact returned source-visible type is `EventSubscribeResult`, whose only variants are `Subscribed(Subscription)` and `Full` (see § 29). A caller must handle or explicitly dispose of this result according to ordinary result/ownership rules; discarded `Subscribed` payloads are never silently leaked. Argument transfer and cleanup when `Full` is returned follow ordinary call/capture rules until a more specific accepted event rule states otherwise.

## § 8 Explicit capacity — addition

Append **§ 8(4)**:

> **§ 8(4)** `Event[T]` without a capacity uses the default four slots. `Event[T, N]` and `EventStorage[T, N]` require a positive compile-time constant capacity `N`. `EventStorage[T]` has no implicit default-capacity spelling in Sec 0.1. Capacity participates in the concrete event-storage representation and layout; the compiler must not silently replace it with a dynamic or runtime-sized allocation.

## § 9 Dispatch — additions

Append **§ 9(5)**:

> **§ 9(5)** Synchronous publication invokes eligible handlers as ordinary synchronous callable executions in the publisher's current execution context. It introduces no implicit task, native thread, process, scheduler wait, channel send, or completion edge. Any effects, cancellation requirements, panic behavior or externally visible side effects of invoked handlers remain ordinary transitive effects of the publication operation.

Append **§ 9(6)**:

> **§ 9(6)** A nested publication triggered by a handler is a separate synchronous publication. Each invocation applies §§ 13–14 to the registration state visible at that invocation. The backend may use snapshots or stable slot/generation records, but must not invoke removed registrations, skip eligible current registrations, or violate publication ordering.

## §§ 10–11 Handler signature and payload ownership — **owner decision gate**

**No unilateral replacement is authorized for § 10(1)–(3) or § 11(1)–(3).** They currently assert *both*:

- `fn(data: T) void`, an **owned by-value** parameter under `functions.md` and `lambda-functions.md`; and
- a **borrowed payload** delivered to each synchronous subscriber, without cloning move-only payloads.

These are incompatible for general move-only `T` and multiple independent subscribers. `analyzer_inference.go` currently checks the by-value `fn(T) void` signature; this is implementation evidence, **not** authorization to change the borrowed-payload normative decision. Proposed **MD-134** describes the alternatives. The owner must select the canonical callback/payload model; then update §§ 10–11, the declaration/intrinsic surface and required conformance tests together. Until then, do not make either choice silently in this fix.

## § 12 Panic — addition

Append **§ 12(4)**:

> **§ 12(4)** A handler panic cannot be reclassified as an event-specific failure result or silently converted into normal publication completion. Ordinary panic policy, cleanup and execution-context boundaries apply. Prior completed handler effects are not rolled back; event and subscription state must remain memory-safe during supported panic cleanup, including reentrant or mid-dispatch unregistration.

## § 13 Reentrancy — addition

Append **§ 13(4)**:

> **§ 13(4)** In Sec 0.1 there is no source-visible policy-block switch to disable reentrancy. Valid nested publication must obey the same access, ordering, borrow and storage-lifetime contracts as an independent publication. Reentrancy never permits concurrent unsynchronized publication merely by virtue of being synchronous within one caller.

## § 14 Mutation during dispatch — additions

Append **§ 14(4)**:

> **§ 14(4)** Every publication must maintain a stable distinction between registrations eligible for that invocation and registrations added after its start. A newly added registration may participate in a subsequent, including nested, publication started after registration, but not retroactively in the already-running publication.

Append **§ 14(5)**:

> **§ 14(5)** Unregistration must not invalidate an in-flight invocation's still-live callable environment. The runtime must preserve storage and handler lifetime until the active call has finished, while making that registration unavailable to later dispatch turns. Physical representations may differ if the observable contract is preserved.

## § 15 Full storage form — addition

Append **§ 15(4)**:

> **§ 15(4)** `EventStorage[T, N]` is a compiler-known, non-implicitly-copyable instance-storage capability. `event Name using field` must name an existing compatible storage member of the same owning type. A valid `impl` exposes that storage but cannot add hidden instance fields, silently change its capacity, invent an event policy block or grant publication authority to arbitrary external code.

## § 16 Short-form lowering — replacements and addition

Replace **§ 16(2)**:

> **§ 16(2)** The short form is semantically equivalent, for ownership, fixed storage, capacity, public event behavior, member access and layout, to an associated generated storage identity plus an event exposure using the same default policy. This equivalence is a compiler semantic model; it does not grant user source access to generated field names.

Replace **§ 16(3)**:

> **§ 16(3)** Conceptual model only (not a legal extra source declaration a user must write):
>
> ```sec
> type Button struct {
>     __ButtonPressedStorage: EventStorage[ButtonPressData, 8]
> }
>
> impl Button {
>     event ButtonPressed using __ButtonPressedStorage
> }
> ```
>
> The backend must preserve the short form's single event-member identity and must not accidentally expose, shadow or duplicate the internal generated storage name.

Append **§ 16(6)**:

> **§ 16(6)** The selected lower-level representation cannot retroactively change an instance event into an automatically shared static event, or change the source-visible `Event[T, N]` and `EventStorage[T, N]` identities into unrelated public APIs.

## § 17 Impl customization — complete replacements

Replace **§ 17(1)**:

> **§ 17(1)** Sec 0.1 uses an `impl` event declaration to expose an existing `EventStorage[T, N]` field as an event member. The supported form defines **exposure**, not configurable policy. It is not necessary for the short-form `Event[T, N]` field whose default exposure is already defined.

Replace **§ 17(2)**:

> **§ 17(2)** Canonical Sec 0.1 source:
>
> ```sec
> type Button struct {
>     buttonPressedStorage: EventStorage[ButtonPressData, 8]
> }
>
> impl Button {
>     event ButtonPressed using buttonPressedStorage
> }
> ```
>
> No policy block follows the declaration.

Replace **§ 17(3)**:

> **§ 17(3)** `grammar.md` § 31 forbids an `impl event` body. Source such as `event ButtonPressed using buttonPressedStorage { publish: owner }` is **invalid in Sec 0.1** and must not be parsed as a supported feature. Public policy-block syntax and any configurable dispatch policy belong to a separately approved future language revision.

Replace **§ 17(4)**:

> **§ 17(4)** The approved division is: storage belongs to the owner type; the `impl` supplies the event's exposure name and binds it to existing storage; the default Sec 0.1 event policy belongs to the language and is not user-configurable through an `impl` body.

## § 18 Visibility and external publication — replacement

Replace **§ 18(3)**:

> **§ 18(3)** No general external event-publication capability, `public publish` modifier or alternative publisher access is defined for Sec 0.1. Such a feature may be considered in a later revision, but neither a visible event member nor interface conformance enables it today.

## § 19 Interfaces — addition

Append **§ 19(3)**:

> **§ 19(3)** A type conforms to an interface event requirement only when a visible matching event member exposes the required payload type through its permitted event form. Conformance never materializes extra instance storage, changes the concrete capacity, grants publication rights or gives the interface view a direct storage-mutation capability beyond the event operations it canonically exposes.

## § 20 Copy and move — replacements and addition

Replace **§ 20(1)**:

> **§ 20(1)** `Event[T, N]` and `EventStorage[T, N]` are not implicitly copyable resource-bearing event storage. A containing value cannot become implicitly copyable when that would copy its event storage or duplicate live subscriptions. Ordinary copy/move classification remains compositional with the compiler-known event-storage restrictions.

Replace **§ 20(3)**:

> **§ 20(3)** An event-owning value may move only when the compiler proves the selected representation preserves event storage identity, active subscription validity, callback/capture storage, ownership and address/provenance constraints. In particular, moving an owner **before** any subscription may be valid; moving one with live subscriptions is not implicitly valid and must be rejected if safety cannot be proved.

Append **§ 20(5)**:

> **§ 20(5)** A `Subscription` move transfers one registration owner, not the event, subscriber slot or callback identity itself. It does not duplicate subscription state, and closing the moved handle unregisters that same original registration exactly once.

## § 21 Address stability — addition

Append **§ 21(4)**:

> **§ 21(4)** The compiler must account for any subscription handle that retains a reference, location, generation or storage identity dependent on the owner. It may use a proven stable representation or reject relocation while subscriptions remain live; no implicit heap-promotion or hidden pinned allocation is introduced to make otherwise-invalid movement legal.

## § 22 Allocation — replacement

Replace **§ 22(2)**:

> **§ 22(2)** A dynamically allocating or arena-backed event policy is **not a selectable public Sec 0.1 event policy**. A future approved policy must state its allocation and failure semantics explicitly; the present default and explicit-capacity inline storage forms must not allocate to grow subscriber capacity or repair a capacity error.

## § 23 Thread safety — replacement and addition

Replace **§ 23(2)**:

> **§ 23(2)** Concurrent publication, subscription or unsubscription of the **same** default event storage is invalid without a separately proven synchronization arrangement covering its complete internal state and handler lifetimes. Sec 0.1 provides no selectable built-in `threadSafe` event policy. Cross-execution access is constrained by the ordinary transferability, borrowing, data-race and memory-model rules; shared-reference syntax alone does not prove safety.

Append **§ 23(4)**:

> **§ 23(4)** Safe use through an external mutex or another canonical synchronization mechanism must preserve the event's synchronous callbacks, reentrancy and lifetime constraints; a backend must not invent a hidden lock that would change callback ordering or create unmodeled deadlock behavior. Synchronous event dispatch by itself establishes no inter-thread synchronization edge.

## § 24 Tasks and threads — addition

Append **§ 24(5)**:

> **§ 24(5)** An event handler runs under the real execution context of the publisher. A task publication retains that task's logical context; a physical-thread publication retains that physical thread's identity; ordinary `main` does not gain an implicit `TaskContext`. Handler calls participate in ordinary transitive effect/cancellation analysis and may not silently migrate to the subscribing execution entity.

## § 25 Interrupts — addition

Append **§ 25(4)**:

> **§ 25(4)** No special `@interruptSafe` event publication policy is defined for ordinary `Event[T, N]` in Sec 0.1. A later explicitly defined ISR-safe adapter must separately prove no-allocation, no-blocking, bounded progress, reentrancy, interrupt-context validity and suitable publication ordering. ISR-safe handoff through separately approved channels/atomics/deferred work does not make direct event dispatch ISR-safe.

## § 26 Events versus channels — whole-section replacement

Replace the **unnumbered body of § 26**, preserving the heading:

> ```text
> Event[T]
>     One publication invokes zero or more subscribed handlers.
>     Sec 0.1 default dispatch is synchronous, in the publisher's context.
>     Handler invocation does not by itself queue work or establish
>     completion/message-transfer synchronization.
>
> Channel[T]
>     One committed receive transfers one delivered message to one
>     receiving operation under channels.md. Send/receive may wait,
>     and a channel has its own ownership and synchronization contract.
> ```
>
> This comparison does not define the event handler parameter or payload ownership mode; §§ 10–11 own that contract. Do not infer its answer from the channel comparison.

## § 29 Compiler-known types — replacements and additions

Replace **§ 29(1)**:

> **§ 29(1)** The canonical compiler-known Sec 0.1 event type identities are `Event[T]`, `Event[T, N]`, `EventStorage[T, N]`, `Subscription`, and `EventSubscribeResult`. The optional event capacity is a compile-time integer argument, not an extra runtime parameter or an additional independent generic type. An `EventStorage` use requires explicit capacity. Compiler-only representation fields are not user-visible source fields.

Replace **§ 29(3)**:

> **§ 29(3)** `Subscribe(handler)` returns the source-visible closed `EventSubscribeResult`; `Publish(payload)` returns `void`; `Subscription.Close()` consumes its owning receiver and returns `void`. These identities are source API contracts even on a target/backend whose event runtime or lowering is not yet implemented. The exact handler type and payload delivery contract are governed by §§ 10–11.

Replace **§ 29(4)**:

> **§ 29(4)** The exact Sec 0.1 `EventSubscribeResult` union is:
>
> ```sec
> type EventSubscribeResult union {
>     // Registration succeeded and the caller owns exactly one subscription.
>     Subscribed(Subscription)
>
>     // Fixed subscriber capacity was reached; no subscription was created.
>     Full
> }
> ```
>
> The union has no implicit additional variants, error payload, `Result` wrapper or allocation fallback.

Append **§ 29(5)**:

> **§ 29(5)** The compiler-known source identity `Subscription` is move-only and deterministically unregisters on consuming `Close()` or owner destruction. Its ordinary implementation representation remains private; no address-stability, ownership or runtime result semantics may be inferred from arbitrary source-level user-declared lookalike types.

## § 30 Semantic analysis — additions

Append **§ 30(2)**:

> **§ 30(2)** Semantic analysis must bind an event member to its exact declaring owner and existing/generated backing storage, retain the compile-time capacity and visibility/publication authority, validate the exact handler signature defined by §§ 10–11, and record the returned `EventSubscribeResult` lifecycle obligations. Subscription lifetime, closure captures and active-callback uses remain subject to ordinary borrowing/escape/ownership rules. Unsupported storage/policy configurations are errors, not silent backend fallbacks.

Append **§ 30(3)**:

> **§ 30(3)** Semantic analysis must not mistake synchronous handler invocation for a detached task, implicit thread, channel delivery or proof of thread safety. Publication is an ordinary synchronous call/effect edge for effect analysis. Concurrency, ISR safety, reentrancy, relocation and unsubscribe-during-dispatch facts must be preserved for runtime lowering or rejected when the selected capability cannot be proved.

## § 31 Semantic IR — replacements and addition

Replace **§ 31(1)**:

> **§ 31(1)** Semantic IR must preserve typed event creation and storage identity, exposure, exact member/owner authority, subscription registration and lifecycle, publication and synchronous handler invocation, unregistration, dispatch ordering, fixed capacity, and lifetime/provenance/synchronization effects. These are **semantic facts**; this rulebook does not establish fixed IR opcode spellings such as `EventDispatchBegin` as a public or permanently stable IR vocabulary. Concrete instruction names and verification rules belong to `rules/compiler/semantic_ir.md`.

Replace **§ 31(2)**:

> **§ 31(2)** The IR must preserve resolved payload and capacity, owner and backing storage identity, source provenance, handler callable/capture representation, subscription ownership, active-invocation lifetime, dispatch order, nested publication and unregister-before-turn semantics, and any target/affinity limitations. Neither a source-valid event operation nor an implementation gap licenses emission of an incorrect backend stub.

Append **§ 31(3)**:

> **§ 31(3)** A backend without conforming event subscriber storage, dispatch, cleanup or memory-safe callback lifetime machinery must diagnose unsupported lowering and produce no invalid executable artifact. Normative source semantics and backend implementation status are distinct; the latter is tracked in governance.

## § 32 Diagnostics — addition

Append **§ 32(3)**:

> **§ 32(3)** The compiler must distinguish source-invalid event declarations or calls from missing backend implementation. The former receive canonical syntax, typing, visibility, ownership, lifetime and target-policy diagnostics with stable IDs; an implementation limitation must be identified as such rather than rewriting the legal source contract. A `Full` result is a normal source-visible event outcome, not a compile-time error merely because the event can reach capacity at runtime.

## § 33 Governance boundary — replace section heading and both paragraphs

Change heading from `## § 33 Current implementation status` to:

> `## § 33 Implementation governance boundary`

Replace **§ 33(1)**:

> **§ 33(1)** This rulebook defines the Sec 0.1 **normative event contract** independently of implementation coverage. Authoritative implementation evidence, existing compiler/parser/Sema/LSP functionality, integrated status, missing runtime support and pending test work are maintained in `governance/concurrency_events.yaml` under integration `concurrency.events-v2`. No mutable implementation inventory is part of this normative section.

Replace **§ 33(2)**:

> **§ 33(2)** Tooling and implementers must distinguish an already-defined source contract from a backend capability not yet integrated. A source-valid event construct cannot be redefined as syntactically or semantically illegal simply because registration, dispatch or lowering is incomplete; where required runtime capability is unavailable, target/profile or unsupported-lowering diagnostics apply under the canonical compiler rules. Where the normative contract is incomplete, it must be resolved by the owner rather than guessed from implementation shortcuts.

## § 34 Restrictions — addition

Append **§ 34(2)**:

> **§ 34(2)** Sec 0.1 additionally forbids an `impl event` policy block, inferred external publish authority, implicit task/thread dispatch, hidden subscriber-capacity growth, invalid active-subscription relocation, implicit cloning of move-only captures or subscription handles, unsafe lifetime extension, and assuming that an ordinary event is ISR-safe or thread-safe. A backend must preserve the canonical handler and payload ownership contract once §§ 10–11 are synchronized; it must not silently change the handler ABI.

## § 35 Future extensions — replacement and addition

Replace **§ 35(1)**:

> **§ 35(1)** Possible future additions include dynamic/arena-backed subscriber storage, selectable thread-safe policies, asynchronous dispatch adapters, weak subscriptions, filters, priority ordering, event-to-channel adapters, external publication capabilities and type-level/static event forms. This list is **non-normative**; none of these source features is enabled by Sec 0.1 without its own approved rulebook/grammar/API revision.

Append **§ 35(2)**:

> **§ 35(2)** A future policy must explicitly specify syntax, owner/storage model, handler/payload ownership, cancellation/panic handling, concurrency synchronization, allocation and error behavior, dispatch mutation and reentrancy, interface exposure, and target capability. The mere presence of an example in an older document is not permission to implement a future feature.

## § 36 Related rules — whole-section replacement

Replace the unnumbered list under **§ 36** with canonical repository paths:

> ```text
> rules/foundations/grammar.md
> rules/declarations/struct.md
> rules/declarations/impl.md
> rules/declarations/interfaces.md
> rules/declarations/functions.md
> rules/declarations/lambda-functions.md
> rules/memory/ownership.md
> rules/memory/borrowing.md
> rules/memory/destruction.md
> rules/memory/lifetime_analysis.md
> rules/memory/layout.md
> rules/memory/transferability.md
> rules/concurrency/concurrency.md
> rules/concurrency/concurrency_memory_model.md
> rules/concurrency/channels.md
> rules/concurrency/tasks.md
> rules/concurrency/threads.md
> rules/concurrency/cancellation.md
> rules/platform/interrupts.md
> rules/platform/target_profiles.md
> rules/compiler/semantic_ir.md
> rules/tooling/diagnostics.md
> ```
>
> All paths listed above were checked against the repository where explicitly listed in this correction; do not recreate obsolete `.txt` references.

---

## § 37 Integration and verification instructions (non-normative; for the integrating agent only)

1. Confirm current `main` HEAD; if changed, reconcile this proposal with the updated rulebooks first.
2. Apply only the exact in-situ replacements/additions above; leave unaffected sections and all original IDs intact. If adding an ID, use the next previously unused ID within that section. The heading of § 33 may change without changing its number.
3. Do **not** apply a fabricated payload convention to §§ 10–11; the owner decides proposed MD-134. Reflect the approved outcome in this fix before declaring the full revision synchronized.
4. Create and register `governance/concurrency_events.yaml` from the companion merge proposal; keep source/runtime status out of the rulebook. Do not accidentally mark unimplemented dispatch or lowering `implemented`.
5. Cross-check `grammar.md` §§ 31, 109 (bodyless event exposure; compile-time capacity), `lambda-functions.md`, `functions.md`, `ownership.md`, `borrowing.md`, `transferability.md`, `destruction.md`, `concurrency_memory_model.md`, `interrupts.md`, `semantic_ir.md`, and current core/frontend source declarations. Do not modify these already newer books unless a confirmed contradiction requires its own controlled correction.
6. Add conformance tests as tracked by governance: valid short/full forms, capacity bounds, owner-only publication, exact `EventSubscribeResult`, consuming `Close`, handler capability, interface observation, no policy block, captures, storage lifetime, move/copy classification, synchronous ordering, unregistration during dispatch, reentrancy, concurrent access/ISR rejection, IR and target lowering. The payload-mode tests depend on MD-134.
7. Validate governance schema (`go run ./cmd/yamlstatus check`), test parser/Sema, and verify every old `§ n(k)` reference still exists exactly once. Do not claim runtime tests passed without running them.

**Baseline note:** This correction was prepared by reading repository source and normative books at the verified commit. No implementation files, repository governance or tests have been modified, and no local compiler suite has been run.
