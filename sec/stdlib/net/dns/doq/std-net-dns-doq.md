# Sec Standard Library — DNS over QUIC

- **Status:** Draft
- **Created:** 2026-09-07
- **Last updated:** 2026-09-07
- **Document revision:** 0.1
- **Sec language version:** 0.1
- **Canonical path:** `stdlib/net/dns/doq/std-net-dns-doq.md`
- **Repository path:** `sec/stdlib/net/dns/doq/std-net-dns-doq.md`
- **Parent specification:** `stdlib/net/dns/std-net-dns.md`
- **Sibling specifications:** `stdlib/net/dns/doh/std-net-dns-doh.md`, `stdlib/net/dns/dot/std-net-dns-dot.md`
- **Implementation-status fragment:** `implementation-status-std-net-dns-doq.yaml`
- **Latest verified repository main:** `0f5027d`
- **Standards state rechecked:** 2026-09-07

---

# 1. Purpose

`net/dns/doq` implements DNS over Dedicated QUIC Connections (DoQ).

It maps DNS transactions to independent client-initiated bidirectional QUIC
streams according to RFC 9250.

It owns:

- DoQ server configuration;
- bootstrap address handling;
- DoQ ALPN selection;
- DoQ application error codes;
- DNS message framing on QUIC streams;
- one-stream-per-query transaction semantics;
- QUIC connection reuse;
- parallel DNS transactions;
- transaction cancellation;
- 0-RTT policy for replayable DNS operations;
- DoQ-specific privacy policy;
- QUIC connection-migration policy;
- DoQ padding policy;
- client and server transport adapters;
- multi-response transaction support needed by zone transfers;
- integration with the shared `net/dns` resolver engine.

The package is imported as:

```sec
import "net/dns/doq"
```

Typical direct use:

```sec
let server := try doq.Server.Parse("resolver.example")

let client := try doq.Client.New(
    server,
    doq.ClientOptions.Default()
)

let response := try client.Query(
    new dns.Question(
        try dns.Name.Parse("example.com"),
        dns.RecordType.A,
        dns.DnsClass.IN
    )
)
```

DoQ does not reimplement:

- DNS message/record semantics;
- UDP;
- QUIC;
- TLS 1.3;
- certificate parsing;
- DNS resolver caching.

Those remain owned by their canonical packages.

---

# 2. Package boundary

## 2.1 Dependencies

`net/dns/doq` depends on:

```text
net/dns
net/ip
net/quic
```

Conceptually:

```text
net/dns   <- net/dns/doq
net/ip    <- net/dns/doq
net/quic  <- net/dns/doq
```

The base `net/dns` package must not import `net/dns/doq`.

`net/quic` owns:

- QUIC packets;
- QUIC versions;
- QUIC handshake;
- QUIC/TLS integration;
- connection IDs;
- stream transport;
- flow control;
- congestion control;
- path validation;
- connection migration mechanics;
- QUIC transport errors.

`net/dns/doq` owns the DNS application mapping on top of those QUIC
facilities.

## 2.2 DoQ is not HTTP/3

DoQ uses QUIC directly.

It does not use:

```text
HTTP
HTTP/3
URLs
HTTP status codes
HTTP caching
```

That is the DoH model, not the DoQ model.

HTTP/3 and DoQ may use the same underlying QUIC implementation and may share
UDP port 443 in deployments where explicitly configured, but they remain
different application protocols with different ALPN identifiers.

## 2.3 DoQ is not DNS-over-UDP

QUIC uses UDP as its packet substrate.

That does not make DoQ a variant of classic DNS-over-UDP.

DoQ has:

- QUIC connections;
- TLS 1.3 security;
- QUIC streams;
- flow control;
- connection state;
- connection/application error codes.

The package must not route DoQ through `ip.UdpSocket.SendTo()` as though one
DNS message were one ordinary UDP DNS datagram.

The canonical `net/quic` implementation owns the UDP transport mechanics.

---

# 3. Standards and registries

## 3.1 Primary DoQ standard

Primary protocol specification:

- **RFC 9250 — DNS over Dedicated QUIC Connections**
- https://www.rfc-editor.org/rfc/rfc9250
- https://www.rfc-editor.org/info/rfc9250

RFC 9250 defines:

- DoQ connection establishment;
- ALPN `doq`;
- default UDP port 853;
- one bidirectional stream per DNS query;
- two-octet DNS message length framing;
- DNS Message ID zero;
- stream FIN behavior;
- DoQ error codes;
- transaction cancellation;
- session resumption and 0-RTT;
- message-size rules;
- connection reuse;
- flow control;
- padding;
- privacy behavior;
- zone-transfer requirements.

## 3.2 QUIC version 1

Core QUIC standards:

- **RFC 9000 — QUIC: A UDP-Based Multiplexed and Secure Transport**  
  https://www.rfc-editor.org/rfc/rfc9000
- **RFC 9001 — Using TLS to Secure QUIC**  
  https://www.rfc-editor.org/rfc/rfc9001
- **RFC 9002 — QUIC Loss Detection and Congestion Control**  
  https://www.rfc-editor.org/rfc/rfc9002

DoQ does not duplicate these transport mechanisms.

## 3.3 QUIC version 2

- **RFC 9369 — QUIC Version 2**  
  https://www.rfc-editor.org/rfc/rfc9369

RFC 9369 explicitly states that the `doq` ALPN can operate over QUIC version 2.

Therefore `net/dns/doq` must not hard-code QUIC version 1 as the only allowed
version.

QUIC version selection and negotiation belong to `net/quic`.

The DoQ application mapping remains the same across compatible QUIC versions
unless a future DoQ specification says otherwise.

## 3.4 DNS wire format

DoQ carries ordinary DNS messages.

Relevant baseline:

- RFC 1034  
  https://www.rfc-editor.org/rfc/rfc1034
- RFC 1035  
  https://www.rfc-editor.org/rfc/rfc1035

DoQ reuses:

```sec
dns.Message
dns.EncodeMessage(...)
dns.DecodeMessageToOwned(...)
```

## 3.5 DNS-over-TCP operational guidance

RFC 9250 reuses relevant operational requirements from:

- **RFC 7766 — DNS Transport over TCP - Implementation Requirements**  
  https://www.rfc-editor.org/rfc/rfc7766

This is particularly relevant to:

- connection reuse;
- resource management;
- concurrent processing.

However, DoQ stream mapping is not TCP pipelining.

## 3.6 Authentication and privacy profiles

For stub-to-recursive authentication, RFC 9250 references the same model as
DoT:

- RFC 7858  
  https://www.rfc-editor.org/rfc/rfc7858
- RFC 8310  
  https://www.rfc-editor.org/rfc/rfc8310

RFC 9250 recommends that DoQ stubs use a Strict usage profile.

Sec follows that recommendation for the default client policy.

## 3.7 TLS

QUIC uses TLS 1.3.

Relevant standard:

- RFC 8446 — TLS 1.3  
  https://www.rfc-editor.org/rfc/rfc8446

The QUIC package owns the TLS-for-QUIC integration.

The DoQ package must not create a second standalone TLS layer around QUIC.

## 3.8 DNS privacy

Relevant privacy/operational guidance:

- RFC 8932 — Recommendations for DNS Privacy Service Operators  
  https://www.rfc-editor.org/rfc/rfc8932
- RFC 9076 — DNS Privacy Considerations  
  https://www.rfc-editor.org/rfc/rfc9076

These apply to DoQ in addition to RFC 9250's DoQ-specific privacy sections.

## 3.9 Padding

Relevant standards:

- RFC 7830 — EDNS(0) Padding Option  
  https://www.rfc-editor.org/rfc/rfc7830
- RFC 8467 — Padding Policies for Extension Mechanisms for DNS (EDNS(0))  
  https://www.rfc-editor.org/rfc/rfc8467

RFC 9250 requires implementations to protect against traffic-analysis attacks
using QUIC packet padding where available or EDNS padding otherwise.

## 3.10 Zone transfer

DoQ supports zone transfers.

Relevant references:

- RFC 1995 — IXFR  
  https://www.rfc-editor.org/rfc/rfc1995
- RFC 5936 — AXFR  
  https://www.rfc-editor.org/rfc/rfc5936
- RFC 9103 — Zone Transfer over TLS  
  https://www.rfc-editor.org/rfc/rfc9103

RFC 9250 applies analogous connection-reuse/concurrency guidance to zone
transfers over DoQ.

The higher-level zone-transfer API remains owned by `net/dns/zone`.

## 3.11 Discovery

Relevant standards:

- RFC 9461 — Service Binding Mapping for DNS Servers  
  https://www.rfc-editor.org/rfc/rfc9461
- RFC 9462 — Discovery of Designated Resolvers  
  https://www.rfc-editor.org/rfc/rfc9462

For DoQ, RFC 9461 uses:

```text
alpn=doq
```

and default port:

```text
853
```

when no explicit SVCB port is supplied.

Discovery spans DoQ, DoT, and DoH and therefore remains outside this child
package's core responsibility.

## 3.12 IANA registrations

Relevant IANA registries include:

- TLS ALPN Protocol IDs  
  https://www.iana.org/assignments/tls-extensiontype-values
- Service Name and Transport Protocol Port Number Registry  
  https://www.iana.org/assignments/service-names-port-numbers
- DNS-over-QUIC Error Codes  
  https://www.iana.org/assignments/dns-over-quic

Where the dedicated DoQ error registry is exposed through another IANA registry
location or registry structure, source comments must use the current canonical
IANA URL at implementation time.

---

# 4. Source-file standards headers

Every `.sec` file in this package must identify the directly applicable
standards.

Example:

```sec
// Sec Standard Library: net/dns/doq
// File: stream.sec
//
// Standards:
//   RFC 9250 - DNS over Dedicated QUIC Connections
//   https://www.rfc-editor.org/rfc/rfc9250
//
//   RFC 9000 - QUIC
//   https://www.rfc-editor.org/rfc/rfc9000
//
// DNS framing:
//   RFC 1035
//   https://www.rfc-editor.org/rfc/rfc1035

module doq
```

A source file that mirrors the DoQ error-code registry must include the current
IANA registry URL and synchronization date.

---

# 5. Package and file manifest

Initial package structure:

```text
stdlib/net/dns/doq/
├── std-net-dns-doq.md
├── error.sec
├── protocol.sec
├── server.sec
├── bootstrap.sec
├── options.sec
├── framing.sec
├── padding.sec
├── stream.sec
├── connection.sec
├── client.sec
├── listener.sec
└── transport.sec
```

All package files use:

```sec
module doq
```

Expected tests:

```text
protocol_test.sec
server_test.sec
bootstrap_test.sec
options_test.sec
framing_test.sec
padding_test.sec
stream_test.sec
connection_test.sec
client_test.sec
listener_test.sec
transport_test.sec
```

---

# Part I — Errors and protocol constants

# 6. `error.sec`

## 6.1 Responsibility

`error.sec` owns semantic failures produced by the DoQ package.

It does not duplicate:

```sec
dns.MessageError
ip.NetworkError
quic.TransportError
```

or other canonical QUIC errors.

## 6.2 `ServerError`

```sec
enum ServerError error {
    EmptyName,
    InvalidName,
    InvalidPort,
    InvalidConfiguration,
}
```

## 6.3 `BootstrapError`

```sec
enum BootstrapError error {
    Required,
    EmptyAddressList,
    InvalidConfiguration,
    AddressFamilyUnavailable,
}
```

## 6.4 `FramingError`

```sec
enum FramingError error {
    InvalidLength,
    MessageTooLarge,
    UnexpectedFin,
    UnexpectedEof,
    OutputTooSmall,
    ExtraMessage,
}
```

## 6.5 `DoqError`

```sec
union DoqError error {
    Server(ServerError),
    Bootstrap(BootstrapError),

    Quic(error),
    Dns(dns.MessageError),
    Framing(FramingError),

    Peer(DoqErrorCode),

    InvalidMessageId,
    UnexpectedStream,
    UnexpectedResponseCount,
    TransactionCancelled,
    PrivacyPolicyViolation,
    ConnectionClosed,
    ExcessiveLoad,
}
```

Rules:

- `Quic(error)` preserves the canonical concrete QUIC failure;
- a nonzero DNS RCODE is not a `DoqError`;
- peer application close/reset codes remain observable as `Peer`;
- unknown peer DoQ error codes remain representable;
- cancellation is not flattened into a DNS RCODE;
- local Sec execution cancellation and peer transaction cancellation are
  distinct events.

---

# 7. `protocol.sec`

## 7.1 Responsibility

`protocol.sec` owns DoQ protocol constants and open DoQ application error
codes.

## 7.2 ALPN

The registered ALPN identifier is:

```text
doq
```

The source-level representation must reuse the canonical `net/quic` / TLS ALPN
type.

Conceptually:

```sec
static let Alpn := quic.ApplicationProtocol("doq")
```

The exact constructor/type name is provisional until `std-net-quic.md` is
locked.

DoQ must not use an arbitrary string parameter at each call site.

## 7.3 Default port

```sec
static let DefaultPort: ip.Port := ip.Port(853)
```

This is UDP port 853.

DoQ must not use UDP port 53.

Alternative ports require explicit configuration or validated discovery.

RFC 9250 notes that mutually agreed UDP port 443 can be operationally useful,
but Sec does not make 443 a second implicit default.

## 7.4 `DoqErrorCode`

QUIC application error codes occupy the QUIC variable-integer domain.

The DoQ registry is extensible.

The public wire representation is therefore open:

```sec
enum DoqErrorCode bit[62] {
    NoError = 0x0,
    InternalError = 0x1,
    ProtocolError = 0x2,
    RequestCancelled = 0x3,
    ExcessiveLoad = 0x4,
    UnspecifiedError = 0x5,

    ErrorReserved = 0xd098ea5e,
}
```

Rules:

- unknown values remain representable;
- receipt of an unknown code in an unexpected context is treated
  semantically as `UnspecifiedError` while the original numeric value remains
  available for diagnostics;
- `ErrorReserved` is for protocol-extension testing;
- code names are Sec-readable forms of the IANA/RFC names;
- the numeric wire values are normative.

## 7.5 Protocol errors

RFC 9250 treats several conditions as fatal DoQ protocol errors, including:

- nonzero DNS Message ID;
- premature FIN before a complete framed message;
- more than one query on a transaction stream;
- unexpected response count;
- missing expected FIN;
- `edns-tcp-keepalive` on DoQ;
- attempted unidirectional QUIC stream;
- server-initiated bidirectional stream;
- invalid 0-RTT transaction usage.

A peer encountering a fatal protocol violation should close the QUIC
connection using:

```sec
DoqErrorCode.ProtocolError
```

according to RFC 9250.

---

# Part II — Server identity and bootstrap

# 8. `server.sec`

## 8.1 Responsibility

`server.sec` owns configured DoQ resolver identity and service port.

DoQ server identity is conceptually the same authenticated DNS resolver
identity used by DoT for stub-to-recursive service.

## 8.2 `Server`

```sec
type Server struct {
    AuthenticationName: dns.HostName,
    Port: ip.Port,
    Bootstrap: Bootstrap,
}
```

Canonical API:

```sec
impl Server {
    static fn Parse(
        authenticationName: string
    ) Result[Server, ServerError]

    static fn New(
        authenticationName: dns.HostName,
        port: ip.Port,
        bootstrap: Bootstrap
    ) Result[Server, ServerError]

    static let DefaultPort: ip.Port := ip.Port(853)
}
```

Default parse result:

```text
Port      = 853
Bootstrap = System
```

The parser does not accept:

- a URL;
- an HTTP authority;
- a raw `"host:port"` string with ambiguous IPv6 grammar.

Custom port configuration uses the typed constructor.

## 8.3 Authentication

For stub-to-recursive DoQ, the default Sec policy is Strict authenticated
service.

The configured `AuthenticationName` remains the reference identity used by the
QUIC/TLS configuration.

DoQ does not treat a bootstrap IP as the authenticated identity.

---

# 9. `bootstrap.sec`

## 9.1 Responsibility

`bootstrap.sec` owns how a named DoQ resolver obtains candidate IP addresses
without recursively depending on itself.

## 9.2 `Bootstrap`

```sec
type Bootstrap union {
    System,
    Addresses(ip.Address[]),
}
```

## 9.3 Explicit addresses

```sec
impl Bootstrap {
    static fn FromAddresses(
        addresses: ip.Address[]
    ) Result[Bootstrap, BootstrapError]
}
```

Rules:

- explicit list must not be empty;
- IPv4 and IPv6 may coexist;
- addresses are QUIC routing candidates;
- `AuthenticationName` remains unchanged;
- address selection/racing belongs to `net/quic` / common connection policy.

## 9.4 System bootstrap

`System` uses an already available resolver/configuration path that does not
recursively invoke this DoQ client.

If no safe bootstrap path exists, construction/connection returns:

```sec
BootstrapError.Required
```

rather than entering recursive resolution.

---

# Part III — Options and privacy policy

# 10. `options.sec`

## 10.1 Responsibility

`options.sec` owns DoQ-specific application policy.

It does not duplicate generic QUIC transport tuning.

## 10.2 `AuthenticationPolicy`

```sec
enum AuthenticationPolicy {
    Strict,
    Opportunistic,
}
```

Sec defaults to:

```sec
AuthenticationPolicy.Strict
```

For stub-to-recursive DoQ this follows RFC 9250's recommendation to use a
Strict profile.

`Opportunistic` is available only when a caller deliberately chooses the
weaker RFC 8310-style behavior.

The DoQ package itself never becomes cleartext DNS.

## 10.3 `ZeroRttPolicy`

```sec
enum ZeroRttPolicy {
    Disabled,
    ReplayableTransactions,
}
```

Sec revision 0.1 defaults to:

```sec
ZeroRttPolicy.Disabled
```

This is a conservative privacy default.

When explicitly set to:

```sec
ZeroRttPolicy.ReplayableTransactions
```

0-RTT may only carry DNS operations permitted by RFC 9250.

RFC 9250 defines the replayable OPCODE set for 0-RTT as:

```text
QUERY
NOTIFY
```

Other OPCODEs must not be sent as 0-RTT application data.

The package must inspect the semantic `dns.Opcode`, not a magic integer.

## 10.4 `MigrationPolicy`

```sec
enum MigrationPolicy {
    RestartOnNetworkChange,
    AllowConnectionMigration,
}
```

Default:

```sec
MigrationPolicy.RestartOnNetworkChange
```

RFC 9250 recommends privacy-first behavior for clients: when connectivity
changes, begin a new session rather than letting one long-lived resolver
connection follow the user across networks.

`AllowConnectionMigration` is explicit opt-in for deployments where continuity
is preferred over that privacy property.

## 10.5 `PaddingPolicy`

```sec
enum PaddingPolicy {
    Automatic,
    QuicPacket,
    Edns,
}
```

There is no ordinary safe `Disabled` value.

RFC 9250 requires traffic-analysis mitigation.

Semantics:

- `Automatic`: use QUIC packet padding when the QUIC API exposes adequate
  control; otherwise use EDNS padding;
- `QuicPacket`: require QUIC-layer padding support;
- `Edns`: use RFC 7830 DNS padding.

If explicitly requested `QuicPacket` padding is unavailable:

```sec
DoqError.PrivacyPolicyViolation
```

is returned rather than silently disabling padding.

## 10.6 `ClientOptions`

```sec
type ClientOptions struct {
    Authentication: AuthenticationPolicy,
    ZeroRtt: ZeroRttPolicy,
    Migration: MigrationPolicy,
    Padding: PaddingPolicy,
    MaxMessageBytes: uint,
    EnableSessionResumption: bool,
}
```

Canonical defaults:

```sec
impl ClientOptions {
    static fn Default() ClientOptions
}
```

Defaults:

```text
Authentication          = Strict
ZeroRtt                 = Disabled
Migration               = RestartOnNetworkChange
Padding                 = Automatic
MaxMessageBytes         = 65535
EnableSessionResumption = true
```

`MaxMessageBytes` must not exceed 65535.

## 10.7 QUIC-specific tuning

DoQ does not expose package-local duplicates of:

- QUIC congestion controller;
- UDP socket buffer sizes;
- QUIC versions;
- transport parameter raw integers;
- connection-ID size;
- packet-number behavior;
- flow-control frame internals.

Where applications need explicit QUIC tuning, a future constructor may accept
a canonical `quic.ClientConfig`.

---

# Part IV — Framing

# 11. `framing.sec`

## 11.1 Responsibility

`framing.sec` owns DNS message framing inside each DoQ QUIC stream.

Primary reference:

- RFC 9250 section 4.2
- RFC 1035 DNS-over-TCP length format

## 11.2 Frame format

Every DNS message sent over DoQ is:

```text
+----------------------+-----------------------------+
| uint16 messageLength | DNS wire message bytes      |
+----------------------+-----------------------------+
```

The two-byte length field is network byte order.

This remains true even though each query has its own QUIC stream.

## 11.3 Maximum message size

Maximum DNS message payload:

```text
65535 bytes
```

QUIC streams can carry much more data, but DoQ deliberately retains the DNS
message size limit.

The EDNS advertised UDP payload size is ignored for DoQ transport sizing.

## 11.4 Encoding

```sec
fn EncodeFrame(
    message: ref dns.Message,
    output: ref mut byte[]
) Result[uint, DoqError]
```

The function:

1. encodes a DNS message;
2. validates size;
3. writes a two-byte length prefix;
4. returns total framing + payload bytes written.

## 11.5 Incremental reader

The implementation must support arbitrary QUIC stream segmentation.

Internal state must handle:

- partial two-byte prefix;
- partial DNS payload;
- multiple response frames on one stream;
- FIN immediately after the last complete frame;
- FIN before a complete frame;
- configured message-size limits.

The implementation must not assume that one QUIC STREAM frame corresponds to
one DNS frame.

## 11.6 Message ID

The encoder used for DoQ transaction traffic must enforce:

```text
MessageId = 0
```

A nonzero inbound DoQ DNS Message ID is a protocol error.

When proxying:

- another DNS transport -> DoQ: set Message ID to zero;
- DoQ -> another DNS transport: generate an ID according to that transport's
  requirements.

---

# Part V — Padding

# 12. `padding.sec`

## 12.1 Responsibility

`padding.sec` owns the DoQ application decision for traffic-analysis padding.

RFC 9250 makes padding protection mandatory at the implementation level.

## 12.2 Automatic policy

`PaddingPolicy.Automatic` performs:

1. QUIC packet-level padding if `net/quic` exposes suitable padding control;
2. otherwise EDNS(0) Padding according to RFC 7830;
3. use a small set of fixed padded sizes.

RFC 8467 provides deployed guidance for EDNS padding sizes and should be used
where no stronger/current DoQ-specific policy exists.

## 12.3 No double padding requirement

Using QUIC packet padding does not prohibit EDNS padding, but Sec should avoid
wasteful unconditional double padding.

The effective policy should provide the required traffic-analysis mitigation
with bounded overhead.

## 12.4 Server behavior

DoQ servers must also apply a conforming padding policy to responses.

A server that cannot provide either adequate QUIC padding or EDNS padding must
not claim full RFC 9250 privacy compliance.

---

# Part VI — Transaction streams

# 13. `stream.sec`

## 13.1 Responsibility

`stream.sec` owns one DNS transaction mapped to one client-initiated
bidirectional QUIC stream.

This is the central DoQ abstraction.

## 13.2 Stream mapping rules

For each query:

1. client opens a new client-initiated bidirectional stream;
2. client sends exactly one framed DNS query;
3. query Message ID is zero;
4. client sends STREAM FIN on its send direction;
5. server sends one or more framed DNS responses on the same stream;
6. server sends STREAM FIN after the final response.

A server must not initiate a DoQ bidirectional stream.

DoQ does not use QUIC unidirectional streams.

## 13.3 `Transaction`

```sec
@noCopy
type Transaction struct {
    // Private QUIC bidirectional stream + DNS framing state.
}
```

Canonical public API:

```sec
impl Transaction {
    fn NextResponse() Result[Option[dns.Message], DoqError]

    fn Cancel() Result[void, DoqError]
}
```

The request has already been sent when the `Transaction` is returned.

## 13.4 `NextResponse`

`NextResponse` supports both:

- ordinary one-response DNS transactions;
- multi-response operations such as AXFR/IXFR.

Semantics:

- returns `Some(message)` for each complete DNS response frame;
- every received DNS Message ID must be zero;
- returns `None` only after a valid final STREAM FIN following the final
  complete response;
- premature FIN is an error;
- a stream reset is surfaced through `DoqError`;
- normal DNS RCODE values remain inside each returned `dns.Message`.

## 13.5 `Cancel`

Cancellation of an outstanding DoQ transaction uses QUIC stream cancellation
semantics required by RFC 9250.

Conceptually the client sends:

```text
STOP_SENDING(DOQ_REQUEST_CANCELLED)
```

and abandons the DNS transaction.

The package should use:

```sec
DoqErrorCode.RequestCancelled
```

for the application error code.

Rules:

- cancellation is idempotent locally;
- any later response bytes are discarded;
- the transaction is not inserted into the DNS cache;
- cancelling one transaction does not close the whole QUIC connection unless
  the peer/transport requires it;
- server processing should stop when the corresponding cancellation is
  received as specified by RFC 9250.

## 13.6 Dangling streams

Client and server implementations must bound dangling transaction streams.

A stream is potentially dangling when:

- expected query/response bytes do not arrive; or
- expected FIN does not arrive.

Resource limits and cancellation/deadline policy must prevent unbounded
retention.

Excessive dangling streams may cause connection closure with:

```sec
DoqErrorCode.ExcessiveLoad
```

where appropriate.

---

# Part VII — QUIC connection

# 14. `connection.sec`

## 14.1 Responsibility

`connection.sec` owns one established DoQ QUIC connection.

## 14.2 `Connection`

```sec
@noCopy
type Connection struct {
    // Private quic.Connection, server identity, privacy policy,
    // session state, stream/resource counters.
}
```

Canonical semantic surface:

```sec
impl Connection {
    property Server: Server { get { ... } }

    fn OpenTransaction(
        request: ref dns.Message
    ) Result[Transaction, DoqError]

    fn Close() Result[void, DoqError]
}
```

Construction remains internal to `Client`.

## 14.3 Establishment

Connection establishment conceptually performs:

```text
bootstrap
-> QUIC connect
-> QUIC TLS 1.3 authentication
-> ALPN "doq"
-> usable DoQ connection
```

No DNS query is sent as ordinary application data until the DoQ application
protocol is negotiated, except valid explicitly enabled 0-RTT behavior using
remembered compatible QUIC/ALPN/session state.

## 14.4 ALPN requirement

A DoQ connection must negotiate:

```text
doq
```

A QUIC connection negotiating HTTP/3:

```text
h3
```

is not a DoQ connection.

Failure to negotiate DoQ is a connection/protocol failure.

## 14.5 QUIC versions

The connection delegates version support/negotiation to `net/quic`.

At minimum the application design is compatible with:

- QUIC version 1;
- QUIC version 2.

The DoQ package does not contain a `DoqVersion.V1/V2` enum because these are
QUIC transport versions, not DoQ application versions.

## 14.6 Connection reuse

A DoQ connection is intended to carry many DNS transactions.

The client should:

- keep a usable connection;
- open a new stream per query;
- send independent queries concurrently;
- avoid reconnecting merely because one transaction completed.

## 14.7 Idle connections

The implementation follows QUIC idle-timeout negotiation.

Before sending a query on a long-idle connection, the client should avoid
using a connection so close to expiry that the new query is likely to be lost
to idle timeout.

The exact safety margin is implementation policy.

## 14.8 Connection failure

If a QUIC connection fails:

- all in-progress transactions on that connection are abandoned;
- each waiter completes exactly once;
- higher resolver/client policy may retry eligible ordinary queries;
- arbitrary state-changing `Exchange` traffic must not be blindly replayed.

## 14.9 Connection close

Explicit close uses QUIC connection-close semantics with:

```sec
DoqErrorCode.NoError
```

when no error is being signaled.

`Close()` is idempotent.

---

# Part VIII — Client

# 15. `client.sec`

## 15.1 Responsibility

`client.sec` owns reusable DoQ client state and ordinary query convenience.

## 15.2 `Client`

```sec
@noCopy
type Client struct {
    // Private Server, ClientOptions, reusable Connection,
    // QUIC/session-resumption state, capability-failure state.
}
```

Canonical API:

```sec
impl Client {
    static fn New(
        server: Server,
        options: ClientOptions
    ) Result[Client, DoqError]

    property Server: Server { get { ... } }

    fn OpenTransaction(
        request: ref dns.Message
    ) Result[Transaction, DoqError]

    fn Exchange(
        request: ref dns.Message
    ) Result[dns.Message, DoqError]

    fn Query(
        question: dns.Question
    ) Result[dns.Message, DoqError]

    fn Close() Result[void, DoqError]
}
```

## 15.3 `Query`

`Query`:

- creates one ordinary DNS `QUERY`;
- sets DNS Message ID to zero;
- opens a new bidirectional QUIC stream;
- sends one framed query;
- FINs the send side;
- expects exactly one DNS response;
- expects server FIN after that response;
- returns the response regardless of DNS RCODE.

For an ordinary single-response query, a second response frame is a protocol
error.

## 15.4 `Exchange`

`Exchange` is for explicit DNS messages that are known to have one response.

It:

- forces DoQ Message ID zero;
- opens one transaction stream;
- expects exactly one response;
- validates final FIN.

It is not the API for AXFR/IXFR.

## 15.5 `OpenTransaction`

`OpenTransaction` exposes the underlying DoQ multi-response transaction
semantics needed by:

- zone transfer;
- future DNS operations that legitimately return multiple response messages.

It does not expose raw `quic.Stream` to ordinary DNS callers.

This preserves DoQ protocol validation while supporting RFC 9250's
multi-response model.

## 15.6 Parallel queries

A conforming performance implementation should issue independent queries
concurrently on separate streams.

It must not:

```text
send query A
wait for A
send query B
wait for B
```

unless resource limits or application ordering explicitly require it.

QUIC stream independence is one of DoQ's core latency advantages.

## 15.7 Message IDs

DoQ query and response Message IDs must be zero.

The client does not allocate unique 16-bit transaction IDs.

The QUIC stream identifies the DNS transaction.

Therefore DoQ is not limited to 65536 outstanding transactions by the DNS ID
space.

Actual concurrency is bounded by:

- QUIC stream limits;
- flow control;
- Sec resource limits;
- implementation policy.

## 15.8 Connection fallback

RFC 9250 allows broader clients to fall back to other DNS transports depending
on usage profile.

`doq.Client` itself never silently becomes:

```text
DoT
classic TCP DNS
classic UDP DNS
```

Fallback belongs to an explicit shared resolver transport policy.

## 15.9 Capability-failure memory

RFC 9250 recommends remembering servers/IPs that do not support DoQ for a
reasonable period.

The client may keep a bounded capability-failure cache.

The cache:

- is keyed at least by relevant server/address/network context;
- is invalidated by explicit configuration change;
- must not permanently blacklist a server;
- must not turn a Strict DoQ requirement into cleartext fallback;
- should avoid repeated expensive QUIC handshakes to a known unsupported
  endpoint.

The exact default suppression period remains implementation policy; RFC 9250
gives approximately one hour as an example.

## 15.10 Close

`Client.Close()`:

- stops new transactions;
- closes the reusable QUIC connection;
- completes affected pending operations exactly once;
- is idempotent;
- releases session/connection resources according to QUIC ownership rules.

---

# Part IX — 0-RTT and session resumption

# 16. 0-RTT

RFC 9250 explicitly permits DoQ 0-RTT for replayable transactions.

Sec keeps this explicit because 0-RTT has privacy/replay trade-offs.

Default:

```sec
ZeroRttPolicy.Disabled
```

## 16.1 Enabled policy

When:

```sec
ZeroRttPolicy.ReplayableTransactions
```

is selected, only:

```sec
dns.Opcode.Query
dns.Opcode.Notify
```

may be sent in 0-RTT according to RFC 9250.

Any other OPCODE must wait until the QUIC handshake is complete.

The implementation must not infer replay-safety from method naming or caller
intent.

It inspects the DNS message opcode.

## 16.2 Server handling

A server receiving non-replayable traffic in 0-RTT must follow RFC 9250.

Permitted compliant behavior includes:

- queue until handshake completion;
- return DNS REFUSED plus Extended DNS Error "Too Early";
- close with `DoqErrorCode.ProtocolError`.

Sec server configuration may choose among compliant behaviors when the
canonical server API is finalized.

It must not immediately process a non-replayable 0-RTT transaction as normal.

## 16.3 Session resumption

Session resumption is enabled by default where supported by `net/quic`.

Privacy requirements:

- resumption tickets should be single-use where supported;
- by default, resumption should not continue across a detected connectivity
  change;
- address-validation tokens should be coordinated with resumption to reduce
  incremental tracking risk.

## 16.4 Connectivity change

With default:

```sec
MigrationPolicy.RestartOnNetworkChange
```

the client discards/replaces the DoQ connection after network-context change.

It must not use connection migration merely because QUIC technically can.

---

# Part X — Server-side DoQ

# 17. `listener.sec`

## 17.1 Responsibility

`listener.sec` exposes the server transport adapter.

It does not implement an authoritative or recursive DNS engine.

## 17.2 `Listener`

Conceptual API:

```sec
@noCopy
type Listener struct {
    // Private QUIC listener + DoQ application configuration.
}

impl Listener {
    static fn Bind(
        endpoint: ip.Endpoint,
        config: <- quic.ServerConfig
    ) Result[Listener, DoqError]

    fn Accept() Result[ServerConnection, DoqError]

    fn Close() Result[void, DoqError]
}
```

The exact `quic.ServerConfig` type is provisional until `std-net-quic.md`
locks the canonical QUIC API.

Semantics are locked:

- UDP/QUIC endpoint is owned by `net/quic`;
- server negotiates ALPN `doq`;
- only an established DoQ connection is exposed.

## 17.3 `ServerConnection`

```sec
@noCopy
type ServerConnection struct {
    // Private established QUIC connection + resource limits.
}
```

Canonical semantic operation:

```sec
impl ServerConnection {
    fn AcceptTransaction()
        Result[ServerTransaction, DoqError]

    fn Close() Result[void, DoqError]
}
```

## 17.4 `ServerTransaction`

```sec
@noCopy
type ServerTransaction struct {
    // Private client-initiated bidirectional QUIC stream.
}
```

Canonical API:

```sec
impl ServerTransaction {
    fn ReadQuery() Result[dns.Message, DoqError]

    fn WriteResponse(
        response: ref dns.Message
    ) Result[void, DoqError]

    fn Finish() Result[void, DoqError]

    fn Cancel(
        code: DoqErrorCode
    ) Result[void, DoqError]
}
```

## 17.5 Server transaction rules

`ReadQuery()`:

- reads exactly one framed DNS query;
- requires Message ID zero;
- requires client send-side FIN after the query;
- rejects a second query on the same stream.

`WriteResponse()`:

- may be called one or more times;
- writes one framed response each time;
- requires Message ID zero.

`Finish()`:

- sends the final stream FIN;
- indicates no more responses.

Ordinary queries normally produce one response.

Zone transfers may produce many.

## 17.6 Illegal streams

A DoQ server must treat as protocol error:

- client-created unidirectional application stream;
- server attempt to use a server-initiated bidirectional stream for DoQ
  transaction traffic.

The DoQ server accepts only client-initiated bidirectional transaction streams.

---

# Part XI — Flow control and resource limits

# 18. QUIC flow control

`net/quic` owns flow-control mechanics.

DoQ must configure/use them with DNS workload semantics.

Limits exist at:

- connection data level;
- stream data level;
- concurrent stream count.

## 18.1 Request concurrency

The allowed client-initiated bidirectional stream count naturally limits the
number of outstanding DoQ transactions.

Servers must choose limits that:

- prevent resource exhaustion;
- permit useful parallelism.

## 18.2 Zone transfers

Zone transfers can be large and can run concurrently.

Flow-control values suitable for tiny stub queries may be too small for AXFR
or IXFR.

The `net/dns/zone` integration must therefore be able to request/configure
appropriate DoQ resource policy without exposing raw QUIC transport parameter
integers directly to DNS callers.

## 18.3 Excessive load

Implementations may close streams/connections due to excessive resource use
with:

```sec
DoqErrorCode.ExcessiveLoad
```

where appropriate.

Resource exhaustion must remain bounded and must not cause uncontrolled memory
growth.

---

# Part XII — EDNS interaction

# 19. UDP payload size

The EDNS UDP payload-size field is ignored for DoQ transport sizing.

DoQ always uses the DNS message maximum:

```text
65535 bytes
```

subject to a smaller configured local maximum.

The library must not truncate a DoQ response merely to fit a client's EDNS UDP
size advertisement.

## 19.1 EDNS TCP keepalive

The EDNS(0) TCP Keepalive option defined by RFC 7828 must not be sent over DoQ.

Receipt of:

```text
edns-tcp-keepalive
```

on a DoQ connection is a protocol error under RFC 9250.

DoQ connection lifetime is managed by QUIC idle timeout and connection
management.

---

# Part XIII — Resolver integration

# 20. `transport.sec`

## 20.1 Common DNS transport contract

As established by the DoH and DoT child books, the parent `net/dns` resolver
must separate:

```text
DNS resolver semantics/cache/policy
```

from:

```text
DNS query transport
```

Semantic parent contract:

```sec
interface QueryTransport {
    fn Exchange(
        request: ref Message
    ) Result[Message, error]
}
```

## 20.2 DoQ conformance

`doq.Client.Exchange` satisfies the ordinary one-response resolver transport
contract.

The resolver engine therefore reuses:

- CNAME following;
- address aggregation;
- positive/negative caching;
- reverse lookup;
- RR filtering;
- DNS result handling.

## 20.3 No `doq.Resolver`

Revision 0.1 deliberately does not define:

```sec
doq.Resolver
```

DoQ is a transport.

High-level DNS resolution remains in `net/dns`.

## 20.4 Multi-response operations

The one-response `QueryTransport` interface is not sufficient for AXFR/IXFR.

Zone transfer uses:

```sec
doq.Client.OpenTransaction(...)
```

through the future `net/dns/zone` transport integration.

The parent resolver should not widen ordinary stub lookup into a multi-message
stream abstraction solely because zone transfer exists.

---

# Part XIV — Discovery

# 21. RFC 9461

A validated SVCB record may indicate:

```text
alpn=doq
port=...
ipv4hint=...
ipv6hint=...
```

If `port` is absent, DoQ default is 853.

The discovery layer must preserve:

- Authentication Domain Name;
- selected target;
- port;
- address hints;
- QUIC application protocol.

Address hints are connection candidates, not authenticated identities.

## 21.1 No ALPN default in SVCB mapping

RFC 9461 requires an applicable ALPN indication for this mapping.

A discovery result without a supported protocol indication must not be assumed
to mean DoQ.

## 21.2 Shared discovery

DDR belongs to a shared encrypted-DNS discovery package/facility because it can
select among:

```text
DoQ
DoT
DoH
```

Possible future package:

```text
net/dns/discovery
```

The exact path remains an observation item.

---

# Part XV — Zone transfer

# 22. AXFR and IXFR

RFC 9250 explicitly supports DNS zone transfer over DoQ.

Requirements include support for concurrent transfers over one QUIC
connection.

DoQ servers must be able to handle multiple concurrent:

```text
AXFR
IXFR
```

transactions on separate streams.

## 22.1 Stream mapping

One zone-transfer request uses one bidirectional DoQ stream.

The server may emit multiple framed DNS responses on that stream.

Other transfers use separate streams and may progress concurrently.

Responses from different transfers may therefore be interleaved at the QUIC
packet/connection level while remaining isolated by stream.

## 22.2 API ownership

High-level transfer semantics belong to:

```text
net/dns/zone
```

The DoQ package supplies the transport primitive:

```sec
Transaction.NextResponse()
```

needed to carry multiple messages.

It does not implement zone database semantics.

---

# Part XVI — Connection migration and privacy

# 23. Migration default

QUIC supports connection migration.

For DoQ, migration can link DNS activity across network changes.

Sec therefore defaults:

```sec
MigrationPolicy.RestartOnNetworkChange
```

This follows the RFC 9250 recommendation to prioritize privacy for clients.

## 23.1 Opt-in migration

```sec
MigrationPolicy.AllowConnectionMigration
```

permits `net/quic` to migrate the existing DoQ connection according to its
normal validated path-migration rules.

This may be useful for:

- fixed infrastructure;
- latency-sensitive systems;
- environments where client tracking across address changes is not a concern.

The policy is explicit because it changes privacy behavior.

---

# Part XVII — Address validation

# 24. QUIC address validation

DoQ inherits QUIC anti-amplification protections.

Servers must conform to QUIC address-validation requirements.

RFC 9250 recommends considering QUIC Retry packets and future-connection
validation tokens.

The DoQ package does not reimplement these mechanisms.

## 24.1 Retry policy

A server configuration may request QUIC Retry behavior through canonical
`net/quic` server options.

DoQ does not expose a raw Retry packet API.

## 24.2 Tracking considerations

Address-validation tokens can become tracking signals.

The DoQ package must follow its privacy policy when deciding whether to reuse
tokens across changing connectivity.

---

# Part XVIII — Security and privacy

# 25. Authentication default

Stub-to-recursive DoQ defaults to Strict authenticated operation.

The client must authenticate the configured resolver identity through the
QUIC/TLS security model.

A bootstrap/address hint does not change that identity.

## 25.1 Opportunistic mode

Opportunistic operation may be explicitly selected.

It must not be described as authenticated unless authentication actually
succeeds.

The exact security-state type should be shared with encrypted DNS transports
if the parent DNS design later introduces a common one.

Revision 0.1 does not duplicate the DoT `SecurityState` type into DoQ.

---

# 26. Traffic analysis

QUIC encryption does not hide packet sizes and timing.

RFC 9250 requires padding mitigation.

Therefore no ordinary safe DoQ option disables all padding.

## 26.1 Query aggregation

The implementation must not assume one DNS stream frame maps to one QUIC
packet.

QUIC may aggregate stream data, acknowledgements, and other frames according
to transport behavior.

This is one reason packet-level padding is preferred when the QUIC API can
support it properly.

---

# 27. 0-RTT privacy

0-RTT is replayable and session resumption can create linkability.

Revision 0.1 therefore defaults 0-RTT off.

Explicit opt-in permits only the RFC 9250 replayable opcode set.

The package documentation must make clear that enabling 0-RTT is a
latency/privacy trade-off.

---

# 28. Session linkability

Session-resumption tickets can allow correlation across sessions.

The implementation should:

- use tickets once where feasible;
- avoid resumption after connectivity change by default;
- avoid long-lived token reuse beyond the canonical QUIC/TLS policy.

---

# 29. No silent downgrade

`doq.Client` never silently falls back to:

```text
DoT
DoH
DNS/TCP
DNS/UDP
```

A higher-level resolver may have an explicit fallback policy.

That policy must preserve the distinction between:

- authenticated encrypted DNS;
- opportunistically encrypted DNS;
- cleartext DNS.

---

# Part XIX — Ownership, cancellation, and concurrency

# 30. Ownership

The following are owning `@noCopy` resources:

```sec
Client
Connection
Transaction
Listener
ServerConnection
ServerTransaction
```

They own or exclusively control QUIC resources.

Configuration values such as:

```sec
Server
Bootstrap
ClientOptions
AuthenticationPolicy
ZeroRttPolicy
MigrationPolicy
PaddingPolicy
```

are ordinary semantic values.

---

# 31. Cancellation

Sec execution cancellation of an outstanding ordinary DoQ transaction must
map to the DoQ stream-cancellation semantics where a transaction stream exists.

The implementation should:

1. resolve the Sec cancellation/operation commit race;
2. if the transaction is still active, issue `STOP_SENDING` with
   `RequestCancelled`;
3. abandon the DNS result;
4. release local stream state;
5. leave unrelated streams/connection alive.

No package-specific cancellation token is introduced.

## 31.1 Cancellation after response commit

If the complete expected response has already committed before cancellation,
the normal committed result wins according to Sec's general cancellation
rules.

If cancellation wins first, a later arriving response is discarded.

---

# 32. Parallelism

DoQ is designed for concurrent queries.

Each ordinary DNS transaction has an independent QUIC stream.

This avoids TCP-level application head-of-line blocking between DNS
transactions.

The implementation should not add a package-global mutex that serializes all
query completion.

Shared connection bookkeeping must still be synchronized safely.

---

# Part XX — Platform capability

# 33. Active requirements

Active DoQ requires:

- UDP/IP networking sufficient for QUIC;
- canonical `net/quic`;
- QUIC TLS 1.3 security;
- cryptographic backend;
- server authentication capability for Strict mode;
- bootstrap path for named resolvers.

A target without these capabilities must reject active DoQ usage.

It must not emulate DoQ with classic UDP DNS.

## 33.1 Pure facilities

Pure source-level facilities such as:

- error-code values;
- framing;
- configuration validation

may exist without an active QUIC runtime.

---

# Part XXI — Explicit exclusions

# 34. No UDP port 53 DoQ

DoQ must not use UDP port 53.

Alternative ports require explicit agreement/discovery.

## 34.1 No implicit port 443

Port 443 is not a second default.

It may be configured or discovered explicitly.

---

# 35. No HTTP layer

DoQ has no HTTP semantics.

DoH remains separate.

---

# 36. No server-initiated DNS transactions

DoQ does not permit the server to create independent server-initiated DNS
transactions.

All transaction streams are client-initiated bidirectional streams.

---

# 37. No QUIC unidirectional application streams

DoQ transaction traffic does not use QUIC unidirectional streams.

Attempted use is a protocol error.

---

# 38. No EDNS TCP keepalive

The EDNS TCP Keepalive option must not be sent over DoQ.

QUIC owns connection lifetime.

---

# 39. No raw QUIC leakage

Ordinary DNS users must not need to manipulate:

- stream IDs;
- QUIC packet numbers;
- raw connection IDs;
- STOP_SENDING frame bytes;
- raw QUIC application error integers.

DoQ exposes typed DNS transaction semantics.

Advanced lower-level QUIC use remains available through `net/quic`.

---

# Part XXII — Repository implementation

# 40. Required source structure

Create:

```text
sec/stdlib/net/dns/doq/
├── std-net-dns-doq.md
├── error.sec
├── protocol.sec
├── server.sec
├── bootstrap.sec
├── options.sec
├── framing.sec
├── padding.sec
├── stream.sec
├── connection.sec
├── client.sec
├── listener.sec
└── transport.sec
```

If later repository state already contains files:

- inspect them;
- preserve useful implementation;
- update/correct them;
- do not preserve obsolete API solely for compatibility.

---

# 41. Parent DNS correction

The parent `std-net-dns.md` must support a transport-neutral resolver engine.

DoQ uses the same ordinary resolver logic as:

```text
classic DNS
DoT
DoH
```

Do not introduce:

```sec
doq.Resolver
```

to avoid fixing the parent abstraction.

---

# 42. QUIC dependency

`std-net-quic.md` is not yet locked.

References in this book such as:

```sec
quic.ServerConfig
quic.ApplicationProtocol
quic.Connection
```

are semantic/provisional type names.

When the canonical `net/quic` API is defined, this book must be synchronized
mechanically without changing the DoQ application semantics defined here.

Important required QUIC capabilities are already known:

- client/server connection creation;
- ALPN;
- client-initiated bidirectional streams;
- stream FIN;
- STOP_SENDING;
- RESET_STREAM;
- application CONNECTION_CLOSE codes;
- connection reuse;
- flow control;
- session resumption;
- optional 0-RTT;
- address validation;
- optional packet padding control;
- connection migration control;
- QUIC version negotiation.

---

# Part XXIII — Tests

# 43. Protocol tests

At minimum:

- ALPN is `doq`;
- default UDP port is typed `ip.Port(853)`;
- UDP port 53 rejected as implicit/default DoQ;
- known DoQ application error-code numeric values;
- unknown DoQ application error code remains representable;
- QUIC version 1 compatible;
- QUIC version 2 compatible where `net/quic` supports it.

---

# 44. Framing tests

At minimum:

- two-byte length prefix;
- length prefix split across QUIC reads;
- DNS body split across reads;
- multiple response frames on one stream;
- 65535-byte maximum;
- >65535 rejected;
- zero/invalid message length handling;
- premature FIN;
- no dependence on EDNS UDP size.

---

# 45. Stream tests

At minimum:

- one client-initiated bidirectional stream per query;
- query Message ID zero;
- response Message ID zero;
- nonzero query ID rejected by server;
- nonzero response ID rejected by client;
- client FIN after query;
- server FIN after final response;
- second query on one stream -> protocol error;
- server-initiated bidirectional stream -> protocol error;
- unidirectional stream -> protocol error;
- multi-response stream for zone-transfer style operation.

---

# 46. Cancellation tests

At minimum:

- Sec cancellation maps to STOP_SENDING;
- application error code is RequestCancelled;
- response arriving after cancellation is discarded;
- cancellation does not close unrelated streams;
- cancellation after response commit preserves committed result;
- server stops/abandons transaction after cancellation according to RFC 9250.

---

# 47. Client tests

At minimum:

- A query;
- AAAA query;
- NXDOMAIN remains valid DNS response;
- SERVFAIL remains valid DNS response;
- concurrent independent queries;
- out-of-order completion;
- connection reuse;
- connection close abandons all in-progress transactions exactly once;
- no DNS-ID allocator required;
- bounded outstanding streams;
- no silent fallback to another DNS transport.

---

# 48. 0-RTT tests

At minimum:

- default disabled;
- explicit replayable policy;
- QUERY permitted in 0-RTT;
- NOTIFY permitted in 0-RTT;
- UPDATE not sent in 0-RTT;
- other non-replayable OPCODEs not sent in 0-RTT;
- server compliant handling of invalid non-replayable 0-RTT;
- no accidental replay of arbitrary `Exchange` traffic.

---

# 49. Padding tests

At minimum:

- Automatic chooses QUIC padding when supported;
- Automatic falls back to EDNS padding;
- explicit QuicPacket without support fails rather than disables privacy;
- EDNS padding uses standards-based fixed-size policy;
- response padding present under compliant server policy;
- no ordinary Disabled padding policy exists.

---

# 50. Migration/privacy tests

At minimum:

- default RestartOnNetworkChange;
- network change creates new session under default policy;
- explicit AllowConnectionMigration uses canonical QUIC migration;
- resumption not reused across connectivity change by default;
- bootstrap/authentication identity remains stable after address changes.

---

# 51. Listener/server tests

At minimum:

- QUIC listener on UDP 853/test port;
- ALPN doq required;
- one accepted connection handles multiple query streams;
- server handles query streams concurrently;
- server writes multiple response frames for transfer-style transaction;
- server FIN after final response;
- connection-level protocol error closes with ProtocolError;
- excessive-load handling bounded.

---

# 52. Zone-transfer tests

When `net/dns/zone` integration exists:

- AXFR on one stream with multiple messages;
- concurrent AXFR requests on one QUIC connection;
- concurrent IXFR requests;
- AXFR and IXFR in parallel;
- one transfer cancellation leaves others running;
- flow-control limits do not deadlock reasonable transfers.

---

# 53. Discovery tests

When RFC 9461/9462 support exists:

- `alpn=doq` selected;
- absent port -> 853;
- explicit port respected through validated discovery policy;
- ipv4hint/ipv6hint treated as connection candidates;
- Authentication Domain Name preserved;
- DoQ/DoT/DoH share one discovery implementation.

---

# Part XXIV — AI/repository implementation instructions

# 54. Implementation requirements

An AI or repository automation implementing this book must:

1. treat this book as the normative DoQ package specification;
2. create the declared source files when missing;
3. inspect and update existing files rather than blindly overwrite them;
4. add RFC/IANA URLs to every relevant source file;
5. reuse `net/dns` message/record/codec semantics;
6. reuse canonical `net/quic`;
7. never implement a private QUIC stack inside DoQ;
8. use ALPN `doq`;
9. use typed default UDP port `ip.Port(853)`;
10. never use UDP port 53 for DoQ;
11. open one client-initiated bidirectional QUIC stream per DNS query;
12. use two-byte DNS length framing on every DoQ DNS message;
13. force DNS Message ID zero on DoQ;
14. reject nonzero received DoQ Message IDs as protocol violations;
15. require client FIN after query and server FIN after final response;
16. support multi-response transaction streams;
17. implement RequestCancelled through QUIC STOP_SENDING semantics;
18. preserve open DoQ application error-code values;
19. support connection reuse;
20. support bounded concurrent query streams;
21. implement DoQ flow/resource limits;
22. never send EDNS TCP Keepalive over DoQ;
23. ignore EDNS UDP payload-size field for DoQ transport sizing;
24. enforce 65535-byte DNS message maximum;
25. implement RFC 9250-compliant traffic-analysis padding;
26. default padding to Automatic with no safe Disabled mode;
27. default authentication to Strict for stub use;
28. default 0-RTT to Disabled;
29. when 0-RTT is enabled, restrict it to RFC 9250 replayable OPCODEs;
30. default connection migration to restart on network change;
31. never silently fall back to DoT/DoH/classic DNS;
32. reuse the shared DNS resolver engine rather than create `doq.Resolver`;
33. preserve zone-transfer transport capability for `net/dns/zone`;
34. support QUIC version 2 through `net/quic` when available;
35. add all required tests;
36. update `implementation-status-std-net-dns-doq.yaml`;
37. reconcile status into canonical `implementation-status.yaml` without
    replacing stronger/newer evidence;
38. report missing QUIC compiler/platform functionality instead of mocking
    successful DoQ behavior.

---

# 55. Recommended implementation order

1. create package/book/files;
2. implement protocol/error constants;
3. implement server/bootstrap/options;
4. implement DNS frame codec;
5. integrate `net/quic` ALPN/configuration;
6. establish Strict authenticated DoQ connection;
7. implement one transaction stream;
8. implement `Transaction.NextResponse`;
9. implement cancellation/STOP_SENDING;
10. implement reusable `Connection`;
11. implement `Client.Query`;
12. implement `Client.Exchange`;
13. implement concurrent stream transactions;
14. implement padding policy;
15. implement session resumption;
16. implement explicit replayable-only 0-RTT;
17. implement migration privacy policy;
18. implement server Listener/ServerTransaction;
19. integrate common DNS resolver transport;
20. integrate `net/dns/zone` multi-response transfer;
21. integrate shared RFC 9461/9462 discovery;
22. test QUIC v1 and QUIC v2 through the canonical QUIC package.

---

# Part XXV — Open observations

# 56. Items intentionally not fully locked in revision 0.1

The following remain open:

- final `net/quic` public type names;
- final `quic.ClientConfig` / `quic.ServerConfig` shapes;
- shared encrypted-DNS authentication/security-state types;
- explicit SPKI pinning API through QUIC/TLS configuration;
- DANE authentication for DoQ;
- common DDR/discovery package path;
- exact parent `dns.QueryTransport` syntax;
- resolver cross-transport fallback policy;
- exact DoQ capability-failure cache duration;
- exact fixed padding-size set;
- whether QUIC packet padding can be mandated on targets that expose it;
- exact flow-control configuration API for zone transfer;
- explicit max-concurrent-transaction option;
- server 0-RTT policy selector;
- QUIC Retry defaults for DoQ servers;
- connection pool with multiple simultaneous QUIC connections to one resolver;
- preferred QUIC v1/v2 selection policy;
- stateful DNS operations beyond ordinary Query/Notify;
- authenticated recursive-to-authoritative DoQ policy as standards evolve.

These observations are not permission to invent incompatible public APIs.

---

# 57. Decisions established by revision 0.1

Revision 0.1 establishes:

- canonical package path `net/dns/doq`;
- RFC 9250 is the primary DoQ standard;
- DoQ reuses `net/dns`, `net/ip`, and `net/quic`;
- DoQ is direct DNS-over-QUIC, not HTTP/3;
- registered ALPN is `doq`;
- default service is UDP port 853;
- DoQ must not use UDP port 53;
- port 443 is not an implicit second default;
- each DNS query uses a new client-initiated bidirectional QUIC stream;
- no server-initiated transaction streams are allowed;
- no DoQ unidirectional application streams are allowed;
- every DNS message uses the two-byte DNS length prefix;
- query and response DNS Message IDs are zero;
- maximum DNS message payload is 65535 bytes;
- EDNS UDP payload size is ignored for DoQ transport sizing;
- EDNS TCP Keepalive is forbidden on DoQ;
- `Transaction` supports multiple response messages and therefore zone
  transfer transport semantics;
- ordinary `Query`/`Exchange` expect one response;
- cancellation maps to DoQ/QUIC stream cancellation using
  `DOQ_REQUEST_CANCELLED`;
- DoQ application error codes are modeled as an open wire namespace;
- connection reuse is required as normal behavior;
- concurrent streams are the normal high-performance model;
- valid nonzero DNS RCODEs remain DNS responses;
- Sec stub clients default to Strict authentication;
- Sec revision 0.1 defaults 0-RTT off;
- explicit 0-RTT is limited to RFC 9250 replayable QUERY/NOTIFY operations;
- session resumption is enabled where safely available;
- Sec defaults to starting a new session after connectivity change instead of
  QUIC connection migration;
- connection migration is explicit opt-in;
- DoQ traffic-analysis padding is mandatory;
- Automatic padding prefers QUIC packet padding and falls back to EDNS padding;
- no ordinary safe option disables all padding;
- no silent fallback to another DNS transport exists;
- DoQ integrates with the shared parent DNS resolver engine;
- no `doq.Resolver` is introduced;
- AXFR/IXFR high-level semantics remain owned by `net/dns/zone`;
- RFC 9461/9462 discovery remains shared across encrypted DNS transports;
- QUIC version selection remains owned by `net/quic`;
- DoQ is compatible with QUIC version 2 as specified by RFC 9369.

---

**Document revision:** 0.1  
**Last updated:** 2026-09-07
