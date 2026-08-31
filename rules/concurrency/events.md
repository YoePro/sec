# Events

## Purpose

Events provide typed publish/subscribe communication owned by a value or type.

Typical uses include GUI notifications, component state changes, device
notifications, domain events, plugin hooks and diagnostic hooks.

Events are not channels, completions, hardware interrupts, timers, semaphores,
cancellation objects or waitable synchronization signals.

## Core rule

An instance event contains state and therefore belongs to the data definition of
its owning type.

An `impl` block must not silently add per-instance event storage.

Short form:

```sec
type Button struct {
    ButtonPressed: Event[ButtonPressData]
}
```

The event is part of `Button` layout and lifetime.

## Default policy

`Event[T]` uses these version 0.1 defaults:

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

The default capacity is four concurrent subscriptions.

Programs requiring another capacity must declare it explicitly.

## Data and behavior

The type declaration defines storage:

```sec
type Button struct {
    ButtonPressed: Event[ButtonPressData]
}
```

The owning implementation defines publication behavior:

```sec
impl Button {
    fn Press(data: ButtonPressData) void {
        ButtonPressed.Publish(data)
    }
}
```

The event does not need to be repeated in `impl` when the default policy is
sufficient.

## Publication rights

Inside the owning `impl`, the event exposes publication capability.

Outside the owning `impl`, publication is forbidden by default:

```sec
button.ButtonPressed.Publish(data)
```

Expected diagnostic:

```text
event ButtonPressed may only be published by Button
```

External code may observe and subscribe according to visibility.

## Subscription

```sec
let subscription := button.ButtonPressed.Subscribe(OnButtonPressed)
```

A successful subscription returns a move-only `Subscription`.

```sec
subscription.Close()
```

closes the subscription and unregisters the handler.

Destroying the subscription has the same effect.

The event must not retain captures that do not outlive the subscription.

## Capacity

The default event accepts at most four simultaneous subscriptions.

When full, `Subscribe` returns an explicit failure outcome.

It must not allocate, grow, overwrite or silently remove another subscriber.

Conceptually:

```sec
match button.ButtonPressed.Subscribe(handler) {
    Subscribed(subscription) => Store(subscription)
    Full => HandleCapacity()
}
```

The final outcome names belong to the core API.

## Explicit capacity

A different fixed capacity may be selected:

```sec
type Button struct {
    ButtonPressed: Event[ButtonPressData, 8]
}
```

Capacity is compile-time known and must be greater than zero.

It contributes fixed storage to the owning type.

## Dispatch

Default dispatch is synchronous.

```sec
ButtonPressed.Publish(data)
```

invokes all current handlers before returning.

Publication does not spawn tasks, create threads, enqueue work or allocate.

Handlers run in subscription order.

## Handler type

Version 0.1 handlers conceptually have the form:

```sec
fn Handler(data: T) void
```

Handlers return `void`.

Business errors must be handled by the handler or represented through explicit
program state.

## Payload ownership

Default publication borrows the payload during synchronous dispatch.

Handlers do not take ownership unless a future policy explicitly defines that.

Move-only payloads must not be implicitly cloned for multiple subscribers.

## Panic

An unhandled handler panic follows the general panic rules.

The event model does not define a separate exception system.

Version 0.1 does not promise that later handlers run after an unhandled panic.

## Reentrancy

A handler may call methods on the publisher and may cause nested publication.

Nested publication is permitted unless the owning type or selected event policy
forbids it.

The implementation must preserve memory safety and subscription-list integrity.

## Mutation during dispatch

A handler may close its own subscription or another subscription it owns.

A subscription removed before its dispatch turn is not invoked.

A subscription added during publication does not participate until the next
publication.

## Full storage form

Programs requiring explicit storage control may declare:

```sec
type Button struct {
    buttonPressedStorage: EventStorage[ButtonPressData, 8]
}
```

and expose it through `impl`:

```sec
impl Button {
    event ButtonPressed using buttonPressedStorage
}
```

This separates:

```text
EventStorage
    layout, capacity, ownership and storage policy

event member
    public name, access policy and publication rights
```

## Short-form lowering

The short form:

```sec
type Button struct {
    ButtonPressed: Event[ButtonPressData, 8]
}
```

is semantically equivalent to compiler-generated storage plus an event view using
the default policy.

Conceptually:

```sec
type Button struct {
    __ButtonPressedStorage: EventStorage[ButtonPressData, 8]
}

impl Button {
    event ButtonPressed using __ButtonPressedStorage
}
```

The generated storage is still part of layout and must appear in ABI and layout
reports.

It must not use hidden dynamic allocation.

## Impl customization

An `impl` event declaration is needed only when default policy is insufficient.

Conceptual form:

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

The exact policy-block syntax remains open.

The semantic division is fixed:

- storage belongs to the type;
- policy and exposure belong to `impl`.

## Visibility and external publication

Normal field visibility controls whether external code can observe and subscribe.

Visibility does not grant publication rights.

External publication requires an explicit capability or modifier that is still
to be designed.

The language must distinguish:

```text
visible
subscribable
publishable
```

## Interfaces

An interface may require an observable event member:

```sec
interface PressSource {
    event ButtonPressed[ButtonPressData]
}
```

This grants observation/subscription semantics, not publication rights.

## Copy and move

Event storage is not implicitly copyable.

Copying an event-owning type must not duplicate active subscriptions.

Moving an event-owning value is valid only when the selected storage backend and
all active subscriptions remain valid.

The compiler may require stable address or reject moves after subscription.

## Address stability

Inline event storage may require address stability.

The compiler may pin the owner, reject later moves, use address-independent slot
representation or require explicit stable storage.

The selected strategy must be visible to ownership and escape analysis.

## Allocation

The default event performs no hidden allocation.

An allocation-capable backend must be selected explicitly and follows allocation,
arena, failure and target-profile rules.

Reaching default capacity must not trigger allocation.

## Thread safety

The default event is not implicitly thread-safe.

Concurrent publish, subscribe or unsubscribe requires an explicitly thread-safe
policy or external synchronization.

A type must not pay thread-synchronization cost when the event is used only
locally.

## Tasks and threads

Events may be used from tasks and threads when policy permits.

Synchronous handlers run in the publishing execution context.

They do not automatically run on the execution entity that subscribed.

Cross-context delivery requires an explicit channel, dispatcher or adapter.

## Interrupts

Ordinary event publication is not ISR-safe.

An ISR must not invoke arbitrary subscribers, allocate, block or acquire ordinary
mutexes.

ISR code should instead use an ISR-safe channel, bounded notification, atomic
state or deferred work that later publishes the event.

## Events versus channels

```text
Event[T]
    One publication fans out borrowed data to zero or more handlers.
    Default dispatch is synchronous.

Channel[T]
    One committed message transfers ownership to one receiver.
    Delivery may be buffered and asynchronous.
```

## Events versus completion

Completion is one-shot terminal readiness.

An event may publish repeatedly.

Task, thread, process, DMA and I/O completion are therefore not ordinary events,
although adapters may expose them as events.

## Events versus interrupts

A hardware interrupt is an asynchronous control transfer to an ISR.

An event is a software publish/subscribe object.

An ISR may trigger deferred work that later publishes an event.

## Compiler-known types

The compiler recognizes:

```sec
Event[T]
Event[T, Capacity]
EventStorage[T, Capacity]
Subscription
EventSubscribeResult
```

It must understand owner-only publication and external subscription capabilities.

Current semantic analysis uses `EventSubscribeResult` for `Subscribe`.

Its implemented variants are:

```text
Subscribed(Subscription)
Full
```

## Semantic analysis

The compiler must track:

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

## Semantic IR

At minimum:

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

IR must record payload type, capacity, storage policy, owner type, access,
callable, captures, ordering, synchronization and source location.

## Diagnostics

Examples:

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

Diagnostic IDs must remain stable and distinguish parse, semantic, ownership,
concurrency and target-profile rules.

## Current implementation status

Implemented:

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

Not implemented yet:

- runtime subscriber storage;
- runtime dispatch;
- active subscription tracking;
- handler capture lifetime analysis specific to subscriptions;
- address-stability enforcement after subscription;
- thread-safe event policies;
- ISR-safe event policies;
- policy blocks after `impl event ... using ...`;
- Semantic IR lowering for event operations.

## Restrictions

An event must not:

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

## Future extensions

Possible later additions:

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

## Related rules

```text
struct.md
impl.md
properties.md
declarations/interfaces.md
functions.md
closures.txt
ownership.md
borrowing.txt
lifetime_analysis.md
allocation.md
layout.txt
static.txt
channels.md
concurrency.md
interrupts.txt
diagnostics.txt
```
