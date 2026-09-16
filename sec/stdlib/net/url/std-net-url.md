# Sec Standard Library — `net/url`

- **Status:** Draft — normative stdlib implementation specification
- **Created:** 2026-09-15
- **Last updated:** 2026-09-15
- **Document revision:** 0.1
- **Sec language version:** 0.1
- **Canonical path:** `stdlib/net/url/std-net-url.md`
- **Repository path:** `sec/stdlib/net/url/std-net-url.md`
- **Parent specification:** `stdlib/net/std-net.md`
- **Implementation-status fragment:** `implementation-status-std-net-url.yaml`
- **Standards and registries rechecked:** 2026-09-15

---

# 1. Purpose and authority

This book is the normative implementation specification for Sec's `net/url`
standard-library package.

A conforming implementation must be implementable from this book without needing
to infer missing public declarations, invent error behavior, guess normalization
rules, or consult examples to discover API members.

The package is imported as:

```sec
import "net/url"
```

and is referenced as `url` in source code.

`net/url` owns generic URL/URI-reference syntax, percent encoding, relative
reference resolution, authority parsing, URL paths, generic query components,
fragments, and UTF-8 `application/x-www-form-urlencoded` query parameters.

It does not own HTTP request-target rules, DNS resolution, socket addressing,
filesystem path semantics, browser navigation policy, redirects, cookies, or TLS.

## 1.1 Completeness rule

Every public type, enum member, union alternative, associated immutable, property,
function, method, initializer, and destructor owned by this revision is listed in
this book.

Normative API sections must not use phrases such as:

```text
may include
for example, methods such as
and similar
etc.
```

where those phrases would leave the public implementation surface undefined.

Private helpers may be added freely provided they do not alter observable public
semantics.

## 1.2 Sec declaration rules used by this book

The following language rules are assumed throughout:

- one type identifier denotes one type in a module; types are not overloadable;
- functions and methods may overload where Sec overload resolution permits it;
- constructors use `init(...)`, never `New()`;
- an infallible `init(...)` has no `fn` keyword and no return type;
- a fallible initializer writes its error type after the parameter list;
- an empty initializer is not written merely to provide a default constructor;
- destructors use `free()` when custom destruction is required;
- a consuming parameter is written `->name: Type`;
- a read-only borrowed array parameter is written `name: ref T[]`;
- this package does not require custom `free()` because all public values are
  ordinary owned Sec values without external resource handles.

## 1.3 Representation rationale is normative

Every public enum, struct, union, named primitive type, and error union has a
**Representation rationale**.

The rationale is part of the design contract and exists so a human or AI reviewer
can verify that an implementation has not silently changed the semantic model.

---

# 2. Standards model

## 2.1 Primary syntax standard

`net/url` uses **RFC 3986 — Uniform Resource Identifier (URI): Generic Syntax** as
its primary generic parsing, recomposition, normalization, and reference-resolution
standard.

Canonical source:

- RFC 3986 / STD 66 — https://www.rfc-editor.org/rfc/rfc3986

The package deliberately exposes the user-facing term `URL` because that is the
common programming term and is already used by Sec networking APIs. The generic
syntax implemented by the package is nevertheless the RFC 3986 URI syntax.

## 2.2 RFC 3986 updates

The implementation must also account for:

- RFC 7320 — URI Design and Ownership — https://www.rfc-editor.org/rfc/rfc7320
- RFC 8820 — URI Design and Ownership — https://www.rfc-editor.org/rfc/rfc8820

These documents do not replace the generic parser but constrain assumptions about
scheme ownership and scheme-specific interpretation.

## 2.3 IPv6 textual form

When `net/url` serializes an IPv6 address literal, the address text must use the
canonical form recommended by:

- RFC 5952 — A Recommendation for IPv6 Address Text Representation —
  https://www.rfc-editor.org/rfc/rfc5952

The surrounding square brackets are URL authority syntax and are added by
`Host.String()` / URL serialization, not considered part of the IPv6 address text.

## 2.4 IPv6 zone identifiers

RFC 6874 is obsolete. RFC 9844 explicitly obsoletes RFC 6874 and reverts its
change to RFC 3986 URI syntax.

Therefore this revision does **not** accept an IPv6 zone identifier embedded in a
URL IP-literal host.

Relevant source:

- RFC 9844 — Entering IPv6 Zone Identifiers in User Interfaces —
  https://www.rfc-editor.org/rfc/rfc9844

An application that needs a local scoped IPv6 address with a zone identifier must
handle that at the socket/address/UI layer rather than encode the zone identifier
as generic RFC 3986 URL host syntax.

## 2.5 WHATWG URL Standard boundary

The WHATWG URL Standard is **not** the parser contract for `Parse()` or
`ParseReference()`.

`url.Parse()` is intentionally strict and does not perform browser-style input
repair, special-scheme state-machine behavior, legacy IPv4 interpretation, or
implicit backslash-to-slash rewriting.

The WHATWG URL Standard is used by this package only for the explicitly named
UTF-8 `application/x-www-form-urlencoded` behavior of `QueryParams`.

Source:

- WHATWG URL Standard — https://url.spec.whatwg.org/

## 2.6 Internationalized resource identifiers

RFC 3987 defines IRIs:

- RFC 3987 — Internationalized Resource Identifiers —
  https://www.rfc-editor.org/rfc/rfc3987

IRI parsing and IDNA/UTS #46 hostname conversion are **not public API in revision
0.1** of this package. This exclusion is deliberate. A future revision may add an
explicit IRI API, but strict `Parse()` must not silently become an IRI or browser
parser.

## 2.7 URI scheme registry

Scheme names are extensible. The source of registered names is:

- IANA Uniform Resource Identifier (URI) Schemes registry —
  https://www.iana.org/assignments/uri-schemes/uri-schemes.xhtml

`Scheme` is therefore an open nominal string type, not an enum.

---

# 3. Semantic model

## 3.1 Absolute URL versus reference

The package distinguishes:

```text
URL        = absolute identifier with a required scheme
Reference  = RFC 3986 URI-reference, which may be absolute or relative
```

This distinction is intentional.

A relative reference such as:

```text
../images/logo.png?size=2#top
```

is not represented as an invalid or partially initialized `URL`. It is a valid
`Reference`.

`URL.Resolve(reference)` performs RFC 3986 section 5 reference resolution and
returns an absolute `URL`.

## 3.2 Encoded components stay encoded

`Path`, `Query`, `Fragment`, and `UserInfo` represent their RFC 3986 encoded
component form.

The package must not eagerly percent-decode path data because decoding `%2F` to
`/` before path segmentation would destroy information.

Example:

```text
/a%2Fb/c
```

contains two path separators, not three.

Decoding is therefore explicit through component methods and percent-decoding
functions.

## 3.3 Absent and present-empty query/fragment differ

These are observably different serializations:

```text
https://example.test/path
https://example.test/path?
https://example.test/path#
```

Accordingly:

```text
Query    = None        means no `?` delimiter
Query    = Some("")    means `?` is present with an empty query
Fragment = None        means no `#` delimiter
Fragment = Some("")    means `#` is present with an empty fragment
```

The implementation must preserve these distinctions.

## 3.4 Authority presence differs from empty authority

RFC 3986 distinguishes no authority component from an authority component whose
host is empty.

This matters for forms such as:

```text
file:/path
file:///path
```

`Option[Authority]` therefore represents authority presence explicitly.

---

# 4. Documentation contract for source and LSP

Every public declaration in this package must have structured documentation that
can be consumed by the Sec LSP for hover and generated documentation.

The canonical function/method form is:

```sec
/**
 * One-sentence summary in present tense.
 *
 * Description:
 *   Additional observable semantics.
 *
 * Parameters:
 *   - name: Exact meaning and accepted domain.
 *
 * Returns:
 *   Exact success result and allocation/ownership behavior where relevant.
 *
 * Errors:
 *   - ErrorType.Variant: Exact failure condition.
 *
 * Ownership:
 *   Borrow/move/returned ownership information when relevant.
 *
 * Standards:
 *   - RFC/standard section when directly applicable.
 */
```

Rules:

1. The first line must work as concise hover text.
2. `Parameters:` must name every parameter exactly once.
3. `Returns:` is mandatory for non-`void` APIs.
4. `Errors:` is mandatory for fallible APIs and lists every directly returned
   error alternative at that abstraction level.
5. `Ownership:` is mandatory for borrowed arrays, consuming parameters, returned
   owned collections, or retained values.
6. `Standards:` is mandatory when a declaration directly implements a standard
   algorithm or grammar element.
7. Comments describe implemented behavior, never planned behavior.
8. TODO markers are forbidden in public API documentation of a conforming
   implementation.

---

# 5. Canonical file manifest

```text
stdlib/net/url/
├── std-net-url.md
├── errors.sec
├── scheme.sec
├── percent.sec
├── host.sec
├── authority.sec
├── path.sec
├── query.sec
├── reference.sec
└── url.sec
```

All `.sec` files declare:

```sec
module url
```

The directory is one Sec module; file boundaries are organizational only.

---

# 6. Required source-file headers

Every implementation file starts with `module url` followed by the file header
specified below. The purpose and Standards list may be expanded only when the file
actually implements the additional referenced standard.

## 6.1 `errors.sec`

```sec
module url

/*
 * Sec Standard Library - net/url - Error model
 *
 * File:        stdlib/net/url/errors.sec
 * Module:      url
 * Revision:    1
 * Updated:     2026-09-15
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Error types used by strict URL parsing, percent decoding, port conversion,
 *   query parameter decoding, and validated URL construction.
 *
 * Standards:
 *   - RFC 3986
 *     https://www.rfc-editor.org/rfc/rfc3986
 *
 * Rulebook:
 *   stdlib/net/url/std-net-url.md
 */
```

## 6.2 `scheme.sec`

```sec
module url

/*
 * Sec Standard Library - net/url - URI schemes
 *
 * File:        stdlib/net/url/scheme.sec
 * Module:      url
 * Revision:    1
 * Updated:     2026-09-15
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Scheme syntax, normalization, validation, and standard convenience values.
 *
 * Standards:
 *   - RFC 3986 section 3.1
 *     https://www.rfc-editor.org/rfc/rfc3986#section-3.1
 *   - IANA URI Schemes registry
 *     https://www.iana.org/assignments/uri-schemes/uri-schemes.xhtml
 *
 * Rulebook:
 *   stdlib/net/url/std-net-url.md
 */
```

## 6.3 `percent.sec`

```sec
module url

/*
 * Sec Standard Library - net/url - Percent encoding
 *
 * File:        stdlib/net/url/percent.sec
 * Module:      url
 * Revision:    1
 * Updated:     2026-09-15
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Component-aware UTF-8 percent encoding, byte percent encoding, decoding,
 *   validation, and RFC 3986 percent normalization.
 *
 * Standards:
 *   - RFC 3986 sections 2.1-2.4 and 6.2.2.2
 *     https://www.rfc-editor.org/rfc/rfc3986
 *
 * Rulebook:
 *   stdlib/net/url/std-net-url.md
 */
```

## 6.4 `host.sec`

```sec
module url

/*
 * Sec Standard Library - net/url - Host syntax
 *
 * File:        stdlib/net/url/host.sec
 * Module:      url
 * Revision:    1
 * Updated:     2026-09-15
 * Author:      Jonas Engström
 *
 * Purpose:
 *   RFC 3986 host parsing and canonical serialization for registered names,
 *   IPv4 address literals, IPv6 address literals, and IPvFuture literals.
 *
 * Standards:
 *   - RFC 3986 section 3.2.2
 *     https://www.rfc-editor.org/rfc/rfc3986#section-3.2.2
 *   - RFC 5952
 *     https://www.rfc-editor.org/rfc/rfc5952
 *   - RFC 9844
 *     https://www.rfc-editor.org/rfc/rfc9844
 *
 * Rulebook:
 *   stdlib/net/url/std-net-url.md
 */
```

## 6.5 `authority.sec`

```sec
module url

/*
 * Sec Standard Library - net/url - Authority and user information
 *
 * File:        stdlib/net/url/authority.sec
 * Module:      url
 * Revision:    1
 * Updated:     2026-09-15
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Authority, userinfo, and port parsing and serialization.
 *
 * Standards:
 *   - RFC 3986 section 3.2
 *     https://www.rfc-editor.org/rfc/rfc3986#section-3.2
 *
 * Rulebook:
 *   stdlib/net/url/std-net-url.md
 */
```

## 6.6 `path.sec`

```sec
module url

/*
 * Sec Standard Library - net/url - Path components
 *
 * File:        stdlib/net/url/path.sec
 * Module:      url
 * Revision:    1
 * Updated:     2026-09-15
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Encoded URL path validation, segment handling, percent normalization,
 *   and dot-segment removal.
 *
 * Standards:
 *   - RFC 3986 sections 3.3, 5.2.4 and 6.2.2
 *     https://www.rfc-editor.org/rfc/rfc3986
 *
 * Rulebook:
 *   stdlib/net/url/std-net-url.md
 */
```

## 6.7 `query.sec`

```sec
module url

/*
 * Sec Standard Library - net/url - Query and query parameters
 *
 * File:        stdlib/net/url/query.sec
 * Module:      url
 * Revision:    1
 * Updated:     2026-09-15
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Generic RFC 3986 query components and ordered UTF-8
 *   application/x-www-form-urlencoded query parameters.
 *
 * Standards:
 *   - RFC 3986 section 3.4
 *     https://www.rfc-editor.org/rfc/rfc3986#section-3.4
 *   - WHATWG URL Standard, application/x-www-form-urlencoded
 *     https://url.spec.whatwg.org/
 *
 * Rulebook:
 *   stdlib/net/url/std-net-url.md
 */
```

## 6.8 `reference.sec`

```sec
module url

/*
 * Sec Standard Library - net/url - URI references
 *
 * File:        stdlib/net/url/reference.sec
 * Module:      url
 * Revision:    1
 * Updated:     2026-09-15
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Absolute and relative RFC 3986 URI-reference values.
 *
 * Standards:
 *   - RFC 3986 sections 4 and 5
 *     https://www.rfc-editor.org/rfc/rfc3986
 *
 * Rulebook:
 *   stdlib/net/url/std-net-url.md
 */
```

## 6.9 `url.sec`

```sec
module url

/*
 * Sec Standard Library - net/url - Absolute URL values and resolution
 *
 * File:        stdlib/net/url/url.sec
 * Module:      url
 * Revision:    1
 * Updated:     2026-09-15
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Strict absolute URL parsing, validated construction, serialization,
 *   syntax normalization, and relative-reference resolution.
 *
 * Standards:
 *   - RFC 3986
 *     https://www.rfc-editor.org/rfc/rfc3986
 *
 * Rulebook:
 *   stdlib/net/url/std-net-url.md
 */
```

---

# 7. Error model — `errors.sec`

## 7.1 `ParseError`

```sec
type ParseError union error {
    EmptyInput,
    MissingScheme,
    RelativeReference,
    InvalidCharacter(uint),
    InvalidScheme(uint),
    InvalidAuthority(uint),
    InvalidUserInfo(uint),
    InvalidHost(uint),
    InvalidPort(uint),
    InvalidPath(uint),
    InvalidQuery(uint),
    InvalidFragment(uint),
    InvalidPercentEncoding(uint),
}
```

**Representation rationale:** `ParseError` is a `union error` rather than an enum
because several alternatives carry the source byte offset at which parsing failed.
The payload is useful for diagnostics and LSP tooling and must not be discarded.

Offsets are zero-based UTF-8 byte offsets into the original input string.

`MissingScheme` is returned by absolute `Parse()` when the input is syntactically
a relative reference.

`RelativeReference` is returned only by conversion of an already parsed
`Reference` to `URL` when the reference has no scheme.

## 7.2 `BuildError`

```sec
enum BuildError error {
    MissingScheme,
    InvalidScheme,
    InvalidAuthority,
    InvalidPath,
    InvalidQuery,
    InvalidFragment,
    AuthorityPathMustBeAbsoluteOrEmpty,
    PathWithoutAuthorityCannotStartWithDoubleSlash,
}
```

**Representation rationale:** construction failures are a closed set without
additional payload, so an error enum is the exact representation.

## 7.3 `PercentError`

```sec
type PercentError union error {
    InvalidEscape(usize),
    InvalidUtf8(usize),
}
```

**Representation rationale:** both failures need an exact byte offset, so this is
an error union.

## 7.4 `PortError`

```sec
type PortError union error {
    InvalidDigit(usize),
    OutOfRange(string),
}
```

**Representation rationale:** conversion of a generic RFC port string to
`uint16` can fail either at a particular invalid character or because the complete
numeric spelling exceeds the target range. The original spelling is retained for
`OutOfRange` diagnostics.

## 7.5 `QueryParamsError`

```sec
type QueryParamsError union error {
    InvalidEscape(usize),
    InvalidUtf8(usize),
}
```

**Representation rationale:** form decoding must report the byte location of the
first malformed escape or malformed UTF-8 sequence.

---

# 8. Scheme — `scheme.sec`

## 8.1 Declaration

```sec
type Scheme string
```

**Representation rationale:** URI schemes form an open IANA-managed namespace.
Applications can define and use schemes that the Sec standard library does not
know. An enum would incorrectly make the namespace closed.

Scheme comparison is ASCII case-insensitive by RFC 3986. `Scheme.Parse()` and URL
parsing store the canonical lowercase spelling.

Direct explicit conversion to `Scheme` does not imply validation. APIs that
construct a `URL` validate the scheme before accepting it.

## 8.2 Complete `Scheme` implementation surface

```sec
impl Scheme {
    static let Http: Scheme := "http"
    static let Https: Scheme := "https"
    static let Ws: Scheme := "ws"
    static let Wss: Scheme := "wss"
    static let Ftp: Scheme := "ftp"
    static let File: Scheme := "file"
    static let Mailto: Scheme := "mailto"
    static let Data: Scheme := "data"
    static let Urn: Scheme := "urn"

    /**
     * Parses and canonicalizes one RFC 3986 scheme name.
     *
     * Parameters:
     *   - value: Scheme spelling without the trailing colon.
     *
     * Returns:
     *   A lowercase validated Scheme.
     *
     * Errors:
     *   - ParseError.InvalidScheme: The spelling does not match RFC 3986 scheme syntax.
     *
     * Standards:
     *   - RFC 3986 section 3.1.
     */
    static fn Parse(value: string) Result[Scheme, ParseError]

    /** Returns true when the value matches RFC 3986 scheme syntax. */
    fn IsValid() bool

    /** Returns true for the canonical `http` or `https` scheme. */
    property IsHttp: bool { get }

    /** Returns true for the canonical `ws` or `wss` scheme. */
    property IsWebSocket: bool { get }

    /** Returns true for schemes normally associated with TLS by Sec networking APIs. */
    property IsSecureTransportScheme: bool { get }
}
```

The nine associated values above are an exact Sec convenience set for revision
0.1. They are **not** an exhaustive mirror of the IANA registry and must not be
described as such.

`IsSecureTransportScheme` is true exactly for:

```text
https
wss
```

It must not guess security properties for arbitrary schemes.

---

# 9. Percent encoding — `percent.sec`

## 9.1 `Component`

```sec
enum Component {
    UserInfo,
    RegisteredName,
    Path,
    PathSegment,
    Query,
    Fragment,
}
```

**Representation rationale:** the package supports a closed set of RFC 3986
generic component encoding contexts. Each context has a defined allowed-character
set; arbitrary user-defined contexts belong in application code.

## 9.2 Public functions

```sec
/**
 * Percent-encodes UTF-8 text for one RFC 3986 component context.
 *
 * Parameters:
 *   - value: Decoded Unicode text to encode as UTF-8 bytes.
 *   - component: The grammar context whose allowed characters remain literal.
 *
 * Returns:
 *   Newly allocated encoded text using uppercase hexadecimal digits.
 *
 * Standards:
 *   - RFC 3986 sections 2.1-2.3 and 3.
 */
fn PercentEncode(value: string, component: Component) string

/**
 * Percent-encodes arbitrary bytes for one RFC 3986 component context.
 *
 * Parameters:
 *   - value: Borrowed bytes. The function does not retain the array.
 *   - component: The grammar context whose ASCII-safe bytes remain literal.
 *
 * Returns:
 *   Newly allocated encoded text using uppercase hexadecimal digits.
 *
 * Ownership:
 *   `value` is borrowed and remains owned by the caller.
 */
fn PercentEncode(value: ref byte[], component: Component) string

/**
 * Percent-decodes encoded text and validates the decoded bytes as UTF-8.
 *
 * Parameters:
 *   - value: Encoded component text.
 *
 * Returns:
 *   Newly allocated decoded string.
 *
 * Errors:
 *   - PercentError.InvalidEscape: A percent sign is not followed by two hex digits.
 *   - PercentError.InvalidUtf8: The decoded byte sequence is not valid UTF-8.
 */
fn PercentDecode(value: string) Result[string, PercentError]

/**
 * Percent-decodes encoded text without imposing UTF-8 validity on the result.
 *
 * Parameters:
 *   - value: Encoded component text.
 *
 * Returns:
 *   A newly allocated owning byte array.
 *
 * Errors:
 *   - PercentError.InvalidEscape: A percent sign is not followed by two hex digits.
 */
fn PercentDecodeBytes(value: string) Result[byte[], PercentError]

/**
 * Validates every percent escape without decoding the input.
 *
 * Parameters:
 *   - value: Encoded text to inspect.
 *
 * Returns:
 *   Ok() when every percent sign begins a valid two-hex-digit escape.
 *
 * Errors:
 *   - PercentError.InvalidEscape: The first malformed escape and its byte offset.
 */
fn ValidatePercentEncoding(value: string) Result[void, PercentError]

/**
 * Canonicalizes RFC 3986 percent encoding.
 *
 * Description:
 *   Hexadecimal digits in escapes become uppercase. Escapes for unreserved ASCII
 *   characters are decoded to their literal character. Reserved characters remain
 *   escaped when they were escaped in the input.
 *
 * Parameters:
 *   - value: Encoded text.
 *
 * Returns:
 *   Newly allocated normalized text.
 *
 * Errors:
 *   - PercentError.InvalidEscape: The input contains a malformed escape.
 *
 * Standards:
 *   - RFC 3986 section 6.2.2.2.
 */
fn NormalizePercentEncoding(value: string) Result[string, PercentError]
```

`PercentEncode()` treats its input as decoded data. A literal `%` in the input is
therefore encoded as `%25`; the function must not interpret pre-existing `%HH`
spelling as already encoded input.

---

# 10. Host — `host.sec`

## 10.1 `HostKind`

```sec
enum HostKind {
    RegisteredName,
    IPv4,
    IPv6,
    IPvFuture,
}
```

**Representation rationale:** RFC 3986 defines exactly these semantic host forms
for generic syntax. A closed enum is appropriate for classification.

An empty host is represented as `RegisteredName` with an empty value.

## 10.2 `Host`

```sec
type Host struct {
    _value: string,
    _kind: HostKind,
}
```

**Representation rationale:** `Host` has two simultaneously meaningful pieces of
state: canonical textual value and parsed host kind. A struct lets the parser
validate the pair once and prevents public construction of inconsistent
kind/value combinations because both fields are private.

For IPv6 and IPvFuture, `_value` excludes surrounding `[` and `]`.

## 10.3 Complete `Host` implementation surface

```sec
impl Host {
    /**
     * Parses one RFC 3986 host component.
     *
     * Parameters:
     *   - value: Serialized host component, including brackets for IP-literals.
     *
     * Returns:
     *   A validated Host in canonical textual form.
     *
     * Errors:
     *   - ParseError.InvalidHost: Host syntax is invalid.
     *   - ParseError.InvalidPercentEncoding: A registered-name escape is malformed.
     *
     * Standards:
     *   - RFC 3986 section 3.2.2.
     *   - RFC 5952 for serialized IPv6 text.
     */
    static fn Parse(value: string) Result[Host, ParseError]

    /** Returns the parsed host category. */
    property Kind: HostKind { get }

    /** Returns the canonical host value without IPv6/IPvFuture square brackets. */
    property Value: string { get }

    /** Returns true when this is the RFC-valid empty registered-name host. */
    property IsEmpty: bool { get }

    /** Returns true for IPv4 or IPv6 host literals. */
    property IsIP: bool { get }

    /** Returns true for a registered-name host. */
    property IsRegisteredName: bool { get }

    /**
     * Serializes the host component.
     *
     * Returns:
     *   Registered names and IPv4 addresses directly; IPv6 and IPvFuture inside
     *   square brackets.
     */
    fn String() string
}
```

### 10.4 Host parsing rules

The parser must:

- accept RFC 3986 IPv4 dotted-decimal syntax only;
- reject browser-style legacy IPv4 forms such as hexadecimal, octal, shortened,
  or one-integer IPv4 spellings;
- accept every valid RFC 3986 / RFC 4291 IPv6 spelling;
- serialize accepted IPv6 using RFC 5952 canonical form;
- accept syntactically valid IPvFuture literals;
- accept registered names made from unreserved characters, sub-delimiters, and
  valid percent escapes;
- canonicalize ASCII letters in registered names to lowercase;
- normalize percent escapes according to `NormalizePercentEncoding()`;
- reject an IPv6 zone identifier in URL host syntax in this revision.

No DNS lookup occurs during host parsing.

---

# 11. Authority — `authority.sec`

## 11.1 `UserInfo`

```sec
type UserInfo string
```

**Representation rationale:** RFC 3986 userinfo is an open encoded component. It
is not generically defined as a username/password pair, so a struct with
`Username` and `Password` would impose scheme-specific semantics that RFC 3986
does not define.

```sec
impl UserInfo {
    /** Parses and validates encoded RFC 3986 userinfo. */
    static fn Parse(value: string) Result[UserInfo, ParseError]

    /** Percent-encodes decoded UTF-8 userinfo text. */
    static fn FromDecoded(value: string) UserInfo

    /** Decodes percent escapes and validates UTF-8. */
    fn Decode() Result[string, PercentError]
}
```

## 11.2 `Port`

```sec
type Port string
```

**Representation rationale:** RFC 3986 defines `port = *DIGIT`; generic URL
syntax therefore does not limit the value to 0..65535 and even permits the
present-but-empty form after `:`. A nominal string preserves generic syntax.

```sec
impl Port {
    /** Parses RFC 3986 generic port syntax, including the empty port spelling. */
    static fn Parse(value: string) Result[Port, ParseError]

    /** Creates a decimal port spelling from a network-compatible 16-bit port. */
    static fn FromNumber(value: uint16) Port

    /** Returns true when an explicit colon was followed by no port digits. */
    property IsEmpty: bool { get }

    /**
     * Converts a non-empty generic port to a 16-bit network port.
     *
     * Returns:
     *   The numeric port when it fits uint16.
     *
     * Errors:
     *   - PortError.InvalidDigit: The value contains a non-decimal digit.
     *   - PortError.OutOfRange: The value is empty or exceeds 65535.
     */
    fn Number() Result[uint16, PortError]
}
```

## 11.3 `Authority`

```sec
type Authority struct {
    UserInfo: Option[UserInfo],
    Host: Host,
    Port: Option[Port],
}
```

**Representation rationale:** userinfo, host, and port are simultaneously
meaningful subcomponents of one authority. A struct represents that product
shape directly. `Option[Port]` preserves the difference between no colon and an
explicit port; `Some(Port(""))` preserves an explicit empty port.

```sec
impl Authority {
    init(host: Host)

    init(host: Host, port: Port)

    init(userInfo: UserInfo, host: Host, port: Option[Port])

    /** Parses an authority without the leading `//`. */
    static fn Parse(value: string) Result[Authority, ParseError]

    /** Serializes userinfo, host, and optional port without leading `//`. */
    fn String() string

    /**
     * Serializes an authority for diagnostics without exposing userinfo content.
     *
     * Description:
     *   When UserInfo is present, the serialized userinfo is replaced by the
     *   literal `REDACTED` before the `@` delimiter.
     */
    fn Redacted() string
}
```

The parser splits userinfo at the **last** literal `@` delimiter permitted by the
authority grammar. An encoded `%40` is data and is never treated as a delimiter.

For an IP-literal host, a port delimiter may occur only after the closing `]`.

---

# 12. Path — `path.sec`

## 12.1 Declaration

```sec
type Path string
```

**Representation rationale:** a generic RFC 3986 path is an encoded sequence of
segments separated by `/`. It is not a filesystem path and must retain encoded
reserved characters exactly enough to preserve segment boundaries.

## 12.2 Complete `Path` implementation surface

```sec
impl Path {
    /** Parses and validates one encoded RFC 3986 path component. */
    static fn Parse(value: string) Result[Path, ParseError]

    /**
     * Encodes decoded UTF-8 path text while retaining `/` as path separators.
     */
    static fn FromDecoded(value: string) Path

    /** Returns true when the encoded path begins with `/`. */
    property IsAbsolute: bool { get }

    /** Returns true when the encoded path is empty. */
    property IsEmpty: bool { get }

    /** Returns true when a non-empty path ends with `/`. */
    property HasTrailingSlash: bool { get }

    /** Decodes the complete path after preserving its already-parsed boundaries. */
    fn Decode() Result[string, PercentError]

    /**
     * Returns decoded path segments in source order.
     *
     * Description:
     *   The encoded path is split on literal `/` before each segment is percent-decoded.
     *   Therefore `%2F` remains part of a segment and never becomes a separator.
     *
     * Returns:
     *   A newly allocated owning array of decoded segment strings.
     */
    fn Segments() Result[string[], PercentError]

    /**
     * Applies the RFC 3986 remove_dot_segments algorithm.
     *
     * Returns:
     *   A new Path with complete literal `.` and `..` segments removed according
     *   to RFC 3986 section 5.2.4.
     */
    fn RemoveDotSegments() Path

    /**
     * Normalizes percent escapes and removes dot segments.
     *
     * Returns:
     *   A new syntactically normalized Path.
     */
    fn Normalize() Path

    /**
     * Appends one decoded path segment.
     *
     * Parameters:
     *   - segment: Decoded UTF-8 segment data. `/` in the value is encoded as `%2F`.
     *
     * Returns:
     *   A new Path with exactly one additional segment.
     */
    fn AppendSegment(segment: string) Path
}
```

`RemoveDotSegments()` interprets only complete literal dot-segments as structural
syntax. Percent-encoded spellings are first subject to normal RFC percent
normalization when `Normalize()` is called.

`Path` never interprets `\` as `/`.

---

# 13. Generic query and form query parameters — `query.sec`

## 13.1 `Query`

```sec
type Query string
```

**Representation rationale:** the generic RFC 3986 query component is opaque to
the generic URL layer beyond its character grammar. Treating every query as a map
would incorrectly impose form semantics and lose duplicate names and ordering.

```sec
impl Query {
    /** Parses and validates one encoded RFC 3986 query component without `?`. */
    static fn Parse(value: string) Result[Query, ParseError]

    /** Percent-encodes decoded UTF-8 query text using the generic query context. */
    static fn FromDecoded(value: string) Query

    /** Decodes generic query percent escapes as UTF-8. `+` remains `+`. */
    fn Decode() Result[string, PercentError]

    /** Returns a query with canonical RFC 3986 percent spelling. */
    fn Normalize() Query
}
```

A generic `Query` never treats `+` as space.

## 13.2 `Fragment`

```sec
type Fragment string
```

**Representation rationale:** fragment syntax is an encoded open component whose
meaning belongs to the media type or scheme/application using the URL.

```sec
impl Fragment {
    /** Parses and validates one encoded RFC 3986 fragment without `#`. */
    static fn Parse(value: string) Result[Fragment, ParseError]

    /** Percent-encodes decoded UTF-8 fragment text. */
    static fn FromDecoded(value: string) Fragment

    /** Decodes fragment percent escapes as UTF-8. */
    fn Decode() Result[string, PercentError]

    /** Returns a fragment with canonical RFC 3986 percent spelling. */
    fn Normalize() Fragment
}
```

## 13.3 `QueryParameter`

```sec
type QueryParameter struct {
    Name: string,
    Value: string,
}
```

**Representation rationale:** one form query parameter always has two
simultaneously meaningful decoded strings. A struct is the exact product shape.

## 13.4 `QueryParams`

```sec
type QueryParams struct {
    Items: QueryParameter[],
}
```

**Representation rationale:** query parameters are represented as an ordered
sequence rather than a map because duplicate names are legal and source order is
observable. The public array also makes the complete representation available
without hidden iteration behavior.

## 13.5 Complete `QueryParams` implementation surface

```sec
impl QueryParams {
    init(->items: QueryParameter[])

    /**
     * Parses UTF-8 application/x-www-form-urlencoded query data.
     *
     * Parameters:
     *   - value: Form-encoded data without the leading `?`.
     *
     * Returns:
     *   Parameters in source order, retaining duplicate names.
     *
     * Errors:
     *   - QueryParamsError.InvalidEscape: A percent escape is malformed.
     *   - QueryParamsError.InvalidUtf8: Decoded bytes are not valid UTF-8.
     *
     * Standards:
     *   - WHATWG URL Standard, application/x-www-form-urlencoded parser.
     */
    static fn Parse(value: string) Result[QueryParams, QueryParamsError]

    /** Returns the number of parameter pairs, including duplicate names. */
    property Count: usize { get }

    /** Returns true when there are no parameter pairs. */
    property IsEmpty: bool { get }

    /** Appends one decoded name/value pair without removing duplicates. */
    fn Add(name: string, value: string) void

    /**
     * Replaces every pair with the given name by one pair.
     *
     * Description:
     *   If the name already exists, the replacement occupies the position of the
     *   first removed pair. If it does not exist, the new pair is appended.
     */
    fn Set(name: string, value: string) void

    /** Returns the first value for `name`, or None when absent. */
    fn Get(name: string) Option[string]

    /** Returns all values for `name` in source order as a new owning array. */
    fn GetAll(name: string) string[]

    /** Returns true when at least one pair has the given name. */
    fn Has(name: string) bool

    /** Removes every pair with `name` and returns the number removed. */
    fn Remove(name: string) usize

    /** Removes all pairs. */
    fn Clear() void

    /**
     * Encodes the ordered parameter list as UTF-8 application/x-www-form-urlencoded.
     *
     * Returns:
     *   Newly allocated encoded data without a leading `?`.
     */
    fn Encode() string

    /** Returns a new owning array containing all pairs in order. */
    fn ToArray() QueryParameter[]
}
```

### 13.6 Form parsing and encoding rules

`QueryParams.Parse()` and `Encode()` follow these exact rules:

- pairs are separated by `&`;
- each pair splits on the first `=` only;
- a pair without `=` has an empty value;
- an empty name is valid;
- an empty value is valid;
- duplicate names are retained;
- pair order is retained;
- during decoding, `+` becomes U+0020 SPACE before percent decoding;
- percent-decoded bytes must form valid UTF-8;
- during encoding, U+0020 SPACE becomes `+`;
- form percent encoding uses the WHATWG
  `application/x-www-form-urlencoded` percent-encode set;
- a literal `+` in decoded data is encoded as `%2B`;
- no semicolon separator extension is accepted.

`QueryParams` is not implicitly substituted for `Query`. Conversion to a generic
query is explicit:

```sec
let encoded := params.Encode()
let query := Query(encoded)
```

---

# 14. Reference — `reference.sec`

## 14.1 Declaration

```sec
type Reference struct {
    _scheme: Option[Scheme],
    _authority: Option[Authority],
    _path: Path,
    _query: Option[Query],
    _fragment: Option[Fragment],
}
```

**Representation rationale:** a URI-reference contains five simultaneously
meaningful components, but scheme and authority may be absent. A struct matches
that product shape. Fields are private so the RFC path/authority structural
invariants cannot be bypassed by a public struct literal.

## 14.2 Construction and API

```sec
impl Reference {
    init(
        scheme: Option[Scheme],
        authority: Option[Authority],
        path: Path,
        query: Option[Query],
        fragment: Option[Fragment],
    ) BuildError

    /** Returns the optional scheme component. */
    property Scheme: Option[Scheme] { get }

    /** Returns the optional authority component. */
    property Authority: Option[Authority] { get }

    /** Returns the encoded path component. */
    property Path: Path { get }

    /** Returns None for no `?`, or Some including the present-empty query. */
    property Query: Option[Query] { get }

    /** Returns None for no `#`, or Some including the present-empty fragment. */
    property Fragment: Option[Fragment] { get }

    /** Returns true when the reference contains a scheme. */
    property IsAbsolute: bool { get }

    /** Returns true when the reference has no scheme. */
    property IsRelative: bool { get }

    /** Returns true when an authority component is present. */
    property HasAuthority: bool { get }

    /** Serializes the reference according to RFC 3986 component recomposition. */
    fn String() string

    /** Serializes the reference while redacting any authority userinfo. */
    fn Redacted() string

    /**
     * Converts an absolute reference to URL.
     *
     * Errors:
     *   - ParseError.RelativeReference: The reference has no scheme.
     */
    fn ToURL() Result[URL, ParseError]

    /** Returns a syntactically normalized reference. */
    fn Normalize() Reference

    /** Returns a copy with the supplied fragment. */
    fn WithFragment(fragment: Option[Fragment]) Reference

    /** Returns a copy with no fragment delimiter. */
    fn WithoutFragment() Reference
}
```

## 14.3 Module-level reference parser

```sec
/**
 * Parses an RFC 3986 URI-reference, absolute or relative.
 *
 * Parameters:
 *   - value: Complete serialized reference.
 *
 * Returns:
 *   A validated Reference preserving absent versus present-empty query and fragment.
 *
 * Errors:
 *   - ParseError.EmptyInput: Reserved for contexts that disallow an empty reference;
 *     generic ParseReference itself accepts the RFC-valid empty reference.
 *   - ParseError.InvalidCharacter: A forbidden raw character occurs.
 *   - ParseError.InvalidScheme: A detected scheme is malformed.
 *   - ParseError.InvalidAuthority: Authority syntax is malformed.
 *   - ParseError.InvalidUserInfo: Userinfo syntax is malformed.
 *   - ParseError.InvalidHost: Host syntax is malformed.
 *   - ParseError.InvalidPort: Port syntax is malformed.
 *   - ParseError.InvalidPath: Path syntax is malformed.
 *   - ParseError.InvalidQuery: Query syntax is malformed.
 *   - ParseError.InvalidFragment: Fragment syntax is malformed.
 *   - ParseError.InvalidPercentEncoding: A percent escape is malformed.
 *
 * Standards:
 *   - RFC 3986 sections 4 and 5.
 */
fn ParseReference(value: string) Result[Reference, ParseError]
```

The empty string is a valid relative reference and resolves to the current base
resource with the base fragment removed/replaced according to RFC 3986 resolution
rules. `ParseReference("")` must therefore succeed.

---

# 15. Absolute URL — `url.sec`

## 15.1 Declaration

```sec
type URL struct {
    _scheme: Scheme,
    _authority: Option[Authority],
    _path: Path,
    _query: Option[Query],
    _fragment: Option[Fragment],
}
```

**Representation rationale:** `URL` is the validated absolute form of the same
five-component RFC model as `Reference`, except scheme is mandatory. A struct is
appropriate because all components coexist. Private fields preserve cross-component
invariants.

## 15.2 Initializer

```sec
impl URL {
    init(
        scheme: Scheme,
        authority: Option[Authority],
        path: Path,
        query: Option[Query],
        fragment: Option[Fragment],
    ) BuildError
```

The initializer validates:

- scheme is non-empty and matches RFC 3986 scheme grammar;
- every supplied component has valid generic syntax;
- when authority is present, path is empty or begins with `/`;
- when authority is absent, path does not begin with `//`;
- the initializer does not apply scheme-specific requirements such as "HTTP must
  have a non-empty host"; those belong to the consuming protocol package.

## 15.3 Properties

The same `impl URL` continues with:

```sec
    /** Returns the required canonical lowercase scheme. */
    property Scheme: Scheme { get }

    /** Returns the optional authority, preserving absent versus empty authority. */
    property Authority: Option[Authority] { get }

    /** Returns the encoded path. */
    property Path: Path { get }

    /** Returns the optional query, preserving a present-empty query. */
    property Query: Option[Query] { get }

    /** Returns the optional fragment, preserving a present-empty fragment. */
    property Fragment: Option[Fragment] { get }

    /** Returns true when an authority component is present. */
    property HasAuthority: bool { get }

    /** Returns true when a query delimiter is present. */
    property HasQuery: bool { get }

    /** Returns true when a fragment delimiter is present. */
    property HasFragment: bool { get }
```

## 15.4 Serialization, normalization and resolution

```sec
    /** Serializes the URL using RFC 3986 component recomposition. */
    fn String() string

    /** Serializes the URL while replacing authority userinfo with `REDACTED`. */
    fn Redacted() string

    /** Converts this URL to the equivalent absolute Reference. */
    fn AsReference() Reference

    /**
     * Resolves a URI-reference against this URL.
     *
     * Parameters:
     *   - reference: Borrowed absolute or relative reference.
     *
     * Returns:
     *   A new absolute URL produced by RFC 3986 section 5.2.
     *
     * Standards:
     *   - RFC 3986 section 5.2.
     */
    fn Resolve(reference: ref Reference) URL

    /**
     * Applies RFC 3986 syntax-based normalization only.
     *
     * Description:
     *   Lowercases scheme and registered-name host, canonicalizes IPv6 text,
     *   canonicalizes percent escapes, decodes escapes for unreserved characters,
     *   and removes path dot-segments. It does not remove scheme-specific default
     *   ports or perform application-specific equivalence transformations.
     */
    fn Normalize() URL

    /**
     * Compares two URLs after `Normalize()`.
     *
     * Description:
     *   This is RFC generic syntax equivalence only. It does not claim that two
     *   scheme-specific resources are globally identical.
     */
    fn Equivalent(other: ref URL) bool
```

## 15.5 Functional component replacement

URL values are immutable unless stored in a mutable binding, but these helpers
return validated replacement values and avoid exposing private fields.

```sec
    /** Returns a copy with a validated replacement scheme. */
    fn WithScheme(scheme: Scheme) Result[URL, BuildError]

    /** Returns a copy with a replacement authority and revalidates path invariants. */
    fn WithAuthority(authority: Option[Authority]) Result[URL, BuildError]

    /** Returns a copy with a replacement path and revalidates authority/path rules. */
    fn WithPath(path: Path) Result[URL, BuildError]

    /** Returns a copy with the supplied optional query. */
    fn WithQuery(query: Option[Query]) URL

    /** Returns a copy with the supplied optional fragment. */
    fn WithFragment(fragment: Option[Fragment]) URL

    /** Returns a copy with no query delimiter. */
    fn WithoutQuery() URL

    /** Returns a copy with no fragment delimiter. */
    fn WithoutFragment() URL
}
```

## 15.6 Module-level absolute parser

```sec
/**
 * Parses one strict absolute RFC 3986 URL/URI.
 *
 * Parameters:
 *   - value: Complete serialized absolute identifier.
 *
 * Returns:
 *   A validated URL with a canonical lowercase Scheme.
 *
 * Errors:
 *   - ParseError.EmptyInput: Input is empty.
 *   - ParseError.MissingScheme: Input is a valid relative reference, not a URL.
 *   - ParseError.InvalidCharacter: Forbidden raw character.
 *   - ParseError.InvalidScheme: Scheme syntax is malformed.
 *   - ParseError.InvalidAuthority: Authority syntax is malformed.
 *   - ParseError.InvalidUserInfo: Userinfo syntax is malformed.
 *   - ParseError.InvalidHost: Host syntax is malformed.
 *   - ParseError.InvalidPort: Port syntax is malformed.
 *   - ParseError.InvalidPath: Path syntax is malformed.
 *   - ParseError.InvalidQuery: Query syntax is malformed.
 *   - ParseError.InvalidFragment: Fragment syntax is malformed.
 *   - ParseError.InvalidPercentEncoding: A percent escape is malformed.
 *
 * Standards:
 *   - RFC 3986.
 */
fn Parse(value: string) Result[URL, ParseError]
```

`Parse()` does not:

- perform DNS lookup;
- infer a missing scheme;
- prepend `http://`;
- treat backslash as slash;
- reinterpret spaces as `%20`;
- accept malformed `%` escapes;
- apply IRI/IDNA conversion;
- apply WHATWG special-scheme parsing;
- validate HTTP-specific host/port requirements;
- access the filesystem for `file:` URLs.

---

# 16. RFC 3986 reference resolution

`URL.Resolve()` must be behaviorally equivalent to RFC 3986 section 5.2.

The implementation must preserve the algorithmic distinctions between:

- reference with a scheme;
- reference with authority;
- empty reference path;
- absolute reference path;
- relative reference path;
- query replacement/inheritance;
- fragment replacement;
- merge-path processing;
- dot-segment removal.

Important examples from RFC-style resolution behavior include:

```text
base: http://a/b/c/d;p?q

g:h      -> g:h
./g      -> http://a/b/c/g
g/       -> http://a/b/c/g/
/g       -> http://a/g
//g      -> http://g
?y       -> http://a/b/c/d;p?y
g?y      -> http://a/b/c/g?y
#s       -> http://a/b/c/d;p?q#s
g#s      -> http://a/b/c/g#s
..       -> http://a/b/
../g     -> http://a/b/g
../../g  -> http://a/g
```

These examples are required conformance tests, not merely explanatory examples.

---

# 17. Normalization contract

## 17.1 Generic syntax normalization only

`Normalize()` performs only transformations justified by generic RFC syntax:

1. lowercase scheme;
2. lowercase ASCII registered-name host spelling;
3. RFC 5952 IPv6 serialization;
4. uppercase hex digits in percent escapes;
5. decode percent escapes that represent unreserved characters;
6. remove path dot-segments.

## 17.2 Forbidden generic normalization

`Normalize()` must not generically:

- remove port `80` from `http`;
- remove port `443` from `https`;
- insert `/` for an empty HTTP path;
- sort query parameters;
- interpret query parameters;
- remove an empty query delimiter;
- remove an empty fragment delimiter;
- collapse repeated `/` characters;
- Unicode-normalize component data;
- resolve hostnames;
- remove `www.`;
- change scheme-specific path case.

Those operations are either scheme-specific, application-specific, or would
change observable syntax not proven equivalent by the generic model.

---

# 18. Equality and security guidance

Ordinary `URL` value equality, where derived by Sec from its fields, compares the
stored component representation. It is not a claim of resource identity.

For generic normalized comparison use:

```sec
left.Equivalent(ref right)
```

Security-sensitive code must not assume that textual URL equality proves:

- same DNS result;
- same network endpoint;
- same HTTP origin;
- same filesystem object;
- same authorization scope;
- same resource content.

Those concepts belong to their owning protocol/domain packages.

`Redacted()` exists for diagnostics and logging and must not expose authority
userinfo. It does not search query/path/fragment values for application secrets.

---

# 19. Allocation and ownership

Public URL values do not own OS resources and require no `free()`.

Operations returning `string`, arrays, `URL`, `Reference`, `Path`, or normalized
components return ordinary owned Sec values.

Functions taking `ref` borrow for the duration of the call and do not retain the
borrow.

No API in this revision consumes a URL, Reference, Host, Path, Query, Fragment,
or Authority merely to inspect or transform it.

`QueryParams.init(->items: QueryParameter[])` is consuming because the new
`QueryParams` takes ownership of the caller-provided array rather than copying it.

---

# 20. Parser strictness and diagnostics

The parser must reject control characters and raw characters outside the allowed
RFC 3986 ASCII syntax.

A malformed escape such as:

```text
%2
%GG
%
```

must report the byte offset of `%`.

A parser diagnostic should preserve both the high-level component and the source
position. Example LSP presentation:

```text
invalid percent encoding in URL path

byte offset: 24
expected: '%' followed by two hexadecimal digits
```

The library error value remains the exact public `ParseError` alternative; richer
human diagnostic wording is tooling policy.

---

# 21. Explicit non-goals for revision 0.1

The following are deliberately outside the public API of this revision:

- WHATWG browser URL parsing;
- IRI parsing;
- IDNA / UTS #46 domain conversion;
- public suffix processing;
- DNS resolution;
- HTTP origin computation;
- URLPattern-style matching;
- URI-template expansion;
- `file:` URL to filesystem-path conversion;
- platform-specific path conversion;
- scheme-specific normalization beyond the generic rules;
- data-URL payload parsing;
- mailto-address parsing;
- URN namespace-specific parsing.

These exclusions prevent `net/url` from silently conflating generic syntax with
scheme-specific or browser-specific semantics.

They may become separate APIs or packages later.

---

# 22. Minimum conformance tests

A conforming implementation must test at least the following classes.

## 22.1 Scheme

- first character must be ASCII alpha;
- subsequent ASCII alphanumeric, `+`, `-`, `.` accepted;
- uppercase input canonicalized lowercase;
- empty rejected;
- colon is not part of `Scheme.Parse()` input.

## 22.2 Percent encoding

- every byte value encodable;
- uppercase hex output;
- malformed escapes rejected with exact offset;
- decoded invalid UTF-8 rejected by `PercentDecode()`;
- same bytes accepted by `PercentDecodeBytes()`;
- unreserved escape normalization;
- reserved escape retained.

## 22.3 Host

- registered names;
- empty registered-name;
- exact IPv4 dotted decimal;
- rejection of legacy IPv4 spellings;
- valid compressed and uncompressed IPv6 input;
- RFC 5952 output;
- IPvFuture;
- bracket errors;
- malformed percent escapes;
- IPv6 zone identifier rejection.

## 22.4 Authority

- no userinfo / no port;
- userinfo;
- encoded `@` in userinfo;
- IPv6 literal with port;
- absent port versus explicit empty port;
- `Redacted()` behavior.

## 22.5 Query presence

All must round-trip distinctly:

```text
x:a
x:a?
x:a#
x:a?#
```

## 22.6 Path

- empty path;
- absolute path;
- rootless path;
- trailing slash;
- `%2F` retained inside segment;
- dot-segment removal;
- literal backslash not treated as separator.

## 22.7 QueryParams

- duplicate keys;
- stable order;
- empty key;
- empty value;
- pair without `=`;
- first `=` only;
- `+` decoding to space;
- `%2B` decoding to plus;
- UTF-8 round trip;
- malformed escape;
- invalid decoded UTF-8;
- no semicolon separator extension.

## 22.8 Reference resolution

The normal and abnormal RFC 3986 section 5.4 reference-resolution examples must
be represented in the test suite.

---

# 23. Implementation-status requirements

`implementation-status-std-net-url.yaml` should track at least:

```yaml
book: stdlib/net/url/std-net-url.md
module: net/url
status: specified

areas:
  errors:
    status: required
  scheme:
    status: required
  percent_encoding:
    status: required
  host:
    status: required
  authority:
    status: required
  path:
    status: required
  generic_query:
    status: required
  query_params:
    status: required
  reference:
    status: required
  absolute_url:
    status: required
  reference_resolution:
    status: required
  normalization:
    status: required
  lsp_documentation:
    status: required
  conformance_tests:
    status: required

explicitly_deferred:
  - whatwg_browser_parser
  - iri
  - idna_uts46
  - public_suffix
  - uri_templates
  - url_patterns
  - file_url_path_conversion
```

The status fragment reports implementation state only. It does not redefine the
API in this book.

---

# 24. Public API inventory

An AI or human validator can use this section as a concise completeness index.
The normative signatures remain the detailed sections above.

## Types

```text
ParseError          union error
BuildError          enum error
PercentError        union error
PortError           union error
QueryParamsError    union error
Scheme              named string
Component           enum
HostKind            enum
Host                 struct
UserInfo             named string
Port                 named string
Authority            struct
Path                 named string
Query                named string
Fragment             named string
QueryParameter       struct
QueryParams          struct
Reference            struct
URL                  struct
```

## Module functions

```sec
fn PercentEncode(value: string, component: Component) string
fn PercentEncode(value: ref byte[], component: Component) string
fn PercentDecode(value: string) Result[string, PercentError]
fn PercentDecodeBytes(value: string) Result[byte[], PercentError]
fn ValidatePercentEncoding(value: string) Result[void, PercentError]
fn NormalizePercentEncoding(value: string) Result[string, PercentError]
fn ParseReference(value: string) Result[Reference, ParseError]
fn Parse(value: string) Result[URL, ParseError]
```

## `Scheme`

```sec
static let Http: Scheme
static let Https: Scheme
static let Ws: Scheme
static let Wss: Scheme
static let Ftp: Scheme
static let File: Scheme
static let Mailto: Scheme
static let Data: Scheme
static let Urn: Scheme
static fn Parse(value: string) Result[Scheme, ParseError]
fn IsValid() bool
property IsHttp: bool { get }
property IsWebSocket: bool { get }
property IsSecureTransportScheme: bool { get }
```

## `Host`

```sec
static fn Parse(value: string) Result[Host, ParseError]
property Kind: HostKind { get }
property Value: string { get }
property IsEmpty: bool { get }
property IsIP: bool { get }
property IsRegisteredName: bool { get }
fn String() string
```

## `UserInfo`

```sec
static fn Parse(value: string) Result[UserInfo, ParseError]
static fn FromDecoded(value: string) UserInfo
fn Decode() Result[string, PercentError]
```

## `Port`

```sec
static fn Parse(value: string) Result[Port, ParseError]
static fn FromNumber(value: uint16) Port
property IsEmpty: bool { get }
fn Number() Result[uint16, PortError]
```

## `Authority`

```sec
init(host: Host)
init(host: Host, port: Port)
init(userInfo: UserInfo, host: Host, port: Option[Port])
static fn Parse(value: string) Result[Authority, ParseError]
fn String() string
fn Redacted() string
```

## `Path`

```sec
static fn Parse(value: string) Result[Path, ParseError]
static fn FromDecoded(value: string) Path
property IsAbsolute: bool { get }
property IsEmpty: bool { get }
property HasTrailingSlash: bool { get }
fn Decode() Result[string, PercentError]
fn Segments() Result[string[], PercentError]
fn RemoveDotSegments() Path
fn Normalize() Path
fn AppendSegment(segment: string) Path
```

## `Query`

```sec
static fn Parse(value: string) Result[Query, ParseError]
static fn FromDecoded(value: string) Query
fn Decode() Result[string, PercentError]
fn Normalize() Query
```

## `Fragment`

```sec
static fn Parse(value: string) Result[Fragment, ParseError]
static fn FromDecoded(value: string) Fragment
fn Decode() Result[string, PercentError]
fn Normalize() Fragment
```

## `QueryParams`

```sec
init(->items: QueryParameter[])
static fn Parse(value: string) Result[QueryParams, QueryParamsError]
property Count: usize { get }
property IsEmpty: bool { get }
fn Add(name: string, value: string) void
fn Set(name: string, value: string) void
fn Get(name: string) Option[string]
fn GetAll(name: string) string[]
fn Has(name: string) bool
fn Remove(name: string) usize
fn Clear() void
fn Encode() string
fn ToArray() QueryParameter[]
```

## `Reference`

```sec
init(
    scheme: Option[Scheme],
    authority: Option[Authority],
    path: Path,
    query: Option[Query],
    fragment: Option[Fragment],
) BuildError
property Scheme: Option[Scheme] { get }
property Authority: Option[Authority] { get }
property Path: Path { get }
property Query: Option[Query] { get }
property Fragment: Option[Fragment] { get }
property IsAbsolute: bool { get }
property IsRelative: bool { get }
property HasAuthority: bool { get }
fn String() string
fn Redacted() string
fn ToURL() Result[URL, ParseError]
fn Normalize() Reference
fn WithFragment(fragment: Option[Fragment]) Reference
fn WithoutFragment() Reference
```

## `URL`

```sec
init(
    scheme: Scheme,
    authority: Option[Authority],
    path: Path,
    query: Option[Query],
    fragment: Option[Fragment],
) BuildError
property Scheme: Scheme { get }
property Authority: Option[Authority] { get }
property Path: Path { get }
property Query: Option[Query] { get }
property Fragment: Option[Fragment] { get }
property HasAuthority: bool { get }
property HasQuery: bool { get }
property HasFragment: bool { get }
fn String() string
fn Redacted() string
fn AsReference() Reference
fn Resolve(reference: ref Reference) URL
fn Normalize() URL
fn Equivalent(other: ref URL) bool
fn WithScheme(scheme: Scheme) Result[URL, BuildError]
fn WithAuthority(authority: Option[Authority]) Result[URL, BuildError]
fn WithPath(path: Path) Result[URL, BuildError]
fn WithQuery(query: Option[Query]) URL
fn WithFragment(fragment: Option[Fragment]) URL
fn WithoutQuery() URL
fn WithoutFragment() URL
```

---

# 25. Summary of locked revision-0.1 decisions

- The public package is `net/url`, module `url`.
- `Parse()` is a strict RFC 3986 absolute parser.
- `ParseReference()` accepts both absolute and relative RFC 3986 references.
- `URL` always has a scheme; `Reference` may omit it.
- Generic encoded components are preserved instead of eagerly decoded.
- Query/fragment absence is distinct from present-empty.
- Authority absence is distinct from present empty authority.
- `Scheme` is an open named string, not an enum.
- `HostKind` is a closed enum.
- `Host` is a validated struct with private representation.
- `UserInfo`, `Port`, `Path`, `Query`, and `Fragment` are named string types.
- Generic `Query` is not a parameter map.
- `QueryParams` is an ordered duplicate-preserving form parameter sequence.
- Percent encoding is component-aware and uses uppercase hex output.
- IPv6 serialization follows RFC 5952.
- RFC 6874 URL zone-id syntax is not supported because RFC 9844 obsoletes it.
- WHATWG browser URL parsing is not mixed into strict `Parse()`.
- IRI/IDNA support is explicitly deferred rather than partially implemented.
- Reference resolution follows RFC 3986 section 5.2.
- Normalization is generic syntax normalization only, never speculative
  scheme-specific canonicalization.
- No public URL type requires `free()`.
- No `New()` pseudo-constructor exists anywhere in the package.
- Consuming parameter syntax uses `->name: Type`; this revision uses it only for
  `QueryParams.init(->items: QueryParameter[])`.
