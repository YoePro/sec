# Sec Standard Library — I/O

Status: Draft  
Created: 2026-10-02  
Updated: 2026-10-02  
Revision: 0.2  
Sec version: 0.1  
Canonical path: `stdlib/io/std-io.md`  
Replaces: none

---

## 1. Purpose

The `io` standard-library area defines Sec's general byte-stream and text-I/O abstractions, common I/O capabilities, generic transfer operations, buffering facilities, in-memory I/O, pipes, standard streams, and composable I/O adapters.

`io` is deliberately independent of the resource that provides or consumes the data. Files, sockets, process streams, device streams, HTTP bodies, memory buffers, and similar resources may implement the common I/O interfaces without moving their resource-specific semantics into `io`.

This book defines the common architecture and root `io` API. Detailed APIs for specialized facilities are specified by dedicated books under their respective package directories.

---

## 2. Design principles

### 2.1 I/O describes behavior, not resource ownership

The root `io` package defines behavioral contracts such as readable, writable, seekable, flushable, and closable behavior.

It does not own filesystem paths, file metadata, sockets, HTTP, serialization formats, compression formats, terminals, or hardware devices merely because those facilities perform I/O.

A concrete resource remains in the package that owns its semantics and may implement one or more `io` interfaces.

Examples include:

```text
file resource        -> Reader, Writer, ReaderAt, WriterAt, Seeker, Flusher, Closer
TCP stream           -> Reader, Writer, Closer
HTTP body            -> Reader, Closer
process stdout pipe  -> Reader, Closer
memory buffer        -> Reader, Writer, Seeker
```

### 2.2 Byte transport and representation are separate concerns

The root I/O model transports bytes. It does not reinterpret binary protocol values, file-format fields, machine representation, or register-defined layouts.

Sec register types already provide explicit fixed-width representation and byte-order semantics where a register representation is appropriate. I/O must not duplicate that model with generic protocol-oriented methods such as `ReadU32BigEndian()` or `WriteU16LittleEndian()`.

Text decoding and encoding belong to the text-I/O layer. Serialization and structured data formats belong to their own standard-library areas.

### 2.3 No hidden whole-input buffering

An operation whose contract is streaming must remain streaming.

Operations that allocate a result containing all remaining input must be explicit and bounded where untrusted or unbounded input could otherwise consume arbitrary memory.

### 2.4 Partial progress is part of ordinary I/O

A successful `Read` or `Write` may transfer fewer bytes than the supplied buffer contains.

Callers that require exact completion use dedicated operations such as `ReadExactly` or `WriteAll` rather than assuming that one `Read` or `Write` completes the whole transfer.

### 2.5 Generic I/O is expressed through Sec interfaces

Sec interfaces are the canonical mechanism for common I/O capabilities.

Conformance is explicit:

```sec
impl SocketStream implements io.Reader, io.Writer, io.Closer {
    ...
}
```

The standard library must not rely on accidental structural conformance.

### 2.6 Receiver capability is part of the contract

Sequential reads and writes advance or otherwise mutate stream state and therefore use `mut fn` in interface declarations.

Random-access operations that are defined not to modify a shared stream position use a shared receiver contract.

Concrete implementations use ordinary `fn` syntax; the compiler verifies that their inferred receiver requirements satisfy the interface requirement.

### 2.7 Cancellation follows Sec's execution model

I/O operations that may block are cancellable where the target and platform support cancellation.

They use Sec's ordinary inferred cancellation/context effect rather than a parallel hierarchy such as `AsyncReader`, `AsyncWriter`, or `AsyncStream`.

The I/O API does not duplicate `Task`, `Context`, or cancellation concepts.

---

## 3. Package structure

The initial I/O family is:

```text
stdlib/io/
├── std-io.md
├── interfaces.sec
├── transfer.sec
│
├── reader/
├── writer/
├── seek/
│
├── buffer/
├── buffered/
│
├── text/
├── memory/
│
├── pipe/
│
├── limit/
├── multi/
├── section/
├── tee/
│
└── stdio/
```

A separate `binary` package is intentionally not part of this structure. Generic byte-order-aware protocol reading would duplicate representation facilities already provided by Sec types such as registers and by future serialization/encoding packages.

The package structure may be consolidated later if several adapter packages prove too small to justify independent public package identities. Such consolidation must not change the semantic contracts defined by this book.

---

## 4. Root module

All root source files use:

```sec
module io
```

The root package is imported as:

```sec
import io
```

Subpackages use their own canonical paths, for example:

```sec
import "io/buffered"
import "io/memory"
import "io/pipe"
```

The final path component is the ordinary package qualifier.

---

## 5. Canonical root file manifest

```text
stdlib/io/
├── std-io.md
├── interfaces.sec
└── transfer.sec
```

The common root error type `IOError` is owned by `core/error.sec` (§ 6), so every I/O-capable package can use it without importing `io`. The root package therefore has no separate results file.

`interfaces.sec` owns common capability interfaces and seek-origin identity.

`transfer.sec` owns generic transfer helpers operating only through those interfaces.

---

## 6. `IOError` in `core/error.sec`

### 6.1 Location

`IOError` is declared in `core/error.sec` beside the other core error families. It is an error enum and carries the `error` marker required for `Result` error channels.

During migration, `core/error.sec` also declares `IOErrorLegacy`: the detailed errno-oriented family used by the existing file and directory API in `stdlib/io/file.linux.amd64.sec`. Each legacy variant documents its meaning and the closest `IOError` variant. The two families are to be merged; `IOErrorLegacy` is not part of the root `io` contract.

### 6.2 `IOError`

The root I/O layer requires one common error family so generic code can operate through `Reader`, `Writer`, and related interfaces without knowing the concrete resource package.

```sec
enum IOError error {
    // The operation was attempted after the relevant I/O endpoint was closed.
    Closed,

    // One or more arguments are invalid for the requested operation.
    InvalidInput,

    // The concrete resource does not support the requested operation.
    Unsupported,

    // The operation was interrupted before it could complete normally.
    Interrupted,

    // The operation reached its configured or externally imposed timeout.
    TimedOut,

    // Cancellation was requested through Sec's execution context.
    Cancelled,

    // An exact-length operation reached end-of-stream before completing.
    UnexpectedEnd,

    // Repeated operations made no progress where progress was required.
    NoProgress,

    // A platform, runtime, driver, or concrete-resource failure occurred and
    // is not represented by another IOError variant.
    NativeFailure,
}
```

`IOError` is intentionally an umbrella error for generic I/O operations. Resource-specific packages may expose more detailed errors through their own concrete APIs, but an operation invoked through a root `io` interface must be representable as `IOError`.

`EndOfStream` is not an `IOError`. Ordinary sequential EOF is represented by the `Reader.Read` contract defined below.

---

## 7. `interfaces.sec`

### 7.1 Source-file header

```sec
module io

/*
 * Sec Standard Library - io - Common I/O capability interfaces
 *
 * File:        stdlib/io/interfaces.sec
 * Module:      io
 * Revision:    1
 * Updated:     2026-10-02
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Shared readable, writable, random-access, seek, flush, and close
 *   capability contracts.
 *
 * Rulebook:
 *   stdlib/io/std-io.md
 */
```

### 7.2 `Reader`

```sec
interface Reader {
    mut fn Read(buffer: ref mut byte[]) Result[uint, IOError]
}
```

`Read` attempts to place bytes into `buffer` and returns the number of bytes transferred.

For a non-empty buffer:

- `Ok(n)` where `n > 0` means exactly `n` bytes were written to `buffer[0..<n]`;
- `Ok(0)` means end-of-stream;
- `Err(...)` means the operation failed.

`Read` may return fewer bytes than `buffer.Len` without indicating EOF.

For an empty buffer, `Ok(0)` is a successful zero-length operation and does not establish whether the underlying stream is at EOF.

A successful read must never return a value greater than `buffer.Len`.

Bytes in `buffer[n..]` are not modified by the `Reader` contract.

### 7.3 `Writer`

```sec
interface Writer {
    mut fn Write(data: ref byte[]) Result[uint, IOError]
}
```

`Write` attempts to consume bytes from `data` and returns the number successfully written.

`Write` may write fewer bytes than `data.Len` without that being an error.

For non-empty `data`, returning `Ok(0)` without a resource-defined reason that permits future progress is not sufficient for helpers that require completion. Generic completion helpers detect repeated zero progress and return `IOError.NoProgress`.

A successful write must never return a value greater than `data.Len`.

`Write` does not consume ownership of the source byte array.

### 7.4 `ReaderAt`

```sec
interface ReaderAt {
    fn ReadAt(offset: uint64, buffer: ref mut byte[]) Result[uint, IOError]
}
```

`ReadAt` reads beginning at absolute byte offset `offset`.

It does not modify a sequential stream position.

The offset is measured in bytes from the beginning of the underlying logical resource.

A non-empty `ReadAt` returning `Ok(0)` means that no byte exists at the requested position or later within the accessible logical resource.

`ReaderAt` does not imply `Reader`, and `Reader` does not imply `ReaderAt`.

### 7.5 `WriterAt`

```sec
interface WriterAt {
    fn WriteAt(offset: uint64, data: ref byte[]) Result[uint, IOError]
}
```

`WriteAt` writes beginning at absolute byte offset `offset` without modifying a sequential stream position.

Support for extending the underlying resource is resource-specific. A generic `WriterAt` caller may not assume that writing beyond the current logical end is supported.

`WriterAt` does not imply `Writer`, and `Writer` does not imply `WriterAt`.

### 7.6 `SeekOrigin`

```sec
enum SeekOrigin {
    // Offset is relative to the beginning of the resource.
    Start

    // Offset is relative to the current sequential position.
    Current

    // Offset is relative to the logical end of the resource.
    End
}
```

### 7.7 `Seeker`

```sec
interface Seeker {
    mut fn Seek(offset: int64, origin: SeekOrigin) Result[uint64, IOError]
}
```

`Seek` changes the sequential position and returns the resulting absolute byte position from the beginning of the resource.

An operation that would produce a negative resulting position returns `IOError.InvalidInput`.

A resource may reject positions beyond its logical end with `IOError.InvalidInput` or may permit them where its own semantics define sparse or future writes.

`Seeker` does not imply `Reader` or `Writer`.

### 7.8 `Flusher`

```sec
interface Flusher {
    mut fn Flush() Result[void, IOError]
}
```

`Flush` requests that data buffered by the concrete I/O object be forwarded to its next underlying layer.

`Flush` does not imply durable storage. Filesystem or device durability guarantees such as synchronization to persistent media belong to the concrete resource API.

### 7.9 `Closer`

```sec
interface Closer {
    mut fn Close() Result[void, IOError]
}
```

`Close` releases the I/O endpoint's explicit operational capability.

`Close` is idempotent. Calling `Close` on an already closed endpoint returns `Ok()`.

Concrete owning types that require deterministic cleanup also provide `free()` according to Sec's destruction model. `free()` must release the resource exactly once even when explicit `Close()` was not called. Destructor cleanup cannot propagate an ordinary `Result` error.

### 7.10 Composite interfaces

Common combinations are named so APIs do not need to invent local interface combinations.

```sec
interface ReadWriter implements Reader, Writer {
}

interface ReadSeeker implements Reader, Seeker {
}

interface WriteSeeker implements Writer, Seeker {
}

interface ReadWriteSeeker implements Reader, Writer, Seeker {
}

interface ReadCloser implements Reader, Closer {
}

interface WriteCloser implements Writer, Closer {
}

interface ReadWriteCloser implements Reader, Writer, Closer {
}
```

A concrete type explicitly implements the most appropriate public interface or interfaces on its primary `impl` declaration.

Implementing `ReadWriteSeeker` satisfies the requirements inherited from `Reader`, `Writer`, and `Seeker`; the implementation does not repeat separate method bodies for each inherited path.

---

## 8. `transfer.sec`

### 8.1 Source-file header

```sec
module io

/*
 * Sec Standard Library - io - Generic streaming transfer helpers
 *
 * File:        stdlib/io/transfer.sec
 * Module:      io
 * Revision:    1
 * Updated:     2026-10-02
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Generic byte transfer and exact-completion helpers operating through
 *   the root I/O capability interfaces.
 *
 * Rulebook:
 *   stdlib/io/std-io.md
 */
```

### 8.2 Public API

```sec
fn ReadAtLeast(
    reader: ref mut Reader,
    buffer: ref mut byte[],
    minimum: uint,
) Result[uint, IOError]

fn ReadExactly(
    reader: ref mut Reader,
    buffer: ref mut byte[],
) Result[void, IOError]

fn ReadAll(
    reader: ref mut Reader,
    limit: uint64,
) Result[byte[], IOError]

fn WriteAll(
    writer: ref mut Writer,
    data: ref byte[],
) Result[void, IOError]

fn Copy(
    destination: ref mut Writer,
    source: ref mut Reader,
) Result[uint64, IOError]

fn CopyN(
    destination: ref mut Writer,
    source: ref mut Reader,
    count: uint64,
) Result[uint64, IOError]
```

### 8.3 `ReadAtLeast`

`ReadAtLeast` keeps reading until at least `minimum` bytes have been placed in `buffer`, EOF is reached, or an error occurs.

`minimum` must not exceed `buffer.Len`. Otherwise the operation returns `IOError.InvalidInput` without reading from the source.

The return value is the total number of bytes transferred into the beginning of `buffer`.

If EOF is reached before `minimum` bytes are obtained, the operation returns `IOError.UnexpectedEnd`.

### 8.4 `ReadExactly`

`ReadExactly` fills the entire supplied buffer.

An empty buffer succeeds immediately.

If EOF occurs before all bytes are obtained, the operation returns `IOError.UnexpectedEnd`.

The operation does not allocate.

### 8.5 `ReadAll`

`ReadAll` reads until EOF and returns the collected bytes.

The returned array may contain at most `limit` bytes.

If more than `limit` bytes would be required, `ReadAll` returns `IOError.InvalidInput` rather than returning an array larger than the requested bound.

Allocation failure propagates through the ordinary Sec allocation/error mechanism applicable to the backing array implementation.

`ReadAll` is intentionally bounded. The root package does not provide an unbounded whole-stream convenience overload.

### 8.6 `WriteAll`

`WriteAll` keeps calling `Write` until all bytes have been written or an error occurs.

An empty input succeeds immediately.

If the writer repeatedly reports zero progress for non-empty remaining input, `WriteAll` returns `IOError.NoProgress` rather than looping indefinitely.

### 8.7 `Copy`

`Copy` transfers bytes from `source` to `destination` until the source reaches EOF or an error occurs.

The return value is the total number of bytes successfully written to the destination.

`Copy` is streaming. It must not materialize the complete source in memory.

Its implementation may use a bounded reusable temporary buffer or a more efficient resource-specific path when such optimization preserves the same observable semantics.

### 8.8 `CopyN`

`CopyN` transfers exactly `count` bytes unless EOF or another error prevents completion.

If the source reaches EOF before `count` bytes have been transferred, `CopyN` returns `IOError.UnexpectedEnd`.

The success return value is therefore exactly `count`.

`count == 0` succeeds without reading or writing.

---

## 9. Reader-focused package

`io/reader` owns additional generic reader adapters and reader-specific convenience facilities that do not belong in the minimal root contract.

Candidate facilities include byte- and rune-oriented adapters, counting readers, empty readers, repeating readers, and reader-specific helper types.

Its exact public API is defined by `stdlib/io/reader/std-io-reader.md` and is not specified by this root book.

---

## 10. Writer-focused package

`io/writer` owns generic writer adapters and writer-specific convenience facilities that do not belong in the minimal root contract.

Candidate facilities include sink/discard writers, counting writers, and writer-specific helper types.

Its exact public API is defined by `stdlib/io/writer/std-io-writer.md`.

---

## 11. Seek package

`io/seek` owns higher-level seek and cursor helpers built on the root `Seeker`, `ReaderAt`, and `WriterAt` capabilities.

It must not duplicate filesystem-specific positioning or file-size semantics.

Its exact public API is defined by `stdlib/io/seek/std-io-seek.md`.

---

## 12. Buffer and buffered packages

`io/buffer` owns reusable byte-buffer values used as data containers or I/O backing storage.

`io/buffered` owns buffered reader and writer layers that wrap other I/O capabilities.

Buffer storage and buffered stream behavior remain separate concepts: owning a byte buffer is not equivalent to wrapping another stream with buffering.

The buffered package is expected to cover operations such as peeking, buffered-byte inspection, discarding already-buffered bytes, line-oriented reading, and explicit flush behavior where those operations can be specified without violating streaming semantics.

Exact APIs are defined in their package books.

---

## 13. Text I/O

`io/text` owns the bridge between byte-oriented I/O and Sec text values.

It may provide text readers, text writers, line reading, rune reading/writing, and explicit character-encoding bridges.

Unicode algorithms and encoding definitions that are useful outside I/O must live in the appropriate shared Unicode/text package rather than being privately reimplemented by `io/text`.

The byte-oriented root `io.Reader` and `io.Writer` contracts remain encoding-neutral.

---

## 14. Memory I/O

`io/memory` owns I/O implementations backed by memory rather than an external resource.

Expected use cases include:

- reading from an existing byte sequence;
- writing into a growable or fixed memory buffer;
- cursor-style read/write/seek over memory;
- adapting existing in-memory data to `Reader`, `Writer`, `ReaderAt`, `WriterAt`, or `Seeker` where those capabilities are valid.

Construction must make ownership and borrowing explicit. Creating a memory reader over borrowed bytes must not silently copy them, and creating an owning reader must not silently borrow storage whose lifetime may end first.

---

## 15. Pipes

`io/pipe` owns general in-process byte-stream pipe endpoints.

A pipe exposes separate reader and writer capabilities and must define close propagation, blocked-operation wakeup, cancellation, buffering capacity, and ownership exactly.

This package is distinct from typed Sec channels and from process IPC. A byte pipe transports an ordered byte stream; a channel transports typed values according to the channel contract.

---

## 16. Limiting adapters

`io/limit` owns adapters that bound how much data may be observed or written through another stream.

Limits are semantic boundaries, not merely advisory counters.

A limited reader must never expose bytes beyond its configured remaining limit even if the underlying reader contains additional data.

---

## 17. Multi-stream adapters

`io/multi` owns composition of several readers or writers.

Reader composition presents several sources as one logical sequential input.

Writer composition forwards data according to the package's explicitly specified completion and failure policy.

Partial success across several destinations must never be hidden.

---

## 18. Section adapters

`io/section` owns bounded random-access views into another random-access source.

A section has its own logical offset zero and cannot access bytes outside its declared source range.

Creating a section must not copy the represented data.

Section access is expected to build on `ReaderAt` and, where appropriate, `WriterAt`.

---

## 19. Tee adapters

`io/tee` owns adapters that duplicate observed traffic to another I/O destination while preserving the primary streaming operation.

Failure ordering and partial-write behavior must be explicit in the package book; a tee implementation must not silently lose mirror-write failures.

---

## 20. Standard streams

`io/stdio` owns access to the process or environment's standard input, standard output, and standard error streams where the target provides them.

Hosted targets may expose all three streams. Bare-metal and restricted targets may provide none or may bind them through a platform-defined implementation.

Unsupported standard streams are a compile-time target error where absence is statically known and the program requires the capability.

The package must define synchronization and borrowing rules so that standard streams can be used safely from concurrent code without inventing global mutable aliases outside Sec's ownership model.

---

## 21. Relationship to `net`

Networking packages use `io` capabilities where the resource behaves like ordinary byte I/O.

For example, a TCP stream may implement `Reader`, `Writer`, and `Closer`. HTTP bodies may expose `Reader` semantics. Network protocols remain responsible for framing, protocol state, validation, timeouts, and protocol-specific errors.

`io` must not gain networking concepts merely to support those users.

Existing networking APIs should migrate toward the common root I/O contracts where doing so preserves or improves their existing semantics.

---

## 22. Relationship to filesystems

Filesystem packages own paths, open modes, metadata, permissions, directory traversal, durability operations, and filesystem-specific errors.

An opened file object may implement generic I/O interfaces.

`Flush()` on a buffered I/O layer means forwarding buffered bytes to the next layer. It is not synonymous with filesystem durability operations such as forcing modified data to stable storage.

---

## 23. Relationship to registers and binary representation

Sec register types are the preferred representation when fixed-width fields, bit layout, and explicit byte order are part of the data contract.

The I/O package therefore does not provide a protocol-oriented family of primitive endian readers and writers merely to reconstruct register semantics procedurally.

Where a future general serialization package needs numeric encoding independent of registers, that concern belongs to serialization/encoding rather than to the root stream interface.

---

## 24. Relationship to destruction

Owning concrete I/O resources are responsible for deterministic release through `free()`.

`Closer` provides explicit operational closure for callers that need to observe closure errors before destruction.

The following principles apply:

1. Explicit `Close()` is idempotent.
2. `free()` releases any still-owned resource exactly once.
3. `free()` must not require the caller to handle an ordinary `Result`.
4. A successful explicit `Close()` leaves destruction safe and non-duplicating.
5. Borrowed adapters do not destroy resources they do not own.
6. Owning adapters destroy owned child resources according to their declared ownership contract.

---

## 25. Allocation rules

A plain `Read`, `Write`, `ReadAt`, `WriteAt`, `Seek`, `Flush`, or `Close` operation does not allocate merely because it is invoked through an interface.

Generic adapters must document whether construction allocates.

Operations that return newly owned arrays or strings are allocation-producing operations and must remain explicit.

Streaming helpers such as `Copy` must use bounded working storage rather than storage proportional to total stream length.

---

## 26. Threading and concurrent access

Implementing an `io` interface does not by itself promise thread safety.

`Reader` and `Writer` use mutable receiver contracts because sequential state commonly changes during operations.

`ReaderAt` and `WriterAt` have no sequential-position side effect. Their shared receiver form permits implementations whose underlying resource supports independent positioned access without mutating the Sec object. Concrete packages must still specify any additional concurrency restrictions imposed by the resource itself.

Buffered adapters must not silently make a non-thread-safe underlying object thread-safe unless the adapter explicitly documents and implements that guarantee.

---

## 27. Error mapping

Concrete implementations map resource-specific failures to the nearest meaningful `IOError` when called through generic I/O interfaces.

Generic code must not inspect platform-specific numeric error codes.

Concrete APIs may retain richer resource-specific errors when called through their concrete type.

EOF remains a normal stream condition and is not mapped to `IOError` for ordinary `Read`.

Premature EOF becomes `IOError.UnexpectedEnd` only when an operation has an explicit exact-length requirement such as `ReadExactly` or `CopyN`.

---

## 28. Required tests

The root package test suite must cover at least:

1. partial reads;
2. partial writes;
3. EOF on a non-empty read buffer;
4. zero-length reads and writes;
5. exact reads across several partial underlying reads;
6. premature EOF during `ReadExactly`;
7. repeated zero-progress writes;
8. `Copy` across several source and destination chunk sizes;
9. `CopyN` exact completion;
10. `CopyN` premature EOF;
11. seek from start, current position, and end;
12. rejection of negative resulting seek positions;
13. `ReadAt` without sequential-position changes;
14. `WriteAt` without sequential-position changes;
15. idempotent explicit close;
16. destruction after explicit close;
17. destruction without explicit close;
18. cancellation of blocking I/O where supported;
19. target rejection for statically unavailable facilities;
20. adapter composition without hidden whole-stream allocation.

Tests must include implementations that deliberately return short successful reads and writes. Testing only implementations that always fill supplied buffers is insufficient.

---

## 29. Initial sub-books

The planned detailed books are:

```text
stdlib/io/reader/std-io-reader.md
stdlib/io/writer/std-io-writer.md
stdlib/io/seek/std-io-seek.md
stdlib/io/buffer/std-io-buffer.md
stdlib/io/buffered/std-io-buffered.md
stdlib/io/text/std-io-text.md
stdlib/io/memory/std-io-memory.md
stdlib/io/pipe/std-io-pipe.md
stdlib/io/limit/std-io-limit.md
stdlib/io/multi/std-io-multi.md
stdlib/io/section/std-io-section.md
stdlib/io/tee/std-io-tee.md
stdlib/io/stdio/std-io-stdio.md
```

Each sub-book must reproduce its complete public API with exact Sec declarations, ownership behavior, allocation behavior, errors, cancellation behavior, and implementation requirements.

---

## 30. References

Sec language references used by this design:

- Sec Manual — Interfaces: https://www.sec-lang.com/18-interfaces.html
- Sec Manual — References: https://www.sec-lang.com/23-references.html
- Sec Manual — Ownership: https://www.sec-lang.com/24-ownership.html
- Sec Manual — Borrowing: https://www.sec-lang.com/26-borrowing.html
- Sec Manual — Result and Option: https://www.sec-lang.com/33-result-option.html
- Sec Manual — Destruction: https://www.sec-lang.com/43-destruction.html
- Sec Manual — Cancellation and Context: https://www.sec-lang.com/59-cancellation-context.html
- Sec Standard Library — Networking: `stdlib/net/std-net.md`
- Sec Standard Library — HTTP: `stdlib/net/http/std-net-http.md`

Comparative design references include the standard I/O facilities of Go, Rust, .NET, C++, Zig, Odin, and related systems languages. Their APIs are design input only; Sec's ownership, interface, cancellation, allocation, and representation rules remain authoritative.
