# Events

- **Status:** Normative — synchronization required
- **Created:** 2026-07-29
- **Last updated:** 2026-11-07 (stable chapter and paragraph numbering)
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/events.md`
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`
- **Related rulebooks:** `rules/concurrency/channels.md`, `rules/concurrency/select.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/platform/interrupts.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/layout.md`, `rules/compiler/semantic_ir.md`

---

## § 1 Purpose

**§ 1(1)** Events provide typed publish/subscribe communication owned by a value or type.

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

## § 3 Default policy

**§ 3(1)** `Event[T]` uses these version 0.1 defaults:

```text
subscriber capacity: 4
dispatch: synchronous
dispatch order: subscription order
publication: owning impl only
subscription: according to field visibility
storage: inline fixed-capacity storage
allocation: none
thread safety: not implied
ISR publication: not implied
```

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

## § 5 Publication rights

**§ 5(1)** Inside the owning `impl`, the event exposes publication capability.

**§ 5(2)** Outside the owning `impl`, publication is forbidden by default:

```sec
button.ButtonPressed.Publish(data)
```

**§ 5(3)** Expected diagnostic:

```text
event ButtonPressed may only be published by Button
```

**§ 5(4)** External code may observe and subscribe according to visibility.

## § 6 Subscription

```sec
let subscription := button.ButtonPressed.Subscribe(OnButtonPressed)
```

**§ 6(1)** A successful subscription returns a move-only `Subscription`.

```sec
subscription.Close()
```

**§ 6(2)** closes the subscription and unregisters the handler.

**§ 6(3)** Destroying the subscription has the same effect.

**§ 6(4)** The event must not retain captures that do not outlive the subscription.

## § 7 Capacity

**§ 7(1)** The default event accepts at most four simultaneous subscriptions.

**§ 7(2)** When full, `Subscribe` returns an explicit failure outcome.

**§ 7(3)** It must not allocate, grow, overwrite or silently remove another subscriber.

**§ 7(4)** Conceptually:

```sec
match button.ButtonPressed.Subscribe(handler) {
    Subscribed(subscription) => Store(subscription)
    Full => HandleCapacity()
}
```

**§ 7(5)** The final outcome names belong to the core API.

## § 8 Explicit capacity

**§ 8(1)** A different fixed capacity may be selected:

```sec
type Button struct {
    ButtonPressed: Event[ButtonPressData, 8]
}
```

**§ 8(2)** Capacity is compile-time known and must be greater than zero.

**§ 8(3)** It contributes fixed storage to the owning type.

## § 9 Dispatch

**§ 9(1)** Default dispatch is synchronous.

```sec
ButtonPressed.Publish(data)
```

**§ 9(2)** invokes all current handlers before returning.

**§ 9(3)** Publication does not spawn tasks, create threads, enqueue work or allocate.

**§ 9(4)** Handlers run in subscription order.

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

## § 13 Reentrancy

**§ 13(1)** A handler may call methods on the publisher and may cause nested publication.

**§ 13(2)** Nested publication is permitted unless the owning type or selected event policy
forbids it.

**§ 13(3)** The implementation must preserve memory safety and subscription-list integrity.

## § 14 Mutation during dispatch

**§ 14(1)** A handler may close its own subscription or another subscription it owns.

**§ 14(2)** A subscription removed before its dispatch turn is not invoked.

**§ 14(3)** A subscription added during publication does not participate until the next
publication.

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

## § 16 Short-form lowering

**§ 16(1)** The short form:

```sec
type Button struct {
    ButtonPressed: Event[ButtonPressData, 8]
}
```

**§ 16(2)** is semantically equivalent to compiler-generated storage plus an event view using
the default policy.

**§ 16(3)** Conceptually:

```sec
type Button struct {
    __ButtonPressedStorage: EventStorage[ButtonPressData, 8]
}

impl Button {
    event ButtonPressed using __ButtonPressedStorage
}
```

**§ 16(4)** The generated storage is still part of layout and must appear in ABI and layout
reports.

**§ 16(5)** It must not use hidden dynamic allocation.

## § 17 Impl customization

**§ 17(1)** An `impl` event declaration is needed only when default policy is insufficient.

**§ 17(2)** Conceptual form:

```sec
type Button struct {
    buttonPressedStorage: EventStorage[ButtonPressData, 8]
}

impl Button {
    event ButtonPressed using buttonPressedStorage {
        publish: owner
        subscribe: public
        dispatch: synchronous
        order: subscription
    }
}
```

**§ 17(3)** The exact policy-block syntax remains open.

**§ 17(4)** The semantic division is fixed:

- storage belongs to the type;
- policy and exposure belong to `impl`.

## § 18 Visibility and external publication

**§ 18(1)** Normal field visibility controls whether external code can observe and subscribe.

**§ 18(2)** Visibility does not grant publication rights.

**§ 18(3)** External publication requires an explicit capability or modifier that is still
to be designed.

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

## § 20 Copy and move

**§ 20(1)** Event storage is not implicitly copyable.

**§ 20(2)** Copying an event-owning type must not duplicate active subscriptions.

**§ 20(3)** Moving an event-owning value is valid only when the selected storage backend and
all active subscriptions remain valid.

**§ 20(4)** The compiler may require stable address or reject moves after subscription.

## § 21 Address stability

**§ 21(1)** Inline event storage may require address stability.

**§ 21(2)** The compiler may pin the owner, reject later moves, use address-independent slot
representation or require explicit stable storage.

**§ 21(3)** The selected strategy must be visible to ownership and escape analysis.

## § 22 Allocation

**§ 22(1)** The default event performs no hidden allocation.

**§ 22(2)** An allocation-capable backend must be selected explicitly and follows allocation,
arena, failure and target-profile rules.

**§ 22(3)** Reaching default capacity must not trigger allocation.

## § 23 Thread safety

**§ 23(1)** The default event is not implicitly thread-safe.

**§ 23(2)** Concurrent publish, subscribe or unsubscribe requires an explicitly thread-safe
policy or external synchronization.

**§ 23(3)** A type must not pay thread-synchronization cost when the event is used only
locally.

## § 24 Tasks and threads

**§ 24(1)** Events may be used from tasks and threads when policy permits.

**§ 24(2)** Synchronous handlers run in the publishing execution context.

**§ 24(3)** They do not automatically run on the execution entity that subscribed.

**§ 24(4)** Cross-context delivery requires an explicit channel, dispatcher or adapter.

## § 25 Interrupts

**§ 25(1)** Ordinary event publication is not ISR-safe.

**§ 25(2)** An ISR must not invoke arbitrary subscribers, allocate, block or acquire ordinary
mutexes.

**§ 25(3)** ISR code should instead use an ISR-safe channel, bounded notification, atomic
state or deferred work that later publishes the event.

## § 26 Events versus channels

```text
Event[T]
    One publication fans out borrowed data to zero or more handlers.
    Default dispatch is synchronous.

Channel[T]
    One committed message transfers ownership to one receiver.
    Delivery may be buffered and asynchronous.
```

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

**§ 29(1)** The compiler recognizes:

```sec
Event[T]
Event[T, Capacity]
EventStorage[T, Capacity]
Subscription
EventSubscribeResult
```

**§ 29(2)** It must understand owner-only publication and external subscription capabilities.

**§ 29(3)** Current semantic analysis uses `EventSubscribeResult` for `Subscribe`.

**§ 29(4)** Its implemented variants are:

```text
Subscribed(Subscription)
Full
```

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

## § 31 Semantic IR

**§ 31(1)** At minimum:

```text
EventStorageCreate
EventStorageDestroy
EventExpose
EventSubscribe
EventUnsubscribe
EventPublish
EventDispatchBegin
EventDispatchHandler
EventDispatchEnd
SubscriptionMove
SubscriptionClose
```

**§ 31(2)** IR must record payload type, capacity, storage policy, owner type, access,
callable, captures, ordering, synchronization and source location.

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

## § 33 Current implementation status

**§ 33(1)** Implemented:

- parser support for short-form event fields:

```sec
ButtonPressed: Event[ButtonPressData]
ButtonPressed: Event[ButtonPressData, 8]
```

- parser support for explicit storage:

```sec
buttonPressedStorage: EventStorage[ButtonPressData, 8]
```

- parser support for `impl` event exposure:

```sec
event ButtonPressed using buttonPressedStorage
```

- parser support for interface event requirements:

```sec
event ButtonPressed[ButtonPressData]
```

- compiler-known types:

```sec
Event[T]
Event[T, Capacity]
EventStorage[T, Capacity]
Subscription
EventSubscribeResult
```

- default event capacity of 4 for `Event[T]`;
- positive capacity validation;
- `EventStorage` requiring explicit capacity;
- event members recorded on owning struct types;
- storage-backed event members recorded from `impl event ... using ...`;
- interface conformance checks for event payload type;
- `Publish(payload)` payload type checking;
- owner-only `Publish` diagnostics;
- `Subscribe(handler)` handler type checking for `fn(T) void`;
- `Subscription.Close()` as a compiler-known call.

**§ 33(2)** Not implemented yet:

- runtime subscriber storage;
- runtime dispatch;
- active subscription tracking;
- handler capture lifetime analysis specific to subscriptions;
- address-stability enforcement after subscription;
- thread-safe event policies;
- ISR-safe event policies;
- policy blocks after `impl event ... using ...`;
- Semantic IR lowering for event operations.

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

## § 35 Future extensions

**§ 35(1)** Possible later additions:

```text
dynamic subscriber storage
arena-backed storage
thread-safe policies
asynchronous dispatch adapters
weak subscriptions
filters
subscription priorities
event-to-channel adapters
public publication capabilities
static event short forms
```

## § 36 Related rules

```text
struct.md
impl.md
properties.md
declarations/interfaces.md
functions.md
closures.txt
ownership.md
borrowing.md
lifetime_analysis.md
allocation.md
layout.md
static.txt
channels.md
concurrency.md
interrupts.txt
diagnostics.md
```
