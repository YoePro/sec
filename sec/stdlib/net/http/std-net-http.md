# Sec Standard Library — HTTP

- **Status:** Draft
- **Created:** 2026-09-07
- **Last updated:** 2026-09-07
- **Document revision:** 0.1
- **Sec language version:** 0.1
- **Canonical path:** `stdlib/net/http/std-net-http.md`
- **Repository path:** `sec/stdlib/net/http/std-net-http.md`
- **Parent specification:** `stdlib/net/std-net.md`
- **Implementation-status fragment:** `implementation-status-std-net-http.yaml`
- **Latest verified repository main:** `0f5027d`
- **Repository HTTP tree rechecked:** 2026-09-07
- **Standards/registry state rechecked:** 2026-09-07

---

# 1. Purpose

`net/http` is Sec's common Hypertext Transfer Protocol package.

It owns HTTP semantics shared across protocol versions and the ordinary client
and server abstractions used by Sec applications.

The package covers:

- HTTP methods and status codes;
- HTTP versions;
- HTTP fields (headers and trailers);
- Structured Fields;
- request-target semantics;
- request and response messages;
- streaming message bodies;
- redirects;
- HTTP authentication challenge/value parsing;
- digest fields;
- priorities;
- HTTP caching;
- cookies and cookie jars;
- HTTP/1.1;
- HTTP/2 and HPACK;
- HTTP/3 and QPACK;
- HTTP client connection management;
- HTTP server primitives;
- routing convenience;
- cancellation, deadlines, ownership, and resource limits;
- integration with `net/url`, `net/dns`, `net/ip`, `net/quic`, and the
  canonical TLS package.

The package is imported as:

```sec
import "net/http"
```

and used through:

```sec
http.Client
http.Request
http.Response
http.Header
http.Cookie
http.SetCookie
http.Status
http.Method
```

Example:

```sec
let client := try http.Client.New(http.ClientOptions.Default())
let request := try http.Request.Get(try url.URL.Parse("https://example.com/"))

let response := try client.Do(<-request)
defer response.Close()

if response.Status == http.Status.Ok {
    // Consume response.Body.
}
```

The full package path identifies the package. Source code uses `http`, not
`net.http`.

---

# 2. Package architecture

## 2.1 One semantic HTTP package

HTTP/1.1, HTTP/2, and HTTP/3 share the semantics defined by RFC 9110.

Sec therefore exposes one ordinary public package:

```text
net/http
```

rather than forcing users to choose:

```text
net/http1
net/http2
net/http3
```

for ordinary requests and responses.

Protocol-version details remain separate implementation files inside the same
package.

## 2.2 Version-specific files are not separate public packages

Initial implementation files include:

```text
http1.sec
http2.sec
hpack.sec
http3.sec
qpack.sec
```

All use:

```sec
module http
```

The client and server select/negotiate protocol versions according to
configuration, endpoint capability, ALPN, Alt-Svc/SVCB information, and
available platform capabilities.

## 2.3 Package dependencies

`net/http` depends conceptually on:

```text
net/url
net/dns
net/ip
net/quic          # HTTP/3 only
canonical TLS     # HTTPS, HTTP/2 over TLS, HTTP/3/QUIC security
```

It must not duplicate those packages' responsibilities.

## 2.4 WebSocket is separate

WebSocket is a distinct protocol family:

```text
net/websocket
```

The current root HTTP file:

```text
stdlib/net/http/websockets.sec
```

is only a one-line stub and must be removed/migrated when `net/websocket` is
implemented.

HTTP retains the upgrade/extended-CONNECT mechanisms that WebSocket uses, but
the WebSocket protocol itself does not live in `net/http`.

## 2.5 URL encoding is separate

The current:

```text
stdlib/net/http/encode.sec
```

contains only placeholder `URLEncode`/`URLDecode` functions.

Percent-encoding and URL parsing belong to:

```text
net/url
```

not to HTTP.

That file must be removed after its responsibility is migrated to `net/url`.

HTTP-specific message encoding remains in the version-specific HTTP transport
files.

---

# 3. Core design principles

1. **HTTP semantics are version-independent.** `Request`, `Response`,
   `Method`, `Status`, and `Header` do not become different application types
   merely because the connection uses HTTP/1.1, HTTP/2, or HTTP/3.
2. **Extensible registries stay open.** Methods, status codes, field names,
   content codings, authentication schemes, and other open registries must
   preserve extension values.
3. **Known registry values get typed names.** Common registered values are
   exposed through type-associated constants without closing the namespace.
4. **No header injection.** Field names/values must be validated before
   entering an HTTP message.
5. **Message bodies stream.** The public client/server model must not require
   complete request/response bodies in memory.
6. **Body ownership is explicit.** A received response owns a body resource
   that must be consumed/closed or deterministically destroyed.
7. **Connection pooling is internal.** Applications operate on requests,
   responses, and clients rather than native sockets.
8. **DNS is not HTTP.** Name resolution is supplied by `net/dns`.
9. **URLs are not raw strings internally.** Client requests use the canonical
   `net/url` value.
10. **TLS is not HTTP implementation code.** HTTPS uses the canonical TLS
    package.
11. **HTTP/3 uses `net/quic`.** It is not implemented as ad hoc UDP handling.
12. **Cookies follow RFC 10025.** RFC 6265 is obsolete and is not the current
    normative cookie baseline.
13. **Cookie persistence is opt-in.** A generic HTTP client does not silently
    persist application state unless a cookie jar is configured.
14. **Caching is opt-in by default.** The generic HTTP client is not a browser
    cache unless explicitly configured.
15. **Redirect behavior is bounded and security-aware.**
16. **Cancellation uses Sec's common execution model.** HTTP does not invent a
    separate cancellation token.
17. **Resource limits are first-class.** Header size, body size, stream count,
    HPACK/QPACK state, redirects, pending requests, cache, and cookie storage
    are bounded.
18. **Unknown extension data is preserved where protocol semantics require
    it.**
19. **Existing useful Sec implementations are expanded, not discarded solely
    because this book was written later.**
20. **Protocol sentinel values are not invented inside wire namespaces.**

---

# 4. Standards and registries

## 4.1 Core HTTP

The current HTTP core is the RFC 9110 series:

- **RFC 9110 / STD 97 — HTTP Semantics**  
  https://www.rfc-editor.org/rfc/rfc9110
- **RFC 9111 / STD 98 — HTTP Caching**  
  https://www.rfc-editor.org/rfc/rfc9111
- **RFC 9112 / STD 99 — HTTP/1.1**  
  https://www.rfc-editor.org/rfc/rfc9112
- **RFC 9113 — HTTP/2**  
  https://www.rfc-editor.org/rfc/rfc9113
- **RFC 9114 — HTTP/3**  
  https://www.rfc-editor.org/rfc/rfc9114

RFC 9110 defines semantics shared by all HTTP versions.

## 4.2 HTTP/2 field compression

- **RFC 7541 — HPACK**  
  https://www.rfc-editor.org/rfc/rfc7541

## 4.3 HTTP/3 field compression

- **RFC 9204 — QPACK**  
  https://www.rfc-editor.org/rfc/rfc9204

## 4.4 HTTP priorities

- **RFC 9218 — Extensible Prioritization Scheme for HTTP**  
  https://www.rfc-editor.org/rfc/rfc9218

## 4.5 HTTP Structured Fields

- **RFC 9651 — Structured Field Values for HTTP**  
  https://www.rfc-editor.org/rfc/rfc9651

RFC 9651 obsoletes RFC 8941.

## 4.6 Digest fields

- **RFC 9530 — Digest Fields**  
  https://www.rfc-editor.org/rfc/rfc9530

RFC 9530 obsoletes the older `Digest` / `Want-Digest` field model from
RFC 3230.

## 4.7 HTTP QUERY

- **RFC 10008 — The HTTP QUERY Method**  
  https://www.rfc-editor.org/rfc/rfc10008

Published June 2026.

The current `methods.sec` must add:

```sec
Method.QUERY
```

and the current HTTP field registry includes:

```text
Accept-Query
```

## 4.8 Incremental forwarding

- **RFC 10036 — Incremental Forwarding of HTTP Messages**  
  https://www.rfc-editor.org/rfc/rfc10036

Published August 2026.

The registered:

```text
Incremental
```

HTTP field is supported by the normal Header/Structured Fields model.

## 4.9 Cookies

Current normative cookie standard:

- **RFC 10025 — Cookies: HTTP State Management Mechanism**  
  https://www.rfc-editor.org/info/rfc10025

RFC 10025 was published July 2026 and obsoletes RFC 6265.

Cookie source files must use RFC 10025 as the primary normative specification.

## 4.10 PATCH

- RFC 5789 — PATCH Method for HTTP  
  https://www.rfc-editor.org/rfc/rfc5789

## 4.11 Web linking

- RFC 8288 — Web Linking  
  https://www.rfc-editor.org/rfc/rfc8288

## 4.12 Authentication

- RFC 7617 — Basic HTTP Authentication  
  https://www.rfc-editor.org/rfc/rfc7617
- RFC 7616 — HTTP Digest Access Authentication  
  https://www.rfc-editor.org/rfc/rfc7616

Generic challenge/credentials semantics are defined by RFC 9110.

## 4.13 HTTPS/TLS

- RFC 8446 — TLS 1.3  
  https://www.rfc-editor.org/rfc/rfc8446
- RFC 9325 / BCP 195 — TLS/DTLS security recommendations  
  https://www.rfc-editor.org/rfc/rfc9325

The canonical Sec TLS package owns actual TLS policy.

## 4.14 IANA registries

Primary registries:

- HTTP Method Registry  
  https://www.iana.org/assignments/http-methods
- HTTP Status Code Registry  
  https://www.iana.org/assignments/http-status-codes
- HTTP Field Name Registry  
  https://www.iana.org/assignments/http-fields
- HTTP Authentication Scheme Registry  
  https://www.iana.org/assignments/http-authschemes
- HTTP Parameters / Content Codings  
  https://www.iana.org/assignments/http-parameters
- HTTP/2 Parameters  
  https://www.iana.org/assignments/http2-parameters
- HTTP/3 Parameters  
  https://www.iana.org/assignments/http3-parameters

The HTTP Field Name Registry was rechecked on 2026-09-07 and reported a last
update date of 2026-08-28.

Registry-backed source files must include a synchronization date.

---

# 5. Source-file standards headers

Every source file must state the standards directly governing it.

Example:

```sec
// Sec Standard Library: net/http
// File: header.sec
//
// Standards:
//   RFC 9110 - HTTP Semantics
//   https://www.rfc-editor.org/rfc/rfc9110
//
//   RFC 9651 - Structured Field Values for HTTP
//   https://www.rfc-editor.org/rfc/rfc9651
//
// Registry:
//   IANA HTTP Field Name Registry
//   https://www.iana.org/assignments/http-fields
//   Registry synchronized: 2026-09-07

module http
```

Protocol-version files must reference their version RFC.

Cookie files must reference RFC 10025.

---

# 6. Package and file manifest

The initial intended package layout is:

```text
stdlib/net/http/
├── std-net-http.md
├── error.sec
├── version.sec
├── methods.sec
├── status.sec
├── header.sec
├── structured.sec
├── body.sec
├── request.sec
├── response.sec
├── auth.sec
├── digest.sec
├── priority.sec
├── redirect.sec
├── cache.sec
├── cookie.sec
├── cookie_parse.sec
├── cookie_jar.sec
├── cookie_security.sec
├── client.sec
├── transport.sec
├── server.sec
├── router.sec
├── http1.sec
├── http2.sec
├── hpack.sec
├── http3.sec
└── qpack.sec
```

Existing migration/removal:

```text
encode.sec       -> responsibility moves to net/url; remove afterward
websockets.sec   -> responsibility moves to net/websocket; remove afterward
```

All source files above use:

```sec
module http
```

There are no public packages:

```text
net/http/http1
net/http/http2
net/http/http3
```

in revision 0.1.

---

# Part I — Errors and protocol identity

# 7. `error.sec`

## 7.1 `HeaderError`

```sec
enum HeaderError error {
    InvalidName,
    InvalidValue,
    InjectionAttempt,
    TooManyFields,
    FieldsTooLarge,
}
```

## 7.2 `BodyError`

```sec
enum BodyError error {
    Closed,
    TooLarge,
    UnexpectedEof,
    LengthMismatch,
    NotReplayable,
    ReadFailed,
    WriteFailed,
}
```

## 7.3 `MessageError`

```sec
enum MessageError error {
    InvalidMethod,
    InvalidStatus,
    InvalidVersion,
    InvalidTarget,
    InvalidFraming,
    InvalidContentLength,
    ConflictingContentLength,
    InvalidTransferEncoding,
    InvalidTrailer,
    HeaderTooLarge,
    BodyTooLarge,
    MalformedMessage,
    UnsupportedProtocol,
}
```

## 7.4 `ClientError`

```sec
union ClientError error {
    Url(error),
    Dns(error),
    Network(ip.NetworkError),
    Tls(error),
    Quic(error),
    Message(MessageError),
    Header(HeaderError),
    Body(BodyError),
    Redirect(RedirectError),
    Cache(CacheError),
    Cookie(CookieJarError),
    ConnectionClosed,
    NoProtocolAvailable,
    ResourceLimitExceeded,
}
```

## 7.5 `ServerError`

```sec
union ServerError error {
    Network(ip.NetworkError),
    Tls(error),
    Quic(error),
    Message(MessageError),
    Handler(error),
    Closed,
    ResourceLimitExceeded,
}
```

Underlying DNS/TLS/QUIC/network causes must not be flattened into diagnostic
strings.

---

# 8. `version.sec`

The current `request.sec` representation:

```sec
type Version struct {
    Major: uint8,
    Minor: uint8,
}
```

is replaced.

## 8.1 `Version`

```sec
enum Version {
    Http10,
    Http11,
    Http2,
    Http3,
}
```

HTTP version support is a real implementation capability, unlike an open
method/status registry.

## 8.2 `Protocols`

```sec
type Protocols struct {
    Http11: bool,
    Http2: bool,
    Http3: bool,
}

impl Protocols {
    static fn ClientDefault() Protocols
    static fn ServerDefault() Protocols
}
```

HTTP/1.0 is accepted for compatibility by the HTTP/1 parser but is not a
normal newly initiated client preference.

---

# 9. `methods.sec`

The existing core design is retained:

```sec
type Method string
```

HTTP method tokens are an open, case-sensitive registry.

## 9.1 Public API

```sec
impl Method {
    static let GET: Method := "GET"
    static let HEAD: Method := "HEAD"
    static let POST: Method := "POST"
    static let PUT: Method := "PUT"
    static let DELETE: Method := "DELETE"
    static let CONNECT: Method := "CONNECT"
    static let OPTIONS: Method := "OPTIONS"
    static let TRACE: Method := "TRACE"
    static let PATCH: Method := "PATCH"
    static let QUERY: Method := "QUERY"

    // Other current IANA-registered methods remain named where practical.

    static fn FromString(
        value: string
    ) Result[Method, MessageError]

    fn ToString() string

    property IsSafe: Option[bool] { get { ... } }
    property IsIdempotent: Option[bool] { get { ... } }
}
```

Unknown syntactically valid methods are preserved exactly.

Known methods return known safe/idempotent semantics; unknown extensions return
`None` rather than guessed behavior.

## 9.2 `PRI` is not a public method

The current `Method.PRI` value is removed.

HTTP/2's:

```text
PRI * HTTP/2.0
```

connection preface belongs privately in `http2.sec`.

## 9.3 Registry sync

`methods.sec` tracks the IANA Method Registry, including RFC 10008 QUERY.

---

# 10. `status.sec`

Retain:

```sec
type Status int range 100..999
```

Known IANA values remain associated static values.

## 10.1 Remove `199` sentinel

The current:

```sec
default 199
Status.Uninitialized
```

must be removed.

Internal not-yet-written status is represented by `Option[Status]` or private
writer state, never by consuming a protocol code point.

## 10.2 Status behavior

```sec
enum StatusClass {
    Informational,
    Success,
    Redirection,
    ClientError,
    ServerError,
    Extension,
}

impl Status {
    // IANA synchronized named values.

    fn ToString() string

    property Class: StatusClass { get { ... } }
    property IsInformational: bool { get { ... } }
    property IsSuccess: bool { get { ... } }
    property IsRedirection: bool { get { ... } }
    property IsClientError: bool { get { ... } }
    property IsServerError: bool { get { ... } }
}
```

## 10.3 Temporary assignments

Status 104 is temporary at this revision's registry snapshot.

The source file must record its temporary status and recheck IANA when the
registration expires or is extended.

---

# Part II — HTTP fields

# 11. `header.sec`

The existing repeated-field collection is retained and hardened.

## 11.1 `HeaderName`

```sec
type HeaderName string

impl HeaderName {
    static fn Parse(
        value: string
    ) Result[HeaderName, HeaderError]

    fn ToString() string
}
```

Rules:

- valid HTTP field-name token;
- lowercase canonical storage;
- case-insensitive semantics;
- no colon/whitespace/control/CR/LF;
- compatible with HTTP/2 and HTTP/3 lowercase wire requirements.

Common IANA names can be static values, including:

```sec
HeaderName.Accept
HeaderName.AcceptEncoding
HeaderName.AcceptQuery
HeaderName.Authorization
HeaderName.CacheControl
HeaderName.ContentDigest
HeaderName.ContentLength
HeaderName.ContentType
HeaderName.Cookie
HeaderName.Host
HeaderName.Incremental
HeaderName.Location
HeaderName.Priority
HeaderName.ReprDigest
HeaderName.SetCookie
HeaderName.Trailer
HeaderName.TransferEncoding
HeaderName.Upgrade
HeaderName.Vary
```

The namespace remains open.

## 11.2 `HeaderValue`

```sec
type HeaderValue string

impl HeaderValue {
    static fn Parse(
        value: string
    ) Result[HeaderValue, HeaderError]

    fn ToString() string
}
```

It rejects at least CR, LF, NUL, and forbidden controls.

### Opaque field-octet observation

RFC 9110 permits legacy opaque `obs-text`.

If Sec `string` cannot losslessly preserve all legal opaque field octets, the
final implementation must use an appropriate byte-string representation rather
than corrupt received fields.

## 11.3 `Header`

```sec
type _HeaderField struct {
    Name: HeaderName,
    Value: HeaderValue,
}

type Header struct {
    _fields: list[_HeaderField],
}

impl Header {
    static fn New() Header

    fn Add(
        name: HeaderName,
        value: HeaderValue
    ) Result[void, CollectionError]

    fn Set(
        name: HeaderName,
        value: HeaderValue
    ) Result[void, CollectionError]

    fn Get(
        name: HeaderName
    ) Option[HeaderValue]

    fn GetAllToArray(
        name: HeaderName
    ) Result[HeaderValue[], CollectionError]

    fn Contains(
        name: HeaderName
    ) bool

    fn Remove(
        name: HeaderName
    ) bool

    property Count: uint { get { ... } }
}
```

The existing allocation-returning `GetAll` becomes `GetAllToArray`.

## 11.4 Repeated fields

Generic Header preserves repeated field occurrences.

It does not generically comma-combine them.

RFC 10025 specifically forbids combining multiple Set-Cookie field lines.

Version-specific encoders validate connection-specific field legality.

---

# 12. `structured.sec`

Implements RFC 9651 Structured Field Values.

Structured parsing is only used for fields whose specifications opt into it.

## 12.1 Types

```sec
type StructuredBareItem union {
    Integer(int64),
    Decimal(decimal),
    String(string),
    Token(string),
    ByteSequence(byte[]),
    Boolean(bool),
    Date(datetime),
    DisplayString(string),
}

type StructuredParameter struct {
    Key: string,
    Value: StructuredBareItem,
}

type StructuredItem struct {
    Value: StructuredBareItem,
    Parameters: StructuredParameter[],
}

type StructuredInnerList struct {
    Items: StructuredItem[],
    Parameters: StructuredParameter[],
}

type StructuredMember union {
    Item(StructuredItem),
    InnerList(StructuredInnerList),
}

type StructuredList struct {
    Members: StructuredMember[],
}

type StructuredDictionaryMember struct {
    Key: string,
    Value: StructuredMember,
}

type StructuredDictionary struct {
    Members: StructuredDictionaryMember[],
}
```

The exact canonical Sec decimal/time types are mechanically updated when their
stdlib books are locked.

## 12.2 Parsing

```sec
fn ParseStructuredItem(
    value: HeaderValue
) Result[StructuredItem, StructuredFieldError]

fn ParseStructuredList(
    value: HeaderValue
) Result[StructuredList, StructuredFieldError]

fn ParseStructuredDictionary(
    value: HeaderValue
) Result[StructuredDictionary, StructuredFieldError]
```

RFC 9651 strict parsing is preserved; the implementation must not add tolerant
behavior that changes interoperability.

---

# Part III — Bodies, requests, responses

# 13. `body.sec`

```sec
@noCopy
type Body struct {
    // Private streaming source/decoder state.
}

impl Body {
    static fn Empty() Body

    static fn FromBytes(
        data: <- byte[]
    ) Body

    static fn FromString(
        text: string
    ) Result[Body, BodyError]

    property Length: Option[uint64] { get { ... } }
    property Replayable: bool { get { ... } }

    fn Read(
        buffer: ref mut byte[]
    ) Result[uint, BodyError]

    fn ReadAllToArray(
        limit: uint64
    ) Result[byte[], BodyError]

    fn Close()
        Result[void, BodyError]
}
```

For a non-empty read buffer, `Ok(0)` means EOF.

HTTP/1 chunks and H2/H3 DATA frames are transport details, not Body API
boundaries.

`ReadAllToArray` always requires a limit.

A body can be non-replayable, which affects redirects and retries.

---

# 14. `request.sec`

## 14.1 `RequestTarget`

```sec
type RequestTarget union {
    Origin(string),
    Absolute(url.URL),
    Authority(string),
    Asterisk,
}
```

This models the HTTP request-target forms where applicable.

## 14.2 `Request`

```sec
@noCopy
type Request struct {
    Method: Method,
    Target: RequestTarget,
    Header: Header,
    Body: Body,
}

impl Request {
    static fn New(
        method: Method,
        url: url.URL
    ) Result[Request, MessageError]

    static fn Get(
        url: url.URL
    ) Result[Request, MessageError]

    static fn Head(
        url: url.URL
    ) Result[Request, MessageError]

    static fn Post(
        url: url.URL,
        body: <- Body
    ) Result[Request, MessageError]

    fn SetBody(
        body: <- Body
    )

    property Url: Option[url.URL] { get { ... } }
}
```

The exact `url.URL` spelling is synchronized with the future `std-net-url.md`.

## 14.3 Server metadata

```sec
type ConnectionInfo struct {
    LocalEndpoint: ip.Endpoint,
    RemoteEndpoint: ip.Endpoint,
    Version: Version,
    Secure: bool,
}

@noCopy
type ServerRequest struct {
    Message: Request,
    Connection: ConnectionInfo,
}
```

Transport metadata remains outside the protocol `Request` value itself.

---

# 15. `response.sec`

## 15.1 `Response`

```sec
@noCopy
type Response struct {
    Status: Status,
    Header: Header,
    Body: Body,
    Version: Version,
}

impl Response {
    static fn New(
        status: Status
    ) Response

    fn Close()
        Result[void, BodyError]
}
```

## 15.2 `ResponseWriter`

```sec
@noCopy
type ResponseWriter struct {
    // Private active server stream state.
}

impl ResponseWriter {
    property Header: ref mut Header { get { ... } }

    fn WriteStatus(
        status: Status
    ) Result[void, ServerError]

    fn Write(
        data: ref byte[]
    ) Result[uint, ServerError]

    fn WriteAll(
        data: ref byte[]
    ) Result[void, ServerError]

    fn Flush()
        Result[void, ServerError]

    fn Finish()
        Result[void, ServerError]
}
```

First body write commits `Status.Ok` if no explicit status has been committed.

This is server writer state, not a fake `Status.Uninitialized`.

After commit, ordinary response headers can no longer be changed.

---

# Part IV — Authentication, digest, priority

# 16. `auth.sec`

HTTP authentication schemes are an open IANA namespace:

```sec
type AuthScheme string
```

Known values may include:

```sec
AuthScheme.Basic
AuthScheme.Digest
AuthScheme.Bearer
```

Unknown schemes remain representable.

Generic challenge:

```sec
type AuthChallenge struct {
    Scheme: AuthScheme,
    Parameters: HeaderValue,
}
```

Basic helper:

```sec
fn BasicAuthorization(
    username: string,
    password: string
) Result[HeaderValue, AuthError]
```

It uses the canonical Base64 implementation.

RFC 7616 Digest support must use canonical crypto primitives.

Credentials are sensitive and are not included in ordinary debug output by
default.

---

# 17. `digest.sec`

Implements RFC 9530:

```text
Content-Digest
Repr-Digest
Want-Content-Digest
Want-Repr-Digest
```

It uses RFC 9651 Structured Fields and canonical crypto hashing.

Unknown registered digest algorithms remain parseable even if the local crypto
backend cannot verify them.

Digest mismatch is explicit failure.

---

# 18. `priority.sec`

Implements RFC 9218.

```sec
type Priority struct {
    Urgency: uint8,
    Incremental: bool,
}
```

Urgency range and defaults follow RFC 9218.

Priority maps through:

- the `Priority` Structured Field;
- HTTP/2 PRIORITY_UPDATE;
- HTTP/3 PRIORITY_UPDATE.

The public API does not expose the old HTTP/2 dependency-tree model.

---

# Part V — Redirect and cache

# 19. `redirect.sec`

```sec
enum RedirectError error {
    TooManyRedirects,
    InvalidLocation,
    BodyNotReplayable,
    PolicyRejected,
}

type RedirectPolicy struct {
    MaxRedirects: uint8,
    AllowCrossOrigin: bool,
    PreserveSensitiveHeaders: bool,
}

impl RedirectPolicy {
    static fn Default() RedirectPolicy
}
```

Redirect semantics follow RFC 9110.

307/308 preserve method semantics.

Automatic redirects must respect body replayability.

Sensitive authentication/cookie-like headers are not leaked across unsafe
origin changes.

The current simple boolean policy may later become an explicit allow-list
before the API is declared stable.

---

# 20. `cache.sec`

Implements RFC 9111.

Generic client caching is disabled by default.

```sec
enum CacheMode {
    Disabled,
    Memory,
}

type CacheOptions struct {
    Mode: CacheMode,
    MaxEntries: uint,
    MaxBytes: uint64,
}

enum CacheError error {
    InvalidMetadata,
    EntryTooLarge,
    StorageFailed,
    RevalidationFailed,
}
```

The bounded cache handles at least:

- freshness;
- Age;
- Date;
- Cache-Control;
- Expires;
- Vary;
- ETag;
- Last-Modified;
- conditional revalidation;
- invalidation.

Cache miss is not an error.

---

# Part VI — Cookies

# 21. `cookie.sec`

The existing `Cookie` / `SetCookie` split is retained.

Primary standard:

https://www.rfc-editor.org/info/rfc10025

## 21.1 `SameSite`

```sec
enum SameSite {
    Strict,
    Lax,
    None,
}
```

Absence uses `Option[SameSite]`.

## 21.2 `Cookie`

```sec
type Cookie struct {
    Name: string,
    Value: string,
}

impl Cookie {
    init(
        name: string,
        value: string
    )

    fn Validate()
        Result[void, CookieError]

    fn ToString()
        Result[string, CookieError]
}
```

Cookie models one request cookie pair only.

## 21.3 `SetCookie`

Retain:

```sec
type SetCookie struct {
    Pair: Cookie,
    Path: Option[string],
    Domain: Option[string],
    Expires: Option[datetime],
    MaxAge: Option[int64],
    Secure: bool,
    HttpOnly: bool,
    SameSite: Option[SameSite],
    Partitioned: bool,
    Extensions: Option[string[]],
}
```

Canonical behavior:

```sec
impl SetCookie {
    init(
        name: string,
        value: string
    )

    property Name: string { get { ... } }
    property Value: string { get { ... } }

    fn Validate()
        Result[void, CookieError]

    fn ToString()
        Result[string, CookieError]

    static fn Parse(
        value: string
    ) Result[SetCookie, CookieParseError]
}
```

Unknown extension attributes are preserved.

Temporary `NotImplemented` error variants are not stable protocol errors and
must disappear as implementation lands.

## 21.4 Partitioned

`Partitioned` remains a contemporary extension and is not falsely labeled as
RFC 10025 core.

Current external source:

https://github.com/privacycg/CHIPS

Its standards status must be rechecked separately from RFC 10025.

---

# 22. `cookie_parse.sec`

Owns RFC 10025 parsing.

Existing:

```sec
SetCookie.Parse(...)
```

is retained.

Request-cookie helpers:

```sec
fn ParseCookieHeader(
    value: HeaderValue
) Result[Cookie[], CookieParseError]

fn ParseCookiesFromHeaders(
    header: ref Header
) Result[Cookie[], CookieParseError]
```

HTTP/2/3 can split Cookie into multiple field lines, so the header-level helper
processes all occurrences.

Cookie values are never automatically URL-decoded.

---

# 23. `cookie_jar.sec`

The existing storage direction is retained and completed.

```sec
@noCopy
type CookieJar struct {
    // Private bounded stored-cookie set.
}
```

Context:

```sec
type CookieRequestContext struct {
    Url: url.URL,
    Method: Method,
    TopLevelSite: Option[url.URL],
    IsTopLevelNavigation: bool,
}
```

Canonical API:

```sec
impl CookieJar {
    static fn New(
        options: CookieJarOptions
    ) Result[CookieJar, CookieJarError]

    fn Store(
        origin: url.URL,
        cookie: SetCookie
    ) Result[void, CookieJarError]

    fn CookiesForToArray(
        request: ref CookieRequestContext
    ) Result[Cookie[], CookieJarError]

    fn RemoveExpired()
    fn Clear()

    fn Close()
        Result[void, CookieJarError]
}
```

Cookie-domain handling must reject public-suffix supercookies.

Reference policy source:

https://publicsuffix.org/list/

The exact PSL update/provisioning mechanism remains open.

---

# 24. `cookie_security.sec`

The existing signed/private cookie work is retained only as **provisional Sec
application convenience**.

It is not part of HTTP/RFC 10025 protocol conformance.

Current concepts:

```sec
CookieKey
SignedCookieCodec
PrivateCookieCodec
```

remain unstable until:

- canonical crypto key types exist;
- a versioned application-cookie envelope is specified;
- key rotation semantics are complete.

No home-grown crypto is permitted.

This file is not a blocker for declaring the core HTTP protocol/cookie support
implemented.

---

# Part VII — Client and transport

# 25. `client.sec`

The current global stubs:

```sec
fn Get(url: string) void
fn Post(url: string) void
fn Head(url: string) void
```

are replaced by an owned reusable client.

## 25.1 `ClientOptions`

```sec
type ClientOptions struct {
    Protocols: Protocols,
    Redirects: RedirectPolicy,
    Cache: CacheOptions,
    MaxResponseHeaderBytes: uint,
    MaxResponseBodyBytes: Option[uint64],
}
```

## 25.2 `Client`

```sec
@noCopy
type Client struct {
    // Private resolver, connection pools, optional cookie jar/cache,
    // HTTP/1.1/2/3 transports, TLS/QUIC state.
}

impl Client {
    static fn New(
        options: ClientOptions
    ) Result[Client, ClientError]

    static fn WithResolver(
        resolver: <- dns.Resolver,
        options: ClientOptions
    ) Result[Client, ClientError]

    fn SetCookieJar(
        jar: <- CookieJar
    ) Result[void, ClientError]

    fn Do(
        request: <- Request
    ) Result[Response, ClientError]

    fn Get(
        url: url.URL
    ) Result[Response, ClientError]

    fn Head(
        url: url.URL
    ) Result[Response, ClientError]

    fn Post(
        url: url.URL,
        body: <- Body
    ) Result[Response, ClientError]

    fn Close()
        Result[void, ClientError]
}
```

`Client`:

- resolves through `net/dns`;
- connects through `net/ip`;
- uses canonical TLS for HTTPS;
- uses `net/quic` for HTTP/3;
- pools eligible connections;
- never crosses origin/security boundaries improperly;
- has no cookie persistence unless a jar is configured;
- has no reusable HTTP cache unless cache mode is enabled.

Returned `Response` owns its body.

---

# 26. `transport.sec`

Internal version-independent dispatch:

```sec
interface _ClientTransport {
    fn RoundTrip(
        request: <- Request
    ) Result[Response, ClientError]
}
```

This may remain private implementation detail.

Transport mapping:

```text
HTTP/1.1 -> TCP or TCP+TLS
HTTP/2   -> TCP+TLS (ordinary h2)
HTTP/3   -> QUIC
```

---

# Part VIII — Server and router

# 27. `server.sec`

## 27.1 `ServerOptions`

```sec
type ServerOptions struct {
    Protocols: Protocols,
    MaxRequestHeaderBytes: uint,
    MaxRequestBodyBytes: Option[uint64],
    MaxConcurrentRequests: uint,
}
```

All defaults are bounded.

## 27.2 `Handler`

```sec
interface Handler {
    fn Handle(
        request: <- ServerRequest,
        response: ref mut ResponseWriter
    ) Result[void, error]
}
```

## 27.3 `Server`

```sec
@noCopy
type Server struct {
    // Private listeners/transports/handler/connections.
}

impl Server {
    static fn Bind(
        endpoint: ip.Endpoint,
        handler: <- Handler,
        options: ServerOptions
    ) Result[Server, ServerError]

    fn Serve()
        Result[void, ServerError]

    fn Close()
        Result[void, ServerError]
}
```

TLS/HTTP3 constructors/configuration are synchronized after canonical TLS/QUIC
books are locked.

HTTP/1 request framing must reject ambiguity that can cause request smuggling.

A later graceful `Shutdown` is distinct from abrupt `Close`.

---

# 28. `router.sec`

Router is a small stdlib convenience, not a web framework.

```sec
enum RouterError error {
    InvalidPattern,
    DuplicateRoute,
    ConflictingRoute,
}

type RouteParam struct {
    Name: string,
    Value: string,
}

type RouteParams struct {
    Values: RouteParam[],
}

@noCopy
type Router struct {
    // Private route table and owned handlers.
}

impl Router {
    static fn New() Router

    fn Handle(
        method: Method,
        pattern: string,
        handler: <- Handler
    ) Result[void, RouterError]

    fn HandleAny(
        pattern: string,
        handler: <- Handler
    ) Result[void, RouterError]
}
```

Initial pattern grammar is intentionally small:

```text
/static/path
/users/{id}
```

Patterns match URL paths, not raw query text.

`Router` satisfies `Handler`.

---

# Part IX — HTTP/1.1

# 29. `http1.sec`

Primary standards:

- RFC 9110
- RFC 9112

Owns:

- HTTP/1.0 compatibility;
- HTTP/1.1 start lines;
- header framing;
- Content-Length;
- chunked coding;
- trailers;
- persistence;
- Host;
- Connection;
- Upgrade;
- parser/generator security.

The parser is incremental and bounded.

It must follow RFC 9112 framing rules exactly enough to reject ambiguous
Content-Length / Transfer-Encoding combinations and other smuggling vectors.

Chunk boundaries are hidden from ordinary Body consumers.

WebSocket upgrade mechanics remain HTTP; WebSocket frames/protocol do not.

---

# Part X — HTTP/2

# 30. `http2.sec`

Standards:

- RFC 9113
- RFC 9218
- RFC 7541

Owns:

- client/server connection preface;
- frame parser/encoder;
- SETTINGS;
- HEADERS;
- DATA;
- CONTINUATION;
- stream state;
- flow control;
- RST_STREAM;
- GOAWAY;
- PING;
- PRIORITY_UPDATE;
- pseudo-field conversion;
- HTTP semantic conversion.

HTTP/2 pseudo-fields are private transport representation.

`HeaderName.Parse` does not let applications inject arbitrary names beginning
with `:`.

Connection-specific HTTP/1 fields forbidden in H2 are rejected.

High-level server push is not exposed in revision 0.1.

---

# 31. `hpack.sec`

Implements RFC 7541.

Must bound:

- dynamic table;
- decoded header-list size;
- integer/string decoding;
- Huffman decode work/storage.

Sensitive fields use appropriate indexing policy.

HPACK table state is connection state, not part of `Header`.

---

# Part XI — HTTP/3

# 32. `http3.sec`

Standards:

- RFC 9114
- RFC 9204
- RFC 9218
- QUIC RFCs through `net/quic`

Owns:

- H3 control streams;
- request streams;
- frames;
- SETTINGS;
- GOAWAY;
- priorities;
- pseudo-field conversion;
- request cancellation;
- semantic mapping.

It does not implement QUIC transport.

HTTP/3 discovery can use Alt-Svc and HTTPS/SVCB information; exact endpoint
discovery is expanded after `net/url`, DNS HTTPS RR, and `net/quic` are locked.

---

# 33. `qpack.sec`

Implements RFC 9204.

Must bound:

- dynamic table capacity;
- blocked streams;
- encoded/decoded field size;
- integer/string/Huffman operations.

Sensitive fields use correct never-index behavior.

QPACK state remains private to HTTP/3 connections.

---

# Part XII — Current 2026 changes

# 34. RFC 10008 QUERY

`methods.sec` adds:

```sec
static let QUERY: Method := "QUERY"
```

QUERY is safe and idempotent **and carries request content**.

Generic HTTP logic must therefore not assume:

```text
safe method => body forbidden
```

`HeaderName.AcceptQuery` is current registry-backed functionality.

---

# 35. RFC 10036 Incremental

The current field registry contains:

```text
Incremental
```

Generic Header preserves it.

Typed parsing uses Structured Fields according to RFC 10036.

Older HTTP implementation code must not silently discard the field because it
did not exist when RFC 9110 was published.

---

# Part XIII — Negotiation and retries

# 36. Protocol negotiation

For HTTPS/TCP:

```text
h2
http/1.1
```

are negotiated through canonical TLS ALPN.

HTTP/3 uses QUIC/H3 and is not selected through a TCP TLS connection.

Cleartext h2c is not required by revision 0.1.

---

# 37. Retry/cancellation

Potential cancellation points include DNS, connect, TLS/QUIC handshake, body
write, response wait, and body read.

No HTTP-specific cancellation token is introduced.

Automatic retry must account for:

- whether the request may have committed;
- Method idempotence;
- body replayability;
- transport semantics.

Unknown extension methods have unknown retry safety unless explicitly told by
a future caller policy.

A non-replayable body is never silently resent after an uncertain commit.

---

# Part XIV — Security/resource limits

# 38. Header/body limits

Client/server apply limits while parsing/streaming, before attacker-controlled
sizes cause large allocation.

For H2/H3, limits apply to **decoded** fields as well as compressed bytes.

For bodies, configured maximums are enforced while reading.

---

# 39. Smuggling/splitting

At minimum defend against:

- CRLF injection;
- conflicting Content-Length;
- Transfer-Encoding ambiguity;
- whitespace parsing divergence;
- malformed Host/authority;
- invalid pseudo-field ordering;
- duplicate pseudo-fields;
- forbidden connection fields.

RFC 9112/9113/9114 security requirements are normative.

---

# Part XV — Ownership

# 40. Owning resources

At minimum:

```sec
@noCopy Body
@noCopy Request
@noCopy Response
@noCopy ResponseWriter
@noCopy Client
@noCopy Server
@noCopy Router
@noCopy CookieJar
```

`Client.Do(<-request)` consumes the request because its body may be an owned
stream.

Returned `Response` owns body/stream lifecycle.

---

# Part XVI — Platform capability

# 41. Pure facilities

Usable without active networking:

- Method;
- Status;
- Header;
- Structured Fields;
- cookie parsing/serialization;
- semantic messages;
- version codecs where dependencies exist.

## 41.1 Active capabilities

HTTP/1.1 requires TCP.

HTTPS additionally requires TLS.

HTTP/2 normally requires TCP+TLS+ALPN.

HTTP/3 requires canonical `net/quic`.

Targets missing capability reject unsupported active use rather than silently
changing security/protocol semantics.

---

# Part XVII — Repository migration

# 42. Current files

The HTTP directory rechecked on 2026-09-07 contains:

```text
client.sec
cookie.sec
cookie_jar.sec
cookie_parse.sec
cookie_security.sec
encode.sec
header.sec
methods.sec
request.sec
response.sec
router.sec
server.sec
status.sec
websockets.sec
```

## 42.1 Existing maturity

Substantial/usable design exists in:

```text
cookie.sec
cookie_jar.sec
cookie_parse.sec
cookie_security.sec
header.sec
methods.sec
request.sec
status.sec
```

Mostly empty/stub:

```text
client.sec
response.sec
router.sec
server.sec
encode.sec
websockets.sec
```

---

# 43. Migration map

## 43.1 Keep/expand

```text
client.sec
cookie.sec
cookie_jar.sec
cookie_parse.sec
cookie_security.sec
header.sec
methods.sec
request.sec
response.sec
router.sec
server.sec
status.sec
```

## 43.2 Add

```text
std-net-http.md
error.sec
version.sec
structured.sec
body.sec
auth.sec
digest.sec
priority.sec
redirect.sec
cache.sec
transport.sec
http1.sec
http2.sec
hpack.sec
http3.sec
qpack.sec
```

## 43.3 Remove after migration

```text
encode.sec
websockets.sec
```

### `encode.sec`

URL percent encoding moves to `net/url`.

### `websockets.sec`

Full WebSocket protocol moves to `net/websocket`.

---

# 44. Existing-file corrections

## `methods.sec`

- keep `type Method string`;
- add `QUERY`;
- sync IANA;
- remove public `PRI`;
- validate tokens;
- add optional safe/idempotent semantics.

## `status.sec`

- keep open 100..999 range;
- remove `default 199`;
- remove `Uninitialized`;
- recheck temporary 104;
- sync IANA.

## `header.sec`

- preserve repeated values/order;
- add validated `HeaderName` / `HeaderValue`;
- rename allocation-returning `GetAll` to `GetAllToArray`;
- never combine Set-Cookie generically.

## `request.sec`

- move Version to `version.sec`;
- replace major/minor struct;
- integrate RequestTarget;
- integrate streaming Body;
- integrate `net/url`.

## `response.sec`

Expand empty Response/ResponseWriter.

## `client.sec`

Replace void global stubs with reusable Client.

## Cookie files

- RFC 10025 is normative;
- remove stable `NotImplemented` error variants;
- use canonical `net/url`;
- preserve extensions;
- keep Partitioned separately sourced;
- keep cookie-security envelope provisional.

---

# Part XVIII — Tests

# 45. Method/status tests

At minimum:

- GET/HEAD/POST/etc.;
- PATCH;
- QUERY;
- extension method preservation/case;
- invalid token;
- safe/idempotent metadata;
- no Method.PRI;
- IANA sync;
- known status values;
- unknown status within range;
- no status sentinel;
- temporary IANA assignment handling.

---

# 46. Header/structured tests

At minimum:

- case-insensitive field names;
- lowercase canonical names;
- repeated values;
- Set/Remove;
- CR/LF injection rejection;
- Set-Cookie repetition;
- H2/H3 forbidden fields;
- RFC 9651 test vectors;
- strict Structured Fields parser;
- allocation failure transactional behavior.

---

# 47. Body/message tests

At minimum:

- empty/bytes/string bodies;
- partial reads;
- EOF;
- bounded ReadAllToArray;
- replayable/non-replayable;
- GET;
- POST;
- QUERY with content;
- CONNECT authority;
- OPTIONS `*`;
- response close;
- response writer commit.

---

# 48. HTTP/1 tests

At minimum:

- HTTP/1.0 compatibility;
- HTTP/1.1;
- keepalive;
- Content-Length;
- chunked;
- trailers;
- malformed chunk;
- conflicting Content-Length;
- Transfer-Encoding ambiguity;
- Host;
- Upgrade;
- smuggling corpus;
- parsing limits.

---

# 49. HTTP/2 tests

At minimum:

- preface;
- SETTINGS;
- HEADERS/DATA;
- CONTINUATION;
- flow control;
- reset/goaway/ping;
- pseudo-fields;
- invalid connection fields;
- concurrent streams;
- cancellation;
- HPACK RFC vectors;
- decompressed header limits.

---

# 50. HTTP/3 tests

At minimum:

- QUIC/H3 negotiation;
- control streams;
- SETTINGS;
- HEADERS/DATA;
- cancellation;
- GOAWAY;
- pseudo-fields;
- multiple request streams;
- QPACK RFC vectors;
- blocked-stream limits;
- QUIC v1/v2 when `net/quic` supports them.

---

# 51. Cookie tests

Use RFC 10025 examples/vectors where practical.

At minimum:

- Cookie/SetCookie validation;
- parsing/serialization;
- Expires/Max-Age/Domain/Path;
- Secure/HttpOnly/SameSite;
- prefixes;
- multiple Set-Cookie;
- multiple Cookie fields;
- unknown extension preservation;
- no implicit URL decoding;
- jar expiry/domain/path/order;
- public suffix;
- SameSite context;
- Partitioned tests separately marked as extension.

---

# 52. Client/server tests

At minimum:

- HTTP/HTTPS;
- DNS/network/TLS failures;
- H1/H2/H3;
- pooling;
- redirects;
- sensitive header handling;
- non-replayable body;
- cookies off by default;
- cookie jar;
- cache off by default;
- cache revalidation;
- cancellation at all phases;
- bind/server/handler/router;
- header/body limits;
- concurrent requests.

---

# Part XIX — AI/repository instructions

# 53. Requirements

An implementation task must:

1. treat this book as normative;
2. inspect every current HTTP file before changing it;
3. preserve useful working code;
4. add standards/registry URLs to source files;
5. use RFC 9110–9114 core semantics;
6. use RFC 9651, RFC 9530, RFC 10008, RFC 10025, and RFC 10036 where applicable;
7. keep open registries open;
8. remove public Method.PRI;
9. remove Status.Uninitialized/default 199;
10. harden headers against injection;
11. preserve repeated Set-Cookie;
12. implement streaming Body;
13. replace client stubs with @noCopy Client;
14. implement Response/ResponseWriter;
15. migrate percent encoding to net/url;
16. remove encode.sec after migration;
17. migrate WebSocket responsibility to net/websocket;
18. remove websockets.sec after migration;
19. reuse net/dns, net/ip, TLS, net/quic;
20. implement smuggling-safe HTTP/1.1;
21. implement HPACK/HTTP2;
22. implement QPACK/HTTP3;
23. bound attacker-controlled state;
24. keep cookie_security nonstandard/provisional until its envelope is specified;
25. add required tests;
26. update `implementation-status-std-net-http.yaml`;
27. reconcile canonical status without overwriting stronger/newer evidence.

---

# 54. Recommended implementation order

1. book/status/source headers;
2. error;
3. method/status synchronization;
4. Header hardening;
5. Version;
6. Body;
7. Request;
8. Response/ResponseWriter;
9. cookie parse/serialize;
10. CookieJar;
11. Structured Fields;
12. HTTP/1.1;
13. Client;
14. Server;
15. Router;
16. Redirect;
17. Cache;
18. Auth/Digest/Priority;
19. HPACK;
20. HTTP/2;
21. QPACK;
22. HTTP/3;
23. remove migrated encode/websocket stubs;
24. revisit cookie-security after crypto is locked.

---

# Part XX — Open observations

# 55. Open items

- final `net/url` type names;
- final TLS package path;
- final `net/quic` API;
- HeaderValue opaque-byte representation;
- generic `io.Reader`/`io.Writer` Body integration;
- handler ownership syntax;
- graceful server shutdown;
- advanced router grammar;
- proxy API;
- CONNECT tunnel API;
- extended CONNECT;
- h2c;
- server push;
- Alt-Svc/HTTPS RR H3 discovery;
- address racing;
- persistent HTTP cache;
- PSL update strategy;
- canonical site/partition key;
- Partitioned cookie standards state;
- signed/private cookie envelope;
- MIME/media-type ownership;
- multipart ownership;
- message signatures and additional modern HTTP extensions.

---

# 56. Decisions established by revision 0.1

Revision 0.1 establishes:

- canonical package `net/http`;
- one semantic API across H1/H2/H3;
- RFC 9110–9114 core baseline;
- open Method and Status domains;
- RFC 10008 QUERY support;
- no public Method.PRI;
- no Status protocol sentinel;
- safe case-insensitive Header model;
- repeated fields preserved;
- Set-Cookie never generically combined;
- RFC 9651 Structured Fields;
- RFC 9530 Digest Fields;
- RFC 10036 Incremental compatibility;
- streaming owned Body;
- semantic request targets;
- owned Response/ResponseWriter;
- reusable @noCopy Client;
- caching disabled by default;
- cookie persistence opt-in;
- RFC 10025 cookie baseline;
- Cookie vs SetCookie distinction retained;
- unknown Set-Cookie extensions retained;
- Partitioned remains separately sourced extension behavior;
- cookie-security is nonstandard/provisional;
- HTTP/1 framing is smuggling-safe;
- HTTP/2 uses HPACK;
- HTTP/3 uses QPACK over canonical QUIC;
- DNS/IP/TLS/QUIC responsibilities are reused rather than duplicated;
- URL percent encoding leaves HTTP;
- WebSocket leaves HTTP;
- cancellation uses Sec's common execution model;
- attacker-controlled resource state is bounded.

---

**Document revision:** 0.1  
**Last updated:** 2026-09-07
