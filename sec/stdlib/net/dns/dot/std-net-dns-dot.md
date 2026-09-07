# Sec Standard Library — DNS over TLS

- **Status:** Draft
- **Created:** 2026-09-07
- **Last updated:** 2026-09-07
- **Document revision:** 0.1
- **Sec language version:** 0.1
- **Canonical path:** `stdlib/net/dns/dot/std-net-dns-dot.md`
- **Repository path:** `sec/stdlib/net/dns/dot/std-net-dns-dot.md`
- **Parent specification:** `stdlib/net/dns/std-net-dns.md`
- **Sibling specifications:** `stdlib/net/dns/doh/std-net-dns-doh.md`, `stdlib/net/dns/doq/std-net-dns-doq.md`
- **Implementation-status fragment:** `implementation-status-std-net-dns-dot.yaml`
- **Latest verified repository main:** `0f5027d`
- **Standards state rechecked:** 2026-09-07

---

# 1. Purpose

`net/dns/dot` implements DNS over Transport Layer Security (DoT).

The package maps ordinary DNS-over-TCP framing onto an authenticated or
opportunistically encrypted TLS connection according to the DoT standards.

It owns:

- configured DoT server identity;
- bootstrap address handling;
- DoT privacy profile selection;
- TLS-backed DNS transport;
- DNS-over-TCP message framing inside TLS;
- connection establishment and reuse;
- multiple DNS exchanges over a TLS connection;
- transaction matching;
- strict and opportunistic authentication policy;
- DoT-specific transport errors;
- server-side TLS-wrapped DNS transport adaptation;
- integration with the common `net/dns` resolver engine.

The package is imported as:

```sec
import "net/dns/dot"
```

Typical direct use:

```sec
let server := try dot.Server.Parse("resolver.example")
let client := try dot.Client.New(
    server,
    dot.ClientOptions.Default()
)

let message := try client.Query(
    new dns.Question(
        try dns.Name.Parse("example.com"),
        dns.RecordType.A,
        dns.DnsClass.IN
    )
)
```

`net/dns/dot` does not reimplement:

- DNS message parsing;
- DNS records;
- DNS caching;
- TCP;
- TLS;
- certificates;
- system trust stores.

Those remain owned by their canonical packages.

---

# 2. Package boundary

## 2.1 Dependencies

`net/dns/dot` depends on:

```text
net/dns
net/ip
```

and on the canonical TLS package once its final stdlib ownership is locked.

Conceptually:

```text
net/dns       <- net/dns/dot
net/ip        <- net/dns/dot
TLS package   <- net/dns/dot
```

The base `net/dns` package must not import `net/dns/dot`.

The DoT package may satisfy the shared DNS query-transport contract defined by
the parent DNS package, but parent code must not depend on the child package.

## 2.2 Sibling encrypted DNS transports

DoT is distinct from:

```text
net/dns/doh
net/dns/doq
```

The package must not become a generic encrypted-DNS namespace.

DoH maps DNS to HTTP.

DoQ maps DNS directly to QUIC streams.

DoT maps DNS-over-TCP framing to TLS over TCP.

## 2.3 DTLS is not DoT

DNS over DTLS is defined separately by RFC 8094.

It is not silently treated as part of `net/dns/dot`.

A later package may be:

```text
net/dns/dtls
```

if Sec chooses to support it.

---

# 3. Standards and registries

## 3.1 Primary DoT standard

Primary protocol specification:

- **RFC 7858 — Specification for DNS over Transport Layer Security (TLS)**
- https://www.rfc-editor.org/rfc/rfc7858
- https://www.rfc-editor.org/info/rfc7858

RFC 7858 defines:

- DNS over TLS on TCP;
- the dedicated DoT service;
- immediate TLS handshake;
- no cleartext DNS on a DoT port;
- DNS-over-TCP framing inside TLS;
- connection reuse;
- session resumption guidance;
- privacy usage profiles.

## 3.2 Usage profiles and authentication

Primary update:

- **RFC 8310 — Usage Profiles for DNS over TLS and DNS over DTLS**
- https://www.rfc-editor.org/rfc/rfc8310
- https://www.rfc-editor.org/info/rfc8310

RFC 8310 updates RFC 7858.

It defines:

- Strict Privacy;
- Opportunistic Privacy;
- Authentication Domain Names;
- PKIX-based authentication;
- SPKI pinning;
- DANE-based authentication;
- TLS protocol-profile requirements.

Where RFC 8310 and RFC 7858 differ, RFC 8310 takes precedence for the areas it
updates.

## 3.3 DNS over TCP

DoT uses the DNS-over-TCP framing model.

Relevant standard:

- **RFC 7766 — DNS Transport over TCP - Implementation Requirements**
- https://www.rfc-editor.org/rfc/rfc7766

A DoT implementation must not assume one TLS connection contains exactly one
DNS query.

## 3.4 DNS wire format

The DNS message carried inside the TLS stream is the ordinary DNS wire message
defined by the base DNS package.

Relevant baseline:

- RFC 1035  
  https://www.rfc-editor.org/rfc/rfc1035

The TLS layer does not alter DNS message semantics.

## 3.5 TLS

The TLS implementation is not owned by this package.

Relevant standards include:

- **RFC 8446 — TLS 1.3**  
  https://www.rfc-editor.org/rfc/rfc8446
- **RFC 9325 — Recommendations for Secure Use of TLS and DTLS / BCP 195**  
  https://www.rfc-editor.org/rfc/rfc9325
- **RFC 9852 — New Protocols Using TLS Must Require TLS 1.3**  
  https://www.rfc-editor.org/rfc/rfc9852
- **RFC 10015 — Deprecating Obsolete Key Exchange Methods in TLS 1.2 and DTLS 1.2**  
  https://www.rfc-editor.org/rfc/rfc10015

RFC 8310 references RFC 7525, but RFC 7525 is obsolete and replaced by the
current BCP 195 guidance.

Therefore Sec DoT follows:

1. DoT-specific requirements from RFC 7858 and RFC 8310;
2. current applicable BCP 195 TLS guidance;
3. the canonical Sec TLS package security policy.

DoT predates RFC 9852 and therefore remains an existing protocol rather than a
new protocol created by Sec. The DoT package must not independently relax the
canonical TLS package's current minimum-security defaults.

## 3.6 Port registry

IANA service registration:

```text
service: domain-s
transport: TCP
port: 853
```

Registry:

https://www.iana.org/assignments/service-names-port-numbers

Port 853 is the conventional DoT port.

The source code must expose this as a typed constant using `ip.Port`, not
scatter integer `853` throughout the implementation.

## 3.7 TLS ALPN

IANA registers:

```text
dot
```

as the TLS ALPN protocol identifier for DNS-over-TLS.

Registry:

https://www.iana.org/assignments/tls-extensiontype-values

RFC 9461 uses the `dot` ALPN value when advertising DoT through DNS SVCB.

The canonical TLS ALPN representation must be reused.

DoT must not duplicate ALPN as an arbitrary package-local string API.

## 3.8 Discovery

Relevant standards:

- **RFC 9461 — Service Binding Mapping for DNS Servers**  
  https://www.rfc-editor.org/rfc/rfc9461
- **RFC 9462 — Discovery of Designated Resolvers**  
  https://www.rfc-editor.org/rfc/rfc9462

RFC 9461 defines SVCB discovery for DoT including:

```text
alpn=dot
```

and default port 853.

RFC 9462 defines discovery policy spanning DoT, DoH, and DoQ.

DDR therefore does not belong solely inside `net/dns/dot`.

## 3.9 DANE

RFC 8310 permits DANE-based server authentication.

Relevant specifications include:

- RFC 6698 — TLSA  
  https://www.rfc-editor.org/rfc/rfc6698
- RFC 7671 — DANE operational guidance  
  https://www.rfc-editor.org/rfc/rfc7671

Full DANE validation depends on authenticated DNSSEC data and therefore crosses
the boundary into `net/dns/dnssec`.

---

# 4. Source-file standards headers

Every `.sec` file in this package must identify its governing standards.

Example:

```sec
// Sec Standard Library: net/dns/dot
// File: client.sec
//
// Standards:
//   RFC 7858 - Specification for DNS over TLS
//   https://www.rfc-editor.org/rfc/rfc7858
//
//   RFC 8310 - Usage Profiles for DNS over TLS and DNS over DTLS
//   https://www.rfc-editor.org/rfc/rfc8310
//
//   RFC 7766 - DNS Transport over TCP
//   https://www.rfc-editor.org/rfc/rfc7766
//
// TLS guidance:
//   RFC 9325 - BCP 195
//   https://www.rfc-editor.org/rfc/rfc9325
//
//   RFC 9852
//   https://www.rfc-editor.org/rfc/rfc9852
//
//   RFC 10015
//   https://www.rfc-editor.org/rfc/rfc10015

module dot
```

TLS algorithm details should not be copied into every DoT source file. The
canonical TLS package remains authoritative for actual TLS algorithm policy.

---

# 5. Package and file manifest

Initial package structure:

```text
stdlib/net/dns/dot/
├── std-net-dns-dot.md
├── error.sec
├── profile.sec
├── server.sec
├── bootstrap.sec
├── authentication.sec
├── options.sec
├── framing.sec
├── connection.sec
├── client.sec
├── listener.sec
└── transport.sec
```

All package source files use:

```sec
module dot
```

Expected tests:

```text
profile_test.sec
server_test.sec
bootstrap_test.sec
authentication_test.sec
framing_test.sec
connection_test.sec
client_test.sec
listener_test.sec
transport_test.sec
```

---

# Part I — Errors and privacy profiles

# 6. `error.sec`

## 6.1 Responsibility

`error.sec` owns failures specific to:

- DoT configuration;
- privacy-profile enforcement;
- authentication;
- framing;
- TLS-backed DNS transport;
- local DoT connection state.

It does not duplicate:

```sec
dns.MessageError
ip.NetworkError
```

or the canonical TLS package's error types.

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

## 6.4 `AuthenticationError`

```sec
enum AuthenticationError error {
    MissingAuthenticationInformation,
    NameMismatch,
    CertificateRejected,
    PinMismatch,
    DaneUnavailable,
    DaneValidationFailed,
    AuthenticationRequired,
}
```

Specific certificate/TLS details should remain available through the canonical
TLS error when that package exposes a more precise cause.

## 6.5 `FramingError`

```sec
enum FramingError error {
    MessageTooLarge,
    InvalidLength,
    UnexpectedEof,
    OutputTooSmall,
}
```

## 6.6 `DotError`

```sec
union DotError error {
    Server(ServerError),
    Bootstrap(BootstrapError),
    Authentication(AuthenticationError),

    Network(ip.NetworkError),
    Tls(error),
    Dns(dns.MessageError),
    Framing(FramingError),

    MismatchedResponse,
    PrivacyProfileViolation,
    ConnectionClosed,
    NoServer,
}
```

Rules:

- `Tls(error)` preserves the concrete canonical TLS error;
- a valid DNS response with nonzero DNS RCODE is not a `DotError`;
- `Network` is for TCP/IP failure before/under TLS;
- `Dns` is for malformed DNS wire content;
- `PrivacyProfileViolation` is used when configured policy forbids proceeding;
- cancellation is not duplicated as a DoT error.

---

# 7. `profile.sec`

## 7.1 Responsibility

`profile.sec` owns the RFC 8310 privacy-profile choice.

## 7.2 `PrivacyProfile`

```sec
enum PrivacyProfile {
    Strict,
    Opportunistic,
}
```

These are closed standardized policy choices.

They must not be represented as:

```text
"strict"
"opportunistic"
```

## 7.3 Default

Sec defaults to:

```sec
PrivacyProfile.Strict
```

This means:

- TLS encryption is required;
- server authentication is required;
- failure to establish authenticated DoT is a hard failure;
- no cleartext fallback is performed by this package.

## 7.4 Opportunistic profile

`PrivacyProfile.Opportunistic` allows a caller to deliberately request weaker
authentication policy.

However, the `net/dns/dot` package still represents a **DoT transport**.

Therefore the DoT package itself never sends cleartext DNS on the configured
DoT connection.

If a broader resolver policy wishes to implement the full RFC 8310
opportunistic behavior that may eventually fall back to classic DNS, that
fallback must occur explicitly outside this package through the common DNS
resolver transport policy.

This separation ensures that:

```sec
dot.Client
```

never silently changes from encrypted DNS to cleartext DNS.

## 7.5 No implicit downgrade

Neither profile permits:

- cleartext DNS after a failed TLS handshake on port 853;
- cleartext DNS on an established DoT TCP connection;
- STARTTLS-style upgrade from plain DNS.

RFC 7858 requires the DoT port to carry encrypted DNS only.

---

# Part II — Server identity and bootstrap

# 8. `server.sec`

## 8.1 Responsibility

`server.sec` owns the configured DoT resolver identity.

The configuration must distinguish:

- the resolver's authentication name;
- its DoT port;
- how IP addresses are obtained.

## 8.2 `AuthenticationName`

RFC 8310 uses an Authentication Domain Name.

The Sec representation is:

```sec
type AuthenticationName struct {
    _host: dns.HostName,
}
```

Canonical API:

```sec
impl AuthenticationName {
    static fn Parse(
        text: string
    ) Result[AuthenticationName, ServerError]

    property Host: dns.HostName { get { ... } }

    fn ToString() string
}
```

This is intentionally not an arbitrary `string`.

It is also intentionally not an `ip.Address`.

RFC 8310's Authentication Domain Name is domain-name based.

Direct IP deployments requiring strict authentication can use a different
authentication mechanism such as explicit SPKI pins if the canonical TLS
package supports that configuration.

## 8.3 `Server`

```sec
type Server struct {
    AuthenticationName: AuthenticationName,
    Port: ip.Port,
    Bootstrap: Bootstrap,
}
```

Canonical helpers:

```sec
impl Server {
    static fn Parse(
        authenticationName: string
    ) Result[Server, ServerError]

    static fn New(
        authenticationName: AuthenticationName,
        port: ip.Port,
        bootstrap: Bootstrap
    ) Result[Server, ServerError]

    static let DefaultPort: ip.Port := ip.Port(853)
}
```

`Server.Parse("resolver.example")` creates:

```text
AuthenticationName = resolver.example
Port               = 853
Bootstrap          = System
```

The parser does not accept a URL.

DoT is not HTTP and does not use a URI endpoint.

## 8.4 Non-default ports

A caller may configure a non-default port explicitly.

This is required for:

- private deployments;
- test environments;
- SVCB-discovered alternative ports.

The use of a non-default port does not alter the Authentication Domain Name.

---

# 9. `bootstrap.sec`

## 9.1 Responsibility

`bootstrap.sec` owns how the client obtains IP addresses for a named DoT
resolver before the encrypted resolver is usable.

## 9.2 `Bootstrap`

```sec
type Bootstrap union {
    System,
    Addresses(ip.Address[]),
}
```

## 9.3 `System`

`System` uses an already available host-resolution mechanism that does not
depend recursively on the newly constructed DoT client.

Possible sources:

- platform resolver service;
- pre-existing classic DNS resolver;
- externally configured host mapping;
- platform-managed encrypted DNS resolver.

If the implementation cannot guarantee a non-recursive bootstrap path, it
returns:

```sec
BootstrapError.Required
```

## 9.4 Explicit addresses

```sec
impl Bootstrap {
    static fn FromAddresses(
        addresses: ip.Address[]
    ) Result[Bootstrap, BootstrapError]
}
```

Rules:

- list must not be empty;
- IPv4 and IPv6 may coexist;
- caller order is preserved as input policy;
- connection selection/racing follows the canonical networking connection
  strategy;
- the TLS authentication identity remains `AuthenticationName`;
- a bootstrap IP must never silently replace the certificate reference name.

## 9.5 Authentication identity

Given:

```text
AuthenticationName = resolver.example
Bootstrap address   = 192.0.2.53
```

Sec connects TCP to:

```text
192.0.2.53:853
```

while authenticating TLS as:

```text
resolver.example
```

The bootstrap address is routing information, not identity.

---

# Part III — Authentication

# 10. `authentication.sec`

## 10.1 Responsibility

`authentication.sec` defines the DoT authentication policy requested from the
canonical TLS layer.

RFC 8310 recognizes multiple mechanisms.

Sec must not flatten those mechanisms into:

```text
verify = true
```

## 10.2 `Authentication`

Revision 0.1 defines:

```sec
type Authentication union {
    Pkix,
    SpkiPins(SpkiPinSet),
    None,
}
```

`None` is legal only with:

```sec
PrivacyProfile.Opportunistic
```

Strict Privacy with `Authentication.None` is a configuration error.

## 10.3 PKIX

`Authentication.Pkix` means:

- authenticate the TLS peer using the canonical PKIX trust model;
- verify the certificate/reference identity against
  `Server.AuthenticationName`;
- use the canonical TLS trust store/configuration.

The DoT package does not implement X.509 parsing itself.

## 10.4 SPKI pins

Conceptual type:

```sec
type SpkiPin struct {
    Algorithm: crypto.HashAlgorithm,
    Digest: byte[],
}

type SpkiPinSet struct {
    Pins: SpkiPin[],
}
```

The exact cryptographic type names must be synchronized with the future
canonical crypto/TLS specifications.

Rules:

- empty pin set is invalid;
- pins apply to the configured server;
- pin matching follows RFC 7858 / RFC 8310 requirements;
- the DoT package must not invent a proprietary certificate-pin format.

## 10.5 DANE

RFC 8310 supports DANE authentication, but local DANE validation requires
DNSSEC-authenticated TLSA records.

Revision 0.1 does not expose a half-validating:

```sec
Authentication.Dane
```

until:

```text
net/dns/dnssec
```

and the canonical TLSA/DANE integration are designed.

DANE remains a required future extension.

## 10.6 Raw public keys

RFC 8310 discusses raw public key support.

If the canonical TLS package supports RFC 7250 raw public keys, DoT may expose
that through the shared TLS configuration model.

DoT does not create a second raw-public-key implementation.

## 10.7 Opportunistic authentication

With:

```sec
PrivacyProfile.Opportunistic
```

the caller may use:

```sec
Authentication.None
```

or an authentication method that may fail without aborting the TLS-encrypted
connection, according to the configured policy.

The API must still make the resulting security state observable.

## 10.8 `SecurityState`

```sec
enum SecurityState {
    Authenticated,
    EncryptedUnauthenticated,
}
```

A successful DoT client connection exposes:

```sec
property SecurityState: SecurityState
```

Strict Privacy can only succeed with:

```sec
SecurityState.Authenticated
```

Opportunistic Privacy may succeed with either state.

There is no:

```text
Cleartext
```

state because a `dot.Client` never becomes cleartext DNS.

---

# Part IV — Options

# 11. `options.sec`

## 11.1 Responsibility

`options.sec` owns DoT connection and policy configuration.

## 11.2 `ClientOptions`

```sec
type ClientOptions struct {
    PrivacyProfile: PrivacyProfile,
    Authentication: Authentication,
    MaxMessageBytes: uint,
    EnableSessionResumption: bool,
}
```

Canonical default:

```sec
impl ClientOptions {
    static fn Default() ClientOptions
}
```

Defaults:

```text
PrivacyProfile         = Strict
Authentication         = Pkix
MaxMessageBytes        = 65535
EnableSessionResumption = true
```

`MaxMessageBytes` must not exceed the maximum DNS-over-TCP frame size.

## 11.3 TLS versions and algorithms

There is intentionally no DoT option such as:

```text
TlsVersion = "1.2"
Cipher = "..."
```

General TLS algorithm/version policy belongs to the canonical TLS package.

A caller that needs custom TLS configuration should use a later constructor
accepting a canonical TLS configuration object.

DoT-specific code must not provide a shortcut for weakening global TLS
security policy.

## 11.4 0-RTT

TLS 1.3 0-RTT use is **disabled for DoT DNS queries** in revision 0.1.

Reason:

- DNS queries can be replayed;
- BCP 195 advises against 0-RTT unless the application protocol defines safe
  usage;
- revision 0.1 does not define DoT replay-safe request classes.

Session resumption without early application data remains supported.

## 11.5 Compression

TLS-level compression must not be enabled.

DNS name compression inside DNS messages remains ordinary DNS wire behavior
and is unrelated to TLS compression.

---

# Part V — DNS framing

# 12. `framing.sec`

## 12.1 Responsibility

`framing.sec` owns DNS-over-TCP length framing used inside the TLS stream.

Primary references:

- RFC 1035
- RFC 7766
- RFC 7858

## 12.2 Frame format

Each DNS message is carried as:

```text
+----------------------+-----------------------------+
| uint16 messageLength | DNS wire message bytes      |
+----------------------+-----------------------------+
```

The two-byte length is network byte order.

The maximum payload is:

```text
65535 bytes
```

The framing bytes are not part of `dns.Message`.

## 12.3 Encoding

```sec
fn EncodeFrame(
    message: ref dns.Message,
    output: ref mut byte[]
) Result[uint, DotError]
```

The function:

1. reserves two bytes;
2. encodes one DNS wire message;
3. validates length <= 65535;
4. writes the two-byte length prefix;
5. returns total bytes written including framing.

## 12.4 Reading

Internal framing state must support:

- TLS reads returning less than two prefix bytes;
- prefix split across reads;
- DNS payload split across arbitrary reads;
- multiple frames arriving in one TLS read;
- zero-byte TCP/TLS EOF before or during a frame;
- bounded allocation.

A frame length does not justify blindly allocating that many bytes when a
stricter configured message limit exists.

## 12.5 Zero-length frame

A zero-length DNS frame is not a valid ordinary DNS message.

It is rejected as:

```sec
FramingError.InvalidLength
```

---

# Part VI — TLS connection

# 13. `connection.sec`

## 13.1 Responsibility

`connection.sec` owns one established TLS-protected DoT connection.

## 13.2 `Connection`

```sec
@noCopy
type Connection struct {
    // Private TLS stream, framing state, pending-query state,
    // server identity, and negotiated security state.
}
```

Canonical surface:

```sec
impl Connection {
    property Server: Server { get { ... } }
    property SecurityState: SecurityState { get { ... } }

    fn Exchange(
        request: ref dns.Message
    ) Result[dns.Message, DotError]

    fn Close() Result[void, DotError]
}
```

The exact constructor is internal to `Client`.

## 13.3 Establishment

Connection establishment conceptually performs:

```text
bootstrap
-> TCP connect
-> TLS handshake/resumption
-> authentication/profile evaluation
-> usable DoT connection
```

No cleartext DNS bytes may be sent before TLS is established.

## 13.4 ALPN

When required by the selected discovery/configuration mode, the TLS client
offers the registered:

```text
dot
```

ALPN identifier.

The implementation must reuse the canonical TLS ALPN type.

For statically configured RFC 7858 DoT where ALPN is not required by the
chosen configuration source, the implementation follows the applicable
standards/TLS package behavior.

A discovered RFC 9461 endpoint advertising `alpn=dot` must negotiate a
compatible protocol.

## 13.5 Authentication evaluation

After TLS handshake:

- Strict + successful configured authentication -> usable;
- Strict + failed/missing authentication -> hard failure;
- Opportunistic + successful authentication -> `Authenticated`;
- Opportunistic + TLS encryption but no successful authentication may ->
  `EncryptedUnauthenticated`;
- no TLS connection -> DoT connection failure.

## 13.6 Reuse

A successful connection should be reused for multiple DNS queries.

Opening a new TCP+TLS connection per DNS query is not the intended normal
operation.

## 13.7 Session resumption

Where the canonical TLS stack supports it:

- resumption should be used for reconnects;
- session state remains scoped to the authenticated DoT server identity;
- resumption must not weaken identity checking;
- 0-RTT DNS application data remains disabled in revision 0.1.

## 13.8 Close

`Close()` is explicit and idempotent.

The implementation should use the canonical TLS close-notify semantics where
available.

Deterministic destruction remains the ownership backstop.

---

# Part VII — Client

# 14. `client.sec`

## 14.1 Responsibility

`client.sec` owns a reusable DoT DNS client.

## 14.2 `Client`

```sec
@noCopy
type Client struct {
    // Private Server, ClientOptions, reusable Connection,
    // bootstrap/cache state, and synchronization.
}
```

Canonical API:

```sec
impl Client {
    static fn New(
        server: Server,
        options: ClientOptions
    ) Result[Client, DotError]

    property Server: Server { get { ... } }

    property SecurityState: Option[SecurityState] { get { ... } }

    fn Exchange(
        request: ref dns.Message
    ) Result[dns.Message, DotError]

    fn Query(
        question: dns.Question
    ) Result[dns.Message, DotError]

    fn Close() Result[void, DotError]
}
```

`SecurityState` is `None` before a successful DoT connection is established.

## 14.3 Query generation

`Query`:

- builds an ordinary one-question DNS query;
- generates a DNS transaction ID according to base DNS rules;
- sends it through the reusable DoT connection;
- validates the response ID and question;
- returns valid DNS responses regardless of DNS RCODE.

DoT does not set message ID to zero merely because the stream is encrypted.

Transaction IDs remain part of DNS-over-TCP message matching.

## 14.4 Exchange

`Exchange`:

- accepts one explicit DNS message;
- frames it;
- sends it over TLS;
- receives one matched DNS response;
- validates wire format;
- returns the owning `dns.Message`.

Protocols such as AXFR that intentionally produce multiple DNS messages are
not modeled as one ordinary `Exchange`.

## 14.5 Nonzero RCODE

Responses such as:

```text
NXDOMAIN
SERVFAIL
REFUSED
```

remain successful DNS messages at the direct DoT client layer.

Higher resolver logic may translate terminal DNS result semantics into
`dns.ResolveError`.

## 14.6 Connection recovery

When the reusable TLS connection closes unexpectedly:

- an operation that has not committed may reconnect according to bounded
  retry policy;
- an operation whose query may have committed must not be blindly duplicated
  unless DNS semantics and the specific operation make retry acceptable;
- ordinary DNS queries are generally retryable according to resolver/client
  policy;
- stateful/mutating DNS operations require separate explicit semantics.

Revision 0.1 `Query` may retry once on a clean pre-response connection failure
if the request is an ordinary query.

`Exchange` does not automatically retry arbitrary caller-constructed messages
unless the request class is known safe.

## 14.7 Concurrent queries

The protocol and RFC 7766 support multiple outstanding DNS queries on one
connection.

Responses are matched using DNS transaction identity/question semantics.

A complete implementation should support concurrent outstanding ordinary
queries over one TLS connection.

An initial implementation may serialize exchanges, but it must not be marked
performance-complete until safe pipelining/multiplexing is implemented.

## 14.8 Backpressure

The client must bound:

- outstanding queued requests;
- message buffers;
- response waiters.

An attacker or application must not cause unlimited queue growth by issuing
unbounded concurrent DNS queries.

The exact queue limit is an implementation/configuration detail until a
cross-stdlib resource-limit policy is defined.

## 14.9 Close

`Client.Close()`:

- closes the reusable DoT connection;
- prevents new exchanges;
- is idempotent;
- returns `ConnectionClosed` for later operations;
- releases owned TLS/TCP resources deterministically.

---

# Part VIII — Server-side DoT

# 15. `listener.sec`

## 15.1 Responsibility

`listener.sec` provides the transport adaptation required to accept DoT
connections.

It does not implement a recursive or authoritative DNS server.

## 15.2 `Listener`

```sec
@noCopy
type Listener struct {
    // Private TCP listener + TLS server configuration.
}
```

Conceptual API:

```sec
impl Listener {
    static fn Bind(
        endpoint: ip.Endpoint,
        tlsConfig: <- tls.ServerConfig
    ) Result[Listener, DotError]

    fn Accept() Result[ServerConnection, DotError]

    fn Close() Result[void, DotError]
}
```

The exact canonical TLS type name is provisional until TLS stdlib ownership is
locked.

The semantic rule is locked: server TLS configuration comes from the shared
TLS package and is not duplicated by DoT.

## 15.3 `ServerConnection`

```sec
@noCopy
type ServerConnection struct {
    // Private accepted authenticated/encrypted TLS stream and framing state.
}
```

Canonical semantic operations:

```sec
impl ServerConnection {
    fn ReadQuery() Result[dns.Message, DotError]

    fn WriteResponse(
        response: ref dns.Message
    ) Result[void, DotError]

    fn Close() Result[void, DotError]
}
```

## 15.4 Server handshake

A DoT listener:

1. accepts TCP;
2. immediately performs TLS;
3. never accepts a cleartext DNS message on the DoT port;
4. only exposes `ServerConnection` after TLS establishment succeeds.

## 15.5 Client authentication

Ordinary DoT does not require TLS client-certificate authentication.

If an application deploys mutual TLS, that is an explicit TLS/server policy.

The DoT package must not require client certificates by default.

## 15.6 Multiple requests

One accepted `ServerConnection` must support multiple DNS messages over its
lifetime.

The server side must not close after every response unless deployment policy
requires it.

## 15.7 Response ordering

When the server processes requests concurrently, response order need not match
query arrival order if transaction IDs allow correct matching.

The implementation must not alter DNS transaction IDs merely to force
in-order behavior.

---

# Part IX — Resolver integration

# 16. `transport.sec`

## 16.1 Common resolver transport

As established by `std-net-dns-doh.md`, high-level resolver logic must not be
duplicated for each DNS transport.

The parent DNS package requires a common transport contract with semantic
shape:

```sec
interface QueryTransport {
    fn Exchange(
        request: ref Message
    ) Result[Message, error]
}
```

The canonical owner is:

```text
net/dns
```

## 16.2 DoT conformance

`dot.Client` satisfies the common transport contract through:

```sec
fn Exchange(
    request: ref dns.Message
) Result[dns.Message, DotError]
```

The exact Sec interface conformance syntax follows the canonical interface
rulebook.

## 16.3 No `dot.Resolver`

Revision 0.1 deliberately does not define:

```sec
dot.Resolver
```

The high-level resolver is shared.

This prevents duplication of:

- cache;
- CNAME following;
- negative caching;
- A/AAAA aggregation;
- reverse lookup;
- DNSSEC result handling;
- host-resolution policy.

## 16.4 Resolver fallback policy

A common resolver may be configured with a transport policy such as:

```text
Strict DoT only
Prefer DoT, explicit classic fallback
Classic only
```

That policy belongs above `dot.Client`.

`dot.Client` itself never becomes cleartext DNS.

This is especially important for `PrivacyProfile.Opportunistic`.

---

# Part X — Discovery and DDR

# 17. RFC 9461 service binding

RFC 9461 allows a named DNS resolver to publish SVCB records advertising DoT.

Relevant SVCB values include:

```text
alpn=dot
port=<optional>
ipv4hint=<optional>
ipv6hint=<optional>
```

Default DoT port is 853 when `port` is absent.

The SVCB authentication name remains distinct from address hints.

## 17.1 Discovery result

A future common DNS discovery package should produce a transport-neutral
result from which DoT can construct:

```sec
Server
```

including:

- authentication name;
- port;
- candidate addresses;
- selected transport.

The DoT package should not independently query and interpret DDR records.

## 17.2 Port redirection safety

RFC 9461 warns that discovered alternative ports can create cross-protocol
confusion risks.

Therefore a common discovery implementation must not blindly connect DoT TLS
to arbitrary local/elevated service ports received from untrusted discovery
data.

Port safety policy belongs in the common discovery layer.

---

# 18. RFC 9462 DDR

DDR can discover:

```text
DoT
DoH
DoQ
```

It therefore belongs in a shared area, likely:

```text
net/dns/discovery
```

This exact package path remains an observation item.

DoT accepts a validated discovery result but does not own the complete DDR
algorithm.

---

# Part XI — DANE and DNSSEC

# 19. DANE authentication boundary

DoT DANE authentication creates an important dependency question:

```text
DoT needs DNSSEC-validated TLSA
DNSSEC resolver may itself use DoT
```

The implementation must avoid a circular trust/bootstrap assumption.

A valid DANE configuration requires a defined trust path for the TLSA result.

Possible sources include:

- an already validated DNSSEC resolver;
- prevalidated configuration;
- a trust anchor and local DNSSEC validator independent of the DoT session.

The package must not claim DANE validation based on an unauthenticated TLSA
lookup through the very server being authenticated.

## 19.1 Future API

A future authentication variant may be:

```sec
Authentication.Dane(...)
```

only after the DNSSEC and TLSA validation contract is fully specified.

Until then, revision 0.1 supports PKIX and explicit SPKI pinning.

---

# Part XII — TLS policy

# 20. TLS version floor

DoT-specific historical standards permit TLS 1.2 or later.

Sec must nevertheless follow the current canonical TLS security policy.

At minimum:

- SSL is forbidden;
- TLS 1.0 is forbidden;
- TLS 1.1 is forbidden;
- TLS 1.3 is preferred;
- TLS 1.2, when the canonical TLS package still supports it for this existing
  protocol, must follow current BCP 195 and RFC 10015 restrictions.

The DoT package must not re-enable cipher suites or key exchanges disabled by
the canonical TLS package.

## 20.1 New deployment guidance

New Sec deployments should prefer TLS 1.3.

This aligns with the direction of current BCP 195 and avoids unnecessarily
expanding TLS 1.2 compatibility surface.

## 20.2 Renegotiation

DoT does not use application-driven TLS renegotiation as a protocol feature.

The package must not expose a DoT-specific renegotiation API.

## 20.3 Session resumption

Session resumption is enabled by default where the TLS package safely supports
it.

Resumption state is scoped to the appropriate server identity and TLS
configuration.

## 20.4 0-RTT

No DNS query application data is sent in TLS 1.3 0-RTT in revision 0.1.

---

# Part XIII — Privacy and security

# 21. Strict Privacy is the Sec default

The package defaults to:

```sec
PrivacyProfile.Strict
```

because that profile:

- requires encryption;
- requires authentication;
- avoids silent downgrade;
- gives the strongest standardized DoT privacy semantics.

Opportunistic use remains explicit for environments where strict
authentication information genuinely cannot be provisioned.

---

# 22. Opportunistic Privacy limitations

An encrypted but unauthenticated connection does not prevent an active
attacker from impersonating the resolver.

Therefore:

```sec
SecurityState.EncryptedUnauthenticated
```

must remain observable.

Applications must not label this state "secure", "verified", or
"authenticated".

---

# 23. No cleartext on DoT port

The package must never:

1. connect TCP port 853;
2. fail TLS;
3. send cleartext DNS on that same connection.

RFC 7858 explicitly forbids this behavior.

---

# 24. Authentication name versus address

IP routing identity and TLS authentication identity are separate.

A discovered or bootstrap address may change without changing the resolver's
Authentication Domain Name.

TLS validation must not simply authenticate whichever IP address happened to
be selected.

---

# 25. Traffic analysis

TLS encrypts DNS contents but does not hide all metadata.

Observers may still infer information from:

- packet sizes;
- timing;
- connection frequency;
- server IP address.

The package must not claim DoT to provide anonymity.

Future padding policy may mitigate some traffic-analysis signals.

---

# 26. TLS record and DNS message padding

Revision 0.1 does not invent a package-specific padding scheme.

EDNS Padding exists in DNS and TLS has its own record-layer behavior.

Any privacy padding policy must be standards-backed and coordinated with the
base DNS/EDNS and TLS layers.

---

# 27. Resolver capability probing

RFC 7858/8310 discuss remembering servers that fail DoT capability tests.

Revision 0.1 allows an internal bounded capability-failure cache so clients do
not continuously retry a known unavailable DoT endpoint.

However:

- Strict Privacy must still fail rather than silently downgrade;
- the cache lifetime must be bounded;
- explicit configuration changes invalidate relevant cached failures;
- a temporary failure must not permanently disable DoT.

The exact default negative capability-cache duration is implementation policy
until dogfooding provides evidence.

---

# Part XIV — Cancellation, ownership, and concurrency

# 28. Ownership

The following are `@noCopy` owning resources:

```sec
Client
Connection
Listener
ServerConnection
```

They own underlying TCP/TLS resources.

Configuration/value types such as:

```sec
Server
Bootstrap
PrivacyProfile
ClientOptions
AuthenticationName
```

are ordinary semantic values.

---

# 29. Cancellation

Potential cancellation points include:

- bootstrap resolution;
- TCP connect;
- TLS handshake;
- TLS read;
- TLS write;
- waiting for a DNS response;
- reconnect.

They participate in Sec's canonical cancellation/context model.

The package does not define:

```text
DotCancellationToken
```

Cancellation must not:

- publish a partially decoded response;
- turn into NXDOMAIN/SERVFAIL;
- silently downgrade transport;
- lose ownership of a connection resource.

---

# 30. Exchange commit semantics

A DNS query can be in one of several states:

```text
not written
partially written
fully written
response partially received
response complete
```

Cancellation/retry behavior must respect these states.

The implementation must not automatically retry arbitrary `Exchange`
messages when it cannot determine whether the original request committed.

Ordinary idempotent `Query` operations may use a documented bounded retry
policy.

---

# 31. Concurrency

A single client should eventually support multiple outstanding DNS queries over
one reusable DoT connection.

Concurrency requires:

- unique outstanding transaction IDs;
- response matching;
- bounded pending-query table;
- cancellation-safe removal;
- correct connection-failure fan-out.

An initial serialized implementation is acceptable as an implementation
milestone but not as the final performance target.

---

# Part XV — Platform capability

# 32. Pure and active facilities

Pure facilities:

- framing helpers;
- privacy-profile values;
- server configuration validation.

Active DoT requires:

- TCP support;
- TLS support;
- a bootstrap path when the server is named;
- certificate/trust functionality for Strict PKIX authentication.

A target without TLS capability must reject active DoT use rather than provide
a cleartext substitute.

---

# Part XVI — Explicit exclusions

# 33. No STARTTLS mode

DoT is direct TLS.

There is no:

```text
plain DNS -> STARTTLS -> DoT
```

upgrade sequence.

---

# 34. No DoH behavior

DoT has no:

- HTTP method;
- URL;
- HTTP status;
- HTTP cache;
- media type;
- HTTP redirect.

Those belong to DoH.

---

# 35. No DoQ behavior

DoT does not expose:

- QUIC streams;
- QUIC connection IDs;
- QUIC transport parameters.

Those belong to DoQ.

---

# 36. No silent classic DNS fallback

A DoT client never sends classic DNS as fallback.

Fallback belongs to an explicit higher-level resolver transport policy.

---

# 37. No duplicate TLS configuration universe

DoT must not create private enums for:

- TLS versions;
- cipher suites;
- signature algorithms;
- certificate formats;
- trust stores;
- ALPN values in general.

Those are owned by the canonical TLS package.

---

# Part XVII — Repository implementation

# 38. Required source structure

Create:

```text
sec/stdlib/net/dns/dot/
├── std-net-dns-dot.md
├── error.sec
├── profile.sec
├── server.sec
├── bootstrap.sec
├── authentication.sec
├── options.sec
├── framing.sec
├── connection.sec
├── client.sec
├── listener.sec
└── transport.sec
```

If newer repository revisions already contain files in this directory:

- inspect them;
- preserve useful work;
- expand/correct them;
- do not keep obsolete API solely for source compatibility.

---

# 39. Parent DNS correction

As already identified by `std-net-dns-doh.md`, the parent DNS design must
separate:

```text
resolver engine/cache/policy
```

from:

```text
query transport
```

The common transport abstraction must support at least:

```text
classic DNS
DoT
DoH
DoQ
```

without base DNS importing child packages.

DoT implementation is not resolver-complete until this integration exists.

---

# 40. TLS dependency observation

The canonical TLS stdlib location remains open at the root networking design
level.

Therefore provisional references such as:

```sec
tls.ServerConfig
```

in this book express semantics, not a final package path.

When TLS ownership is locked, this book must be mechanically updated to the
canonical TLS type/path without changing DoT semantics.

---

# Part XVIII — Tests

# 41. Profile tests

At minimum:

- default profile is Strict;
- Strict + PKIX valid;
- Strict + no authentication rejected;
- Strict authentication failure is hard failure;
- Opportunistic + PKIX success -> Authenticated;
- Opportunistic + authentication unavailable -> EncryptedUnauthenticated
  where policy permits;
- no profile results in cleartext within `dot.Client`.

---

# 42. Server/bootstrap tests

At minimum:

- default port is 853;
- custom port;
- valid Authentication Domain Name;
- invalid name rejected;
- system bootstrap;
- explicit IPv4 bootstrap;
- explicit IPv6 bootstrap;
- empty address list rejected;
- bootstrap IP does not replace TLS authentication name;
- self-recursive bootstrap rejected/detected where observable.

---

# 43. TLS tests

At minimum:

- TLS begins immediately after TCP connect;
- cleartext DNS never sent before TLS;
- PKIX success;
- name mismatch rejected under Strict;
- untrusted certificate rejected under Strict;
- SPKI pin success;
- SPKI pin failure;
- session resumption;
- TLS 1.3;
- TLS 1.2 only where canonical TLS policy permits;
- prohibited old TLS versions rejected;
- 0-RTT DNS application data not sent;
- close-notify behavior where supported.

---

# 44. Framing tests

At minimum:

- one DNS frame;
- two-byte prefix split across TLS reads;
- payload split across reads;
- multiple frames in one read;
- zero-length frame rejected;
- 65535-byte maximum payload;
- 65536-byte payload rejected;
- premature EOF in prefix;
- premature EOF in payload;
- configured lower message limit enforced.

---

# 45. Client tests

At minimum:

- A query over DoT;
- AAAA query over DoT;
- DNS NXDOMAIN returned as valid DNS message;
- DNS SERVFAIL returned as valid DNS message;
- malformed DNS body -> DNS/DoT error;
- ID mismatch;
- question mismatch;
- connection reuse;
- reconnect after clean connection loss;
- arbitrary `Exchange` not unsafely retried;
- close idempotent;
- operation after close;
- cancellation during connect;
- cancellation during handshake;
- cancellation during response read.

---

# 46. Pipelining tests

When concurrent exchange support is implemented:

- multiple outstanding requests;
- responses out of order;
- transaction-ID matching;
- duplicate transaction-ID allocation prevented;
- one request cancellation does not corrupt another;
- connection failure completes all affected waiters exactly once;
- pending-query count bounded.

---

# 47. Listener/server tests

At minimum:

- bind port 853 or test port;
- immediate TLS handshake;
- cleartext DNS input rejected;
- one query/response;
- multiple sequential queries;
- multiple pipelined queries where supported;
- server connection reuse;
- listener close;
- accepted resource ownership/destruction.

---

# 48. Discovery integration tests

When RFC 9461/9462 support exists:

- `alpn=dot` recognized;
- default DoT port 853;
- explicit SVCB port;
- authentication name preserved independently of target address;
- unsupported ALPN ignored/rejected according to discovery policy;
- insecure discovery cannot change authenticated server identity;
- DoT discovered alongside DoH/DoQ without duplicated discovery logic.

---

# Part XIX — AI/repository implementation instructions

# 49. Implementation requirements

An AI or repository automation implementing this book must:

1. treat this book as the normative DoT specification;
2. create the declared source files when missing;
3. inspect and update existing files rather than blindly overwrite them;
4. add RFC and IANA URLs to source-file headers;
5. reuse base `net/dns` message/record/codec functionality;
6. reuse `net/ip` TCP, endpoints, addresses, ports, and network errors;
7. reuse the canonical TLS implementation;
8. never implement TLS locally in this package;
9. expose the typed default port as `ip.Port(853)`;
10. implement RFC 7858 DNS-over-TLS framing correctly;
11. implement RFC 8310 Strict and Opportunistic profiles;
12. default Sec to Strict Privacy;
13. never send cleartext DNS on a DoT connection or DoT port;
14. never silently fall back to classic DNS;
15. preserve bootstrap IP versus Authentication Domain Name separation;
16. support PKIX authentication;
17. support SPKI pinning when canonical crypto/TLS types are available;
18. defer DANE until DNSSEC/TLSA validation is properly designed;
19. disable TLS 0-RTT DNS application data in revision 0.1;
20. support connection reuse;
21. support or plan bounded concurrent outstanding queries;
22. preserve nonzero DNS RCODE as valid DNS response;
23. integrate cancellation using the common Sec model;
24. update the parent DNS transport abstraction before claiming resolver
    integration complete;
25. do not create a duplicate `dot.Resolver`;
26. add required tests;
27. update `implementation-status-std-net-dns-dot.yaml`;
28. reconcile status into canonical `implementation-status.yaml` without
    replacing stronger/newer evidence;
29. report TLS/package blockers rather than inventing incompatible placeholder
    security APIs.

---

# 50. Recommended implementation order

1. create package/book/files;
2. implement profile/error values;
3. implement server identity and bootstrap;
4. implement options;
5. implement DNS framing;
6. integrate canonical TLS client configuration;
7. implement PKIX Strict connection;
8. implement `Connection`;
9. implement reusable `Client`;
10. implement ordinary `Query`;
11. implement explicit `Exchange`;
12. implement reconnection policy;
13. implement session resumption;
14. implement Opportunistic profile state reporting;
15. implement SPKI pinning integration;
16. implement server listener/connection adapter;
17. implement bounded concurrent outstanding requests;
18. integrate common DNS resolver transport contract;
19. integrate shared RFC 9461/9462 discovery when available;
20. add DANE only after DNSSEC validation contract exists.

---

# Part XX — Open observations

# 51. Items intentionally not fully locked in revision 0.1

The following remain open:

- final canonical TLS package path;
- final TLS client/server configuration type names;
- final canonical SPKI pin/hash types;
- DANE authentication API;
- raw-public-key API;
- common encrypted-DNS discovery package path;
- exact parent `dns.QueryTransport` interface syntax;
- final resolver constructor accepting alternate transports;
- cross-transport fallback policy API;
- connection failure capability-cache duration;
- configurable pending-query queue limit;
- whether connection pools should support more than one DoT connection per
  server;
- TCP Fast Open policy;
- DNS EDNS padding policy for DoT privacy;
- TLS record padding policy;
- server mutual-TLS hooks;
- authoritative-server DoT use beyond stub-to-recursive baseline;
- DNS zone transfer over TLS relationship;
- dynamic DNS Update retry semantics;
- exact TLS 1.2 retention policy once the canonical TLS package is locked.

These observations are not permission to invent incompatible public APIs.

---

# 52. Decisions established by revision 0.1

Revision 0.1 establishes:

- canonical package path `net/dns/dot`;
- RFC 7858 is the primary DoT protocol standard;
- RFC 8310 updates the DoT privacy/authentication profile;
- current BCP 195 TLS guidance supersedes obsolete RFC 7525 guidance;
- port 853 is exposed as typed `ip.Port`, not a magic integer;
- `dot` is the relevant registered ALPN identifier;
- DoT uses DNS-over-TCP two-byte framing inside TLS;
- no DNS-over-TCP framing logic is duplicated as DNS wire semantics;
- DoT reuses `net/dns`, `net/ip`, and the canonical TLS package;
- `AuthenticationName` is a typed DNS host name;
- bootstrap IP addresses are routing data, not TLS identity;
- `PrivacyProfile` has `Strict` and `Opportunistic`;
- Sec defaults to Strict Privacy;
- Strict requires encryption and successful authentication;
- Opportunistic may expose `EncryptedUnauthenticated`;
- no DoT state represents cleartext DNS;
- the DoT package never silently falls back to classic DNS;
- TLS starts immediately after TCP connection;
- cleartext DNS is never sent on the DoT connection;
- PKIX authentication is supported;
- SPKI pinning is part of the design;
- DANE is deferred until DNSSEC validation integration is safe;
- TLS 1.3 is preferred;
- old TLS versions below 1.2 are forbidden;
- TLS 1.2, if retained, follows the current canonical TLS policy and BCP 195;
- TLS 0-RTT DNS query data is disabled in revision 0.1;
- session resumption is enabled by default where safely available;
- one connection is reusable for multiple DNS exchanges;
- concurrent outstanding queries are a required performance target;
- direct DoT clients return valid nonzero-RCODE DNS messages;
- `dot.Client` is `@noCopy`;
- server-side DoT uses immediate TLS and supports connection reuse;
- DoT integrates with the shared parent DNS resolver engine;
- no duplicate `dot.Resolver` is introduced;
- RFC 9461/9462 discovery belongs in shared encrypted-DNS discovery logic;
- the final TLS package path remains an explicit observation until the root
  networking/TLS ownership decision is locked.

---

**Document revision:** 0.1  
**Last updated:** 2026-09-07
