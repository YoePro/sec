# Sec Events — Normative Modernization Correction

| Field | Value |
| --- | --- |
| **Status** | Applied 2026-11-17 to `rules/concurrency/events.md` revision 2.0 (all §§ 0–36 paragraphs present verbatim; governance `concurrency.events-v2` synchronized; MD-134 removed) |
| **Created** | 2026-11-13 |
| **Last updated** | 2026-11-13 |
| **Document revision** | 1.1 (correction document; owner decisions consolidated) |
| **Target rulebook** | `rules/concurrency/events.md` revision 1 → 2.0 |
| **Sec language version** | 0.1 |
| **Reviewed repository** | `YoePro/sec` |
| **Repository baseline** | `ee75e5d036d6611519a1b34d570bb6778002acfd` |
| **Governance integration** | Proposed `governance/concurrency_events.yaml` (`concurrency.events-v2`); register in `governance/index.yaml` |
| **Owner decision status** | MD-134 resolved in owner discussion on 2026-11-13: borrowed payload, retained external callable storage, and the associated subscription/reentrancy/lifetime invariants below. No separate MD file is required. |

## § 0 Application contract

**§ 0(1)** This document supplies **normative replacement text**. It is not a defect list. For each identified original paragraph, replace that paragraph **in situ** with the exact text below. For a new paragraph, append the new ID at the end of its existing section. For a whole-section replacement, retain the existing section number. Preserve all original section numbers **§ 1–§ 36**, original paragraph IDs and existing unaffected normative sentences. No paragraph may be renumbered, deleted, re-used with a different meaning, or silently merged with another paragraph.

**§ 0(2)** Update the original rulebook metadata to **Status: Normative**, **Last updated: 2026-11-13**, **Document revision: 2.0**, **Repository baseline reviewed: `ee75e5d036d6611519a1b34d570bb6778002acfd`**; add a link to `governance/concurrency_events.yaml`, this correction, `rules/foundations/grammar.md`, `rules/declarations/lambda-functions.md`, `rules/concurrency/concurrency.md`, `rules/memory/transferability.md`, `rules/memory/destruction.md`, and `rules/platform/target_profiles.md`. Keep the original creation date.

**§ 0(3)** The owner has decided that Sec 0.1 event payloads are borrowed as `ref T` and that callbacks receive `ref T`, never implicit per-subscriber copies. The same discussion established external storage for capturing handlers, immobility of the event owner while registrations are live, exclusive callable-environment access for mutable handlers, dynamic panic on unprovable reentrant exclusive conflicts, prompt logical unregistration with safe deferred physical reclamation, and per-publication registration eligibility. These decisions are integrated into the replacement paragraphs below; they do not imply that the existing frontend/runtime is already synchronized.

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
> These are language-visible guarantees, not permission for a backend to silently select an allocating, asynchronous or thread-safe substitute. Payload delivery is borrowed (`ref T`), according to §§ 10–11.

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

> **§ 6(4)** A registered capturing handler is held through a retained borrow of the **externally owned, address-stable callable storage**. The event retains no owned copy of the callable or capture environment and performs no hidden allocation of it. The callable and all borrowed captures must remain valid through the last in-flight invocation, including an invocation whose subscription has just been logically closed. Registration may retain `ref callable` for shared reusable callable capability or `ref mut callable` for an exclusive mutable callable capability, subject to the closure and borrow rules. Registering a callable does not implicitly extend its lifetime, move it into event storage, clone its captures, or confer cross-thread safety.

Append **§ 6(5)**:

> **§ 6(5)** A live `Subscription` retains compiler-verified lifetime and location dependencies on the **event owner and, for a capturing handler, its external callable storage**. The event owner and retained callable storage must not move, be destroyed, replaced, or otherwise invalidated until all registrations and in-flight callback dependencies on them have ended. This is an invariant of the default Sec 0.1 inline event model, not a backend choice to permit relocation. The non-generic source spelling `Subscription` does not erase its compiler-tracked origins or release its borrow obligations.

Append **§ 6(6)**:

> **§ 6(6)** Removing a subscription during its own currently executing synchronous callback is legal under § 14. Unregistration immediately prevents later invocation turns, including in nested publication, but must not destroy the callable environment while an invocation still uses it. Retained lifetime/borrow obligations end only after the last active invocation and registration dependency have both ended.

Append **§ 6(7)**:

> **§ 6(7)** An externally stored capturing callable is subscribed using an explicit retained reference, for example `Subscribe(ref handler)` or `Subscribe(ref mut handler)` when its inferred callable capability requires exclusive access. A named non-capturing function may continue to be passed as a function value (`Subscribe(OnPressed)`) because no external capture environment is retained. A temporary callable with no provably sufficient storage lifetime cannot be subscribed by an escaping retained borrow. The ordinary call-only implicit borrowing rule does **not** extend the lifetime of a callback retained by `Subscribe`.

Append **§ 6(8)**:

> **§ 6(8)** A live `Subscription` imposes a *retained storage-validity and non-relocation dependency* on its owning event instance. This dependency is **not an ordinary shared borrow of the whole publisher value** and does not by itself prohibit safe mutation of unrelated publisher fields or owner-authorized synchronous `Publish` calls. Existing ordinary borrows, receiver authority, alias analysis and data-race rules continue to apply independently. Only compiler-known event operations may access the event's internal registration state through their defined synchronization/interior-mutation contract; subscription does not confer arbitrary mutation through `ref` to other fields. This separation is essential: otherwise a successful subscription would make ordinary event-producing publisher methods unusable while subscribed.

## § 7 Capacity — replacements

Replace **§ 7(2)**:

> **§ 7(2)** When the subscription capacity is exhausted, `Subscribe` returns `EventSubscribeResult.Full` with **no** `Subscription` owner created. This is an ordinary closed-union outcome, not a `Result` error or a hidden allocation request.

Replace **§ 7(5)**:

> **§ 7(5)** The exact returned source-visible type is `EventSubscribeResult`, whose only variants are `Subscribed(Subscription)` and `Full` (see § 29). A caller must handle or explicitly dispose of this result according to ordinary result/ownership rules; discarded `Subscribed` payloads are never silently leaked. When `Full` is returned, no registration or retained callback/event borrow is established; the caller retains the ordinary ownership of its external callable.

Append **§ 7(6)**:

> **§ 7(6)** A logically closed registration whose callback is still executing continues to reserve its physical slot. The slot becomes reusable only after no in-flight invocation requires the callable, slot generation, or retained storage. Consequently `Subscribe` may return `Full` although fewer than `N` registrations remain logically active. No additional slot is allocated to bypass this bound.

## § 8 Explicit capacity — addition

Append **§ 8(4)**:

> **§ 8(4)** `Event[T]` without a capacity uses the default four slots. `Event[T, N]` and `EventStorage[T, N]` require a positive compile-time constant capacity `N`. `EventStorage[T]` has no implicit default-capacity spelling in Sec 0.1. Capacity participates in the concrete event-storage representation and layout; the compiler must not silently replace it with a dynamic or runtime-sized allocation.

## § 9 Dispatch — additions

Append **§ 9(5)**:

> **§ 9(5)** Synchronous publication invokes eligible handlers as ordinary synchronous callable executions in the publisher's current execution context. It introduces no implicit task, native thread, process, scheduler wait, channel send, or completion edge. Any effects, cancellation requirements, panic behavior or externally visible side effects of invoked handlers remain ordinary transitive effects of the publication operation.

Append **§ 9(6)**:

> **§ 9(6)** A nested publication triggered by a handler is a separate synchronous publication. Each invocation applies §§ 13–14 to the registration state visible at that invocation. The backend may use snapshots or stable slot/generation records, but must not invoke removed registrations, skip eligible current registrations, or violate publication ordering.

## § 10 Handler type — owner-approved replacements and additions

Replace **§ 10(1)**:

> **§ 10(1)** Every Sec 0.1 event handler receives its published payload by **shared borrow**, never by an implicit owned value. A named handler is declared as:
>
> ```sec
> fn OnPressed(data: ref ButtonPressData) void {
>     Observe(data)
> }
> ```
>
> The corresponding reusable callable type is `fn(ref T) void`. A capturing lambda is written using ordinary `capture(...) fn(data: ref T) void { ... }` syntax; no `mut fn` **implementation** syntax is introduced.

Replace **§ 10(2)**:

> **§ 10(2)** Event callbacks return exactly `void` and must be **reusable**. Non-consuming callable capability `fn(ref T) void` is accepted through shared callable access; an environment-mutating callable of inferred `mut fn(ref T) void` capability is accepted only through a valid exclusive `ref mut` to its external storage. In Sec, `mut fn` here is a **callable type/capability**, not a legal function or lambda implementation introducer. Consuming `-> fn` callback capability is incompatible with an event that may publish repeatedly.

Replace **§ 10(3)**:

> **§ 10(3)** Business errors are handled by the handler or represented through explicit program state. The event does not turn such errors into a `Publish` result. Unhandled panic and an illegal dynamic exclusive-callable reentrancy conflict follow § 12 and § 13, respectively.

Append **§ 10(4)**:

> **§ 10(4)** `Subscribe` retains a named function value or a non-capturing lambda as a stateless callable value without requiring user-owned capture storage; for a capturing callable, it retains only a proven-live shared or exclusive reference to externally owned callable storage under § 6. A `ref T` payload parameter does not by itself imply that the callback's captured environment is immutable: the inferred callable/environment capability is a separate axis from payload mutability.

## § 11 Payload ownership — owner-approved replacements and addition

Replace **§ 11(1)**:

> **§ 11(1)** `Publish(payload)` borrows a single caller-owned `T` for the complete synchronous publication, including nested calls that retain a valid dependent borrow. This follows the normal call-site borrow rule for a parameter of type `ref T`; an owned addressable value or compatible owned temporary may be passed without moving it. The compiler must preserve the payload's required lifetime through all handler invocations in the publication.

Replace **§ 11(2)**:

> **§ 11(2)** Each invoked handler receives `ref T` to the **same published value**. Neither event dispatch nor a handler parameter transfers ownership of that referent; no handler obtains `ref mut T` from the publication, and a handler may not retain the reference beyond its legal lifetime. A handler can separately request an explicit owned copy using the ordinary copy rules when `T` is copyable.

Replace **§ 11(3)**:

> **§ 11(3)** There is no implicit payload clone, copy, move, or per-handler allocation, even when several handlers are registered and `T` is move-only. Event delivery and channel ownership transfer have distinct contracts (§ 26).

Append **§ 11(4)**:

> **§ 11(4)** The source-visible conceptual publication parameter contract is `Publish(data: ref T) void`. Owner-only publication rights remain unchanged. Compiler-known `Publish` typing, normal call-site borrow materialization, LSP and tests must implement this same parameter ownership contract rather than accepting a by-value argument through a hidden move.


## § 12 Panic — addition

Append **§ 12(4)**:

> **§ 12(4)** A handler panic cannot be reclassified as an event-specific failure result or silently converted into normal publication completion. Ordinary panic policy, cleanup and execution-context boundaries apply. Prior completed handler effects are not rolled back; event and subscription state must remain memory-safe during supported panic cleanup, including reentrant or mid-dispatch unregistration.

## § 13 Reentrancy — addition

Append **§ 13(4)**:

> **§ 13(4)** In Sec 0.1 there is no source-visible policy-block switch to disable reentrancy. Valid nested publication must obey the same access, ordering, borrow and storage-lifetime contracts as an independent publication. Reentrancy never permits concurrent unsynchronized publication merely by virtue of being synchronous within one caller.

Append **§ 13(5)** and **§ 13(6)**:

> **§ 13(5)** A reusable mutable capturing handler requires **exclusive** callable-environment access throughout its invocation, as required by its inferred callable capability. A statically provable overlapping reentrant or concurrent invocation is a compile-time error; a dynamically encountered exclusive-callable conflict **panics** rather than returning an event failure, silently skipping the handler, or violating exclusivity. Shared reusable callbacks can be recursively invoked when their own borrowing and effect requirements permit it.

> **§ 13(6)** A runtime exclusive-handler-invocation guard is released on ordinary callback return and along any supported panic cleanup path. Its potential panic must be reflected by canonical effect analysis, including `@noPanic` when applicable. No `mut fn` implementation spelling, special cancellation outcome or `Publish` error result is introduced.

## § 14 Mutation during dispatch — additions

Append **§ 14(4)**:

> **§ 14(4)** Every publication must maintain a stable distinction between registrations eligible for that invocation and registrations added after its start. A newly added registration may participate in a subsequent, including nested, publication started after registration, but not retroactively in the already-running publication.

Append **§ 14(5)**:

> **§ 14(5)** Logical unregistration takes effect immediately, even during the handler's own callback, and excludes the registration from future invocation turns. A currently executing callback completes normally; its callable/capture storage and borrow guards remain valid until all in-flight calls finish. Physical slot reclamation and generation reuse are deferred to that last use, including under nested publication.

Append **§ 14(6)**:

> **§ 14(6)** Each `Publish` fixes a **logical eligibility set** of registrations active at its start, in subscription order. Registrations added afterward cannot be invoked by that publication; they may be invoked by a nested publication started after their addition. A registration removed before its turn is skipped. An already-running invocation is not interrupted. The runtime may implement eligibility using slot generations/epoch records rather than copying closure objects or allocating a snapshot.

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

> **§ 20(3)** In the default Sec 0.1 inline event model, an event-owning value **must not move while any subscription to its event storage is live**. This includes relocation as a field/element of a moved aggregate, collection reallocation, replacement or other storage-identity-invalidating operation. After the last subscription and in-flight invocation have ended, ordinary move rules apply; implicit copying of event storage remains forbidden. The compiler must reject movement when it cannot prove the absence of live dependencies.

Append **§ 20(5)**:

> **§ 20(5)** A `Subscription` move transfers one registration owner, not the event, subscriber slot or callback identity itself. It does not duplicate subscription state, and closing the moved handle unregisters that same original registration exactly once.

## § 21 Address stability — addition

Append **§ 21(4)**:

> **§ 21(4)** Every live subscription pins the identity and location of its owning inline event storage under the Sec 0.1 source contract. The compiler must reject relocation of the event owner until every registration and callback dependency has ended; it may not silently enable relocation by heap-promoting, copying or redirecting the event storage. This pinning is a lifetime/ownership constraint, not an implicit persistent physical memory lock or a new public pin keyword.

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
> The event delivery contract is explicitly **borrowed `ref T`**, while the channel contract is ownership transfer on committed receive; see §§ 10–11 and `channels.md`.

## § 29 Compiler-known types — replacements and additions

Replace **§ 29(1)**:

> **§ 29(1)** The canonical compiler-known Sec 0.1 event type identities are `Event[T]`, `Event[T, N]`, `EventStorage[T, N]`, `Subscription`, and `EventSubscribeResult`. The optional event capacity is a compile-time integer argument, not an extra runtime parameter or an additional independent generic type. An `EventStorage` use requires explicit capacity. Compiler-only representation fields are not user-visible source fields.

Replace **§ 29(3)**:

> **§ 29(3)** `Subscribe` accepts a reusable `fn(ref T) void` callable or an exclusively borrowed reusable mutating callable of inferred `mut fn(ref T) void` capability, under § 10 and § 6. It returns the source-visible closed `EventSubscribeResult`; `Publish(data: ref T)` returns `void`; `Subscription.Close()` consumes its owning receiver and returns `void`. These source contracts apply even on a target/backend whose event runtime or lowering is not yet implemented. Named stateless function values and externally borrowed capturing handlers have different storage obligations but the same borrowed payload contract.

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

> **§ 30(2)** Semantic analysis must bind an event member to its exact declaring owner and existing/generated backing storage, retain the compile-time capacity and visibility/publication authority, validate borrowed `ref T` payload/callback signatures and reusable callable capabilities, and record the returned `EventSubscribeResult` lifecycle obligations. Each successful `Subscription` carries compiler-known lifetime and location dependencies on its owner event and any retained external callable storage even though the public handle is non-generic. The compiler must reject invalid implicit call-only borrows, move/relocation while live, overlapping exclusive callable borrows and unsupported configurations; failed `Full` registrations retain no such dependency.

Append **§ 30(3)**:

> **§ 30(3)** Semantic analysis must not mistake synchronous handler invocation for a detached task, implicit thread, channel delivery or proof of thread safety. Publication is an ordinary synchronous call/effect edge for effect analysis. Concurrency, ISR safety, reentrancy, relocation and unsubscribe-during-dispatch facts must be preserved for runtime lowering or rejected when the selected capability cannot be proved.

## § 31 Semantic IR — replacements and addition

Replace **§ 31(1)**:

> **§ 31(1)** Semantic IR must preserve typed event creation and storage identity, exposure, exact member/owner authority, subscription registration and lifecycle, publication and synchronous handler invocation, unregistration, dispatch ordering, fixed capacity, and lifetime/provenance/synchronization effects. These are **semantic facts**; this rulebook does not establish fixed IR opcode spellings such as `EventDispatchBegin` as a public or permanently stable IR vocabulary. Concrete instruction names and verification rules belong to `rules/compiler/semantic_ir.md`.

Replace **§ 31(2)**:

> **§ 31(2)** The IR must preserve resolved borrowed payload and capacity, owner and backing storage identity, source provenance, externally retained handler callable/capture reference and its shared/exclusive capability, subscription ownership and retained lifetime dependencies, active-invocation guard, synchronous dispatch ordering and per-invocation eligibility, slot reclamation, nested publication, dynamic reentrant exclusive-conflict panic, and unregister-before-turn semantics, plus any target/affinity limitations. Neither a source-valid event operation nor an implementation gap licenses emission of an incorrect backend stub.

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

> **§ 34(2)** Sec 0.1 additionally forbids an `impl event` policy block, inferred external publish authority, implicit task/thread dispatch, hidden subscriber-capacity growth, movement of any event owner with live subscriptions, implicit copying of move-only payloads or captures, hidden ownership of capturing callback environments, invalid retained callable references, one-shot handlers, unsafe lifetime extension, and assuming that an ordinary event is ISR-safe or thread-safe. A backend must preserve the approved borrowed `ref T` payload/callback ABI.

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
3. Apply the owner-approved borrowed payload and external callable storage contract in §§ 10–11 and the other amended clauses. The design discussion that began as MD-134 is **resolved**; do not reopen it or create another MD for the listed derived invariants. Verify the `mut fn` callable **type/capability** is never misrepresented as legal source syntax introducing a function/lambda implementation.
4. Create and register `governance/concurrency_events.yaml` from the companion merge proposal; keep source/runtime status out of the rulebook. Do not accidentally mark unimplemented dispatch or lowering `implemented`.
5. Cross-check `grammar.md` §§ 31, 109 (bodyless event exposure; compile-time capacity), `lambda-functions.md`, `functions.md`, `ownership.md`, `borrowing.md`, `transferability.md`, `destruction.md`, `concurrency_memory_model.md`, `interrupts.md`, `semantic_ir.md`, and current core/frontend source declarations. Do not modify these already newer books unless a confirmed contradiction requires its own controlled correction.
6. Add conformance tests as tracked by governance: valid short/full forms, capacity bounds, owner-only publication, exact `EventSubscribeResult`, consuming `Close`, `fn(ref T)` and inferred mutable callable handler capability, external capture storage/lifetimes, move-only payload borrowing, interface observation, no policy block, owner pinning, synchronous ordering, per-invocation eligibility, deferred slot reclamation, static rejection and runtime panic for exclusive-handler reentrancy, concurrent access/ISR rejection, IR and target lowering.
7. Validate governance schema (`go run ./cmd/yamlstatus check`), test parser/Sema, and verify every old `§ n(k)` reference still exists exactly once. Do not claim runtime tests passed without running them.

**Owner decisions incorporated (2026-11-13):** Borrowed `ref T` payload and callback, retained external capturing callable storage (`ref` or `ref mut` depending on inferred callable capability), event-owner immobility while registered, runtime panic for dynamically detected exclusive-handler reentrancy, immediate logical unregistration with deferred active-callback release, per-publication eligibility snapshots, and physical slot reservation through final use. These are not claims of compiler implementation.

**Baseline note:** This correction was prepared by reading repository source and normative books at the verified commit. No implementation files, repository governance or tests have been modified, and no local compiler suite has been run.
