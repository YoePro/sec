# Sec Standard Library — Domain Name System

- **Status:** Draft
- **Created:** 2026-09-07
- **Last updated:** 2026-09-07
- **Document revision:** 0.1
- **Sec language version:** 0.1
- **Canonical path:** `stdlib/net/dns/std-net-dns.md`
- **Repository path:** `sec/stdlib/net/dns/std-net-dns.md`
- **Parent specification:** `stdlib/net/std-net.md`
- **Implementation-status fragment:** `implementation-status-std-net-dns.yaml`
- **Latest verified repository main baseline:** `0f5027d`
- **Repository tree rechecked:** 2026-09-07

---

# 1. Purpose

`net/dns` is Sec's foundational Domain Name System package.

It owns the common public model for:

- DNS names;
- host names used with DNS;
- DNS message headers, questions, resource records, and sections;
- IANA DNS registries represented as typed open enums;
- common resource-record data;
- EDNS(0);
- DNSSEC wire-level record and flag representation;
- classic DNS transport over UDP and TCP;
- DNS query clients;
- stub resolution using configured recursive DNS servers;
- positive and negative resolver caching;
- reverse DNS name construction and PTR lookup;
- allocation-aware DNS message encoding and decoding.

The package is imported as:

```sec
import "net/dns"
```

and used as:

```sec
let name := try dns.Name.Parse("example.com")
let resolver := try dns.Resolver.System()
let addresses := try resolver.LookupAddresses(try dns.HostName.ParseAscii("example.com"))
```

The full package path identifies the package. The final path component is the
normal source qualifier:

```text
"net/dns" -> dns
```

The base package deliberately does not absorb every DNS-related protocol and
tool into one directory. Substantial higher-layer extensions may receive
subpackages and their own standard-library books.

---

# 2. Package boundary

## 2.1 Base package

The initial `net/dns` package owns:

```text
DNS names
host-name validation
DNS registries
DNS wire messages
common resource records
EDNS(0)
DNSSEC wire records
UDP transport
TCP transport
DNS client
stub resolver
resolver cache
reverse lookup helpers
```

## 2.2 Child packages

The following deeper packages are reserved because they introduce substantial
dependencies or independently large semantics:

```text
net/dns/doh
net/dns/dot
net/dns/doq
net/dns/dnssec
net/dns/zone
```

Their intended responsibilities are:

| Package | Responsibility |
|---|---|
| `net/dns/doh` | DNS over HTTPS |
| `net/dns/dot` | DNS over TLS |
| `net/dns/doq` | DNS over Dedicated QUIC Connections |
| `net/dns/dnssec` | local DNSSEC validation, trust anchors, chain validation |
| `net/dns/zone` | zone/master-file parsing, writing, and zone-oriented tooling |

Each child package receives its own local book when designed:

```text
net/dns/doh/std-net-dns-doh.md
net/dns/dot/std-net-dns-dot.md
net/dns/doq/std-net-dns-doq.md
net/dns/dnssec/std-net-dns-dnssec.md
net/dns/zone/std-net-dns-zone.md
```

The base `net/dns` package must not import these children.

This is particularly important for DoH: `net/http` is expected to depend on
name resolution, so placing a DoH HTTP client implementation directly in the
base `dns` package would risk a `dns -> http -> dns` dependency cycle.

## 2.3 Related packages outside `net/dns`

The following are not folded into this package:

```text
net/mdns
net/dnssd
```

Multicast DNS and DNS Service Discovery use DNS-compatible concepts but have
their own discovery semantics and are governed by their own package books.

---

# 3. Design goals

`net/dns` follows the general Sec stdlib rules and additionally requires:

1. **DNS names are typed.** DNS APIs do not use arbitrary strings where the
   semantic value is a validated DNS name.
2. **DNS names are not automatically host names.** DNS can store data at names
   that are not valid Internet host names.
3. **IANA registries are typed, extensible values.** Open wire registries use
   fixed-width open enums, not magic integers or strings.
4. **Unknown protocol values survive decoding.** A future or privately
   assigned RR type, EDNS option, RCODE, or algorithm must remain
   representable.
5. **DNS transport is distinct from DNS semantics.** Messages and records can
   be encoded/decoded without an active socket capability.
6. **Classic DNS supports both UDP and TCP.** A general-purpose
   implementation must not treat TCP as an optional afterthought.
7. **UDP truncation is explicit.** A truncated response can trigger TCP retry
   according to resolver/client policy.
8. **EDNS is first-class.** It is not bolted onto an opaque "additional
   record".
9. **DNSSEC metadata is not equivalent to validated security.** Merely seeing
   AD/DO/RRSIG/DNSKEY data does not prove authenticity.
10. **Name compression is validated defensively.** Compression loops,
    out-of-range pointers, and malformed label sequences are protocol errors.
11. **Allocation is visible where practical.** Materializing an owning DNS
    message, record array, TXT field array, or resolver answer is documented
    as allocating.
12. **Network errors reuse `net/ip`.** DNS does not duplicate
    `ip.NetworkError`.
13. **Cancellation and deadlines reuse Sec's normal execution model.**
14. **System resolver configuration and DNS protocol behavior are not
    conflated silently.**
15. **Higher-level protocols may use system-style host resolution while raw
    DNS clients retain precise DNS semantics.**
16. **Wire names are case-insensitive for comparison where DNS requires it,
    while case preservation remains possible.**
17. **Internationalized names are explicit.** Unicode-to-IDNA processing is
    not an accidental side effect of ordinary DNS wire parsing.
18. **Full recursive resolution is not part of the base package.** The
    standard resolver is initially a stub resolver using configured recursive
    DNS servers.

---

# 4. Standards and registries

## 4.1 Standards policy

The RFCs and registries below form the baseline for this package.

DNS is an old protocol family with many updating RFCs. Implementations must
follow the current applicable standards state, not blindly reproduce an older
base RFC when later standards update it.

Relevant RFC errata must be considered during implementation.

IANA registries are authoritative for live numeric namespaces. Source files
that mirror an IANA registry must record the registry URL and be updated when
the package is deliberately synchronized with a newer registry state.

## 4.2 Core DNS references

| Area | Standard | URL |
|---|---|---|
| DNS concepts and architecture | RFC 1034 | https://www.rfc-editor.org/rfc/rfc1034 |
| DNS implementation and wire format | RFC 1035 | https://www.rfc-editor.org/rfc/rfc1035 |
| DNS clarifications | RFC 2181 | https://www.rfc-editor.org/rfc/rfc2181 |
| DNS terminology | RFC 9499 / BCP 219 | https://www.rfc-editor.org/rfc/rfc9499 |
| DNS case-insensitive comparison | RFC 4343 | https://www.rfc-editor.org/rfc/rfc4343 |
| Negative caching | RFC 2308 | https://www.rfc-editor.org/rfc/rfc2308 |
| DNS over TCP requirements | RFC 7766 | https://www.rfc-editor.org/rfc/rfc7766 |
| EDNS(0) | RFC 6891 / STD 75 | https://www.rfc-editor.org/rfc/rfc6891 |
| DNS IANA considerations | RFC 6895 | https://www.rfc-editor.org/rfc/rfc6895 |
| IP fragmentation avoidance in DNS/UDP | RFC 9715 | https://www.rfc-editor.org/rfc/rfc9715 |
| DNS Query Name Minimisation | RFC 9156 | https://www.rfc-editor.org/rfc/rfc9156 |
| Serve stale | RFC 8767 | https://www.rfc-editor.org/rfc/rfc8767 |

## 4.3 Internationalized domain names

| Area | Standard | URL |
|---|---|---|
| IDNA definitions/framework | RFC 5890 | https://www.rfc-editor.org/rfc/rfc5890 |
| IDNA protocol | RFC 5891 | https://www.rfc-editor.org/rfc/rfc5891 |
| Unicode code-point rules | RFC 5892 | https://www.rfc-editor.org/rfc/rfc5892 |
| IDNA bidi rules | RFC 5893 | https://www.rfc-editor.org/rfc/rfc5893 |
| IDNA background/rationale | RFC 5894 | https://www.rfc-editor.org/rfc/rfc5894 |
| IDNA mapping considerations | RFC 5895 | https://www.rfc-editor.org/rfc/rfc5895 |

The base DNS wire protocol is ASCII/octet-oriented. IDNA processing is an
application-facing transformation and is therefore explicit.

## 4.4 DNSSEC baseline references

The base package implements DNSSEC wire values and resource-record data. Full
local validation belongs to `net/dns/dnssec`.

| Area | Standard / registry | URL |
|---|---|---|
| DNSSEC introduction | RFC 4033 | https://www.rfc-editor.org/rfc/rfc4033 |
| DNSSEC resource records | RFC 4034 | https://www.rfc-editor.org/rfc/rfc4034 |
| DNSSEC protocol modifications | RFC 4035 | https://www.rfc-editor.org/rfc/rfc4035 |
| NSEC3 | RFC 5155 | https://www.rfc-editor.org/rfc/rfc5155 |
| DNSSEC clarifications | RFC 6840 | https://www.rfc-editor.org/rfc/rfc6840 |
| DNSSEC algorithm update process | RFC 9904 | https://www.rfc-editor.org/rfc/rfc9904 |
| DNSSEC algorithm registry | IANA DNSSEC Algorithm Numbers | https://www.iana.org/assignments/dns-sec-alg-numbers |

RFC 9904 moves current DNSSEC algorithm implementation/usage recommendations
to IANA registry metadata. The library must therefore not hard-code old
algorithm recommendations from obsolete guidance as if they were permanent.

## 4.5 EDNS and related references

| Area | Standard | URL |
|---|---|---|
| EDNS(0) | RFC 6891 | https://www.rfc-editor.org/rfc/rfc6891 |
| EDNS padding | RFC 7830 | https://www.rfc-editor.org/rfc/rfc7830 |
| DNS Cookies | RFC 7873 | https://www.rfc-editor.org/rfc/rfc7873 |
| EDNS Client Subnet | RFC 7871 | https://www.rfc-editor.org/rfc/rfc7871 |
| Extended DNS Errors | RFC 8914 | https://www.rfc-editor.org/rfc/rfc8914 |

EDNS Client Subnet is supported as wire data but must not be enabled
implicitly by the standard resolver because it changes privacy properties.

## 4.6 Secure DNS transport references

These are owned by child-package books but are recorded here for package
taxonomy:

| Child package | Standard | URL |
|---|---|---|
| `net/dns/dot` | RFC 7858 — DNS over TLS | https://www.rfc-editor.org/rfc/rfc7858 |
| `net/dns/doh` | RFC 8484 — DNS over HTTPS | https://www.rfc-editor.org/rfc/rfc8484 |
| `net/dns/doq` | RFC 9250 — DNS over Dedicated QUIC Connections | https://www.rfc-editor.org/rfc/rfc9250 |

## 4.7 IANA DNS registries

Primary registry:

https://www.iana.org/assignments/dns-parameters

This registry includes, among other namespaces:

- DNS CLASSes;
- Resource Record TYPEs;
- DNS OpCodes;
- DNS RCODEs;
- EDNS option codes;
- DNS header flags;
- EDNS header flags;
- Extended DNS Error codes;
- DSO type codes.

The registry was rechecked for this revision on 2026-09-07.

---

# 5. Source-file standards headers

Every protocol source file in `stdlib/net/dns` must identify the standards or
IANA registries directly governing it.

Example:

```sec
// Sec Standard Library: net/dns
// File: message.sec
//
// Standards:
//   RFC 1035 - Domain Names - Implementation and Specification
//   https://www.rfc-editor.org/rfc/rfc1035
//
//   RFC 6891 - Extension Mechanisms for DNS (EDNS(0))
//   https://www.rfc-editor.org/rfc/rfc6891
//
// Related:
//   RFC 2181 - Clarifications to the DNS Specification
//   https://www.rfc-editor.org/rfc/rfc2181

module dns
```

A file that mirrors an IANA registry must include the registry URL and should
record the synchronization date in a comment.

---

# 6. Package and file manifest

The initial base-package layout is:

```text
stdlib/net/dns/
├── std-net-dns.md
├── error.sec
├── name.sec
├── idna.sec
├── registry.sec
├── message.sec
├── record.sec
├── records_basic.sec
├── records_service.sec
├── records_dnssec.sec
├── edns.sec
├── codec.sec
├── reverse.sec
├── transport_udp.sec
├── transport_tcp.sec
├── client.sec
├── cache.sec
├── resolver.sec
├── system.<target>.sec
│
├── doh/
│   └── std-net-dns-doh.md          # later child book
├── dot/
│   └── std-net-dns-dot.md          # later child book
├── doq/
│   └── std-net-dns-doq.md          # later child book
├── dnssec/
│   └── std-net-dns-dnssec.md       # later child book
└── zone/
    └── std-net-dns-zone.md         # later child book
```

All ordinary base-package source files use:

```sec
module dns
```

The child directories are separate packages and do not use `module dns`
unless their own package rule explicitly selects that name.

The current repository contains only:

```text
sec/stdlib/net/dns/dns.sec
```

with:

```sec
module dns
```

That one-line file is a stub. It carries no public API that must be preserved.
After the new package files exist, it should be removed rather than retained
as an unnecessary empty package anchor.

---

# Part I — Errors and names

# 7. `error.sec`

## 7.1 Responsibility

`error.sec` owns errors specific to DNS names, DNS message encoding/decoding,
classic DNS transport policy, and high-level resolution.

It reuses:

```sec
ip.NetworkError
```

for IP/socket failures.

## 7.2 `NameError`

```sec
enum NameError error {
    Empty,
    EmptyLabel,
    LabelTooLong,
    NameTooLong,
    InvalidCharacter,
    InvalidEscape,
    InvalidHostName,
    InvalidIdna,
}
```

Meaning:

- `Empty`: an API requiring a DNS name received no name;
- `EmptyLabel`: an interior empty label was encountered;
- `LabelTooLong`: one DNS label exceeds the protocol limit;
- `NameTooLong`: encoded DNS name exceeds the protocol limit;
- `InvalidCharacter`: textual syntax contains a character not legal for the
  selected parser;
- `InvalidEscape`: DNS presentation-form escaping is malformed;
- `InvalidHostName`: a valid DNS name does not satisfy the stricter host-name
  contract;
- `InvalidIdna`: IDNA2008 conversion/validation failed.

## 7.3 `MessageError`

```sec
enum MessageError error {
    BufferTooSmall,
    MalformedHeader,
    MalformedQuestion,
    MalformedRecord,
    MalformedName,
    LabelTooLong,
    NameTooLong,
    CompressionPointerOutOfRange,
    CompressionLoop,
    CompressionDepthExceeded,
    SectionCountInvalid,
    RecordLengthInvalid,
    UnsupportedLabelType,
    InvalidEdns,
    MultipleOptRecords,
    InvalidDnssecRecord,
    OutputTooSmall,
    MessageTooLarge,
}
```

Message parsing must distinguish malformed DNS wire data from network
transport failure.

## 7.4 `TransportError`

```sec
union TransportError error {
    Network(ip.NetworkError),
    Message(MessageError),
    MismatchedResponse,
    TruncatedWithoutFallback,
    NoServer,
}
```

`MismatchedResponse` covers a response that cannot be accepted as the response
to the outstanding query, for example because transaction identity or
question matching fails.

## 7.5 `ResolveError`

```sec
union ResolveError error {
    Name(NameError),
    Transport(TransportError),
    Response(ResponseCode),
    NoData,
    NoServers,
    SearchExhausted,
    Unsupported,
}
```

Rules:

- a raw `Client.Exchange` still returns a syntactically valid DNS message even
  when its DNS RCODE is nonzero;
- higher-level resolver operations may map a terminal non-success RCODE into
  `ResolveError.Response`;
- `NXDomain` and `NoData` are distinct;
- `NoData` means the queried name exists but no data of the requested type is
  available in the resolver result;
- cancellation is not duplicated as a DNS-specific error;
- platform DNS configuration absence becomes `NoServers` when no usable
  recursive server can be determined.

---

# 8. `name.sec`

## 8.1 Responsibility

`name.sec` owns DNS domain-name values and the stricter ASCII host-name value.

Primary references:

- RFC 1034
- RFC 1035
- RFC 2181
- RFC 4343
- RFC 9499

## 8.2 DNS name model

A `dns.Name` is an **absolute DNS name**.

It is not:

- a URL host component;
- a relative zone-file name;
- a search-list input;
- automatically an Internet host name.

The root name is represented explicitly.

The DNS protocol limits are:

- an ordinary label is at most 63 octets;
- a complete encoded domain name is at most 255 octets including label length
  octets and the terminating root label.

## 8.3 `Name`

```sec
type Name struct {
    // Private immutable canonical DNS-name representation.
}
```

Canonical public API:

```sec
impl Name {
    static fn Parse(text: string) Result[Name, NameError]

    static let Root: Name := ...

    property IsRoot: bool { get { ... } }
    property LabelCount: uint { get { ... } }

    fn ToString() string
    fn ToCanonicalString() string

    fn Equals(other: ref Name) bool
}
```

Rules:

- `Name.Parse(".")` produces `Name.Root`;
- ordinary textual input may include or omit the final presentation-form dot;
- the resulting value is always semantically absolute;
- interior empty labels are rejected;
- DNS presentation escapes supported by RFC 1035-compatible syntax must be
  decoded correctly;
- wire limits are checked after escape processing;
- ASCII letters compare case-insensitively according to DNS rules;
- case information may be preserved for presentation;
- `ToCanonicalString()` emits a stable lowercase-ASCII comparison form with a
  final root dot;
- `ToString()` emits a valid DNS presentation form and may preserve stored
  case;
- equality is DNS-name equality, not byte-for-byte presentation-string
  equality.

The package must not implement DNS equality using locale-sensitive Unicode
case folding.

## 8.4 Relative names

Relative DNS names are intentionally not represented by `Name`.

They are required by zone-file processing and may later be owned by:

```text
net/dns/zone
```

with an explicit origin.

This prevents hidden dependence on a process-global "current DNS origin".

## 8.5 Name labels

Revision 0.1 does not expose a mutable public label collection.

A future read-only label iterator may be introduced when it can avoid
unnecessary allocation.

---

# 9. `idna.sec`

## 9.1 Responsibility

`idna.sec` owns the stricter host-name abstraction and IDNA2008 conversion
used when an application needs an internationalized Internet host name.

Primary references:

- RFC 5890
- RFC 5891
- RFC 5892
- RFC 5893
- RFC 5895

## 9.2 `HostName`

```sec
type HostName struct {
    _name: Name,
}
```

Canonical public API:

```sec
impl HostName {
    static fn ParseAscii(text: string) Result[HostName, NameError]

    static fn FromUnicode(text: string) Result[HostName, NameError]

    property Name: Name { get { ... } }

    fn ToAsciiString() string
    fn ToUnicodeString() Result[string, NameError]
}
```

Rules:

- `HostName` is stricter than `Name`;
- ASCII host labels must satisfy the Internet host-name rules selected by this
  specification;
- the old restriction that a host label must begin with a letter is not used;
- `_service`-style labels are not ordinary host-name labels and therefore do
  not become valid merely because DNS can store them;
- `FromUnicode` performs explicit IDNA2008 processing;
- the canonical DNS-facing form is the ASCII A-label form;
- Unicode display conversion is distinct from DNS wire encoding;
- the implementation must not silently apply browser-specific compatibility
  mappings as if they were the DNS protocol itself.

If later `net/url` chooses WHATWG/UTS-style mapping for web compatibility,
that is a URL policy layered above this strict DNS type.

---

# Part II — Registry values and wire header

# 10. `registry.sec`

## 10.1 Responsibility

`registry.sec` mirrors the open IANA DNS protocol registries used throughout
the package.

Primary registry:

https://www.iana.org/assignments/dns-parameters

Values are numeric wire contracts. Known names improve readability, but
unknown future values must remain representable.

## 10.2 `RecordType`

```sec
enum RecordType bit[16] {
    A = 1,
    NS = 2,
    CNAME = 5,
    SOA = 6,
    PTR = 12,
    MX = 15,
    TXT = 16,
    AAAA = 28,
    SRV = 33,
    NAPTR = 35,
    DNAME = 39,
    OPT = 41,
    DS = 43,
    SSHFP = 44,
    RRSIG = 46,
    NSEC = 47,
    DNSKEY = 48,
    NSEC3 = 50,
    NSEC3PARAM = 51,
    TLSA = 52,
    CDS = 59,
    CDNSKEY = 60,
    OPENPGPKEY = 61,
    CSYNC = 62,
    ZONEMD = 63,
    SVCB = 64,
    HTTPS = 65,
    CAA = 257,

    TKEY = 249,
    TSIG = 250,
    IXFR = 251,
    AXFR = 252,
    ANY = 255,

    // ...all current assigned IANA values where practical...
}
```

Rules:

- the underlying 16-bit value is open;
- unknown values survive decoding;
- values such as `ANY`, `AXFR`, and `IXFR` are query/meta types and do not
  imply that an ordinary `Record` carrying RDATA of that type is valid;
- the implementation file should mirror all stable assigned IANA values,
  including obsolete values where packet decoding requires symbolic
  preservation;
- a later IANA assignment must not require redesign of the type.

## 10.3 `DnsClass`

```sec
enum DnsClass bit[16] {
    IN = 1,
    CH = 3,
    HS = 4,
    NONE = 254,
    ANY = 255,

    // ...current assigned IANA values...
}
```

The same open-value rule applies.

## 10.4 `Opcode`

```sec
enum Opcode bit[4] {
    Query = 0,
    IQueryObsolete = 1,
    Status = 2,
    Notify = 4,
    Update = 5,
    Dso = 6,
}
```

The value space remains open across 4 bits.

An obsolete value may remain named for decoding and diagnostics. Naming it
does not make it recommended for new use.

## 10.5 `ResponseCode`

EDNS extends the original four-bit RCODE namespace. The semantic type must
therefore represent the extended value:

```sec
enum ResponseCode bit[12] {
    NoError = 0,
    FormErr = 1,
    ServFail = 2,
    NXDomain = 3,
    NotImp = 4,
    Refused = 5,
    YXDomain = 6,
    YXRRSet = 7,
    NXRRSet = 8,
    NotAuth = 9,
    NotZone = 10,
    DsoTypeNotImplemented = 11,
    BadVersOrSig = 16,
    BadKey = 17,
    BadTime = 18,
    BadMode = 19,
    BadName = 20,
    BadAlg = 21,
    BadTrunc = 22,
    BadCookie = 23,

    // ...current assigned IANA values...
}
```

The decoder combines:

- the base header's low four RCODE bits; and
- EDNS extended RCODE bits when an OPT record is present.

The encoder performs the reverse split.

## 10.6 `MessageId`

```sec
type MessageId uint16
```

`MessageId` is nominally distinct from an arbitrary integer.

Transaction-ID generation policy belongs to the client implementation.

For UDP classic DNS, IDs must be generated with sufficient unpredictability
for off-path spoofing resistance. A simple incrementing global counter is not
an acceptable general default.

## 10.7 DNSSEC algorithm registries

Wire-level DNSSEC algorithm and digest identifiers are also open numeric
registries.

Base declarations:

```sec
enum DnssecAlgorithm bit[8] {
    RsaSha256 = 8,
    RsaSha512 = 10,
    EcdsaP256Sha256 = 13,
    EcdsaP384Sha384 = 14,
    Ed25519 = 15,
    Ed448 = 16,

    // ...all assigned registry values...
}

enum DsDigestType bit[8] {
    Sha1 = 1,
    Sha256 = 2,
    GostR3411 = 3,
    Sha384 = 4,

    // ...all assigned registry values...
}
```

The exact "use", "implement", "recommended", and "must not" status is **not**
encoded as enum membership.

Current algorithm policy comes from the IANA registries governed by RFC 9904.

A decoder must be able to represent assigned, deprecated, private, and future
values even if the local cryptographic backend cannot validate them.

---

# 11. `message.sec`

## 11.1 Responsibility

`message.sec` owns semantic DNS message, question, section, and header values.

Primary references:

- RFC 1035
- RFC 2181
- RFC 4035
- RFC 6891

## 11.2 Wire flags

```sec
type HeaderFlags register[16] msb-first {
    IsResponse: bit,
    Opcode: bit[4],
    AuthoritativeAnswer: bit,
    Truncated: bit,
    RecursionDesired: bit,
    RecursionAvailable: bit,
    Reserved: bit,
    AuthenticatedData: bit,
    CheckingDisabled: bit,
    ResponseCodeLow: bit[4],
}
```

`Reserved` must be validated according to the applicable protocol state.

The `AuthenticatedData` and `CheckingDisabled` bits come from DNSSEC protocol
updates and must not be treated as part of a generic unused-Z field.

## 11.3 Fixed wire header

```sec
type WireHeader register[96] msb-first big-endian {
    Id: MessageId,
    Flags: HeaderFlags,
    QuestionCount: bit[16],
    AnswerCount: bit[16],
    AuthorityCount: bit[16],
    AdditionalCount: bit[16],
}
```

This is exactly the fixed 12-byte DNS header.

## 11.4 `Question`

```sec
type Question struct {
    Name: Name,
    Type: RecordType,
    Class: DnsClass,
}
```

A normal Internet query usually uses `DnsClass.IN`, but the public protocol
model does not hard-code that assumption.

## 11.5 Message flags

The owning semantic message exposes flags without forcing users to edit raw
wire bit positions.

```sec
type MessageFlags struct {
    IsResponse: bool,
    AuthoritativeAnswer: bool,
    Truncated: bool,
    RecursionDesired: bool,
    RecursionAvailable: bool,
    AuthenticatedData: bool,
    CheckingDisabled: bool,
}
```

## 11.6 `Message`

```sec
type Message struct {
    Id: MessageId,
    Opcode: Opcode,
    ResponseCode: ResponseCode,
    Flags: MessageFlags,
    Questions: Question[],
    Answers: Record[],
    Authorities: Record[],
    Additionals: Record[],
    Edns: Option[Edns],
}
```

Rules:

- `Message` is an owning decoded/builder representation;
- arrays are materialized storage and may allocate;
- an OPT pseudo-record is represented through `Edns`, not duplicated as an
  ordinary `Record` in `Additionals`;
- a decoder rejects multiple OPT records;
- `ResponseCode` is already combined into the semantic 12-bit value;
- a builder/encoder splits the response code correctly when EDNS is present;
- nonzero RCODE does not make the message structurally invalid.

---

# Part III — Resource records

# 12. `record.sec`

## 12.1 Responsibility

`record.sec` owns common RR envelope types and the `RecordData` union.

Primary references:

- RFC 1035
- RFC 2181
- RFC 3597

RFC 3597 is relevant because a DNS implementation must be able to preserve
unknown RR types without requiring knowledge of their RDATA format.

## 12.2 `Ttl`

```sec
type Ttl uint32
```

`Ttl` carries the DNS wire TTL field in seconds.

It is nominally distinct from an arbitrary integer.

Cache code interprets it as a duration according to the current DNS caching
standards. The wire field itself remains an unsigned 32-bit protocol value.

## 12.3 Unknown record data

```sec
type UnknownRecordData struct {
    Type: RecordType,
    Data: byte[],
}
```

The bytes are the unparsed RDATA payload.

This is mandatory for protocol extensibility.

## 12.4 `RecordData`

```sec
type RecordData union {
    A(Ipv4Record),
    AAAA(Ipv6Record),
    NS(NameServerRecord),
    CNAME(CanonicalNameRecord),
    SOA(StartOfAuthorityRecord),
    PTR(PointerRecord),
    MX(MailExchangeRecord),
    TXT(TextRecord),

    SRV(ServiceRecord),
    NAPTR(NaptrRecord),
    DNAME(DelegationNameRecord),
    CAA(CaaRecord),
    SVCB(SvcbRecord),
    HTTPS(HttpsRecord),
    TLSA(TlsaRecord),
    SSHFP(SshfpRecord),

    DS(DsRecord),
    DNSKEY(DnskeyRecord),
    RRSIG(RrsigRecord),
    NSEC(NsecRecord),
    NSEC3(Nsec3Record),
    NSEC3PARAM(Nsec3ParamRecord),

    Unknown(UnknownRecordData),
}
```

The union variant determines the semantic RR type for known records.

Unknown RDATA retains its explicit `Type`.

## 12.5 `Record`

```sec
type Record struct {
    Name: Name,
    Class: DnsClass,
    Ttl: Ttl,
    Data: RecordData,
}
```

Associated behavior:

```sec
impl Record {
    property Type: RecordType { get { ... } }
}
```

`Type` is derived from `Data`.

The public owning model therefore avoids an independently mutable `Type` field
that could disagree with the typed `RecordData` variant.

---

# 13. `records_basic.sec`

## 13.1 Responsibility

`records_basic.sec` owns common foundational resource-record RDATA.

Primary references include RFC 1035 and later specifications governing the
individual RR types.

## 13.2 Address records

```sec
type Ipv4Record struct {
    Address: ip.Ipv4Address,
}

type Ipv6Record struct {
    Address: ip.Ipv6Address,
}
```

`A` and `AAAA` therefore reuse the canonical `net/ip` address types.

They must not duplicate IPv4/IPv6 address representations inside DNS.

## 13.3 Name-bearing records

```sec
type NameServerRecord struct {
    Host: Name,
}

type CanonicalNameRecord struct {
    Canonical: Name,
}

type PointerRecord struct {
    Target: Name,
}

type DelegationNameRecord struct {
    Target: Name,
}
```

## 13.4 `MailExchangeRecord`

```sec
type MailExchangeRecord struct {
    Preference: uint16,
    Exchange: Name,
}
```

The DNS layer does not decide whether `Exchange` is reachable or has required
address records. It represents the RR.

## 13.5 `StartOfAuthorityRecord`

```sec
type StartOfAuthorityRecord struct {
    PrimaryNameServer: Name,
    ResponsibleMailbox: Name,
    Serial: uint32,
    Refresh: uint32,
    Retry: uint32,
    Expire: uint32,
    Minimum: uint32,
}
```

The timer fields are DNS wire values expressed in seconds by the governing
standards.

`ResponsibleMailbox` is DNS SOA mailbox-name encoding, not a general
`net/mail` mailbox object.

## 13.6 `TextRecord`

DNS TXT RDATA is not inherently Unicode or UTF-8.

```sec
type TextRecord struct {
    Values: byte[][],
}
```

Each wire character-string remains individually represented.

Convenience conversion may be supplied explicitly:

```sec
impl TextRecord {
    fn Utf8ValuesToArray() Result[string[], TextRecordError]
}
```

with:

```sec
enum TextRecordError error {
    InvalidUtf8,
}
```

A decoder must not silently replace invalid bytes merely to return `string`.

---

# 14. `records_service.sec`

## 14.1 Responsibility

`records_service.sec` owns modern service-discovery and service-binding
resource-record RDATA that remains part of ordinary DNS.

Primary references include:

- RFC 2782 — SRV
- RFC 3403 — NAPTR
- RFC 6698 — TLSA
- RFC 6844 / updates — CAA
- RFC 9460 — SVCB and HTTPS
- RFC 4255 — SSHFP

Each implementation section must cite the direct current standard.

## 14.2 `ServiceRecord`

```sec
type ServiceRecord struct {
    Priority: uint16,
    Weight: uint16,
    Port: ip.Port,
    Target: Name,
}
```

The port field reuses `ip.Port`.

The DNS package does not model SRV service/protocol labels as closed enums.
The relevant namespaces are extensible/open.

## 14.3 `NaptrRecord`

```sec
type NaptrRecord struct {
    Order: uint16,
    Preference: uint16,
    Flags: byte[],
    Services: byte[],
    Regexp: byte[],
    Replacement: Name,
}
```

NAPTR character-string fields remain octet data at the DNS layer unless a
specific standard above DNS defines stronger semantics.

## 14.4 `CaaRecord`

```sec
type CaaRecord struct {
    Flags: uint8,
    Tag: string,
    Value: byte[],
}
```

`Tag` follows the CAA tag syntax.

`Value` remains protocol bytes because interpretation depends on the tag.

## 14.5 `TlsaRecord`

```sec
type TlsaRecord struct {
    CertificateUsage: uint8,
    Selector: uint8,
    MatchingType: uint8,
    AssociationData: byte[],
}
```

The three one-byte fields are registry-backed namespaces.

A later revision may introduce dedicated open enums for those registries in
this file if that improves the API without duplicating another shared type.

## 14.6 `SshfpRecord`

```sec
type SshfpRecord struct {
    Algorithm: uint8,
    FingerprintType: uint8,
    Fingerprint: byte[],
}
```

As with TLSA, registry values may become open enums once the exact IANA
ownership is integrated.

## 14.7 SVCB / HTTPS

SVCB and HTTPS use the same service-binding RDATA structure.

```sec
enum SvcParamKey bit[16] {
    Mandatory = 0,
    Alpn = 1,
    NoDefaultAlpn = 2,
    Port = 3,
    Ipv4Hint = 4,
    Ech = 5,
    Ipv6Hint = 6,

    // ...current assigned IANA keys...
}
```

Known and unknown parameter values are represented by:

```sec
type UnknownSvcParam struct {
    Key: SvcParamKey,
    Value: byte[],
}

type SvcParamData union {
    Mandatory(SvcParamKey[]),
    Alpn(byte[][]),
    NoDefaultAlpn,
    Port(ip.Port),
    Ipv4Hint(ip.Ipv4Address[]),
    Ech(byte[]),
    Ipv6Hint(ip.Ipv6Address[]),
    Unknown(UnknownSvcParam),
}

type SvcParam struct {
    Data: SvcParamData,
}
```

Service-binding records:

```sec
type SvcbRecord struct {
    Priority: uint16,
    Target: Name,
    Parameters: SvcParam[],
}

type HttpsRecord struct {
    Priority: uint16,
    Target: Name,
    Parameters: SvcParam[],
}
```

Rules:

- unknown SvcParam keys must survive decoding;
- key ordering and duplicate-key restrictions from the governing standard are
  validated;
- `HTTPS` remains a DNS RR type even though `net/http` is expected to consume
  it at a higher layer;
- the DNS package does not perform HTTP connection selection by merely parsing
  this record.

---

# 15. `records_dnssec.sec`

## 15.1 Responsibility

`records_dnssec.sec` owns DNSSEC wire-level RR data.

It does **not** by itself claim local cryptographic validation.

Primary references:

- RFC 4033
- RFC 4034
- RFC 4035
- RFC 5155
- RFC 6840
- IANA DNSSEC registries

## 15.2 `DsRecord`

```sec
type DsRecord struct {
    KeyTag: uint16,
    Algorithm: DnssecAlgorithm,
    DigestType: DsDigestType,
    Digest: byte[],
}
```

## 15.3 `DnskeyRecord`

```sec
type DnskeyRecord struct {
    Flags: uint16,
    Protocol: uint8,
    Algorithm: DnssecAlgorithm,
    PublicKey: byte[],
}
```

Protocol-field validation must follow the DNSSEC specification.

## 15.4 `RrsigRecord`

```sec
type RrsigRecord struct {
    TypeCovered: RecordType,
    Algorithm: DnssecAlgorithm,
    Labels: uint8,
    OriginalTtl: Ttl,
    SignatureExpiration: uint32,
    SignatureInception: uint32,
    KeyTag: uint16,
    SignerName: Name,
    Signature: byte[],
}
```

The signature time fields use the DNSSEC serial/time representation specified
by the governing RFCs. They are not automatically converted into local wall
clock values during wire decoding.

## 15.5 `NsecRecord`

```sec
type NsecRecord struct {
    NextDomainName: Name,
    Types: RecordType[],
}
```

## 15.6 `Nsec3Record`

```sec
type Nsec3Record struct {
    HashAlgorithm: uint8,
    Flags: uint8,
    Iterations: uint16,
    Salt: byte[],
    NextHashedOwnerName: byte[],
    Types: RecordType[],
}
```

## 15.7 `Nsec3ParamRecord`

```sec
type Nsec3ParamRecord struct {
    HashAlgorithm: uint8,
    Flags: uint8,
    Iterations: uint16,
    Salt: byte[],
}
```

## 15.8 Security status

The base package may expose received DNSSEC signaling status:

```sec
enum DnssecSignal {
    NotRequested,
    Requested,
    AuthenticatedDataClaimed,
}
```

This type is deliberately named `DnssecSignal`, not `DnssecValidation`.

An AD bit from an unauthenticated or untrusted upstream channel is not
cryptographic proof to the application.

Local validation and trust-chain status belong in `net/dns/dnssec`.

---

# Part IV — EDNS

# 16. `edns.sec`

## 16.1 Responsibility

`edns.sec` owns EDNS(0) semantic representation and common option data.

Primary references:

- RFC 6891
- RFC 7830
- RFC 7873
- RFC 7871
- RFC 8914

Primary option registry:

https://www.iana.org/assignments/dns-parameters

## 16.2 `EdnsOptionCode`

```sec
enum EdnsOptionCode bit[16] {
    Nsid = 3,
    ClientSubnet = 8,
    Cookie = 10,
    TcpKeepalive = 11,
    Padding = 12,
    ExtendedDnsError = 15,

    // ...current assigned IANA option codes...
}
```

The namespace is open.

## 16.3 EDNS header flags

```sec
type EdnsFlags register[16] msb-first {
    DnssecOk: bit,
    CompactAnswersOk: bit,
    DelegationExtensions: bit,
    Reserved: bit[13],
}
```

The implementation must synchronize this declaration against the current IANA
EDNS Header Flags registry.

Temporary assignments must be documented as temporary and must not silently
be treated as permanent semantics after their registry expiry.

## 16.4 Common option data

```sec
type EdnsClientSubnet struct {
    Family: uint16,
    SourcePrefixLength: uint8,
    ScopePrefixLength: uint8,
    Address: byte[],
}

type EdnsCookie struct {
    Client: byte[],
    Server: byte[],
}

type ExtendedDnsError struct {
    InfoCode: uint16,
    ExtraText: Option[string],
}

type UnknownEdnsOption struct {
    Code: EdnsOptionCode,
    Data: byte[],
}
```

`EdnsCookie.Client` must be validated to the size required by RFC 7873.

## 16.5 `EdnsOption`

```sec
type EdnsOption union {
    Nsid(byte[]),
    ClientSubnet(EdnsClientSubnet),
    Cookie(EdnsCookie),
    TcpKeepalive(Option[uint16]),
    Padding(byte[]),
    ExtendedDnsError(ExtendedDnsError),
    Unknown(UnknownEdnsOption),
}
```

Unknown options survive decoding.

## 16.6 `Edns`

```sec
type Edns struct {
    UdpPayloadSize: uint16,
    Version: uint8,
    Flags: EdnsFlags,
    Options: EdnsOption[],
}
```

Revision 0.1 supports EDNS version 0.

A received unsupported version is represented sufficiently to produce or
surface the appropriate protocol error; it must not be interpreted as EDNS(0)
silently.

## 16.7 Default UDP payload size

The standard resolver should use a conservative EDNS UDP payload size intended
to avoid IP fragmentation.

Revision 0.1 selects:

```text
1232 bytes
```

as the default advertised payload size.

The value is configurable.

The implementation must consider current fragmentation-avoidance guidance,
including RFC 9715, and must remain prepared to retry through TCP rather than
assuming that a large UDP payload will arrive successfully.

## 16.8 Privacy defaults

`ClientSubnet` must be **disabled by default**.

The library must not add EDNS Client Subnet data merely to improve CDN
locality.

DNS Cookies may be enabled by classic DNS client policy when implemented.

EDNS Padding is primarily useful for encrypted DNS transports and should not
be added indiscriminately to cleartext UDP messages.

---

# Part V — Encoding and decoding

# 17. `codec.sec`

## 17.1 Responsibility

`codec.sec` owns safe conversion between DNS wire messages and the owning
semantic `Message` representation.

Primary references:

- RFC 1035
- RFC 2181
- RFC 3597
- RFC 6891

## 17.2 Owning decode

```sec
fn DecodeMessageToOwned(
    buffer: ref byte[]
) Result[Message, MessageError]
```

This function may allocate because it materializes:

- names;
- question arrays;
- record arrays;
- RDATA arrays;
- unknown RDATA;
- EDNS option arrays.

Allocation is visible in the function name through `ToOwned`.

## 17.3 Encoding

```sec
fn EncodeMessage(
    message: ref Message,
    output: ref mut byte[]
) Result[uint, MessageError]
```

Rules:

- caller owns output storage;
- no heap allocation is required merely to write encoded bytes when the
  message is already materialized;
- return value is the number of bytes written;
- `OutputTooSmall` is returned when the destination cannot hold the message;
- DNS name compression may be used;
- encoder-generated compression pointers must obey RFC 1035 constraints;
- an implementation may choose not to compress a legal message if the
  uncompressed message fits and protocol semantics are preserved.

## 17.4 Name compression safety

The decoder must:

- validate pointer targets;
- reject pointer loops;
- bound pointer traversal;
- reject names exceeding the post-decompression DNS size limit;
- reject invalid label encodings;
- avoid recursion depth that can exhaust the native stack on malicious input.

No malformed packet may cause unbounded pointer chasing.

## 17.5 Section count safety

Header section counts are attacker-controlled.

Before allocating based on those values, the decoder must:

- validate against remaining message size;
- apply sensible overflow checks;
- reject impossible counts;
- avoid direct "count * structure size" allocation without validation.

## 17.6 Unknown data

Unknown RR types and EDNS options are not decode failures solely because their
semantic formats are unknown.

Their wire data is preserved in the corresponding `Unknown` variant.

## 17.7 Future zero-copy view

A borrowed zero-copy `MessageView`/cursor API remains an observation item.

It should only be added when compressed-name access can be specified cleanly
without creating lifetime or hidden-allocation traps.

The owning decoder is the revision-0.1 portable baseline.

---

# Part VI — Reverse DNS

# 18. `reverse.sec`

## 18.1 Responsibility

`reverse.sec` owns conversion between IP addresses and reverse-DNS query names,
plus the corresponding resolver convenience method.

Primary references include the current standards for:

- `in-addr.arpa`;
- `ip6.arpa`.

## 18.2 Reverse query name

```sec
fn ReverseName(
    address: ip.Address
) Name
```

This function is pure and infallible for valid `ip.Address`.

Examples conceptually map:

```text
192.0.2.1
-> 1.2.0.192.in-addr.arpa.

IPv6 address
-> reversed nibble sequence under ip6.arpa.
```

The function must not perform a DNS query.

## 18.3 PTR lookup

Resolver convenience:

```sec
impl Resolver {
    fn ReverseLookup(
        address: ip.Address
    ) Result[Name[], ResolveError]
}
```

Multiple PTR records remain representable.

The library must not assume one reverse name per IP address.

---

# Part VII — Classic DNS transport

# 19. `transport_udp.sec`

## 19.1 Responsibility

`transport_udp.sec` owns classic DNS message exchange over UDP.

Primary references:

- RFC 1035
- RFC 6891
- RFC 9715

It uses:

```sec
ip.UdpSocket
ip.Endpoint
```

from `net/ip`.

## 19.2 Transport endpoint

Classic DNS server endpoints are typed IP endpoints.

The conventional port is represented through:

```sec
static let DefaultPort: ip.Port := ip.Port(53)
```

owned by an appropriate DNS type/impl, not scattered as magic integer `53`
throughout the package.

## 19.3 UDP exchange semantics

The transport must:

- send exactly one DNS query datagram;
- receive at most one response datagram per receive operation;
- validate source endpoint according to client policy;
- validate transaction identity;
- validate that the response can correspond to the request;
- surface datagram truncation;
- honor the DNS TC bit;
- respect EDNS UDP payload sizing;
- integrate cancellation/deadline semantics;
- avoid silent acceptance of unrelated datagrams.

## 19.4 UDP source validation

By default, a query sent to one concrete recursive server endpoint expects the
response from that server endpoint.

A future anycast/multi-server transport policy may relax address handling only
through an explicit design.

## 19.5 Truncation

A UDP result with:

- IP/socket-level receive truncation; or
- DNS `TC` bit set

must not be returned as though it were a complete ordinary answer.

The higher client policy decides whether to:

- retry using TCP; or
- return `TransportError.TruncatedWithoutFallback`.

---

# 20. `transport_tcp.sec`

## 20.1 Responsibility

`transport_tcp.sec` owns DNS transport over TCP.

Primary reference:

- RFC 7766

It uses:

```sec
ip.TcpStream
```

## 20.2 Message framing

DNS over TCP prefixes each DNS message with its required two-octet message
length framing.

The framing is a DNS transport detail and is not included in
`EncodeMessage()` output for ordinary DNS wire messages.

The TCP transport adds/removes the length field.

## 20.3 TCP requirements

The implementation must support:

- TCP as a first-class DNS transport;
- receiving one framed message across multiple TCP reads;
- multiple DNS messages on one TCP connection;
- connection reuse where enabled;
- matching responses to outstanding queries;
- cancellation and close semantics inherited from `net/ip`;
- the 65535-byte DNS message framing limit.

Revision 0.1 client implementation may serialize outstanding requests on one
connection initially.

Pipelining/concurrent outstanding queries is required before claiming a
high-performance general TCP DNS client.

## 20.4 TCP is not merely emergency fallback

RFC 7766 requires support for both UDP and TCP in general-purpose DNS
implementations.

The client policy may select TCP directly.

The standard resolver may use UDP first and retry TCP for truncation, but that
is a policy, not a protocol limitation.

---

# Part VIII — DNS client

# 21. `client.sec`

## 21.1 Responsibility

`client.sec` owns direct classic DNS query exchange against concrete DNS
servers.

It is lower level than `Resolver`.

A `Client` sends DNS questions. It does not:

- apply search domains automatically;
- treat host files as DNS records;
- synthesize application connection policy;
- perform full iterative recursion from the DNS root.

## 21.2 Transport policy

Classic built-in transport choice is closed for this base client:

```sec
enum ClassicTransport {
    UdpThenTcp,
    UdpOnly,
    TcpOnly,
}
```

This enum does not claim to enumerate every standardized DNS transport.

DoT, DoH, and DoQ are separate child packages and therefore do not need to be
forced into this base enum.

## 21.3 `ClientOptions`

```sec
type ClientOptions struct {
    Transport: ClassicTransport,
    UseEdns: bool,
    UdpPayloadSize: uint16,
    RecursionDesired: bool,
    DnssecOk: bool,
}
```

Canonical defaults:

```text
Transport       = UdpThenTcp
UseEdns         = true
UdpPayloadSize  = 1232
RecursionDesired = true
DnssecOk        = false
```

`DnssecOk` controls the EDNS DO signal. It does **not** enable local DNSSEC
validation.

## 21.4 `Client`

```sec
@noCopy
type Client struct {
    // Private sockets, reusable TCP state, random-ID state, and options.
}
```

Construction:

```sec
impl Client {
    static fn New(
        server: ip.Endpoint,
        options: ClientOptions
    ) Result[Client, TransportError]

    property Server: ip.Endpoint { get { ... } }

    fn Exchange(
        request: ref Message
    ) Result[Message, TransportError]

    fn Query(
        question: Question
    ) Result[Message, TransportError]

    fn Close() Result[void, TransportError]
}
```

Rules:

- `Client` owns reusable transport state and is move-only;
- `Query` creates the request envelope around one question;
- `Exchange` supports callers that need explicit message flags/sections;
- a syntactically valid DNS response with `NXDomain`, `ServFail`, or another
  RCODE is still returned as `Message`;
- UDP truncation follows `ClassicTransport`;
- `Close()` is idempotent;
- deterministic destruction releases live socket resources.

## 21.5 Transaction IDs

The client generates transaction IDs unless the caller explicitly constructs
a full message for `Exchange`.

For generated IDs:

- IDs must not use a trivially predictable global sequence as the only
  spoofing defense for UDP;
- response ID must match;
- question matching must also be checked;
- source endpoint validation remains required.

## 21.6 One-question convenience

`Query` sends one question because that is the interoperable ordinary DNS
usage model.

`Exchange` remains available for standards/features that require explicit
message construction.

The library must not imply that every server handles arbitrary multi-question
queries simply because the count field can encode them.

---

# Part IX — Resolver and cache

# 22. `cache.sec`

## 22.1 Responsibility

`cache.sec` owns the resolver's internal positive and negative cache
semantics.

Primary references:

- RFC 1034
- RFC 1035
- RFC 2181
- RFC 2308
- RFC 8767

The initial cache implementation is module-private.

There is no requirement for applications to manipulate raw cache entries.

## 22.2 Positive caching

Positive RRsets are cached according to DNS TTL semantics.

Cache expiration must use monotonic elapsed-time measurement where possible,
not be corrupted by ordinary wall-clock adjustments.

## 22.3 Negative caching

Negative responses are cached according to RFC 2308 semantics.

`NXDomain` and "name exists but requested type does not" must not be collapsed
into one cache state.

## 22.4 TTL limits

A resolver may impose implementation safety caps consistent with current DNS
standards.

It must not turn an untrusted huge TTL into an effectively permanent cache
entry.

## 22.5 Serve stale

RFC 8767 stale-answer behavior is **disabled by default** in revision 0.1.

It may later become an explicit resolver policy.

A resolver must not silently return expired data as current merely because a
refresh failed.

---

# 23. `resolver.sec`

## 23.1 Responsibility

`resolver.sec` owns Sec's high-level **stub resolver**.

It sends queries to configured recursive DNS servers.

It is not initially a full iterative recursive resolver.

## 23.2 Resolver configuration

```sec
type ResolverConfig struct {
    Servers: ip.Endpoint[],
    SearchDomains: Name[],
    Attempts: uint8,
    RotateServers: bool,
    UseEdns: bool,
    UdpPayloadSize: uint16,
}
```

`Servers` contains concrete DNS server endpoints.

For conventional system DNS configuration, port 53 is inserted where the
platform configuration only supplies addresses.

`Attempts` must have a bounded implementation-defined maximum to prevent
unbounded retry multiplication.

## 23.3 `Resolver`

```sec
@noCopy
type Resolver struct {
    // Private configuration, clients, cache, and platform/system policy.
}
```

Construction:

```sec
impl Resolver {
    static fn System() Result[Resolver, ResolveError]

    static fn New(
        config: ResolverConfig
    ) Result[Resolver, ResolveError]

    fn Resolve(
        name: Name,
        recordType: RecordType
    ) Result[Record[], ResolveError]

    fn LookupAddresses(
        host: HostName
    ) Result[ip.Address[], ResolveError]

    fn LookupIpv4(
        host: HostName
    ) Result[ip.Ipv4Address[], ResolveError]

    fn LookupIpv6(
        host: HostName
    ) Result[ip.Ipv6Address[], ResolveError]

    fn LookupCanonicalName(
        name: Name
    ) Result[Name, ResolveError]

    fn LookupNameServers(
        name: Name
    ) Result[NameServerRecord[], ResolveError]

    fn LookupMailExchanges(
        name: Name
    ) Result[MailExchangeRecord[], ResolveError]

    fn LookupText(
        name: Name
    ) Result[TextRecord[], ResolveError]

    fn ReverseLookup(
        address: ip.Address
    ) Result[Name[], ResolveError]

    fn ClearCache()

    fn Close() Result[void, ResolveError]
}
```

## 23.4 `Resolve`

`Resolve` is the general typed DNS RR lookup.

Rules:

- it performs DNS resolution through configured recursive servers;
- it may use the internal cache;
- it returns only records relevant to the requested semantic result;
- it does not expose a non-success RCODE as an empty successful array;
- unknown RR types can still be returned through `RecordData.Unknown`;
- query/meta types requiring multi-message protocol behavior, such as AXFR,
  are not automatically supported merely because `RecordType` can represent
  them.

## 23.5 Address lookup

`LookupAddresses` performs the high-level A/AAAA host lookup.

It may:

- consult configured static host mappings when `Resolver.System()` uses the
  selected platform's normal host-resolution policy;
- query A and AAAA;
- return multiple addresses.

It must not:

- discard one address family merely because another succeeded;
- invent connection ordering policy such as Happy Eyeballs;
- establish network connections.

Connection-selection/racing belongs to the consumer using `net/ip`.

`LookupIpv4` and `LookupIpv6` are explicit family-specific operations.

## 23.6 System resolver semantics

`Resolver.System()` may integrate the selected platform's standard host-name
resolution policy for high-level host lookups.

That may include platform facilities such as:

- static host mappings;
- configured recursive DNS servers;
- platform resolver services.

However:

```sec
resolver.Resolve(name, recordType)
```

must retain DNS RR semantics.

A platform API that cannot perform arbitrary RR lookup must not fabricate
those records from a host-entry API. The implementation must route general RR
queries through DNS protocol facilities when necessary.

This distinction allows applications such as HTTP to honor normal host
configuration while preserving a real DNS API for callers that ask for MX,
TXT, SRV, HTTPS, DNSSEC records, or unknown RR types.

## 23.7 Search domains

The `Name` type is always absolute.

Search-list behavior therefore belongs to separate resolver input policy, not
to `Name`.

Revision 0.1 does not overload `Name.Parse("host")` to remember whether the
user typed a trailing dot.

A later explicit search-name type/helper may be introduced if needed.

For now, high-level `HostName` input without a root dot may be combined with
the configured system search policy inside `Resolver.System()` while the
resulting DNS query names are always concrete absolute `Name` values.

The exact source-level API for explicitly requesting search-list expansion
remains an observation item.

## 23.8 CNAME behavior

Address lookup follows CNAME chains according to DNS rules with:

- a bounded chain length;
- loop detection;
- cache integration.

A CNAME loop is a resolution error, not an infinite query sequence.

The exact public error variant for a CNAME loop remains an observation item
and may be added to `ResolveError`.

## 23.9 Retry behavior

The resolver uses bounded retries across configured servers.

It must avoid:

- infinite retry loops;
- multiplying `Attempts * SearchDomains * Servers * RecordTypes` without
  implementation caps;
- immediately treating one transient server failure as authoritative negative
  DNS data.

## 23.10 Cancellation

Resolver lookup operations are cancellation points according to Sec's normal
execution model.

Cancellation must not:

- insert an incomplete response into the cache;
- publish a partially constructed result array;
- convert cancellation into `NXDomain` or `ServFail`.

---

# 24. `system.<target>.sec`

## 24.1 Responsibility

Target-specific system files discover and integrate the platform's resolver
configuration and host-resolution facilities required by `Resolver.System()`.

Examples may include:

```text
system.linux.amd64.sec
system.linux.arm64.sec
system.macos.arm64.sec
system.freebsd.amd64.sec
```

Exact target filename coverage follows the repository's platform naming
rules.

## 24.2 Possible platform inputs

Depending on target, implementation may use:

- `/etc/resolv.conf`;
- platform resolver APIs;
- system resolver services;
- `/etc/hosts` or equivalent static mappings;
- target/RTOS network configuration;
- application-supplied DNS server configuration on embedded targets.

No one Unix file format is the semantic API contract.

## 24.3 Bare-metal and embedded targets

Pure DNS name/message/codec APIs can exist without an active resolver.

`Resolver.System()` requires a target-provided DNS configuration source or
network-stack integration.

A bare-metal target with an IP stack but no global resolver configuration may
require `Resolver.New(config)`.

A target with no active IP networking must reject active resolution use
through the platform-capability model rather than fake success.

---

# Part X — DNSSEC boundary

# 25. DNSSEC wire support versus validation

The base package supports enough DNSSEC semantics to:

- request DNSSEC records using EDNS DO;
- parse DNSKEY, DS, RRSIG, NSEC, NSEC3, and NSEC3PARAM;
- preserve DNSSEC algorithms;
- observe AD/CD signaling;
- return DNSSEC records to applications.

This does **not** mean:

```text
response.Flags.AuthenticatedData == trustworthy
```

in every context.

The AD bit is a claim from the resolver that performed validation.

Whether the caller may trust that claim depends on the trust relationship to
that resolver and transport.

## 25.1 Local validation package

Full local validation belongs under:

```text
net/dns/dnssec
```

That package is expected to own:

- trust anchors;
- chain-of-trust construction;
- DS/DNSKEY matching;
- RRSIG verification;
- authenticated denial of existence;
- NSEC/NSEC3 validation;
- clock/time checks;
- cryptographic algorithm policy;
- secure/insecure/bogus/indeterminate outcomes;
- integration with current IANA algorithm policy.

It will receive its own `std-net-dns-dnssec.md`.

This keeps the base DNS codec useful on constrained targets without forcing a
large cryptographic validator into every DNS user.

---

# Part XI — Secure transports

# 26. DNS over TLS

Reserved package:

```text
net/dns/dot
```

Primary standard:

https://www.rfc-editor.org/rfc/rfc7858

The detailed package is deferred until the canonical TLS package ownership in
Sec stdlib is decided.

The base package must not create a second TLS implementation for DNS.

---

# 27. DNS over HTTPS

Reserved package:

```text
net/dns/doh
```

Primary standard:

https://www.rfc-editor.org/rfc/rfc8484

DoH belongs in a child package because it requires `net/http`.

This prevents the base `net/dns` package from importing `net/http` while HTTP
itself depends on name resolution.

DoH must reuse:

- base DNS message encoding;
- `net/http`;
- the canonical TLS/HTTPS stack.

It must not duplicate an HTTP client internally.

---

# 28. DNS over QUIC

Reserved package:

```text
net/dns/doq
```

Primary standard:

https://www.rfc-editor.org/rfc/rfc9250

DoQ will reuse the canonical `net/quic` package.

It is not implemented inside classic `transport_udp.sec` merely because QUIC
uses UDP underneath.

---

# Part XII — Exclusions and future branches

# 29. mDNS

Multicast DNS belongs to:

```text
net/mdns
```

rather than being an alternate mode flag on `dns.Resolver`.

mDNS differs in:

- multicast transport;
- link-local scope;
- query/response behavior;
- cache rules;
- probing/conflict semantics.

The shared wire concepts may be reused without merging package semantics.

---

# 30. DNS-SD

DNS Service Discovery belongs to:

```text
net/dnssd
```

It may reuse:

- `dns.Name`;
- SRV;
- TXT;
- PTR;
- mDNS where applicable.

It is an application/discovery model above raw DNS records.

---

# 31. Dynamic DNS Update

RFC 2136 UPDATE is a live DNS protocol feature.

Revision 0.1 preserves the `Opcode.Update` value and can encode/decode UPDATE
messages through the generic message model.

A higher-level update API is not yet locked.

Possible future package:

```text
net/dns/update
```

if update prerequisites, authorization, TSIG/SIG(0), and update section
semantics become substantial enough to justify a child package.

---

# 32. Zone transfers

AXFR and IXFR are represented as query TYPE values.

A complete transfer can span multiple TCP DNS messages and therefore cannot be
implemented correctly as a simple `Resolver.Resolve(..., AXFR)` array return.

A future transfer API must define:

- TCP session ownership;
- AXFR completion;
- IXFR state;
- SOA framing rules;
- cancellation;
- size limits;
- streaming rather than forced whole-zone allocation.

Possible future package:

```text
net/dns/zone
```

or a dedicated transfer child if needed.

---

# 33. Authoritative and recursive DNS servers

The base stdlib does not initially include a complete authoritative or
recursive DNS server framework.

A full server involves substantial independent concerns:

- zone storage;
- authoritative-answer rules;
- recursive iteration;
- caching;
- bailiwick;
- DNSSEC signing/validation;
- rate limiting;
- amplification defense;
- ACLs;
- EDNS policy;
- cookies;
- update authorization;
- zone transfer;
- persistence.

These are large enough to become external/framework-scale packages unless a
later Sec stdlib use case demonstrates a compact broadly useful server
primitive.

Wire codecs remain reusable by such packages.

---

# 34. Full recursive resolver

`Resolver` is initially a **stub resolver**.

It sends requests to configured recursive servers.

It does not walk:

```text
root -> TLD -> authoritative server
```

itself.

A full recursive implementation requires substantially more behavior,
including:

- delegation following;
- bailiwick;
- cache hierarchy;
- lame delegation handling;
- QNAME minimization;
- DNSSEC validation;
- aggressive negative caching;
- resilience policy.

That belongs in a later specialized package or external library.

Therefore RFC 9156 QNAME minimization is relevant to future recursive-resolver
work but is not pretended to be performed by the revision-0.1 stub resolver.

---

# Part XIII — Resource, cancellation, and concurrency semantics

# 35. Ownership

The following are owning resources:

```sec
Client
Resolver
```

and are declared `@noCopy`.

They may own:

- UDP sockets;
- reusable TCP streams;
- cache storage;
- transaction state;
- platform resolver handles.

`Name`, `HostName`, `Question`, and individual RR data values are semantic
values and are not network handles.

## 35.1 Explicit close

`Client.Close()` and `Resolver.Close()` are explicit and idempotent.

Deterministic destruction still releases owned networking resources.

Applications need explicit close only when they want:

- early release; or
- observable close errors.

---

# 36. Cancellation and deadlines

Potentially blocking DNS operations include:

- UDP response waits;
- TCP connect;
- TCP framed reads;
- TCP writes;
- resolver retries;
- system resolver calls where the platform makes them cancellable.

They integrate with the canonical Sec task/thread cancellation model.

No DNS-specific cancellation token is introduced.

Cancellation must resolve atomically against completion:

- a committed complete response may be returned;
- otherwise cancellation wins according to the common execution semantics;
- an incomplete response is never cached as successful.

---

# 37. Concurrency

A `Resolver` may serve multiple logical lookups when its implementation
provides safe internal synchronization.

Revision 0.1 does not rely on freely copying the owning resolver.

The exact callable receiver/borrowing form used to enable concurrent shared
resolution must align with the final Sec concurrency/borrowing rules and may
be refined during implementation.

A `Client` may initially serialize classic UDP/TCP exchanges.

High-performance TCP pipelining is an implementation milestone, not permission
to violate RFC 7766 response matching.

---

# Part XIV — Security and robustness

# 38. DNS spoofing resistance

Classic UDP DNS is unauthenticated.

The implementation must at least use the standard defensive information
available to it:

- unpredictable transaction IDs;
- server endpoint validation;
- question matching;
- appropriate source-port behavior from the IP/socket layer;
- bounded retry policy.

The library must not claim classic UDP DNS to be cryptographically secure.

Applications requiring authenticated transport should use DoT/DoH/DoQ when
those child packages are available, and applications requiring authenticated
DNS data may additionally require DNSSEC validation.

---

# 39. Compression attacks

Malformed name compression is attacker-controlled input.

The codec must prevent:

- pointer loops;
- unbounded pointer chains;
- pointer-to-invalid-offset reads;
- label-length overrun;
- post-decompression name overflow;
- stack exhaustion through recursive parsing.

---

# 40. Allocation attacks

Message counts and RDLENGTH fields are attacker controlled.

The decoder/resolver must use bounded allocation and overflow checks.

A small received packet claiming enormous section counts must fail before
attempting enormous allocations.

---

# 41. Cache poisoning and additional data

A resolver must not treat arbitrary additional-section records as trusted
answers merely because they are present.

Bailiwick and full recursive cache rules become especially relevant to a
future recursive resolver.

The stub resolver should cache only records selected by its defined response
processing rules.

---

# 42. DNSSEC claims

The base package must never equate:

```text
AD bit present
```

with:

```text
cryptographically validated locally
```

The distinction must remain visible in APIs and documentation.

---

# Part XV — Repository migration

# 43. Current repository state

The repository tree was rechecked on 2026-09-07.

Current DNS directory:

```text
sec/stdlib/net/dns/
└── dns.sec
```

Current file contents:

```sec
module dns
```

The file is one line and contains no API implementation.

No existing DNS types or methods therefore require source compatibility during
this first package definition.

---

# 44. Required migration

The implementation task should:

1. create `sec/stdlib/net/dns/std-net-dns.md`;
2. create the base-package source files declared by this book;
3. move the package from a one-line stub to those functional files;
4. remove the obsolete empty `dns.sec` once at least one real module source
   file exists;
5. create the child directories declared by this book;
6. create child rulebook files only when their corresponding package design is
   actually written;
7. not invent DoH/DoT/DoQ APIs merely because their directories are reserved;
8. reuse `net/ip` address, port, endpoint, TCP, UDP, and network-error types;
9. update implementation status as each layer becomes real.

---

# Part XVI — Tests

# 45. Test placement

Expected base-package test families:

```text
name_test.sec
idna_test.sec
registry_test.sec
message_test.sec
record_test.sec
edns_test.sec
codec_test.sec
reverse_test.sec
transport_udp_test.sec
transport_tcp_test.sec
client_test.sec
cache_test.sec
resolver_test.sec
```

Target-specific integration tests may be added where needed.

---

# 46. Required name tests

At minimum:

- root name;
- ordinary absolute name;
- input with and without final dot;
- maximum legal label;
- overlong label;
- maximum legal encoded name;
- overlong encoded name;
- escaped presentation characters;
- malformed escapes;
- interior empty label rejection;
- DNS ASCII case-insensitive equality;
- case-preserving `ToString`;
- canonical `ToCanonicalString`;
- a valid DNS name rejected as `HostName` when host rules are stricter.

---

# 47. Required IDNA tests

At minimum:

- ASCII host;
- valid IDNA2008 internationalized host;
- A-label round trip;
- invalid code point;
- bidi failure;
- invalid A-label;
- root/empty edge cases;
- no accidental locale-sensitive transformation.

Reference test vectors should be used where standards provide them.

---

# 48. Required registry tests

At minimum:

- assigned RR type values match IANA;
- assigned class values match IANA;
- opcode values match IANA;
- RCODE values match IANA;
- unknown 16-bit RR type remains representable;
- unknown EDNS option remains representable;
- deprecated assigned values decode without becoming recommended APIs;
- DNSSEC algorithm values match the synchronized IANA registry.

---

# 49. Required codec tests

At minimum:

- empty/minimum DNS header;
- one-question query;
- A answer;
- AAAA answer;
- compressed owner names;
- compressed RDATA names;
- pointer loop;
- pointer out of range;
- excessive compression depth;
- malformed label length;
- oversized decoded name;
- impossible section counts;
- invalid RDLENGTH;
- unknown RR preservation;
- unknown EDNS option preservation;
- OPT extraction from additional section;
- multiple OPT rejection;
- extended RCODE composition;
- output buffer too small;
- encode/decode round trip for all implemented common record types.

Fuzz testing of DNS decode paths is strongly recommended.

---

# 50. Required EDNS tests

At minimum:

- no EDNS;
- EDNS0 with default payload size;
- DO flag;
- current assigned EDNS flags;
- unknown EDNS flags preserved/rejected according to standard rules;
- NSID;
- Cookie valid/invalid lengths;
- Padding;
- Extended DNS Error;
- unknown option;
- Client Subnet decode;
- Client Subnet absent by default from generated resolver queries.

---

# 51. Required UDP tests

At minimum:

- IPv4 DNS server;
- IPv6 DNS server;
- valid matching response;
- mismatched ID;
- mismatched question;
- response from unexpected endpoint;
- DNS TC bit;
- socket-level receive truncation;
- default EDNS payload size;
- cancellation;
- timeout/network-error propagation.

---

# 52. Required TCP tests

At minimum:

- two-octet framing;
- message split across multiple TCP reads;
- multiple messages on one connection;
- connection reuse;
- exact maximum framing boundary;
- premature EOF;
- mismatched response;
- cancellation;
- close;
- fallback from truncated UDP.

---

# 53. Required resolver tests

At minimum:

- A lookup;
- AAAA lookup;
- combined address lookup;
- CNAME chain;
- CNAME loop protection;
- NXDOMAIN;
- NODATA;
- SERVFAIL not cached as authoritative negative data;
- multiple configured recursive servers;
- bounded retries;
- positive cache TTL expiry;
- negative cache expiry;
- cache clear;
- reverse IPv4;
- reverse IPv6;
- static/system host mapping behavior where supported;
- arbitrary RR lookup;
- unknown RR lookup;
- no connection ordering policy hidden in address lookup;
- cancellation does not cache incomplete result.

---

# Part XVII — AI/repository implementation instructions

# 54. Scaffolding and implementation requirements

An AI or repository automation implementing this book should:

1. treat `std-net-dns.md` as the normative package document;
2. create missing declared base files;
3. preserve/expand existing useful code where present;
4. recognize that current `dns.sec` is only a stub and may be removed;
5. create reserved child directories;
6. not invent child-package implementation before its local book exists;
7. add standards headers and authoritative URLs to each source file;
8. synchronize registry enums against current IANA registries;
9. preserve unknown open-registry values;
10. reuse `net/ip` types rather than duplicate IP addresses, ports, endpoints,
    TCP, UDP, or network errors;
11. implement DNS TCP support rather than shipping a UDP-only "complete"
    resolver;
12. implement EDNS0 as first-class semantics;
13. keep EDNS Client Subnet disabled by default;
14. keep DNSSEC wire parsing distinct from DNSSEC validation;
15. avoid a `dns -> http` dependency in the base package;
16. use bounded parsing/allocation defenses;
17. test malicious compression and malformed count fields;
18. update `implementation-status-std-net-dns.yaml`;
19. reconcile status evidence into canonical `implementation-status.yaml`
    without duplicating newer/stronger evidence;
20. report compiler/platform blockers rather than mock successful networking.

---

# 55. Recommended implementation order

1. create package/book/source layout;
2. implement `error.sec`;
3. implement `name.sec`;
4. implement `registry.sec` from IANA;
5. implement fixed header/message structures;
6. implement basic RR data;
7. implement generic unknown RR support;
8. implement EDNS;
9. implement codec;
10. implement reverse-name generation;
11. implement UDP classic transport;
12. implement TCP framing/transport;
13. implement `Client`;
14. implement resolver cache;
15. implement system resolver configuration adapter;
16. implement stub `Resolver`;
17. implement IDNA2008;
18. expand modern service records;
19. complete DNSSEC wire records;
20. create child books for DoT/DoH/DoQ/DNSSEC validation/zone functionality.

Codec/value work must not be blocked on active networking support.

---

# Part XVIII — Open observations

# 56. Items intentionally not fully locked in revision 0.1

The following require later design or dogfooding:

- explicit search-list input type/API;
- exact CNAME-loop error variant;
- platform-specific system-host-resolution precedence;
- whether `Resolver.System()` should expose an option to bypass host mappings;
- TCP query pipelining API versus internal implementation only;
- DNS Cookies default enablement;
- exact public representation of all TLSA/SSHFP registry subfields;
- full current SVCB parameter typed union;
- zero-copy `MessageView`;
- streaming decode for very large messages;
- authoritative server primitive;
- dynamic update child package;
- zone-transfer streaming API;
- TSIG and SIG(0) ownership;
- DNS Stateful Operations (DSO);
- serve-stale resolver option;
- resolver cache size/eviction configuration;
- full recursive resolver placement;
- aggressive negative caching;
- QNAME minimization in a future recursive resolver;
- interaction between strict IDNA2008 and later URL/web compatibility mapping;
- whether system host-resolution APIs should eventually receive a separate
  cross-protocol `net` package abstraction.

These observations are not permission for implementations to invent
incompatible public APIs.

---

# 57. Decisions established by this revision

Revision 0.1 establishes:

- canonical package path `net/dns`;
- package qualifier `dns`;
- `dns.Name` is an absolute DNS name;
- `dns.HostName` is stricter and explicitly supports IDNA2008 conversion;
- DNS names and host names are not represented by unrestricted strings in
  typed public APIs;
- IANA TYPE/CLASS/Opcode/RCODE/EDNS/DNSSEC namespaces use open fixed-width
  values;
- unknown RR types and EDNS options survive decoding;
- A/AAAA reuse `net/ip` address types;
- SRV ports reuse `ip.Port`;
- TXT is octet data, not automatically UTF-8;
- owning message decode is explicit through `DecodeMessageToOwned`;
- encoder writes into caller-provided storage;
- name compression is defensively validated;
- EDNS0 is first-class;
- default EDNS UDP payload size is 1232 bytes;
- EDNS Client Subnet is disabled by default;
- classic general-purpose DNS supports UDP and TCP;
- the direct `Client` can return valid nonzero-RCODE messages;
- `Resolver` is initially a stub resolver, not a full recursive resolver;
- positive and negative caching follow DNS TTL semantics;
- DNSSEC wire support does not imply local validation;
- full local validation belongs in `net/dns/dnssec`;
- DoT, DoH, and DoQ are child packages;
- DoH is kept out of the base package to avoid dependency-cycle pressure with
  `net/http`;
- mDNS and DNS-SD remain separate networking packages;
- active resolver capability depends on target/platform networking support;
- current one-line `dns.sec` is a removable stub, not a compatibility
  contract.

---

**Document revision:** 0.1  
**Last updated:** 2026-09-07
