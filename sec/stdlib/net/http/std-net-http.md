# Sec Standard Library — `net/http`

- **Status:** Draft — normative stdlib implementation specification
- **Created:** 2026-09-07
- **Last updated:** 2026-09-08
- **Document revision:** 0.2
- **Sec language version:** 0.1
- **Canonical path:** `stdlib/net/http/std-net-http.md`
- **Repository path:** `sec/stdlib/net/http/std-net-http.md`
- **Parent specification:** `stdlib/net/std-net.md`
- **Implementation-status fragment:** `implementation-status-std-net-http.yaml`
- **Repository content rechecked:** 2026-09-08
- **Standards and IANA registries rechecked:** 2026-09-08

---

# 1. Purpose and authority

This book is the normative implementation specification for Sec's `net/http`
standard-library package. A human implementer must be able to derive the complete
public surface and the required observable behavior from this book without having
to infer missing members from examples or comments.

The older broad stdlib rulebook is not authoritative for HTTP where it conflicts
with this book. Existing source files are implementation evidence and may be
reorganized, corrected, or replaced when they conflict with this specification.

The package is imported as:

```sec
import "net/http"
```

and is referenced as `http` in source code.

## 1.1 Completeness rule

For revision 0.2, every public type, associated immutable, property, function,
method, constructor, destructor, and interface owned by `net/http` is listed in
this book. An implementation may add private declarations using Sec visibility
rules, but it must not silently add another public API and still claim conformance
to this revision.

Phrases such as "may include", "and similar", "etc." and omitted public members
are forbidden in normative API sections. Registry-backed constant sets state
whether they are exhaustive registry mirrors or an explicitly bounded convenience
subset.

## 1.2 Signature blocks

Normative API blocks below describe Sec declaration shapes. Function bodies are
implementation work and are therefore not reproduced in the rulebook. Their
absence is not an omitted API member. Constructors are always written as
`init(...)`; `New()` is not a Sec constructor. Destructors are `free()`.

Types are not overloadable: one identifier denotes one type in the module.
Functions, methods, and constructors may have overloads where Sec overload
resolution permits them.

## 1.3 Why representation choices are documented

Every public enum, struct, union, nominal primitive type, and error union in this
book has a **Representation rationale**. It is normative design guidance for the
implementer and for AI validation:

- **enum** — a closed semantic set whose alternatives are controlled by this API;
- **struct** — one value is composed of several simultaneously meaningful fields;
- **union** — exactly one of several payload shapes is present at a time;
- **nominal primitive type** — the wire/value namespace is open but still deserves
  type identity and associated behavior;
- **enum error** — a closed set of errors that carry no payload;
- **`type X union error`** — a closed error family where one or more alternatives
  must preserve a payload/cause.

---

# 2. Documentation contract for source and LSP

Public declarations in `net/http` must use structured documentation comments that
can be consumed by the Sec LSP for hover and generated documentation.

The canonical public-function comment shape is:

```sec
/**
 * One-sentence summary in present tense.
 *
 * Description:
 *   Additional semantic detail when the summary is not sufficient.
 *
 * Parameters:
 *   - name: Meaning, accepted domain, and ownership where relevant.
 *
 * Returns:
 *   Exact success meaning. State whether returned storage is newly allocated,
 *   borrowed, owned, or derived from the receiver where relevant.
 *
 * Errors:
 *   - ErrorType.Member: Exact condition that produces the error.
 *
 * Ownership:
 *   State borrows, moves, retained resources, and post-call ownership.
 *
 * Standards:
 *   - RFC NNNN section/title when directly applicable.
 */
```

Rules:

1. The first line is suitable as compact hover text.
2. `Parameters:` names every parameter exactly once.
3. `Returns:` is mandatory for non-`void` functions and fallible functions.
4. `Errors:` is mandatory for `Result`-returning APIs and enumerates all errors
   this API may directly return at its abstraction level.
5. `Ownership:` is mandatory when a value is borrowed, consumed with `<-`, owns a
   resource, returns newly allocated storage, or changes ownership state.
6. `Standards:` is mandatory when the declaration directly represents a standard
   or registry concept.
7. Public constants receive a concise doc comment when the value has special
   protocol status such as temporary, obsolete, deprecated, reserved, or unsafe.
8. A comment must describe the declared API, not planned functionality.
9. TODO text is forbidden in public API comments in a conforming implementation.

---

# 3. Package architecture

`net/http` is one semantic package across HTTP/1.1, HTTP/2, and HTTP/3. Public
request, response, status, method, header, authentication, cookie, priority, and
cache semantics do not split into separate packages by wire version.

Version-specific parsers, encoders, compression state, and stream state remain
private implementation files within the same `http` module.

Conceptual dependencies are:

```text
net/url
net/dns
net/ip
net/quic        # HTTP/3 only
TLS package     # HTTPS / h2 over TLS / QUIC security
encoding/base64 # Basic auth and Structured Fields where applicable
crypto          # digest verification when implemented
```

`encode.sec` is removed after URL percent-encoding responsibility moves to
`net/url`. `websockets.sec` is removed after WebSocket responsibility moves to
`net/websocket`. `cookie_security.sec` is not part of this revision's public HTTP
API; application-specific encrypted cookie envelopes belong in a future security
facility rather than in HTTP protocol semantics.

---

# 4. Standards and registries

The implementation follows, as applicable:

- RFC 9110 / STD 97 — HTTP Semantics — https://www.rfc-editor.org/rfc/rfc9110
- RFC 9111 / STD 98 — HTTP Caching — https://www.rfc-editor.org/rfc/rfc9111
- RFC 9112 / STD 99 — HTTP/1.1 — https://www.rfc-editor.org/rfc/rfc9112
- RFC 9113 — HTTP/2 — https://www.rfc-editor.org/rfc/rfc9113
- RFC 9114 — HTTP/3 — https://www.rfc-editor.org/rfc/rfc9114
- RFC 7541 — HPACK — https://www.rfc-editor.org/rfc/rfc7541
- RFC 9204 — QPACK — https://www.rfc-editor.org/rfc/rfc9204
- RFC 9218 — Extensible Prioritization Scheme for HTTP — https://www.rfc-editor.org/rfc/rfc9218
- RFC 9651 — Structured Field Values for HTTP — https://www.rfc-editor.org/rfc/rfc9651
- RFC 9530 — Digest Fields — https://www.rfc-editor.org/rfc/rfc9530
- RFC 5789 — PATCH — https://www.rfc-editor.org/rfc/rfc5789
- RFC 7617 — Basic HTTP Authentication — https://www.rfc-editor.org/rfc/rfc7617
- RFC 6750 — OAuth 2.0 Bearer Token Usage — https://www.rfc-editor.org/rfc/rfc6750
- RFC 10008 — HTTP QUERY Method — https://www.rfc-editor.org/rfc/rfc10008
- RFC 10025 — Cookies: HTTP State Management Mechanism — https://www.rfc-editor.org/info/rfc10025

Registry sources:

- IANA HTTP Method Registry — https://www.iana.org/assignments/http-methods
- IANA HTTP Status Code Registry — https://www.iana.org/assignments/http-status-codes
- IANA HTTP Field Name Registry — https://www.iana.org/assignments/http-fields
- IANA HTTP Authentication Scheme Registry — https://www.iana.org/assignments/http-authschemes
- IANA HTTP Parameters — https://www.iana.org/assignments/http-parameters
- IANA HTTP/2 Parameters — https://www.iana.org/assignments/http2-parameters
- IANA HTTP/3 Parameters — https://www.iana.org/assignments/http3-parameters

Registry-backed files record a synchronization date in their source header.

---

# 5. Canonical file manifest

```text
stdlib/net/http/
├── std-net-http.md
├── results.sec
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
├── structured_fields.sec
├── redirect.sec
├── cache.sec
├── cookie.sec
├── cookie_parse.sec
├── cookie_jar.sec
├── client.sec
├── transport.sec
├── server.sec
├── router.sec
├── http1.sec
├── hpack.sec
├── http2.sec
├── qpack.sec
└── http3.sec
```

All `.sec` files use:

```sec
module http
```

No public packages named `net/http/http1`, `net/http/http2`, or `net/http/http3`
exist in this revision.

---

# 6. Required source-file headers

Every file must carry a header with file identity, purpose, standards, rulebook,
and registry synchronization where applicable. The following headers are
normative for revision 2; wording may be wrapped but the information must not be
omitted.

## 6.1 `results.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - HTTP error and result types
 *
 * File:        stdlib/net/http/results.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   HTTP error and result types.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.2 `version.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - HTTP protocol-version identity and enabled protocol sets
 *
 * File:        stdlib/net/http/version.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   HTTP protocol-version identity and enabled protocol sets.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *   - RFC 9113
 *     https://www.rfc-editor.org/rfc/rfc9113
 *   - RFC 9114
 *     https://www.rfc-editor.org/rfc/rfc9114
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.3 `methods.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - HTTP method tokens and IANA method metadata
 *
 * File:        stdlib/net/http/methods.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   HTTP method tokens and IANA method metadata.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *   - RFC 5789
 *     https://www.rfc-editor.org/rfc/rfc5789
 *   - RFC 10008
 *     https://www.rfc-editor.org/rfc/rfc10008
 *   - IANA HTTP Method Registry
 *     https://www.iana.org/assignments/http-methods
 *
 * Registry:
 *   IANA HTTP Method Registry — https://www.iana.org/assignments/http-methods
 *   Registry synchronized: 2026-09-08
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.4 `status.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - HTTP status codes and status-class helpers
 *
 * File:        stdlib/net/http/status.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   HTTP status codes and status-class helpers.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *   - IANA HTTP Status Code Registry
 *     https://www.iana.org/assignments/http-status-codes
 *
 * Registry:
 *   IANA HTTP Status Code Registry — https://www.iana.org/assignments/http-status-codes
 *   Registry synchronized: 2026-09-08
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.5 `header.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - HTTP field names, field values, and repeated field storage
 *
 * File:        stdlib/net/http/header.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   HTTP field names, field values, and repeated field storage.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *   - IANA HTTP Field Name Registry
 *     https://www.iana.org/assignments/http-fields
 *
 * Registry:
 *   IANA HTTP Field Name Registry — https://www.iana.org/assignments/http-fields
 *   Registry synchronized: 2026-09-08
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.6 `structured.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - HTTP Structured Fields parsing and serialization
 *
 * File:        stdlib/net/http/structured.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   HTTP Structured Fields parsing and serialization.
 *
 * Standards:
 *   - RFC 9651
 *     https://www.rfc-editor.org/rfc/rfc9651
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.7 `body.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - Owned streaming HTTP message bodies
 *
 * File:        stdlib/net/http/body.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Owned streaming HTTP message bodies.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.8 `request.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - Version-independent HTTP request values and server request metadata
 *
 * File:        stdlib/net/http/request.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Version-independent HTTP request values and server request metadata.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.9 `response.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - Version-independent HTTP responses and server response writing
 *
 * File:        stdlib/net/http/response.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Version-independent HTTP responses and server response writing.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.10 `auth.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - Generic HTTP authentication values plus Basic and Bearer helpers
 *
 * File:        stdlib/net/http/auth.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Generic HTTP authentication values plus Basic and Bearer helpers.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *   - RFC 7617
 *     https://www.rfc-editor.org/rfc/rfc7617
 *   - RFC 6750
 *     https://www.rfc-editor.org/rfc/rfc6750
 *   - IANA HTTP Authentication Scheme Registry
 *     https://www.iana.org/assignments/http-authschemes
 *
 * Registry:
 *   IANA HTTP Authentication Scheme Registry — https://www.iana.org/assignments/http-authschemes
 *   Registry synchronized: 2026-09-08
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.11 `digest.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - HTTP Content-Digest, Repr-Digest, and digest preferences
 *
 * File:        stdlib/net/http/digest.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   HTTP Content-Digest, Repr-Digest, and digest preferences.
 *
 * Standards:
 *   - RFC 9530
 *     https://www.rfc-editor.org/rfc/rfc9530
 *   - RFC 9651
 *     https://www.rfc-editor.org/rfc/rfc9651
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.12 `priority.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - Extensible HTTP priority values
 *
 * File:        stdlib/net/http/priority.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Extensible HTTP priority values.
 *
 * Standards:
 *   - RFC 9218
 *     https://www.rfc-editor.org/rfc/rfc9218
 *   - RFC 9651
 *     https://www.rfc-editor.org/rfc/rfc9651
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.13 `redirect.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - Bounded client redirect policy and redirect semantics
 *
 * File:        stdlib/net/http/redirect.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Bounded client redirect policy and redirect semantics.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.14 `cache.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - Bounded HTTP client caching policy
 *
 * File:        stdlib/net/http/cache.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Bounded HTTP client caching policy.
 *
 * Standards:
 *   - RFC 9111
 *     https://www.rfc-editor.org/rfc/rfc9111
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.15 `cookie.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - Cookie and Set-Cookie value types
 *
 * File:        stdlib/net/http/cookie.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Cookie and Set-Cookie value types.
 *
 * Standards:
 *   - RFC 10025
 *     https://www.rfc-editor.org/info/rfc10025
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.16 `cookie_parse.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - Cookie and Set-Cookie parsing
 *
 * File:        stdlib/net/http/cookie_parse.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Cookie and Set-Cookie parsing.
 *
 * Standards:
 *   - RFC 10025
 *     https://www.rfc-editor.org/info/rfc10025
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.17 `cookie_jar.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - Cookie storage and request selection
 *
 * File:        stdlib/net/http/cookie_jar.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Cookie storage and request selection.
 *
 * Standards:
 *   - RFC 10025
 *     https://www.rfc-editor.org/info/rfc10025
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.18 `client.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - Reusable HTTP client API
 *
 * File:        stdlib/net/http/client.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Reusable HTTP client API.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *   - RFC 9111
 *     https://www.rfc-editor.org/rfc/rfc9111
 *   - RFC 9112
 *     https://www.rfc-editor.org/rfc/rfc9112
 *   - RFC 9113
 *     https://www.rfc-editor.org/rfc/rfc9113
 *   - RFC 9114
 *     https://www.rfc-editor.org/rfc/rfc9114
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.19 `transport.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - Private version-independent client transport dispatch
 *
 * File:        stdlib/net/http/transport.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Private version-independent client transport dispatch.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *   - RFC 9112
 *     https://www.rfc-editor.org/rfc/rfc9112
 *   - RFC 9113
 *     https://www.rfc-editor.org/rfc/rfc9113
 *   - RFC 9114
 *     https://www.rfc-editor.org/rfc/rfc9114
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.20 `server.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - HTTP server and handler API
 *
 * File:        stdlib/net/http/server.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   HTTP server and handler API.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *   - RFC 9112
 *     https://www.rfc-editor.org/rfc/rfc9112
 *   - RFC 9113
 *     https://www.rfc-editor.org/rfc/rfc9113
 *   - RFC 9114
 *     https://www.rfc-editor.org/rfc/rfc9114
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.21 `router.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - Small HTTP routing convenience layer
 *
 * File:        stdlib/net/http/router.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Small HTTP routing convenience layer.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.22 `http1.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - HTTP/1.0 compatibility and HTTP/1.1 framing
 *
 * File:        stdlib/net/http/http1.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   HTTP/1.0 compatibility and HTTP/1.1 framing.
 *
 * Standards:
 *   - RFC 9110
 *     https://www.rfc-editor.org/rfc/rfc9110
 *   - RFC 9112
 *     https://www.rfc-editor.org/rfc/rfc9112
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.23 `hpack.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - HPACK field compression
 *
 * File:        stdlib/net/http/hpack.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   HPACK field compression.
 *
 * Standards:
 *   - RFC 7541
 *     https://www.rfc-editor.org/rfc/rfc7541
 *   - RFC 9113
 *     https://www.rfc-editor.org/rfc/rfc9113
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.24 `http2.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - HTTP/2 framing and semantic mapping
 *
 * File:        stdlib/net/http/http2.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   HTTP/2 framing and semantic mapping.
 *
 * Standards:
 *   - RFC 9113
 *     https://www.rfc-editor.org/rfc/rfc9113
 *   - RFC 7541
 *     https://www.rfc-editor.org/rfc/rfc7541
 *   - RFC 9218
 *     https://www.rfc-editor.org/rfc/rfc9218
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.25 `qpack.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - QPACK field compression
 *
 * File:        stdlib/net/http/qpack.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   QPACK field compression.
 *
 * Standards:
 *   - RFC 9204
 *     https://www.rfc-editor.org/rfc/rfc9204
 *   - RFC 9114
 *     https://www.rfc-editor.org/rfc/rfc9114
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.26 `http3.sec`

```sec
module http

/*
 * Sec Standard Library - net/http - HTTP/3 framing and semantic mapping over QUIC
 *
 * File:        stdlib/net/http/http3.sec
 * Module:      http
 * Revision:    2
 * Updated:     2026-09-08
 * Author:      Jonas Engström
 *
 * Purpose:
 *   HTTP/3 framing and semantic mapping over QUIC.
 *
 * Standards:
 *   - RFC 9114
 *     https://www.rfc-editor.org/rfc/rfc9114
 *   - RFC 9204
 *     https://www.rfc-editor.org/rfc/rfc9204
 *   - RFC 9218
 *     https://www.rfc-editor.org/rfc/rfc9218
 *
 * Rulebook:
 *   stdlib/net/http/std-net-http.md
 */
```

## 6.27 `structured_fields.sec`
```sec
module http

/*
* Sec Standard Library - net/http - Structured Fields
* 
* File: stdlib/net/http/structured_fields.sec
* Module: http
* Revision: 1
* Updated: 2026-09-11
* Author: Jonas Engström
* 
* Purpose:
* Internal parsing and representation support for HTTP Structured Fields.
* 
* This file provides the shared RFC 9651 parsing machinery used by HTTP
* fields such as Priority. The Structured Fields implementation is kept
* private to the http module and does not form part of the public net/http
* API.
* 
* Standards:
*   - RFC 9651
*     https://www.rfc-editor.org/rfc/rfc9651
*
* Rulebook:
*   stdlib/net/http/std-net-http.md
*/
```

---

# Part I — Error model and protocol identity

# 7. `results.sec`

`results.sec` owns the package-level error vocabulary. Errors with no payload use
`enum ... error`. Error families that preserve an underlying cause use Sec's
`type X union error` form.

## 7.1 Public declarations

```sec
enum HeaderError error {
    InvalidName,
    InvalidValue,
    InvalidEncoding,
    InjectionAttempt,
    TooManyFields,
    FieldsTooLarge,
    AllocationFailed,
}

enum StructuredFieldError error {
    InvalidSyntax,
    InvalidKey,
    InvalidBareItem,
    InvalidInteger,
    InvalidDecimal,
    InvalidString,
    InvalidToken,
    InvalidByteSequence,
    InvalidBoolean,
    InvalidDate,
    InvalidDisplayString,
    DuplicateKey,
    DuplicateParameter,
    OutOfRange,
    AllocationFailed,
}

type BodyError union error {
    Closed,
    TooLarge,
    UnexpectedEof,
    LengthMismatch,
    NotReplayable,
    Source(error),
    Sink(error),
    Allocation(error),
}

enum MessageError error {
    InvalidMethod,
    ReservedMethod,
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

enum AuthError error {
    InvalidScheme,
    InvalidSyntax,
    InvalidToken68,
    InvalidParameter,
    DuplicateParameter,
    InvalidCredentials,
    InvalidBase64,
    InvalidUtf8,
    UnsupportedScheme,
    AllocationFailed,
}

enum DigestError error {
    InvalidSyntax,
    DuplicateAlgorithm,
    InvalidDigestValue,
    InvalidPreference,
    UnsupportedAlgorithm,
    DeprecatedAlgorithm,
    Mismatch,
    AllocationFailed,
}

enum RedirectError error {
    TooManyRedirects,
    InvalidLocation,
    BodyNotReplayable,
    CrossOriginRejected,
    PolicyRejected,
}

enum CacheError error {
    InvalidMetadata,
    EntryTooLarge,
    StorageLimit,
    RevalidationFailed,
    AllocationFailed,
}

enum CookieError error {
    InvalidName,
    InvalidValue,
    InvalidDomain,
    InvalidPath,
    InvalidPrefix,
    SecureRequired,
    SameSiteNoneRequiresSecure,
    PartitionedRequiresSecure,
    AllocationFailed,
}

enum CookieParseError error {
    InvalidCookiePair,
    InvalidAttribute,
    InvalidDate,
    InvalidMaxAge,
    DuplicateAttribute,
    TooLarge,
    AllocationFailed,
}

enum CookieJarError error {
    InvalidOrigin,
    InvalidCookie,
    DomainMismatch,
    PublicSuffixRejected,
    InsecureSecureCookie,
    SameSiteRejected,
    PartitionRejected,
    StorageLimit,
    AllocationFailed,
}

enum RouterError error {
    InvalidPattern,
    DuplicateRoute,
    ConflictingRoute,
    AllocationFailed,
}

type ClientError union error {
    Url(error),
    Dns(error),
    Network(error),
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
    Closed,
}

type ServerError union error {
    Network(error),
    Tls(error),
    Quic(error),
    Message(MessageError),
    Header(HeaderError),
    Body(BodyError),
    Handler(error),
    ConnectionClosed,
    ResourceLimitExceeded,
    Closed,
}
```

### Representation rationale

The simple protocol-validation failures are error enums because no additional
payload is required to identify the failure. `BodyError`, `ClientError`, and
`ServerError` are error unions because preserving the underlying source, sink,
DNS, network, TLS, QUIC, handler, or subordinate HTTP error is diagnostically and
programmatically useful. Implementations must not flatten those causes into text.

---

# 8. `version.sec`

## 8.1 `Version`

```sec
enum Version {
    Http10,
    Http11,
    Http2,
    Http3,
}

impl Version {
    fn ToString() string
    property IsMultiplexed: bool
}
```

`ToString()` returns exactly `"HTTP/1.0"`, `"HTTP/1.1"`, `"HTTP/2"`, or
`"HTTP/3"` for diagnostic/configuration use. It does not imply that HTTP/2 or
HTTP/3 put such a version token on their request wire format.

`IsMultiplexed` is false for `Http10` and `Http11`, true for `Http2` and `Http3`.

### Representation rationale

`Version` is an enum because the HTTP package supports a closed semantic set of
protocol versions. A `{Major, Minor}` struct would incorrectly imply that an
arbitrary numeric pair is a supported HTTP protocol. The name `Version` appears
exactly once in the module.

## 8.2 `Protocols`

```sec
type Protocols struct {
    Http11: bool,
    Http2: bool,
    Http3: bool,
}

impl Protocols {
    init() {
        self.Http11 = true
        self.Http2 = true
        self.Http3 = true
    }

    fn Validate() Result[void, MessageError]
    property Any: bool
}
```

`Validate()` returns `MessageError.UnsupportedProtocol` when all three fields are
false. Target capability may reduce what can actually be used, but does not
silently mutate the user's requested configuration.

HTTP/1.0 is accepted by the HTTP/1 parser for compatibility and is not an
outbound client negotiation preference.

### Representation rationale

`Protocols` is a struct because HTTP/1.1, HTTP/2, and HTTP/3 are independent
capability switches that can be enabled simultaneously; they are not mutually
exclusive alternatives.

---

# 9. `methods.sec`

`Method` is an open, case-sensitive HTTP token namespace.

```sec
type Method string

impl Method {
    static let ACL: Method := "ACL"
    static let BASELINE_CONTROL: Method := "BASELINE-CONTROL"
    static let BIND: Method := "BIND"
    static let CHECKIN: Method := "CHECKIN"
    static let CHECKOUT: Method := "CHECKOUT"
    static let CONNECT: Method := "CONNECT"
    static let COPY: Method := "COPY"
    static let DELETE: Method := "DELETE"
    static let GET: Method := "GET"
    static let HEAD: Method := "HEAD"
    static let LABEL: Method := "LABEL"
    static let LINK: Method := "LINK"
    static let LOCK: Method := "LOCK"
    static let MERGE: Method := "MERGE"
    static let MKACTIVITY: Method := "MKACTIVITY"
    static let MKCALENDAR: Method := "MKCALENDAR"
    static let MKCOL: Method := "MKCOL"
    static let MKREDIRECTREF: Method := "MKREDIRECTREF"
    static let MKWORKSPACE: Method := "MKWORKSPACE"
    static let MOVE: Method := "MOVE"
    static let OPTIONS: Method := "OPTIONS"
    static let ORDERPATCH: Method := "ORDERPATCH"
    static let PATCH: Method := "PATCH"
    static let POST: Method := "POST"
    static let PRI: Method := "PRI"
    static let PROPFIND: Method := "PROPFIND"
    static let PROPPATCH: Method := "PROPPATCH"
    static let PUT: Method := "PUT"
    static let QUERY: Method := "QUERY"
    static let REBIND: Method := "REBIND"
    static let REPORT: Method := "REPORT"
    static let SEARCH: Method := "SEARCH"
    static let TRACE: Method := "TRACE"
    static let UNBIND: Method := "UNBIND"
    static let UNCHECKOUT: Method := "UNCHECKOUT"
    static let UNLINK: Method := "UNLINK"
    static let UNLOCK: Method := "UNLOCK"
    static let UPDATE: Method := "UPDATE"
    static let UPDATEREDIRECTREF: Method := "UPDATEREDIRECTREF"
    static let VERSION_CONTROL: Method := "VERSION-CONTROL"
    static let ASTERISK: Method := "*"

    static fn Parse(value: string) Result[Method, MessageError]
    fn ToString() string

    property IsRegistered: bool
    property IsReserved: bool
    property IsSafe: Option[bool]
    property IsIdempotent: Option[bool]
}
```

`Parse` validates RFC 9110 `token` syntax and preserves the exact case supplied
for syntactically valid extension methods. It must not lowercase method names.
Registered constants use their registry spelling.

`IsRegistered` is true for every constant listed above. `IsReserved` is true for
`PRI` and `ASTERISK`; those values are represented because the IANA registry
contains them, but `Request.Validate()` rejects them for ordinary application
requests. HTTP/2 connection-preface processing handles `PRI` privately.

For registered methods, `IsSafe` and `IsIdempotent` return the IANA metadata
synchronized below. For syntactically valid unregistered extension methods they
return `None`; the library never guesses retry semantics.

## 9.1 Registry metadata snapshot

| Sec constant | Wire method | Safe | Idempotent |
|---|---|---:|---:|
| `Method.ACL` | `ACL` | no | yes |
| `Method.BASELINE_CONTROL` | `BASELINE-CONTROL` | no | yes |
| `Method.BIND` | `BIND` | no | yes |
| `Method.CHECKIN` | `CHECKIN` | no | yes |
| `Method.CHECKOUT` | `CHECKOUT` | no | yes |
| `Method.CONNECT` | `CONNECT` | no | no |
| `Method.COPY` | `COPY` | no | yes |
| `Method.DELETE` | `DELETE` | no | yes |
| `Method.GET` | `GET` | yes | yes |
| `Method.HEAD` | `HEAD` | yes | yes |
| `Method.LABEL` | `LABEL` | no | yes |
| `Method.LINK` | `LINK` | no | yes |
| `Method.LOCK` | `LOCK` | no | no |
| `Method.MERGE` | `MERGE` | no | yes |
| `Method.MKACTIVITY` | `MKACTIVITY` | no | yes |
| `Method.MKCALENDAR` | `MKCALENDAR` | no | yes |
| `Method.MKCOL` | `MKCOL` | no | yes |
| `Method.MKREDIRECTREF` | `MKREDIRECTREF` | no | yes |
| `Method.MKWORKSPACE` | `MKWORKSPACE` | no | yes |
| `Method.MOVE` | `MOVE` | no | yes |
| `Method.OPTIONS` | `OPTIONS` | yes | yes |
| `Method.ORDERPATCH` | `ORDERPATCH` | no | yes |
| `Method.PATCH` | `PATCH` | no | no |
| `Method.POST` | `POST` | no | no |
| `Method.PRI` | `PRI` | yes | yes |
| `Method.PROPFIND` | `PROPFIND` | yes | yes |
| `Method.PROPPATCH` | `PROPPATCH` | no | yes |
| `Method.PUT` | `PUT` | no | yes |
| `Method.QUERY` | `QUERY` | yes | yes |
| `Method.REBIND` | `REBIND` | no | yes |
| `Method.REPORT` | `REPORT` | yes | yes |
| `Method.SEARCH` | `SEARCH` | yes | yes |
| `Method.TRACE` | `TRACE` | yes | yes |
| `Method.UNBIND` | `UNBIND` | no | yes |
| `Method.UNCHECKOUT` | `UNCHECKOUT` | no | yes |
| `Method.UNLINK` | `UNLINK` | no | yes |
| `Method.UNLOCK` | `UNLOCK` | no | yes |
| `Method.UPDATE` | `UPDATE` | no | yes |
| `Method.UPDATEREDIRECTREF` | `UPDATEREDIRECTREF` | no | yes |
| `Method.VERSION_CONTROL` | `VERSION-CONTROL` | no | yes |
| `Method.ASTERISK` | `*` | no | no |

Registry synchronization date: **2026-09-08**.

### Representation rationale

`Method` is a nominal `string`, not an enum, because HTTP methods are an open
registry and extension methods are explicitly allowed. The nominal type prevents
ordinary strings from being confused with validated method tokens and supplies
registry metadata without closing the namespace.

---

# 10. `status.sec`

## 10.1 `StatusClass`

```sec
enum StatusClass {
    Informational,
    Success,
    Redirection,
    ClientError,
    ServerError,
    Extension,
}
```

### Representation rationale

`StatusClass` is an enum because these six semantic classifications are closed by
this API. `Extension` covers valid Sec `Status` values in 600..999.

## 10.2 `Status`

Current Sec 0.1 ranged named types require a default. The normative declaration is:

```sec
type Status int range 100..999 default 500

impl Status {
    static let Continue: Status := 100
    static let SwitchingProtocols: Status := 101
    static let Processing: Status := 102
    static let EarlyHints: Status := 103
    static let UploadResumptionSupported: Status := 104
    static let Ok: Status := 200
    static let Created: Status := 201
    static let Accepted: Status := 202
    static let NonAuthoritativeInformation: Status := 203
    static let NoContent: Status := 204
    static let ResetContent: Status := 205
    static let PartialContent: Status := 206
    static let MultiStatus: Status := 207
    static let AlreadyReported: Status := 208
    static let ImUsed: Status := 226
    static let MultipleChoices: Status := 300
    static let MovedPermanently: Status := 301
    static let Found: Status := 302
    static let SeeOther: Status := 303
    static let NotModified: Status := 304
    static let UseProxy: Status := 305
    static let Unused306: Status := 306
    static let TemporaryRedirect: Status := 307
    static let PermanentRedirect: Status := 308
    static let BadRequest: Status := 400
    static let Unauthorized: Status := 401
    static let PaymentRequired: Status := 402
    static let Forbidden: Status := 403
    static let NotFound: Status := 404
    static let MethodNotAllowed: Status := 405
    static let NotAcceptable: Status := 406
    static let ProxyAuthenticationRequired: Status := 407
    static let RequestTimeout: Status := 408
    static let Conflict: Status := 409
    static let Gone: Status := 410
    static let LengthRequired: Status := 411
    static let PreconditionFailed: Status := 412
    static let ContentTooLarge: Status := 413
    static let UriTooLong: Status := 414
    static let UnsupportedMediaType: Status := 415
    static let RangeNotSatisfiable: Status := 416
    static let ExpectationFailed: Status := 417
    static let Unused418: Status := 418
    static let MisdirectedRequest: Status := 421
    static let UnprocessableContent: Status := 422
    static let Locked: Status := 423
    static let FailedDependency: Status := 424
    static let TooEarly: Status := 425
    static let UpgradeRequired: Status := 426
    static let PreconditionRequired: Status := 428
    static let TooManyRequests: Status := 429
    static let RequestHeaderFieldsTooLarge: Status := 431
    static let UnavailableForLegalReasons: Status := 451
    static let InternalServerError: Status := 500
    static let NotImplemented: Status := 501
    static let BadGateway: Status := 502
    static let ServiceUnavailable: Status := 503
    static let GatewayTimeout: Status := 504
    static let HttpVersionNotSupported: Status := 505
    static let VariantAlsoNegotiates: Status := 506
    static let InsufficientStorage: Status := 507
    static let LoopDetected: Status := 508
    static let NotExtended: Status := 510
    static let NetworkAuthenticationRequired: Status := 511

    fn ToString() string
    property ReasonPhrase: Option[string]
    property Class: StatusClass
    property IsRegistered: bool
    property IsTemporary: bool
    property IsObsolete: bool
    property IsInformational: bool
    property IsSuccess: bool
    property IsRedirection: bool
    property IsClientError: bool
    property IsServerError: bool
}
```

The default is **500**, not a fabricated uninitialized protocol code. The choice
is defensive: implicit/default construction must not silently imply success. If
Sec later allows a ranged nominal type to have no default, this book should be
revisited and the semantic default should preferably be removed rather than
inventing a wire sentinel.

`ToString()` returns `"<code> <reason>"` for a value with a known reason phrase,
for example `"404 Not Found"`; otherwise it returns the decimal code only.
`ReasonPhrase` is `None` for unknown/unassigned values and for the two registry
entries whose description is `Unused`.

104 is marked temporary at this registry snapshot and must be rechecked before
its current registration expiry date of 2026-11-13. 510 remains representable but
`IsObsolete` is true.

### Representation rationale

`Status` is an open nominal ranged integer rather than an enum because HTTP status
codes have an extensible numeric namespace and unknown extension values must
remain representable. The range enforces the three-digit protocol domain while
associated immutables provide names for the synchronized registry values.

---

# Part II — HTTP fields

# 11. `header.sec`

## 11.1 `HeaderName`

```sec
type HeaderName string

impl HeaderName {
    static let Accept: HeaderName := "accept"
    static let AcceptCharset: HeaderName := "accept-charset"
    static let AcceptEncoding: HeaderName := "accept-encoding"
    static let AcceptLanguage: HeaderName := "accept-language"
    static let AcceptPatch: HeaderName := "accept-patch"
    static let AcceptQuery: HeaderName := "accept-query"
    static let AcceptRanges: HeaderName := "accept-ranges"
    static let Age: HeaderName := "age"
    static let Allow: HeaderName := "allow"
    static let AltSvc: HeaderName := "alt-svc"
    static let Authorization: HeaderName := "authorization"
    static let CacheControl: HeaderName := "cache-control"
    static let Connection: HeaderName := "connection"
    static let ContentDigest: HeaderName := "content-digest"
    static let ContentDisposition: HeaderName := "content-disposition"
    static let ContentEncoding: HeaderName := "content-encoding"
    static let ContentLanguage: HeaderName := "content-language"
    static let ContentLength: HeaderName := "content-length"
    static let ContentLocation: HeaderName := "content-location"
    static let ContentRange: HeaderName := "content-range"
    static let ContentType: HeaderName := "content-type"
    static let Cookie: HeaderName := "cookie"
    static let Date: HeaderName := "date"
    static let ETag: HeaderName := "etag"
    static let Expect: HeaderName := "expect"
    static let Expires: HeaderName := "expires"
    static let Forwarded: HeaderName := "forwarded"
    static let From: HeaderName := "from"
    static let Host: HeaderName := "host"
    static let IfMatch: HeaderName := "if-match"
    static let IfModifiedSince: HeaderName := "if-modified-since"
    static let IfNoneMatch: HeaderName := "if-none-match"
    static let IfRange: HeaderName := "if-range"
    static let IfUnmodifiedSince: HeaderName := "if-unmodified-since"
    static let Incremental: HeaderName := "incremental"
    static let LastModified: HeaderName := "last-modified"
    static let Link: HeaderName := "link"
    static let Location: HeaderName := "location"
    static let MaxForwards: HeaderName := "max-forwards"
    static let Origin: HeaderName := "origin"
    static let Priority: HeaderName := "priority"
    static let ProxyAuthenticate: HeaderName := "proxy-authenticate"
    static let ProxyAuthorization: HeaderName := "proxy-authorization"
    static let Range: HeaderName := "range"
    static let Referer: HeaderName := "referer"
    static let ReprDigest: HeaderName := "repr-digest"
    static let RetryAfter: HeaderName := "retry-after"
    static let Server: HeaderName := "server"
    static let SetCookie: HeaderName := "set-cookie"
    static let TE: HeaderName := "te"
    static let Trailer: HeaderName := "trailer"
    static let TransferEncoding: HeaderName := "transfer-encoding"
    static let Upgrade: HeaderName := "upgrade"
    static let UserAgent: HeaderName := "user-agent"
    static let Vary: HeaderName := "vary"
    static let Via: HeaderName := "via"
    static let WantContentDigest: HeaderName := "want-content-digest"
    static let WantReprDigest: HeaderName := "want-repr-digest"
    static let WWWAuthenticate: HeaderName := "www-authenticate"

    static fn Parse(value: string) Result[HeaderName, HeaderError]
    fn ToString() string
}
```

This list is the **complete convenience-constant set for revision 0.2**. It is
not claimed to be a mirror of every IANA field name. Other valid registered or
extension field names remain representable through `HeaderName.Parse()`.

`Parse` accepts RFC 9110 field-name token syntax, canonicalizes ASCII letters to
lowercase, and rejects empty names, colon, whitespace, controls, CR, LF, and any
name beginning with `:`. HTTP/2 and HTTP/3 pseudo-fields are private transport
representation and cannot be injected through the application Header API.

### Representation rationale

`HeaderName` is a nominal string because the HTTP field-name registry is open and
case-insensitive. Canonical lowercase storage makes equality deterministic and is
compatible with HTTP/2 and HTTP/3 requirements without closing the namespace.

## 11.2 `HeaderValue`

```sec
type HeaderValue string

impl HeaderValue {
    static fn Parse(value: string) Result[HeaderValue, HeaderError]
    fn ToString() string
}
```

Sec's `HeaderValue` uses a reversible one-codepoint-per-octet representation for
HTTP field octets. Accepted scalar values are HTAB, SP, U+0021..U+007E, and
U+0080..U+00FF. CR, LF, NUL, other C0 controls, DEL, and code points above U+00FF
are rejected. On wire encode, each accepted scalar maps to the octet of the same
numeric value. On wire decode, each accepted octet maps back to that scalar.

This preserves RFC 9110 `obs-text` as opaque data instead of corrupting it through
UTF-8 reinterpretation. Field-specific specifications such as Structured Fields
may decode their own Unicode representation on top of this opaque field layer.

### Representation rationale

`HeaderValue` remains a nominal string for ergonomic field handling, but its
allowed scalar domain is deliberately byte-preserving. This is preferable to an
ordinary Unicode string contract that could not round-trip all legal HTTP field
octets.

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
    init() {
    }

    fn Add(name: HeaderName, value: HeaderValue) Result[void, HeaderError]
    fn Add(name: string, value: string) Result[void, HeaderError]

    fn Set(name: HeaderName, value: HeaderValue) Result[void, HeaderError]
    fn Set(name: string, value: string) Result[void, HeaderError]

    fn Get(name: HeaderName) Option[HeaderValue]
    fn Get(name: string) Result[Option[HeaderValue], HeaderError]

    fn GetAllToArray(name: HeaderName) Result[HeaderValue[], HeaderError]
    fn GetAllToArray(name: string) Result[HeaderValue[], HeaderError]

    fn Contains(name: HeaderName) bool
    fn Contains(name: string) Result[bool, HeaderError]

    fn Remove(name: HeaderName) bool
    fn Remove(name: string) Result[bool, HeaderError]

    fn Clear()
    property Count: uint
}
```

`Add` appends a new field occurrence without combining existing values. `Set`
removes all occurrences of the name and inserts exactly one new occurrence at the
position of the first removed occurrence; if the name did not exist, it appends.
The operation is transactional on allocation failure.

`Get` returns the first occurrence. `GetAllToArray` returns all occurrences in
wire/logical order in newly allocated storage. `Remove` removes every occurrence
and reports whether at least one existed. `Count` counts field occurrences, not
unique names.

Generic `Header` never performs comma combination. Field-specific code decides
whether combination is legal; in particular multiple `Set-Cookie` field lines
must remain separate.

### Representation rationale

`Header` is a struct because a field section is one value with owned ordered
state. Its private repeated-field list is intentional: a map would lose ordering
and naturally collapse repeated names, both of which are incorrect for a generic
HTTP field section.

---

# 12. `structured.sec`

RFC 9651 Structured Fields are exposed independently of particular registered
field names.

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

fn ParseStructuredItem(value: HeaderValue) Result[StructuredItem, StructuredFieldError]
fn ParseStructuredList(value: HeaderValue) Result[StructuredList, StructuredFieldError]
fn ParseStructuredDictionary(value: HeaderValue) Result[StructuredDictionary, StructuredFieldError]

fn SerializeStructuredItem(value: ref StructuredItem) Result[HeaderValue, StructuredFieldError]
fn SerializeStructuredList(value: ref StructuredList) Result[HeaderValue, StructuredFieldError]
fn SerializeStructuredDictionary(value: ref StructuredDictionary) Result[HeaderValue, StructuredFieldError]
```

Parsing is strict RFC 9651 parsing. Duplicate dictionary keys and duplicate
parameter keys are errors rather than silently overwritten. Serialization emits
the canonical RFC 9651 representation and preserves list/dictionary member order.

### Representation rationale

`StructuredBareItem` and `StructuredMember` are unions because exactly one wire
value shape is present. Parameters, items, inner lists, lists, and dictionaries
are structs because each value consists of several simultaneously meaningful
components. The byte-sequence variant owns its byte array; move/copy behavior of
containing values follows Sec aggregate ownership rules.

---

# Part III — Bodies, requests, and responses

# 13. `body.sec`

```sec
@noCopy
type Body struct {
    _state: _BodyState,
}

impl Body {
    init() {
    }

    init(data: <- byte[])

    static fn FromString(text: string) Result[Body, BodyError]

    property Length: Option[uint64]
    property Replayable: bool
    property IsClosed: bool

    fn Read(buffer: ref mut byte[]) Result[uint, BodyError]
    fn ReadAllToArray(limit: uint64) Result[byte[], BodyError]
    fn Rewind() Result[void, BodyError]
    fn Close() Result[void, BodyError]

    free()
}
```

`_BodyState` is private and implementation-defined. `init()` creates an empty,
replayable body of known length zero. `init(data: <- byte[])` consumes the array
without requiring an additional content copy and creates a replayable body.
`FromString` UTF-8 encodes the Sec string and is fallible because it allocates.

For a non-empty read buffer, `Ok(0)` means EOF. `ReadAllToArray` always enforces
the supplied maximum and returns `BodyError.TooLarge` before returning an array
larger than `limit`. `Rewind` succeeds only when `Replayable` is true. `Close` is
idempotent and releases the underlying stream/transport ownership. `free()` must
release the resource exactly once even when `Close()` was not called; destructor
cleanup cannot propagate an ordinary `Result` error.

### Representation rationale

`Body` is a move-only struct because it owns streaming state and potentially a
transport/resource lifetime. It is not an enum because the public API does not
expose transport-specific body source variants. The private representation may
use a union internally.

---

# 14. `request.sec`

## 14.1 Request-target component types

```sec
type OriginTarget string

impl OriginTarget {
    static fn Parse(value: string) Result[OriginTarget, MessageError]
    fn ToString() string
}

type AuthorityTarget string

impl AuthorityTarget {
    static fn Parse(value: string) Result[AuthorityTarget, MessageError]
    fn ToString() string
}

type RequestTarget union {
    Origin(OriginTarget),
    Absolute(url.URL),
    Authority(AuthorityTarget),
    Asterisk,
}
```

`OriginTarget` accepts an absolute-path plus optional query as required by HTTP
origin-form. `AuthorityTarget` accepts a valid authority value for CONNECT-style
authority-form. `RequestTarget.Asterisk` represents exactly `*`.

### Representation rationale

The two nominal strings prevent arbitrary strings from being mistaken for
validated target forms. `RequestTarget` is a union because one request uses
exactly one target form at a time and the forms have different payload types.

## 14.2 `Request`

```sec
type Request struct {
    Method: Method,
    Target: RequestTarget,
    Header: Header,
    Trailer: Header,
    Body: Body,
}

impl Request {
    init(method: Method, target: RequestTarget) {
        self.Method = method
        self.Target = target
    }

    static fn Get(url: url.URL) Request
    static fn Head(url: url.URL) Request
    static fn Post(url: url.URL, body: <- Body) Request
    static fn Put(url: url.URL, body: <- Body) Request
    static fn Patch(url: url.URL, body: <- Body) Request
    static fn Delete(url: url.URL) Request
    static fn Query(url: url.URL, body: <- Body) Request

    fn SetBody(body: <- Body)
    fn Validate() Result[void, MessageError]

    property Url: Option[url.URL]
    property HasBody: bool

    free()
}
```

A request is move-only transitively because it owns `Body`; it does not require a
second public move-only marker merely to restate that fact. The convenience
factories create `RequestTarget.Absolute(url)` requests with the corresponding
registered method.

`SetBody` closes/destroys any existing body before taking ownership of the new
body. `Validate` verifies method syntax, rejects `Method.PRI` and
`Method.ASTERISK` as ordinary application methods, validates target-form/method
compatibility, and validates message-level field/framing constraints that are
known before transport selection.

`Url` returns `Some` only for `Absolute` targets. `HasBody` is false only for the
known empty body state; it does not infer semantic absence from the method name.
`free()` releases the owned body.

### Representation rationale

`Request` is a struct because method, target, fields, trailers, and body are all
parts of one HTTP request. It is move-only through aggregate ownership because
`Body` is move-only.

## 14.3 Server transport metadata

```sec
type ConnectionInfo struct {
    LocalEndpoint: ip.Endpoint,
    RemoteEndpoint: ip.Endpoint,
    Version: Version,
    Secure: bool,
}

type ServerRequest struct {
    Message: Request,
    Connection: ConnectionInfo,
}
```

`ConnectionInfo` is a struct because its fields describe simultaneous properties
of one accepted connection/stream. `ServerRequest` is a struct because it couples
the protocol request with transport metadata without polluting the portable
`Request` value. It is move-only transitively through `Request.Body`.

---

# 15. `response.sec`

## 15.1 `Response`

```sec
type Response struct {
    Status: Status,
    Header: Header,
    Trailer: Header,
    Body: Body,
    Version: Version,
}

impl Response {
    init(status: Status) {
        self.Status = status
        self.Version = Version.Http11
    }

    init(status: Status, version: Version) {
        self.Status = status
        self.Version = version
    }

    fn Close() Result[void, BodyError]
    free()
}
```

The one-argument constructor's `Http11` value is only the default for a manually
constructed semantic response; protocol negotiation and parsers set the actual
version on received responses. `Close()` delegates body shutdown and is
idempotent. `free()` guarantees owned-body release when the caller omits or cannot
successfully complete `Close()`.

`Response` is move-only transitively because it contains `Body`.

### Representation rationale

`Response` is a struct because status, fields, trailers, body, and version are
simultaneous parts of one response. It is not represented as a union by HTTP
version because the application semantics are shared across versions.

## 15.2 `ResponseWriter`

```sec
@noCopy
type ResponseWriter struct {
    _state: _ResponseWriterState,
}

impl ResponseWriter {
    property Header: ref mut Header
    property Trailer: ref mut Header
    property Status: Option[Status]
    property IsCommitted: bool
    property IsFinished: bool

    fn WriteStatus(status: Status) Result[void, ServerError]
    fn Write(data: ref byte[]) Result[uint, ServerError]
    fn WriteAll(data: ref byte[]) Result[void, ServerError]
    fn Flush() Result[void, ServerError]
    fn Finish() Result[void, ServerError]

    free()
}
```

The first successful `Write`/`WriteAll` commits `Status.Ok` when no explicit
status was committed. A second `WriteStatus` after commitment is an error through
`ServerError.Message(MessageError.InvalidStatus)`. Ordinary response headers are
immutable after commitment. Trailer values may be populated only according to
version/framing rules and are committed at end of body.

`Finish` commits any pending status, completes trailers/body framing, and is
idempotent after successful completion. `free()` on an unfinished writer aborts
or resets the active stream/connection as required by the selected HTTP version;
it must never fabricate a successful response merely because the writer was
destroyed.

### Representation rationale

`ResponseWriter` is a move-only struct because it owns one active server response
stream and its commit state. Its transport-specific states are private.

---

# Part IV — Authentication, digest fields, and priority

# 16. `auth.sec`

HTTP authentication scheme names form an open, case-insensitive IANA namespace.
The package therefore uses a nominal string rather than an enum.

## 16.1 `AuthScheme`

```sec
type AuthScheme string

impl AuthScheme {
    static let Basic: AuthScheme := "basic"
    static let Bearer: AuthScheme := "bearer"
    static let Concealed: AuthScheme := "concealed"
    static let Digest: AuthScheme := "digest"
    static let DPoP: AuthScheme := "dpop"
    static let GNAP: AuthScheme := "gnap"
    static let HOBA: AuthScheme := "hoba"
    static let Mutual: AuthScheme := "mutual"
    static let Negotiate: AuthScheme := "negotiate"
    static let OAuth: AuthScheme := "oauth"
    static let PrivateToken: AuthScheme := "privatetoken"
    static let ScramSha1: AuthScheme := "scram-sha-1"
    static let ScramSha256: AuthScheme := "scram-sha-256"
    static let Vapid: AuthScheme := "vapid"

    static fn Parse(value: string) Result[AuthScheme, AuthError]
    fn ToString() string
    property IsRegistered: bool
}
```

The constants above are the **complete IANA HTTP Authentication Scheme registry
snapshot as synchronized on 2026-09-08**. Sec stores the underlying value in
lowercase ASCII so ordinary nominal-value equality matches RFC 9110's
case-insensitive scheme identity. Constant names retain the registry's familiar
spelling; `AuthScheme.Vapid` has the underlying value `"vapid"`.

`Parse` validates `token` syntax and ASCII-lowercases every accepted scheme,
including unknown extension schemes. This gives deterministic equality without
closing the namespace. `ToString()` therefore returns Sec's lowercase canonical
wire spelling. `IsRegistered` compares the canonical value to the synchronized
constant set.

### Representation rationale

`AuthScheme` is a nominal string rather than an enum because RFC 9110 explicitly
defines authentication schemes as an extensible registry. An enum would reject
future or application-specific schemes and would therefore model the protocol
incorrectly.

## 16.2 Generic `Auth`

```sec
type Auth struct {
    Scheme: AuthScheme,
    Data: Option[string],
}

impl Auth {
    init(scheme: AuthScheme) {
        self.Scheme = scheme
        self.Data = None
    }

    init(scheme: AuthScheme, data: string) {
        self.Scheme = scheme
        self.Data = Some(data)
    }

    static fn Parse(value: HeaderValue) Result[Auth, AuthError]
    fn Validate() Result[void, AuthError]
    fn ToHeaderValue() Result[HeaderValue, AuthError]
    property HasData: bool
}

fn ParseAuthChallenges(value: HeaderValue) Result[Auth[], AuthError]
```

`Auth` represents one generic credentials or challenge value: an auth scheme plus
its scheme-specific remainder. `Data` excludes the separating whitespace. The
remainder is deliberately preserved as a string instead of forced into one
parameter structure because RFC 9110 permits either `token68` or authentication
parameters, and registered schemes such as Negotiate have interoperability syntax
that does not fit a single universal high-level parameter model.

`Auth.Parse` parses exactly one credentials/challenge value and preserves the
scheme-specific remainder without interpreting it. `ParseAuthChallenges` strictly
implements RFC 9110 challenge-list grammar; it must not split naively on commas
because commas can delimit auth parameters within a challenge. A registered
scheme whose deployed syntax intentionally violates the generic grammar, notably
`Negotiate` as noted by the IANA registry, can still be represented by
`Auth.Parse` as one opaque value; the generic challenge-list parser does not
invent a non-standard grammar for it.

### Representation rationale

`Auth` is a struct because scheme and scheme-specific data are simultaneous parts
of one authentication value. `Data` is optional because a syntactically valid
scheme can appear without following data where its scheme grammar permits it.

## 16.3 Basic authentication

```sec
type BasicAuthentication struct {
    Username: string,
    Password: string,
}

impl BasicAuthentication {
    init(username: string, password: string) {
        self.Username = username
        self.Password = password
    }

    fn ToAuth() Result[Auth, AuthError]
    static fn Parse(auth: ref Auth) Result[BasicAuthentication, AuthError]
}

fn BasicAuthorization(username: string, password: string) Result[Auth, AuthError]
```

`ToAuth` and `BasicAuthorization` reject a username containing `:` because RFC
7617 uses the first colon as the user/password delimiter. They encode the
`username:password` octets with UTF-8 and Base64 through the canonical encoding
package. `BasicAuthentication.Parse` requires scheme `Basic`, Base64-decodes the
credentials, requires valid UTF-8 for Sec strings, and splits at the first colon.

Basic credentials are sensitive. LSP/debug formatting must not print the password
through an automatically derived ordinary representation. HTTP logging helpers
must redact `Authorization` and `Proxy-Authorization` by default.

### Representation rationale

`BasicAuthentication` is a struct because username and password are two
simultaneously meaningful fields of one decoded credential. It is not a union;
there are no alternative payload shapes.

## 16.4 Bearer authentication

```sec
fn BearerAuthorization(token: string) Result[Auth, AuthError]
fn ParseBearerAuthorization(auth: ref Auth) Result[string, AuthError]
```

`BearerAuthorization` validates RFC 6750 bearer credential syntax and returns
`Auth { Scheme: AuthScheme.Bearer, ... }`. `ParseBearerAuthorization` requires a
Bearer scheme and returns the validated token. Returned bearer tokens are
sensitive and must be redacted by ordinary HTTP logging.

## 16.5 Other registered schemes

All other registered and extension schemes are fully representable and parseable
through `AuthScheme`, `Auth`, and `ParseAuthChallenges`. Revision 0.2 deliberately
defines **no additional scheme-specific credential builder**. In particular,
RFC 7616 Digest authentication is not claimed as a typed high-level helper until
its nonce, qop, algorithm, and crypto integration are specified completely in a
separate revision. This is an explicit scope boundary, not an omitted API.

---

# 17. `digest.sec`

This file implements RFC 9530 HTTP digest fields; it is unrelated to the
scheme-specific RFC 7616 Digest authentication helper deferred above.

```sec
type DigestAlgorithm string

impl DigestAlgorithm {
    static let Sha512: DigestAlgorithm := "sha-512"
    static let Sha256: DigestAlgorithm := "sha-256"
    static let Md5: DigestAlgorithm := "md5"
    static let Sha1: DigestAlgorithm := "sha"
    static let UnixSum: DigestAlgorithm := "unixsum"
    static let UnixCksum: DigestAlgorithm := "unixcksum"

    static fn Parse(value: string) Result[DigestAlgorithm, DigestError]
    fn ToString() string
    property IsActive: Option[bool]
    property IsDeprecated: Option[bool]
}

type DigestValue struct {
    Algorithm: DigestAlgorithm,
    Value: byte[],
}

type DigestPreference uint8 range 0..10 default 0

type DigestPreferenceValue struct {
    Algorithm: DigestAlgorithm,
    Preference: DigestPreference,
}

fn ParseContentDigest(value: HeaderValue) Result[DigestValue[], DigestError]
fn ParseReprDigest(value: HeaderValue) Result[DigestValue[], DigestError]
fn ParseWantContentDigest(value: HeaderValue) Result[DigestPreferenceValue[], DigestError]
fn ParseWantReprDigest(value: HeaderValue) Result[DigestPreferenceValue[], DigestError]

fn SerializeContentDigest(values: ref DigestValue[]) Result[HeaderValue, DigestError]
fn SerializeReprDigest(values: ref DigestValue[]) Result[HeaderValue, DigestError]
fn SerializeWantContentDigest(values: ref DigestPreferenceValue[]) Result[HeaderValue, DigestError]
fn SerializeWantReprDigest(values: ref DigestPreferenceValue[]) Result[HeaderValue, DigestError]
```

Unknown syntactically valid digest algorithm keys remain representable. The six
constants above are the RFC 9530 initial registry set: SHA-512 and SHA-256 are
active; MD5, SHA-1 (`sha`), `unixsum`, and `unixcksum` are deprecated.

Parsing/serialization is implemented through RFC 9651 dictionaries. Duplicate
algorithm keys are rejected. A preference of 0 means not acceptable; 1 is the
lowest positive preference and 10 the highest.

This revision does not expose a generic hashing implementation from `net/http`.
When the `crypto` book is locked, verification helpers may be added here by a
later revision. Parsing deprecated algorithms remains allowed; generating or
verifying them by convenience APIs is not implied.

### Representation rationale

`DigestAlgorithm` is a nominal string because the hash-algorithm registry is
open. `DigestValue` and `DigestPreferenceValue` are structs because algorithm and
payload/preference coexist. `DigestPreference` is a ranged nominal integer
because RFC 9530 defines the exact 0..10 numeric domain. Its Sec default is 0,
the conservative `not acceptable` value, rather than silently expressing a
positive preference.

---

# 18. `priority.sec`

```sec
type PriorityUrgency uint8 range 0..7 default 3

type Priority struct {
    Urgency: PriorityUrgency,
    Incremental: bool,
}

impl Priority {
    init() {
        self.Urgency = 3
        self.Incremental = false
    }

    init(urgency: PriorityUrgency, incremental: bool) {
        self.Urgency = urgency
        self.Incremental = incremental
    }

    static fn Parse(value: HeaderValue) Result[Priority, StructuredFieldError]
    fn ToHeaderValue() Result[HeaderValue, StructuredFieldError]
    property IsDefault: bool
}
```

Urgency 0 is highest and 7 is lowest. Default urgency is 3. `Incremental=false`
is the default. Serialization uses the RFC 9218 Structured Field representation
and omits default members when canonical serialization permits it.

### Representation rationale

`PriorityUrgency` is a ranged nominal integer because RFC 9218 defines a compact
numeric domain with ordering semantics. `Priority` is a struct because urgency
and incremental preference are independent simultaneous dimensions.

---

# Part V — Redirects and caching

# 19. `redirect.sec`

```sec
type RedirectPolicy struct {
    MaxRedirects: uint8,
    AllowCrossOrigin: bool,
    PreserveSensitiveHeaders: bool,
}

impl RedirectPolicy {
    init() {
        self.MaxRedirects = 10
        self.AllowCrossOrigin = true
        self.PreserveSensitiveHeaders = false
    }

    fn Validate() Result[void, RedirectError]
}
```

### Redirect algorithm

A client with automatic redirects enabled follows these exact rules:

- 301 and 302: POST is rewritten to GET and the body is dropped; other methods are
  preserved and require a replayable body when a body must be resent.
- 303: HEAD remains HEAD; every other method becomes GET; the body is dropped.
- 307 and 308: method and body semantics are preserved; a body resend requires
  `Body.Replayable`.
- Any redirect beyond `MaxRedirects` returns `TooManyRedirects`.
- A relative `Location` is resolved against the previous request URL through
  `net/url`.
- A cross-origin redirect is rejected when `AllowCrossOrigin` is false.
- `Host` is always recomputed for the new target.
- `Cookie` is never copied directly across redirect processing; it is recomputed
  from the configured `CookieJar` for the new request context.
- When `PreserveSensitiveHeaders` is false, `Authorization` and
  `Proxy-Authorization` are removed across an origin change.
- `Proxy-Authorization` is never forwarded to an origin server as ordinary
  origin credentials.

### Representation rationale

`RedirectPolicy` is a struct because its limits and security switches are
independent settings used together. Redirect errors are a closed payload-free
enum in `results.sec`.

---

# 20. `cache.sec`

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

impl CacheOptions {
    init() {
        self.Mode = CacheMode.Disabled
        self.MaxEntries = 256
        self.MaxBytes = 16777216
    }

    fn Validate() Result[void, CacheError]
}
```

`CacheMode` is an enum because revision 0.2 exposes exactly two cache modes.
`CacheOptions` is a struct because mode and two independent limits coexist.

Caching is disabled by default. Memory mode is bounded by both entry count and
aggregate stored bytes. An individual response larger than `MaxBytes` is not
stored; the original response still succeeds unless another application policy
requires failure.

The cache implements RFC 9111 freshness, Age, Date, Cache-Control, Expires, Vary,
ETag, Last-Modified, conditional revalidation, invalidation, and authorization
rules. Cache miss is ordinary control flow, not `CacheError`.

---

# Part VI — Cookies

# 21. `cookie.sec`

## 21.1 `SameSite`

```sec
enum SameSite {
    Strict,
    Lax,
    None,
}
```

Absence is represented with `Option[SameSite]`; there is no synthetic
`Unspecified` enum member.

### Representation rationale

`SameSite` is an enum because the attribute's explicit values form a closed set.
Absence is not another protocol value and therefore belongs in `Option`.

## 21.2 `Cookie`

```sec
type Cookie struct {
    Name: string,
    Value: string,
}

impl Cookie {
    init(name: string, value: string) {
        self.Name = name
        self.Value = value
    }

    fn Validate() Result[void, CookieError]
    fn ToString() Result[string, CookieError]
}
```

`Cookie` represents exactly one request `cookie-pair`; response attributes do not
belong here. `ToString()` serializes exactly `name=value` after validation.

### Representation rationale

`Cookie` is a struct because name and value coexist in every cookie pair.

## 21.3 `SetCookie`

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

impl SetCookie {
    init(name: string, value: string) {
        self.Pair = new Cookie(name, value)
        self.Path = None
        self.Domain = None
        self.Expires = None
        self.MaxAge = None
        self.Secure = false
        self.HttpOnly = false
        self.SameSite = None
        self.Partitioned = false
        self.Extensions = None
    }

    property Name: string
    property Value: string

    fn Validate() Result[void, CookieError]
    fn ToString() Result[string, CookieError]
}
```

`SetCookie.Validate` enforces RFC 10025 name/value/attribute syntax, cookie-prefix
requirements, `SameSite=None` secure requirements, and Partitioned/Secure
requirements adopted by the package. `Extensions` preserves unrecognized
extension attributes for round-trip serialization; each entry contains one
complete extension-av without the leading semicolon.

### Representation rationale

`SetCookie` is a struct because a response cookie has one cookie pair plus a set
of independently present attributes. Optional attributes use `Option`; boolean
attributes use booleans because their presence is their value.

---

# 22. `cookie_parse.sec`

```sec
fn ParseCookieHeader(value: HeaderValue) Result[Cookie[], CookieParseError]
fn ParseCookieHeaders(header: ref Header) Result[Cookie[], CookieParseError]

impl SetCookie {
    static fn Parse(value: HeaderValue) Result[SetCookie, CookieParseError]
}

fn ParseSetCookieHeaders(header: ref Header) Result[SetCookie[], CookieParseError]
```

`ParseCookieHeader` parses one `Cookie` field value into newly allocated cookie
pairs in their received order. `ParseCookieHeaders` parses every Cookie field
occurrence in order. `SetCookie.Parse` parses exactly one Set-Cookie field line.
`ParseSetCookieHeaders` preserves every separate Set-Cookie field occurrence and
must never comma-combine them.

Parsing and serialization are intentionally separate from jar policy. A
syntactically valid Set-Cookie value may parse successfully and later be rejected
by `CookieJar.Store` because origin, SameSite, Secure, domain, or partition policy
does not permit storage.

---

# 23. `cookie_jar.sec`

```sec
type CookieJarOptions struct {
    MaxCookies: uint,
    MaxCookiesPerDomain: uint,
    MaxCookieBytes: uint,
}

impl CookieJarOptions {
    init() {
        self.MaxCookies = 3000
        self.MaxCookiesPerDomain = 180
        self.MaxCookieBytes = 4096
    }

    fn Validate() Result[void, CookieJarError]
}

type CookieRequestContext struct {
    Url: url.URL,
    Method: Method,
    TopLevelSite: Option[url.URL],
    IsTopLevelNavigation: bool,
}

@noCopy
type CookieJar struct {
    _state: _CookieJarState,
}

impl CookieJar {
    init() {
    }

    init(options: CookieJarOptions)

    fn Store(origin: ref url.URL, cookie: SetCookie) Result[void, CookieJarError]
    fn StoreAt(origin: ref url.URL, cookie: SetCookie, now: datetime) Result[void, CookieJarError]

    fn Cookies(context: ref CookieRequestContext) Result[Cookie[], CookieJarError]
    fn CookieHeader(context: ref CookieRequestContext) Result[Option[HeaderValue], CookieJarError]

    fn RemoveExpired(now: datetime) uint
    fn Clear()
    property Count: uint

    free()
}
```

The no-argument constructor uses the same defaults as `new CookieJarOptions()`. `Store` uses the
canonical current clock; `StoreAt` exists for deterministic tests and applications
with an explicit clock. Storage enforces origin/domain/path, expiry, Secure,
HttpOnly, SameSite, partitioning, prefix, and bounded-resource rules before
commit. An insertion that fails leaves the previous jar state intact.

Selection order follows RFC 10025 cookie ordering requirements. `CookieHeader`
returns `None` when no cookie is applicable; otherwise it serializes the selected
cookies into one request field value. HttpOnly controls non-HTTP access APIs; it
does not prevent HTTP selection by this jar.

Public-suffix rejection is required for Domain cookies. The implementation may use
an internal snapshot or the future canonical public-suffix facility, but the
observable rule is mandatory; it must not accept a Domain cookie for a public
suffix merely because a helper package is unavailable.

### Representation rationale

`CookieJarOptions` and `CookieRequestContext` are structs because their fields are
simultaneous independent inputs. `CookieJar` is an owned move-only struct because
it owns mutable bounded storage and expiration state.

---

# Part VII — Client and transport

# 24. `client.sec`

## 24.1 `ClientOptions`

```sec
type ClientOptions struct {
    Protocols: Protocols,
    Redirects: RedirectPolicy,
    Cache: CacheOptions,
    MaxResponseHeaderBytes: uint,
    MaxResponseBodyBytes: Option[uint64],
    MaxConnectionsPerOrigin: uint,
    MaxIdleConnections: uint,
    MaxIdleConnectionsPerOrigin: uint,
}

impl ClientOptions {
    init() {
        self.Protocols = new Protocols()
        self.Redirects = new RedirectPolicy()
        self.Cache = new CacheOptions()
        self.MaxResponseHeaderBytes = 65536
        self.MaxResponseBodyBytes = None
        self.MaxConnectionsPerOrigin = 16
        self.MaxIdleConnections = 64
        self.MaxIdleConnectionsPerOrigin = 8
    }

    fn Validate() Result[void, ClientError]
}
```

`MaxResponseBodyBytes=None` means there is no client-wide byte-count cap on a
streaming body; it does not mean the body is buffered without limit. Header,
connection, redirect, cache, compression, and cookie state remain bounded.

### Representation rationale

`ClientOptions` is a struct because protocol choices, policies, and limits are
independent settings used together.

## 24.2 `Client`

```sec
@noCopy
type Client struct {
    _state: _ClientState,
}

impl Client {
    init() {
    }

    init(options: ClientOptions)

    fn SetCookieJar(jar: <- CookieJar) Result[void, ClientError]
    fn ClearCookieJar()

    fn Do(request: <- Request) Result[Response, ClientError]

    fn Get(url: url.URL) Result[Response, ClientError]
    fn Head(url: url.URL) Result[Response, ClientError]
    fn Post(url: url.URL, body: <- Body) Result[Response, ClientError]
    fn Put(url: url.URL, body: <- Body) Result[Response, ClientError]
    fn Patch(url: url.URL, body: <- Body) Result[Response, ClientError]
    fn Delete(url: url.URL) Result[Response, ClientError]
    fn Query(url: url.URL, body: <- Body) Result[Response, ClientError]

    fn CloseIdleConnections()
    fn Close() Result[void, ClientError]

    free()
}
```

Construction stores validated configuration and may initialize purely local
state; network connections are established lazily. `Do` consumes the request
because request-body ownership and transport commit cannot safely be duplicated.
On pre-commit failure, ownership rollback follows Sec's ordinary consuming-call
rules; after protocol commit, the request is consumed even when the operation
ultimately returns an error.

The convenience methods are exactly equivalent to constructing the corresponding
`Request` helper and passing it to `Do`.

Cookie persistence is disabled until `SetCookieJar` is called. Cache behavior is
controlled by `ClientOptions.Cache` and is disabled by default. `Close` makes the
client permanently closed, closes pooled connections, and terminates owned
transport state. `free()` performs deterministic best-effort release and cannot
propagate close errors.

Client operations use Sec's current Task/Thread cancellation context and do not
introduce an HTTP-specific cancellation token.

### Representation rationale

`Client` is a move-only struct because it owns mutable resolver/transport state,
connection pools, optional cookie storage, and cache state.

---

# 25. `transport.sec`

`transport.sec` has no public declarations in revision 0.2.

A conforming implementation may use this private interface:

```sec
interface _ClientTransport {
    fn RoundTrip(request: <- Request) Result[Response, ClientError]
}
```

Transport mapping is:

```text
HTTP/1.1 -> TCP or TCP + TLS
HTTP/2   -> TCP + TLS + ALPN for ordinary h2
HTTP/3   -> QUIC
```

Cleartext h2c is not required by revision 0.2.

---

# Part VIII — Server and routing

# 26. `server.sec`

## 26.1 `ServerOptions`

```sec
type ServerOptions struct {
    Protocols: Protocols,
    MaxRequestHeaderBytes: uint,
    MaxRequestBodyBytes: Option[uint64],
    MaxConcurrentRequests: uint,
}

impl ServerOptions {
    init() {
        self.Protocols = new Protocols()
        self.MaxRequestHeaderBytes = 65536
        self.MaxRequestBodyBytes = None
        self.MaxConcurrentRequests = 1024
    }

    fn Validate() Result[void, ServerError]
}
```

## 26.2 `Handler`

```sec
interface Handler {
    fn Handle(
        request: <- ServerRequest,
        response: ref mut ResponseWriter
    ) Result[void, error]
}
```

The handler consumes the request because the request owns its body. It borrows the
response writer exclusively for the duration of the call. Handler errors are
preserved as `ServerError.Handler(error)` by the server boundary unless the
application has already committed a response and configured another error policy.

## 26.3 `Server`

```sec
@noCopy
type Server struct {
    _state: _ServerState,
}

impl Server {
    init(endpoint: ip.Endpoint, handler: <- Handler)
    init(endpoint: ip.Endpoint, handler: <- Handler, options: ServerOptions)

    fn Serve() Result[void, ServerError]
    fn Shutdown() Result[void, ServerError]
    fn Close() Result[void, ServerError]

    property IsServing: bool
    property IsClosed: bool

    free()
}
```

Construction stores configuration and handler ownership but does not need to bind
or perform network I/O. `Serve` performs bind/listen and blocks while serving.
`Shutdown` stops accepting new requests and waits for active requests/streams to
finish, subject to the current Sec cancellation/deadline context. `Close` stops
accepting and aborts remaining active transport state. Both are idempotent after
successful completion. `free()` performs deterministic close semantics without
propagating an ordinary error.

### Representation rationale

`ServerOptions` is a struct because limits and protocol switches coexist.
`Handler` is an interface because independent user types can provide request
handling behavior. `Server` is a move-only struct because it owns handler,
listener, connection, and lifecycle state.

---

# 27. `router.sec`

Router is a small stdlib convenience, not a web framework.

```sec
type RouteParam struct {
    Name: string,
    Value: string,
}

type RouteParams struct {
    _values: RouteParam[],
}

impl RouteParams {
    fn Get(name: string) Option[string]
    fn Contains(name: string) bool
    property Count: uint
}

type RoutedRequest struct {
    Request: ServerRequest,
    Params: RouteParams,
}

interface RouteHandler {
    fn Handle(
        request: <- RoutedRequest,
        response: ref mut ResponseWriter
    ) Result[void, error]
}

@noCopy
type Router struct {
    _state: _RouterState,
}

impl Router {
    init() {
    }

    fn Handle(method: Method, pattern: string, handler: <- RouteHandler) Result[void, RouterError]
    fn Get(pattern: string, handler: <- RouteHandler) Result[void, RouterError]
    fn Head(pattern: string, handler: <- RouteHandler) Result[void, RouterError]
    fn Post(pattern: string, handler: <- RouteHandler) Result[void, RouterError]
    fn Put(pattern: string, handler: <- RouteHandler) Result[void, RouterError]
    fn Patch(pattern: string, handler: <- RouteHandler) Result[void, RouterError]
    fn Delete(pattern: string, handler: <- RouteHandler) Result[void, RouterError]
    fn Query(pattern: string, handler: <- RouteHandler) Result[void, RouterError]

    fn Serve(
        request: <- ServerRequest,
        response: ref mut ResponseWriter
    ) Result[void, error]

    free()
}
```

`Router.Serve` is the adapter used when wiring a router into a server; an
implementation may make `Router` conform to `Handler` using the same method body.

Route pattern grammar for revision 0.2 is exact:

- `/literal/path` — literal segments;
- `/users/{id}` — one non-empty decoded segment captured as `id`;
- `/files/{*path}` — final wildcard capturing the remaining path, possibly empty;
- parameter names use ASCII letters, digits, and underscore and may not start with
  a digit;
- duplicate parameter names within one pattern are invalid;
- a wildcard is permitted only in the final segment;
- matching is performed on the URL path, not on the query component.

Literal segments outrank parameter segments; parameter segments outrank wildcard
segments. If two registrations have identical method and normalized pattern,
registration fails with `DuplicateRoute`. If two distinct patterns have equal
precedence and can match the same path, registration fails with
`ConflictingRoute`; registration order is not used to hide ambiguity.

A missing path match produces 404. A path match with no registered method produces
405 and an `Allow` field derived from registered methods. HEAD uses an explicit
HEAD route when present; otherwise a GET route may service HEAD with body output
suppressed by the writer.

### Representation rationale

`RouteParam`, `RouteParams`, and `RoutedRequest` are structs because their fields
coexist. `RouteHandler` is an interface to allow independent handler types.
`Router` is move-only because it owns route-table and handler state.

---

# Part IX — HTTP version implementations

# 28. `http1.sec`

`http1.sec` has no public declarations.

It owns HTTP/1.0 compatibility and HTTP/1.1 start lines, fields, Content-Length,
chunked transfer coding, trailers, persistence, Host, Connection, Upgrade, and
incremental parser/generator state.

The parser must be bounded and reject ambiguous framing. At minimum it must reject:

- conflicting Content-Length values;
- illegal Transfer-Encoding / Content-Length combinations;
- invalid chunk sizes or chunk framing;
- prohibited whitespace around the start line or field name;
- CR/LF field injection;
- invalid trailer declarations or forbidden trailer fields;
- message lengths that exceed configured limits.

The implementation follows RFC 9112 message-length precedence exactly; it must not
invent tolerant framing that can disagree with another compliant intermediary.

---

# 29. `hpack.sec`

`hpack.sec` has no public declarations. It implements RFC 7541 for `http2.sec`.

Requirements:

- bounded dynamic table size;
- peer SETTINGS limits honored;
- local decoded-header size limit applied independently of compressed size;
- Huffman decoding validated completely;
- integer overflow and malformed representation rejected;
- dynamic-table mutations are transactional with respect to parse failure where
  the RFC state machine requires connection continuity;
- compression state is never shared across unrelated HTTP/2 connections.

---

# 30. `http2.sec`

`http2.sec` has no public declarations.

It implements RFC 9113 and owns connection preface handling, SETTINGS, HEADERS,
DATA, CONTINUATION, stream states, flow control, RST_STREAM, GOAWAY, PING,
PRIORITY_UPDATE, pseudo-fields, and conversion to/from the public semantic types.

Application `Header` values never contain pseudo-field names. The transport maps
`:method`, `:scheme`, `:authority`, `:path`, and `:status` privately and validates
their ordering, uniqueness, and request/response applicability before exposing a
message.

Connection-specific HTTP/1 fields forbidden by RFC 9113 are rejected rather than
forwarded into HTTP/2.

`PRI * HTTP/2.0` preface processing is private even though the IANA method registry
entry is represented as `Method.PRI` for registry completeness.

---

# 31. `qpack.sec`

`qpack.sec` has no public declarations. It implements RFC 9204 for `http3.sec`.

Requirements:

- bounded dynamic table capacity;
- bounded blocked streams;
- peer SETTINGS and local limits enforced;
- decoded-header limits independent of compressed bytes;
- encoder/decoder stream instructions validated incrementally;
- stream cancellation releases blocked-state accounting;
- compression state is connection-local.

---

# 32. `http3.sec`

`http3.sec` has no public declarations.

It implements RFC 9114 over the canonical `net/quic` package and owns control
streams, request streams, SETTINGS, HEADERS, DATA, GOAWAY, priorities, pseudo-field
mapping, cancellation, and semantic conversion. It does not reimplement QUIC,
TLS, congestion control, or packet protection.

HTTP/3 endpoint discovery may use Alt-Svc and DNS HTTPS/SVCB information once the
corresponding `net/url`, `net/dns`, and `net/quic` books define their canonical
interfaces. Absence of such discovery never justifies ad hoc UDP probing.

---

# Part X — Cross-cutting behavior

# 33. Protocol negotiation

For HTTPS over TCP, ordinary ALPN preferences are `h2` and `http/1.1` according to
`Protocols`. HTTP/3 is negotiated through QUIC/H3, not through a TCP TLS
connection. Target capability may make one configured protocol unavailable; if no
configured protocol remains viable, the client returns
`ClientError.NoProtocolAvailable`.

# 34. Retry and cancellation

Potential cancellation points include DNS resolution, connect, TLS/QUIC
handshake, request-body write, response wait, body read, cache revalidation, and
server graceful shutdown. HTTP uses Sec's current execution cancellation context
and adds no separate token type.

Automatic retries require all of the following to be established:

- the method is known idempotent or an explicit future caller policy says retry is
  safe;
- the request body is replayable when it must be resent;
- transport state establishes whether the previous attempt committed in a way
  that permits retry;
- cancellation has not been requested.

Unknown extension methods are not automatically retried. A non-replayable body is
never silently resent after uncertain commit.

# 35. Resource limits

Limits are enforced while data is received, before attacker-controlled length
causes proportional unbounded allocation. For HTTP/2 and HTTP/3, decoded field
size is limited separately from HPACK/QPACK compressed size.

Server concurrency, connection-pool counts, redirect counts, cache storage,
cookie storage, dynamic compression tables, blocked QPACK streams, header bytes,
and configured body limits are all bounded.

# 36. Header splitting and request smuggling

A conforming implementation must reject ambiguous framing and must not normalize
malformed messages into a different message boundary than a compliant peer could
observe. CR/LF injection is rejected at `HeaderValue` construction and again at
wire-parser boundaries. Version-specific encoders reject illegal connection
fields and pseudo-field misuse.

# 37. Sensitive information

At minimum the following are redacted from ordinary request/response logging:

- `Authorization`;
- `Proxy-Authorization`;
- `Cookie`;
- `Set-Cookie`.

Scheme-specific credential values, bearer tokens, Basic passwords, cookie values,
TLS secrets, and digest-auth secrets must not appear in automatically derived
ordinary debug output.

# 38. Target capability

Pure facilities such as Method, Status, Header, Structured Fields, cookie parsing,
auth parsing, priority parsing, and semantic message construction are usable on
targets without networking where their ordinary allocation dependencies exist.

Active HTTP requires target capabilities:

- HTTP/1.1: TCP;
- HTTPS: TCP + TLS;
- HTTP/2 ordinary h2: TCP + TLS + ALPN;
- HTTP/3: QUIC and its security requirements.

Unsupported active use fails through target/compiler capability mechanisms or the
specified runtime error; the implementation must not silently downgrade security.

---

# Part XI — Implementation and validation requirements

# 39. Existing-source migration

The current source is implementation evidence, not a compatibility constraint.
Revision 0.2 requires these corrections:

- replace all constructor-like `static fn New()` APIs with `init(...)`;
- remove duplicate type identifiers such as a struct and enum both named
  `Version`;
- replace the status sentinel model with the ranged `Status` declaration in this
  book and private/Option writer state;
- make Method parsing case-sensitive and synchronize the complete current method
  registry constants listed here;
- remove the legacy `Status.Teapot` alias so status naming matches the current IANA
  registry;
- harden Header name/value validation and return HTTP-level errors rather than
  exposing collection allocation errors directly;
- replace in-memory-only message bodies with `Body` ownership;
- add `free()` where an HTTP value owns body/stream/client/server/router resources;
- replace incomplete Auth placeholders with the exact `AuthScheme`, `Auth`, Basic,
  and Bearer APIs in this book;
- remove `encode.sec` after URL responsibility is moved to `net/url`;
- remove `websockets.sec` after WebSocket responsibility is moved to
  `net/websocket`;
- remove/migrate `cookie_security.sec` from the normative HTTP protocol surface.

# 40. Required tests — Method and Status

At minimum:

- every Method constant in the registry table parses to the same value;
- method case preservation for unknown extension tokens;
- invalid method token rejection;
- safe/idempotent metadata for every registered method;
- reserved PRI and ASTERISK request rejection;
- every Status constant maps to the expected integer;
- status class boundaries 199/200/299/300/399/400/499/500/599/600/999;
- unknown valid status representation;
- default Status is 500 under current Sec 0.1 rules;
- 104 temporary metadata;
- 510 obsolete metadata;
- 418 registry-unused handling.

# 41. Required tests — Header and Structured Fields

At minimum:

- case-insensitive field-name input and lowercase canonical storage;
- every convenience HeaderName constant has the exact lower-case value;
- repeated values preserve order;
- Set replacement and Remove-all behavior;
- transactional allocation failure;
- CR/LF/NUL/control injection rejection;
- reversible obs-text mapping for every octet 0x80..0xFF;
- rejection of scalars above U+00FF in generic HeaderValue;
- Set-Cookie is never comma-combined;
- pseudo-field injection rejected;
- RFC 9651 parsing and canonical serialization vectors;
- duplicate dictionary/parameter rejection;
- all StructuredBareItem variants.

# 42. Required tests — Body, Request, Response

At minimum:

- empty body EOF behavior;
- owned byte-array body and replay;
- FromString UTF-8 bytes;
- ReadAllToArray exact limit boundary;
- Close idempotence;
- free without explicit Close;
- request-target form validation;
- request factory methods;
- reserved method rejection;
- body replacement ownership;
- response constructor versions;
- response Close/free cleanup;
- ResponseWriter implicit 200 commit;
- double WriteStatus rejection;
- header immutability after commit;
- Finish and destructor/abort paths.

# 43. Required tests — Authentication and digest fields

At minimum:

- every AuthScheme registry constant and case-insensitive registered lookup;
- unknown auth scheme preservation;
- invalid scheme token rejection;
- generic auth with and without data;
- challenge parsing where commas separate parameters versus challenges;
- Basic username colon rejection;
- Basic UTF-8/Base64 round trip;
- invalid Basic Base64 and missing colon;
- Bearer grammar acceptance/rejection;
- credential redaction;
- all RFC 9530 initial digest algorithm constants;
- active/deprecated metadata;
- Content-Digest/Repr-Digest parse/serialize;
- preference 0..10 boundaries;
- duplicate digest algorithm rejection;
- RFC 9651 canonicalization integration.

# 44. Required tests — Redirect, cache, and cookies

At minimum:

- redirect count bound;
- 301/302 POST-to-GET behavior;
- 303 method behavior;
- 307/308 method/body preservation;
- non-replayable redirect failure;
- cross-origin sensitive-header stripping;
- cache disabled default;
- freshness, Vary, validators, revalidation, invalidation, and storage limits;
- RFC 10025 cookie name/value and attributes;
- multiple Set-Cookie field occurrences;
- expiry and Max-Age precedence;
- domain/path matching;
- Secure/HttpOnly/SameSite behavior;
- prefix rules;
- partitioned rules;
- public-suffix rejection;
- cookie jar eviction limits and deterministic StoreAt behavior.

# 45. Required tests — HTTP/1.1

At minimum:

- request/response start lines;
- Content-Length precedence and conflicts;
- chunked coding and trailers;
- persistent connections;
- Host validation;
- malformed whitespace;
- all known smuggling ambiguity cases in RFC 9112 requirements;
- header/body limits during incremental parse.

# 46. Required tests — HTTP/2 and HPACK

At minimum:

- connection preface;
- SETTINGS;
- HEADERS/DATA/CONTINUATION;
- stream-state transitions;
- flow control;
- reset/goaway/ping;
- priority update;
- pseudo-field ordering and uniqueness;
- forbidden connection fields;
- concurrent streams and cancellation;
- HPACK RFC vectors;
- dynamic-table limits;
- decoded-header limits.

# 47. Required tests — HTTP/3 and QPACK

At minimum:

- QUIC/H3 negotiation;
- control/request streams;
- SETTINGS;
- HEADERS/DATA;
- GOAWAY and cancellation;
- pseudo-field validation;
- multiple concurrent request streams;
- QPACK RFC vectors;
- dynamic-table limits;
- blocked-stream limits;
- decoded-header limits;
- no accidental reimplementation of QUIC semantics inside HTTP.

# 48. Required tests — Client, server, and router

At minimum:

- HTTP and HTTPS;
- DNS/network/TLS/QUIC error preservation;
- H1/H2/H3 negotiation according to Protocols;
- connection pooling bounds;
- cookies disabled until jar configured;
- cache disabled by default;
- cancellation at DNS/connect/handshake/write/wait/read phases;
- client Close and free;
- server Serve/Shutdown/Close/free lifecycle;
- handler error preservation;
- request/header/body/concurrency limits;
- route literals, parameters, wildcard, precedence, ambiguity rejection;
- route 404 and 405/Allow behavior;
- HEAD fallback to GET with body suppression.

---

# 49. AI validation rules

An AI validating `stdlib/net/http` against this book must:

1. compare every public declaration against the exact public API in this book;
2. reject duplicate type identifiers in module `http`;
3. reject constructor-shaped `New()` methods when they merely construct the type;
4. require constructors to use `init(...)` and resource destructors to use
   `free()` where specified;
5. verify all Method constants against the table in section 9;
6. verify all AuthScheme constants against section 16;
7. verify Status integer values and special registry metadata against section 10;
8. verify the exact HeaderName convenience set rather than assuming it is the full
   IANA registry;
9. reject placeholder public comments, TODO public behavior, and undocumented
   public declarations;
10. check that doc comments contain the structured sections required by section 2;
11. verify that move-only behavior is not weakened for Body-containing aggregate
    types;
12. verify that `free()` releases owned HTTP resources without inventing a
    propagating destructor error;
13. preserve underlying causes in error unions;
14. distinguish registry representation from scheme-specific high-level support;
15. treat explicit scope exclusions as exclusions, not as missing implementation.

---

# 50. Recommended implementation order

1. `results.sec` and source headers;
2. `version.sec`;
3. `methods.sec` registry sync;
4. `status.sec` registry sync and sentinel removal;
5. `header.sec` hardening;
6. `structured.sec`;
7. `body.sec`;
8. `request.sec`;
9. `response.sec` and response writer;
10. `auth.sec`;
11. `digest.sec`;
12. `priority.sec`;
13. `cookie.sec` and `cookie_parse.sec`;
14. `cookie_jar.sec`;
15. `http1.sec`;
16. `client.sec` and private transport dispatch;
17. `server.sec`;
18. `router.sec`;
19. redirect and cache;
20. HPACK then HTTP/2;
21. QPACK then HTTP/3;
22. remove migrated `encode.sec`, `websockets.sec`, and non-normative
    `cookie_security.sec` responsibility;
23. run the complete test matrix and synchronize
    `implementation-status-std-net-http.yaml`.

---

# 51. Revision summary

Revision 0.2 is a structural rewrite of the HTTP book. It replaces suggestion-like
API fragments with an explicit implementation contract. In particular it fixes
the duplicate `Version` type design, removes pseudo-constructors named `New`,
defines deterministic destruction responsibilities, provides the complete current
IANA authentication-scheme constants, provides the complete current IANA method
constant set, makes the Status default valid under current Sec 0.1 rules without
using a fake protocol sentinel, aligns 418 with the current IANA `Unused` entry,
and makes Auth a concrete usable API instead of a
three-name sketch.
