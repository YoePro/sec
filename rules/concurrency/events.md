# Events

- **Status:** Normative — synchronization required
- **Created:** 2026-07-29
- **Last updated:** 2026-11-13 (independent modernization clauses applied; handler/payload synchronization awaits MD-134)
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/events.md`
- **Repository baseline reviewed:** `ee75e5d036d6611519a1b34d570bb6778002acfd`
- **Related rulebooks:** `rules/concurrency/channels.md`, `rules/concurrency/select.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/platform/interrupts.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/layout.md`, `rules/compiler/semantic_ir.md`, `rules/foundations/grammar.md`, `rules/declarations/lambda-functions.md`, `rules/concurrency/concurrency.md`, `rules/memory/transferability.md`, `rules/memory/destruction.md`, `rules/platform/target_profiles.md`

- **Implementation governance:** `governance/concurrency_events.yaml` (`concurrency.events-v2`)
- **Partially applied correction:** `rules/corrections/events-normative-fix-20261113.md`
- **Open synchronization decision:** MD-134 in `missing-decisions.yaml`; §§ 10–11 remain unchanged pending the owner's separate correction. Revision 2.0 is not yet fully synchronized.

---

## § 1 Purpose

**§ 1(1)** A Sec event is an instance-owned, typed, synchronous publish/subscribe member for software notifications. In Sec 0.1 its storage is part of the owning value, either through a short-form `Event[T]` field or through an explicit `EventStorage[T, Capacity]` field exposed by its owning `impl`. A future type-level/static event form is not implicitly enabled by this rule.

**§ 1(2)** Typical uses include GUI notifications, component state changes, device
notifications, domain events, plugin hooks and diagnostic hooks.

**§ 1(3)** Events are not channels, completions, hardware interrupts, timers, semaphores,
cancellation objects or waitable synchronization signals.

## § 2 Core rule
**§ 2(1)** An instance event contains state and therefore belongs to the data definition of
its owning type.

**§ 2(2)** An `impl` block must not silently add per-instance event storage.

**§ 2(3)** Short form:

```sec
type Button struct {
    ButtonPressed: Event[ButtonPressData]
}
```

**§ 2(4)** The event is part of `Button` layout and lifetime.

**§ 2(5)** The only Sec 0.1 `impl` event declaration is `event Name using existingStorage`, with no body. It exposes an existing instance-storage field and does not allocate or synthesize another field. `grammar.md` § 31 owns the grammar; a trailing policy block is invalid syntax.

## § 3 Default policy

**§ 3(1)** For Sec 0.1, `Event[T]` has the following exact default properties:

```text
subscriber capacity: 4
dispatch: synchronous, in the publisher's current execution context
dispatch order: subscription order
publication: only from the declaring type's valid owning impl context
subscription: according to event-member visibility
storage: fixed-capacity inline instance storage
dynamic allocation: none
implicit concurrent thread safety: none
ordinary ISR publication: not permitted
```

These are language-visible guarantees, not permission for a backend to silently select an allocating, asynchronous or thread-safe substitute. The payload argument and handler-parameter ownership model remain subject to MD-134.

**§ 3(2)** The default capacity is four concurrent subscriptions.

**§ 3(3)** Programs requiring another capacity must declare it explicitly.

## § 4 Data and behavior
**§ 4(1)** The type declaration defines storage:

```sec
type Button struct {
    ButtonPressed: Event[ButtonPressData]
}
```

**§ 4(2)** The owning implementation defines publication behavior:

```sec
impl Button {
    fn Press(data: ButtonPressData) void {
        ButtonPressed.Publish(data)
    }
}
```

**§ 4(3)** The event does not need to be repeated in `impl` when the default policy is
sufficient.

**§ 4(4)** `Subscribe` and `Publish` operate on compiler-managed event state. Their availability on event members must not be generalized into a source-level right to mutate arbitrary fields through a shared reference. The compiler must prove the event member, receiver, operation, lifetime, and applicable concurrent-access policy. Subscription operations are usable on an accessible event through an ordinary `ref` owner where the owning event contract allows them; this does not make the default event thread-safe.

## § 5 Publication rights
**§ 5(1)** Publication authority belongs to the `impl` of the event's declaring owner type and its valid same-owner extensions under the ordinary `impl` coherence rules. The compiler validates declaration identity and receiver authority; it must not infer publisher rights solely from visibility, field name, an interface requirement, or access to the underlying storage representation.

**§ 5(2)** Outside the owning `impl`, publication is forbidden by default:

```sec
button.ButtonPressed.Publish(data)
```

**§ 5(3)** Expected diagnostic:

```text
event ButtonPressed may only be published by Button
```

**§ 5(4)** External code may observe and subscribe according to visibility.

**§ 5(5)** Sec 0.1 has no public event-publication modifier or general external-publisher capability. Only already-defined owner publication is legal; the presence of a readable or subscribable event member grants no right to invoke `Publish` externally.

## § 6 Subscription
```sec
let subscription := button.ButtonPressed.Subscribe(OnButtonPressed)
```

**§ 6(1)** `Subscribe(handler)` returns the exact closed `EventSubscribeResult` union with `Subscribed(Subscription)` or `Full`. Only `Subscribed` contains an owning `Subscription` value. That value is move-only, represents one registration, and must not be implicitly copied or abandoned as if it carried no cleanup obligation.

**§ 6(2)** `Subscription.Close()` is a **consuming** operation on the subscription owner. A successful close unregisters the associated handler, releases the subscription's lifecycle obligation, and leaves the consumed source handle unavailable. A second `Close()` through that source is invalid.

**§ 6(3)** Ordinary deterministic destruction of a still-owned `Subscription` performs the same unregistration and resource cleanup as `Close()`. A moved subscription is cleaned up only by its current owner. No subscription registration or captured handler resource may be destroyed twice.

**§ 6(4)** A registered handler environment must not retain captures or references whose lifetime, provenance, ownership authority or execution affinity expires before its last possible invocation. Handler storage and captures are subject to the canonical callable-value, borrowing, transferability and destruction rules; registering a closure does not extend an invalid borrow or silently clone move-only captures.

**§ 6(5)** A live subscription may not refer to event backing storage that has been destroyed, invalidated or relocated in a way forbidden by the selected event-storage contract. Destruction, replacement and movement of the event owner must account for every remaining subscription. Compiler analysis may accept an operation only when the required lifetime and storage-stability proof is established; a missing proof is not permission to leave a dangling subscription.

**§ 6(6)** Removing a subscription during its own currently executing synchronous callback is legal under § 14. Unregistration prevents subsequent invocations but must not destroy callable/capture state still required by the active invocation. Destruction of that state occurs after its final required use.

## § 7 Capacity

**§ 7(1)** The default event accepts at most four simultaneous subscriptions.

**§ 7(2)** When the subscription capacity is exhausted, `Subscribe` returns `EventSubscribeResult.Full` with **no** `Subscription` owner created. This is an ordinary closed-union outcome, not a `Result` error or a hidden allocation request.

**§ 7(3)** It must not allocate, grow, overwrite or silently remove another subscriber.

**§ 7(4)** Conceptually:

```sec
match button.ButtonPressed.Subscribe(handler) {
    Subscribed(subscription) => Store(subscription)
    Full => HandleCapacity()
}
```

**§ 7(5)** The exact returned source-visible type is `EventSubscribeResult`, whose only variants are `Subscribed(Subscription)` and `Full` (see § 29). A caller must handle or explicitly dispose of this result according to ordinary result/ownership rules; discarded `Subscribed` payloads are never silently leaked. Argument transfer and cleanup when `Full` is returned follow ordinary call/capture rules until a more specific accepted event rule states otherwise.

## § 8 Explicit capacity
**§ 8(1)** A different fixed capacity may be selected:

```sec
type Button struct {
    ButtonPressed: Event[ButtonPressData, 8]
}
```

**§ 8(2)** Capacity is compile-time known and must be greater than zero.

**§ 8(3)** It contributes fixed storage to the owning type.

**§ 8(4)** `Event[T]` without a capacity uses the default four slots. `Event[T, N]` and `EventStorage[T, N]` require a positive compile-time constant capacity `N`. `EventStorage[T]` has no implicit default-capacity spelling in Sec 0.1. Capacity participates in the concrete event-storage representation and layout; the compiler must not silently replace it with a dynamic or runtime-sized allocation.

## § 9 Dispatch
**§ 9(1)** Default dispatch is synchronous.

```sec
ButtonPressed.Publish(data)
```

**§ 9(2)** invokes all current handlers before returning.

**§ 9(3)** Publication does not spawn tasks, create threads, enqueue work or allocate.

**§ 9(4)** Handlers run in subscription order.

**§ 9(5)** Synchronous publication invokes eligible handlers as ordinary synchronous callable executions in the publisher's current execution context. It introduces no implicit task, native thread, process, scheduler wait, channel send, or completion edge. Any effects, cancellation requirements, panic behavior or externally visible side effects of invoked handlers remain ordinary transitive effects of the publication operation.

**§ 9(6)** A nested publication triggered by a handler is a separate synchronous publication. Each invocation applies §§ 13–14 to the registration state visible at that invocation. The backend may use snapshots or stable slot/generation records, but must not invoke removed registrations, skip eligible current registrations, or violate publication ordering.

## § 10 Handler type

**§ 10(1)** Version 0.1 handlers conceptually have the form:

```sec
fn Handler(data: T) void
```

**§ 10(2)** Handlers return `void`.

**§ 10(3)** Business errors must be handled by the handler or represented through explicit
program state.

## § 11 Payload ownership

**§ 11(1)** Default publication borrows the payload during synchronous dispatch.

**§ 11(2)** Handlers do not take ownership unless a future policy explicitly defines that.

**§ 11(3)** Move-only payloads must not be implicitly cloned for multiple subscribers.

## § 12 Panic
**§ 12(1)** An unhandled handler panic follows the general panic rules.

**§ 12(2)** The event model does not define a separate exception system.

**§ 12(3)** Version 0.1 does not promise that later handlers run after an unhandled panic.

**§ 12(4)** A handler panic cannot be reclassified as an event-specific failure result or silently converted into normal publication completion. Ordinary panic policy, cleanup and execution-context boundaries apply. Prior completed handler effects are not rolled back; event and subscription state must remain memory-safe during supported panic cleanup, including reentrant or mid-dispatch unregistration.

## § 13 Reentrancy
**§ 13(1)** A handler may call methods on the publisher and may cause nested publication.

**§ 13(2)** Nested publication is permitted unless the owning type or selected event policy
forbids it.

**§ 13(3)** The implementation must preserve memory safety and subscription-list integrity.

**§ 13(4)** In Sec 0.1 there is no source-visible policy-block switch to disable reentrancy. Valid nested publication must obey the same access, ordering, borrow and storage-lifetime contracts as an independent publication. Reentrancy never permits concurrent unsynchronized publication merely by virtue of being synchronous within one caller.

## § 14 Mutation during dispatch
**§ 14(1)** A handler may close its own subscription or another subscription it owns.

**§ 14(2)** A subscription removed before its dispatch turn is not invoked.

**§ 14(3)** A subscription added during publication does not participate until the next
publication.

**§ 14(4)** Every publication must maintain a stable distinction between registrations eligible for that invocation and registrations added after its start. A newly added registration may participate in a subsequent, including nested, publication started after registration, but not retroactively in the already-running publication.

**§ 14(5)** Unregistration must not invalidate an in-flight invocation's still-live callable environment. The runtime must preserve storage and handler lifetime until the active call has finished, while making that registration unavailable to later dispatch turns. Physical representations may differ if the observable contract is preserved.

## § 15 Full storage form
**§ 15(1)** Programs requiring explicit storage control may declare:

```sec
type Button struct {
    buttonPressedStorage: EventStorage[ButtonPressData, 8]
}
```

**§ 15(2)** and expose it through `impl`:

```sec
impl Button {
    event ButtonPressed using buttonPressedStorage
}
```

**§ 15(3)** This separates:

```text
EventStorage
    layout, capacity, ownership and storage policy

event member
    public name, access policy and publication rights
```

**§ 15(4)** `EventStorage[T, N]` is a compiler-known, non-implicitly-copyable instance-storage capability. `event Name using field` must name an existing compatible storage member of the same owning type. A valid `impl` exposes that storage but cannot add hidden instance fields, silently change its capacity, invent an event policy block or grant publication authority to arbitrary external code.

## § 16 Short-form lowering
**§ 16(1)** The short form:

```sec
type Button struct {
    ButtonPressed: Event[ButtonPressData, 8]
}
```

**§ 16(2)** The short form is semantically equivalent, for ownership, fixed storage, capacity, public event behavior, member access and layout, to an associated generated storage identity plus an event exposure using the same default policy. This equivalence is a compiler semantic model; it does not grant user source access to generated field names.

**§ 16(3)** Conceptual model only (not a legal extra source declaration a user must write):

```sec
type Button struct {
    __ButtonPressedStorage: EventStorage[ButtonPressData, 8]
}

impl Button {
    event ButtonPressed using __ButtonPressedStorage
}
```

The backend must preserve the short form's single event-member identity and must not accidentally expose, shadow or duplicate the internal generated storage name.

**§ 16(4)** The generated storage is still part of layout and must appear in ABI and layout
reports.

**§ 16(5)** It must not use hidden dynamic allocation.

**§ 16(6)** The selected lower-level representation cannot retroactively change an instance event into an automatically shared static event, or change the source-visible `Event[T, N]` and `EventStorage[T, N]` identities into unrelated public APIs.

## § 17 Impl customization

**§ 17(1)** Sec 0.1 uses an `impl` event declaration to expose an existing `EventStorage[T, N]` field as an event member. The supported form defines **exposure**, not configurable policy. It is not necessary for the short-form `Event[T, N]` field whose default exposure is already defined.

**§ 17(2)** Canonical Sec 0.1 source:

```sec
type Button struct {
    buttonPressedStorage: EventStorage[ButtonPressData, 8]
}

impl Button {
    event ButtonPressed using buttonPressedStorage
}
```

No policy block follows the declaration.

**§ 17(3)** `grammar.md` § 31 forbids an `impl event` body. Source such as `event ButtonPressed using buttonPressedStorage { publish: owner }` is **invalid in Sec 0.1** and must not be parsed as a supported feature. Public policy-block syntax and any configurable dispatch policy belong to a separately approved future language revision.

**§ 17(4)** The approved division is: storage belongs to the owner type; the `impl` supplies the event's exposure name and binds it to existing storage; the default Sec 0.1 event policy belongs to the language and is not user-configurable through an `impl` body.

## § 18 Visibility and external publication

**§ 18(1)** Normal field visibility controls whether external code can observe and subscribe.

**§ 18(2)** Visibility does not grant publication rights.

**§ 18(3)** No general external event-publication capability, `public publish` modifier or alternative publisher access is defined for Sec 0.1. Such a feature may be considered in a later revision, but neither a visible event member nor interface conformance enables it today.

**§ 18(4)** The language must distinguish:

```text
visible
subscribable
publishable
```

## § 19 Interfaces
**§ 19(1)** An interface may require an observable event member:

```sec
interface PressSource {
    event ButtonPressed[ButtonPressData]
}
```

**§ 19(2)** This grants observation/subscription semantics, not publication rights.

**§ 19(3)** A type conforms to an interface event requirement only when a visible matching event member exposes the required payload type through its permitted event form. Conformance never materializes extra instance storage, changes the concrete capacity, grants publication rights or gives the interface view a direct storage-mutation capability beyond the event operations it canonically exposes.

## § 20 Copy and move
**§ 20(1)** `Event[T, N]` and `EventStorage[T, N]` are not implicitly copyable resource-bearing event storage. A containing value cannot become implicitly copyable when that would copy its event storage or duplicate live subscriptions. Ordinary copy/move classification remains compositional with the compiler-known event-storage restrictions.

**§ 20(2)** Copying an event-owning type must not duplicate active subscriptions.

**§ 20(3)** An event-owning value may move only when the compiler proves the selected representation preserves event storage identity, active subscription validity, callback/capture storage, ownership and address/provenance constraints. In particular, moving an owner **before** any subscription may be valid; moving one with live subscriptions is not implicitly valid and must be rejected if safety cannot be proved.

**§ 20(4)** The compiler may require stable address or reject moves after subscription.

**§ 20(5)** A `Subscription` move transfers one registration owner, not the event, subscriber slot or callback identity itself. It does not duplicate subscription state, and closing the moved handle unregisters that same original registration exactly once.

## § 21 Address stability
**§ 21(1)** Inline event storage may require address stability.

**§ 21(2)** The compiler may pin the owner, reject later moves, use address-independent slot
representation or require explicit stable storage.

**§ 21(3)** The selected strategy must be visible to ownership and escape analysis.

**§ 21(4)** The compiler must account for any subscription handle that retains a reference, location, generation or storage identity dependent on the owner. It may use a proven stable representation or reject relocation while subscriptions remain live; no implicit heap-promotion or hidden pinned allocation is introduced to make otherwise-invalid movement legal.

## § 22 Allocation

**§ 22(1)** The default event performs no hidden allocation.

**§ 22(2)** A dynamically allocating or arena-backed event policy is **not a selectable public Sec 0.1 event policy**. A future approved policy must state its allocation and failure semantics explicitly; the present default and explicit-capacity inline storage forms must not allocate to grow subscriber capacity or repair a capacity error.

**§ 22(3)** Reaching default capacity must not trigger allocation.

## § 23 Thread safety
**§ 23(1)** The default event is not implicitly thread-safe.

**§ 23(2)** Concurrent publication, subscription or unsubscription of the **same** default event storage is invalid without a separately proven synchronization arrangement covering its complete internal state and handler lifetimes. Sec 0.1 provides no selectable built-in `threadSafe` event policy. Cross-execution access is constrained by the ordinary transferability, borrowing, data-race and memory-model rules; shared-reference syntax alone does not prove safety.

**§ 23(3)** A type must not pay thread-synchronization cost when the event is used only
locally.

**§ 23(4)** Safe use through an external mutex or another canonical synchronization mechanism must preserve the event's synchronous callbacks, reentrancy and lifetime constraints; a backend must not invent a hidden lock that would change callback ordering or create unmodeled deadlock behavior. Synchronous event dispatch by itself establishes no inter-thread synchronization edge.

## § 24 Tasks and threads
**§ 24(1)** Events may be used from tasks and threads when policy permits.

**§ 24(2)** Synchronous handlers run in the publishing execution context.

**§ 24(3)** They do not automatically run on the execution entity that subscribed.

**§ 24(4)** Cross-context delivery requires an explicit channel, dispatcher or adapter.

**§ 24(5)** An event handler runs under the real execution context of the publisher. A task publication retains that task's logical context; a physical-thread publication retains that physical thread's identity; ordinary `main` does not gain an implicit `TaskContext`. Handler calls participate in ordinary transitive effect/cancellation analysis and may not silently migrate to the subscribing execution entity.

## § 25 Interrupts
**§ 25(1)** Ordinary event publication is not ISR-safe.

**§ 25(2)** An ISR must not invoke arbitrary subscribers, allocate, block or acquire ordinary
mutexes.

**§ 25(3)** ISR code should instead use an ISR-safe channel, bounded notification, atomic
state or deferred work that later publishes the event.

**§ 25(4)** No special `@interruptSafe` event publication policy is defined for ordinary `Event[T, N]` in Sec 0.1. A later explicitly defined ISR-safe adapter must separately prove no-allocation, no-blocking, bounded progress, reentrancy, interrupt-context validity and suitable publication ordering. ISR-safe handoff through separately approved channels/atomics/deferred work does not make direct event dispatch ISR-safe.

## § 26 Events versus channels

```text
Event[T]
    One publication invokes zero or more subscribed handlers.
    Sec 0.1 default dispatch is synchronous, in the publisher's context.
    Handler invocation does not by itself queue work or establish
    completion/message-transfer synchronization.

Channel[T]
    One committed receive transfers one delivered message to one
    receiving operation under channels.md. Send/receive may wait,
    and a channel has its own ownership and synchronization contract.
```

This comparison does not define the event handler parameter or payload ownership mode; §§ 10–11 own that contract. Do not infer its answer from the channel comparison.

## § 27 Events versus completion

**§ 27(1)** Completion is one-shot terminal readiness.

**§ 27(2)** An event may publish repeatedly.

**§ 27(3)** Task, thread, process, DMA and I/O completion are therefore not ordinary events,
although adapters may expose them as events.

## § 28 Events versus interrupts

**§ 28(1)** A hardware interrupt is an asynchronous control transfer to an ISR.

**§ 28(2)** An event is a software publish/subscribe object.

**§ 28(3)** An ISR may trigger deferred work that later publishes an event.

## § 29 Compiler-known types
**§ 29(1)** The canonical compiler-known Sec 0.1 event type identities are `Event[T]`, `Event[T, N]`, `EventStorage[T, N]`, `Subscription`, and `EventSubscribeResult`. The optional event capacity is a compile-time integer argument, not an extra runtime parameter or an additional independent generic type. An `EventStorage` use requires explicit capacity. Compiler-only representation fields are not user-visible source fields.

**§ 29(2)** It must understand owner-only publication and external subscription capabilities.

**§ 29(3)** `Subscribe(handler)` returns the source-visible closed `EventSubscribeResult`; `Publish(payload)` returns `void`; `Subscription.Close()` consumes its owning receiver and returns `void`. These identities are source API contracts even on a target/backend whose event runtime or lowering is not yet implemented. The exact handler type and payload delivery contract are governed by §§ 10–11.

**§ 29(4)** The exact Sec 0.1 `EventSubscribeResult` union is:

```sec
type EventSubscribeResult union {
    // Registration succeeded and the caller owns exactly one subscription.
    Subscribed(Subscription)

    // Fixed subscriber capacity was reached; no subscription was created.
    Full
}
```

The union has no implicit additional variants, error payload, `Result` wrapper or allocation fallback.

**§ 29(5)** The compiler-known source identity `Subscription` is move-only and deterministically unregisters on consuming `Close()` or owner destruction. Its ordinary implementation representation remains private; no address-stability, ownership or runtime result semantics may be inferred from arbitrary source-level user-declared lookalike types.

## § 30 Semantic analysis
**§ 30(1)** The compiler must track:

- payload type;
- capacity;
- storage backend;
- owner type;
- publication rights;
- visibility;
- subscription ownership;
- handler lifetime;
- captures;
- dispatch order;
- reentrancy;
- active dispatch;
- mutation during dispatch;
- thread-safety policy;
- allocation capability;
- address stability;
- move/copy restrictions;
- target support.

**§ 30(2)** Semantic analysis must bind an event member to its exact declaring owner and existing/generated backing storage, retain the compile-time capacity and visibility/publication authority, validate the exact handler signature defined by §§ 10–11, and record the returned `EventSubscribeResult` lifecycle obligations. Subscription lifetime, closure captures and active-callback uses remain subject to ordinary borrowing/escape/ownership rules. Unsupported storage/policy configurations are errors, not silent backend fallbacks.

**§ 30(3)** Semantic analysis must not mistake synchronous handler invocation for a detached task, implicit thread, channel delivery or proof of thread safety. Publication is an ordinary synchronous call/effect edge for effect analysis. Concurrency, ISR safety, reentrancy, relocation and unsubscribe-during-dispatch facts must be preserved for runtime lowering or rejected when the selected capability cannot be proved.

## § 31 Semantic IR
**§ 31(1)** Semantic IR must preserve typed event creation and storage identity, exposure, exact member/owner authority, subscription registration and lifecycle, publication and synchronous handler invocation, unregistration, dispatch ordering, fixed capacity, and lifetime/provenance/synchronization effects. These are **semantic facts**; this rulebook does not establish fixed IR opcode spellings such as `EventDispatchBegin` as a public or permanently stable IR vocabulary. Concrete instruction names and verification rules belong to `rules/compiler/semantic_ir.md`.

**§ 31(2)** The IR must preserve resolved payload and capacity, owner and backing storage identity, source provenance, handler callable/capture representation, subscription ownership, active-invocation lifetime, dispatch order, nested publication and unregister-before-turn semantics, and any target/affinity limitations. Neither a source-valid event operation nor an implementation gap licenses emission of an incorrect backend stub.

**§ 31(3)** A backend without conforming event subscriber storage, dispatch, cleanup or memory-safe callback lifetime machinery must diagnose unsupported lowering and produce no invalid executable artifact. Normative source semantics and backend implementation status are distinct; the latter is tracked in governance.

## § 32 Diagnostics
**§ 32(1)** Examples:

```text
event capacity must be greater than zero
```

```text
event ButtonPressed has reached its subscriber capacity of 4
```

```text
event ButtonPressed may only be published by Button
```

```text
subscription handler captures value local that does not outlive the subscription
```

```text
event-owning value button cannot be moved while subscriptions are active
```

```text
event publication is not thread-safe under the selected policy
```

```text
event publication is not permitted in an interrupt routine
```

```text
copying Button would duplicate event storage and subscriptions
```

**§ 32(2)** Diagnostic IDs must remain stable and distinguish parse, semantic, ownership,
concurrency and target-profile rules.

**§ 32(3)** The compiler must distinguish source-invalid event declarations or calls from missing backend implementation. The former receive canonical syntax, typing, visibility, ownership, lifetime and target-policy diagnostics with stable IDs; an implementation limitation must be identified as such rather than rewriting the legal source contract. A `Full` result is a normal source-visible event outcome, not a compile-time error merely because the event can reach capacity at runtime.

## § 33 Implementation governance boundary

**§ 33(1)** This rulebook defines the Sec 0.1 **normative event contract** independently of implementation coverage. Authoritative implementation evidence, existing compiler/parser/Sema/LSP functionality, integrated status, missing runtime support and pending test work are maintained in `governance/concurrency_events.yaml` under integration `concurrency.events-v2`. No mutable implementation inventory is part of this normative section.

**§ 33(2)** Tooling and implementers must distinguish an already-defined source contract from a backend capability not yet integrated. A source-valid event construct cannot be redefined as syntactically or semantically illegal simply because registration, dispatch or lowering is incomplete; where required runtime capability is unavailable, target/profile or unsupported-lowering diagnostics apply under the canonical compiler rules. Where the normative contract is incomplete, it must be resolved by the owner rather than guessed from implementation shortcuts.

## § 34 Restrictions
**§ 34(1)** An event must not:

- be declared only in `impl` when it requires per-instance storage;
- add hidden instance storage from `impl`;
- allocate under the default policy;
- grow beyond capacity;
- silently replace subscribers;
- grant publication through visibility alone;
- clone move-only payloads;
- copy active subscriptions implicitly;
- dispatch asynchronously by default;
- run arbitrary handlers from ISR;
- pretend to be a channel, completion or interrupt;
- hide synchronization cost.

**§ 34(2)** Sec 0.1 additionally forbids an `impl event` policy block, inferred external publish authority, implicit task/thread dispatch, hidden subscriber-capacity growth, invalid active-subscription relocation, implicit cloning of move-only captures or subscription handles, unsafe lifetime extension, and assuming that an ordinary event is ISR-safe or thread-safe. A backend must preserve the canonical handler and payload ownership contract once §§ 10–11 are synchronized; it must not silently change the handler ABI.

## § 35 Future extensions
**§ 35(1)** Possible future additions include dynamic/arena-backed subscriber storage, selectable thread-safe policies, asynchronous dispatch adapters, weak subscriptions, filters, priority ordering, event-to-channel adapters, external publication capabilities and type-level/static event forms. This list is **non-normative**; none of these source features is enabled by Sec 0.1 without its own approved rulebook/grammar/API revision.

**§ 35(2)** A future policy must explicitly specify syntax, owner/storage model, handler/payload ownership, cancellation/panic handling, concurrency synchronization, allocation and error behavior, dispatch mutation and reentrancy, interface exposure, and target capability. The mere presence of an example in an older document is not permission to implement a future feature.

## § 36 Related rules

```text
rules/foundations/grammar.md
rules/declarations/struct.md
rules/declarations/impl.md
rules/declarations/interfaces.md
rules/declarations/functions.md
rules/declarations/lambda-functions.md
rules/memory/ownership.md
rules/memory/borrowing.md
rules/memory/destruction.md
rules/memory/lifetime_analysis.md
rules/memory/layout.md
rules/memory/transferability.md
rules/concurrency/concurrency.md
rules/concurrency/concurrency_memory_model.md
rules/concurrency/channels.md
rules/concurrency/tasks.md
rules/concurrency/threads.md
rules/concurrency/cancellation.md
rules/platform/interrupts.md
rules/platform/target_profiles.md
rules/compiler/semantic_ir.md
rules/tooling/diagnostics.md
```
