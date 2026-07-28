# Channels

## Purpose

Channels provide typed, unidirectional communication between concurrent
execution entities.

A channel transfers values from one or more send capabilities to one receive
capability. It is not a task, thread, process, implicit mailbox, or ordinary IPC
transport.

Ordinary in-process `Channel[T]` supports communication between tasks and
threads in any combination:

```text
task -> task
task -> thread
thread -> task
thread -> thread
```

Separate processes do not communicate through ordinary in-process `Channel[T]`
unless a future IPC adapter explicitly defines that mapping.

## Core model

Conceptually:

```text
Sender[T] -> channel storage -> Receiver[T]
```

Channel construction yields explicit TX and RX capabilities:

```sec
let channel := Channel[Message](32)

let tx := channel.tx
let rx := channel.rx
```

The compiler recognizes:

```sec
Channel[T]
Sender[T]
Receiver[T]
MessageTicket[T]
```

`Sender[T]` grants TX rights. `Receiver[T]` grants RX rights. Neither exposes
internal storage.

A basic channel is unidirectional. Bidirectional communication uses two channels
or a higher-level abstraction.

## Capability ownership

`Sender[T]` and `Receiver[T]` are move-only.

Moving a capability transfers its rights. Capabilities are never copied
implicitly.

Additional senders are created explicitly:

```sec
let secondTx := tx.Share()
```

Each shared sender remains move-only.

Version 0.1 has one move-only receiver capability. Multi-consumer queues require
a separate future abstraction.

## Sender count and identity

A channel has a configured or profile-bounded maximum number of live senders.

For embedded and statically analyzed targets, this maximum should be known at
compile time.

Conceptual configuration:

```sec
let channel := Channel[Message](
    ChannelOptions {
        capacity: 32
        maxSenders: 8
    }
)
```

Every sender has a `SenderID` unique within the channel lifetime.

`SenderID` identifies a logical send capability for one channel.

It is not a `TaskID`, `ThreadID`, operating-system thread ID, executor worker ID
or current execution-entity identity.

A sender may move between tasks or threads while retaining the same sender
identity.

A sender identity conceptually contains:

```text
sender slot
sender generation
```

Reusing a slot increments its generation. A stale identity must never identify a
new sender.

The physical width of `SenderID` is profile-specific. The compiler should select
the smallest safe representation from the sender limit, channel lifetime and
generation-wrap requirements.

## Capacity

Capacity zero creates a rendezvous channel:

```sec
let channel := Channel[Message](0)
```

There is no buffered slot. A send commits only when a receiver is ready, and
ownership transfers directly.

Capacity greater than zero creates a bounded buffered channel:

```sec
let channel := Channel[Message](32)
```

Accepted messages may wait in channel storage. When full, blocking sends wait
until capacity is available.

This provides backpressure: producers cannot enqueue accepted messages without
bound faster than consumers remove them.

Version 0.1 does not require unbounded channels.

## Allocation

Channel construction is allocation-capable when storage cannot be selected
statically.

Compile-time-known capacity may permit stack, static, arena or fixed ring-buffer
storage.

Runtime capacity follows the active allocation and arena rules.

A profile that forbids dynamic allocation must reject runtime-sized channels
unless explicit compatible storage is provided.

Ordinary send and receive operations must not hide dynamic allocation.

## Optional capabilities

`Channel[T]` is the external abstraction. The compiler may choose different
internal channel types from explicit compile-time capabilities.

The public channel type remains `Channel[T]`.

The compiler must not expose a separate `ThreadChannel[T]` merely because a
channel may cross a physical thread boundary.

Possible internal implementations include:

- task-local storage
- executor-shared storage
- thread-shared storage
- hybrid storage
- ISR-safe storage when requested and supported

A capability that may cross a physical thread boundary must use a representation
that is safe for cross-thread access.

The compiler may determine this through escape analysis, call-graph analysis or
whole-program analysis.

Public APIs may require a conservative thread-safe representation because the
capability can escape beyond the current task.

A task-local channel capability must not later escape to a physical thread.

Possible capabilities include:

```text
revocation
expiration
priority
ISR-safe send
statistics
```

Unused capabilities must add no required per-message storage or runtime work.

Conceptual configuration:

```sec
let channel := Channel[Message](
    ChannelOptions {
        capacity: 32
        maxSenders: 8
        capabilities: {
            revocation
            expiration
        }
    }
)
```

A simple FIFO channel must not pay for sender metadata, deadlines, priorities,
tickets or ISR state that it does not use.

## Basic send

A final message is sent with:

```sec
tx.Send(message)
```

Ownership transfers only when the send commits successfully.

On commit:

```text
sender -> channel
```

or, for rendezvous:

```text
sender -> receiver
```

After commit, the sender may not use a moved message.

A committed final send cannot be retracted.

## Send outcomes

If the receive side closes before commit, the unsent message must remain owned by
the caller.

Conceptually:

```sec
match tx.Send(message) {
    Sent => {
    }

    Closed(message) => {
        Recover(message)
    }
}
```

A blocking send is a cancellation point.

Inside a task, blocking send may suspend the task.

Inside a physical thread, blocking send may block or park the physical thread.

The ownership and commit semantics are the same in both cases.

If cancellation occurs before commit:

- the message is not delivered;
- no channel ownership transfer occurs;
- the pending message is destroyed through normal task cleanup.

Cancellation remains task control flow rather than an ordinary send-result
variant.

When a task or thread ends, sender and receiver capabilities owned by that
entity are destroyed normally.

The last live sender closes the send side.

Committed messages remain valid after the originating task or thread has ended.

A detached execution entity must not retain a channel capability backed by
storage that can be destroyed before that detached entity stops using it.

## Message lifetime

Channels with expiration capability support:

```sec
tx.Send(message, 500ms)
```

The duration is the maximum lifetime of the committed message.

It is not the maximum wait for channel capacity.

The lifetime starts when send commits. Internally it becomes an absolute
monotonic deadline.

The send family is:

```sec
tx.Send(message)
tx.Send(message, lifetime)
```

There is no `SendTimed`.

## Revocable send

A channel with revocation capability supports:

```sec
let ticket := tx.SendRevocable(message)
```

and:

```sec
let ticket := tx.SendRevocable(message, 500ms)
```

There is no `SendTimedRevocable`.

`MessageTicket[T]` is move-only and represents the right to attempt revocation
of exactly one accepted message.

A ticket conceptually identifies:

```text
channel
message slot
message generation
originating SenderID
```

A stale ticket must never refer to a newer message that reused the same slot.

## Revoke

Revocation consumes the ticket:

```sec
match tx.Revoke(ticket) {
    Revoked(message) => {
        Recover(message)
    }

    Unavailable(disposition) => {
        Handle(disposition)
    }
}
```

Exactly one terminal operation may win:

- receive;
- revoke;
- expiration;
- receiver discard;
- receiver close.

A successful revoke returns ownership of `T`.

Possible terminal dispositions include:

```sec
enum MessageDisposition {
    Received
    Expired
    Discarded
}
```

The final API may simplify these names, but duplicate ownership is never valid.

## Sender disappearance

Closing or destroying a sender does not remove messages it already committed.

Such a message may be orphaned.

`Orphaned` means only that the originating sender no longer exists. It does not
mean cancelled, expired, discarded or undeliverable.

An orphaned pending message remains deliverable.

Sender lifecycle and message delivery state are separate concepts.

## Send-side closing

```sec
tx.Close()
```

consumes only that sender.

Destroying a sender has the same effect.

The send side closes when the last `Sender[T]` is closed or destroyed.

Closing the send side does not destroy the channel. Already accepted messages
remain available to the receiver.

## Receive

The blocking receive operation returns `Option[T]`:

```sec
let message := rx.Receive()
```

Meaning:

```text
Some(T)
    A deliverable message was received.

None
    The send side is closed and no deliverable queued messages remain.
```

A temporarily empty but open channel waits.

`None` never means temporary emptiness, timeout or cancellation.

`Receive()` is a cancellation point while waiting.

## TryReceive

A non-blocking receive must distinguish:

```text
Received(T)
Empty
Closed
```

Therefore `TryReceive()` requires a dedicated outcome type rather than
`Option[T]`.

## Message state

A message slot conceptually has a delivery state such as:

```sec
enum MessageState {
    Pending
    Cancelled
    Received
    Expired
    Discarded
}
```

`Orphaned` is not part of this enum because sender existence is a separate
dimension.

## Tombstones

Non-deliverable slots may remain as tombstones until the receiver head passes
them.

Example:

```text
head -> Cancelled, Cancelled, Expired, Pending <- tail
```

One `Receive()` may internally reclaim the first three slots and return only the
pending message.

Tombstones are never exposed as empty messages.

A tombstone remains physically reserved until the head passes it. Version 0.1
does not require reuse of tombstones in the middle of the ring.

This avoids moving later entries and provides stable slot identity, bounded
storage and deterministic embedded behavior.

## Expiration

An expiring channel stores an absolute deadline for messages that have a
lifetime.

An expired message:

- is never delivered;
- is destroyed deterministically;
- becomes non-deliverable;
- is reclaimed when the receiver advances past it.

Expiration may be lazy. One timer per message is not required.

## FIFO ordering

A normal `Channel[T]` is FIFO.

Messages from one sender preserve successful commit order.

When several senders race, actual commit order determines receive order.

No fairness guarantee exists beyond commit ordering.

## Priority capability

Priority is an optional channel capability or specialized internal form.

Core rules:

- higher-priority deliverable messages are selected first;
- lower-priority messages never overwrite higher-priority messages;
- the highest priority is never overwritten by a lower priority;
- when replacement is enabled, the lowest-priority eligible message is replaced
  first;
- among equal lowest priorities, the oldest eligible message is replaced first;
- cancelled, expired and discarded entries receive no priority protection.

Possible explicit overflow policies include:

```text
Block
Reject
ReplaceLowest
```

A normal FIFO channel never overwrites accepted messages.

## Receiver discard

The receiver may discard all currently queued messages without closing RX:

```sec
rx.Discard()
```

`Discard()`:

- destroys all messages committed before its queue boundary;
- leaves RX open;
- permits future sends;
- does not wait for future messages;
- does not affect messages already received;
- does not consume the receiver.

Messages committed after the discard boundary remain available.

## Receiver close

```sec
rx.Close()
```

consumes the receiver and:

- closes RX permanently;
- destroys queued messages;
- wakes waiting sends without commit;
- makes future sends fail;
- prevents future receives.

Destroying `Receiver[T]` has the same closing effect.

Normal draining and processing is written explicitly:

```sec
while let message := rx.Receive() {
    Process(message)
}
```

`Discard()` destroys queued messages instead of returning them.

## No sender drain

A basic sender does not remove all messages previously sent by that sender.

Committed final messages belong to the channel.

Selective removal uses `SendRevocable()` and `MessageTicket[T]`.

## Waiting timeout

Version 0.1 does not require `SendTimed`, `ReceiveTimed` or a general `Timeout`
type.

Maximum waiting time is expressed through `select`:

```sec
select {
    tx.Send(message) => {
        Sent()
    }

    after 20ms => {
        NotSent(message)
    }
}
```

If `after` is selected, the send does not commit and ownership remains with the
current task.

Receive timeout is analogous:

```sec
select {
    message := rx.Receive() => {
        Process(message)
    }

    after 20ms => {
        HandleTimeout()
    }
}
```

## Select integration

Channel operations may participate in `select`.

Select uses two phases:

```text
readiness
commit
```

Only the selected branch commits.

For non-selected branches:

- receive consumes no message;
- send transfers no ownership;
- revocable send creates no ticket;
- queue state does not change.

Detailed branch ordering and syntax belong in `select.txt`.

## ISR-safe communication

Ordinary blocking `Send()` is not interrupt-safe.

An ISR-safe channel capability may permit bounded non-blocking send only when:

- storage is statically valid;
- no heap allocation occurs;
- no blocking fallback occurs;
- required atomics are target-supported;
- execution time is bounded;
- destruction is interrupt-safe.

ISR-safe support is explicit and profile-validated.

Critical embedded or telecom communication may use separate critical channels,
priority capability, reserved critical capacity or atomic flags for simple
signals.

## Memory synchronization

A successful send publishes the complete message.

A successful receive acquires it.

Writes that initialize the message before commit happen-before receiver access
after successful receive.

Rendezvous handoff establishes the same synchronization.

Detailed ordering is defined in `concurrency_memory_model.txt`.

## Detached tasks

A detached task may own channel capabilities only when channel storage outlives
the task and shutdown, cancellation and ownership remain valid.

A detached task may not retain a capability backed by shorter-lived
scope-owned storage.

## Destruction

A channel may be destroyed only when:

- TX is closed;
- RX is closed or consumed;
- no send or receive waits;
- no ticket can reference channel state;
- no task retains a capability;
- all queued messages are received or destroyed.

Static channels may live until normal shutdown.

Forced termination does not guarantee cleanup.

## Message-type rules

A channel may carry any type whose ownership, alignment, lifetime and
destruction are valid for transfer and selected storage.

Move-only values transfer normally.

A channel never silently clones move-only values.

Sending references does not extend pointee lifetime. Every reference in a
message must outlive queueing, receipt and use.

Long-lived or detached channels should normally transfer owned values.

## Target profiles

A channel profile may define:

- storage strategy;
- maximum capacity;
- maximum sender count;
- scheduler integration;
- lock-free support;
- ISR-safe support;
- clock support for expiration;
- ticket width;
- SenderID representation;
- priority implementation;
- allocation availability.

Unsupported feature combinations must be rejected.

## Semantic analysis

The compiler must track at least:

- channel identity;
- message type;
- capacity;
- selected capabilities;
- storage strategy;
- sender count and limits;
- sender identity and generation;
- receiver ownership;
- capability moves;
- send commit;
- message ownership;
- lifetime and expiration;
- ticket ownership;
- revocation state;
- close state;
- discard boundaries;
- cancellation points;
- select readiness and commit;
- task escape;
- target support.

## Current implementation status

Implemented:

- `Channel[T]`, `Sender[T]`, `Receiver[T]` and `MessageTicket[T]` are
  compiler-known intrinsic generic types.
- Channel outcome/supporting types are compiler-known:
  `ChannelSendResult[T]`, `ChannelTryReceiveResult[T]`,
  `ChannelRevokeResult[T]`, `MessageDisposition`, `ChannelOptions` and
  `SenderID`.
- `Sender[T]`, `Receiver[T]`, `MessageTicket[T]` and `Channel[T]` are
  classified as move-only.
- `Channel[T](capacity)` is recognized by semantic analysis and requires
  exactly one message type plus one integer capacity argument.
- `channel.tx` resolves to `Sender[T]`.
- `channel.rx` resolves to `Receiver[T]`.
- `Sender[T].Share()` resolves to `Sender[T]`.
- `Sender[T].Send(message)` and `Sender[T].TrySend(message)` require `message`
  to be `T`, consume move-only message values, and return
  `ChannelSendResult[T]`.
- `Sender[T].SendRevocable(message)` requires `message` to be `T`, consumes
  move-only message values, and returns `MessageTicket[T]`.
- `Sender[T].Revoke(ticket)` requires `MessageTicket[T]`, consumes move-only
  tickets, and returns `ChannelRevokeResult[T]`.
- `Sender[T].Close()` consumes the sender capability and returns `void`.
- `Receiver[T].Receive()` returns `Option[T]`.
- `Receiver[T].TryReceive()` returns `ChannelTryReceiveResult[T]`.
- `Receiver[T].Discard()` returns `void`.
- `Receiver[T].Close()` consumes the receiver capability and returns `void`.
- Channel receive, try-receive, send, try-send and revocable-send operations
  are recognized as selectable operands by initial `select` semantic analysis.

Not implemented yet:

- Runtime channel storage, queues, blocking behavior and synchronization.
- Channel capacity/profile validation beyond integer type checking.
- Explicit sender-count and sender-identity tracking.
- Capability duplication prevention for repeated `channel.tx` / `channel.rx`
  member extraction from the same channel value.
- Channel close-state analysis across control flow.
- Expiration, priority, revocation capability flags and tombstone behavior.
- Semantic IR channel operations and backend lowering.
- Runtime select integration beyond semantic recognition of channel operands.

## Semantic IR

Semantic IR must represent channel operations explicitly.

At minimum:

```text
ChannelCreate
ChannelPublish
ChannelDestroy
SenderCreate
SenderShare
SenderMove
SenderClose
ReceiverMove
ReceiverReceive
ReceiverTryReceive
ReceiverDiscard
ReceiverClose
ChannelSend
ChannelSendRevocable
ChannelTrySend
ChannelRevoke
ChannelExpire
ChannelSkipTombstone
ChannelCommit
ChannelSelectReady
ChannelSelectCommit
```

IR must record message type, channel identity, direction, sender identity,
capacity, storage profile, features, ownership, ticket identity, deadline,
priority, state transition, cancellation, synchronization and source location.

## Diagnostics

Examples:

```text
Channel requires exactly one message type
```

```text
use of moved sender tx
```

```text
use of moved receiver rx
```

```text
channel supports at most 8 concurrent senders
```

```text
channel does not support revocable messages
```

```text
channel does not support message expiration
```

```text
message lifetime requires expiration capability
```

```text
cannot send message because receive side is closed
```

```text
message ticket has already been consumed
```

```text
message is no longer revocable: already received
```

```text
runtime channel capacity requires dynamic allocation
```

```text
target profile does not support ISR-safe send for Message
```

## Restrictions

A channel must not:

- become bidirectional implicitly;
- expose queue storage;
- copy move-only capabilities or tickets;
- clone move-only messages;
- transfer ownership before commit;
- retract final committed messages;
- return cancelled or expired messages;
- use `None` for temporary emptiness;
- add unused feature metadata;
- overwrite messages in a normal FIFO channel;
- hide dynamic allocation;
- bypass lifetime or ownership rules;
- replace IPC semantics.

## Future extensions

Possible future additions include:

```text
multi-consumer channels
unbounded hosted channels
broadcast channels
latest-value channels
priority-channel API refinements
revocation groups
explicit backing storage
absolute message deadlines
channel observers
cross-process adapters
```

## Related rules

```text
tasks.txt
spawn.txt
await.txt
concurrency.txt
select.txt
mutex.txt
atomics.txt
static.txt
concurrency_memory_model.txt
allocation.txt
processes.txt
ipc.txt
```
