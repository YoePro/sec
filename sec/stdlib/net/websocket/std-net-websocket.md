# Sec Standard Library — `net/websocket`

- **Status:** Draft — normative stdlib implementation specification
- **Created:** 2026-09-26
- **Last updated:** 2026-09-26
- **Document revision:** 0.1
- **Sec language version:** 0.1
- **Canonical path:** `stdlib/net/websocket/std-net-websocket.md`
- **Repository path:** `sec/stdlib/net/websocket/std-net-websocket.md`
- **Parent specification:** `stdlib/net/std-net.md`
- **Implementation-status fragment:** `implementation-status-std-net-websocket.yaml`
- **IANA WebSocket registry synchronized:** 2026-09-26
- **IANA registry source last updated:** 2026-06-10

---

# 1. Purpose and authority

This book is the normative implementation specification for Sec's
`net/websocket` standard-library package.

A human implementer must be able to derive the complete public API owned by this
revision and the required observable behavior from this document without
guessing omitted members from examples.

The package is imported as:

```sec
import "net/websocket"
```

and referenced as `websocket`.

Revision 0.1 implements the WebSocket protocol defined by RFC 6455 over the
HTTP/1.1 opening handshake. It also specifies RFC 7692 per-message DEFLATE.

HTTP/2 Extended CONNECT from RFC 8441 and HTTP/3 Extended CONNECT from RFC 9220
are deliberately not part of revision 0.1. The current `net/http` public API
does not yet expose a version-independent duplex protocol-stream handoff.
`net/websocket` must not invent a fake handoff that works only for HTTP/1.1
while pretending to cover HTTP/2 and HTTP/3.

A later revision shall add HTTP/2 and HTTP/3 WebSocket bootstrapping after the
common HTTP protocol-stream abstraction has been specified in `net/http`.

## 1.1 Completeness rule

Every public type, associated immutable, property, constructor, destructor,
function, method, and interface owned by revision 0.1 is listed in this book.

Normative public API sections must not use:

- ellipsis placeholders;
- “may include”;
- “and similar”;
- omitted members;
- pseudo-Sec declarations;
- placeholder constructors.

Types are not overloadable. One type identifier denotes one type in the module.

Functions, methods, and constructors may be overloaded when Sec overload
resolution permits it.

A constructor uses Sec's `init` declaration form. It is never represented by
a factory method merely to emulate construction, and the declaration does not
contain `fn`.

An empty `init()` declaration is not written merely to restate default
construction.

A destructor is named `free()`.

Consuming parameters use Sec's parameter-side move syntax. For example:

```sec
fn Send(->message: Message) Result[void, ConnectionError]
```

The move marker belongs before the parameter identifier.

## 1.2 Representation rationale rule

Every public enum, struct, union, nominal primitive type, and error union has a
**Representation rationale**.

The rationale states why the chosen Sec representation models the protocol
correctly.

In particular:

- an `enum` is used for a closed semantic set controlled by this package;
- a `struct` is used when several fields are simultaneously meaningful;
- a `union` is used when exactly one payload shape is present;
- a nominal primitive type is used for an open protocol namespace or a scalar
  value with domain-specific semantics;
- `type X union error` is used for a closed error family when variants carry
  different payloads or underlying causes.

---

# 2. Documentation contract for source and LSP

Public declarations must use structured documentation comments that can be
consumed by the Sec LSP for hover and generated reference documentation.

The canonical function/method comment shape is:

```sec
/**
 * One-sentence summary in present tense.
 *
 * Description:
 *   Additional protocol semantics.
 *
 * Parameters:
 *   - name: Meaning, accepted domain, and ownership where relevant.
 *
 * Returns:
 *   Exact success meaning and ownership of returned data.
 *
 * Errors:
 *   - ErrorType.Member: Exact condition.
 *
 * Ownership:
 *   State borrows, moves, retained resources, and post-call ownership.
 *
 * Standards:
 *   - RFC NNNN section/title.
 */
```

Rules:

1. The first sentence must stand alone as useful hover text.
2. `Parameters:` names every parameter exactly once.
3. `Returns:` is required for every non-`void` or fallible API.
4. `Errors:` is required for every `Result`-returning API.
5. `Ownership:` is required whenever the API borrows, consumes, returns owned
   storage, or owns a resource.
6. `Standards:` is required for declarations that directly represent a standard
   or registry concept.
7. Public enum/union variant comments immediately precede the variant they
   document.
8. Public struct-field comments immediately precede the field they document.
9. Public associated immutable comments immediately precede the immutable.
10. TODO text is forbidden in public API documentation in a conforming
    implementation.
11. A comment documents the declaration that exists now, not planned behavior.

---

# 3. Standards and registries

Revision 0.1 follows:

| Area | Standard / registry | URL |
|---|---|---|
| WebSocket protocol | RFC 6455 | https://www.rfc-editor.org/rfc/rfc6455 |
| Origin model used by WebSocket | RFC 6454 | https://www.rfc-editor.org/rfc/rfc6454 |
| UTF-8 | RFC 3629 | https://www.rfc-editor.org/rfc/rfc3629 |
| Per-message DEFLATE | RFC 7692 | https://www.rfc-editor.org/rfc/rfc7692 |
| Subprotocol registry clarification | RFC 7936 | https://www.rfc-editor.org/rfc/rfc7936 |
| Well-known WebSocket URIs | RFC 8307 | https://www.rfc-editor.org/rfc/rfc8307 |
| WebSocket registries | IANA WebSocket Protocol Registries | https://www.iana.org/assignments/websocket |
| HTTP semantics | RFC 9110 | https://www.rfc-editor.org/rfc/rfc9110 |
| HTTP/1.1 | RFC 9112 | https://www.rfc-editor.org/rfc/rfc9112 |

Standards explicitly reserved for a later revision:

| Area | Standard | URL |
|---|---|---|
| WebSocket over HTTP/2 | RFC 8441 | https://www.rfc-editor.org/rfc/rfc8441 |
| WebSocket over HTTP/3 | RFC 9220 | https://www.rfc-editor.org/rfc/rfc9220 |

The IANA registry snapshot used by this revision was checked on 2026-09-26.
The IANA registry page reported its most recent update as 2026-06-10.

---

# 4. Package dependencies

Revision 0.1 depends conceptually on:

```text
net/url
net/ip
net/http
encoding/base64
crypto/sha1
compression/deflate
```

Responsibilities are separated as follows:

- `net/url` parses and represents `ws` and `wss` URLs.
- `net/ip` owns TCP networking and IP endpoints.
- `net/http` owns HTTP token/header value types reused by the handshake.
- `encoding/base64` encodes/decodes the RFC 6455 nonce and accept value.
- SHA-1 is used only for the RFC 6455 handshake accept computation.
- `compression/deflate` owns the DEFLATE algorithm used by RFC 7692.
- `net/websocket` owns WebSocket handshake validation, framing, masking,
  fragmentation, control frames, message assembly, close semantics,
  subprotocol negotiation, and WebSocket extension negotiation.

SHA-1 use in the opening handshake is protocol compatibility, not a general
security recommendation. The SHA-1 result is not used as a cryptographic
signature or password hash.

`wss` requires a TLS transport. Until the canonical TLS stdlib package is
specified, `wss` support is a target/package capability requirement rather than
permission for `net/websocket` to invent a private TLS API.

---

# 5. Canonical file manifest

Revision 0.1 owns exactly these package files:

```text
stdlib/net/websocket/
├── std-net-websocket.md
├── error.sec
├── version.sec
├── subprotocol.sec
├── extension.sec
├── compression.sec
├── close.sec
├── message.sec
├── options.sec
├── handshake.sec
├── frame.sec
├── connection.sec
├── client.sec
└── server.sec
```

All `.sec` files use:

```sec
module websocket
```

`frame.sec` has no public declarations in revision 0.1. Its wire behavior is
nevertheless normative because all public connection APIs depend on it.

---

# 6. Required source-file headers

Every `.sec` file has a complete source header.

## 6.1 `error.sec`

```sec
module websocket

/*
 * Sec Standard Library - net/websocket - WebSocket error families
 *
 * File:        stdlib/net/websocket/error.sec
 * Module:      websocket
 * Revision:    1
 * Updated:     2026-09-26
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Public configuration, handshake, protocol, connection, and listener errors.
 *
 * Standards:
 *   - RFC 6455
 *     https://www.rfc-editor.org/rfc/rfc6455
 *   - RFC 7692
 *     https://www.rfc-editor.org/rfc/rfc7692
 *
 * Rulebook:
 *   stdlib/net/websocket/std-net-websocket.md
 */
```

## 6.2 `version.sec`

```sec
module websocket

/*
 * Sec Standard Library - net/websocket - WebSocket protocol version values
 *
 * File:        stdlib/net/websocket/version.sec
 * Module:      websocket
 * Revision:    1
 * Updated:     2026-09-26
 * Author:      Jonas Engström
 *
 * Purpose:
 *   WebSocket protocol version identity and parsing.
 *
 * Standards:
 *   - RFC 6455
 *     https://www.rfc-editor.org/rfc/rfc6455
 *   - IANA WebSocket Version Number Registry
 *     https://www.iana.org/assignments/websocket
 *
 * Registry:
 *   IANA WebSocket Version Number Registry
 *   Registry synchronized: 2026-09-26
 *
 * Rulebook:
 *   stdlib/net/websocket/std-net-websocket.md
 */
```

## 6.3 `subprotocol.sec`

```sec
module websocket

/*
 * Sec Standard Library - net/websocket - WebSocket subprotocol identifiers
 *
 * File:        stdlib/net/websocket/subprotocol.sec
 * Module:      websocket
 * Revision:    1
 * Updated:     2026-09-26
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Validation and representation of WebSocket subprotocol identifiers.
 *
 * Standards:
 *   - RFC 6455
 *     https://www.rfc-editor.org/rfc/rfc6455
 *   - RFC 7936
 *     https://www.rfc-editor.org/rfc/rfc7936
 *   - IANA WebSocket Subprotocol Name Registry
 *     https://www.iana.org/assignments/websocket
 *
 * Registry:
 *   IANA WebSocket Subprotocol Name Registry
 *   Registry synchronized: 2026-09-26
 *
 * Rulebook:
 *   stdlib/net/websocket/std-net-websocket.md
 */
```

## 6.4 `extension.sec`

```sec
module websocket

/*
 * Sec Standard Library - net/websocket - WebSocket extension syntax
 *
 * File:        stdlib/net/websocket/extension.sec
 * Module:      websocket
 * Revision:    1
 * Updated:     2026-09-26
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Generic WebSocket extension names, parameters, parsing, and serialization.
 *
 * Standards:
 *   - RFC 6455
 *     https://www.rfc-editor.org/rfc/rfc6455
 *   - IANA WebSocket Extension Name Registry
 *     https://www.iana.org/assignments/websocket
 *
 * Registry:
 *   IANA WebSocket Extension Name Registry
 *   Registry synchronized: 2026-09-26
 *
 * Rulebook:
 *   stdlib/net/websocket/std-net-websocket.md
 */
```

## 6.5 `compression.sec`

```sec
module websocket

/*
 * Sec Standard Library - net/websocket - Per-message DEFLATE negotiation
 *
 * File:        stdlib/net/websocket/compression.sec
 * Module:      websocket
 * Revision:    1
 * Updated:     2026-09-26
 * Author:      Jonas Engström
 *
 * Purpose:
 *   RFC 7692 permessage-deflate options, negotiation, and negotiated state.
 *
 * Standards:
 *   - RFC 7692
 *     https://www.rfc-editor.org/rfc/rfc7692
 *
 * Rulebook:
 *   stdlib/net/websocket/std-net-websocket.md
 */
```

## 6.6 `close.sec`

```sec
module websocket

/*
 * Sec Standard Library - net/websocket - WebSocket close codes and close metadata
 *
 * File:        stdlib/net/websocket/close.sec
 * Module:      websocket
 * Revision:    1
 * Updated:     2026-09-26
 * Author:      Jonas Engström
 *
 * Purpose:
 *   WebSocket close status codes, validation, and received close information.
 *
 * Standards:
 *   - RFC 6455
 *     https://www.rfc-editor.org/rfc/rfc6455
 *   - IANA WebSocket Close Code Number Registry
 *     https://www.iana.org/assignments/websocket
 *
 * Registry:
 *   IANA WebSocket Close Code Number Registry
 *   Registry synchronized: 2026-09-26
 *
 * Rulebook:
 *   stdlib/net/websocket/std-net-websocket.md
 */
```

## 6.7 `message.sec`

```sec
module websocket

/*
 * Sec Standard Library - net/websocket - Application messages and receive events
 *
 * File:        stdlib/net/websocket/message.sec
 * Module:      websocket
 * Revision:    1
 * Updated:     2026-09-26
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Owned text/binary messages and received control/data events.
 *
 * Standards:
 *   - RFC 6455
 *     https://www.rfc-editor.org/rfc/rfc6455
 *
 * Rulebook:
 *   stdlib/net/websocket/std-net-websocket.md
 */
```

## 6.8 `options.sec`

```sec
module websocket

/*
 * Sec Standard Library - net/websocket - Client, server, and connection options
 *
 * File:        stdlib/net/websocket/options.sec
 * Module:      websocket
 * Revision:    1
 * Updated:     2026-09-26
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Resource limits, origin policy, subprotocol policy, and extension options.
 *
 * Standards:
 *   - RFC 6455
 *     https://www.rfc-editor.org/rfc/rfc6455
 *   - RFC 6454
 *     https://www.rfc-editor.org/rfc/rfc6454
 *   - RFC 7692
 *     https://www.rfc-editor.org/rfc/rfc7692
 *
 * Rulebook:
 *   stdlib/net/websocket/std-net-websocket.md
 */
```

## 6.9 `handshake.sec`

```sec
module websocket

/*
 * Sec Standard Library - net/websocket - HTTP/1.1 WebSocket opening handshake
 *
 * File:        stdlib/net/websocket/handshake.sec
 * Module:      websocket
 * Revision:    1
 * Updated:     2026-09-26
 * Author:      Jonas Engström
 *
 * Purpose:
 *   HTTP/1.1 WebSocket handshake parsing, validation, and accept computation.
 *
 * Standards:
 *   - RFC 6455
 *     https://www.rfc-editor.org/rfc/rfc6455
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *   - RFC 9112
 *     https://www.rfc-editor.org/rfc/rfc9112
 *
 * Rulebook:
 *   stdlib/net/websocket/std-net-websocket.md
 */
```

## 6.10 `frame.sec`

```sec
module websocket

/*
 * Sec Standard Library - net/websocket - WebSocket wire framing
 *
 * File:        stdlib/net/websocket/frame.sec
 * Module:      websocket
 * Revision:    1
 * Updated:     2026-09-26
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Private frame encoding, decoding, masking, fragmentation, and RSV validation.
 *
 * Standards:
 *   - RFC 6455
 *     https://www.rfc-editor.org/rfc/rfc6455
 *   - RFC 7692
 *     https://www.rfc-editor.org/rfc/rfc7692
 *
 * Rulebook:
 *   stdlib/net/websocket/std-net-websocket.md
 */
```

## 6.11 `connection.sec`

```sec
module websocket

/*
 * Sec Standard Library - net/websocket - Established WebSocket connections
 *
 * File:        stdlib/net/websocket/connection.sec
 * Module:      websocket
 * Revision:    1
 * Updated:     2026-09-26
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Full-duplex message, control-frame, close, and lifecycle API.
 *
 * Standards:
 *   - RFC 6455
 *     https://www.rfc-editor.org/rfc/rfc6455
 *   - RFC 7692
 *     https://www.rfc-editor.org/rfc/rfc7692
 *
 * Rulebook:
 *   stdlib/net/websocket/std-net-websocket.md
 */
```

## 6.12 `client.sec`

```sec
module websocket

/*
 * Sec Standard Library - net/websocket - WebSocket client connection establishment
 *
 * File:        stdlib/net/websocket/client.sec
 * Module:      websocket
 * Revision:    1
 * Updated:     2026-09-26
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Client-side ws/wss target validation and RFC 6455 HTTP/1.1 handshake.
 *
 * Standards:
 *   - RFC 6455
 *     https://www.rfc-editor.org/rfc/rfc6455
 *   - RFC 8307
 *     https://www.rfc-editor.org/rfc/rfc8307
 *
 * Rulebook:
 *   stdlib/net/websocket/std-net-websocket.md
 */
```

## 6.13 `server.sec`

```sec
module websocket

/*
 * Sec Standard Library - net/websocket - Standalone WebSocket listener and pending handshakes
 *
 * File:        stdlib/net/websocket/server.sec
 * Module:      websocket
 * Revision:    1
 * Updated:     2026-09-26
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Standalone HTTP/1.1 WebSocket listening, handshake inspection, acceptance, and rejection.
 *
 * Standards:
 *   - RFC 6455
 *     https://www.rfc-editor.org/rfc/rfc6455
 *   - RFC 6454
 *     https://www.rfc-editor.org/rfc/rfc6454
 *   - RFC 9112
 *     https://www.rfc-editor.org/rfc/rfc9112
 *
 * Rulebook:
 *   stdlib/net/websocket/std-net-websocket.md
 */
```

---

# 7. `error.sec`

The package uses separate error families so callers can distinguish
configuration failure, opening-handshake failure, established-protocol failure,
and listener lifecycle failure without string inspection.

```sec
/**
 * Reports invalid user-provided WebSocket configuration.
 */
type ConfigError union error {
    /**
     * A byte or message limit is zero where a positive limit is required.
     */
    InvalidLimit,

    /**
     * The same subprotocol occurs more than once in one configured list.
     */
    DuplicateSubprotocol(Subprotocol),

    /**
     * An additional client or server header attempts to replace a
     * WebSocket-controlled handshake field.
     */
    ReservedHeader(http.HeaderName),

    /**
     * Per-message DEFLATE configuration is internally inconsistent.
     */
    InvalidCompression,

    /**
     * An origin allow-list is empty where the selected policy requires entries.
     */
    EmptyOriginAllowList,
}

/**
 * Reports failure while establishing or rejecting a WebSocket opening handshake.
 */
type HandshakeError union error {
    /**
     * The target URL cannot be used as a WebSocket target.
     */
    InvalidTarget,

    /**
     * The target scheme is neither ws nor wss.
     */
    UnsupportedScheme(string),

    /**
     * The selected target requires TLS but the current target has no configured
     * WebSocket TLS capability.
     */
    TlsUnavailable,

    /**
     * The HTTP opening handshake exceeds the configured byte limit.
     */
    HeaderTooLarge,

    /**
     * The HTTP request or response is syntactically invalid.
     */
    InvalidHttp,

    /**
     * A required WebSocket handshake field is absent.
     */
    MissingHeader(http.HeaderName),

    /**
     * A WebSocket handshake field is present but invalid.
     */
    InvalidHeader(http.HeaderName),

    /**
     * The peer requested a WebSocket version unsupported by this implementation.
     */
    UnsupportedVersion(Version),

    /**
     * The server response status does not establish a WebSocket connection.
     */
    Rejected(http.Status),

    /**
     * Sec-WebSocket-Accept does not match the client nonce.
     */
    InvalidAccept,

    /**
     * The server selected a subprotocol that the client did not offer.
     */
    InvalidSubprotocol(Subprotocol),

    /**
     * Extension negotiation is invalid or selects an extension that was not offered.
     */
    InvalidExtension,

    /**
     * The request origin is rejected by server policy.
     */
    OriginRejected,

    /**
     * User configuration is invalid.
     */
    Configuration(ConfigError),

    /**
     * An underlying network, HTTP, TLS, allocation, or platform operation failed.
     */
    Underlying(error),
}

/**
 * Reports a WebSocket wire-protocol violation after the opening handshake.
 */
type ProtocolError union error {
    /**
     * One or more RSV bits are set without a negotiated extension that owns them.
     */
    ReservedBits,

    /**
     * A received opcode is unassigned or unsupported.
     */
    InvalidOpcode(uint8),

    /**
     * A client-to-server frame is not masked.
     */
    ClientFrameNotMasked,

    /**
     * A server-to-client frame is masked.
     */
    ServerFrameMasked,

    /**
     * A control frame is fragmented.
     */
    FragmentedControlFrame,

    /**
     * A control frame payload exceeds 125 bytes.
     */
    ControlPayloadTooLarge,

    /**
     * A continuation frame is received while no fragmented message is active.
     */
    UnexpectedContinuation,

    /**
     * A new text or binary data frame is received while a fragmented message is active.
     */
    UnexpectedDataFrame,

    /**
     * A payload length uses a non-minimal wire encoding or an invalid 64-bit value.
     */
    InvalidPayloadLength,

    /**
     * A close frame has an invalid payload shape.
     */
    InvalidClosePayload,

    /**
     * A close status code is forbidden by the WebSocket wire protocol.
     */
    InvalidCloseCode(uint16),

    /**
     * Text message or close-reason data is not valid UTF-8.
     *
     * The payload is the byte offset at which validation failed.
     */
    InvalidUtf8(uint64),

    /**
     * Per-message DEFLATE framing or compressed payload is invalid.
     */
    InvalidCompression,
}

/**
 * Reports failure while using an established WebSocket connection.
 */
type ConnectionError union error {
    /**
     * The peer violated the WebSocket protocol.
     */
    Protocol(ProtocolError),

    /**
     * An incoming frame exceeds MaxFramePayloadBytes.
     */
    FrameTooLarge(uint64),

    /**
     * An assembled or decompressed message exceeds MaxMessageBytes.
     */
    MessageTooLarge(uint64),

    /**
     * The operation requires an open connection but the connection is closed.
     */
    Closed,

    /**
     * A DEFLATE implementation operation failed.
     */
    Compression(error),

    /**
     * An underlying network, TLS, allocation, or platform operation failed.
     */
    Underlying(error),
}

/**
 * Reports standalone WebSocket listener failures.
 */
type ListenError union error {
    /**
     * Listener configuration is invalid.
     */
    Configuration(ConfigError),

    /**
     * The listener is already closed.
     */
    Closed,

    /**
     * An underlying bind, listen, accept, allocation, or platform operation failed.
     */
    Underlying(error),
}
```

## Representation rationale

All four declarations are `union error` types because callers need a closed
error family while several alternatives carry typed context or an underlying
cause.

`ProtocolError.InvalidUtf8` carries a `uint64` **byte offset**. Byte offsets in
this API therefore have an explicit, portable width rather than depending on a
platform-sized integer convention.

---

# 8. `version.sec`

WebSocket version numbers form an IANA-managed numeric namespace. The namespace
is not modeled as an enum because future standard versions may be registered.

```sec
/**
 * Represents a WebSocket protocol version number.
 *
 * Standards:
 *   - RFC 6455 section 4.4.
 *   - IANA WebSocket Version Number Registry.
 */
type Version uint8

impl Version {
    /**
     * Identifies the WebSocket protocol version standardized by RFC 6455.
     */
    static let RFC6455: Version := 13

    /**
     * Parses one decimal WebSocket version number.
     *
     * Parameters:
     *   - value: Decimal ASCII representation without surrounding whitespace.
     *
     * Returns:
     *   The represented version number.
     *
     * Errors:
     *   - HandshakeError.InvalidHeader: The value is empty, non-decimal,
     *     negative, signed, or outside uint8.
     *
     * Standards:
     *   - RFC 6455 section 4.4.
     */
    static fn Parse(value: string) Result[Version, HandshakeError]

    /**
     * Returns the canonical decimal representation.
     */
    fn ToString() string

    /**
     * Reports whether this library revision can establish the represented version.
     */
    property IsSupported: bool

    /**
     * Reports whether the value appears in the synchronized IANA version registry.
     */
    property IsRegistered: bool
}
```

`IsSupported` is true only for version 13 in revision 0.1.

`IsRegistered` is true for the IANA entries 0 through 13, including the
historical interim and reserved entries in the registry snapshot. Registry
presence does not imply implementation support.

## Representation rationale

`Version` is a nominal `uint8`, not an enum, because protocol version numbers
are registry-managed and extensible. The nominal type prevents unrelated
integers from being passed accidentally while retaining forward
representability.

---

# 9. `subprotocol.sec`

WebSocket subprotocol identifiers are case-sensitive tokens. The IANA registry
is open and application-oriented, so revision 0.1 does not mirror its large and
changing set as static constants.

```sec
/**
 * Represents one syntactically valid WebSocket subprotocol identifier.
 *
 * Standards:
 *   - RFC 6455 section 4.
 *   - RFC 7936.
 */
type Subprotocol string

impl Subprotocol {
    /**
     * Parses and validates a WebSocket subprotocol identifier.
     *
     * Parameters:
     *   - value: Candidate identifier.
     *
     * Returns:
     *   A Subprotocol preserving the exact supplied case.
     *
     * Errors:
     *   - HandshakeError.InvalidHeader: The value is empty or violates the
     *     RFC 6455 token grammar.
     *
     * Standards:
     *   - RFC 6455 section 4.1.
     */
    static fn Parse(value: string) Result[Subprotocol, HandshakeError]

    /**
     * Returns the exact validated identifier.
     */
    fn ToString() string
}
```

Subprotocol equality is case-sensitive.

A client offer list and a server preference list must not contain the same
identifier more than once.

RFC 7936 prevents new IANA registrations that differ from existing
registrations only by case, but the WebSocket negotiation value itself remains
case-sensitive as defined by RFC 6455.

## Representation rationale

`Subprotocol` is a nominal string rather than an enum because the namespace is
open, registry-managed, and explicitly extensible by application protocols.

---

# 10. `extension.sec`

## 10.1 `ExtensionName`

```sec
/**
 * Represents one syntactically valid WebSocket extension identifier.
 */
type ExtensionName string

impl ExtensionName {
    /**
     * Identifies RFC 7692 per-message DEFLATE.
     */
    static let PerMessageDeflate: ExtensionName := "permessage-deflate"

    /**
     * Identifies the IANA-registered Broadband Forum USP extension.
     */
    static let BbfUspProtocol: ExtensionName := "bbf-usp-protocol"

    /**
     * Parses a WebSocket extension identifier.
     *
     * Parameters:
     *   - value: Candidate extension token.
     *
     * Returns:
     *   The validated identifier with exact case preserved.
     *
     * Errors:
     *   - HandshakeError.InvalidExtension: The identifier violates token syntax.
     */
    static fn Parse(value: string) Result[ExtensionName, HandshakeError]

    /**
     * Returns the exact validated identifier.
     */
    fn ToString() string

    /**
     * Reports whether the identifier occurs in the synchronized IANA registry.
     */
    property IsRegistered: bool

    /**
     * Reports whether revision 0.1 implements the extension semantics.
     */
    property IsImplemented: bool
}
```

The two static values are the complete WebSocket Extension Name Registry
snapshot at synchronization time.

`IsImplemented` is true only for `PerMessageDeflate` in revision 0.1.

Extension identifiers are compared case-sensitively.

## 10.2 Extension parameters

```sec
/**
 * Represents a WebSocket extension parameter name.
 */
type ExtensionParameterName string

impl ExtensionParameterName {
    /**
     * Parses one extension parameter token.
     */
    static fn Parse(value: string) Result[ExtensionParameterName, HandshakeError]

    /**
     * Returns the validated token.
     */
    fn ToString() string
}

/**
 * Represents one WebSocket extension parameter.
 */
type ExtensionParameter struct {
    /**
     * Identifies the parameter.
     */
    Name: ExtensionParameterName,

    /**
     * Contains the decoded parameter value when a value was present.
     */
    Value: Option[string],
}

/**
 * Represents one extension offer or negotiated extension value.
 */
type Extension struct {
    /**
     * Identifies the extension.
     */
    Name: ExtensionName,

    /**
     * Contains the parameters in received or configured order.
     */
    Parameters: ExtensionParameter[],
}

/**
 * Parses a Sec-WebSocket-Extensions field value.
 *
 * Parameters:
 *   - value: One validated HTTP field value.
 *
 * Returns:
 *   Newly allocated extensions preserving extension and parameter order.
 *
 * Errors:
 *   - HandshakeError.InvalidExtension: The grammar, quoting, escaping, or token
 *     syntax is invalid.
 *
 * Ownership:
 *   The returned array and contained strings are owned by the caller.
 *
 * Standards:
 *   - RFC 6455 section 9.1.
 */
fn ParseExtensions(value: http.HeaderValue) Result[Extension[], HandshakeError]

/**
 * Serializes extension values into one canonical handshake field value.
 *
 * Parameters:
 *   - extensions: Borrowed extension values.
 *
 * Returns:
 *   A newly allocated validated HTTP field value.
 *
 * Errors:
 *   - HandshakeError.InvalidExtension: A value cannot be represented legally.
 *
 * Ownership:
 *   extensions is borrowed and remains owned by the caller.
 */
fn SerializeExtensions(extensions: ref Extension[]) Result[http.HeaderValue, HandshakeError]
```

Parsing removes HTTP quoted-string delimiters and unescapes quoted-pair syntax
before storing a parameter value.

Serialization uses token form when the value is a legal token; otherwise it
uses quoted-string with required escaping.

Duplicate parameter names within one extension occurrence are rejected by the
generic parser. RFC-specific negotiators may impose stronger restrictions.

## Representation rationale

The three name/value abstractions are structs or nominal strings because
extension syntax is an open registry with simultaneous name and parameter
data. `Option[string]` distinguishes a valueless parameter from a parameter
whose value is present.

---

# 11. `compression.sec`

Revision 0.1 implements only the RFC 7692 `permessage-deflate` extension.

## 11.1 `WindowBits`

```sec
/**
 * Represents an RFC 7692 DEFLATE window size exponent.
 */
type WindowBits uint8 range 8..15 default 15
```

The default value 15 is the DEFLATE maximum window size and is a valid
protocol default. This ranged type therefore has a meaningful Sec default.

## 11.2 Client offer options

```sec
/**
 * Configures the client's permessage-deflate offer.
 */
type PerMessageDeflateClientOptions struct {
    /**
     * Enables offering permessage-deflate.
     */
    Enabled: bool,

    /**
     * Requests server_no_context_takeover.
     */
    ServerNoContextTakeover: bool,

    /**
     * Requests client_no_context_takeover.
     */
    ClientNoContextTakeover: bool,

    /**
     * Controls whether client_max_window_bits is offered.
     */
    OfferClientMaxWindowBits: bool,

    /**
     * Provides a maximum client window when the parameter is offered with a value.
     * None with OfferClientMaxWindowBits=true emits the parameter without a value.
     */
    ClientMaxWindowBits: Option[WindowBits],

    /**
     * Provides the maximum server window requested by the client.
     * None omits server_max_window_bits.
     */
    ServerMaxWindowBits: Option[WindowBits],
}

impl PerMessageDeflateClientOptions {
    init() {
        self.Enabled = true
        self.ServerNoContextTakeover = false
        self.ClientNoContextTakeover = false
        self.OfferClientMaxWindowBits = true
        self.ClientMaxWindowBits = None
        self.ServerMaxWindowBits = None
    }

    /**
     * Validates the RFC 7692 offer configuration.
     */
    fn Validate() Result[void, ConfigError]
}
```

`ClientMaxWindowBits=Some(x)` requires
`OfferClientMaxWindowBits=true`.

## 11.3 Server negotiation options

```sec
/**
 * Configures server-side permessage-deflate negotiation.
 */
type PerMessageDeflateServerOptions struct {
    /**
     * Enables accepting permessage-deflate.
     */
    Enabled: bool,

    /**
     * Allows accepting client_no_context_takeover.
     */
    AllowClientNoContextTakeover: bool,

    /**
     * Allows selecting server_no_context_takeover.
     */
    AllowServerNoContextTakeover: bool,

    /**
     * Limits the selected client window size when the client offer permits it.
     */
    ClientMaxWindowBits: Option[WindowBits],

    /**
     * Limits the selected server window size when the client offer permits it.
     */
    ServerMaxWindowBits: Option[WindowBits],
}

impl PerMessageDeflateServerOptions {
    init() {
        self.Enabled = true
        self.AllowClientNoContextTakeover = true
        self.AllowServerNoContextTakeover = true
        self.ClientMaxWindowBits = None
        self.ServerMaxWindowBits = None
    }

    /**
     * Validates server-side compression policy.
     */
    fn Validate() Result[void, ConfigError]
}
```

## 11.4 Negotiated state

```sec
/**
 * Describes the exact negotiated permessage-deflate parameters.
 */
type PerMessageDeflate struct {
    /**
     * True when the client resets its compression context after every message.
     */
    ClientNoContextTakeover: bool,

    /**
     * True when the server resets its compression context after every message.
     */
    ServerNoContextTakeover: bool,

    /**
     * Negotiated client-to-server LZ77 window size exponent.
     */
    ClientMaxWindowBits: WindowBits,

    /**
     * Negotiated server-to-client LZ77 window size exponent.
     */
    ServerMaxWindowBits: WindowBits,
}
```

An absent window-size parameter resolves to 15 for the corresponding direction.

The negotiated value is never constructed from unvalidated response parameters.

## 11.5 Compression wire semantics

For an outbound compressed data message:

1. The sender compresses the complete logical message as one RFC 7692 message.
2. RSV1 is set on the first text/binary frame only.
3. RSV1 is clear on continuation frames.
4. Control frames are never compressed by this extension.
5. The DEFLATE message trailer required by the RFC 7692 algorithm is handled
   exactly as specified by RFC 7692; it is not exposed to application payloads.
6. Context takeover is reset according to the negotiated direction-specific
   flag.
7. Fragmentation occurs at the WebSocket frame layer and does not create new
   compression contexts.

For inbound compressed messages, `MaxMessageBytes` applies to the
**decompressed** application message size. A small compressed message therefore
cannot bypass the configured message limit.

## Representation rationale

The two option types are structs because independent negotiation switches and
limits coexist. `PerMessageDeflate` is a separate struct because configured
preferences are not the same thing as the exact negotiated state.

---

# 12. `close.sec`

## 12.1 `CloseCode`

Close codes are an extensible numeric namespace, so they are not an enum.

```sec
/**
 * Represents a WebSocket close status code.
 */
type CloseCode uint16

impl CloseCode {
    /**
     * 1000: Normal Closure.
     */
    static let NormalClosure: CloseCode := 1000

    /**
     * 1001: Going Away.
     */
    static let GoingAway: CloseCode := 1001

    /**
     * 1002: Protocol Error.
     */
    static let ProtocolError: CloseCode := 1002

    /**
     * 1003: Unsupported Data.
     */
    static let UnsupportedData: CloseCode := 1003

    /**
     * 1004: Reserved. This value must not be sent.
     */
    static let Reserved1004: CloseCode := 1004

    /**
     * 1005: No Status Received. This synthetic value must not be sent.
     */
    static let NoStatusReceived: CloseCode := 1005

    /**
     * 1006: Abnormal Closure. This synthetic value must not be sent.
     */
    static let AbnormalClosure: CloseCode := 1006

    /**
     * 1007: Invalid frame payload data.
     */
    static let InvalidPayloadData: CloseCode := 1007

    /**
     * 1008: Policy Violation.
     */
    static let PolicyViolation: CloseCode := 1008

    /**
     * 1009: Message Too Big.
     */
    static let MessageTooBig: CloseCode := 1009

    /**
     * 1010: Mandatory Extension.
     */
    static let MandatoryExtension: CloseCode := 1010

    /**
     * 1011: Internal Error.
     */
    static let InternalError: CloseCode := 1011

    /**
     * 1012: Service Restart.
     */
    static let ServiceRestart: CloseCode := 1012

    /**
     * 1013: Try Again Later.
     */
    static let TryAgainLater: CloseCode := 1013

    /**
     * 1014: Bad Gateway / invalid upstream response.
     */
    static let BadGateway: CloseCode := 1014

    /**
     * 1015: TLS Handshake. This synthetic value must not be sent.
     */
    static let TlsHandshake: CloseCode := 1015

    /**
     * 3000: IANA-registered Unauthorized application/library code.
     */
    static let Unauthorized: CloseCode := 3000

    /**
     * 3003: IANA-registered Forbidden application/library code.
     */
    static let Forbidden: CloseCode := 3003

    /**
     * 3008: IANA-registered Timeout application/library code.
     */
    static let Timeout: CloseCode := 3008

    /**
     * Reports whether this value is legal in an on-wire Close frame.
     */
    property IsWireValid: bool

    /**
     * Reports whether this value is assigned in the synchronized IANA registry.
     */
    property IsRegistered: bool

    /**
     * Reports whether the value is in the application/library range 3000..4999.
     */
    property IsApplicationDefined: bool

    /**
     * Reports whether the value is in the private-use range 4000..4999.
     */
    property IsPrivateUse: bool

    /**
     * Returns the decimal status-code representation.
     */
    fn ToString() string
}
```

The static constants above are the complete individually assigned IANA
close-code entries at synchronization time.

`IsWireValid` is false for:

- values below 1000;
- values above 4999;
- 1004;
- 1005;
- 1006;
- 1015.

Unknown values in otherwise protocol-valid ranges remain representable so future
standards and registered application codes are not rejected solely because this
library snapshot predates them.

`IsRegistered` reflects the synchronized IANA snapshot and therefore is not a
permission check.

## 12.2 `CloseInfo`

```sec
/**
 * Represents the close information carried by a received or locally prepared
 * WebSocket Close frame.
 */
type CloseInfo struct {
    /**
     * Contains the close code when the frame carried one.
     */
    Code: Option[CloseCode],

    /**
     * Contains the UTF-8 close reason without the two-byte status code.
     */
    Reason: string,
}

impl CloseInfo {
    init(code: CloseCode) {
        self.Code = Some(code)
        self.Reason = ""
    }

    init(code: CloseCode, reason: string) {
        self.Code = Some(code)
        self.Reason = reason
    }

    /**
     * Validates this value for transmission in a Close frame.
     *
     * Returns:
     *   Ok when the encoded close payload is legal.
     *
     * Errors:
     *   - ProtocolError.InvalidCloseCode: Code exists but is forbidden on wire.
     *   - ProtocolError.InvalidClosePayload: The UTF-8 encoded payload exceeds
     *     the 125-byte control-frame limit.
     *
     * Standards:
     *   - RFC 6455 sections 5.5.1 and 7.4.
     */
    fn ValidateForSend() Result[void, ProtocolError]
}
```

A received Close frame with an empty payload is represented by:

```text
Code   = None
Reason = ""
```

A Close frame payload of exactly one byte is invalid.

When a code is present, the reason begins after the two-byte network-order code
and must be valid UTF-8.

## Representation rationale

`CloseCode` is a nominal integer because the close-code namespace is extensible.
`CloseInfo` is a struct because code presence and reason text are simultaneous
parts of one close event.

---

# 13. `message.sec`

## 13.1 Message kind

```sec
/**
 * Identifies the semantic kind of an application WebSocket message.
 */
enum MessageType {
    /**
     * UTF-8 text message.
     */
    Text,

    /**
     * Opaque binary message.
     */
    Binary,
}
```

## 13.2 Owned messages

```sec
/**
 * Represents one complete application WebSocket message.
 */
type Message union {
    /**
     * Owns a validated Sec string decoded from one complete text message.
     */
    Text(string),

    /**
     * Owns the exact bytes of one complete binary message.
     */
    Binary(byte[]),
}

impl Message {
    /**
     * Returns the semantic message type.
     */
    property Type: MessageType
}
```

A `Text` value is created only after the complete logical message has passed
UTF-8 validation across all fragments.

A fragmented text message is not validated independently per frame because a
UTF-8 sequence may span frame boundaries.

## 13.3 Receive results and events

```sec
/**
 * Represents the ordinary high-level result of receiving from a WebSocket.
 */
type ReceiveResult union {
    /**
     * A complete application message was received.
     */
    Message(Message),

    /**
     * The peer completed the WebSocket closing handshake.
     */
    Closed(CloseInfo),
}

/**
 * Represents one application or control event from the receive path.
 */
type Event union {
    /**
     * A complete application message was received.
     */
    Message(Message),

    /**
     * A Ping control frame was received.
     */
    Ping(byte[]),

    /**
     * A Pong control frame was received.
     */
    Pong(byte[]),

    /**
     * A Close control frame was received.
     */
    Close(CloseInfo),
}
```

Ping and Pong payload arrays contain at most 125 bytes.

`ReceiveResult` exists so an orderly peer close is not misrepresented as an
ordinary error.

`Event` exists for applications that need explicit control-frame visibility.

## Representation rationale

`MessageType` is an enum because RFC 6455 defines the two application message
kinds as a closed set.

`Message`, `ReceiveResult`, and `Event` are unions because exactly one payload
shape is present at a time.

---

# 14. `options.sec`

## 14.1 Connection resource limits

```sec
/**
 * Configures established-connection behavior and resource limits.
 */
type ConnectionOptions struct {
    /**
     * Maximum accepted payload length of one incoming WebSocket frame.
     */
    MaxFramePayloadBytes: uint64,

    /**
     * Maximum accepted complete application message size after decompression.
     */
    MaxMessageBytes: uint64,

    /**
     * Automatically sends Pong with an identical application payload when Ping
     * is received.
     */
    AutoPong: bool,
}

impl ConnectionOptions {
    init() {
        self.MaxFramePayloadBytes = 16777216
        self.MaxMessageBytes = 16777216
        self.AutoPong = true
    }

    /**
     * Validates connection limits.
     */
    fn Validate() Result[void, ConfigError]
}
```

Both byte limits must be greater than zero.

The defaults are 16 MiB. They are deliberate denial-of-service bounds, not
protocol limits.

## 14.2 Origin values and policy

```sec
/**
 * Represents one RFC 6454 serialized origin value used by the WebSocket Origin
 * request field.
 */
type Origin string

impl Origin {
    /**
     * Parses a serialized origin value.
     */
    static fn Parse(value: string) Result[Origin, HandshakeError]

    /**
     * Returns the canonical serialized origin.
     */
    fn ToString() string
}

/**
 * Defines server policy for the Origin request field.
 */
type OriginPolicy union {
    /**
     * Accepts any syntactically valid Origin value and also accepts absence.
     */
    Any,

    /**
     * Accepts no Origin field or an Origin matching the WebSocket target's
     * corresponding HTTP(S) origin.
     */
    SameOrigin,

    /**
     * Requires an Origin field matching the WebSocket target's corresponding
     * HTTP(S) origin.
     */
    RequireSameOrigin,

    /**
     * Accepts no Origin field or an exact member of the allow-list.
     */
    AllowList(Origin[]),

    /**
     * Requires an Origin field that is an exact member of the allow-list.
     */
    RequireAllowList(Origin[]),
}
```

The protocol text `"null"` defined by the web origin model is ordinary protocol
text; it does not introduce a Sec `null` value.

Origin comparison uses normalized scheme, host, and effective port semantics,
not naive raw-string comparison.

For same-origin comparison:

- `ws` corresponds to an `http` origin;
- `wss` corresponds to an `https` origin;
- default ports are normalized before comparison.

## 14.3 Client options

```sec
/**
 * Configures a client opening handshake.
 */
type ClientOptions struct {
    /**
     * Additional application-controlled HTTP request fields.
     */
    Header: http.Header,

    /**
     * Subprotocols offered in client preference order.
     */
    Subprotocols: Subprotocol[],

    /**
     * Optional Origin request field.
     */
    Origin: Option[Origin],

    /**
     * Per-message DEFLATE offer policy.
     */
    Compression: PerMessageDeflateClientOptions,

    /**
     * Established-connection limits and control behavior.
     */
    Connection: ConnectionOptions,
}

impl ClientOptions {
    init() {
        self.Header = new http.Header()
        self.Subprotocols = []
        self.Origin = None
        self.Compression = new PerMessageDeflateClientOptions()
        self.Connection = new ConnectionOptions()
    }

    /**
     * Validates the complete client configuration.
     */
    fn Validate() Result[void, ConfigError]
}
```

`Header` must not set any of these WebSocket-controlled fields:

```text
Connection
Host
Upgrade
Sec-WebSocket-Key
Sec-WebSocket-Version
Sec-WebSocket-Protocol
Sec-WebSocket-Extensions
```

The implementation owns generation of those values.

## 14.4 Server options

```sec
/**
 * Configures a standalone WebSocket listener.
 */
type ServerOptions struct {
    /**
     * Additional fields emitted on successful 101 responses.
     */
    Header: http.Header,

    /**
     * Supported subprotocols in server preference order.
     */
    Subprotocols: Subprotocol[],

    /**
     * Origin validation policy.
     */
    Origin: OriginPolicy,

    /**
     * Server-side per-message DEFLATE negotiation policy.
     */
    Compression: PerMessageDeflateServerOptions,

    /**
     * Maximum bytes accepted for one HTTP opening-handshake header section.
     */
    MaxHandshakeBytes: uint,

    /**
     * Established-connection limits and control behavior.
     */
    Connection: ConnectionOptions,
}

impl ServerOptions {
    init() {
        self.Header = new http.Header()
        self.Subprotocols = []
        self.Origin = OriginPolicy.SameOrigin
        self.Compression = new PerMessageDeflateServerOptions()
        self.MaxHandshakeBytes = 65536
        self.Connection = new ConnectionOptions()
    }

    /**
     * Validates the complete server configuration.
     */
    fn Validate() Result[void, ConfigError]
}
```

`Header` must not set:

```text
Connection
Upgrade
Sec-WebSocket-Accept
Sec-WebSocket-Protocol
Sec-WebSocket-Extensions
```

`MaxHandshakeBytes` must be greater than zero.

Subprotocol lists are checked for exact duplicate identifiers.

## Representation rationale

The option declarations are structs because their settings coexist.

`OriginPolicy` is a union because exactly one origin policy is active and two
policy forms carry an allow-list payload.

`Origin` is a nominal string because serialized web origins have defined
syntax and comparison semantics but remain textual and extensible.

---

# 15. `handshake.sec`

## 15.1 WebSocket key and accept values

The HTTP/1.1 client opening handshake generates a fresh unpredictable 16-byte
nonce for every connection attempt.

It Base64-encodes those 16 bytes into `Sec-WebSocket-Key`.

The server computes:

```text
Base64(
    SHA1(
        Sec-WebSocket-Key-as-ASCII
        +
        "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
    )
)
```

The fixed GUID is protocol syntax and must match RFC 6455 exactly.

The server sends the result in `Sec-WebSocket-Accept`.

The client independently computes the expected accept value from the exact key
it sent and rejects a mismatch.

The package exposes the pure accept computation for testability and compatible
server integrations:

```sec
/**
 * Computes the RFC 6455 Sec-WebSocket-Accept value for one validated client key.
 *
 * Parameters:
 *   - key: Exact Base64 Sec-WebSocket-Key field value.
 *
 * Returns:
 *   The Base64 SHA-1 accept value.
 *
 * Errors:
 *   - HandshakeError.InvalidHeader: key is not canonical Base64 encoding of
 *     exactly 16 bytes.
 *   - HandshakeError.Underlying: Allocation or encoding support failed.
 *
 * Standards:
 *   - RFC 6455 section 4.2.2.
 */
fn ComputeAccept(key: http.HeaderValue) Result[http.HeaderValue, HandshakeError]
```

## 15.2 HTTP/1.1 client request

For target:

```text
ws://example.com/chat?room=1
```

the request uses:

```text
GET /chat?room=1 HTTP/1.1
Host: example.com
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Key: <fresh-base64-nonce>
Sec-WebSocket-Version: 13
```

Optional fields are added for:

- Origin;
- Sec-WebSocket-Protocol;
- Sec-WebSocket-Extensions;
- caller-provided non-reserved HTTP fields.

For `wss`, TLS is established before this HTTP request is sent.

The URI fragment is never transmitted in the request target. A client target
containing a fragment is rejected as `HandshakeError.InvalidTarget`.

User information in a WebSocket URL is rejected by revision 0.1 rather than
silently converting it to authorization credentials.

The request method is exactly GET.

## 15.3 HTTP/1.1 server validation

A server candidate request is accepted as a WebSocket opening handshake only
when all of these conditions hold:

1. The request is HTTP/1.1 or later in HTTP/1.x form.
2. The method is GET.
3. Host is valid for HTTP/1.1.
4. Upgrade contains the case-insensitive token `websocket`.
5. Connection contains the case-insensitive token `Upgrade`.
6. Sec-WebSocket-Key occurs exactly once and decodes from Base64 to exactly
   16 bytes.
7. Sec-WebSocket-Version includes version 13.
8. Sec-WebSocket-Protocol, when present, parses as a comma-separated list of
   valid non-empty subprotocol tokens with no duplicates.
9. Sec-WebSocket-Extensions, when present, parses according to RFC 6455.
10. Origin, when present, parses as an RFC 6454 serialized origin.
11. Server origin policy accepts the request.
12. The complete handshake stays within `MaxHandshakeBytes`.

Header-name comparison follows HTTP rules.

Token-list comparison for `Connection` and `Upgrade` is ASCII
case-insensitive.

Subprotocol identifiers remain case-sensitive.

## 15.4 Successful server response

Successful revision 0.1 acceptance emits:

```text
HTTP/1.1 101 Switching Protocols
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Accept: <computed-value>
```

The response includes exactly one `Sec-WebSocket-Protocol` field when a
subprotocol was selected.

It includes a `Sec-WebSocket-Extensions` field when one or more extensions were
successfully negotiated.

A server must not select:

- a subprotocol the client did not offer;
- an extension the client did not offer;
- extension parameters that violate the client's offer.

## 15.5 Subprotocol selection

Automatic server selection uses this exact algorithm:

1. Iterate `ServerOptions.Subprotocols` in server preference order.
2. Select the first configured subprotocol that occurs exactly in the client's
   offer list.
3. If no configured value matches, select no subprotocol.

The server does not fail merely because no subprotocol is selected.

An application that requires a subprotocol can inspect a `PendingConnection`
and reject the handshake before acceptance.

## 15.6 Rejection

Before a 101 response is committed, the standalone server may reject a pending
handshake with an ordinary HTTP error response.

After 101 is committed, HTTP response semantics end and errors are handled by
WebSocket close/connection semantics.

---

# 16. `frame.sec`

`frame.sec` has no public declarations in revision 0.1.

The implementation is nevertheless normative.

## 16.1 Frame header

The decoder processes these fields in order:

```text
FIN  RSV1 RSV2 RSV3 OPCODE
MASK PAYLOAD-LEN
EXTENDED-PAYLOAD-LEN
MASKING-KEY
APPLICATION-DATA
```

The opcode values defined by RFC 6455 are:

| Opcode | Meaning |
|---:|---|
| 0x0 | continuation |
| 0x1 | text |
| 0x2 | binary |
| 0x8 | close |
| 0x9 | ping |
| 0xA | pong |

Values 0x3..0x7 and 0xB..0xF are unassigned in the synchronized IANA registry.

Revision 0.1 does not claim an extension that owns those opcodes. Receiving one
therefore fails the connection with `ProtocolError.InvalidOpcode`.

## 16.2 Payload length

Payload length encoding is:

- 0..125: encoded directly in the 7-bit length field;
- 126..65535: marker 126 plus an unsigned 16-bit network-order length;
- 65536 and above: marker 127 plus an unsigned 64-bit network-order length.

The most significant bit of the 64-bit representation must be zero.

The shortest available encoding must be used.

A received non-minimal encoding is `ProtocolError.InvalidPayloadLength`.

Before allocating payload storage, the decoder compares the declared frame
payload length with `MaxFramePayloadBytes`.

## 16.3 Masking

Every client-to-server frame is masked.

Every server-to-client frame is unmasked.

A client generates a fresh unpredictable 32-bit masking key independently for
each frame.

Masking is:

```text
transformed[i] = source[i] XOR key[i mod 4]
```

Masking is not encryption and must not be treated as cryptographic
confidentiality.

A server receiving an unmasked client frame fails with
`ClientFrameNotMasked`.

A client receiving a masked server frame fails with `ServerFrameMasked`.

## 16.4 Control frames

Close, Ping, and Pong are control frames.

Every control frame:

- has FIN set;
- is never fragmented;
- has payload length at most 125;
- may appear between fragments of a fragmented data message;
- is never compressed by permessage-deflate.

A violation produces the corresponding `ProtocolError`.

## 16.5 Fragmentation

A non-control message starts with Text or Binary opcode.

When FIN is false, the message remains open and later fragments use Continuation
opcode.

While a fragmented message is open:

- another Text frame is invalid;
- another Binary frame is invalid;
- Continuation frames append to the current message;
- control frames may be processed between continuations.

The final Continuation frame sets FIN.

Message-size accounting accumulates the complete logical message and, when
compression is active, applies the configured limit to the decompressed result.

## 16.6 UTF-8

Text messages must be valid UTF-8 as complete messages.

Close reasons must be valid UTF-8.

The decoder may validate incrementally, but it must carry partial UTF-8 state
across fragments.

A malformed sequence fails with `ProtocolError.InvalidUtf8(byteOffset)`.

## 16.7 RSV bits

Without negotiated extensions, RSV1, RSV2, and RSV3 must all be zero.

With `permessage-deflate`:

- RSV1 may be set only on the first data frame of a compressed message;
- RSV1 is clear on its continuation frames;
- RSV1 is clear on control frames;
- RSV2 and RSV3 remain zero.

No other RSV usage is implemented in revision 0.1.

---

# 17. `connection.sec`

## 17.1 Role and state

```sec
/**
 * Identifies which endpoint role this connection performs.
 */
enum Role {
    /**
     * This endpoint initiated the WebSocket opening handshake.
     */
    Client,

    /**
     * This endpoint accepted the WebSocket opening handshake.
     */
    Server,
}

/**
 * Identifies the public lifecycle state of a WebSocket connection.
 */
enum State {
    /**
     * The opening handshake is in progress.
     */
    Connecting,

    /**
     * Application and control frames may be exchanged.
     */
    Open,

    /**
     * A close handshake has started but transport shutdown is not complete.
     */
    Closing,

    /**
     * The WebSocket and its underlying transport are closed.
     */
    Closed,
}
```

`Connection` values returned by `Connect` or `PendingConnection.Accept` begin in
`State.Open`. `Connecting` exists because the state model is protocol-defined
and may be surfaced by later asynchronous connection APIs.

## 17.2 Established connection

```sec
/**
 * Owns one established full-duplex WebSocket connection.
 */
@noCopy
type Connection struct {
    _state: _ConnectionState,
}

impl Connection {
    /**
     * Returns the endpoint role.
     */
    property Role: Role

    /**
     * Returns the current connection lifecycle state.
     */
    property State: State

    /**
     * Returns the selected WebSocket subprotocol, if one was negotiated.
     */
    property Subprotocol: Option[Subprotocol]

    /**
     * Returns the negotiated permessage-deflate state when compression is active.
     */
    property Compression: Option[PerMessageDeflate]

    /**
     * Reports whether application data may currently be sent.
     */
    property IsOpen: bool

    /**
     * Sends one complete owned message.
     *
     * Parameters:
     *   - message: Message whose ownership is transferred to the call.
     *
     * Returns:
     *   Ok after the complete message has been written to the WebSocket transport.
     *
     * Errors:
     *   - ConnectionError.Closed: The connection cannot send application data.
     *   - ConnectionError.Underlying: Transport/allocation failure.
     *   - ConnectionError.Compression: Compression failure.
     *
     * Ownership:
     *   message is consumed by the call.
     */
    fn Send(->message: Message) Result[void, ConnectionError]

    /**
     * Sends one complete UTF-8 text message.
     *
     * Parameters:
     *   - text: Borrowed/copyable Sec string value.
     */
    fn SendText(text: string) Result[void, ConnectionError]

    /**
     * Sends one complete binary message without transferring caller ownership.
     *
     * Parameters:
     *   - data: Borrowed binary payload.
     *
     * Ownership:
     *   data remains owned by the caller after the synchronous call returns.
     */
    fn SendBinary(data: ref byte[]) Result[void, ConnectionError]

    /**
     * Receives the next complete application message or orderly close.
     *
     * Description:
     *   Ping and Pong frames are consumed internally. If AutoPong is true,
     *   received Ping frames are answered before receive processing continues.
     *
     * Returns:
     *   A complete message or an orderly close result.
     *
     * Errors:
     *   - ConnectionError.Protocol: Invalid peer framing.
     *   - ConnectionError.FrameTooLarge: A frame exceeds the configured limit.
     *   - ConnectionError.MessageTooLarge: A logical message exceeds the limit.
     *   - ConnectionError.Underlying: Transport/allocation failure.
     */
    fn Receive() Result[ReceiveResult, ConnectionError]

    /**
     * Receives the next application or control event.
     *
     * Description:
     *   Unlike Receive(), Ping and Pong are returned to the caller. AutoPong still
     *   controls whether Ping is answered automatically before the event is returned.
     */
    fn ReceiveEvent() Result[Event, ConnectionError]

    /**
     * Sends one Ping control frame.
     *
     * Parameters:
     *   - payload: Borrowed payload of at most 125 bytes.
     */
    fn SendPing(payload: ref byte[]) Result[void, ConnectionError]

    /**
     * Sends one Pong control frame.
     *
     * Parameters:
     *   - payload: Borrowed payload of at most 125 bytes.
     */
    fn SendPong(payload: ref byte[]) Result[void, ConnectionError]

    /**
     * Performs a normal close handshake using status 1000 and an empty reason.
     *
     * Returns:
     *   Peer close information when the closing handshake completes.
     */
    fn Close() Result[CloseInfo, ConnectionError]

    /**
     * Performs a close handshake with the supplied status code.
     */
    fn Close(code: CloseCode) Result[CloseInfo, ConnectionError]

    /**
     * Performs a close handshake with status and UTF-8 reason.
     */
    fn Close(code: CloseCode, reason: string) Result[CloseInfo, ConnectionError]

    /**
     * Immediately terminates the underlying transport without a WebSocket close handshake.
     *
     * Description:
     *   This is an abnormal closure operation for cases where graceful close is
     *   impossible or explicitly unwanted.
     */
    fn Abort()

    /**
     * Releases the connection.
     *
     * Description:
     *   Destruction never blocks waiting for a peer close handshake. An open or
     *   closing connection is aborted as necessary.
     */
    free()
}
```

## 17.3 Send semantics

`Send`, `SendText`, and `SendBinary` produce one logical message.

The implementation may fragment an outbound message into multiple frames.
Fragment size is an implementation strategy and does not change message
semantics.

Client-side frames are masked. Server-side frames are not.

If permessage-deflate is negotiated, the implementation may compress eligible
text and binary messages. Compression is never applied to control frames.

A partially written message after transport commitment cannot be rolled back.
The connection is failed/aborted if the remaining message cannot be sent
correctly.

## 17.4 Receive semantics

`Receive()`:

1. reads frames until one complete application message or close is available;
2. processes fragmentation;
3. processes negotiated decompression;
4. validates text UTF-8;
5. applies configured frame and message limits;
6. responds to Ping automatically when `AutoPong=true`;
7. ignores Pong for the high-level result;
8. returns `ReceiveResult.Closed` after a valid peer Close is processed.

When a peer initiates Close, the endpoint sends a Close response if it has not
already sent one, then completes transport closure.

An orderly close is not `ConnectionError.Closed`.

`ConnectionError.Closed` is used when an operation that requires an open
connection is attempted after closure.

## 17.5 ReceiveEvent semantics

`ReceiveEvent()` returns control frames rather than hiding them.

When `AutoPong=true`, a Ping is answered with identical application data before
`Event.Ping` is returned.

When `AutoPong=false`, the caller is responsible for choosing whether and when
to call `SendPong`.

Close frames always trigger required close-handshake state changes even when
they are surfaced as `Event.Close`.

## 17.6 Concurrency contract

A `Connection` supports:

- one send-side operation at a time;
- one receive-side operation at a time;
- one send-side and one receive-side operation concurrently.

Send-side operations are:

```text
Send
SendText
SendBinary
SendPing
SendPong
Close
```

Receive-side operations are:

```text
Receive
ReceiveEvent
Close
```

Calling two send-side operations concurrently is rejected or serialized by the
implementation without interleaving WebSocket frame bytes.

Calling two receive-side operations concurrently is a programming error and
must be diagnosed by a deterministic `ConnectionError` path rather than
allowing two decoders to consume the same frame stream.

`Close` coordinates both directions.

## Representation rationale

`Role` and `State` are enums because their semantic alternatives are closed.

`Connection` is a move-only struct because it owns mutable framing state,
compression contexts, transport ownership, close state, and receive/send
coordination.

---

# 18. `client.sec`

## 18.1 Client handshake metadata

```sec
/**
 * Contains HTTP response metadata from a successful client opening handshake.
 */
type ClientHandshake struct {
    /**
     * Successful HTTP switching status.
     */
    Status: http.Status,

    /**
     * Response fields received before the protocol switch.
     */
    Header: http.Header,
}

/**
 * Owns a newly established client connection plus its handshake metadata.
 */
type ConnectResult struct {
    /**
     * Established WebSocket connection.
     */
    Connection: Connection,

    /**
     * Successful opening-handshake response metadata.
     */
    Handshake: ClientHandshake,
}
```

For revision 0.1 `ClientHandshake.Status` is exactly HTTP 101.

## 18.2 Connection functions

```sec
/**
 * Connects to a WebSocket URL using default client options.
 *
 * Parameters:
 *   - target: Parsed ws or wss URL.
 *
 * Returns:
 *   An established connection and successful handshake metadata.
 *
 * Errors:
 *   - HandshakeError.InvalidTarget: URL shape is not valid for WebSocket.
 *   - HandshakeError.UnsupportedScheme: Scheme is not ws or wss.
 *   - HandshakeError.TlsUnavailable: wss cannot be provided by this target.
 *   - HandshakeError.Rejected: Server returned a non-101 response.
 *   - HandshakeError.InvalidAccept: Server proof is wrong.
 *   - HandshakeError.InvalidSubprotocol: Server selected an unoffered protocol.
 *   - HandshakeError.InvalidExtension: Extension negotiation is invalid.
 *   - HandshakeError.Underlying: Network, TLS, allocation, or platform failure.
 *
 * Standards:
 *   - RFC 6455 section 4.1.
 */
fn Connect(target: url.URL) Result[ConnectResult, HandshakeError]

/**
 * Connects to a WebSocket URL using explicit client options.
 */
fn Connect(
    target: url.URL,
    options: ClientOptions
) Result[ConnectResult, HandshakeError]

/**
 * Parses and connects to a WebSocket URL using default client options.
 */
fn Connect(target: string) Result[ConnectResult, HandshakeError]

/**
 * Parses and connects to a WebSocket URL using explicit client options.
 */
fn Connect(
    target: string,
    options: ClientOptions
) Result[ConnectResult, HandshakeError]
```

The string overloads parse through `net/url` and then use the corresponding
`url.URL` overload.

Connection establishment obeys the current Sec execution context for
cancellation/deadline behavior. `net/websocket` does not define a second
cancellation-token abstraction.

## 18.3 Target validation

Revision 0.1 requires:

- scheme `ws` or `wss`, ASCII case-insensitively through URL scheme semantics;
- a host;
- no fragment;
- no userinfo;
- path defaults to `/` when absent/empty;
- query is preserved;
- default port 80 for `ws`;
- default port 443 for `wss`.

The Host request field includes an explicit port when the target uses a
non-default port.

DNS resolution belongs to the networking stack, not URL parsing.

## Representation rationale

`ClientHandshake` is a struct because status and fields coexist.

`ConnectResult` is a struct because the established connection and its
handshake metadata are both results of the same successful operation.
It is move-only transitively because `Connection` is move-only.

---

# 19. `server.sec`

Revision 0.1 defines a standalone WebSocket listener. It owns its TCP listener
and therefore does not share a port with an existing `net/http.Server`.

HTTP-server integration is reserved for the later common protocol-stream
handoff design.

## 19.1 Parsed pending request

```sec
/**
 * Contains validated opening-handshake request metadata.
 */
type ServerHandshake struct {
    /**
     * Reconstructed ws/wss target URL.
     */
    Target: url.URL,

    /**
     * Validated HTTP request fields.
     */
    Header: http.Header,

    /**
     * Client-offered subprotocols in wire order.
     */
    Subprotocols: Subprotocol[],

    /**
     * Parsed client extension offers in wire order.
     */
    Extensions: Extension[],

    /**
     * Parsed Origin field when present.
     */
    Origin: Option[Origin],

    /**
     * Remote TCP endpoint.
     */
    RemoteEndpoint: ip.Endpoint,
}
```

## 19.2 Pending connection

```sec
/**
 * Owns a validated but not yet accepted standalone WebSocket handshake.
 */
@noCopy
type PendingConnection struct {
    _state: _PendingConnectionState,
}

impl PendingConnection {
    /**
     * Borrows the parsed handshake metadata.
     */
    property Handshake: ref ServerHandshake

    /**
     * Accepts the handshake using automatic configured subprotocol selection.
     *
     * Returns:
     *   An established server-role WebSocket connection.
     *
     * Errors:
     *   - HandshakeError.OriginRejected: Origin policy rejects the request.
     *   - HandshakeError.InvalidExtension: Extension negotiation fails.
     *   - HandshakeError.Underlying: Response or transport operation fails.
     *
     * Ownership:
     *   On success the pending transport is transferred into the returned Connection.
     */
    fn Accept() Result[Connection, HandshakeError]

    /**
     * Accepts the handshake with an explicitly selected offered subprotocol.
     *
     * Parameters:
     *   - subprotocol: Exact client-offered subprotocol to select.
     *
     * Errors:
     *   - HandshakeError.InvalidSubprotocol: The client did not offer the value.
     *   - HandshakeError.OriginRejected: Origin policy rejects the request.
     *   - HandshakeError.InvalidExtension: Extension negotiation fails.
     *   - HandshakeError.Underlying: Response or transport operation fails.
     */
    fn Accept(subprotocol: Subprotocol) Result[Connection, HandshakeError]

    /**
     * Rejects the handshake with an HTTP status and no response body.
     *
     * Parameters:
     *   - status: HTTP status other than 101.
     */
    fn Reject(status: http.Status) Result[void, HandshakeError]

    /**
     * Rejects the handshake with an HTTP status and additional response fields.
     */
    fn Reject(
        status: http.Status,
        header: ref http.Header
    ) Result[void, HandshakeError]

    /**
     * Releases an uncommitted pending connection.
     *
     * Description:
     *   Destruction closes the transport without pretending that the WebSocket
     *   handshake succeeded.
     */
    free()
}
```

`Reject` refuses status 101 because acceptance must go through `Accept`.

After successful `Accept`, the consumed internal pending state cannot also be
rejected or accepted again.

## 19.3 Listener

```sec
/**
 * Owns one standalone WebSocket TCP listener.
 */
@noCopy
type Listener struct {
    _state: _ListenerState,
}

impl Listener {
    /**
     * Accepts and parses the next WebSocket opening handshake.
     *
     * Returns:
     *   A pending connection after HTTP and mandatory WebSocket syntax validation.
     *
     * Errors:
     *   - ListenError.Closed: Listener is closed.
     *   - ListenError.Underlying: TCP accept or platform operation failed.
     *   - HandshakeError values from an individual malformed connection are handled
     *     by sending an appropriate HTTP failure where possible and do not terminate
     *     the listener itself.
     */
    fn Accept() Result[PendingConnection, ListenError]

    /**
     * Stops accepting new connections.
     */
    fn Close() Result[void, ListenError]

    /**
     * Reports whether the listener is closed.
     */
    property IsClosed: bool

    /**
     * Releases the listener and closes it if necessary.
     */
    free()
}

/**
 * Binds a standalone WebSocket listener using default server options.
 *
 * Parameters:
 *   - endpoint: Local IP endpoint to bind.
 *
 * Returns:
 *   An owning listener.
 *
 * Errors:
 *   - ListenError.Configuration: Default/configured state is invalid.
 *   - ListenError.Underlying: Bind/listen operation failed.
 */
fn Listen(endpoint: ip.Endpoint) Result[Listener, ListenError]

/**
 * Binds a standalone WebSocket listener using explicit server options.
 */
fn Listen(
    endpoint: ip.Endpoint,
    options: ServerOptions
) Result[Listener, ListenError]
```

`Listen` provides plain HTTP/1.1 `ws` transport in revision 0.1.

Direct TLS termination for `wss` is added when the canonical TLS package is
specified. Deployments may terminate TLS before the standalone listener using a
trusted reverse proxy, but proxy forwarding/security policy is outside this
revision.

## Representation rationale

`ServerHandshake` is a struct because target, headers, negotiated candidates,
origin, and peer endpoint coexist.

`PendingConnection` is move-only because it owns an accepted transport before
the application decides whether to accept or reject the WebSocket handshake.

`Listener` is move-only because it owns a platform listening socket and
lifecycle state.

---

# 20. Closing handshake state machine

The implementation maintains independent facts:

```text
local close sent?
peer close received?
underlying transport closed?
```

The public `State` derives from them.

## 20.1 Locally initiated close

For `Connection.Close(code, reason)`:

1. Validate the close code and encoded payload.
2. Transition Open -> Closing.
3. Send one Close frame.
4. Stop accepting new application `Send*` operations.
5. Continue processing incoming control/data as required until peer Close or
   transport failure.
6. When peer Close arrives, close the underlying transport cleanly.
7. Transition to Closed.
8. Return the peer `CloseInfo`.

If the peer had already sent Close, the local endpoint sends its required close
reply and completes transport closure rather than sending a second independent
close initiation.

## 20.2 Peer-initiated close

When peer Close is received while Open:

1. validate the Close payload;
2. transition to Closing;
3. preserve its `CloseInfo`;
4. send one Close response if none has been sent;
5. close the underlying transport as defined by RFC 6455;
6. transition to Closed;
7. surface `ReceiveResult.Closed` or `Event.Close`.

## 20.3 Abnormal transport termination

Transport termination without a completed WebSocket closing handshake is not
reported as an orderly `ReceiveResult.Closed`.

It produces `ConnectionError.Underlying`.

`CloseCode.AbnormalClosure` exists for protocol/status reporting semantics but
is never transmitted on wire.

---

# 21. Ping and Pong semantics

Ping payload length is at most 125 bytes.

A Pong sent in response to Ping contains exactly the same application data.

An endpoint may send unsolicited Pong frames.

`AutoPong=true` is the ergonomic default because RFC 6455 requires a Pong
response as soon as practical unless a Close was already received.

The library does not invent a heartbeat interval. Applications choose when to
send Ping based on their own liveness requirements.

Sec execution context cancellation/deadline remains the mechanism for bounding
operations.

---

# 22. Resource and denial-of-service requirements

A conforming implementation must enforce bounds before allocating attacker
controlled sizes where possible.

Required protections include:

1. `MaxHandshakeBytes` limits the opening HTTP header section.
2. `MaxFramePayloadBytes` is checked from the frame length before frame payload
   allocation.
3. `MaxMessageBytes` limits complete logical message size.
4. For compressed messages, `MaxMessageBytes` applies after decompression.
5. Control frames never allocate more than 125 payload bytes.
6. Invalid 64-bit payload lengths are rejected before conversion to allocation
   sizes.
7. Fragmentation cannot bypass the message limit.
8. Extension parsing has bounded token/parameter processing governed by the
   handshake byte limit.
9. A failed protocol connection is not reused as if it were valid.
10. Masking-key generation uses a source appropriate for unpredictable
    per-frame keys.

The frame payload's 64-bit protocol length is not converted to a narrower
platform size without an explicit checked conversion.

---

# 23. Failure response and close-code guidance

When the opening handshake has not succeeded, failures use HTTP responses where
possible.

After the WebSocket connection is established, protocol failures use WebSocket
Close where the connection state permits a valid Close frame.

Recommended mappings in revision 0.1:

| Condition | Close code |
|---|---:|
| protocol framing violation | 1002 |
| unsupported application data | 1003 |
| invalid text UTF-8 | 1007 |
| application policy violation | 1008 |
| message exceeds configured limit | 1009 |
| internal server condition | 1011 |

A fatal error may require immediate transport termination when sending another
frame would itself violate the protocol or cannot be performed safely.

The API must not promise that every error results in a Close frame.

---

# 24. HTTP field handling

The handshake implementation reuses `http.HeaderName`, `http.HeaderValue`, and
`http.Header` validation where applicable.

WebSocket-controlled field names are:

```text
Upgrade
Connection
Sec-WebSocket-Key
Sec-WebSocket-Accept
Sec-WebSocket-Version
Sec-WebSocket-Protocol
Sec-WebSocket-Extensions
Origin
Host
```

`Sec-WebSocket-Key` and `Sec-WebSocket-Accept` are HTTP/1.1 RFC 6455 handshake
fields.

`Connection` and `Upgrade` are token-list fields and must not be validated by
naive whole-string equality.

For example, this is a valid Connection field for upgrade purposes:

```text
Connection: keep-alive, Upgrade
```

Field occurrence and combination rules must follow the relevant HTTP and RFC
6455 rules rather than blindly joining all fields with commas.

---

# 25. Extension negotiation requirements

Unknown syntactically valid client extension offers may be parsed and exposed
through `ServerHandshake.Extensions`.

Revision 0.1 never selects an unknown extension.

A client rejects a server extension response when:

- the extension was not offered;
- the server selects the same extension in an invalid multiplicity;
- parameters are invalid;
- parameters exceed the permitted client offer;
- the extension is known but not implemented by this revision.

Only `permessage-deflate` changes frame semantics in revision 0.1.

The IANA-registered `bbf-usp-protocol` identifier is representable and reported
as registered but is not implemented by this package revision.

---

# 26. Registry synchronization policy

Registry-backed APIs are not silently changed by implementation code.

When the IANA WebSocket registry changes:

1. update the relevant source-file registry date;
2. update this book's registry date;
3. update static constants only when the revision intentionally mirrors that
   registry subset or complete assigned set;
4. add tests for new assignments;
5. do not repurpose an existing Sec identifier to mean a different registered
   value;
6. preserve open nominal types so unrecognized future values remain
   representable where protocol rules allow them.

Revision 0.1 mirrors completely:

- individually assigned close-code entries;
- WebSocket extension names.

It intentionally does **not** mirror the WebSocket subprotocol registry as
static constants because the registry is application-oriented, large, and open.
`Subprotocol.Parse` is the complete API for arbitrary valid subprotocol names.

---

# 27. Implementation and validation checklist

An implementation claiming conformance to revision 0.1 must verify all of the
following.

## 27.1 API shape

- no public type name is declared twice;
- no constructor is spelled `New`;
- constructors use Sec's `init` declaration form;
- no empty constructor is declared only to restate default construction;
- destructors are `free()`;
- consuming parameters use `->name: Type`;
- index and byte-offset types are explicit Sec integer types;
- all public declarations have LSP-structured documentation.

## 27.2 Handshake

- fresh 16-byte client nonce;
- canonical Base64 key;
- exact RFC 6455 GUID;
- SHA-1 accept computation;
- GET request;
- HTTP/1.1 101 validation;
- token-aware Upgrade and Connection parsing;
- version 13 negotiation;
- strict selected-subprotocol validation;
- strict selected-extension validation;
- Origin policy;
- handshake byte limit;
- reserved user-header rejection.

## 27.3 Frames

- client masking;
- server non-masking;
- unpredictable per-frame masking keys;
- minimal payload-length encoding;
- 64-bit high-bit validation;
- frame limit before allocation;
- control-frame FIN and 125-byte rules;
- continuation state;
- interleaved control frames;
- unknown opcode rejection;
- RSV validation.

## 27.4 Messages

- fragmented text UTF-8 validation across frame boundaries;
- binary byte preservation;
- complete logical message limit;
- decompressed-size limit;
- high-level orderly close result;
- explicit control-event API.

## 27.5 Close

- empty close payload support;
- one-byte close payload rejection;
- network-order status code;
- forbidden synthetic code rejection on send;
- UTF-8 reason validation;
- reason/control-frame byte bound;
- symmetric close handshake;
- abnormal transport closure distinguished from orderly close.

## 27.6 Compression

- RFC 7692 offer parsing;
- RFC 7692 response validation;
- client/server window negotiation;
- context-takeover semantics;
- RSV1 semantics;
- no control-frame compression;
- fragmented compressed-message handling;
- decompression-bomb limit.

---

# 28. Required test classes

At minimum, tests must cover:

## 28.1 RFC examples

- RFC 6455 sample key:
  `dGhlIHNhbXBsZSBub25jZQ==`
- expected accept:
  `s3pPLMBiTxaQ9kYGzzhZRbK+xOo=`

## 28.2 Handshake negatives

- wrong method;
- missing Host;
- missing Upgrade;
- missing Connection;
- missing key;
- key decoding to other than 16 bytes;
- missing version;
- unsupported version;
- invalid subprotocol token;
- duplicated subprotocol;
- server selecting unoffered subprotocol;
- invalid extension response;
- wrong accept value;
- origin rejection;
- fragment-bearing target URL;
- reserved caller-controlled handshake field.

## 28.3 Framing

- unmasked client frame;
- masked server frame;
- all legal payload-length encodings;
- non-minimal payload lengths;
- 64-bit length with top bit set;
- payload exactly at configured limits;
- payload one byte over configured limits;
- fragmented text;
- fragmented binary;
- control frame between fragments;
- continuation without active message;
- second data start during fragmentation;
- fragmented control frame;
- oversized control frame;
- reserved opcode;
- unexpected RSV bits.

## 28.4 UTF-8

- valid ASCII;
- valid multi-byte UTF-8;
- multi-byte sequence split across frames;
- malformed continuation;
- overlong encoding;
- surrogate encoding;
- code point above U+10FFFF;
- invalid close reason UTF-8.

## 28.5 Close codes

- every static IANA constant;
- private-use values 4000 and 4999;
- invalid 999;
- invalid 5000;
- forbidden 1004;
- forbidden 1005;
- forbidden 1006;
- forbidden 1015;
- future/unrecognized protocol-range values remain representable.

## 28.6 Compression

- no-context-takeover in each direction;
- context takeover in each direction;
- every WindowBits value 8..15;
- invalid negotiation combinations;
- compressed single frame;
- compressed fragmented message;
- uncompressed message on a compressed connection;
- RSV1 on continuation rejected;
- RSV1 on control rejected;
- decompressed message exceeding limit rejected.

---

# 29. Explicit revision-0.1 exclusions

The following are intentionally outside revision 0.1:

- RFC 8441 HTTP/2 Extended CONNECT;
- RFC 9220 HTTP/3 Extended CONNECT;
- sharing an existing `net/http.Server` listener;
- a public raw-frame construction API;
- a streaming partial-message API;
- application heartbeat scheduling;
- automatic reconnect;
- application message schemas;
- WebSocket subprotocol implementations;
- `bbf-usp-protocol` extension semantics;
- proxy configuration;
- SOCKS proxy configuration;
- TLS API ownership;
- browser WebSocket API emulation.

These exclusions are scope boundaries, not permission to invent undocumented
public APIs.

---

# 30. Planned HTTP integration boundary

A later revision should integrate with `net/http` without making
`net/websocket` depend on HTTP/1.1 connection hijacking semantics.

The required concept is a **version-independent bidirectional protocol stream**
that can represent:

- an HTTP/1.1 connection after a successful 101 protocol switch;
- an HTTP/2 Extended CONNECT stream;
- an HTTP/3 Extended CONNECT stream.

That abstraction belongs to `net/http` or a shared I/O layer, not to a private
WebSocket-only workaround.

When specified, WebSocket server integration should be able to validate a
WebSocket handshake from an HTTP request and receive ownership of such a
protocol stream only after the HTTP layer has performed the version-correct
transition.

The WebSocket book must then add RFC 8441 and RFC 9220 semantics without
changing application message/framing semantics.

---

# 31. Implementation status contract

The corresponding governance fragment is:

```text
implementation-status-std-net-websocket.yaml
```

It must distinguish at least:

- package structure;
- public API specified;
- source files scaffolded;
- HTTP/1.1 client handshake;
- standalone server handshake;
- frame parser;
- frame encoder;
- masking;
- fragmentation;
- UTF-8 validation;
- close handshake;
- Ping/Pong;
- subprotocol negotiation;
- permessage-deflate negotiation;
- permessage-deflate codec integration;
- ws client support;
- wss client support;
- standalone ws listener;
- TLS listener support;
- tests;
- target support;
- identified bugs.

HTTP/2 and HTTP/3 WebSocket support remain explicitly unimplemented for
revision 0.1 and must not be reported as an implementation bug.

---

# 32. Summary of normative public surface

Revision 0.1 owns these public types:

```text
ConfigError
HandshakeError
ProtocolError
ConnectionError
ListenError

Version
Subprotocol

ExtensionName
ExtensionParameterName
ExtensionParameter
Extension

WindowBits
PerMessageDeflateClientOptions
PerMessageDeflateServerOptions
PerMessageDeflate

CloseCode
CloseInfo

MessageType
Message
ReceiveResult
Event

ConnectionOptions
Origin
OriginPolicy
ClientOptions
ServerOptions

Role
State
Connection

ClientHandshake
ConnectResult

ServerHandshake
PendingConnection
Listener
```

Revision 0.1 owns these module-level public functions:

```sec
fn ParseExtensions(
    value: http.HeaderValue
) Result[Extension[], HandshakeError]

fn SerializeExtensions(
    extensions: ref Extension[]
) Result[http.HeaderValue, HandshakeError]

fn ComputeAccept(
    key: http.HeaderValue
) Result[http.HeaderValue, HandshakeError]

fn Connect(
    target: url.URL
) Result[ConnectResult, HandshakeError]

fn Connect(
    target: url.URL,
    options: ClientOptions
) Result[ConnectResult, HandshakeError]

fn Connect(
    target: string
) Result[ConnectResult, HandshakeError]

fn Connect(
    target: string,
    options: ClientOptions
) Result[ConnectResult, HandshakeError]

fn Listen(
    endpoint: ip.Endpoint
) Result[Listener, ListenError]

fn Listen(
    endpoint: ip.Endpoint,
    options: ServerOptions
) Result[Listener, ListenError]
```

No additional public declaration is implied by examples, implementation notes,
or private framing requirements.

---

**Revision:** 0.1  
**Updated:** 2026-09-26
