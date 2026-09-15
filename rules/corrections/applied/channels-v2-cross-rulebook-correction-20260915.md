# Correction — Channels v2 cross-rulebook synchronization

- **Status:** Applied
- **Created:** 2026-09-15
- **Last updated:** 2026-09-15
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `main-reviewed-2026-09-15`
- **Primary owning rulebook:** `rules/concurrency/channels.md`
- **Implementation governance:** `governance/concurrency_channels.yaml`
- **Classification:** Normative synchronization of decided ordinary-channel semantics
- **Canonical applied path:** `rules/corrections/applied/channels-v2-cross-rulebook-correction-20260915.md`

---

## 1. Canonical source surface

Synchronize compiler-known/core declarations, Sema, LSP, tests, examples, and docs to the exact channel v2 declarations in `rules/concurrency/channels.md`.

Required identities include:

```sec
@noCopy
type Channel[T]

@noCopy
type Sender[T]

@noCopy
type Receiver[T]

@noCopy
type MessageTicket[T]

enum ChannelCapability {
    Revocation
    Expiration
    ISRSafeSend
    Statistics
}

type ChannelOptions struct {
    Capacity: uint
    MaxSenders: uint
    Capabilities: set[ChannelCapability]
}

type ChannelStatistics struct {
    Sent: uint64
    Received: uint64
    Revoked: uint64
    Expired: uint64
    Discarded: uint64
}
```

Also synchronize the exact result/disposition declarations:

```sec
type ChannelSendResult[T] union {
    Sent
    Closed(T)
}

type ChannelTrySendResult[T] union {
    Sent
    WouldBlock(T)
    Closed(T)
}

type ChannelTryReceiveResult[T] union {
    Received(T)
    Empty
    Closed
}

type ChannelRevocableSendResult[T] union {
    Accepted(MessageTicket[T])
    Closed(T)
}

enum MessageDisposition {
    Received
    Expired
    Discarded
}

type ChannelRevokeResult[T] union {
    Revoked(T)
    Unavailable(MessageDisposition)
}
```

Do not replace these exact unions with `Option`, `Result`, bool flags, or a shared wider union that introduces impossible exhaustive-match arms.

---

## 2. Channel endpoint members

Replace legacy:

```text
channel.tx
channel.rx
```

with:

```text
channel.Tx
channel.Rx
```

`Tx` and `Rx` are owning fields/capabilities and follow ordinary partial-move availability rules.

Repeated extraction from the same field must be rejected.

Do not duplicate endpoint capabilities merely because `Channel[T]` remains in scope.

---

## 3. Constructors and allocation

Synchronize `Channel[T]` construction to fallible Sec constructor semantics:

```sec
try init(capacity: uint) AllocationError
try init(options: ChannelOptions) AllocationError
```

Example call:

```sec
let channel := try Channel[Message](32)
```

Use `AllocationError` only when dynamic allocation is required and ordinary allocation can fail.

Do not introduce `ChannelCreateError`.

Invalid compile-time configuration and unsupported target/profile combinations remain compile-time diagnostics.

If the canonical `AllocationError` source-visible declaration/variants are not yet fully declared in their owning allocation/core rulebook/source, complete that declaration in the allocation/core owner rather than inventing a channel-local copy.

---

## 4. ChannelOptions/capabilities

The Sec 0.1 capability set is exactly:

```text
Revocation
Expiration
ISRSafeSend
Statistics
```

Remove legacy `priority` capability references from the current Sec 0.1 channel API.

`MaxSenders >= 1`.

`MaxSenders` and `Capabilities` must be compile-time-known for construction.

The simple capacity constructor uses no optional capabilities and the selected profile's finite default sender limit.

No unused capability may require mandatory per-message runtime metadata/work.

---

## 5. `Sender[T].Share()`

Change the public signature from:

```sec
fn Share() Sender[T]
```

to:

```sec
fn Share() Option[Sender[T]]
```

Semantics:

```text
Some(sender)
    a new sender capability and generation-safe SenderID were created

None
    MaxSenders is already reached
```

`Share()` does not consume the original sender.

Receiver closure does not redefine `None`.

---

## 6. Send results

Change blocking send to:

```sec
fn Send(message: T) ChannelSendResult[T]
```

with exactly:

```text
Sent
Closed(T)
```

Change nonblocking send to:

```sec
fn TrySend(message: T) ChannelTrySendResult[T]
```

with exactly:

```text
Sent
WouldBlock(T)
Closed(T)
```

`WouldBlock(T)` applies both to a full buffered channel and an unmatched zero-capacity rendezvous channel.

Both TrySend non-commit variants return ownership of `T`.

Do not use:

```sec
Option[ChannelSendResult[T]]
```

because `None` would not carry the uncommitted ownership.

---

## 7. Revocable send and ticket-owned revoke

Replace legacy direct-ticket return:

```sec
fn SendRevocable(message: T) MessageTicket[T]
```

with:

```sec
fn SendRevocable(message: T) ChannelRevocableSendResult[T]
fn SendRevocable(message: T, lifetime: duration) ChannelRevocableSendResult[T]
```

where:

```text
Accepted(MessageTicket[T])
Closed(T)
```

are exhaustive.

Move revocation authority from legacy:

```sec
sender.Revoke(ticket)
```

to:

```sec
ticket.Revoke()
```

with:

```sec
impl MessageTicket[T] {
    fn Revoke() ChannelRevokeResult[T]
}
```

`Revoke()` consumes the ticket.

The original Sender[T] may already have been moved, closed, or destroyed.

Ticket destruction without Revoke relinquishes revocation authority but leaves the committed message untouched.

---

## 8. Receiver and message disposition

Keep:

```sec
fn Receive() Option[T]
fn TryReceive() ChannelTryReceiveResult[T]
fn Discard() void
fn Close() void
```

`Receive() == None` means only send-side closed and drained.

`TryReceive()` must distinguish:

```text
Received(T)
Empty
Closed
```

`Receiver.Close()` and receiver destruction destroy queued committed messages.

Their ticket disposition is:

```text
MessageDisposition.Discarded
```

Do not add `ReceiverClosed` to `MessageDisposition`.

---

## 9. Statistics

When `ChannelCapability.Statistics` is enabled, both endpoints expose:

```sec
fn Statistics() ChannelStatistics
```

Counters are saturating `uint64`.

Count:

```text
Sent
    successful send commits

Received
    ownership transfers to receiver

Revoked
    successful ticket revokes

Expired
    expiration destructions

Discarded
    Receiver.Discard/Close message destructions
```

Do not add queue depth, waiter counts, timing, capacity, or implementation counters to this public snapshot.

Non-selected select readiness, WouldBlock, Closed-before-commit, Share, endpoint Close itself, and ticket destruction do not count as message commits.

---

## 10. Expiration

`Send(message, lifetime: duration)` requires `Expiration`.

`SendRevocable(message, lifetime: duration)` requires both `Revocation` and `Expiration`.

The lifetime begins at successful send commit and is not an operation wait timeout.

Expiration must be implementable without hidden per-send dynamic allocation.

Remove/ignore legacy priority-channel sections from current v0.1 semantics.

---

## 11. Select synchronization

Update `rules/concurrency/select.md` channel examples.

A selected operation yields its ordinary exact result type.

In particular, a selected revocable send no longer binds a raw ticket directly:

```sec
result := tx.SendRevocable(<-message)
```

has type:

```text
ChannelRevocableSendResult[T]
```

Only `Accepted(ticket)` contains the ticket.

Non-selected channel branches:

- move no message;
- remove no receive message;
- create no ticket;
- increment no statistics;
- change no queue/message terminal state.

Preserve generic select ownership merge and source-order priority in `select.md`.

---

## 12. Cancellation and Context

Keep waiting:

```text
Send
Send with lifetime
SendRevocable
Receive
```

as current-execution cancellation points.

Keep:

```text
TrySend
TryReceive
Share
Revoke
Discard
Statistics
Close
```

non-waiting.

Do not add `ref Context` channel overloads in Sec 0.1.

A Context in lexical scope does not implicitly cancel a channel wait.

Wait timeouts remain composed through `select`/`after`.

`cancellation.md` remains owner of current task/thread cancellation and Context semantics.

---

## 13. IPC boundary

Keep ordinary:

```text
Channel[T]
Sender[T]
Receiver[T]
MessageTicket[T]
```

strictly in-process.

They are not automatically ProcessTransferable and never become:

```text
IPCSender[T]
IPCReceiver[T]
```

through lowering.

`ipc.md` remains owner of process messaging and capability/resource transfer.

---

## 14. Memory model

Ensure `concurrency_memory_model.md` retains:

```text
successful send commit
    publishes transferred message state

matching successful receive commit
    acquires that publication
```

Failed/non-selected/uncommitted sends do not create a message-transfer synchronization edge.

Rendezvous handoff provides equivalent publication/acquire semantics.

---

## 15. Semantic IR

`semantic_ir.md` must be able to preserve channel identity, endpoint identity, capacity/options, sender generation, message/ticket generation, conditional ownership, exact result outcome, expiration deadline, terminal disposition, discard/close, statistics-enabled fact, select readiness/commit, cancellation classification, synchronization and target requirements.

Do not treat the legacy channel rulebook's literal opcode-name list as canonical unless those names are independently defined by `semantic_ir.md`.

The rulebook owns source semantics; `semantic_ir.md` owns concrete IR vocabulary.

---

## 16. Tooling

LSP hover must show all exact declarations and variant payloads.

Completion must:

- use `Tx` / `Rx`;
- offer `Revoke()` on `MessageTicket[T]`, not on `Sender[T]`;
- distinguish blocking and TrySend result types;
- omit `Priority`;
- expose Statistics only when capability facts permit or mark the capability requirement.

Navigation must treat compiler-known/source-visible core identities as one semantic symbol.

---

## 17. `language-rulebook-status.md`

Change:

```text
concurrency/channels.md | Written — sync required
```

to `Written`.

Suggested note:

```text
Revision 2.0 defines exact in-process Channel/endpoint/result APIs,
allocation-only fallible construction, Share limits, revocable-ticket ownership,
expiration/statistics, select/cancellation commit semantics, and strict IPC separation.
```

---

## 18. Governance

Populate the already-registered canonical fragment:

```text
governance/concurrency_channels.yaml
```

with `concurrency.channels-v2`.

Do not create a new `implementation-status-channels.yaml`.

Do not append a duplicate integration to root `implementation-status.yaml`.

After this correction is fully applied, archive it under:

```text
rules/corrections/applied/
```

and update the governance entry from `corrections_pending` to the corresponding applied-correction record.

---

## 19. Required tests

Synchronize/add tests for:

- fallible allocation-only construction;
- `Tx`/`Rx` ownership and partial moves;
- MaxSenders/Share Option;
- SenderID generation-safe reuse;
- blocking Send exhaustive two-variant result;
- TrySend exhaustive three-variant result;
- WouldBlock ownership return;
- Receive Option closed/drained semantics;
- TryReceive Empty versus Closed;
- SendRevocable Accepted/Closed;
- ticket-owned Revoke;
- ticket lifetime independent from sender;
- ticket destruction without revoke;
- RX close -> Discarded disposition;
- expiration/revoke race;
- receive/revoke race;
- discard/revoke race;
- statistics exact counters and saturation;
- non-destructive select readiness;
- cancellation before/after commit;
- no hidden ordinary-operation allocation;
- ISR-safe TrySend validation;
- ordinary-channel process-boundary rejection.
