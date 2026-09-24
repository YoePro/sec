# Sec Inter-Process Communication

- **Status:** Normative
- **Created:** 2026-09-15
- **Last updated:** 2026-09-15
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/ipc.md`
- **Implementation governance:** `governance/concurrency_ipc.yaml`
- **Governance tags:** `concurrency.ipc-v1`, `frontend.ipc-v1`, `analysis.ipc-v1`, `semantic-ir.ipc-v1`, `platform.ipc-v1`
- **Replaces:** Planned IPC entry; no earlier canonical IPC rulebook

---

## 1. Purpose and authority

This rulebook defines the canonical Sec 0.1 inter-process communication model.

It owns the source and semantic contracts for:

```text
anonymous byte pipes
typed process-to-process message transport
process-shared backing storage
process-shared synchronization
safe typed shared values
process-shared atomics
process capability/resource transfer
IPC lifecycle and peer-loss behavior
IPC target capabilities
IPC Semantic IR, analysis, diagnostics, and lowering obligations
```

This rulebook does not redefine:

```text
ordinary in-process Channel[T]
Process[T] or Command lifecycle
ProcessTransferable eligibility
ordinary ownership, borrowing, copy/move, or destruction
ordinary Atomic[T] and the general MemoryOrder model
network RPC
local-service discovery or named IPC namespaces
external serialization formats
```

The owning rulebooks for those areas remain authoritative. IPC consumes their contracts and adds process-isolation semantics where required.

A Sec process boundary is a semantic isolation boundary. IPC never turns ordinary process-local references, raw addresses, native descriptor numbers, or implementation handles into portable cross-process ownership merely by copying their representation.

---

## 2. Core IPC families

Sec 0.1 defines four fundamental IPC families:

```text
Pipe
Typed message IPC
Shared memory
Capability/resource transfer
```

Process-shared synchronization and process-shared atomics are explicit facilities built on the same IPC boundary model.

The families are semantically distinct:

- a pipe is an ordered byte stream with no message boundaries;
- typed IPC transfers complete typed messages;
- shared memory exposes one backing-storage identity through process-local mappings;
- capability transfer preserves authority to a resource while allowing the destination process to receive its own valid local representation.

The implementation may realize these families through different native mechanisms on different targets. Native mechanisms do not alter the canonical Sec contract.

---

## 3. Ordinary channels never become IPC

`Channel[T]`, `Sender[T]`, and `Receiver[T]` remain in-process concurrency abstractions.

They may coordinate tasks and threads inside one process address space, but they do not cross a process isolation boundary and are never automatically lowered to IPC.

Typed process messaging uses:

```sec
IPCSender[T]
IPCReceiver[T]
```

The process boundary is therefore source-visible.

The compiler must reject any attempt to transfer an ordinary in-process channel capability across a process boundary unless a future rulebook defines a distinct explicit adapter. Sec 0.1 defines no such adapter.

---

## 4. IPC and RPC are different layers

Typed IPC is message transport, not remote-call semantics.

`IPCSender[T]` and `IPCReceiver[T]` provide:

```text
message transfer
message ordering
backpressure
ownership transfer
peer lifecycle observation through IPC results
```

They do not implicitly provide:

```text
method or service identity
request/response correlation
request IDs
RPC deadlines
remote application exceptions
RPC protocol negotiation
service discovery
```

RPC may use IPC as one possible transport. IPC does not depend on RPC.

---

## 5. Public naming

Established initialisms retain uppercase spelling inside public Sec identifiers.

The canonical typed IPC names are therefore:

```sec
IPCSender[T]
IPCReceiver[T]
IPCPair[T]
IPCMutex
IPCSemaphore
IPCAtomic[T]
```

Spellings such as `IpcSender`, `IpcReceiver`, or `IpcMutex` are not canonical public API names.

---

## 6. Compiler-known status

The following IPC types are compiler-known/core-visible concepts in Sec 0.1:

```text
Pipe
PipeReader
PipeWriter

IPCPair[T]
IPCSender[T]
IPCReceiver[T]
IPCError

SharedMemory
SharedReadMapping
SharedWriteMapping
SharedMemoryError

IPCMutex
IPCMutexGuard
IPCSemaphore
IPCSyncError

SharedValue[T]
SharedValueGuard[T]
IPCSharedError

IPCAtomic[T]
IPCAtomicError
```

Compiler-known status means that parser/Sema, ownership analysis, LSP, Semantic IR, target planning, runtime support, and lowering agree on one canonical type identity and member surface.

It does not permit a backend-only private replacement with different source semantics.

---

## 7. Compiler semantic properties

Sec 0.1 uses three relevant compiler-derived semantic properties.

### 7.1 `ProcessTransferable`

`ProcessTransferable` remains owned by `rules/memory/transferability.md`.

It answers whether a semantic value can validly cross a process isolation boundary under a defined process-transfer representation or capability adapter.

IPC consumes that proof.

### 7.2 `SharedStorable(T)`

`SharedStorable(T)` is a compiler-inferred semantic property. It is not a source interface and has no user-written conformance declaration in Sec 0.1.

A type may satisfy `SharedStorable(T)` only when the compiler can provide a fixed-size, self-contained canonical shared representation with:

```text
no process-local safe reference
no process-local RawPtr meaning
no resource-owning field
no destructor-dependent ownership state
no process-local allocation identity requirement
no hidden pointer graph requiring one process heap
```

Primitive fixed-size value types may qualify.

Fixed arrays qualify recursively when their element type qualifies.

Structs, enums, unions, `Option[T]`, and `Result[T, E]` may qualify only when their complete stored representation recursively qualifies and the compiler defines one canonical shared representation.

Examples that do not qualify in Sec 0.1 include ordinary:

```text
string
ref T
ref mut T
RawPtr[T]
owning arrays
slices
collections
File
Process[T]
Task[T]
Thread[T]
PipeReader
PipeWriter
IPCSender[T]
IPCReceiver[T]
SharedMemory
IPCMutex
```

A type requiring custom `free` or equivalent resource destruction is not `SharedStorable` in Sec 0.1.

### 7.3 `IPCAtomicValue(T)`

`IPCAtomicValue(T)` is also compiler-inferred and is not a source interface.

A type may satisfy it only when:

- it is a compiler-approved scalar atomic value category;
- it has a stable atomic comparison/replacement semantics;
- it does not contain process-local address meaning;
- the selected target can provide the complete lock-free process-shared operation set required by `IPCAtomic[T]`.

The Sec 0.1 eligible semantic set is intentionally conservative:

```text
bool
primitive signed integer types
primitive unsigned integer types
integer-backed enums whose contracts permit the operation
named types whose underlying eligible scalar semantics remain valid
```

`RawPtr[T]` is excluded from `IPCAtomicValue(T)` even though ordinary `Atomic[RawPtr[T]]` may be permitted by the ordinary atomics rulebook. A process-local address does not gain cross-process meaning through atomic storage.

---

# Part I — Pipes

## 8. Pipe model

A pipe is an ordered, unidirectional byte stream:

```text
PipeWriter -> bytes -> PipeReader
```

A pipe has no portable message boundaries.

Multiple `Write` or `WriteAll` operations produce one ordered byte stream. A receiver cannot infer write-call boundaries from the stream.

---

## 9. Exact pipe declarations

The canonical declarations are:

```sec
@noCopy
type Pipe struct {
    Reader: PipeReader,
    Writer: PipeWriter,
}

@noCopy
type PipeReader

@noCopy
type PipeWriter

fn CreatePipe() Result[Pipe, IOError]
```

`Pipe` is an ordinary owning aggregate whose two fields are independently tracked by normal partial-move and destruction rules.

`Pipe` has no custom lifecycle operation of its own.

Example:

```sec
let pipe := try CreatePipe()
let reader :<- pipe.Reader
let writer :<- pipe.Writer
```

After moving one field, the moved field is unavailable. Remaining fields are destroyed normally when their owner ends.

---

## 10. Exact `PipeReader` surface

```sec
impl PipeReader {
    property IsClosed: bool {
        get
    }

    fn Read(buffer: ref mut byte[]) Result[uint, IOError]
    fn ReadExact(buffer: ref mut byte[]) Result[uint, IOError]
    fn Duplicate() Result[PipeReader, IOError]
    fn Close() Result[void, IOError]

    free {
        // Best-effort close of an open endpoint.
    }
}
```

`PipeReader` is move-only and owns one read capability for one logical pipe.

---

## 11. Exact `PipeWriter` surface

```sec
impl PipeWriter {
    property IsClosed: bool {
        get
    }

    fn Write(data: ref byte[]) Result[uint, IOError]
    fn WriteAll(data: ref byte[]) Result[uint, IOError]
    fn Duplicate() Result[PipeWriter, IOError]
    fn Close() Result[void, IOError]

    free {
        // Best-effort close of an open endpoint.
    }
}
```

`PipeWriter` is move-only and owns one write capability for one logical pipe.

There is no pipe-specific `Flush()` in Sec 0.1.

---

## 12. Pipe creation

`CreatePipe()` creates both endpoints atomically from source semantics.

On success it returns one complete `Pipe`.

On failure it returns `Err(IOError)` and exposes no partial endpoint.

If native setup creates one temporary endpoint before another step fails, the runtime must close all temporary state before returning the error.

A selected target that statically cannot implement the canonical pipe contract rejects `CreatePipe()` at compile time. Statically unsupported targets do not return `IOError.Unsupported` merely because pipe support is absent.

---

## 13. Pipe `Read`

For a non-empty buffer:

```sec
reader.Read(ref mut buffer[..])
```

may return any successful count in:

```text
1..buffer.Len
```

when bytes are available.

A successful count may be smaller than the buffer length even when the stream has not ended.

For a non-empty buffer:

```text
Ok(0)
```

means end-of-stream.

For an empty buffer, `Read` returns `Ok(0)` immediately and that result does not establish EOF.

---

## 14. Pipe EOF

EOF is observable only after:

```text
all write capabilities for the logical pipe are closed/released
and
all previously accepted buffered bytes are consumed
```

Closing one of several duplicated writers does not produce EOF while another writer capability remains alive.

Buffered bytes are always delivered before EOF.

---

## 15. Pipe `ReadExact`

`ReadExact` continues until the requested buffer is completely filled or the operation cannot complete.

On success:

```text
Ok(buffer.Len)
```

is returned.

If EOF occurs after only a prefix can be read, the operation returns:

```sec
Err(IOError.UnexpectedEndOfFile)
```

The already-read prefix remains ordinary caller buffer state according to the canonical I/O contract.

`ReadExact` is a composed operation and may perform multiple underlying waits.

---

## 16. Pipe `Write`

`Write` may accept fewer bytes than the provided slice length.

A successful result:

```text
Ok(count)
```

means that the first `count` bytes have committed to the logical pipe transport.

An empty write returns `Ok(0)` immediately.

`Write` does not mean that the peer has already read the accepted bytes.

---

## 17. Pipe `WriteAll`

`WriteAll` attempts to commit the complete provided byte sequence.

On success:

```text
Ok(data.Len)
```

is returned.

The operation is not transactional with respect to already written bytes. If a prefix commits and a later write fails, the committed prefix remains in the stream and `WriteAll` returns the applicable `IOError`.

A zero-progress condition where progress is required may use the canonical `IOError.WriteZero` contract.

---

## 18. Broken read side

When all read capabilities for the logical pipe are closed/released, a later write that cannot be delivered returns:

```sec
Err(IOError.BrokenPipe)
```

Closing one duplicated reader does not establish `BrokenPipe` while another reader capability remains alive.

---

## 19. Pipe buffering and backpressure

Pipe buffering is implementation-defined and bounded.

A canonical implementation may use:

```text
zero-capacity rendezvous
bounded kernel buffering
bounded runtime buffering
another bounded equivalent
```

Sec 0.1 exposes no portable pipe capacity property and no capacity parameter to `CreatePipe()`.

The runtime must not satisfy writes through hidden unbounded buffering.

When bounded capacity is exhausted, a writer may wait for read-side progress.

---

## 20. Pipe close and destruction

`Close()` closes the local endpoint capability.

`Close()` is idempotent. Re-closing an already closed endpoint returns success.

After successful close:

```text
IsClosed == true
```

An operation attempted on a locally closed endpoint returns the canonical bad-endpoint error from `IOError` where runtime state is involved. When local closed state is statically provable, the compiler should diagnose the invalid operation earlier.

`free` performs best-effort close when the endpoint remains open. Cleanup failure cannot propagate a `Result` from `free`.

`IsClosed` is local endpoint state. It does not claim that the peer endpoint still exists.

---

## 21. Pipe duplication

`Duplicate()` creates a new independently owned capability to the same logical pipe side.

It does not create a second pipe and does not copy buffered data.

Example:

```sec
let secondWriter := try writer.Duplicate()
```

After success:

```text
writer       -> logical pipe P, write side
secondWriter -> logical pipe P, write side
```

The owners are distinct. The logical resource identity is the same.

Duplication is all-or-nothing. Failure does not modify the original capability.

---

## 22. Pipe ordering and atomicity

Bytes accepted in one logical writer sequence preserve byte order.

Sec does not promise portable whole-write atomicity between duplicated concurrent writers.

A program requiring complete message boundaries or message atomicity uses typed IPC rather than depending on target-native pipe buffer rules.

---

## 23. Pipe waiting and `select`

`Read` and `Write` are primitive selectable operations.

`ReadExact` and `WriteAll` are composed operations and are not primitive selectable branches in Sec 0.1.

A selectable pipe operation has distinct readiness and commit phases.

A non-selected `Read` branch:

```text
does not consume bytes
does not modify the user buffer as a committed read
```

A non-selected `Write` branch:

```text
does not write bytes
```

A read branch is ready when it can return at least one byte, EOF, or an already established error without further waiting.

A write branch is ready when it can accept at least one byte or return an already established error without further waiting.

Native temporary non-readiness must normally integrate with the Sec wait/readiness model rather than leak as `IOError.WouldBlock` from the ordinary blocking/suspending pipe API.

---

## 24. Pipe process transfer

`PipeReader` and `PipeWriter` are process-transferable capabilities when the selected target provides the required route adapter.

Moving an endpoint across a process boundary gives the destination a capability to the same logical pipe.

The operation does not serialize a native descriptor number and does not create a new pipe.

Existing named owners consumed by a process-boundary operation require the ordinary explicit Sec move marker.

Example:

```sec
let child := try spawn process Consume(<-reader)
```

Successful process-start transfer makes the parent binding unavailable.

Failed pre-commit process startup leaves the source endpoint available according to the process transactional-transfer contract.

---

# Part II — Typed IPC

## 25. Typed IPC model

Typed IPC is an ordered typed message transport across process isolation domains.

Conceptually:

```text
IPCSender[T] -> complete T messages -> IPCReceiver[T]
```

Every successful `Send` commits exactly one complete semantic message.

The receiver never observes a partial `T`, raw message fragments, or bytes from different messages merged into one `T`.

Typed IPC is not restricted to parent/child relationships. Any processes that validly establish or receive the required capabilities may communicate.

---

## 26. Exact typed IPC declarations

```sec
@noCopy
type IPCPair[T] struct {
    Sender: IPCSender[T],
    Receiver: IPCReceiver[T],
}

@noCopy
type IPCSender[T]

@noCopy
type IPCReceiver[T]

fn CreateIPC[T]() Result[IPCPair[T], IPCError]
```

The compiler requires `ProcessTransferable(T)` for the complete typed message contract.

If `T` is statically not process-transferable, `CreateIPC[T]()` is rejected at compile time.

---

## 27. Exact `IPCError`

```sec
enum IPCError error {
    Closed
    PeerClosed
    TransferFailed
    MaterializationFailed
    OutOfMemory
    ResourceLimit
    ProtocolFailure
    NativeFailure
}
```

### `Closed`

A requested operation used a locally closed typed IPC endpoint.

### `PeerClosed`

The peer capability family no longer contains any endpoint able to satisfy the requested operation. It is primarily a send-side failure.

Clean exhaustion of all sender capabilities on the receive side is represented by `Ok(None)`, not `PeerClosed`.

### `TransferFailed`

An outgoing semantic message was valid in principle but its complete process-transfer representation could not commit.

For a failed consuming send, source ownership remains available.

### `MaterializationFailed`

An incoming transfer could not be established as one complete valid destination `T`.

No partial `T` becomes source-visible.

### `OutOfMemory`

Required runtime transfer/materialization memory could not be obtained.

### `ResourceLimit`

A bounded transport, capability count, materialization limit, queue limit, message size limit, or applicable target/runtime resource quota prevented the operation.

### `ProtocolFailure`

The established IPC transport observed invalid or contradictory transport metadata, framing, schema identity, capability records, or another state that violates the canonical typed-transport protocol.

This is not an application-level protocol error.

### `NativeFailure`

A target-native transport failure occurred and no more specific portable `IPCError` describes it.

---

## 28. Exact `IPCSender[T]` surface

```sec
impl IPCSender[T] {
    property IsClosed: bool {
        get
    }

    fn Send(message: T) Result[void, IPCError]
    fn Duplicate() Result[IPCSender[T], IPCError]
    fn Close() void

    free {
        // Cleanly releases this local sender capability if still open.
    }
}
```

`IPCSender[T]` is move-only.

---

## 29. Exact `IPCReceiver[T]` surface

```sec
impl IPCReceiver[T] {
    property IsClosed: bool {
        get
    }

    fn Receive() Result[Option[T], IPCError]
    fn Duplicate() Result[IPCReceiver[T], IPCError]
    fn Close() void

    free {
        // Cleanly releases this local receiver capability if still open.
    }
}
```

`IPCReceiver[T]` is move-only.

---

## 30. Typed IPC creation

`CreateIPC[T]()` creates one logical typed transport with one sender and one receiver capability.

Creation is atomic from source semantics.

On success:

```text
one complete IPCPair[T]
```

is returned.

On failure, no user-visible partial endpoint exists.

The selected target must provide `IPCTypedMessages` and a complete realizable transfer plan for `T`.

---

## 31. `Send` ownership

`Send(message: T)` follows ordinary Sec parameter and ownership rules.

A copyable value may be copied when the transfer contract permits it.

A named existing value whose ownership must be consumed uses explicit `<-` at the call site:

```sec
try sender.Send(<-message)
```

A fresh temporary needs no synthetic move marker.

---

## 32. Transactional send commit

Consuming `Send` is transactional with respect to source ownership.

Before semantic commit:

```text
source owns the complete message
```

On `Ok()`:

```text
complete message ownership commits to the IPC transport/destination path
source consumed binding becomes unavailable
```

On `Err(...)` before commit:

```text
source ownership remains complete
no source-visible partial field transfer exists
```

A composite message containing multiple owning capabilities is committed as one source-semantic operation.

Runtime preparation may have internal substeps, but failure must roll back all temporary destination/transport state before returning the error.

---

## 33. Meaning of successful `Send`

`Send()` success means that the complete message transfer has committed to the canonical typed IPC transport.

It does not mean:

```text
the receiving application called Receive
the receiving application processed the message
an application-level request succeeded
a reply exists
```

Typed IPC is not implicit RPC.

After a consuming `Send()` has committed, later peer failure does not restore source ownership.

---

## 34. Receive semantics

`Receive()` returns:

```text
Ok(Some(message))
    one complete semantic T was received and is now owned locally

Ok(None)
    the typed message stream ended cleanly and no committed messages remain

Err(error)
    typed IPC transport or materialization failed
```

A temporarily empty but open typed transport waits.

`None` never means temporary emptiness, cancellation, timeout, or abnormal peer failure.

---

## 35. Clean send-side completion

Clean receive-side end-of-stream occurs only after:

```text
all sender capabilities for the logical transport ended cleanly
and
all previously committed messages were delivered or otherwise canonically resolved
```

Queued committed messages are observed before `Ok(None)`.

After a receiver reaches clean terminal state, later `Receive()` calls continue to return `Ok(None)` while the local receiver remains open.

---

## 36. Abnormal sender disappearance

Abrupt process death that bypasses normal endpoint destruction is not clean sender closure.

If one sender disappears abnormally while other senders remain usable, the logical stream may continue.

When the sender side finally becomes terminal and queued committed messages are exhausted:

- if every sender capability ended cleanly, the receiver observes `Ok(None)`;
- if at least one sender capability disappeared abnormally, the receiver observes the canonical terminal transport error, normally `IPCError.NativeFailure` unless a more specific portable error applies.

The abnormal terminal result remains stable on later `Receive()` calls. It must not later transform into clean `None`.

If the underlying transport becomes unusable earlier, the receiver may receive the terminal transport error immediately.

---

## 37. Receiver closure and disappearance

`IPCReceiver[T].Close()` cleanly releases that local receiver capability.

When at least one receiver capability remains alive, other receivers may continue consuming messages.

When no receiver capabilities remain, a new send cannot commit and returns:

```sec
Err(IPCError.PeerClosed)
```

Queued messages that can no longer be delivered are destroyed exactly once by the runtime. Any embedded transferred resources are released according to their canonical destruction contracts.

---

## 38. Typed IPC duplication and MPMC behavior

`IPCSender[T].Duplicate()` creates another sender capability to the same logical transport.

`IPCReceiver[T].Duplicate()` creates another receiver capability to the same logical queue.

Typed IPC therefore supports multi-producer/multi-consumer capability arrangements.

Each committed message is delivered to exactly one receiving capability.

Duplicated receivers do not broadcast a message.

Per-sender successful commit order is preserved.

Ordering between concurrently committing different sender capabilities is not portable-guaranteed beyond the actual commit ordering established by the runtime.

Which of several waiting receivers receives a message is not portable-guaranteed.

---

## 39. Typed endpoint process transfer

`IPCSender[T]` and `IPCReceiver[T]` are process-transferable capabilities when the selected route provides the required adapter.

Moving an endpoint transfers one capability to the same logical typed transport. It does not serialize a queue object, create a new independent transport, or copy the peer capability.

Example:

```sec
let worker := try spawn process Work(<-receiver)
```

On successful process-start commit, the parent binding is unavailable and the child owns the transferred receiver capability.

On failed pre-commit process startup, the parent binding remains available according to the canonical process transactional-transfer rule.

Endpoint duplication and endpoint process transfer are different operations: duplication creates another owner to the same logical side; move transfer changes which process owns one existing capability.

---

## 40. Typed IPC buffering and backpressure

Typed message buffering is implementation-defined and bounded.

A target/runtime may use a zero-capacity rendezvous realization.

There is no hidden unbounded message queue.

`Send()` may wait until the transport can commit the complete message.

A program must not assume that `Send()` merely appends to a buffer.

This matters to deadlock analysis: two peers that both perform a waiting send before either receives can deadlock on a zero-capacity realization.

---

## 41. Typed IPC close

`Close()` is infallible and idempotent for typed endpoints.

For a sender, local close means the capability will commit no further messages.

For a receiver, local close means the capability will receive no further messages.

The local endpoint becomes closed regardless of whether a peer can still observe a native clean-close notification. Peer-side transport failure is represented by the peer's own canonical operation result.

`IsClosed` reports local endpoint state only.

---

## 42. Typed IPC `select`

`Send()` and `Receive()` are primitive selectable operations.

A non-selected `Send()`:

```text
commits no message
transfers no ownership
creates no destination capability
```

A non-selected `Receive()`:

```text
removes no message
materializes no T
changes no source-visible queue state
```

For a consuming send:

```sec
select {
    sender.Send(<-message) => {
        // ownership commits only if this selected operation returns Ok()
    }

    after 1<s> => {
        // message remains available
    }
}
```

If the send branch is selected but the send returns `Err`, source ownership remains available.

---

# Part III — Shared memory

## 43. Shared-memory model

Sec distinguishes the shared backing capability from each process-local mapping.

The canonical types are:

```text
SharedMemory
SharedReadMapping
SharedWriteMapping
```

`SharedMemory` identifies authority to one logical shared backing object.

A mapping represents one process's mapping of that backing object into its own address space.

Different processes may map the same backing bytes at different virtual addresses.

The virtual address is not portable shared-memory identity.

---

## 44. Exact `SharedMemoryError`

```sec
enum SharedMemoryError error {
    Closed
    InvalidSize
    PermissionDenied
    OutOfMemory
    ResourceLimit
    MappingFailed
    DuplicationFailed
    NativeFailure
}
```

### `Closed`

The local shared-memory capability or mapping is closed.

### `InvalidSize`

The requested logical size is invalid. `CreateSharedMemory(0)` is invalid in Sec 0.1.

### `PermissionDenied`

The target security/access policy denied the requested operation.

### `OutOfMemory`

Required backing, address-space, or runtime state could not be allocated.

### `ResourceLimit`

A native/runtime object, mapping, handle, or related resource quota was reached.

### `MappingFailed`

The capability is valid but the requested process-local mapping could not be established and no more specific portable error applies.

### `DuplicationFailed`

The local capability could not be duplicated despite duplication being semantically and target-supported.

### `NativeFailure`

A native failure occurred and no more specific portable variant applies.

---

## 45. Exact `SharedMemory` surface

```sec
@noCopy
type SharedMemory

fn CreateSharedMemory(size: uint) Result[SharedMemory, SharedMemoryError]

impl SharedMemory {
    property Size: uint {
        get
    }

    property IsClosed: bool {
        get
    }

    fn MapRead() Result[SharedReadMapping, SharedMemoryError]
    fn MapWrite() Result[SharedWriteMapping, SharedMemoryError]
    fn Duplicate() Result[SharedMemory, SharedMemoryError]
    fn Close() Result[void, SharedMemoryError]

    free {
        // Best-effort release of this local backing capability.
    }
}
```

---

## 46. Exact mapping surfaces

```sec
@noCopy
type SharedReadMapping

impl SharedReadMapping {
    property Size: uint {
        get
    }

    property IsClosed: bool {
        get
    }

    unsafe fn View() ref byte[]
    fn Close() Result[void, SharedMemoryError]

    free {
        // Best-effort unmap if still mapped.
    }
}
```

```sec
@noCopy
type SharedWriteMapping

impl SharedWriteMapping {
    property Size: uint {
        get
    }

    property IsClosed: bool {
        get
    }

    unsafe fn View() ref mut byte[]
    fn Close() Result[void, SharedMemoryError]

    free {
        // Best-effort unmap if still mapped.
    }
}
```

---

## 47. Shared-memory creation

`CreateSharedMemory(size)` requires:

```text
size > 0
```

The newly created logical region is zero-initialized before any mapping may observe it.

`SharedMemory.Size` is the requested logical size.

A target may allocate/map larger native page-rounded storage internally, but bytes outside the logical `Size` are not portable accessible storage through the Sec mapping contract.

Creation is all-or-nothing.

---

## 48. Shared-memory duplication

`SharedMemory.Duplicate()` creates a new independently owned capability to the same logical backing region.

It is not a byte copy.

Example:

```sec
let childMemory := try memory.Duplicate()
```

After success, both owners refer to the same backing identity.

Duplication failure leaves the original capability unchanged and releases all temporary destination/native state.

---

## 49. Shared-memory process transfer

`SharedMemory` is process-transferable when the selected target route provides the required adapter.

Transferring `SharedMemory` transfers authority to the same logical backing region.

It does not transfer a process-local virtual address.

The destination maps the backing region independently in its own address space.

---

## 50. Mappings are process-local

`SharedReadMapping` and `SharedWriteMapping` are not process-transferable.

This is invalid:

```sec
spawn process Work(<-mapping)
```

A process transfers or duplicates the `SharedMemory` capability and calls `MapRead()` or `MapWrite()` in the destination process.

---

## 51. Mapping lifetime

A mapping owns enough backing-lifetime authority to keep its mapped bytes valid independently of the particular `SharedMemory` value that created it.

Therefore a mapping may remain valid after the originating local `SharedMemory` capability is closed, provided the mapping itself remains open.

The logical backing object remains alive while any owning capability or mapping still requires it.

This is native resource lifetime, not garbage collection and not an ordinary Sec borrow from `SharedMemory`.

`SharedMemory.Close()`, `SharedReadMapping.Close()`, and `SharedWriteMapping.Close()` are idempotent. Re-closing an already closed local capability/mapping returns success. Other operations on a locally closed value return `SharedMemoryError.Closed` where the state is not already rejected statically.

---

## 52. Raw mapping views are `unsafe`

`View()` is `unsafe` because ordinary process-local borrow checking cannot prove exclusivity against mappings in other processes.

A writable mapping may have another process mapping the same bytes concurrently.

Entering `unsafe` does not make a process-local reference valid in another process and does not disable ownership, lifetime, or data-race analysis.

The returned reference must not outlive the mapping.

Closing a mapping while a view borrow is live is invalid according to ordinary borrow/lifetime rules.

---

## 53. Shared memory gives no implicit synchronization

Shared backing storage alone does not provide:

```text
mutual exclusion
atomicity
release/acquire ordering
race prevention
application invariants
```

Programs coordinate shared mappings through process-aware synchronization, process-shared atomics, or another explicitly defined shared-memory protocol.

Process join does not generally publish arbitrary shared memory.

---

# Part IV — Process-shared synchronization

## 54. Synchronization family

Sec 0.1 provides two fundamental process-shared synchronization capabilities:

```sec
IPCMutex
IPCSemaphore
```

They are distinct from ordinary in-process synchronization types.

The source abstraction does not require the native synchronization object itself to reside inside `SharedMemory`.

A target may realize the logical synchronization identity through kernel objects, shared storage, or another mechanism that preserves the full contract.

---

## 55. Exact `IPCSyncError`

```sec
enum IPCSyncError error {
    Closed
    InvalidCount
    LimitExceeded
    OwnerDied
    PermissionDenied
    OutOfMemory
    ResourceLimit
    NativeFailure
}
```

### `Closed`

The local synchronization capability is closed.

### `InvalidCount`

Semaphore construction used an invalid initial/maximum count.

### `LimitExceeded`

A semaphore `Release()` would exceed `Maximum`.

### `OwnerDied`

The logical `IPCMutex` is permanently poisoned because its previous owning execution/process died before normal guard release.

### `PermissionDenied`

Native policy denied the requested synchronization operation.

### `OutOfMemory`

Required runtime/native synchronization state could not be allocated.

### `ResourceLimit`

The applicable native/runtime synchronization quota was reached.

### `NativeFailure`

A native synchronization failure occurred and no more specific portable variant applies.

---

## 56. Exact `IPCMutex` surface

```sec
@noCopy
type IPCMutex

@noCopy
type IPCMutexGuard

fn CreateIPCMutex() Result[IPCMutex, IPCSyncError]

impl IPCMutex {
    property IsClosed: bool {
        get
    }

    fn Lock() Result[IPCMutexGuard, IPCSyncError]
    fn TryLock() Result[Option[IPCMutexGuard], IPCSyncError]
    fn Duplicate() Result[IPCMutex, IPCSyncError]
    fn Close() Result[void, IPCSyncError]

    free {
        // Best-effort release of this local mutex capability.
    }
}

impl IPCMutexGuard {
    free {
        // Releases the acquired logical IPC mutex.
    }
}
```

---

`IPCMutex` is process-transferable when the selected route provides the required adapter. `IPCMutexGuard` is never process-transferable.

---

## 57. IPC mutex semantics

`IPCMutex` is non-recursive in canonical Sec semantics.

A successful `Lock()` returns exactly one live `IPCMutexGuard` representing ownership of the logical critical section.

The guard is the unlock capability. There is no separate public `Unlock()` method in Sec 0.1.

Destroying the guard releases the mutex.

`TryLock()` returns:

```text
Ok(Some(guard))
    acquired immediately

Ok(None)
    not immediately available

Err(error)
    actual synchronization failure
```

`TryLock()` does not wait.

---

## 58. IPC mutex guard affinity

`IPCMutexGuard` is:

```text
not ProcessTransferable
not ThreadTransferable
not TaskTransferable
```

It belongs to the execution that acquired the mutex.

A live `IPCMutexGuard` may not cross a potentially waiting/suspending boundary in Sec 0.1.

This includes, where applicable:

```text
await
join
waiting select
Pipe.Read/Write waits
IPCSender.Send waits
IPCReceiver.Receive waits
IPCMutex.Lock waits
IPCSemaphore.Acquire waits
SharedValue.Lock waits
Task.Yield
```

The compiler uses actual liveness/destruction facts. A guard destroyed before the wait creates no violation.

---

## 59. Mutex owner death and poisoning

If the execution/process that owns an `IPCMutex` terminates without normal guard release, the logical mutex becomes permanently poisoned.

Later `Lock()` and `TryLock()` return:

```sec
Err(IPCSyncError.OwnerDied)
```

No new guard is produced.

Sec 0.1 provides no automatic recovery API for a poisoned IPC mutex.

The protected shared state may be logically inconsistent, so transparent unlock/reuse is forbidden.

---

## 60. Mutex duplication and identity

`IPCMutex.Duplicate()` creates a new capability to the same logical mutex identity.

It does not create an independent mutex.

All duplicates participate in the same exclusion, poisoning, and release/acquire synchronization contract.

`IPCMutex.Close()` is idempotent and releases only the local mutex capability. It does not destroy the logical mutex while another capability, waiter, or valid guard keeps required state alive. Closing a mutex capability while a guard obtained through that local owner remains live is invalid; ordinary lifetime/borrow analysis should reject the operation when provable. Other operations on a locally closed mutex return `IPCSyncError.Closed` where not rejected statically.

---

## 61. Exact `IPCSemaphore` surface

```sec
@noCopy
type IPCSemaphore

fn CreateIPCSemaphore(
    Initial: uint,
    Maximum: uint
) Result[IPCSemaphore, IPCSyncError]

impl IPCSemaphore {
    property Maximum: uint {
        get
    }

    property IsClosed: bool {
        get
    }

    fn Acquire() Result[void, IPCSyncError]
    fn TryAcquire() Result[bool, IPCSyncError]
    fn Release() Result[void, IPCSyncError]
    fn Duplicate() Result[IPCSemaphore, IPCSyncError]
    fn Close() Result[void, IPCSyncError]

    free {
        // Best-effort release of this local semaphore capability.
    }
}
```

---

`IPCSemaphore` is process-transferable when the selected route provides the required adapter. Duplication and move transfer preserve the same logical semaphore identity.

---

## 62. Semaphore construction

Valid construction requires:

```text
Maximum > 0
Initial <= Maximum
```

Invalid construction returns:

```sec
Err(IPCSyncError.InvalidCount)
```

The semaphore exposes no portable current-count property.

---

## 63. Semaphore operations

`Acquire()` atomically consumes one permit when available. If no permit is available, it waits.

`TryAcquire()` returns:

```text
true
    one permit was acquired immediately

false
    no permit was immediately available
```

`Release()` atomically adds one permit.

If the count is already `Maximum`, `Release()` returns:

```sec
Err(IPCSyncError.LimitExceeded)
```

and leaves the count unchanged.

Semaphore permits have no ownership identity tied to the process that acquired them.

If a process acquires a permit and dies, the permit is not restored automatically.

`IPCSemaphore.Close()` is idempotent and releases only the local capability. It does not reset the logical count or destroy a semaphore still referenced by another capability/waiter. Other operations on a locally closed semaphore return `IPCSyncError.Closed` where not rejected statically.

---

## 64. Synchronization memory ordering

Normal mutex guard release followed by a later successful acquisition of the same logical mutex establishes process-shared release/acquire synchronization.

`IPCSemaphore.Release()` followed by the successful `Acquire()` that consumes the published permit establishes corresponding release/acquire synchronization for the shared-memory protocol coordinated by that permit.

These synchronization edges apply to explicitly shared storage according to the canonical concurrency memory model.

They do not make unrelated process-local storage shared.

---

## 65. Synchronization `select`

`IPCMutex.Lock()` and `IPCSemaphore.Acquire()` are primitive selectable operations.

A non-selected mutex branch does not acquire the mutex and produces no guard.

A non-selected semaphore branch does not decrement the permit count.

`TryLock()` and `TryAcquire()` are nonblocking and are not primitive selectable operations. `IPCSemaphore.Release()` is likewise not selectable.

---

## 66. Process-shared synchronization scope

Sec 0.1 does not define core:

```text
IPCEvent
IPCConditionVariable
IPCRWLock
```

`IPCMutex` and `IPCSemaphore` are the fundamental process-shared blocking synchronization capabilities for this revision.

Additional higher-level synchronization may be added by later rulebooks or libraries without changing these contracts.

---

# Part V — Safe typed shared values

## 67. `SharedValue[T]` purpose

Raw `SharedMemory` intentionally exposes only bytes through `unsafe` mappings.

`SharedValue[T]` is the safe typed abstraction that binds:

```text
one shared backing identity
one synchronization identity
one canonical shared representation of T
one shared-layout identity
```

The components cannot be separated through the safe `SharedValue[T]` API.

`T` must satisfy `SharedStorable(T)`.

---

## 68. Exact `IPCSharedError`

```sec
enum IPCSharedError error {
    Closed
    LayoutMismatch
    OwnerDied
    PermissionDenied
    OutOfMemory
    ResourceLimit
    MappingFailed
    DuplicationFailed
    NativeFailure
}
```

### `Closed`

The local `SharedValue[T]` capability is closed.

### `LayoutMismatch`

A destination/runtime capability establishment could not prove compatibility with the exact canonical shared-layout identity required for `T`.

Compile-time-known type mismatches must be diagnosed earlier rather than converted to this runtime error.

### `OwnerDied`

The synchronization identity protecting the shared value is permanently poisoned.

### Remaining variants

`PermissionDenied`, `OutOfMemory`, `ResourceLimit`, `MappingFailed`, `DuplicationFailed`, and `NativeFailure` have the same portable categories described for the underlying shared-memory/synchronization operations.

---

## 69. Exact `SharedValue[T]` surface

```sec
@noCopy
type SharedValue[T]

@noCopy
type SharedValueGuard[T]

fn CreateSharedValue[T](
    initial: T
) Result[SharedValue[T], IPCSharedError]

impl SharedValue[T] {
    property IsClosed: bool {
        get
    }

    fn Lock() Result[SharedValueGuard[T], IPCSharedError]
    fn TryLock() Result[Option[SharedValueGuard[T]], IPCSharedError]
    fn Duplicate() Result[SharedValue[T], IPCSharedError]
    fn Close() Result[void, IPCSharedError]

    free {
        // Best-effort release of this local SharedValue capability.
    }
}

impl SharedValueGuard[T] {
    fn Read() T
    fn Write(value: T) void

    free {
        // Publishes the required release ordering and releases the guard.
    }
}
```

---

## 70. Shared value initialization

`CreateSharedValue[T](initial)` is valid only when `SharedStorable(T)` is proven.

The operation establishes:

```text
backing storage
canonical shared representation
initial value
synchronization identity
shared-layout identity
```

atomically from source semantics.

No capability is published before the complete initialized state exists.

---

## 71. No safe shared reference escapes

`SharedValueGuard[T]` does not return `ref T` or `ref mut T`.

`Read()` materializes a process-local snapshot of `T`.

`Write(value)` copies the semantic value into the canonical shared representation while the guard owns the logical critical section.

A local value returned from `Read()` has no alias relationship to shared storage.

Mutating the local snapshot does not alter shared storage until `Write()` is called.

This keeps safe typed shared access independent of process-local virtual addresses.

---

## 72. Shared value guard affinity

`SharedValueGuard[T]` is execution-local and is not transferable to another task, thread, or process.

It may not remain live across a potentially waiting/suspending operation under the same rule as `IPCMutexGuard`.

---

## 73. Shared value duplication

`SharedValue[T].Duplicate()` creates another capability to the same logical:

```text
shared value identity
backing identity
synchronization identity
layout identity
```

It does not copy the stored `T` into a second independent shared value.

If several native capabilities must be duplicated internally, the operation is transactional. Partial duplication is rolled back before an error becomes source-visible.

`SharedValue[T].Close()` is idempotent and releases only the local composite capability. It does not invalidate another duplicate. Closing the local capability while a `SharedValueGuard[T]` derived from it remains live is invalid where the compiler can prove the dependency. Other operations on a locally closed value return `IPCSharedError.Closed` where not rejected statically.

---

## 74. Shared value process transfer

`SharedValue[T]` is process-transferable when the selected target can transfer its complete composite capability.

The source code transfers the `SharedValue[T]` as one logical capability. It does not separately move the hidden backing and synchronization components.

---

## 75. Shared-layout identity

Compiler/runtime associates a shared-layout identity with `T` sufficient to establish compatibility of:

```text
nominal type identity
generic specialization
canonical shared representation
relevant representation revision
```

Ordinary in-process ABI padding or virtual address placement is not itself the shared-layout identity.

An incompatible independently established peer cannot reinterpret the backing bytes as another `T` through the safe `SharedValue[T]` surface.

---

## 76. Shared value poisoning

If an execution/process dies while owning `SharedValueGuard[T]`, the logical shared value becomes permanently poisoned according to the `IPCMutex` owner-death model.

Later `Lock()` and `TryLock()` return:

```sec
Err(IPCSharedError.OwnerDied)
```

Sec 0.1 defines no automatic safe recovery for a poisoned `SharedValue[T]`.

---

# Part VI — Process-shared atomics

## 77. IPC atomic model

`IPCAtomic[T]` is one process-shared atomic storage identity.

It is a resource capability, not an inline field that may be embedded arbitrarily into external shared-memory layouts.

Canonical declaration:

```sec
@noCopy
type IPCAtomic[T]
```

The selected target must provide the complete process-shared lock-free atomic contract for the concrete `T`.

There is no hidden mutex fallback.

---

## 78. Ordinary atomic authority

`rules/concurrency/atomics.md` remains the canonical owner of:

```sec
MemoryOrder
CompareExchangeResult[T]
```

and the common operation naming/order-validity model.

The exact canonical result declaration consumed by IPC is:

```sec
enum CompareExchangeResult[T] {
    Exchanged
    NotExchanged(T)
}
```

IPC does not introduce a second compare-exchange result type.

The canonical read-modify-write replacement operation is named `Swap`, matching the ordinary atomic owner.

---

## 79. Exact `IPCAtomicError`

```sec
enum IPCAtomicError error {
    PermissionDenied
    OutOfMemory
    ResourceLimit
    DuplicationFailed
    NativeFailure
}
```

These errors apply to creation and duplication.

Ordinary atomic operations on a successfully established `IPCAtomic[T]` are infallible in their source signatures.

There is no `Closed` or `OwnerDied` state and no public `Close()`.

---

## 80. Exact `IPCAtomic[T]` surface

```sec
fn CreateIPCAtomic[T](
    initial: T
) Result[IPCAtomic[T], IPCAtomicError]

impl IPCAtomic[T] {
    fn Load() T
    fn Load(order: MemoryOrder) T

    fn Store(value: T) void
    fn Store(value: T, order: MemoryOrder) void

    fn Swap(value: T) T
    fn Swap(value: T, order: MemoryOrder) T

    fn CompareExchange(
        expected: T,
        desired: T
    ) CompareExchangeResult[T]

    fn CompareExchange(
        expected: T,
        desired: T,
        successOrder: MemoryOrder,
        failureOrder: MemoryOrder
    ) CompareExchangeResult[T]

    fn Duplicate() Result[IPCAtomic[T], IPCAtomicError]

    free {
        // Releases this local IPC atomic capability.
    }
}
```

Sec 0.1 defines no IPC-specific `FetchAdd`, `FetchSub`, bitwise fetch, `Wait`, `Notify`, or fence surface in this rulebook.

Future atomic convenience operations must be harmonized with the ordinary atomics owner rather than invented separately for IPC.

---

## 81. IPC atomic lock-free requirement

A canonical `IPCAtomic[T]` exists only when the selected target/runtime profile can provide lock-free process-shared operations for the complete required surface.

The target capability must cover:

```text
Load
Store
Swap
strong CompareExchange
required MemoryOrder semantics
process-shared synchronization scope
atomicity without tearing
```

Unsupported type/width/profile combinations are compile-time errors.

A backend must not silently replace `IPCAtomic[T]` with `IPCMutex`.

Atomic operations must not allocate or block after a valid capability has been established.

---

## 82. IPC atomic memory order

IPC atomics reuse the exact `MemoryOrder` enum:

```sec
enum MemoryOrder {
    Relaxed
    Acquire
    Release
    AcqRel
    SeqCst
}
```

Omitted operation orders use `MemoryOrder.SeqCst`.

`Load` accepts the canonical load order set.

`Store` accepts the canonical store order set.

`Swap` accepts all canonical RMW orders.

`CompareExchange` success and failure orders follow the exact validity rules owned by `rules/concurrency/atomics.md`.

Invalid operation/order combinations are compile-time semantic errors.

For `IPCAtomic[T]`, explicit `MemoryOrder` arguments must be compile-time-known so that target capability and process-shared synchronization semantics are statically validated.

---

## 83. IPC atomic synchronization scope

Atomicity and ordering apply across all executions that access the same logical `IPCAtomic[T]` identity through valid capabilities.

The backend must not lower IPC atomics to a synchronization scope narrower than the process-shared domain required by the logical capability.

A release operation followed by an observing acquire operation on the same synchronization relation can publish/observe explicitly shared memory according to the concurrency memory model.

`Relaxed` provides atomic-cell guarantees but does not add acquire/release publication for unrelated shared state.

`SeqCst` participates in the canonical sequential-consistency ordering defined by the ordinary memory model.

---

`IPCAtomic[T]` is process-transferable when the selected route provides the required adapter. Move transfer preserves the same logical atomic storage identity.

---

## 84. IPC atomic process death

Process death does not poison an `IPCAtomic[T]`.

Each atomic operation has one linearization point and is either committed or not committed.

The latest committed value remains part of the logical atomic storage while another capability keeps that storage alive.

---

## 85. IPC atomic duplication

`Duplicate()` creates another capability to the same logical atomic storage and synchronization identity.

It does not create a second cell containing a copied value.

Duplication is transactional and leaves the original capability unchanged on failure.

---

# Part VII — Capability and resource transfer

## 86. Type-preserving capability transfer

Sec defines no portable untyped `IPCHandle` owner.

A transferable resource retains its concrete semantic type across the boundary:

```text
File          -> File
PipeReader    -> PipeReader
PipeWriter    -> PipeWriter
SharedMemory  -> SharedMemory
IPCMutex      -> IPCMutex
IPCSemaphore  -> IPCSemaphore
IPCAtomic[T]  -> IPCAtomic[T]
SharedValue[T] -> SharedValue[T]
```

The destination native handle, descriptor, port, mapping token, or equivalent may differ from the source representation.

Copying a native integer handle value never establishes Sec ownership in another process.

---

## 87. Move transfer versus duplication

Source semantics distinguish exactly:

```text
move transfer
explicit duplication
```

A move transfer changes ownership when the boundary operation commits.

Duplication creates another owner to the same logical underlying resource where the type defines such semantics.

Native inheritance is not a third Sec ownership operation. A backend may use inheritance internally when it correctly realizes move transfer or explicit duplication.

---

## 88. Capability authority

Transfer and duplication may preserve or reduce authority but must never silently increase it.

A read-only or otherwise restricted source capability must not become a more powerful destination capability merely because the native duplication mechanism permits broader rights.

Only the capabilities explicitly required by the operation may become available to the destination.

Bulk inheritance of unrelated parent resources is forbidden by the portable IPC contract.

---

## 89. Resource identity

Sec owner identity and logical resource identity are separate.

After `Duplicate()`, two Sec owners may refer to one underlying logical resource.

Duplication of an open `File` is not reopen-by-path.

A duplicated file may share underlying open-resource state such as file position where that behavior is part of the target/resource contract.

A duplicated `PipeWriter` remains part of the same logical pipe write side.

---

## 90. Composite capability transfer

A typed IPC value may contain transferable resource capabilities.

Example:

```sec
type WorkerRequest struct {
    Input: File,
    Output: PipeWriter,
}
```

A consuming send:

```sec
try sender.Send(<-request)
```

may combine:

```text
ordinary value transfer
compiler-generated materialization
File capability transfer
PipeWriter capability transfer
```

The complete `WorkerRequest` commits atomically from source semantics.

Failure before commit leaves the complete source value owned locally and rolls back every temporary destination capability.

---

## 91. Route-specific target adapters

Semantic `ProcessTransferable` eligibility does not mean that every target can realize every transfer route.

CompilationPlan metadata must be able to distinguish at least:

```text
CapabilityDuplicate(T)
ProcessStartupTransfer(T)
IPCTypedTransfer(T)
ProcessStandardIOResourceBinding(T, role)
```

A resource may be valid for one route but not another.

Statically missing route support is a compile-time error.

---

# Part VIII — Command standard-I/O resource binding

## 92. Authority boundary

`rules/concurrency/processes.md` owns `Command` and process lifecycle.

This IPC rulebook owns the general capability-transfer semantics that make existing resource binding valid.

The process rulebook must expose the source API defined by the IPC correction package.

---

## 93. Portable direct resource scope

Sec 0.1 portable direct `Command` standard-I/O binding supports:

```text
stdin:
    File
    PipeReader

stdout:
    File
    PipeWriter

stderr:
    File
    PipeWriter
```

Sockets and arbitrary native handles are not part of the portable Sec 0.1 direct-binding surface.

Target-specific unsafe integration may expose additional resources under separate target contracts.

---

## 94. Command mode extension

The canonical process enums must include:

```sec
enum CommandInputMode {
    Inherit
    Closed
    Pipe
    Resource
}

enum CommandOutputMode {
    Inherit
    Discard
    Pipe
    Resource
}
```

`Resource` means that the caller supplied an already existing owned I/O capability.

It is distinct from `Pipe`, where `Command.Start()` creates a new anonymous pipe and later exposes the parent endpoint through `Take*Pipe()`.

---

## 95. Direct binding overloads

The process-owned `Command` API must include:

```sec
fn SetStdin(file: File) Result[void, CommandConfigurationError]
fn SetStdin(pipe: PipeReader) Result[void, CommandConfigurationError]

fn SetStdout(file: File) Result[void, CommandConfigurationError]
fn SetStdout(pipe: PipeWriter) Result[void, CommandConfigurationError]

fn SetStderr(file: File) Result[void, CommandConfigurationError]
fn SetStderr(pipe: PipeWriter) Result[void, CommandConfigurationError]
```

Existing named move-only resources are passed with explicit `<-`:

```sec
try command.SetStdout(<-logFile)
```

---

## 96. Direct binding ownership

The configuration setter is transactional.

On success:

```text
Command owns the supplied resource capability
source consumed binding becomes unavailable
mode becomes Resource
```

On configuration error:

```text
previous Command configuration remains unchanged
source capability remains owned by the caller
```

A caller that must retain access duplicates the resource first:

```sec
let childLog := try logFile.Duplicate()
try command.SetStdout(<-childLog)
```

---

## 97. `InvalidIOResource`

The process-owned error enum must add:

```sec
InvalidIOResource
```

to `CommandConfigurationError`.

It represents a supplied resource that cannot satisfy the requested stream direction or portable standard-I/O contract.

Example: a write-only `File` cannot be used as child stdin.

On `InvalidIOResource`, ownership transfer does not commit.

---

## 98. Resource-mode startup

`Command` owns the configured resource until `Start()` commit.

On successful start:

```text
child standard stream is established from the resource capability
Command releases any no-longer-required parent-side configuration owner
```

The process implementation must not keep a hidden extra pipe/file capability that changes EOF or lifetime behavior.

On failed `Start()`:

```text
Command returns to Created
configured resource remains owned by Command
retry remains possible
all temporary child/native duplicates are rolled back
```

Actual native launch-time binding failure uses the existing `CommandStartError.IOSetupFailed` contract.

---

## 99. `Take*Pipe` in Resource mode

`TakeStdinPipe()`, `TakeStdoutPipe()`, and `TakeStderrPipe()` return `None` for streams configured with `Resource` mode because no new parent-side pipe endpoint was created by `Command`.

---

# Part IX — Rendezvous, names, and authority

## 100. Core IPC is capability-based

Core IPC resources are established through explicit creation, duplication, process-start transfer, typed IPC transfer, or another authorized mechanism.

Core Sec 0.1 defines no global named IPC registry.

It defines no generic:

```text
CreateNamedIPC
OpenIPC
IPCAddress
IPCListener
```

surface.

---

## 101. Rendezvous is separate

Two independently started processes may require a local transport, service manager, named pipe, Unix-domain path, platform service, or another bootstrap mechanism to find each other.

The abstraction that owns that rendezvous mechanism also owns:

```text
namespace
permissions
authentication
cleanup
name collisions
scope
```

Once authority is established, that mechanism may transfer or expose canonical IPC capabilities where the selected target supports it.

---

## 102. Endpoints are capabilities, not addresses

`IPCSender[T]`, `IPCReceiver[T]`, pipe endpoints, `SharedMemory`, synchronization objects, and IPC atomics are established capabilities.

They are not service names or connectable addresses.

---

## 103. No ambient IPC authority

Knowing a:

```text
ProcessID
native PID
native handle number
resource ID
string name
```

does not itself grant IPC capability ownership.

`ProcessID` is not a master key for another process's IPC resources.

Core IPC does not provide enumeration or arbitrary attachment to all IPC resources on the machine.

---

# Part X — Typed transfer representation and compatibility

## 104. Typed endpoints are monomorphic

An `IPCSender[T]` and its logical `IPCReceiver[T]` are typed to exactly `T` for their lifetime.

Core Sec 0.1 defines no dynamic `any` typed IPC endpoint.

Generic specialization arguments are part of typed endpoint identity.

---

## 105. Hidden transfer identity

Compiler/runtime associates typed IPC endpoints with internal transfer identity sufficient to represent:

```text
nominal type identity
generic specialization
canonical ProcessTransferable plan
relevant schema/transfer revision
capability-bearing component plan
```

This identity is not a source-visible `IPCTypeID` API.

---

## 106. Nominal compatibility

Typed IPC compatibility is nominal, not structural.

Two different nominal types remain different even if their field layouts happen to match.

Two incompatible revisions of one nominal application protocol are not automatically migrated by core IPC.

If application protocol evolution is required, it is expressed explicitly by the application or by a higher-level protocol/serialization layer.

---

## 107. Transfer representation is semantic

Typed IPC does not memcpy ordinary process memory as the portable transfer contract.

The compiler may generate transfer/materialization routines for semantic values whose process-local storage differs between processes.

For example, a `string` value that is semantically process-transferable is materialized in destination-owned storage; the destination does not receive the source process heap pointer.

Capability-bearing fields use the capability transfer rules in this rulebook rather than serialized native handle numbers.

---

## 108. No public stable wire ABI

Compiler/runtime typed IPC representation is not a public stable file/network/cross-language wire format.

Programs requiring:

```text
persistent serialized data
cross-language interchange
network protocol stability
independent protocol versioning
```

use an explicit serialization/protocol layer.

---

## 109. Compatibility establishment

When compiler/runtime can prove both endpoints use the same transfer contract, redundant runtime checking may be eliminated.

When independently started processes establish a typed capability through an external rendezvous path, the establishment mechanism must verify compatible transfer identity before exposing the typed endpoint.

A mismatch must never reinterpret incompatible bytes as `T`.

---

## 110. Target-dependent semantic widths

Ordinary ABI layout, virtual addresses, native padding, and endianness are not by themselves typed IPC incompatibility.

Compiler/runtime may normalize the semantic representation.

Target-sized language types such as `int` and `uint` have target-dependent semantic width. Different resolved semantic widths are compatibility-relevant and may make an independently established typed IPC contract incompatible.

Fixed-width semantic values such as `uint64` may remain compatible across different ordinary ABIs when the transfer plan can represent the same semantic value.

---

## 111. Robust materialization

Incoming transfer state is validated before a source-visible `T` is committed.

Validation includes, where relevant:

```text
framing
lengths and size arithmetic
variant discriminants
allocation bounds
capability counts
capability type identity
schema/transfer identity
integer bounds required by representation
```

Malformed or malicious peer state must not create memory unsafety.

Partial materialization state is destroyed on failure.

---

## 112. Compiler-generated transfer plans

Transferability is recursive through the semantic value graph.

Compiler may monomorphize concrete transfer, validation, rollback, and materialization routines for each used `T`.

Runtime reflection is not required by Sec 0.1.

---

# Part XI — Lifecycle, waiting, and cancellation

## 113. Process death releases local capabilities

Process death releases resources owned solely by that process according to native/runtime cleanup capability, but it does not destroy a logical IPC resource still owned elsewhere.

Examples:

- shared memory remains alive while another capability/mapping remains;
- IPC atomics retain the latest committed value while another capability exists;
- semaphore permits already consumed by the dead process are not restored;
- a mutex held by the dead execution becomes poisoned;
- typed endpoint abnormal disappearance is distinguished from clean closure as defined earlier.

---

## 114. Cancellation before commit

Cancellation follows the general Sec cancellation model and is not represented by additional `Cancelled` variants in IPC error enums.

A waiting IPC operation may be interrupted by cancellation only before its semantic commit.

Before commit, cancellation leaves the operation's semantic state unchanged.

Examples:

```text
Send: message ownership remains local
Receive: no message is dequeued
Mutex Lock: no mutex is acquired
Semaphore Acquire: no permit is consumed
```

---

## 115. Commit wins

After an IPC operation has committed its semantic effect, cancellation, timeout, or competing readiness cannot revoke that effect.

The operation completes to its canonical source-visible result.

This rule prevents states such as:

```text
message removed but result discarded
mutex acquired but guard lost
permit consumed but call reported as uncommitted
ownership moved but source treated as available
```

---

## 116. Lifecycle transitions wake waiters

A lifecycle transition that makes a waiting result decidable makes the affected operation ready.

Examples:

```text
last pipe writer gone -> Read may return EOF
last pipe reader gone -> Write may return BrokenPipe
last clean IPC sender gone -> Receive may eventually return None after queued messages
all IPC receivers gone -> Send may return PeerClosed
IPCMutex poisoned -> Lock returns OwnerDied
```

The transition does not secretly perform unrelated user work.

---

## 117. Cancellation does not invent resource actions

Cancellation does not automatically:

```text
close unrelated IPC endpoints
terminate a child process
detach a process
destroy shared memory
poison an unlocked mutex
reset a semaphore
```

Normal control flow and deterministic destruction may later release resources according to their ordinary contracts.

---

# Part XII — Target capabilities

## 118. No universal IPC support flag

CompilationPlan uses granular semantic target capabilities.

The canonical capability families are:

```text
IPCPipes
IPCTypedMessages
IPCSharedMemory
IPCMutex
IPCSemaphore
IPCAtomicRepresentation(width)
```

These facts mean that the selected target/runtime profile can provide the complete Sec semantics for that family.

They do not merely mean that one similarly named native API exists.

---

## 119. Route-specific resource adapters

Target metadata must be able to resolve at least:

```text
CapabilityDuplicate(T)
ProcessStartupTransfer(T)
IPCTypedTransfer(T)
ProcessStandardIOResourceBinding(T, role)
```

`ProcessTransferable(T)` remains the semantic language eligibility proof.

A route-specific adapter is the selected target's proof that the particular operation can realize that semantic transfer.

---

## 120. Process standard-I/O capabilities

The process target facts are semantically named:

```text
ProcessStandardIOPipes
ProcessStandardIOResourceBinding(T, role)
```

Backend-specific inheritance, descriptor duplication, handle lists, or equivalent mechanisms are not canonical source capability names.

`CommandOutputMode.Pipe`/`CommandInputMode.Pipe` require the relevant process execution support, `IPCPipes`, and `ProcessStandardIOPipes`.

`Resource` binding requires the matching `ProcessStandardIOResourceBinding(T, role)`.

---

## 121. Derived capability requirements

Composite source abstractions derive requirements from fundamental facts.

Examples:

```text
CreateSharedValue[T]
    requires IPCSharedMemory
    requires IPCMutex
    requires SharedStorable(T)

CreateIPCAtomic[T]
    requires IPCAtomicValue(T)
    requires IPCAtomicRepresentation(width(T))

CreateIPC[T]
    requires IPCTypedMessages
    requires complete target-realizable transfer plan for T
```

IPC does not require redundant target booleans for every wrapper type.

---

## 122. Static support versus runtime failure

If selected CompilationPlan statically lacks a required IPC family, atomic width, or transfer adapter, the operation is rejected at compile time.

Runtime errors are used when the canonical mechanism exists but the actual operation fails because of conditions such as:

```text
permission
memory exhaustion
quota/resource limit
mapping failure
peer loss
native operational failure
```

A backend must not use `Unsupported` or `NativeFailure` as a substitute for missing static target capability information.

---

## 123. Optional CPU features

A CPU/ISA feature counts as supported only when the selected target/runtime profile guarantees it for the compiled artifact.

`IPCAtomic[T]` must never change at runtime from lock-free to hidden-mutex semantics merely because an optional CPU feature is unavailable.

---

## 124. Full-family conformance

A target may declare an IPC family supported only if it implements the complete locked contract, including applicable:

```text
ownership
lifecycle
duplication
rollback
ordering
clean/abnormal peer semantics
owner-death behavior
memory visibility
waiting
select readiness/commit
Task/Thread blocking integration
canonical error mapping
process-death cleanup
```

The backend may emulate a missing native primitive only when the complete Sec semantics are preserved.

---

## 125. No universal portable IPC native-handle surface

Sec 0.1 defines no universal:

```text
IPCPlatform
IPCHandle
NativeHandle property
```

for portable IPC resources.

Target-specific native integration belongs to platform/unsafe surfaces with their own ownership contracts.

Extracting a native representation does not duplicate Sec ownership and does not make numeric handles transferable.

---

## 126. Multi-target compilation

IPC capability resolution occurs independently for each selected target/runtime profile.

An unconditional source operation that requires an unsupported capability makes that target artifact invalid.

Existing compile-time target/build mechanisms may select different source implementations.

Sec does not replace target validation with runtime `if IPCSupported` checks.

Bare shared RAM, RTOS tasks, scheduler queues, or similarly named native primitives do not automatically satisfy process-isolated IPC semantics.

---

# Part XIII — Semantic IR and analysis

## 127. IPC remains semantic until safe lowering

IPC operations must not be erased into generic native calls before ownership, lifecycle, synchronization, and target obligations are resolved.

Semantic IR must preserve, where relevant:

```text
IPC operation kind
logical resource identity
capability owner identity
resource/capability relationship
message/transfer type identity
memory order
process-shared synchronization scope
source location
```

---

## 128. Logical identities

Semantic IR distinguishes:

```text
Sec owner identity
logical IPC resource identity
shared-storage identity
synchronization identity
native handle/descriptor identity
```

Duplication creates a new owner identity while preserving the applicable logical resource/synchronization identity.

Pipe read and write capabilities refer to different sides of one logical pipe.

Typed sender and receiver capabilities refer to one logical typed message transport.

---

## 129. Transactional prepare/commit/rollback

Operations that conditionally transfer ownership or build composite destination state must preserve a semantic prepare/commit/rollback model.

This includes at least:

```text
process startup transfer
IPCSender.Send
capability transfer
Command resource configuration
Command startup resource binding
SharedValue duplication
composite typed materialization
select branch commit
```

Failure before commit leaves source-semantic ownership unchanged.

Success commits the complete source-visible operation.

Partial aggregate ownership states must not become source-visible.

`try` does not erase failure-path cleanup responsibility.

---

## 130. Readiness and commit

Selectable IPC operations require non-destructive readiness followed by commit of the chosen branch.

Backend implementations must not speculatively perform all observable operations and then attempt to undo losing branches.

The same commit boundary governs:

```text
select
cancellation
timeout
peer lifecycle transitions
```

---

## 131. Waiting effects

The compiler classifies the following as potentially waiting:

```text
Pipe.Read
Pipe.Write
Pipe.ReadExact
Pipe.WriteAll
IPCSender.Send
IPCReceiver.Receive
IPCMutex.Lock
IPCSemaphore.Acquire
SharedValue.Lock
```

along with the process waiting operations owned by `processes.md`.

The following are nonwaiting source operations:

```text
IPCMutex.TryLock
IPCSemaphore.TryAcquire
IPCAtomic operations
simple IsClosed/status properties
```

Native implementation details must not change the source effect category.

---

## 132. Guard-liveness analysis

The compiler tracks live `IPCMutexGuard` and `SharedValueGuard[T]` values across control flow.

A live guard across a potentially waiting/suspending operation is rejected under the Sec 0.1 guard rule.

The check uses actual liveness, not merely lexical presence earlier in the function.

---

## 133. Cross-process deadlock analysis

Deadlock analysis may include:

```text
Task/Thread/Process execution nodes
IPCMutex identities
IPCSemaphore waits
IPCSender.Send
IPCReceiver.Receive
pipe backpressure
process join/wait
```

Deep analysis should report provable or sufficiently supported wait cycles across process boundaries.

A zero-capacity typed transport makes `Send()` a real rendezvous wait edge.

Process pipe backpressure can form a cycle with process join and must be represented accordingly.

---

## 134. Shared-memory race analysis

Analysis tracks shared backing identity through duplication and transfer.

Different process-local mappings of the same backing region are potential aliases to the same shared bytes.

`unsafe View()` does not disable race analysis.

`SharedValue[T]` provides a compiler-known relation between storage identity and synchronization identity.

`IPCMutex`, `IPCSemaphore`, and `IPCAtomic` provide synchronization facts according to their contracts.

Ordinary process transfer/materialization by value creates destination storage and does not create a shared-memory alias edge.

---

## 135. Atomic analysis

`MemoryOrder` remains semantically visible through the analysis/IR stages that need it.

Release/acquire/SeqCst operations on the same logical IPC atomic synchronization relation create the ordering facts defined by the concurrency memory model.

Relaxed access does not create acquire/release publication of unrelated shared state.

Optimization must preserve the required atomic ordering and synchronization scope.

---

## 136. Lowering obligations

An IPC semantic operation may be erased only after the compiler/backend has a concrete plan preserving:

```text
ownership
commit/rollback
cleanup
target adapter
waiting/readiness
synchronization scope/order
lifecycle
canonical error mapping
```

Compiler-development verification should reject contradictory or incomplete IPC lowering facts.

---

# Part XIV — Security and resource limits

## 137. IPC peers are not inherently trusted

A peer may be buggy, compromised, built separately, terminated mid-transfer, or connected through a future external rendezvous mechanism.

Typed transfer state is therefore validated before it becomes source-visible typed state.

Type safety does not provide application authentication or authorization.

Application-level questions such as permission to execute a command or accept a job remain application protocol responsibilities.

---

## 138. Bounded resource use

Core IPC permits implementation-defined bounded limits for:

```text
message size
queue storage
capability count
materialization work/depth
temporary transfer resources
```

A peer must not force hidden unbounded runtime memory growth merely by producing data faster than the consumer can process it.

A semantically valid operation that exceeds an applicable runtime limit returns the canonical `ResourceLimit` variant of the operation's error family.

---

## 139. Overflow-safe validation

All transfer size/count arithmetic must be checked before allocation, indexing, or native resource reservation.

Malformed metadata must never cause undersized allocation, out-of-bounds materialization, integer wraparound, or capability-count overflow.

---

## 140. Exact cleanup

Every temporary native/runtime object created during transfer, duplication, process bootstrap, materialization, or rollback must have exactly one defined owner or cleanup path.

This obligation applies to:

```text
success
failure
cancellation before commit
peer death
process bootstrap failure
materialization failure
```

After semantic commit, the source does not regain ownership because of later peer failure.

---

## 141. Raw shared memory as explicit trust boundary

Raw shared-memory contents are concurrently mutable external state unless protected by a canonical synchronization protocol.

`unsafe View()` gives the programmer direct responsibility for validating any application-defined raw representation and synchronization discipline.

Safe `SharedValue[T]` exists for the common typed synchronized case.

---

## 142. No silent family fallback

A runtime may choose different native mechanisms internally, but it must preserve the selected Sec abstraction's complete contract.

It must not change message, ownership, lifecycle, or synchronization semantics merely because another native mechanism is easier to use.

---

# Part XV — Diagnostics and tooling

## 143. Diagnostic principles

IPC diagnostics are mentor-style and identify the concrete semantic reason for rejection.

Where known, diagnostics should identify:

```text
the value/type/field that cannot cross
the exact process boundary or route
the missing target capability/adapter
whether the problem is semantic eligibility or target support
the required explicit move marker
the live guard that crosses a wait
the legal atomic memory-order set
an actionable alternative
```

Diagnostics must use stable IDs according to the canonical diagnostics rulebook.

---

## 144. Transferability diagnostic example

A rejected typed IPC value should explain the containing path, for example:

```text
WorkRequest cannot be sent through IPC because field Input contains a
process-local reference with no ProcessTransferable representation.

Move or copy the required semantic value into process-transferable storage,
or use an explicit IPC/shared-memory capability.
```

---

## 145. Missing route adapter diagnostic

A type may be semantically process-transferable while the selected transport route is unavailable.

The diagnostic must distinguish those facts, for example:

```text
File is process-transferable, but the selected target has no
IPCTypedTransfer(File) adapter for this IPC transport.

File may still be transferable during process startup if that route is supported.
```

---

## 146. Explicit move diagnostic

For a required consuming send:

```text
sending "message" transfers its ownership; write sender.Send(<-message)
for an existing named value.

If Send returns Err before commit, "message" remains available.
```

---

## 147. Guard diagnostic

A live IPC guard across a wait should identify both values/operations:

```text
IPCMutexGuard "guard" remains live across receiver.Receive(), which may wait.
Release the guard before waiting or restructure the critical section.
```

A statically provable recursive acquisition of the same logical `IPCMutex` is diagnosed as a self-deadlock.

---

## 148. `SharedStorable` diagnostic

The compiler should show the first relevant non-shared-storable path, for example:

```text
State is not SharedStorable:
    State.Name -> string -> process-local owned storage

SharedValue[T] requires a fixed-size self-contained shared representation.
```

---

## 149. IPC atomic diagnostic

Diagnostics distinguish semantic type ineligibility from target capability failure.

Examples:

```text
string is not an IPCAtomicValue
```

versus:

```text
uint64 is an IPCAtomicValue, but the selected target does not provide the
required lock-free process-shared 64-bit atomic operation set.
```

Invalid `MemoryOrder` combinations are compile-time errors.

---

## 150. LSP obligations

LSP completion/hover must expose the exact public IPC surface for the resolved type and target-independent language contract.

Where target context is available, diagnostics/hover may additionally report selected-target support without changing the canonical type identity.

Compiler-known semantic properties such as `SharedStorable(T)` and `IPCAtomicValue(T)` may be explained by tooling but are not presented as user-implemented interfaces.

---

# Part XVI — Required conformance tests

## 151. Pipe tests

Every claimed pipe implementation must cover at least:

```text
creation success/failure atomicity
byte ordering
short Read
short Write
ReadExact
WriteAll partial-progress failure
EOF after buffered data
BrokenPipe
duplicate reader
duplicate writer
EOF only after last writer disappears
BrokenPipe only after last reader disappears
idempotent Close
destructor close
bounded backpressure
select Read
select Write
non-selected branch has no effect
process death releases local endpoints
```

---

## 152. Typed IPC tests

Every claimed typed IPC implementation must cover at least:

```text
one Send -> one complete Receive
per-sender ordering
multiple senders
multiple receivers
exactly-once delivery
no broadcast
clean close -> queued messages -> None
abnormal sender disappearance -> terminal error
stable clean terminal state
stable abnormal terminal state
receiver disappearance -> PeerClosed
sender/receiver duplication
transactional moved Send success
transactional moved Send failure
capability-bearing message
rollback on component transfer failure
materialization failure cleanup
bounded backpressure
zero-capacity/rendezvous realization
select Send
select Receive
cancellation before commit
commit wins over cancellation
```

---

## 153. Shared-memory tests

Required shared-memory coverage includes:

```text
zero initialization
logical size
same backing visible through different process mappings
mapping survives originating SharedMemory.Close while mapping lives
backing survives while any capability/mapping remains
mapping is not ProcessTransferable
unsafe View lifetime
duplicate backing identity
cross-process visibility under canonical synchronization
process death cleanup
```

---

## 154. Synchronization tests

`IPCMutex` tests include:

```text
cross-process mutual exclusion
non-recursive semantics
TryLock
guard destruction releases
non-transferable guard
guard across wait rejected
owner death -> permanent poison
duplicate same synchronization identity
release/acquire visibility
select Lock
```

`IPCSemaphore` tests include:

```text
Initial/Maximum validation
Acquire
TryAcquire
Release
LimitExceeded
cross-process sharing
permit not restored after process death
release/acquire visibility
select Acquire
```

---

## 155. Shared value tests

Required `SharedValue[T]` coverage includes:

```text
accepted SharedStorable type
rejected nested string/reference/resource
initial value
Read snapshot
Write commit
duplicate sees same logical value
process transfer
guard liveness
owner-death poison
layout identity
runtime LayoutMismatch where applicable
transactional duplicate
```

---

## 156. IPC atomic tests

For every supported concrete target representation, cover:

```text
Load
Store
Swap
strong CompareExchange success
strong CompareExchange mismatch
no spurious failure
Relaxed
Acquire
Release
AcqRel
SeqCst
invalid operation/order rejection
non-constant explicit IPC MemoryOrder rejection
duplicate same atomic identity
cross-process atomicity
release/acquire publication
unsupported width compile-time rejection
no hidden mutex fallback
```

---

## 157. Capability-transfer tests

Required capability-transfer coverage includes:

```text
File duplication preserves the same underlying open-resource identity
Pipe duplication preserves the same logical pipe identity
move transfer consumes only on successful commit
failed transfer retains source ownership
composite transfer is all-or-nothing
destination native handle value may differ
authority never increases
no unrelated resource bulk inheritance
rollback after peer death before commit
unsupported route rejected at compile time
```

---

## 158. Command integration tests

Required process/IPC integration coverage includes:

```text
File -> stdin
File -> stdout
File -> stderr
PipeReader -> stdin
PipeWriter -> stdout
PipeWriter -> stderr
Resource mode distinct from Pipe mode
Take*Pipe returns None for Resource mode
InvalidIOResource
resource setter success commits ownership
resource setter failure preserves ownership
Start success establishes child capability
Start failure retains configured resource for retry
no hidden extra parent resource after successful Start
```

---

# Part XVII — Cross-rulebook authority

## 159. Canonical ownership map

The authoritative ownership is:

```text
rules/concurrency/ipc.md
    Pipe/PipeReader/PipeWriter
    typed IPC
    shared memory/mappings
    process-shared synchronization
    SharedValue
    IPCAtomic
    general capability-transfer semantics
    IPC target/analysis/security contracts

rules/concurrency/processes.md
    Process[T]
    Command
    process lifecycle/completion
    process standard-I/O source API using IPC-owned capability semantics

rules/memory/transferability.md
    ProcessTransferable eligibility and boundary proof

rules/concurrency/channels.md
    ordinary in-process Channel[T]

rules/concurrency/select.md
    generic select source-order/readiness/commit semantics

rules/concurrency/blocking.md
    generic blocking/suspension/effect policy

rules/concurrency/atomics.md
    ordinary Atomic[T]
    MemoryOrder
    CompareExchangeResult[T]
    common atomic operation naming/order validity

rules/concurrency/concurrency_memory_model.md
    acquire/release/SeqCst and happens-before semantics
```

---

## 160. Required harmonization

This rulebook is synchronized by
`rules/corrections/applied/ipc-cross-rulebook-correction-20260915.md`.

At minimum it must synchronize:

```text
rules/concurrency/processes.md
rules/memory/transferability.md
rules/concurrency/channels.md
rules/concurrency/select.md
rules/concurrency/blocking.md
rules/concurrency/concurrency_memory_model.md
rules/library/stdlib.md and the canonical File API
rules/concurrency/concurrency.md
language-rulebook-status.md
governance/concurrency_process.yaml
governance/index.yaml
```

The ordinary atomic rulebook remains the source of truth for `MemoryOrder`, `Swap`, and `CompareExchangeResult[T]`; IPC reuses those canonical names and semantics rather than introducing a duplicate atomic vocabulary.

---

## 161. Completeness rule

This rulebook is complete only when implementations preserve all source-visible types, operations, errors, ownership transitions, lifecycle states, target requirements, synchronization semantics, analysis facts, and rollback obligations defined here.

Implementation-defined variability is permitted only where this rulebook explicitly allows it, including:

```text
bounded capacity/queue limits
native mechanism selection
process-local virtual mapping address
native handle/descriptor value
internal transfer representation
internal helper structure
```

Such variability must not weaken the canonical Sec contract.

---

## 162. Summary of source-visible IPC API

The complete Sec 0.1 IPC-owned public surface defined by this rulebook is:

```sec
@noCopy
type Pipe struct {
    Reader: PipeReader,
    Writer: PipeWriter,
}

@noCopy
type PipeReader

@noCopy
type PipeWriter

fn CreatePipe() Result[Pipe, IOError]

@noCopy
type IPCPair[T] struct {
    Sender: IPCSender[T],
    Receiver: IPCReceiver[T],
}

@noCopy
type IPCSender[T]

@noCopy
type IPCReceiver[T]

fn CreateIPC[T]() Result[IPCPair[T], IPCError]

enum IPCError error {
    Closed
    PeerClosed
    TransferFailed
    MaterializationFailed
    OutOfMemory
    ResourceLimit
    ProtocolFailure
    NativeFailure
}

@noCopy
type SharedMemory

@noCopy
type SharedReadMapping

@noCopy
type SharedWriteMapping

fn CreateSharedMemory(size: uint) Result[SharedMemory, SharedMemoryError]

enum SharedMemoryError error {
    Closed
    InvalidSize
    PermissionDenied
    OutOfMemory
    ResourceLimit
    MappingFailed
    DuplicationFailed
    NativeFailure
}

@noCopy
type IPCMutex

@noCopy
type IPCMutexGuard

@noCopy
type IPCSemaphore

fn CreateIPCMutex() Result[IPCMutex, IPCSyncError]

fn CreateIPCSemaphore(
    Initial: uint,
    Maximum: uint
) Result[IPCSemaphore, IPCSyncError]

enum IPCSyncError error {
    Closed
    InvalidCount
    LimitExceeded
    OwnerDied
    PermissionDenied
    OutOfMemory
    ResourceLimit
    NativeFailure
}

@noCopy
type SharedValue[T]

@noCopy
type SharedValueGuard[T]

fn CreateSharedValue[T](
    initial: T
) Result[SharedValue[T], IPCSharedError]

enum IPCSharedError error {
    Closed
    LayoutMismatch
    OwnerDied
    PermissionDenied
    OutOfMemory
    ResourceLimit
    MappingFailed
    DuplicationFailed
    NativeFailure
}

@noCopy
type IPCAtomic[T]

fn CreateIPCAtomic[T](
    initial: T
) Result[IPCAtomic[T], IPCAtomicError]

enum IPCAtomicError error {
    PermissionDenied
    OutOfMemory
    ResourceLimit
    DuplicationFailed
    NativeFailure
}
```

The exact methods on each opaque type are the methods defined in the corresponding sections above and are normative.

`IOError`, `MemoryOrder`, and `CompareExchangeResult[T]` are dependencies owned by their canonical rulebooks and are not redefined by IPC.
