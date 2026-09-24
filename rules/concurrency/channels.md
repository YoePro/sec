# Channels

- **Status:** Normative
- **Created:** 2026-09-15
- **Last updated:** 2026-09-24
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/channels.md`
- **Replaces:** Earlier unversioned/legacy revision at the same canonical path
- **Repository baseline reviewed:** `main-reviewed-2026-09-15`
- **Implementation governance:** `governance/concurrency_channels.yaml`
- **Related rulebooks:** `rules/concurrency/concurrency.md`, `rules/concurrency/select.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/blocking.md`, `rules/concurrency/ipc.md`, `rules/memory/allocation.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/transferability.md`, `rules/memory/destruction.md`, `rules/compiler/semantic_ir.md`, `rules/platform/target_profiles.md`, `rules/platform/platform_model.md`, `rules/tooling/lsp.md`

---

## § 1. Purpose and authority

**Governance tags:** `concurrency.channels-v2`

§ 1(1) `Channel[T]` provides typed, unidirectional, in-process ownership transfer between concurrent Sec execution entities.

§ 1(2) Ordinary channels support:

```text
task -> task
task -> thread
thread -> task
thread -> thread
```

§ 1(3) This rulebook owns the ordinary in-process channel source API, endpoint capability model, send/receive outcomes, channel lifecycle, optional channel capabilities, revocation, expiration, statistics, select integration, and channel-specific compiler/runtime obligations.

§ 1(4) This rulebook does not own generic `select` syntax, task/thread lifecycle, general cancellation, general allocation semantics, or IPC semantics.

§ 1(5) Mutable implementation progress belongs only in `governance/concurrency_channels.yaml`.

---

## § 2. Strict in-process boundary

**Governance tags:** `concurrency.channels-v2`, `concurrency.ipc-v1`

§ 2(1) `Channel[T]`, `Sender[T]`, `Receiver[T]`, and `MessageTicket[T]` are strictly in-process capabilities.

§ 2(2) They are never automatically lowered to process IPC.

§ 2(3) They are not `ProcessTransferable`.

§ 2(4) Typed process messaging uses the distinct canonical IPC types:

```sec
IPCSender[T]
IPCReceiver[T]
```

defined by `rules/concurrency/ipc.md`.

§ 2(5) A future explicit bridge, if standardized, must remain an explicit adapter and must not change the ordinary channel type identity.

---

## § 3. Public naming

**Governance tags:** `concurrency.channels-v2`, `tooling.channels-v2`

§ 3(1) Public channel members use canonical Sec CamelCase.

§ 3(2) Canonical examples include:

```text
Tx
Rx
Share
Send
TrySend
SendRevocable
Revoke
Receive
TryReceive
Discard
Close
Statistics
Capacity
MaxSenders
Capabilities
```

§ 3(3) Legacy lowercase public spellings such as `channel.tx` and `channel.rx` are non-canonical.

§ 3(4) Tooling, diagnostics, examples, and generated documentation must use the canonical spellings.

---

## § 4. Compiler-known/source-visible channel types

**Governance tags:** `concurrency.channels-v2`, `frontend.channels-v2`, `tooling.channels-v2`

§ 4(1) The following are compiler-known/source-visible channel types in Sec 0.1:

```text
Channel[T]
Sender[T]
Receiver[T]
MessageTicket[T]
ChannelCapability
ChannelOptions
ChannelStatistics
SenderID
ChannelSendResult[T]
ChannelTrySendResult[T]
ChannelTryReceiveResult[T]
ChannelRevocableSendResult[T]
ChannelRevokeResult[T]
MessageDisposition
```

§ 4(2) Compiler-known status does not permit an implementation to expose only hidden compiler names while omitting the exact source-visible type surface.

§ 4(3) Compiler Sema, LSP hover/completion/navigation, documentation, Semantic IR generation, and lowering must consume the same canonical identities.

§ 4(4) Runtime storage fields that implement queueing, waiting, synchronization, timers, or generations remain hidden unless this rulebook explicitly makes them public.

---

## § 5. Exact `ChannelCapability`

**Governance tags:** `concurrency.channels-v2`, `tooling.channels-v2`

§ 5(1) The exact Sec 0.1 declaration is:

```sec
enum ChannelCapability {
    Revocation
    // Enables SendRevocable(...) and MessageTicket[T] revocation.

    Expiration
    // Enables message-lifetime Send/SendRevocable overloads.

    ISRSafeSend
    // Allows target-validated bounded non-blocking TrySend from ISR context.

    Statistics
    // Enables Statistics() snapshots on live Sender/Receiver endpoints.
}
```

§ 5(2) These four variants are the complete Sec 0.1 public capability set.

§ 5(3) `Priority` is not a Sec 0.1 `ChannelCapability`.

§ 5(4) An implementation must not expose a capability value whose public source behavior is not fully specified.

---

## § 6. Exact `ChannelOptions`

**Governance tags:** `concurrency.channels-v2`, `frontend.channels-v2`

§ 6(1) The exact public configuration type is:

```sec
type ChannelOptions struct {
    Capacity: uint
    // 0 creates a rendezvous channel.
    // >0 creates a bounded buffered channel.

    MaxSenders: uint
    // Maximum number of simultaneously live Sender[T] capabilities.
    // The initial Tx capability counts as one.

    Capabilities: set[ChannelCapability]
    // Closed compile-time capability set selected for this channel.
}
```

§ 6(2) `MaxSenders` must be at least `1`.

§ 6(3) `MaxSenders` and `Capabilities` must be compile-time-known at channel construction so endpoint identity layout and optional runtime metadata are fixed before lowering.

§ 6(4) `Capacity` may be compile-time-known or runtime-known subject to allocation/profile rules.

§ 6(5) Unsupported capability combinations are compile-time errors.

§ 6(6) Invalid compile-time-known configuration is a compile-time diagnostic, not a runtime channel error.

---

## § 7. Exact `ChannelStatistics`

**Governance tags:** `concurrency.channels-v2`, `tooling.channels-v2`

§ 7(1) The exact public statistics snapshot type is:

```sec
type ChannelStatistics struct {
    Sent: uint64
    // Number of sends that committed ownership to the channel/receiver.

    Received: uint64
    // Number of committed messages whose ownership reached Receiver[T].

    Revoked: uint64
    // Number of successful MessageTicket[T].Revoke() operations.

    Expired: uint64
    // Number of committed messages destroyed because expiration won.

    Discarded: uint64
    // Number of committed messages destroyed by Receiver.Discard()
    // or Receiver.Close().
}
```

§ 7(2) Each counter saturates at `uint64` maximum rather than wrapping to zero.

§ 7(3) A statistics snapshot is observational and does not alter channel readiness, ordering, ownership, or lifecycle.

§ 7(4) Queue depth, waiter count, capacity, and implementation timing counters are intentionally not part of this public snapshot.

---

## § 8. Exact `SenderID`

**Governance tags:** `concurrency.channels-v2`

§ 8(1) The source-visible declaration shape is:

```sec
type SenderID
// Copyable opaque logical identity for one Sender[T] capability incarnation.
//
// Not a TaskID.
// Not a ThreadID.
// Not an operating-system thread identifier.
// Not an executor-worker identifier.
// Not a process identifier.
//
// Physical representation is CompilationPlan/runtime-defined.
```

§ 8(2) A `SenderID` identity conceptually contains enough information to distinguish sender slot and sender generation.

§ 8(3) Reusing a sender slot must change generation identity so a stale identity never names a newer sender.

§ 8(4) Sec 0.1 defines no arithmetic over `SenderID`.

§ 8(5) Sender identity is primarily a compiler/runtime semantic identity in this revision; `Sender[T]` does not require a public mutable identity field.

---

## § 9. Exact blocking-send result

**Governance tags:** `concurrency.channels-v2`, `frontend.channels-v2`

§ 9(1) The exact declaration is:

```sec
type ChannelSendResult[T] union {
    Sent
    // The send committed.
    // Ownership transferred to the channel or rendezvous receiver.

    Closed(T)
    // The receive side closed before commit.
    // Ownership of the unsent T is returned in the payload.
}
```

§ 9(2) `ChannelSendResult[T]` is used by blocking `Send(...)`.

§ 9(3) It contains no `WouldBlock` variant because blocking send waits rather than reporting temporary backpressure.

§ 9(4) It is not an error type.

---

## § 10. Exact non-blocking-send result

**Governance tags:** `concurrency.channels-v2`, `frontend.channels-v2`

§ 10(1) The exact declaration is:

```sec
type ChannelTrySendResult[T] union {
    Sent
    // The send committed immediately.

    WouldBlock(T)
    // The channel is open but the send cannot commit immediately.
    // Ownership of the unsent T is returned.

    Closed(T)
    // The receive side is closed.
    // Ownership of the unsent T is returned.
}
```

§ 10(2) `WouldBlock(T)` covers both:

- a full bounded buffered channel;
- a rendezvous channel without an immediately matching receiver.

§ 10(3) `Option[ChannelSendResult[T]]` is not used because `None` would hide ownership return of `T`.

§ 10(4) Distinct result types preserve ordinary exhaustive `match` semantics without producer-refined exhaustiveness.

---

## § 11. Exact non-blocking-receive result

**Governance tags:** `concurrency.channels-v2`, `frontend.channels-v2`

§ 11(1) The exact declaration is:

```sec
type ChannelTryReceiveResult[T] union {
    Received(T)
    // One deliverable message was received immediately.

    Empty
    // The channel is open but no deliverable message is available immediately.

    Closed
    // The send side is closed and no deliverable queued message remains.
}
```

§ 11(2) `TryReceive()` does not use `Option[T]` because temporary emptiness and permanent closure are distinct.

§ 11(3) The `Closed` variant has no payload.

---

## § 12. Exact revocable-send result

**Governance tags:** `concurrency.channels-v2`, `frontend.channels-v2`

§ 12(1) The exact declaration is:

```sec
type ChannelRevocableSendResult[T] union {
    Accepted(MessageTicket[T])
    // The revocable send committed.
    // The returned ticket owns the right to attempt later revocation.

    Closed(T)
    // RX closed before commit.
    // No ticket exists and ownership of T is returned.
}
```

§ 12(2) `Accepted` means accepted by the channel, not necessarily already received by user code.

§ 12(3) Pre-commit closure never produces a ticket.

---

## § 13. Exact message disposition

**Governance tags:** `concurrency.channels-v2`

§ 13(1) The exact declaration is:

```sec
enum MessageDisposition {
    Received
    // Ownership already transferred to Receiver[T].

    Expired
    // Message lifetime expired and the message was destroyed.

    Discarded
    // Receiver-side discard/close destroyed the message without delivery.
}
```

§ 13(2) Receiver close maps pending-message disposition to `Discarded`.

§ 13(3) `MessageDisposition` describes what became of the committed message, not the detailed cause chain that led to receiver shutdown.

§ 13(4) `Orphaned` is not a `MessageDisposition`; sender disappearance is a separate dimension.

---

## § 14. Exact revoke result

**Governance tags:** `concurrency.channels-v2`, `frontend.channels-v2`

§ 14(1) The exact declaration is:

```sec
type ChannelRevokeResult[T] union {
    Revoked(T)
    // Revocation committed first.
    // Ownership of T returns to the ticket owner.

    Unavailable(MessageDisposition)
    // Another terminal message disposition committed before revoke.
}
```

§ 14(2) `ChannelRevokeResult[T]` is not an error type.

§ 14(3) `Unavailable(Received)`, `Unavailable(Expired)`, and `Unavailable(Discarded)` are exhaustive Sec 0.1 unavailable outcomes.

---

## § 15. Exact `Channel[T]` source shape

**Governance tags:** `concurrency.channels-v2`, `frontend.channels-v2`, `tooling.channels-v2`

§ 15(1) The canonical source-visible declaration shape is:

```sec
@noCopy
type Channel[T] struct {
    Tx: Sender[T]
    // Initial owning send capability.

    Rx: Receiver[T]
    // The single owning receive capability.

    try init(capacity: uint) AllocationError
    // Constructs a normal FIFO channel with no optional capabilities.
    // Uses the selected profile's finite default sender limit.

    try init(options: ChannelOptions) AllocationError
    // Constructs a channel with the exact requested options.
}
```

§ 15(2) `Channel[T]` has exactly one generic message type parameter.

§ 15(3) The public fields are named exactly `Tx` and `Rx`.

§ 15(4) `Channel[T]` is `@noCopy` because its endpoint capabilities are unique ownership capabilities.

§ 15(5) The two endpoint fields support ordinary Sec partial-move semantics.

§ 15(6) The hidden shared channel-state identity is not a third public field.

---

## § 16. Exact `Sender[T]` source shape

**Governance tags:** `concurrency.channels-v2`, `frontend.channels-v2`, `tooling.channels-v2`

§ 16(1) The canonical source-visible declaration shape is:

```sec
@noCopy
type Sender[T]
// Owns one send capability for one channel identity.
// Physical endpoint/runtime fields are hidden.

impl Sender[T] {
    fn Share() Option[Sender[T]]

    fn Send(message: T) ChannelSendResult[T]
    fn Send(message: T, lifetime: duration) ChannelSendResult[T]

    fn TrySend(message: T) ChannelTrySendResult[T]

    fn SendRevocable(message: T) ChannelRevocableSendResult[T]
    fn SendRevocable(
        message: T,
        lifetime: duration
    ) ChannelRevocableSendResult[T]

    fn Statistics() ChannelStatistics

    fn Close() void
}
```

§ 16(2) `Sender[T]` is move-only.

§ 16(3) `Share()` does not consume the original sender.

§ 16(4) `Close()` consumes the local sender capability.

§ 16(5) Message parameters are by-value ownership-transfer parameters; existing named move-only values use the canonical call-site `<-` marker.

§ 16(6) Lifetime overloads require `ChannelCapability.Expiration`.

§ 16(7) `SendRevocable` overloads require `ChannelCapability.Revocation`.

§ 16(8) `Statistics()` requires `ChannelCapability.Statistics`.

---

## § 17. Exact `Receiver[T]` source shape

**Governance tags:** `concurrency.channels-v2`, `frontend.channels-v2`, `tooling.channels-v2`

§ 17(1) The canonical source-visible declaration shape is:

```sec
@noCopy
type Receiver[T]
// Owns the unique receive capability for one ordinary Channel[T].
// Physical endpoint/runtime fields are hidden.

impl Receiver[T] {
    fn Receive() Option[T]
    fn TryReceive() ChannelTryReceiveResult[T]

    fn Discard() void
    fn Statistics() ChannelStatistics

    fn Close() void
}
```

§ 17(2) `Receiver[T]` is move-only.

§ 17(3) Sec 0.1 has exactly one live receive capability per ordinary channel.

§ 17(4) `Discard()` does not consume the receiver.

§ 17(5) `Close()` consumes the receiver.

§ 17(6) `Statistics()` requires `ChannelCapability.Statistics`.

---

## § 18. Exact `MessageTicket[T]` source shape

**Governance tags:** `concurrency.channels-v2`, `frontend.channels-v2`, `tooling.channels-v2`

§ 18(1) The canonical source-visible declaration shape is:

```sec
@noCopy
type MessageTicket[T]
// Owns the right to attempt revocation of exactly one committed
// revocable message.
//
// Hidden state includes:
// - channel identity;
// - message slot identity;
// - message generation;
// - originating SenderID.

impl MessageTicket[T] {
    fn Revoke() ChannelRevokeResult[T]
}
```

§ 18(2) `MessageTicket[T]` is move-only.

§ 18(3) `Revoke()` consumes the ticket.

§ 18(4) Revocation does not require the original `Sender[T]` to still exist.

§ 18(5) A ticket cannot be used to target another channel or another message generation.

---

## § 19. Construction fallibility

**Governance tags:** `concurrency.channels-v2`, `allocation.general-v2`

§ 19(1) Channel construction is fallible only for allocation when dynamic storage is required and success is not proven.

§ 19(2) The canonical failure identity is `AllocationError` from the allocation/core model.

§ 19(3) This rulebook introduces no `ChannelCreateError`.

§ 19(4) Unsupported target/profile features and invalid compile-time-known options are compile-time diagnostics, not runtime constructor error variants.

§ 19(5) If the compiler proves static/fixed storage sufficient, it may eliminate the runtime allocation/failure path while preserving the source-level fallible constructor contract.

§ 19(6) The exact declaration and variants of `AllocationError` remain owned by the canonical allocation/core source; channels must reuse that identity rather than defining a duplicate error enum.

---

## § 20. Construction syntax

**Governance tags:** `concurrency.channels-v2`, `frontend.channels-v2`

§ 20(1) Simple construction is:

```sec
let channel := try Channel[Message](32)
```

§ 20(2) Option construction is:

```sec
let channel := try Channel[Message](
    ChannelOptions {
        Capacity: 32
        MaxSenders: 8
        Capabilities: {
            ChannelCapability.Revocation
            ChannelCapability.Expiration
        }
    }
)
```

§ 20(3) The simple `capacity` constructor means:

- normal FIFO semantics;
- no optional `ChannelCapability`;
- selected profile's finite default sender limit.

§ 20(4) Construction creates exactly one initial `Sender[T]` and exactly one `Receiver[T]`.

---

## § 21. Endpoint extraction and partial moves

**Governance tags:** `concurrency.channels-v2`, `analysis.transferability`

§ 21(1) Endpoint extraction uses ordinary field move/partial-move semantics.

Example:

```sec
let channel := try Channel[Message](32)

let tx :<- channel.Tx
let rx :<- channel.Rx
```

§ 21(2) After moving `channel.Tx`, that field is unavailable.

§ 21(3) After moving `channel.Rx`, that field is unavailable.

§ 21(4) Re-reading or re-moving an unavailable endpoint field is a compile-time ownership error.

§ 21(5) Destroying a `Channel[T]` value with one or both endpoint fields still owned destroys those still-owned endpoint capabilities normally.

§ 21(6) A fully partially-moved shell has no independent public channel capability beyond its remaining fields.

---

## § 22. Capacity

**Governance tags:** `concurrency.channels-v2`

§ 22(1) `Capacity == 0` creates a rendezvous channel.

§ 22(2) Rendezvous send commits only when a receiver can commit the matching receive.

§ 22(3) `Capacity > 0` creates a bounded buffered channel.

§ 22(4) A buffered channel may hold at most `Capacity` committed deliverable/tombstoned message slots according to the storage rules.

§ 22(5) Sec 0.1 does not provide unbounded ordinary channels.

§ 22(6) A normal FIFO channel never overwrites an accepted message to make room for another.

---

## § 23. Allocation and hidden allocation

**Governance tags:** `concurrency.channels-v2`, `allocation.general-v2`

§ 23(1) Channel construction is allocation-capable when the selected storage cannot be fully provided statically/fixed.

§ 23(2) Compile-time-known bounded configurations may permit stack, static, arena, fixed-ring, runtime-reserved, or equivalent proven storage.

§ 23(3) Runtime capacity may require dynamic allocation from the active canonical allocation context.

§ 23(4) `Send`, `TrySend`, `SendRevocable`, `Receive`, `TryReceive`, `Share`, `Revoke`, `Discard`, `Close`, and `Statistics` must not hide dynamic allocation.

§ 23(5) Optional-capability metadata required at runtime must be included in construction planning rather than lazily allocated by ordinary send/receive calls.

§ 23(6) A no-allocation profile may accept a channel only when required storage is proven available without forbidden dynamic allocation.

---

## § 24. Sender count and `Share()`

**Governance tags:** `concurrency.channels-v2`

§ 24(1) The initial `Tx` capability counts toward `MaxSenders`.

§ 24(2) `Share()` returns:

```sec
Option[Sender[T]]
```

§ 24(3) `Some(sender)` means a new sender capability and new sender identity were created.

§ 24(4) `None` means the configured `MaxSenders` is already reached.

§ 24(5) `None` does not consume or invalidate the original sender.

§ 24(6) Receiver closure does not redefine `None`; if a sender slot is available, sharing may still produce a sender whose future sends observe RX closure.

§ 24(7) Destroying or closing one sender releases that live-sender slot for later reuse subject to generation safety.

---

## § 25. Sender identity

**Governance tags:** `concurrency.channels-v2`

§ 25(1) Every live sender has a logical `SenderID` unique for its channel lifetime incarnation.

§ 25(2) Moving a sender preserves its `SenderID`.

§ 25(3) Moving a sender between a task and a physical thread does not change sender identity.

§ 25(4) Closing/destroying a sender retires that sender identity.

§ 25(5) Reusing an implementation sender slot creates a different generation identity.

§ 25(6) Message ticket provenance may retain originating sender identity even after the sender itself disappears.

---

## § 26. Blocking `Send`

**Governance tags:** `concurrency.channels-v2`, `concurrency.cancellation-v2`

§ 26(1) The basic operation is:

```sec
fn Send(message: T) ChannelSendResult[T]
```

§ 26(2) On `Sent`, ownership commits exactly once to the channel or rendezvous receiver.

§ 26(3) On `Closed(message)`, no send commit occurred and ownership is returned in the result payload.

§ 26(4) When a bounded channel is full and RX remains open, `Send` waits.

§ 26(5) On a rendezvous channel, `Send` waits until a receiver can commit or RX closes.

§ 26(6) Waiting `Send` is a current-execution cancellation point.

§ 26(7) Current-task/thread cancellation before send commit does not return a channel result; cancellation proceeds through normal terminal cancellation cleanup.

§ 26(8) A committed send cannot be retracted unless it was explicitly created as revocable.

---

## § 27. Non-blocking `TrySend`

**Governance tags:** `concurrency.channels-v2`

§ 27(1) The operation is:

```sec
fn TrySend(message: T) ChannelTrySendResult[T]
```

§ 27(2) `TrySend` never waits.

§ 27(3) `Sent` means immediate commit.

§ 27(4) `WouldBlock(message)` means RX is open but immediate commit is impossible.

§ 27(5) `Closed(message)` means RX is closed.

§ 27(6) Both non-commit outcomes return ownership in their payload.

§ 27(7) `TrySend` is not a cancellation point merely by being a channel operation.

---

## § 28. Message lifetime send

**Governance tags:** `concurrency.channels-v2`, `frontend.temporal-duration`

§ 28(1) Expiring send uses:

```sec
fn Send(message: T, lifetime: duration) ChannelSendResult[T]
```

§ 28(2) This overload exists semantically only for a channel constructed with `ChannelCapability.Expiration`.

§ 28(3) `lifetime` is the maximum post-commit message lifetime.

§ 28(4) It is not a timeout for waiting to obtain buffer capacity/rendezvous readiness.

§ 28(5) The lifetime starts at successful send commit.

§ 28(6) The runtime converts the relative duration to an absolute monotonic expiration deadline.

§ 28(7) Compatible time-unit expressions may materialize as `duration` through the canonical temporal/unit conversion rules.

§ 28(8) Sec 0.1 defines no `SendTimed` spelling.

---

## § 29. Revocable send

**Governance tags:** `concurrency.channels-v2`, `concurrency.cancellation-v2`

§ 29(1) The canonical overloads are:

```sec
fn SendRevocable(message: T) ChannelRevocableSendResult[T]

fn SendRevocable(
    message: T,
    lifetime: duration
) ChannelRevocableSendResult[T]
```

§ 29(2) Both overloads require `ChannelCapability.Revocation`.

§ 29(3) The lifetime overload additionally requires `ChannelCapability.Expiration`.

§ 29(4) On `Accepted(ticket)`, ownership committed and the ticket owns revocation authority.

§ 29(5) On `Closed(message)`, ownership did not commit and no ticket exists.

§ 29(6) A waiting revocable send is a cancellation point before commit.

§ 29(7) Cancellation before commit creates no ticket.

§ 29(8) Sec 0.1 defines no `SendTimedRevocable` spelling.

---

## § 30. Ticket-owned revocation

**Governance tags:** `concurrency.channels-v2`

§ 30(1) Revocation is invoked on the ticket:

```sec
match ticket.Revoke() {
    Revoked(message) => {
        Recover(<-message)
    }

    Unavailable(disposition) => {
        Handle(disposition)
    }
}
```

§ 30(2) `Revoke()` consumes the ticket.

§ 30(3) The original sender is not required.

§ 30(4) A successful revoke returns ownership of the message.

§ 30(5) A failed revoke returns the already-committed terminal disposition.

§ 30(6) Exactly one of receive, revoke, expiration, discard, or receiver-close destruction may commit as the terminal disposition of the message.

---

## § 31. Ticket destruction

**Governance tags:** `concurrency.channels-v2`

§ 31(1) Destroying a live `MessageTicket[T]` relinquishes revocation authority only.

§ 31(2) Ticket destruction does not cancel, revoke, discard, expire, or otherwise remove the committed message.

§ 31(3) The message remains deliverable unless another terminal message transition occurs.

§ 31(4) Runtime ticket bookkeeping may be reclaimed when no source-level ticket can reference the message and other lifecycle conditions permit it.

---

## § 32. Blocking `Receive`

**Governance tags:** `concurrency.channels-v2`, `concurrency.cancellation-v2`

§ 32(1) The operation is:

```sec
fn Receive() Option[T]
```

§ 32(2) `Some(message)` means ownership of one deliverable committed message transfers to the receiver caller.

§ 32(3) `None` means the send side is permanently closed and no deliverable queued message remains.

§ 32(4) `None` never means temporary emptiness.

§ 32(5) `None` never means timeout.

§ 32(6) `None` never means cancellation.

§ 32(7) A temporarily empty open channel waits.

§ 32(8) Waiting `Receive()` is a current-execution cancellation point.

---

## § 33. Non-blocking `TryReceive`

**Governance tags:** `concurrency.channels-v2`

§ 33(1) The operation is:

```sec
fn TryReceive() ChannelTryReceiveResult[T]
```

§ 33(2) It never waits.

§ 33(3) `Received(message)` transfers ownership immediately.

§ 33(4) `Empty` means open but no deliverable message is immediately available.

§ 33(5) `Closed` means send side closed and drained.

§ 33(6) `TryReceive()` skips/reclaims any logically non-deliverable head tombstones needed to determine the immediate semantic result.

---

## § 34. Receiver `Discard`

**Governance tags:** `concurrency.channels-v2`

§ 34(1) `Discard()` destroys all currently committed queued messages up to one atomic discard boundary.

§ 34(2) It does not close RX.

§ 34(3) Future sends remain permitted.

§ 34(4) Messages committed after the discard boundary remain deliverable.

§ 34(5) Messages already received are unaffected.

§ 34(6) Every revocable message destroyed by `Discard()` subsequently yields `Unavailable(MessageDisposition.Discarded)` if its ticket is revoked.

§ 34(7) Each discarded committed message increments `ChannelStatistics.Discarded` when statistics are enabled.

---

## § 35. Receiver `Close`

**Governance tags:** `concurrency.channels-v2`

§ 35(1) `Receiver.Close()` consumes the receive capability.

§ 35(2) Receiver destruction has the same semantic closing effect.

§ 35(3) RX closure:

- permanently closes receive capability;
- destroys queued committed messages;
- prevents future receive operations;
- wakes waiting uncommitted sends;
- makes future sends return their closed outcome;
- makes pending tickets for destroyed messages resolve as `Discarded`.

§ 35(4) Each queued message destroyed by RX close increments `Discarded` when statistics are enabled.

§ 35(5) A send that has not committed at RX close retains/returns message ownership according to its declared result/terminal-cancellation path.

---

## § 36. Sender `Close`

**Governance tags:** `concurrency.channels-v2`

§ 36(1) `Sender.Close()` consumes only that sender capability.

§ 36(2) Sender destruction has the same local close effect.

§ 36(3) Closing one sender does not revoke or remove messages it already committed.

§ 36(4) The send side closes when the last live sender is closed or destroyed.

§ 36(5) Already committed messages remain available after send-side closure until received, revoked, expired, discarded, or destroyed by RX close.

---

## § 37. Drain after last sender

**Governance tags:** `concurrency.channels-v2`

§ 37(1) Closing the last sender does not destroy the receiver.

§ 37(2) The receiver may continue to drain deliverable committed messages.

§ 37(3) `Receive()` returns `None` only after the send side is closed and the channel has no deliverable queued messages.

§ 37(4) `TryReceive()` returns `Closed` under the same permanent drained condition.

§ 37(5) Expired/discarded/revoked tombstones do not postpone the semantic drained state once they are known non-deliverable.

---

## § 38. Hidden message-state model

**Governance tags:** `concurrency.channels-v2`

§ 38(1) Implementations must represent an equivalent state machine for committed message identity.

§ 38(2) A useful conceptual hidden state is:

```text
Pending
Received
Revoked
Expired
Discarded
```

§ 38(3) Only `Pending` is deliverable/revocable.

§ 38(4) Exactly one terminal transition may win.

§ 38(5) This hidden state is not a required public enum.

§ 38(6) `MessageDisposition` intentionally omits `Revoked` because a successful revoker receives `Revoked(T)` directly rather than observing an unavailable disposition.

---

## § 39. Tombstones

**Governance tags:** `concurrency.channels-v2`

§ 39(1) A physical buffered implementation may retain non-deliverable slots as tombstones until receiver-head reclamation.

§ 39(2) Tombstones are not exposed as empty or zero-valued messages.

§ 39(3) A receive may internally skip multiple tombstones before returning one message or determining empty/closed state.

§ 39(4) Sec 0.1 does not require mid-ring tombstone compaction or reuse.

§ 39(5) An implementation may use another representation if it preserves stable message/ticket identity, bounded capacity, FIFO commit semantics, and exactly-once destruction.

---

## § 40. Expiration

**Governance tags:** `concurrency.channels-v2`, `frontend.temporal-duration`

§ 40(1) Expiration applies only to messages committed through a lifetime overload.

§ 40(2) An expired message is never delivered.

§ 40(3) Expiration destroys the message exactly once.

§ 40(4) A ticket for that message thereafter returns:

```sec
Unavailable(MessageDisposition.Expired)
```

§ 40(5) Expiration may be detected lazily.

§ 40(6) One timer per message is not required.

§ 40(7) Expiration increments `ChannelStatistics.Expired` when statistics are enabled.

---

## § 41. Sender disappearance and orphaned messages

**Governance tags:** `concurrency.channels-v2`

§ 41(1) A committed message does not depend on its originating sender remaining alive.

§ 41(2) If the originating sender disappears while the message remains pending, the message is conceptually orphaned only with respect to sender provenance.

§ 41(3) Orphaned does not mean cancelled, expired, discarded, or undeliverable.

§ 41(4) An orphaned message remains deliverable.

§ 41(5) A separately owned `MessageTicket[T]` remains valid after originating sender destruction.

---

## § 42. Statistics capability

**Governance tags:** `concurrency.channels-v2`

§ 42(1) `Statistics()` is available only when the channel was constructed with `ChannelCapability.Statistics`.

§ 42(2) Sender and receiver snapshots observe the same logical channel counters.

§ 42(3) `Sent` increments exactly once for each successful send commit, including revocable sends.

§ 42(4) `Received` increments exactly once for each message delivered to receiver ownership.

§ 42(5) `Revoked` increments exactly once for each successful ticket revocation.

§ 42(6) `Expired` increments exactly once for each expiration destruction.

§ 42(7) `Discarded` increments exactly once for each message destroyed by `Discard()` or RX close.

§ 42(8) `WouldBlock`, pre-commit `Closed`, sender sharing, sender close, ticket destruction, and readiness probes do not increment message counters.

§ 42(9) Statistics must be race-free and snapshot-coherent enough that each individual counter is a valid saturating observation.

§ 42(10) The API does not promise one globally atomic multi-counter instant across all fields.

---

## § 43. FIFO ordering

**Governance tags:** `concurrency.channels-v2`

§ 43(1) Ordinary Sec 0.1 channels are FIFO by successful message commit order.

§ 43(2) One sender preserves its own successful commit order.

§ 43(3) Concurrent senders are ordered by the actual global channel commit order.

§ 43(4) No cross-sender fairness guarantee exists beyond committed FIFO ordering.

§ 43(5) Revoked, expired, and discarded messages are skipped without reordering remaining deliverable committed messages.

---

## § 44. No priority API in Sec 0.1

**Governance tags:** `concurrency.channels-v2`

§ 44(1) Sec 0.1 ordinary channels expose no priority capability, priority parameter, overflow replacement policy, or priority queue semantics.

§ 44(2) A normal channel never replaces one accepted message with another.

§ 44(3) Priority-channel design may be standardized later as an explicit extension or distinct abstraction.

---

## § 45. ISR-safe send capability

**Governance tags:** `concurrency.channels-v2`, `compiler.platform-model`

§ 45(1) `ChannelCapability.ISRSafeSend` does not create a separate channel type.

§ 45(2) It allows `TrySend()` to be considered for ISR use when the selected `CompilationPlan` validates the complete operation.

§ 45(3) ISR-safe channel send requires at least:

- statically valid storage lifetime;
- no hidden dynamic allocation;
- no blocking fallback;
- bounded execution;
- target-valid synchronization/atomics;
- interrupt-safe message destruction requirements;
- target-valid cross-context visibility.

§ 45(4) Blocking `Send`, `SendRevocable`, and `Receive` are not made ISR-safe by this capability.

§ 45(5) Unsupported ISR-safe configuration is a compile-time target/profile diagnostic.

---

## § 46. Select readiness versus commit

**Governance tags:** `concurrency.channels-v2`, `concurrency.select-v2`

§ 46(1) Selectable channel operations use distinct readiness and commit phases.

§ 46(2) Readiness inspection must be non-destructive.

§ 46(3) Only the selected branch commits.

§ 46(4) A non-selected channel send does not transfer ownership.

§ 46(5) A non-selected receive removes no message.

§ 46(6) A non-selected revocable send creates no ticket.

§ 46(7) A non-selected operation does not increment statistics.

§ 46(8) Readiness must not consume capacity, advance receiver head, or change terminal message state.

---

## § 47. Select send result typing

**Governance tags:** `concurrency.channels-v2`, `concurrency.select-v2`

§ 47(1) A selected channel operation yields its ordinary declared result type.

Example:

```sec
select {
    result := tx.Send(<-message) => {
        match result {
            Sent => {
            }

            Closed(message) => {
                Recover(<-message)
            }
        }
    }

    after 1<s> => {
        // The original message remains available because Send did not commit.
        Use(message)
    }
}
```

§ 47(2) A selected `SendRevocable` branch yields `ChannelRevocableSendResult[T]`, not a raw `MessageTicket[T]`.

§ 47(3) `TrySend` and `TryReceive` are ordinary nonblocking calls and are never primitive selectable operations in Sec 0.1. Their ordinary-call result types remain `ChannelTrySendResult[T]` and `ChannelTryReceiveResult[T]` respectively.

§ 47(4) `MessageTicket[T].Revoke`, `Share`, `Discard`, `Statistics`, and endpoint `Close` are ordinary non-waiting operations and are not primitive selectable operations.

§ 47(5) Ownership merge after `select` must account for the exact selected outcome.

---

## § 48. Waiting timeout

**Governance tags:** `concurrency.channels-v2`, `concurrency.select-v2`

§ 48(1) Sec 0.1 defines no channel-specific `SendTimed` or `ReceiveTimed` waiting API.

§ 48(2) Maximum wait duration is composed through `select` and `after`.

Example:

```sec
select {
    result := tx.Send(<-message) => {
        HandleSend(<-result)
    }

    after 20<ms> => {
        HandleTimeout(message)
    }
}
```

§ 48(3) If timeout is selected before send commit, ownership remains with the current execution.

§ 48(4) Receive timeout is analogous.

§ 48(5) Message expiration lifetime and operation wait timeout are distinct concepts.

---

## § 49. Current-execution cancellation

**Governance tags:** `concurrency.channels-v2`, `concurrency.cancellation-v2`

§ 49(1) Waiting `Send`, lifetime `Send`, `SendRevocable`, and `Receive` are current-execution cancellation points.

§ 49(2) `TrySend`, `TryReceive`, `Share`, `Revoke`, `Discard`, `Statistics`, and endpoint `Close` are non-waiting operations and are not cancellation points merely due to their channel role.

§ 49(3) If cancellation commits before a waiting channel operation commits, no channel ownership transfer occurs.

§ 49(4) A value staged for a terminally cancelled send is destroyed through normal cancellation cleanup when no reachable caller path remains to recover it.

§ 49(5) Cancellation after channel commit cannot retroactively un-send or un-receive the operation.

---

## § 50. Explicit `Context` relationship

**Governance tags:** `concurrency.channels-v2`, `concurrency.context-v1`

§ 50(1) Sec 0.1 ordinary channel methods defined by this rulebook do not add `ref Context` overloads.

§ 50(2) Merely having a `Context` in lexical scope does not cancel a channel wait.

§ 50(3) Current-task/thread cancellation follows `cancellation.md`.

§ 50(4) Operation timeouts follow `select`/`after`.

§ 50(5) A future Context-aware channel overload would require its own exact ownership/result design and must not be inferred implicitly from the mutex API.

---

## § 51. Memory synchronization

**Governance tags:** `concurrency.channels-v2`, `concurrency.memory-model-v2`

§ 51(1) Successful send commit publishes the complete transferred message.

§ 51(2) Successful receive commit acquires that message publication.

§ 51(3) Valid writes that initialize the message before send commit happen-before receiver use after matching receive commit.

§ 51(4) Rendezvous transfer establishes equivalent publication/acquire semantics.

§ 51(5) Failed/non-selected/uncommitted sends establish no message-transfer synchronization edge.

§ 51(6) Statistics observation does not publish unrelated application memory.

---

## § 52. Task/thread movement of endpoints

**Governance tags:** `concurrency.channels-v2`, `analysis.transferability`

§ 52(1) `Sender[T]` and `Receiver[T]` may move between tasks and physical threads when transferability/lifetime rules prove the move valid.

§ 52(2) Such movement remains in-process.

§ 52(3) Moving an endpoint preserves logical channel identity.

§ 52(4) Moving a sender preserves `SenderID`.

§ 52(5) The compiler/runtime must choose a representation safe for every execution context to which the endpoint can validly escape.

§ 52(6) An endpoint proven task-local may use a narrower implementation only while analysis proves it cannot later cross into a physical-thread/shared context requiring stronger synchronization.

---

## § 53. Message-type rules

**Governance tags:** `concurrency.channels-v2`, `analysis.transferability`

§ 53(1) `Channel[T]` may carry a `T` whose ownership, lifetime, alignment, destruction, and in-process transfer semantics are valid for the selected channel storage/execution paths.

§ 53(2) Move-only values transfer normally.

§ 53(3) The channel never silently clones a move-only value.

§ 53(4) References in `T` do not gain longer lifetime merely because `T` is sent.

§ 53(5) Every reference inside a queued message must remain valid through queueing, receipt, and use.

§ 53(6) Detached/long-lived channel usage should normally transfer owning values unless lifetime proof establishes safe borrowing.

§ 53(7) Process-transfer rules are irrelevant to ordinary channel eligibility because ordinary channels never cross process isolation.

---

## § 54. Channel-state lifetime

**Governance tags:** `concurrency.channels-v2`, `analysis.transferability`

§ 54(1) Hidden channel state remains alive while any live endpoint, waiting operation, queued message, or live ticket can require it.

§ 54(2) A `Channel[T]` aggregate shell is not itself the sole owner of the hidden shared state after endpoints are extracted.

§ 54(3) Hidden state may be destroyed only when:

- no sender capability remains;
- receiver capability is gone/closed;
- no wait registration can commit;
- no queued message remains live;
- no ticket can still reference ticket state;
- no select preparation retains a channel dependency.

§ 54(4) Every remaining message/value/resource must be destroyed exactly once before backing storage is reclaimed.

§ 54(5) Forced process/runtime termination may bypass ordinary deterministic cleanup according to the owning runtime/termination rules.

---

## § 55. Hidden implementation state

**Governance tags:** `concurrency.channels-v2`

§ 55(1) A conforming implementation may internally represent state such as:

```text
logical channel identity
capacity
storage/ring identity
head/tail or equivalent queue metadata
receiver-open state
live-sender count
sender slot/generation table
sender IDs
send wait registrations
receive wait registrations
select readiness/commit registrations
per-message state
per-message generation
expiration deadline metadata
ticket-reference metadata
statistics counters
synchronization primitives
allocation-domain/backing-storage identity
```

§ 55(2) These are hidden implementation obligations, not public fields.

§ 55(3) Optional features that are not enabled must not require per-message metadata or runtime work solely for those disabled features.

§ 55(4) An implementation may use a different physical representation when all public semantics and required proof facts are preserved.

---

## § 56. Target and CompilationPlan requirements

**Governance tags:** `concurrency.channels-v2`, `compiler.platform-model`

§ 56(1) The selected immutable `CompilationPlan` is target truth for concrete channel implementation capability.

§ 56(2) Relevant target/profile facts may include:

- permitted storage strategies;
- maximum capacity;
- default/max sender count;
- dynamic allocation availability;
- task/thread scheduler integration;
- cross-thread synchronization support;
- monotonic clock support for expiration/select timeout;
- ISR-safe non-blocking send support;
- required atomic/synchronization widths;
- bounded-work guarantees.

§ 56(3) Compiler-host capabilities must not substitute for selected-target facts.

§ 56(4) Unsupported statically known feature combinations are compile-time errors.

§ 56(5) Target limitations must not silently change result types, ownership commit, FIFO semantics, or lifecycle.

---

## § 57. Semantic analysis

**Governance tags:** `frontend.channels-v2`, `analysis.transferability`

§ 57(1) Sema must track at least:

- channel identity;
- message type `T`;
- capacity;
- max sender count;
- selected capabilities;
- endpoint ownership/availability;
- sender identities/generations;
- receiver uniqueness;
- message ownership at preparation/commit;
- ticket ownership and message generation;
- expiration capability/deadline facts;
- close/drain state;
- discard boundaries;
- wait/cancellation classification;
- select readiness/commit facts;
- target/profile requirements.

§ 57(2) Sema must validate exact method availability from channel capability facts.

§ 57(3) Sema must reject repeated extraction/use of moved `Tx`/`Rx`.

§ 57(4) Sema must reject copying `Channel`, `Sender`, `Receiver`, or `MessageTicket`.

§ 57(5) Sema must distinguish blocking `ChannelSendResult[T]` from `ChannelTrySendResult[T]`.

§ 57(6) Sema must resolve `MessageTicket[T].Revoke()` rather than legacy sender-owned revoke.

§ 57(7) Sema must preserve conditional ownership for select preparation and operation outcomes.

---

## § 58. Allocation/effect analysis

**Governance tags:** `concurrency.channels-v2`, `allocation.general-v2`

§ 58(1) Channel construction carries an allocation-capable effect unless analysis proves a non-dynamic realization.

§ 58(2) Ordinary send/receive/revoke/share/statistics operations are allocation-free under this rulebook.

§ 58(3) Expiration bookkeeping must be provisioned without hidden per-send dynamic allocation.

§ 58(4) Revocation/ticket bookkeeping must be provisioned without hidden per-send dynamic allocation.

§ 58(5) If the chosen target implementation cannot meet enabled capabilities without forbidden hidden allocation, compilation must fail for that plan.

---

## § 59. Deadlock/blocking analysis

**Governance tags:** `concurrency.channels-v2`, `sema.deadlock-analysis`

§ 59(1) Blocking `Send`, `SendRevocable`, and `Receive` contribute wait edges according to `blocking.md` and `deadlock_analysis.md`.

§ 59(2) Rendezvous capacity `0` must be modeled as a real synchronization dependency, not as a buffered special case.

§ 59(3) `TrySend`, `TryReceive`, `Share`, `Revoke`, `Discard`, `Statistics`, and `Close` do not contribute waiting edges merely by invocation.

§ 59(4) Select preparation does not itself commit a waiting dependency; selected branch commit follows generic select/blocking analysis.

---

## § 60. Semantic IR

**Governance tags:** `analysis.semantic-ir-v2`, `semantic-ir.channels-v2`

§ 60(1) Semantic IR must preserve channel operations/facts explicitly enough that lowering never rediscovers them from ordinary calls or names.

§ 60(2) Required semantic facts include:

- logical channel identity;
- endpoint capability identity;
- concrete `T`;
- capacity/max-senders/capabilities;
- sender identity/generation;
- operation kind;
- conditional ownership preparation;
- exact commit outcome;
- message/ticket identity and generation;
- lifetime deadline;
- message terminal disposition;
- discard/close transition;
- statistics-enabled fact;
- select readiness versus commit;
- cancellation-point classification;
- memory-synchronization relation;
- source provenance;
- target capability requirements.

§ 60(3) Concrete Semantic IR opcode names remain owned by `semantic_ir.md`.

§ 60(4) The legacy opcode-name list in the previous channel book is not normative.

§ 60(5) IR verification must reject duplicated ownership, impossible ticket/message identities, and contradictory committed/uncommitted states.

---

## § 61. Lowering

**Governance tags:** `lowering.channels-v2`, `compiler.platform-model`

§ 61(1) Lowering consumes validated channel Semantic IR plus the selected `CompilationPlan`.

§ 61(2) Lowering must preserve:

- exactly-once message ownership transfer;
- exactly-one terminal message disposition;
- FIFO successful-commit order;
- sender-generation safety;
- receiver uniqueness;
- ticket-generation safety;
- cancellation/select commit atomicity;
- publication/acquire memory semantics;
- exactly-once destruction;
- saturating statistics semantics.

§ 61(3) Lowering must not turn `TrySend` into a blocking operation.

§ 61(4) Lowering must not introduce hidden dynamic allocation into operations declared allocation-free.

§ 61(5) Lowering may specialize away unused capability metadata when source semantics remain identical.

---

## § 62. LSP hover

**Governance tags:** `tooling.channels-v2`

§ 62(1) Hover must expose exact source-visible declarations, generic arity, `@noCopy`, result variants/payloads, and comments.

§ 62(2) Hover for `Channel[T]` must show `Tx`, `Rx`, and both fallible constructors.

§ 62(3) Hover for `Sender[T]` must show the exact method overload set and capability preconditions.

§ 62(4) Hover for `Receiver[T]` must show exact receive/discard/statistics/close methods.

§ 62(5) Hover for `MessageTicket[T]` must show ticket-owned `Revoke()`.

§ 62(6) Hover must distinguish `ChannelSendResult[T]` from `ChannelTrySendResult[T]`.

§ 62(7) Hover must not show `Priority` as a Sec 0.1 channel capability.

---

## § 63. Completion and navigation

**Governance tags:** `tooling.channels-v2`

§ 63(1) Completion on `Channel[T]` must expose canonical `Tx` and `Rx`.

§ 63(2) Completion on `Sender[T]` must use canonical CamelCase methods.

§ 63(3) Capability-dependent operations should be offered only when the compiler/LSP has sufficient channel-origin capability facts, or clearly marked unavailable with the required capability.

§ 63(4) Completion on `MessageTicket[T]` must offer `Revoke`, not sender-owned `Revoke`.

§ 63(5) Navigation must resolve compiler-known and core/source-visible identities as one semantic symbol.

§ 63(6) LSP must consume shared Sema channel facts rather than rebuilding an independent capability/result model.

---

## § 64. Diagnostics

**Governance tags:** `tooling.channels-v2`, `frontend.channels-v2`

§ 64(1) Suggested diagnostics include:

```text
Channel requires exactly one message type
```

```text
ChannelOptions.MaxSenders must be at least 1
```

```text
channel supports at most 8 live senders
```

```text
channel does not enable Revocation
```

```text
message lifetime requires ChannelCapability.Expiration
```

```text
Statistics() requires ChannelCapability.Statistics
```

```text
target does not support ISR-safe TrySend for this channel configuration
```

```text
use of moved channel endpoint Tx
```

```text
MessageTicket[Message] has already been consumed
```

```text
channel construction may allocate but no valid allocation context is available
```

§ 64(2) Diagnostics should explain whether failure is source semantic, ownership/lifetime, capability selection, allocation, or target support.

§ 64(3) Diagnostics must not suggest converting ordinary channels to IPC implicitly.

---

## § 65. Restrictions

**Governance tags:** `concurrency.channels-v2`

§ 65(1) Ordinary channels must not become bidirectional implicitly.

§ 65(2) They must not expose queue storage.

§ 65(3) They must not copy endpoints or tickets.

§ 65(4) They must not clone move-only messages.

§ 65(5) They must not transfer message ownership before semantic commit.

§ 65(6) They must not return committed messages through a pre-commit failure outcome.

§ 65(7) `Receive()` must not use `None` for temporary emptiness.

§ 65(8) `TrySend()` must not use `Option[ChannelSendResult[T]]`.

§ 65(9) Blocking `Send()` must not expose a `WouldBlock` variant.

§ 65(10) Receiver close must not produce a separate `ReceiverClosed` `MessageDisposition`.

§ 65(11) Channel construction must not introduce `ChannelCreateError` merely to wrap allocation failure.

§ 65(12) Disabled optional capabilities must not impose required per-message runtime overhead.

§ 65(13) Channels must not bypass ownership, lifetime, destruction, cancellation, or allocation rules.

§ 65(14) Ordinary channels must not replace IPC semantics.

---

## § 66. Explicitly absent Sec 0.1 APIs

**Governance tags:** `concurrency.channels-v2`

§ 66(1) Sec 0.1 ordinary channels do not define:

```text
multi-consumer Receiver sharing
unbounded Channel[T]
broadcast channels
latest-value channels
priority channels
priority replacement/overflow policies
SendTimed
ReceiveTimed
SendTimedRevocable
Context-aware channel overloads
sender-wide drain/retract
manual queue storage access
implicit process bridging
```

§ 66(2) These absences are part of the current portable surface and must not be filled by ad-hoc compiler-only extensions under the same canonical type identities.

---

## § 67. Conformance scenarios

**Governance tags:** `concurrency.channels-v2`

§ 67(1) Conformance tests must include at least:

- capacity-zero rendezvous;
- bounded buffered backpressure;
- `Send` versus `TrySend` result typing;
- ownership return on `Closed`/`WouldBlock`;
- last-sender close and receiver drain;
- RX close with waiting sends;
- `Share()` sender limit and generation reuse;
- revocable send pre-commit close;
- successful revoke;
- receive/revoke race;
- expiration/revoke race;
- discard/revoke race;
- RX-close/revoke disposition;
- ticket destruction without revoke;
- sender destruction with pending message/ticket;
- `TryReceive` Empty versus Closed;
- statistics counter semantics and saturation;
- non-destructive select readiness;
- selected/non-selected send ownership;
- cancellation before versus after commit;
- no hidden allocation in ordinary operations;
- ISR-safe `TrySend` plan validation;
- channel versus IPC type-boundary rejection.

---

## § 68. Governance

**Governance tags:** `concurrency.channels-v2`, `frontend.channels-v2`, `tooling.channels-v2`, `semantic-ir.channels-v2`, `lowering.channels-v2`, `concurrency.select-v2`, `concurrency.cancellation-v2`, `concurrency.memory-model-v2`, `allocation.general-v2`, `analysis.transferability`, `sema.deadlock-analysis`, `compiler.platform-model`

§ 68(1) `governance/concurrency_channels.yaml` is the sole canonical implementation-status owner for the channel integration introduced by this revision.

§ 68(2) Root `implementation-status.yaml` is not a second canonical ledger for new channel work.

§ 68(3) Related compiler phases are linked from the owning channel integration rather than duplicating the same integration into multiple governance fragments.

§ 68(4) `select.md` remains owner of generic selection ordering/readiness/commit syntax.

§ 68(5) `cancellation.md` remains owner of current-execution cancellation and Context semantics.

§ 68(6) `concurrency_memory_model.md` remains owner of general happens-before/acquire/release rules.

§ 68(7) `ipc.md` remains owner of process IPC and process-shared communication.

§ 68(8) `allocation.md` remains owner of `AllocationError` and general allocation semantics.

§ 68(9) Cross-rulebook synchronization required by this revision is tracked by the accompanying channel v2 correction.
