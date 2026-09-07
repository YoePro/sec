# Sec Standard Library — DNS over HTTPS

- **Status:** Draft
- **Created:** 2026-09-07
- **Last updated:** 2026-09-07
- **Document revision:** 0.1
- **Sec language version:** 0.1
- **Canonical path:** `stdlib/net/dns/doh/std-net-dns-doh.md`
- **Repository path:** `sec/stdlib/net/dns/doh/std-net-dns-doh.md`
- **Parent specification:** `stdlib/net/dns/std-net-dns.md`
- **Implementation-status fragment:** `implementation-status-std-net-dns-doh.yaml`
- **Latest verified repository main:** `0f5027d`
- **Standards state rechecked:** 2026-09-07

---

# 1. Purpose

`net/dns/doh` implements DNS over HTTPS (DoH).

The package maps one DNS query/response exchange onto one HTTPS request/response
exchange according to RFC 8484.

It owns:

- validated DoH endpoint configuration;
- RFC 6570 DoH URI Template usage;
- GET and POST request generation;
- `application/dns-message`;
- GET base64url encoding;
- DoH-specific HTTP/DNS error separation;
- HTTPS bootstrap-address handling;
- HTTP cache-age adjustment of DNS TTLs;
- DoH client behavior;
- server-side HTTP-to-DNS request decoding;
- server-side DNS-to-HTTP response encoding;
- integration requirements for the common `net/dns` resolver engine.

The package is imported as:

```sec
import "net/dns/doh"
```

and used through:

```sec
doh.Client
doh.Endpoint
doh.ClientOptions
```

Example:

```sec
let endpoint := try doh.Endpoint.Parse(
    "https://resolver.example/dns-query{?dns}"
)

let client := try doh.Client.New(
    endpoint,
    doh.ClientOptions.Default()
)

let response := try client.Query(
    new dns.Question(
        try dns.Name.Parse("example.com"),
        dns.RecordType.A,
        dns.DnsClass.IN
    )
)
```

The package does not implement DNS wire semantics itself. It reuses `net/dns`.

The package does not implement HTTP or TLS itself. It reuses `net/http` and the
canonical HTTPS/TLS behavior selected by the HTTP package.

---

# 2. Package boundary

## 2.1 Dependencies

`net/dns/doh` depends on:

```text
net/dns
net/http
net/url
net/ip
```

and indirectly on the canonical TLS implementation used by HTTPS.

The dependency direction is:

```text
net/dns        <- net/dns/doh
net/http       <- net/dns/doh
net/url        <- net/dns/doh
net/ip         <- net/dns/doh

net/dns  X-> net/dns/doh
net/http X-> net/dns/doh
```

The base `net/dns` package must not import `net/dns/doh`.

This prevents dependency cycles such as:

```text
dns -> doh -> http -> dns
```

## 2.2 Related DNS transports

These remain separate sibling packages:

```text
net/dns/dot
net/dns/doq
```

DoH must not become a generic "encrypted DNS" package.

## 2.3 Not owned here

This package does not own:

- DNS message parsing or record types;
- DNS cache semantics in general;
- HTTP request/response primitives;
- HTTP connection pooling;
- HTTP/1.1, HTTP/2, or HTTP/3 framing;
- TLS handshakes or certificate verification;
- system trust stores;
- URI parsing in general;
- generic RFC 6570 URI Templates;
- full DNSSEC validation;
- Discovery of Designated Resolvers (DDR);
- Oblivious DoH;
- mDNS or DNS-SD.

Those responsibilities remain in their canonical packages.

---

# 3. Standards and registries

## 3.1 Primary DoH standard

The normative DoH protocol baseline is:

- **RFC 8484 — DNS Queries over HTTPS (DoH)**
- https://www.rfc-editor.org/rfc/rfc8484
- https://www.rfc-editor.org/info/rfc8484

RFC 8484 defines:

- selection of a configured DoH URI Template;
- GET and POST mappings;
- `application/dns-message`;
- DNS/HTTP error separation;
- caching requirements;
- bootstrap considerations;
- privacy/security considerations.

## 3.2 DNS wire format

DoH with `application/dns-message` carries one ordinary DNS wire message.

Relevant DNS standards are inherited from `net/dns`, particularly:

- RFC 1035 — https://www.rfc-editor.org/rfc/rfc1035
- RFC 6891 — https://www.rfc-editor.org/rfc/rfc6891

The DoH body does **not** use the two-octet DNS-over-TCP length prefix.

The maximum `application/dns-message` payload is 65535 bytes.

## 3.3 HTTP standards

RFC 8484 references the HTTP documents current at its publication date.
Sec uses the current HTTP semantics specifications where they supersede those
documents.

Relevant current standards include:

- RFC 9110 — HTTP Semantics  
  https://www.rfc-editor.org/rfc/rfc9110
- RFC 9111 — HTTP Caching  
  https://www.rfc-editor.org/rfc/rfc9111
- RFC 9112 — HTTP/1.1  
  https://www.rfc-editor.org/rfc/rfc9112
- RFC 9113 — HTTP/2  
  https://www.rfc-editor.org/rfc/rfc9113
- RFC 9114 — HTTP/3  
  https://www.rfc-editor.org/rfc/rfc9114

RFC 8484 recommends HTTP/2 or newer behavior for competitive DoH performance,
but its semantic mapping is not restricted to one HTTP protocol version.

The DoH package therefore delegates protocol-version selection to `net/http`.

## 3.4 HTTPS and TLS

Relevant TLS baseline:

- RFC 8446 — TLS 1.3  
  https://www.rfc-editor.org/rfc/rfc8446

HTTPS identity verification follows the canonical HTTP/TLS package rules.

DoH bootstrap IP addresses must never replace the configured HTTPS
authentication name.

## 3.5 URI Templates

DoH endpoints use:

- RFC 6570 — URI Template  
  https://www.rfc-editor.org/rfc/rfc6570

Generic RFC 6570 parsing/expansion belongs to `net/url`.

The DoH package adds stricter validation for templates used as DoH endpoints.

## 3.6 Base64url

GET requests use:

- RFC 4648 — Base-N Encodings  
  https://www.rfc-editor.org/rfc/rfc4648

The DNS message is encoded using base64url without `=` padding.

## 3.7 Media type

DoH requires support for:

```text
application/dns-message
```

IANA media-type registry:

https://www.iana.org/assignments/media-types

The registration is defined by RFC 8484.

## 3.8 Encrypted DNS discovery

DoH endpoint discovery is related to, but not owned by, this package:

- RFC 9461 — Service Binding Mapping for DNS Servers  
  https://www.rfc-editor.org/rfc/rfc9461
- RFC 9462 — Discovery of Designated Resolvers  
  https://www.rfc-editor.org/rfc/rfc9462
- RFC 9460 — Service Binding and Parameter Specification via the DNS  
  https://www.rfc-editor.org/rfc/rfc9460

RFC 9461 defines the `dohpath` SVCB parameter.

DDR spans DoH, DoT, and DoQ and should therefore eventually live in a common
encrypted-DNS discovery area rather than being implemented only inside
`net/dns/doh`.

## 3.9 Oblivious DoH

Oblivious DoH is not part of this package revision.

Relevant document:

- RFC 9230 — Oblivious DNS over HTTPS  
  https://www.rfc-editor.org/rfc/rfc9230

RFC 9230 is Experimental, not the base DoH Standards Track protocol.

A future implementation should use a separate package rather than silently
changing ordinary DoH privacy semantics.

---

# 4. Source-file standards headers

Each `.sec` file must name the directly relevant standards.

Example:

```sec
// Sec Standard Library: net/dns/doh
// File: client.sec
//
// Standards:
//   RFC 8484 - DNS Queries over HTTPS (DoH)
//   https://www.rfc-editor.org/rfc/rfc8484
//
//   RFC 9110 - HTTP Semantics
//   https://www.rfc-editor.org/rfc/rfc9110
//
//   RFC 9111 - HTTP Caching
//   https://www.rfc-editor.org/rfc/rfc9111
//
// Related:
//   RFC 6570 - URI Template
//   https://www.rfc-editor.org/rfc/rfc6570

module doh
```

DoH source files must not copy old HTTP rules from RFC 723x where RFC 9110/9111
now own the applicable HTTP semantics.

---

# 5. Package and file manifest

The initial package layout is:

```text
stdlib/net/dns/doh/
├── std-net-dns-doh.md
├── error.sec
├── endpoint.sec
├── bootstrap.sec
├── options.sec
├── media.sec
├── client.sec
├── server.sec
└── transport.sec
```

All source files use:

```sec
module doh
```

No nested package is required in revision 0.1.

Expected tests:

```text
endpoint_test.sec
bootstrap_test.sec
media_test.sec
client_test.sec
server_test.sec
transport_test.sec
```

---

# Part I — Error model

# 6. `error.sec`

## 6.1 Responsibility

`error.sec` owns DoH-specific endpoint, bootstrap, HTTP mapping, content, and
protocol errors.

It does not duplicate DNS message errors or HTTP transport errors.

## 6.2 `EndpointError`

```sec
enum EndpointError error {
    InvalidTemplate,
    NotAbsolute,
    RequiresHttps,
    MissingHost,
    UserInfoNotAllowed,
    FragmentNotAllowed,
    InvalidDnsVariable,
    UnsupportedTemplateExpansion,
}
```

Meaning:

- `InvalidTemplate`: generic RFC 6570 parsing failed;
- `NotAbsolute`: the configured endpoint does not identify an absolute HTTPS
  origin;
- `RequiresHttps`: scheme is not `https`;
- `MissingHost`: HTTPS authority has no host;
- `UserInfoNotAllowed`: endpoint includes URI user information;
- `FragmentNotAllowed`: endpoint contains a fragment;
- `InvalidDnsVariable`: GET-capable configuration uses the `dns` variable in
  an invalid way;
- `UnsupportedTemplateExpansion`: template uses a form that generic RFC 6570
  supports but revision-0.1 DoH deliberately does not accept.

## 6.3 `BootstrapError`

```sec
enum BootstrapError error {
    Required,
    EmptyAddressList,
    AddressFamilyUnavailable,
    ConnectionFailed,
    InvalidConfiguration,
}
```

`ConnectionFailed` is only used where bootstrap selection itself fails before
the HTTP layer has a normal connection error to expose.

## 6.4 HTTP status failure

```sec
type HttpStatusFailure struct {
    Status: http.StatusCode,
    RetryAfter: Option[Duration],
}
```

`RetryAfter` is populated only when the response contains a valid Retry-After
value representable by the common HTTP/time model.

## 6.5 `DohError`

```sec
union DohError error {
    Endpoint(EndpointError),
    Bootstrap(BootstrapError),

    Http(error),
    HttpStatus(HttpStatusFailure),

    Dns(dns.MessageError),

    UnexpectedContentType,
    MissingContentType,
    ResponseTooLarge,
    InvalidResponse,
    RedirectRejected,
    Closed,
}
```

Rules:

- `Http(error)` preserves the concrete error reported by `net/http`, including
  TLS/certificate/network failures carried by that package;
- `HttpStatus` is for a syntactically valid non-success HTTP response;
- `Dns` is for malformed DNS message content in a successful DoH HTTP body;
- `UnexpectedContentType` is not converted to a DNS RCODE;
- DNS `NXDomain`, `ServFail`, and other DNS response codes are **not**
  `DohError` merely because they indicate DNS failure;
- cancellation is not duplicated as a DoH-specific error;
- `Closed` is local resource state.

---

# Part II — Endpoint configuration

# 7. `endpoint.sec`

## 7.1 Responsibility

`endpoint.sec` owns a validated configured DoH URI Template.

It builds on the generic `net/url` RFC 6570 implementation.

## 7.2 `Endpoint`

```sec
type Endpoint struct {
    _template: url.UriTemplate,
}
```

Canonical public surface:

```sec
impl Endpoint {
    static fn Parse(
        template: string
    ) Result[Endpoint, EndpointError]

    static fn FromTemplate(
        template: url.UriTemplate
    ) Result[Endpoint, EndpointError]

    property Template: url.UriTemplate { get { ... } }

    fn ToString() string
}
```

`Endpoint` is immutable after validation.

## 7.3 Validation rules

A revision-0.1 DoH endpoint must:

- be an absolute URI Template;
- use the `https` scheme;
- contain an authority host;
- contain no URI userinfo;
- contain no fragment;
- expand to valid HTTPS request targets;
- support POST expansion with no variables;
- support GET expansion with the single `dns` variable when GET is selected;
- preserve explicitly configured static path/query components.

The endpoint may use:

```text
https://resolver.example/dns-query
```

for POST-only client use.

A GET-capable endpoint normally uses a template such as:

```text
https://resolver.example/dns-query{?dns}
```

A client configured for GET must reject an endpoint that cannot place the
`dns` variable into a valid request target.

## 7.4 Why userinfo is rejected

RFC 8484 does not require Sec to accept URI userinfo.

Revision 0.1 rejects it because:

- credentials embedded in configured resolver URIs are easy to leak;
- authorization should use the normal `net/http` authentication facilities;
- endpoint identity should remain explicit.

A later revision may reconsider this only with a concrete interoperability
need.

## 7.5 Redirect identity

`Endpoint` represents the configured DoH resolver identity.

An HTTP redirect does not silently replace that configured identity.

Redirect policy is controlled explicitly by `ClientOptions`.

---

# Part III — Bootstrap

# 8. `bootstrap.sec`

## 8.1 Responsibility

`bootstrap.sec` solves the DoH bootstrap problem.

A DoH endpoint such as:

```text
https://resolver.example/dns-query
```

requires the client to obtain an IP address for `resolver.example` before it
can send the DNS query that may itself be intended to provide DNS resolution.

A DoH client must not resolve its own configured HTTPS host through itself
before a usable connection exists.

RFC 8484 explicitly discusses this bootstrap deadlock.

## 8.2 `Bootstrap`

```sec
type Bootstrap union {
    System,
    Addresses(ip.Address[]),
}
```

Semantics:

### `System`

Use an already available platform/system host-resolution path that does not
depend on the newly constructed DoH client.

Examples may include:

- operating-system resolver service;
- previously configured classic DNS resolver;
- externally managed platform resolver.

If the implementation cannot ensure a non-recursive bootstrap path, it must
return:

```sec
BootstrapError.Required
```

rather than recursively invoking the same DoH resolver.

### `Addresses`

Use the explicitly configured IP addresses to establish HTTPS connections to
the DoH origin.

The configured HTTPS host name remains the:

- HTTP authority;
- TLS authentication name;
- certificate identity.

The bootstrap IP is a connection-routing hint, not the authenticated server
identity.

## 8.3 Address list rules

```sec
impl Bootstrap {
    static fn FromAddresses(
        addresses: ip.Address[]
    ) Result[Bootstrap, BootstrapError]
}
```

Rules:

- the list must not be empty;
- multiple addresses preserve caller order as input policy;
- the HTTP connection layer may race/select addresses according to its
  documented connection strategy;
- no address is treated as a certificate name unless the endpoint itself uses
  that IP literal as its HTTPS host;
- bootstrap addresses do not rewrite the configured URI.

## 8.4 Certificate-related bootstrap loops

Certificate validation can itself trigger network access on some platforms.

Implementations must avoid creating hidden dependence on the same unresolved
DoH path for certificate-chain or revocation retrieval where the selected TLS
backend exposes control over that behavior.

Where the TLS platform controls such behavior externally, Sec must document
the limitation rather than claiming stronger deadlock guarantees.

---

# Part IV — Options and policy

# 9. `options.sec`

## 9.1 `RequestMode`

DoH uses normal HTTP GET or POST, but the user-facing option describes DoH
request policy rather than duplicating `http.Method`.

```sec
enum RequestMode {
    Post,
    Get,
}
```

The actual HTTP request uses the existing `http.Method` enum.

No `"GET"` or `"POST"` magic string is used.

## 9.2 Default request mode

Revision 0.1 defaults to:

```sec
RequestMode.Post
```

Reasons:

- POST is directly defined by RFC 8484;
- the DNS wire message does not appear in the request URI;
- POST is generally smaller than the equivalent base64url GET request;
- query names are less likely to leak through URI-oriented logging,
  analytics, browser history, or intermediary URL telemetry.

GET remains fully supported because:

- RFC 8484 defines it;
- conforming DoH servers must support it;
- GET is friendlier to many HTTP cache implementations.

This is a Sec default policy, not a claim that POST is mandated by RFC 8484.

## 9.3 `RedirectPolicy`

```sec
enum RedirectPolicy {
    Reject,
    SameOrigin,
}
```

No unrestricted automatic redirect mode exists in revision 0.1.

`Reject` is the default.

`SameOrigin` allows redirects only when the resulting request remains within
the same HTTPS origin under the normal HTTP origin definition.

A redirect to another origin returns:

```sec
DohError.RedirectRejected
```

This avoids silently disclosing DNS query contents to a resolver origin the
caller did not configure.

## 9.4 `ClientOptions`

```sec
type ClientOptions struct {
    RequestMode: RequestMode,
    Redirects: RedirectPolicy,
    Bootstrap: Bootstrap,
    UseHttpCache: bool,
    MaxResponseBytes: uint,
}
```

Canonical default constructor:

```sec
impl ClientOptions {
    static fn Default() ClientOptions
}
```

Default values:

```text
RequestMode      = Post
Redirects        = Reject
Bootstrap        = System
UseHttpCache     = false
MaxResponseBytes = 65535
```

`MaxResponseBytes` must never exceed the maximum size supported by
`application/dns-message`.

A smaller application limit is allowed.

## 9.5 Why the local HTTP cache defaults off

The base `dns.Resolver` already has DNS-aware caching.

DoH still must correctly process HTTP `Age` and cache metadata because:

- an intermediary may cache the response;
- a supplied `http.Client` may have its own cache;
- callers may explicitly enable HTTP caching.

The default avoids an unnecessary second local cache layer while preserving
RFC 8484 correctness.

## 9.6 HTTP version policy

`ClientOptions` does not contain:

```text
HttpVersion = "h2"
```

or equivalent protocol magic.

HTTP version negotiation and connection management belong to `net/http`.

The DoH client should benefit from HTTP/2 and HTTP/3 multiplexing when the
configured HTTP client provides them.

---

# Part V — Media mapping

# 10. `media.sec`

## 10.1 Responsibility

`media.sec` owns the DoH `application/dns-message` mapping and GET/POST
body/URI transformations.

Primary references:

- RFC 8484
- RFC 4648
- RFC 6570

## 10.2 Media type

The canonical media type is:

```text
application/dns-message
```

The source code must use the canonical shared HTTP media-type representation
rather than repeatedly constructing the string.

Conceptually:

```sec
static let DnsMessageMediaType: http.MediaType := ...
```

The exact declaration must reuse the final `http.MediaType` constructor/API
defined by `std-net-http.md`.

It must not introduce a second local media-type type.

## 10.3 DNS message encoding

DoH uses the ordinary DNS wire message produced by:

```sec
dns.EncodeMessage(...)
```

and consumed by:

```sec
dns.DecodeMessageToOwned(...)
```

No DNS-over-TCP two-byte length prefix is present.

## 10.4 POST encoding

For POST:

- URI Template is expanded with no DoH variables;
- method is `http.Method.Post`;
- body is the unmodified DNS wire message;
- `Content-Type` is `application/dns-message`;
- `Accept` includes `application/dns-message`.

Conceptual helper:

```sec
fn BuildPostRequest(
    endpoint: ref Endpoint,
    message: ref dns.Message
) Result[http.Request, DohError]
```

## 10.5 GET encoding

For GET:

1. encode the DNS message in DNS wire format;
2. base64url-encode the bytes;
3. omit base64 `=` padding;
4. supply the result as RFC 6570 variable `dns`;
5. expand the configured endpoint template;
6. send an HTTP GET;
7. include `Accept: application/dns-message`.

Conceptual helper:

```sec
fn BuildGetRequest(
    endpoint: ref Endpoint,
    message: ref dns.Message
) Result[http.Request, DohError]
```

Generic base64url implementation should be reused from the appropriate
encoding/data package once that stdlib ownership is defined.

The DoH package must not create an unrelated base64 implementation solely for
this feature.

## 10.6 DNS message ID

For ordinary DoH queries generated by:

```sec
Client.Query(...)
```

the wire DNS `MessageId` is:

```text
0
```

This follows RFC 8484 cache-friendliness guidance because HTTP identifies the
request/response exchange.

`Client.Exchange()` preserves an explicitly constructed request's semantic
message content unless the operation is documented as a generated ordinary
query.

The implementation must not silently rewrite fields that could participate in
a higher-level authentication scheme.

TSIG/SIG(0) integration remains outside revision 0.1.

---

# Part VI — Client

# 11. `client.sec`

## 11.1 Responsibility

`client.sec` owns an HTTP-backed DoH client.

It maps:

```text
dns.Message
    ->
HTTPS request
    ->
HTTPS response
    ->
dns.Message
```

without reimplementing either DNS or HTTP.

## 11.2 Resource type

```sec
@noCopy
type Client struct {
    // Private Endpoint, ClientOptions, owned http.Client, and connection state.
}
```

`Client` is move-only because it owns an HTTP client/connection pool and
related state.

## 11.3 Construction

```sec
impl Client {
    static fn New(
        endpoint: Endpoint,
        options: ClientOptions
    ) Result[Client, DohError]

    static fn WithHttpClient(
        endpoint: Endpoint,
        httpClient: <- http.Client,
        options: ClientOptions
    ) Result[Client, DohError]

    property Endpoint: Endpoint { get { ... } }

    fn Exchange(
        request: ref dns.Message
    ) Result[dns.Message, DohError]

    fn Query(
        question: dns.Question
    ) Result[dns.Message, DohError]

    fn Close() Result[void, DohError]
}
```

When an existing named `http.Client` binding is consumed by
`WithHttpClient`, the Sec call site uses the normal explicit move marker:

```sec
let dohClient := try doh.Client.WithHttpClient(
    endpoint,
    <-httpClient,
    options
)
```

Fresh temporary construction follows the ordinary Sec ownership rules.

## 11.4 `New`

`Client.New` creates/configures an internal `http.Client`.

That HTTP client must:

- use HTTPS;
- preserve the configured DoH authority for TLS authentication;
- use the selected bootstrap policy;
- obey the configured redirect policy;
- obey the configured local HTTP-cache policy;
- use the shared HTTP connection pool and HTTP version negotiation;
- support response size limits before materializing unbounded body data.

It must not create a private TLS implementation.

## 11.5 `WithHttpClient`

`WithHttpClient` allows applications to supply an already configured HTTP
client for:

- proxy policy;
- TLS trust;
- client certificates;
- HTTP connection pooling;
- observability;
- custom transport configuration.

The DoH package still enforces DoH-specific rules.

A custom HTTP client does not get permission to:

- send the request over plain HTTP;
- follow disallowed redirects;
- accept a wrong media type;
- skip DNS body validation;
- ignore `Age` for DNS TTL purposes.

Where the general HTTP client exposes behavior that conflicts with mandatory
DoH rules, DoH applies the stricter rule.

## 11.6 `Query`

`Query`:

- creates a normal single-question DNS query;
- uses message ID zero;
- maps according to `RequestMode`;
- returns any syntactically valid DNS response regardless of DNS RCODE;
- validates that the returned DNS message is a response to the sent question;
- applies HTTP age to the returned DNS TTL semantics.

As in the direct base DNS client, DNS failure codes are DNS results, not
transport errors.

## 11.7 `Exchange`

`Exchange` accepts an explicitly built DNS message.

It is intended for callers that need direct DNS-message control.

It must:

- encode exactly one DNS wire message;
- map to exactly one HTTP exchange;
- decode exactly one DNS wire response;
- reject HTTP bodies larger than configured limits;
- validate response media type;
- return a nonzero DNS RCODE as a valid `dns.Message`;
- preserve protocol fields except where RFC 8484 requires transport-specific
  handling.

It does not support protocols requiring a multi-message DNS response, such as
AXFR, as one `application/dns-message` exchange.

## 11.8 HTTP response status

A successful DoH exchange requires a successful HTTP response status carrying
a DNS response representation.

A DNS response with:

```text
NXDOMAIN
SERVFAIL
REFUSED
```

still uses successful HTTP status when the DoH server processed the HTTP
exchange and returned a valid DNS response.

A non-success HTTP status is not decoded as a DNS answer.

Instead:

```sec
DohError.HttpStatus(...)
```

is returned.

## 11.9 Content type

The client must be able to process:

```text
application/dns-message
```

A successful HTTP response with another media type is rejected in revision
0.1 unless a later standards-backed media type is explicitly added.

No content-sniffing fallback is used.

The client does not assume that arbitrary:

```text
application/dns+json
application/json
text/plain
```

is RFC 8484 binary DNS.

## 11.10 Response body limit

The RFC 8484 binary media type is limited to 65535 bytes.

The client must enforce the configured response limit **while reading** the
HTTP body.

It must not first allocate an arbitrarily large body and only then reject it.

## 11.11 HTTP `Age` and DNS TTL

DoH clients must account for HTTP `Age` when calculating DNS TTL.

Therefore, before a DoH response is handed to the resolver/cache layer, its
effective remaining TTLs must reflect the age of an HTTP-cached response.

Revision 0.1 uses this rule:

```text
effectiveTTL = max(0, dnsTTL - httpAge)
```

for cacheable RRsets carried by the response.

Negative-cache lifetime is reduced consistently according to the DNS negative
caching rules and HTTP age.

The implementation must not:

- cache an RRset for its original full DNS TTL after receiving a cached HTTP
  response with nonzero Age;
- subtract Age twice when the same already-adjusted response enters the DNS
  cache.

The internal response metadata must therefore track whether HTTP-age
adjustment has already been applied.

## 11.12 HTTP cache controls

When local HTTP caching is enabled:

- HTTP cache behavior follows `net/http` and RFC 9111;
- DoH-specific RFC 8484 freshness restrictions still apply;
- DNS TTL and HTTP freshness must remain coherent;
- stale HTTP cache data must not silently become fresh DNS data.

When local HTTP caching is disabled, intermediaries may still return an
`Age` header, so TTL adjustment remains required.

## 11.13 Connection reuse and multiplexing

The client should reuse HTTP connections.

Multiple DNS exchanges may execute concurrently where the underlying HTTP
version/client supports multiplexing.

DoH does not provide ordering guarantees between separate DNS exchanges.

The implementation must not impose TCP-style request ordering semantics on
independent DoH requests merely because one underlying connection is shared.

## 11.14 Close

```sec
fn Close() Result[void, DohError]
```

is:

- explicit;
- idempotent;
- responsible for closing owned HTTP resources;
- not required merely to avoid leaks because deterministic destruction remains
  the ownership backstop.

Operations after local close return:

```sec
DohError.Closed
```

---

# Part VII — Server-side mapping

# 12. `server.sec`

## 12.1 Responsibility

`server.sec` implements the RFC 8484 HTTP mapping required by a DoH server.

It does not implement:

- recursive DNS resolution;
- authoritative DNS serving;
- DNSSEC signing;
- HTTP listener management;
- TLS termination.

Instead, it converts between `http.Request` / `http.Response` and
`dns.Message`.

A user/server package supplies the actual DNS backend.

## 12.2 Server requirements

A conforming Sec DoH server adapter must support **both**:

```text
GET
POST
```

for the configured DoH resource.

It must support:

```text
application/dns-message
```

## 12.3 Server options

```sec
type ServerOptions struct {
    MaxRequestBytes: uint,
}
```

Canonical default:

```sec
impl ServerOptions {
    static fn Default() ServerOptions
}
```

with:

```text
MaxRequestBytes = 65535
```

The configured value must not exceed the RFC 8484 binary DNS-message maximum.

## 12.4 Request decoding

```sec
fn DecodeRequestToOwned(
    request: ref http.Request,
    endpoint: ref Endpoint,
    options: ServerOptions
) Result[dns.Message, DohError]
```

The function:

- verifies the request targets the configured DoH endpoint/template;
- accepts GET and POST;
- rejects unsupported methods using HTTP-layer server mapping policy;
- for GET, extracts the `dns` URI Template variable;
- base64url-decodes GET data without requiring padding;
- for POST, requires the supported DNS media type;
- limits body size while reading;
- decodes exactly one DNS wire message;
- returns an owning `dns.Message`.

The HTTP server integration layer is responsible for mapping request decode
failures to appropriate HTTP status responses.

## 12.5 Response encoding

```sec
fn EncodeResponse(
    message: ref dns.Message,
    output: ref mut byte[]
) Result[http.Response, DohError]
```

The returned HTTP response:

- has a successful HTTP status for any valid DNS response, including nonzero
  DNS RCODE;
- uses `Content-Type: application/dns-message`;
- contains one DNS wire message;
- includes appropriate caching metadata where server policy allows caching.

The exact `http.Response` body ownership form must be synchronized with the
final `std-net-http.md` body/stream model.

If the final HTTP API uses a streaming body rather than `ref mut byte[]`,
this declaration must be mechanically adapted without changing the DoH
semantics.

## 12.6 Server cache freshness

When a DoH server marks a response fresh for HTTP caching:

- freshness lifetime must not exceed the smallest TTL in the DNS Answer
  section;
- equal to the smallest relevant TTL is the normal recommendation;
- when no Answer records exist and negative caching is driven by SOA data,
  HTTP freshness must not exceed the applicable negative-cache lifetime;
- personalized/non-global responses must use HTTP cache controls that prevent
  inappropriate shared reuse.

The server adapter must not choose a generic long HTTP cache duration that
outlives DNS data.

## 12.7 EDNS UDP payload size

For DoH:

- the message may contain EDNS;
- the server ignores the EDNS UDP payload-size field for HTTP transport sizing.

That field describes UDP behavior and is not a DoH body limit.

The DoH `application/dns-message` limit remains the applicable binary message
limit.

## 12.8 DNS truncation bit

A DoH server is allowed to return a syntactically valid DNS response with the
DNS `TC` bit set.

The client must preserve that DNS semantic value.

DoH transport itself is not UDP and does not automatically retry the same
response over DNS/TCP merely because TC is present.

Higher resolver policy may choose what to do with such a DNS response.

---

# Part VIII — Integration with common DNS resolution

# 13. `transport.sec`

## 13.1 Purpose

DoH must be usable by the same high-level DNS resolution logic that handles
classic DNS.

It is incorrect to implement:

```text
dns.Resolver
doh.Resolver
dot.Resolver
doq.Resolver
```

as four independently duplicated resolver engines.

The differences are transport.

The common resolver owns:

- CNAME following;
- positive caching;
- negative caching;
- RR filtering;
- address lookup;
- reverse lookup;
- retry policy appropriate to resolver servers;
- common DNS result semantics.

DoH owns only its transport-specific exchange behavior.

## 13.2 Required parent-package transport contract

`std-net-dns.md` revision 0.1 predates this child design and currently models
classic transport too directly.

The parent DNS book must be revised to introduce a small transport exchange
contract.

The required semantic shape is:

```sec
interface QueryTransport {
    fn Exchange(
        request: ref Message
    ) Result[Message, error]
}
```

Canonical owner:

```text
net/dns
```

not `net/dns/doh`.

The `error` result is intentional:

- classic DNS may return `dns.TransportError`;
- DoH may return `doh.DohError`;
- DoT may return its own transport error;
- DoQ may return its own transport error.

All remain Sec errors.

The shared resolver must preserve the concrete underlying error rather than
flattening every transport into a lossy string.

## 13.3 DoH conformance

`doh.Client` satisfies the `dns.QueryTransport` semantic contract through its
`Exchange` operation.

The exact Sec interface-conformance syntax must follow the canonical interface
rulebook when implemented.

No duplicate `DohTransport` interface is introduced.

## 13.4 Resolver ownership correction

The parent DNS implementation should separate:

```text
resolver engine/policy/cache
```

from:

```text
concrete query transport resource
```

so a caller can configure the same resolver semantics with:

```text
classic UDP/TCP client
DoH client
DoT client
DoQ client
```

without a base-package dependency on child packages.

The final constructor shape must be revised in `std-net-dns.md` before
implementation is declared complete.

This child book therefore establishes a required **parent-book correction**,
not permission to duplicate resolver logic locally.

## 13.5 No `doh.Resolver`

Revision 0.1 deliberately does not define:

```sec
doh.Resolver
```

Applications that need high-level lookups should use the common DNS resolver
engine configured with a DoH query transport once the parent correction is
integrated.

Direct users may use:

```sec
doh.Client.Query(...)
doh.Client.Exchange(...)
```

without the high-level resolver.

---

# Part IX — Security and privacy

# 14. HTTPS authentication is mandatory

DoH is DNS over **HTTPS**, not DNS over arbitrary HTTP.

Endpoint validation requires:

```text
https
```

TLS certificate authentication follows the configured DoH URI host/origin.

The package must not expose:

```text
AllowInsecureHttp = true
SkipCertificateValidation = true
```

as ordinary DoH convenience options.

Any intentionally unsafe TLS testing mechanism belongs to the canonical HTTP
or TLS testing/unsafe surface, not to DoH.

---

# 15. Bootstrap identity separation

Explicit bootstrap addresses affect connection routing only.

For:

```text
https://resolver.example/dns-query
```

with bootstrap address:

```text
192.0.2.53
```

the authenticated identity remains conceptually:

```text
resolver.example
```

not:

```text
192.0.2.53
```

unless the configured URI itself uses the IP address and the HTTPS
certificate is valid for that identity under the canonical TLS rules.

---

# 16. Query privacy

DoH protects DNS contents against passive observers between client and HTTPS
server, subject to the guarantees of HTTPS.

It does **not** hide the query from:

- the configured DoH server;
- application-local logging;
- HTTP request logging at trusted intermediaries;
- endpoint software after TLS termination.

GET additionally places the encoded DNS query in the request URI.

This is one reason Sec defaults the client to POST.

Applications that need resolver-query unlinkability beyond ordinary DoH require
a different privacy design such as an explicitly supported oblivious protocol.

---

# 17. Redirect privacy

Unrestricted redirects can disclose DNS queries to a new origin.

Therefore:

```sec
RedirectPolicy.Reject
```

is the default.

Same-origin redirects remain opt-in.

Cross-origin redirects are rejected in revision 0.1.

A future explicit allow-list policy may be added if real deployments require
it, but it must not become an unrestricted boolean "follow redirects" switch.

---

# 18. HTTP headers and metadata

DoH must use the ordinary HTTP header model.

The package must not duplicate header parsing.

Sensitive headers supplied to a custom HTTP client must obey ordinary HTTP
redirect/header-forwarding security rules.

DoH does not automatically copy:

- Authorization;
- Cookie;
- Proxy-Authorization;
- user tracking headers

to a different resolver origin.

---

# 19. Server push and unsolicited answers

DoH response data is usable only for configured/request-authorized resolver
URIs.

The package must not accept unsolicited HTTP responses or server-pushed DNS
data as resolver truth merely because the bytes decode as a DNS message.

Revision 0.1 does not expose a server-push DoH API.

---

# 20. Response validation

Before a response is accepted for a generated ordinary `Query`, the client
must validate at least:

- it is a DNS response;
- question semantics correspond to the request;
- message is syntactically valid;
- HTTP status is successful;
- response media type is supported;
- body size is within limits.

HTTP request/stream identity replaces the need to use random DNS transaction
IDs for correlation in ordinary generated DoH queries.

---

# Part X — HTTP caching and DNS caching

# 21. Separate cache layers

DoH can encounter both:

```text
HTTP cache
DNS cache
```

They are not interchangeable.

The HTTP cache owns HTTP response reuse.

The DNS cache owns DNS RRset/negative-response lifetime semantics.

The DoH layer connects the two by carrying HTTP age into DNS remaining TTL.

## 21.1 Avoid double-aging

A cached HTTP response may already contain:

```text
Age: N
```

The DoH decoder adjusts effective DNS TTL once.

If that DNS message is then inserted into the DNS resolver cache, the resolver
starts its local cache lifetime from the already adjusted value.

It must not subtract the same HTTP Age again.

## 21.2 HTTP cache key and DNS message ID

Generated ordinary DoH queries use DNS message ID zero.

This allows semantically equivalent GET queries to have stable request content
and improves HTTP cache sharing.

The package must not vary generated IDs without a reason that outweighs DoH
cache semantics.

## 21.3 GET versus POST caching

GET is naturally cache-oriented under HTTP semantics.

POST may be cached only when normal HTTP caching rules and response metadata
make it reusable.

Sec does not invent separate DoH cache semantics that override HTTP.

---

# Part XI — Discovery boundary

# 22. RFC 9461 `dohpath`

RFC 9461 defines a `dohpath` SVCB parameter for DNS services.

The value is a relative URI Template containing the `dns` variable.

`net/dns/doh.Endpoint` is the natural target type after discovery logic has:

- validated the SVCB record;
- selected an HTTP ALPN;
- combined the authentication name/port with `dohpath`;
- produced a complete HTTPS endpoint template.

The DoH package may later provide a constructor that consumes a common
encrypted-DNS discovery result.

It must not itself own all DDR discovery behavior.

---

# 23. Discovery of Designated Resolvers

RFC 9462 spans:

```text
DoH
DoT
DoQ
```

and should therefore be implemented in a common DNS encrypted-resolver
discovery package or parent DNS facility.

Possible future location:

```text
net/dns/discovery
```

This location is an observation, not yet locked.

DoH must not independently implement a competing DDR stack.

---

# Part XII — Server integration with HTTP

# 24. HTTP handler adaptation

The final `std-net-http.md` may define a canonical server handler interface.

When that interface is locked, `net/dns/doh` should provide a thin adapter
rather than forcing users to manually write the same mapping.

Conceptual future shape:

```sec
@noCopy
type Handler struct {
    // backend + options
}
```

or an interface-conforming adapter.

This revision does not lock that exact handler type before the HTTP server API
exists.

The semantics are already locked through:

```sec
DecodeRequestToOwned(...)
EncodeResponse(...)
```

so adding the later adapter does not change DoH protocol behavior.

---

# Part XIII — Ownership, cancellation, and concurrency

# 25. Ownership

`doh.Client` is an owned `@noCopy` resource.

It may own:

- `http.Client`;
- connection pool state;
- bootstrap policy/storage;
- endpoint configuration;
- local HTTP cache state when enabled.

`Endpoint`, `Bootstrap`, and option values are ordinary semantic values.

## 25.1 Supplied HTTP client

`WithHttpClient` consumes ownership of the supplied client.

The DoH client then owns its lifecycle.

A later borrowed-client constructor may be added only if lifetime and
connection-pool ownership semantics are explicit.

---

# 26. Cancellation

Potentially blocking DoH operations include:

- bootstrap resolution;
- HTTPS connection;
- TLS handshake;
- HTTP request write;
- HTTP response wait;
- HTTP body read.

They participate in Sec's normal cancellation/context model.

No:

```text
DohCancellationToken
```

is introduced.

Cancellation must not:

- publish a partial DNS response;
- cache an incomplete DNS message;
- convert cancellation into NXDOMAIN/SERVFAIL;
- lose ownership of the client.

---

# 27. Concurrency

Independent DoH HTTP requests are semantically independent.

A single `doh.Client` should support concurrent exchanges if the final
`http.Client` concurrency contract supports them.

HTTP/2 and HTTP/3 multiplexing should be used by the HTTP layer where
available.

DoH must not require one connection per query.

The package does not promise response order across independent exchanges.

---

# Part XIV — Platform capability

# 28. Pure versus active facilities

Pure DoH facilities include:

- endpoint validation;
- GET base64url mapping;
- POST body mapping;
- server-side request decoding;
- server-side response encoding.

Active DoH client operations require:

- `net/http`;
- HTTPS/TLS support;
- active IP networking;
- a usable bootstrap path.

A target lacking those active capabilities must reject active use through the
normal platform-capability model.

It must not silently downgrade DoH to cleartext DNS.

---

# Part XV — Explicit exclusions

# 29. No cleartext fallback

DoH failure does not automatically mean:

```text
retry via UDP/53
```

because that changes privacy/security properties.

Fallback to classic DNS is an explicit resolver/application policy outside
the DoH transport.

---

# 30. No arbitrary JSON DoH

Revision 0.1 supports the standards-defined interoperable media type:

```text
application/dns-message
```

It does not implement provider-specific JSON DNS APIs merely because they are
served over HTTPS.

Such APIs are not RFC 8484 binary DoH.

---

# 31. No ODoH

Oblivious DoH is separate.

If implemented later, it must not appear as:

```sec
ClientOptions.Oblivious = true
```

on an ordinary DoH client.

Its relay/target/encryption model is substantially different.

---

# 32. No proxy-specific DNS semantics

HTTP proxies may be used through the supplied/configured `http.Client`.

The DoH package does not create a second proxy stack.

Proxy DNS behavior, CONNECT behavior, authentication, and proxy TLS semantics
belong to `net/http` and related proxy packages.

---

# Part XVI — Repository work

# 33. Current repository state

The parent DNS package currently has no DoH implementation in the verified
design baseline.

`std-net-dns.md` reserves:

```text
net/dns/doh
```

for this child package.

An implementation task should treat this book as the first normative DoH API
definition.

---

# 34. Required source creation

Create:

```text
sec/stdlib/net/dns/doh/
├── std-net-dns-doh.md
├── error.sec
├── endpoint.sec
├── bootstrap.sec
├── options.sec
├── media.sec
├── client.sec
├── server.sec
└── transport.sec
```

Existing files, if encountered on a newer repository revision, must be:

- inspected;
- expanded/corrected where useful;
- migrated rather than blindly overwritten;
- reconciled against this normative API.

---

# 35. Required parent correction

Before DoH resolver integration is considered complete:

```text
sec/stdlib/net/dns/std-net-dns.md
```

must be revised to support a common DNS query-transport contract or equivalent
non-cyclic resolver-engine abstraction.

The design must allow the same resolver logic to operate above:

- classic DNS;
- DoH;
- DoT;
- DoQ.

Do not solve this by making base `net/dns` import all child transports.

---

# Part XVII — Tests

# 36. Endpoint tests

At minimum:

- valid HTTPS POST endpoint;
- valid GET URI Template with `dns`;
- plain HTTP rejected;
- relative template rejected;
- missing host rejected;
- URI userinfo rejected;
- fragment rejected;
- invalid URI Template rejected;
- GET mode without usable `dns` expansion rejected;
- static query parameters preserved;
- canonical `ToString()`.

---

# 37. Bootstrap tests

At minimum:

- system bootstrap success where supported;
- explicit IPv4 bootstrap;
- explicit IPv6 bootstrap;
- multiple explicit addresses;
- empty explicit list rejected;
- certificate authentication still uses configured host;
- bootstrap address does not rewrite HTTP Host/authority;
- self-recursive bootstrap detected/rejected where observable.

---

# 38. GET tests

At minimum:

- DNS wire message encoded via base64url;
- no `=` padding;
- RFC 6570 `dns` expansion;
- `http.Method.Get`;
- `Accept: application/dns-message`;
- no POST body;
- exact known RFC-compatible vector.

---

# 39. POST tests

At minimum:

- raw DNS wire body;
- no base64 transformation;
- `http.Method.Post`;
- `Content-Type: application/dns-message`;
- `Accept: application/dns-message`;
- endpoint expanded without DoH variables.

---

# 40. Client response tests

At minimum:

- HTTP 200 + DNS NOERROR;
- HTTP 200 + DNS NXDOMAIN remains successful `dns.Message`;
- HTTP 200 + DNS SERVFAIL remains successful `dns.Message`;
- non-success HTTP status -> `DohError.HttpStatus`;
- missing content type;
- wrong content type;
- malformed DNS body;
- oversized body stopped while streaming;
- cancellation during body read;
- close and post-close operation.

---

# 41. Cache-age tests

At minimum:

- no Age;
- Age smaller than TTL;
- Age equal to TTL;
- Age greater than TTL -> zero effective TTL;
- multiple RR TTLs;
- negative response with SOA;
- no double subtraction when inserted into DNS cache;
- intermediary Age honored even when local HTTP cache is disabled.

---

# 42. Redirect tests

At minimum:

- default redirect rejection;
- same-origin redirect rejected when policy is `Reject`;
- same-origin redirect accepted when policy is `SameOrigin`;
- cross-origin redirect rejected under all revision-0.1 policies;
- sensitive headers are not leaked to another origin by DoH logic.

---

# 43. Server tests

At minimum:

- GET accepted;
- POST accepted;
- unsupported method rejected through HTTP mapping;
- GET malformed base64url;
- GET padded/noncanonical cases according to RFC 4648 policy;
- POST correct media type;
- POST wrong media type;
- request size limit;
- valid DNS NXDOMAIN encoded with successful HTTP status;
- `application/dns-message` response;
- cache freshness does not exceed smallest answer TTL;
- negative response freshness bounded by DNS negative-cache semantics;
- EDNS UDP payload size ignored for DoH transport sizing.

---

# 44. Integration tests

At minimum:

- DoH client through HTTP/1.1 where supported;
- DoH client through HTTP/2;
- DoH client through HTTP/3 once `net/http` + `net/quic` provide it;
- bootstrap host name plus explicit IP;
- TLS certificate mismatch rejected;
- same resolver logic can operate above classic DNS and DoH without code
  duplication;
- no base `net/dns -> net/dns/doh` dependency;
- no `net/http -> net/dns/doh` dependency.

---

# Part XVIII — AI/repository implementation instructions

# 45. Implementation requirements

An AI or repository automation implementing this book must:

1. treat this file as the normative DoH package document;
2. create the declared source files when missing;
3. inspect and update existing DoH files rather than blindly overwriting them;
4. add RFC/source URL headers to each source file;
5. reuse `dns.Message`, `dns.Question`, `dns.EncodeMessage`, and
   `dns.DecodeMessageToOwned`;
6. reuse `http.Client`, `http.Request`, `http.Response`, `http.Method`,
   `http.StatusCode`, headers, media types, cache behavior, and HTTPS;
7. reuse the canonical RFC 6570 type from `net/url`;
8. reuse a shared base64url implementation rather than creating a private
   incompatible codec;
9. require HTTPS;
10. preserve configured HTTPS authentication name when bootstrap IPs are used;
11. support GET and POST;
12. default the client to POST;
13. default redirects to reject;
14. support `application/dns-message`;
15. enforce the 65535-byte media payload maximum while reading;
16. treat DNS nonzero RCODE as DNS response, not HTTP transport error;
17. treat non-success HTTP status as HTTP/DoH failure, not DNS RCODE;
18. account for HTTP Age in effective DNS TTL;
19. prevent double Age subtraction before resolver caching;
20. avoid unbounded body allocation;
21. avoid self-recursive bootstrap;
22. never silently downgrade to UDP/TCP cleartext DNS;
23. never duplicate HTTP or TLS implementation inside DoH;
24. add/update the parent DNS transport abstraction before claiming resolver
    integration complete;
25. add the required tests;
26. update `implementation-status-std-net-dns-doh.yaml`;
27. reconcile status into canonical `implementation-status.yaml` without
    replacing stronger/newer evidence.

---

# 46. Recommended implementation order

1. create package/book/files;
2. implement `Endpoint`;
3. implement `Bootstrap`;
4. implement options/defaults;
5. wire shared `application/dns-message` media type;
6. implement POST request builder;
7. implement GET/base64url request builder;
8. implement HTTP response validation;
9. implement body-size limiting;
10. implement HTTP Age -> DNS TTL adjustment;
11. implement `Client`;
12. implement server request decode;
13. implement server response encode/cache metadata;
14. revise parent DNS transport abstraction;
15. integrate `doh.Client` into common resolver engine;
16. add HTTP/2 integration tests;
17. add HTTP/3 integration tests when the HTTP/QUIC stack exists;
18. consider RFC 9461/9462 common discovery package later.

---

# Part XIX — Open observations

# 47. Items intentionally not locked in revision 0.1

The following remain open:

- final exact `net/url` URI Template type name if `std-net-url.md` chooses a
  different canonical name than `url.UriTemplate`;
- final exact `http.MediaType` construction syntax;
- final exact HTTP request/response body ownership/streaming API;
- final HTTP handler interface and whether DoH provides a ready-made handler;
- exact generic/interface syntax used for `dns.QueryTransport`;
- final parent resolver constructor accepting alternate transports;
- explicit cross-origin redirect allow-list support;
- automatic method selection between GET and POST;
- DNS Cookie behavior over DoH;
- local HTTP cache implementation policy;
- proxy-specific bootstrap interactions;
- client authentication/certificate API, owned by HTTP/TLS;
- RFC 9461 `dohpath` convenience constructor;
- RFC 9462 DDR package placement;
- ODoH package placement;
- server authentication/authorization hooks;
- rate limiting and abuse mitigation for public DoH servers;
- HTTP/2 padding/privacy policy;
- HTTP/3-specific performance tuning.

These observations are not permission to invent incompatible public APIs.

---

# 48. Decisions established by revision 0.1

Revision 0.1 establishes:

- canonical package path `net/dns/doh`;
- DoH is a child of `net/dns`, not part of the base package implementation;
- RFC 8484 is the primary protocol standard;
- current HTTP semantics/caching RFCs are used where they supersede older HTTP
  references in RFC 8484;
- the package reuses `net/dns`, `net/http`, `net/url`, and `net/ip`;
- DoH does not implement its own HTTP or TLS stack;
- DoH endpoints are validated HTTPS URI Templates;
- URI userinfo and fragments are rejected in revision 0.1;
- GET and POST are both supported;
- client default request mode is POST;
- GET uses unpadded base64url in the `dns` URI Template variable;
- POST uses raw DNS wire bytes;
- `application/dns-message` is mandatory;
- the binary body maximum is 65535 bytes;
- generated ordinary DoH queries use DNS message ID zero;
- successful HTTP status can carry any valid DNS RCODE;
- non-success HTTP status is not a DNS response;
- HTTP `Age` reduces effective DNS TTL;
- local HTTP cache defaults off while Age handling remains mandatory;
- redirects default to reject;
- revision 0.1 permits same-origin redirects only when explicitly enabled;
- cross-origin redirects are rejected;
- bootstrap addresses never replace the HTTPS authentication name;
- no silent cleartext DNS fallback exists;
- DoH server mapping supports both GET and POST;
- server HTTP cache freshness cannot outlive DNS freshness;
- DoH client is `@noCopy`;
- DoH uses the canonical Sec cancellation/context model;
- high-level resolver logic must be shared across classic DNS/DoH/DoT/DoQ;
- no duplicate `doh.Resolver` is introduced;
- the parent DNS book requires a transport-abstraction revision;
- DDR is common encrypted-DNS discovery and is not owned solely by DoH;
- ODoH is separate from ordinary DoH.

---

**Document revision:** 0.1  
**Last updated:** 2026-09-07
