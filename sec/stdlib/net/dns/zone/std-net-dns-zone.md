# Sec Standard Library — DNS Zones and Master Files

- **Status:** Draft
- **Created:** 2026-09-07
- **Last updated:** 2026-09-07
- **Document revision:** 0.1
- **Sec language version:** 0.1
- **Canonical path:** `stdlib/net/dns/zone/std-net-dns-zone.md`
- **Repository path:** `sec/stdlib/net/dns/zone/std-net-dns-zone.md`
- **Parent specification:** `stdlib/net/dns/std-net-dns.md`
- **Implementation-status fragment:** `implementation-status-std-net-dns-zone.yaml`
- **Latest verified repository main:** `0f5027d`
- **Standards state rechecked:** 2026-09-07

---

# 1. Purpose

`net/dns/zone` owns Sec's in-memory DNS zone model and standards-based DNS
master-file parsing/writing.

It owns:

- zone apex identity;
- authoritative RRset storage;
- DNS SOA serial-number semantics;
- zone construction and immutable/validated snapshots;
- zone semantic validation;
- DNS master-file tokenization;
- RFC 1035 master-file parsing;
- `$ORIGIN`;
- `$INCLUDE`;
- RFC 2308 `$TTL`;
- owner-name inheritance;
- TTL/class/type field parsing;
- generic unknown-RR presentation format from RFC 3597;
- canonical/stable master-file writing;
- safe caller-controlled include resolution;
- source locations and diagnostics;
- integration points for DNSSEC signing and authoritative server code.

The package is imported as:

```sec
import "net/dns/zone"
```

Example:

```sec
let parser := zone.MasterParser.New(zone.ParserOptions.Default())
let parsed := try parser.Parse(text, try dns.Name.Parse("example.com."))
let snapshot := try parsed.Build()
```

The base zone package is deliberately usable without:

- sockets;
- TLS;
- QUIC;
- authoritative server runtime;
- zone transfer;
- dynamic DNS UPDATE;
- TSIG;
- DNSSEC private keys.

Those concerns are separated below.

---

# 2. Package taxonomy

## 2.1 Base package

This book defines:

```text
net/dns/zone
```

for:

```text
zone model
RRset storage
SOA serial semantics
master-file parser/writer
zone validation
source diagnostics
```

## 2.2 Reserved child packages

Substantial active protocols are separated:

```text
net/dns/zone/xfr
net/dns/zone/update
```

Future books:

```text
net/dns/zone/xfr/std-net-dns-zone-xfr.md
net/dns/zone/update/std-net-dns-zone-update.md
```

### `zone/xfr`

Expected ownership:

- AXFR;
- IXFR;
- DNS NOTIFY coordination;
- TCP transfer;
- TLS-protected zone transfer;
- DoQ transfer adapter;
- streaming transfer;
- transfer authentication hooks.

### `zone/update`

Expected ownership:

- RFC 2136 Dynamic DNS UPDATE;
- prerequisites;
- update sections;
- secure update integration;
- TSIG/authentication hooks;
- atomic zone mutation application.

This separation keeps simple zone-file manipulation free from network
dependencies.

## 2.3 TSIG

TSIG is a general DNS transaction-authentication mechanism, not only a zone
feature.

It should therefore be a sibling DNS package, likely:

```text
net/dns/tsig
```

rather than embedded in `net/dns/zone`.

Current standard:

- RFC 8945 — Secret Key Transaction Authentication for DNS (TSIG)  
  https://www.rfc-editor.org/rfc/rfc8945

RFC 8945 obsoletes RFC 2845.

The exact package path is reserved for later explicit design.

---

# 3. Standards

## 3.1 DNS zone/master-file foundation

Primary standards:

- **RFC 1034 — Domain Names - Concepts and Facilities**  
  https://www.rfc-editor.org/rfc/rfc1034
- **RFC 1035 — Domain Names - Implementation and Specification**  
  https://www.rfc-editor.org/rfc/rfc1035

RFC 1035 section 5 defines DNS master files including:

```text
$ORIGIN
$INCLUDE
resource-record entries
owner-name inheritance
parenthesized continuation
semicolon comments
```

## 3.2 Default TTL

- **RFC 2308 — Negative Caching of DNS Queries**  
  https://www.rfc-editor.org/rfc/rfc2308

RFC 2308 extends the master-file format with:

```text
$TTL
```

The SOA MINIMUM field must not be treated as the generic missing-RR TTL
default in modern master-file parsing.

## 3.3 Unknown RR types

- **RFC 3597 — Handling of Unknown DNS Resource Record Types**  
  https://www.rfc-editor.org/rfc/rfc3597

The parser/writer must support the generic `TYPE####` / `\#` presentation
syntax so future or unknown RR types can be preserved.

## 3.4 DNS terminology

- RFC 9499 / BCP 219 — DNS Terminology  
  https://www.rfc-editor.org/rfc/rfc9499

This book uses current terms such as primary/secondary where applicable rather
than relying on obsolete terminology except when quoting older RFC concepts.

## 3.5 Serial number arithmetic

- **RFC 1982 — Serial Number Arithmetic**  
  https://www.rfc-editor.org/rfc/rfc1982

The SOA serial is a 32-bit serial-number space.

It must not be compared using ordinary naive unsigned greater-than semantics.

## 3.6 Zone transfer references

Owned by the reserved `zone/xfr` child but recorded here:

- RFC 1995 — IXFR  
  https://www.rfc-editor.org/rfc/rfc1995
- RFC 1996 — DNS NOTIFY  
  https://www.rfc-editor.org/rfc/rfc1996
- RFC 5936 — AXFR  
  https://www.rfc-editor.org/rfc/rfc5936
- RFC 9103 — DNS Zone Transfer over TLS  
  https://www.rfc-editor.org/rfc/rfc9103

RFC 9103 updates RFCs 1995 and 5936 and specifies TLS 1.3-or-later zone
transfer protection.

## 3.7 Dynamic update references

Owned by `zone/update`:

- RFC 2136 — Dynamic Updates in the DNS  
  https://www.rfc-editor.org/rfc/rfc2136
- RFC 3007 — Secure Dynamic Update  
  https://www.rfc-editor.org/rfc/rfc3007

## 3.8 DNSSEC interaction

Signed zones use the DNSSEC standards from:

```text
net/dns/dnssec
```

Important zone-level references include:

- RFC 4034 canonical DNS name order and DNSSEC RRs;
- RFC 5155 NSEC3;
- RFC 9276 current NSEC3 parameter guidance;
- RFC 8901 multi-signer operational models.

Zone parsing must preserve DNSSEC records but does not locally validate them
merely by loading a file.

---

# 4. Source-file standards headers

Every source file states the standards it directly implements.

Example:

```sec
// Sec Standard Library: net/dns/zone
// File: master_parser.sec
//
// Standards:
//   RFC 1035 - Domain Names - Implementation and Specification
//   Section 5: Master Files
//   https://www.rfc-editor.org/rfc/rfc1035
//
//   RFC 2308 - Negative Caching of DNS Queries
//   $TTL directive
//   https://www.rfc-editor.org/rfc/rfc2308
//
//   RFC 3597 - Unknown DNS RR Types
//   https://www.rfc-editor.org/rfc/rfc3597

module zone
```

---

# 5. Package and file manifest

Initial base package:

```text
stdlib/net/dns/zone/
├── std-net-dns-zone.md
├── error.sec
├── source.sec
├── serial.sec
├── zone.sec
├── builder.sec
├── validation.sec
├── directive.sec
├── master_token.sec
├── master_parser.sec
├── master_writer.sec
├── include.sec
│
├── xfr/
│   └── std-net-dns-zone-xfr.md       # later book
└── update/
    └── std-net-dns-zone-update.md    # later book
```

All base package source files use:

```sec
module zone
```

Expected tests:

```text
serial_test.sec
zone_test.sec
builder_test.sec
validation_test.sec
master_token_test.sec
master_parser_test.sec
master_writer_test.sec
include_test.sec
```

The child directories are separate packages and are not implemented until
their local books are written.

---

# Part I — Required parent DNS type

# 6. Shared `dns.Rrset`

The zone package and DNSSEC package both require a shared RRset model.

The parent DNS book must define `dns.Rrset` in `net/dns/record.sec`.

The zone package must reuse it.

It must not create:

```sec
zone.Rrset
```

for the same DNS concept.

The required semantic shape is documented in `std-net-dns-dnssec.md` and must
be reconciled into the parent DNS book.

---

# Part II — Errors and source information

# 7. `error.sec`

## 7.1 `ZoneError`

```sec
enum ZoneError error {
    WrongOrigin,
    MissingSoa,
    MultipleSoa,
    SoaNotAtApex,
    MissingApexNs,
    OutOfZoneName,
    DuplicateRecord,
    MixedRrsetTtl,
    InvalidCnameCombination,
    InvalidDnameCombination,
    InvalidDelegation,
    InvalidWildcard,
    InvalidDnssecStructure,
    SerialUndefinedComparison,
    ResourceLimitExceeded,
}
```

`InvalidDnssecStructure` refers to zone-level structural inconsistency, not
cryptographic validation failure.

## 7.2 `MasterFileError`

```sec
enum MasterFileError error {
    UnexpectedToken,
    UnexpectedEnd,
    InvalidOwner,
    InvalidTtl,
    InvalidClass,
    InvalidType,
    InvalidRdata,
    InvalidEscape,
    UnterminatedQuote,
    UnterminatedParentheses,
    InvalidDirective,
    MissingOrigin,
    IncludeNotAllowed,
    IncludeFailed,
    IncludeDepthExceeded,
    IncludeCycle,
    UnknownDirective,
    UnsupportedExtension,
    ResourceLimitExceeded,
}
```

## 7.3 `ZoneLoadError`

```sec
union ZoneLoadError error {
    Master(MasterFileError),
    Name(dns.NameError),
    Zone(ZoneError),
    Dns(dns.MessageError),
}
```

---

# 8. `source.sec`

## 8.1 Responsibility

`source.sec` owns source positions used for master-file diagnostics.

```sec
type SourcePosition struct {
    Line: uint,
    Column: uint,
    Offset: uint,
}
```

```sec
type SourceRange struct {
    Start: SourcePosition,
    End: SourcePosition,
}
```

Optional source identity:

```sec
type SourceId string
```

A parser loaded from an included file may preserve the source identifier for
diagnostics without assuming that it is a filesystem path.

## 8.2 Diagnostic

```sec
type Diagnostic struct {
    Range: SourceRange,
    Message: string,
}
```

The exact common diagnostics integration may later reuse a stdlib/compiler
diagnostic type. Until then, zone parsing must still preserve precise
line/column source information.

---

# Part III — SOA serial arithmetic

# 9. `serial.sec`

## 9.1 Responsibility

`serial.sec` owns DNS SOA serial-number arithmetic.

Primary standard:

https://www.rfc-editor.org/rfc/rfc1982

## 9.2 `Serial`

```sec
type Serial uint32
```

This is not an ordinary monotonic integer.

Canonical API:

```sec
impl Serial {
    init(value: uint32)

    property Value: uint32 { get { ... } }

    fn Add(
        amount: uint32
    ) Result[Serial, ZoneError]

    fn Compare(
        other: Serial
    ) Option[Ordering]
}
```

## 9.3 Undefined comparison

RFC 1982 leaves comparison undefined when values differ by exactly half the
32-bit sequence space.

Therefore:

```sec
Compare(...)
```

returns:

```text
None
```

for the undefined comparison case.

The library must not invent an arbitrary ordering.

## 9.4 Addition

Only increments within RFC 1982's defined range are accepted.

Invalid increments return:

```sec
ZoneError.SerialUndefinedComparison
```

or a later dedicated `SerialError` if the API is refined.

The current error spelling is an observation item; the serial arithmetic
semantics are locked.

---

# Part IV — Zone model

# 10. `zone.sec`

## 10.1 Responsibility

`zone.sec` owns an immutable validated snapshot of one DNS zone.

## 10.2 `Zone`

```sec
type Zone struct {
    _origin: dns.Name,
    _rrsets: dns.Rrset[],
}
```

Canonical public surface:

```sec
impl Zone {
    property Origin: dns.Name { get { ... } }

    property Serial: Serial { get { ... } }

    fn Lookup(
        name: ref dns.Name,
        recordType: dns.RecordType
    ) Option[dns.Rrset]

    fn RrsetsAt(
        name: ref dns.Name
    ) dns.Rrset[]

    fn AllRrsetsToArray()
        dns.Rrset[]
}
```

`Zone` is an immutable snapshot after successful construction.

Mutations occur through `ZoneBuilder`.

## 10.3 Allocation-aware collections

Methods returning owning dynamic arrays must follow the common Sec collection
naming convention.

If the project-wide rule requires explicit `ToArray` naming, the final
canonical methods become:

```sec
fn RrsetsAtToArray(...)
fn AllRrsetsToArray()
```

The exact naming should be synchronized before implementation completion.

## 10.4 Origin

Every authoritative owner name in the zone is either:

- the origin; or
- a descendant of the origin,

subject to DNS delegation/glue rules.

Out-of-zone authoritative data is rejected.

## 10.5 Apex

A valid authoritative zone requires:

- exactly one apex SOA RR;
- an apex NS RRset.

The zone model may load intentionally incomplete fragments through a lower
builder mode, but `Build()` for a normal authoritative `Zone` enforces the
complete-zone invariants.

---

# 11. `builder.sec`

## 11.1 Responsibility

`builder.sec` owns mutable construction before producing an immutable
validated `Zone`.

## 11.2 `ZoneBuilder`

```sec
@noCopy
type ZoneBuilder struct {
    // Private origin, RRset map, source metadata, limits.
}
```

Canonical API:

```sec
impl ZoneBuilder {
    static fn New(
        origin: dns.Name
    ) ZoneBuilder

    fn Add(
        record: dns.Record
    ) Result[void, ZoneError]

    fn AddRrset(
        rrset: dns.Rrset
    ) Result[void, ZoneError]

    fn Remove(
        name: ref dns.Name,
        recordType: dns.RecordType
    ) bool

    fn Build()
        Result[Zone, ZoneError]
}
```

## 11.3 Add semantics

Adding records:

- groups them into shared `dns.Rrset` values;
- rejects duplicate identical records;
- enforces RRset TTL consistency;
- enforces owner/class/type invariants;
- does not silently replace an existing record merely because it has the same
  owner/type.

## 11.4 Build commit

`Build()` validates the complete zone and returns an immutable snapshot only
after validation succeeds.

Failure leaves the builder available according to normal Sec ownership and
transaction semantics.

---

# Part V — Zone validation

# 12. `validation.sec`

## 12.1 Responsibility

`validation.sec` validates structural DNS zone invariants.

It does not perform DNSSEC cryptographic verification.

## 12.2 Required validations

At minimum:

- one SOA at apex;
- no second SOA;
- apex NS exists;
- authoritative names lie within origin;
- RRset TTL consistency;
- no duplicate RRs;
- CNAME coexistence rules;
- DNAME coexistence rules;
- delegation structure;
- wildcard owner syntax;
- class consistency;
- owner-name limits;
- known RR-specific structural constraints;
- DNSSEC RR structural placement where standardized.

## 12.3 CNAME

The validator must enforce current DNS CNAME coexistence rules, including
DNSSEC exceptions such as RRSIG/NSEC material where the DNSSEC standards
permit it.

It must not implement the old simplistic rule:

```text
if CNAME then literally no other RR type whatsoever
```

without considering standards updates.

## 12.4 Unknown RR types

Unknown RR types are allowed.

A zone parser/validator must not reject an RRset merely because Sec has no
semantic decoder for its RDATA.

RFC 3597 generic representation preserves it.

---

# Part VI — Master-file directives

# 13. `directive.sec`

## 13.1 Responsibility

`directive.sec` owns standardized master-file directives.

## 13.2 `Directive`

```sec
type Directive union {
    Origin(dns.Name),
    Ttl(dns.Ttl),
    Include(IncludeDirective),
}
```

Only standardized directives are part of revision 0.1.

## 13.3 `$ORIGIN`

Defined by RFC 1035.

It changes the origin used for subsequent relative owner/RDATA names in the
master-file parse context.

It does not mutate the final zone's apex identity.

An `$ORIGIN` outside the authoritative zone may be syntactically parseable but
records ultimately added to a normal authoritative zone remain subject to
zone validation.

## 13.4 `$TTL`

Defined by RFC 2308.

It supplies the default TTL for subsequent records that do not explicitly
contain one.

The parser must not use SOA MINIMUM as a substitute generic default TTL.

## 13.5 `$INCLUDE`

Defined by RFC 1035.

It includes another master-file source and may supply an alternate origin for
the included content.

The base parser does **not** automatically open arbitrary filesystem paths.

Include resolution is controlled by `include.sec`.

## 13.6 Non-standard directives

Non-standard vendor directives such as:

```text
$GENERATE
```

are not accepted by default.

A future explicit extension mechanism may support them.

Revision 0.1 returns:

```sec
MasterFileError.UnsupportedExtension
```

or `UnknownDirective` as applicable.

---

# Part VII — Includes

# 14. `include.sec`

## 14.1 Security goal

Master files can contain attacker-controlled `$INCLUDE` tokens.

A general parser must not translate those directly into unrestricted local
filesystem access.

## 14.2 `IncludeDirective`

```sec
type IncludeDirective struct {
    Target: string,
    Origin: Option[dns.Name],
}
```

`Target` is intentionally a string because RFC 1035's include file name is an
external source identifier, not a DNS namespace.

## 14.3 `IncludedSource`

```sec
type IncludedSource struct {
    Id: SourceId,
    Text: string,
}
```

## 14.4 `IncludeResolver`

```sec
interface IncludeResolver {
    fn Resolve(
        target: string,
        from: SourceId
    ) Result[IncludedSource, error]
}
```

The caller decides whether targets represent:

- filesystem paths;
- virtual files;
- embedded resources;
- package assets;
- network-fetched configuration.

The zone parser itself does not assume.

## 14.5 Include policy

```sec
type IncludePolicy union {
    Disabled,
    Resolver(ref IncludeResolver),
}
```

The exact lifetime syntax for the stored borrowed interface must follow the
canonical borrowing rules.

If retaining the resolver beyond the call is awkward, `ParseWithIncludes`
may instead receive the resolver as an explicit borrow.

The security semantics are locked; exact storage syntax may be refined.

## 14.6 Include limits

Parser policy must bound:

- include nesting depth;
- include cycles;
- total included bytes;
- total number of included sources.

A cycle returns:

```sec
MasterFileError.IncludeCycle
```

---

# Part VIII — Master-file tokenizer

# 15. `master_token.sec`

## 15.1 Responsibility

`master_token.sec` tokenizes the standard master-file presentation syntax.

## 15.2 Token kinds

```sec
enum MasterTokenKind {
    Atom,
    Quoted,
    OpenParen,
    CloseParen,
    Directive,
    NewLine,
    End,
}
```

Comments are normally discarded after source-position tracking.

## 15.3 Token

```sec
type MasterToken struct {
    Kind: MasterTokenKind,
    Text: string,
    Range: SourceRange,
}
```

Escapes are not necessarily semantically decoded at the token layer if
record-specific parsing needs the original presentation bytes.

## 15.4 Lexical rules

The tokenizer handles:

- spaces/tabs as field separators;
- newline record boundaries;
- semicolon comments outside quoted/escaped context;
- quoted character strings;
- backslash escapes;
- parentheses for multiline logical records;
- directive tokens beginning with `$`;
- source ranges.

A semicolon inside a valid quoted/escaped sequence is not automatically a
comment delimiter.

---

# Part IX — Master-file parser

# 16. `master_parser.sec`

## 16.1 Responsibility

`master_parser.sec` parses RFC master-file text into a `ZoneBuilder`.

## 16.2 `ParserOptions`

```sec
type ParserOptions struct {
    MaxRecords: uint,
    MaxSourceBytes: uint,
    MaxIncludeDepth: uint8,
    AllowUnknownRecordTypes: bool,
}
```

Canonical default:

```sec
impl ParserOptions {
    static fn Default() ParserOptions
}
```

Defaults must permit normal zones while bounding hostile input.

`AllowUnknownRecordTypes` defaults to:

```text
true
```

because RFC 3597 exists specifically to preserve unknown RR types.

## 16.3 `MasterParser`

```sec
@noCopy
type MasterParser struct {
    // Private parser options and temporary parse state.
}
```

Canonical API:

```sec
impl MasterParser {
    static fn New(
        options: ParserOptions
    ) MasterParser

    fn Parse(
        text: string,
        origin: dns.Name
    ) Result[ZoneBuilder, ZoneLoadError]

    fn ParseSource(
        source: IncludedSource,
        origin: dns.Name,
        includes: ref IncludeResolver
    ) Result[ZoneBuilder, ZoneLoadError]
}
```

## 16.4 Initial origin

The caller supplies an explicit zone origin.

There is no hidden process-global DNS origin.

## 16.5 Owner-name inheritance

RFC 1035 allows a blank owner field to inherit the previous explicit owner.

The parser tracks this state explicitly.

At the beginning of a source where no previous owner exists, a blank owner is
an error.

## 16.6 Relative names

Master files permit relative DNS names.

These are resolved against the current `$ORIGIN` during parsing and converted
to absolute `dns.Name` values.

The parent `dns.Name` type remains absolute.

This is why relative names are kept inside zone/master parsing rather than
added to ordinary DNS APIs.

## 16.7 Field ambiguity

Master-file RR fields can omit owner, TTL, and class in standardized ways.

The parser must follow DNS master-file grammar rather than assume fixed column
positions.

It must correctly distinguish:

```text
owner TTL CLASS TYPE RDATA
owner CLASS TTL TYPE RDATA
owner TYPE RDATA
<inherited-owner> ...
```

where permitted.

## 16.8 TTL syntax

Standard DNS TTL values are parsed according to the governing record/master
file specifications.

Any support for human shorthand beyond standardized syntax must be explicitly
documented and not silently accepted as portable RFC master-file syntax.

## 16.9 Parentheses

Parentheses join physical lines into one logical RR entry.

Parentheses do not become bytes in RDATA.

Comments/newlines inside continuation are handled according to master-file
syntax.

## 16.10 Known RR data

Known RR types are parsed into the typed `dns.RecordData` variants owned by
the parent DNS package.

The zone package does not define duplicate A/MX/SRV/etc. record structures.

## 16.11 Unknown RR data

RFC 3597 generic syntax is required.

Unknown record types are represented using:

```sec
dns.RecordData.Unknown(...)
```

and retain exact protocol RDATA bytes after presentation decoding.

---

# Part X — Master-file writer

# 17. `master_writer.sec`

## 17.1 Responsibility

`master_writer.sec` writes `Zone` values as standards-compatible master-file
text.

## 17.2 `WriterOptions`

```sec
type WriterOptions struct {
    EmitOrigin: bool,
    EmitDefaultTtl: bool,
    UseRelativeNames: bool,
    StableOrdering: bool,
}
```

Canonical default:

```sec
impl WriterOptions {
    static fn Default() WriterOptions
}
```

Default:

```text
EmitOrigin       = true
EmitDefaultTtl   = true
UseRelativeNames = true
StableOrdering   = true
```

## 17.3 Writer

```sec
fn WriteMasterFile(
    zone: ref Zone,
    options: WriterOptions
) Result[string, ZoneLoadError]
```

This allocates/materializes a string and the naming makes the operation
explicit enough for a text writer.

A future streaming writer may target a common `io.Writer`.

## 17.4 Stable ordering

Stable output is important for:

- version control;
- reproducible builds;
- review;
- deterministic generated zones.

The default writer emits a deterministic order.

The exact human-oriented ordering need not be identical to DNSSEC canonical
RR ordering.

DNSSEC canonical order is a cryptographic concept owned by
`net/dns/dnssec`.

## 17.5 Unknown RR output

Unknown RR types are emitted using RFC 3597 generic presentation format.

The writer must not lose future/private RR data merely because it lacks a
pretty printer.

## 17.6 `$INCLUDE`

The writer does not recreate original include boundaries by default.

A built `Zone` is a semantic zone snapshot, not a lossless syntax tree.

Writing it emits the effective zone data.

A future syntax-preserving editor would require a separate AST/document model.

---

# Part XI — Syntax tree boundary

# 18. Semantic zone versus source document

Revision 0.1 intentionally distinguishes:

```text
master-file source syntax
```

from:

```text
validated semantic Zone
```

Comments, whitespace, include boundaries, and original field ordering are not
guaranteed to survive:

```text
parse -> build Zone -> write
```

The goal is semantic correctness and stable output.

If Sec later needs a formatting-preserving DNS zone editor, it should add a
separate source-document/AST API rather than pollute the normal `Zone` value
with syntax trivia.

---

# Part XII — DNSSEC integration

# 19. Signed zones

A zone may contain:

```text
DNSKEY
RRSIG
NSEC
NSEC3
NSEC3PARAM
DS
CDS
CDNSKEY
```

The zone parser preserves these using parent `net/dns` record types.

## 19.1 Loading is not validation

Loading a signed zone file does not establish that its signatures are valid.

Cryptographic validation belongs to:

```text
net/dns/dnssec
```

## 19.2 Future signing orchestration

A future zone signing workflow may conceptually:

1. validate semantic zone structure;
2. generate NSEC/NSEC3 chain;
3. obtain signing keys through canonical crypto/key storage;
4. call DNSSEC RRset signing primitives;
5. insert DNSKEY/RRSIG/denial records;
6. update serial according to explicit policy;
7. build a new immutable zone snapshot.

The zone package orchestrates the complete zone.

DNSSEC owns cryptographic/canonical signing rules.

No dnssec->zone dependency is required.

---

# Part XIII — Zone transfer boundary

# 20. `net/dns/zone/xfr`

Zone transfer is sufficiently large to be a child package.

Its future local book must define:

```text
AXFR
IXFR
NOTIFY
streaming transfer
TCP transfer
TLS transfer
DoQ transfer
authentication
resource limits
serial selection
atomic snapshot replacement
```

## 20.1 Standards

The child book must follow at least:

- RFC 1995;
- RFC 1996;
- RFC 5936;
- RFC 9103;
- relevant RFC 7766 connection guidance;
- RFC 9250 for transfer over DoQ.

## 20.2 Why not base zone

Keeping XFR separate prevents:

```text
zone master parser
```

from requiring:

```text
TCP
TLS
QUIC
DoQ
TSIG
```

and preserves use on build tools and constrained targets.

---

# Part XIV — Dynamic update boundary

# 21. `net/dns/zone/update`

Dynamic DNS UPDATE is also a separate active protocol.

Future package responsibilities include:

- UPDATE opcode message semantics;
- prerequisites;
- add/delete operations;
- atomic update application;
- SOA serial changes;
- secure update;
- authentication hooks.

Primary standards:

- RFC 2136;
- RFC 3007.

It must use the same `ZoneBuilder`/transactional zone mutation semantics where
appropriate rather than create a second zone model.

---

# Part XV — NOTIFY boundary

# 22. DNS NOTIFY

DNS NOTIFY belongs with secondary synchronization and is therefore assigned to
`zone/xfr`.

It is not an ordinary zone-file directive.

The base zone package may expose no network `Notify()` function.

---

# Part XVI — Authoritative server integration

# 23. Zone snapshots

An authoritative server should consume immutable `Zone` snapshots.

This supports:

- lock-minimized reads;
- atomic replacement after successful reload/transfer/update;
- rollback on failed zone load;
- clear ownership.

A new zone should become visible only after:

```text
parse/update
-> validate
-> build complete snapshot
-> commit replacement
```

A partially parsed zone must never become authoritative state.

## 23.1 Server ownership

The authoritative DNS server runtime is not specified by this book.

A later server package may use `zone.Zone` without changing the zone model.

---

# Part XVII — Security and robustness

# 24. Include security

`$INCLUDE` is the largest obvious master-file filesystem hazard.

The parser:

- does not open files on its own;
- uses caller-supplied `IncludeResolver`;
- bounds depth;
- detects cycles;
- bounds total bytes.

A service parsing untrusted zone files can simply leave includes disabled.

---

# 25. Resource limits

Master files can be attacker supplied.

The parser must bound at least:

- total source bytes;
- total included bytes;
- include count/depth;
- record count;
- token length;
- quoted-string length;
- parenthesis nesting/state;
- decoded RDATA size;
- diagnostics retained.

Malformed input must not create unbounded memory growth.

---

# 26. Unknown data

Unknown RR types are a normal DNS extensibility case.

They are not security errors by themselves.

RFC 3597 generic representation must round trip wire RDATA.

---

# 27. Paths are not DNS names

`$INCLUDE` targets are strings/external source IDs.

They must not be passed through `dns.Name`.

Likewise DNS names must not be treated as local filesystem paths.

The type distinction is intentional.

---

# Part XVIII — Repository implementation

# 28. Required source structure

Create:

```text
sec/stdlib/net/dns/zone/
├── std-net-dns-zone.md
├── error.sec
├── source.sec
├── serial.sec
├── zone.sec
├── builder.sec
├── validation.sec
├── directive.sec
├── master_token.sec
├── master_parser.sec
├── master_writer.sec
├── include.sec
├── xfr/
└── update/
```

Do not create implementation files inside `xfr/` or `update/` until their own
books are written.

The child directories may be scaffolded.

## 28.1 Existing files

If a newer repository revision already contains zone-related files:

- inspect them;
- preserve useful implementation;
- migrate them into this structure;
- update obsolete syntax/API;
- do not blindly overwrite working code.

---

# 29. Parent correction

Before implementation completion:

```text
std-net-dns.md
```

must gain the shared:

```sec
dns.Rrset
```

required by both zone and DNSSEC.

The zone package must not ship a duplicate RRset abstraction as a workaround.

---

# Part XIX — Tests

# 30. Serial tests

At minimum:

- ordinary increment;
- wraparound;
- comparison across wrap;
- equality;
- undefined half-space comparison returns `None`;
- invalid increment rejected.

Use RFC 1982 examples.

---

# 31. Core zone tests

At minimum:

- valid zone;
- one SOA at apex;
- missing SOA;
- multiple SOA;
- SOA below apex rejected;
- missing apex NS;
- duplicate RR rejected;
- mixed TTL in RRset rejected;
- CNAME coexistence rules;
- DNAME rules;
- delegation structure;
- wildcard names;
- unknown RRset retained.

---

# 32. Tokenizer tests

At minimum:

- comments;
- semicolon in quotes;
- escapes;
- quoted strings;
- whitespace;
- newline;
- parentheses;
- multiline records;
- directives;
- source positions;
- unterminated quotes;
- unterminated parentheses.

---

# 33. Parser tests

At minimum:

- `$ORIGIN`;
- `$TTL`;
- `$INCLUDE`;
- blank owner inheritance;
- relative owner names;
- relative RDATA names;
- explicit absolute names;
- TTL/class order alternatives;
- multiline SOA;
- TXT quoting/escapes;
- A/AAAA/MX/NS/SRV/CAA and other implemented known records;
- RFC 3597 unknown type syntax;
- `TYPE####`;
- `\#` RDATA;
- out-of-zone name;
- missing initial origin;
- resource-limit failures.

Use standards examples/test vectors where practical.

---

# 34. Include tests

At minimum:

- includes disabled;
- one include;
- include with alternate origin;
- nested include;
- maximum depth;
- cycle;
- resolver error;
- virtual/non-filesystem source IDs;
- source diagnostics preserve included source identity;
- parser never opens target string itself.

---

# 35. Writer tests

At minimum:

- valid reparsable output;
- deterministic stable output;
- `$ORIGIN`;
- `$TTL`;
- relative names;
- absolute fallback where necessary;
- unknown RR RFC 3597 output;
- quoted/escaped TXT;
- parse -> build -> write -> parse semantic equality;
- original comments not promised/preserved.

---

# 36. DNSSEC-zone tests

At minimum:

- signed records load as ordinary zone data;
- loading does not claim DNSSEC validation;
- NSEC/NSEC3 structures retained;
- DNSKEY/RRSIG preserved;
- future signer can operate on shared `dns.Rrset`;
- no dnssec->zone dependency is introduced.

---

# Part XX — AI/repository implementation instructions

# 37. Requirements

An AI/repository automation implementing this book must:

1. treat this book as the normative base zone/master-file specification;
2. add/reuse shared `dns.Rrset` in parent `net/dns`;
3. not create `zone.Rrset`;
4. implement RFC 1982 serial arithmetic rather than naive uint32 ordering;
5. implement RFC 1035 master-file syntax;
6. implement RFC 2308 `$TTL`;
7. implement RFC 3597 unknown RR presentation;
8. preserve typed parent DNS record data;
9. keep `dns.Name` absolute and resolve relative master-file names inside the
   zone parser;
10. implement caller-controlled `$INCLUDE`;
11. never open arbitrary include paths automatically;
12. bound parser/include resources;
13. reject non-standard directives by default;
14. produce immutable validated `Zone` snapshots;
15. use `ZoneBuilder` for mutation/build transactions;
16. keep zone loading separate from DNSSEC cryptographic validation;
17. reserve `zone/xfr` and `zone/update` as child packages;
18. do not implement XFR/UPDATE networking inside base zone files;
19. use RFC 8945 rather than obsolete RFC 2845 if TSIG work is later added;
20. add standards URLs to source files;
21. add required tests;
22. update `implementation-status-std-net-dns-zone.yaml`;
23. reconcile canonical status without replacing stronger/newer evidence.

---

# 38. Recommended implementation order

1. parent `dns.Rrset`;
2. errors/source locations;
3. RFC 1982 Serial;
4. Zone/ZoneBuilder;
5. zone structural validation;
6. tokenizer;
7. directives;
8. parser without includes;
9. RFC 3597 unknown RRs;
10. caller-controlled includes;
11. stable writer;
12. DNSSEC record compatibility tests;
13. later `std-net-dns-zone-xfr.md`;
14. later `std-net-dns-zone-update.md`;
15. later TSIG book if accepted.

---

# Part XXI — Open observations

# 39. Open items

- exact collection method naming (`RrsetsAt` versus `RrsetsAtToArray`);
- dedicated `SerialError` versus current `ZoneError`;
- canonical TTL presentation shorthand;
- support for non-standard `$GENERATE`;
- syntax-preserving AST/document editor;
- streaming parser for very large zones;
- streaming writer to common `io.Writer`;
- zone snapshot persistent storage;
- authoritative server package ownership;
- catalog zones;
- DNSSEC zone signing API;
- `zone/xfr` exact package files;
- `zone/update` exact package files;
- TSIG package path;
- SIG(0) ownership;
- atomic dynamic-update transaction API.

These observations do not permit incompatible public APIs.

---

# 40. Decisions established by revision 0.1

Revision 0.1 establishes:

- canonical package path `net/dns/zone`;
- the base package owns zone model and master-file syntax;
- zone transfer is reserved under `net/dns/zone/xfr`;
- dynamic update is reserved under `net/dns/zone/update`;
- TSIG is a general DNS concern and not embedded in base zone;
- `dns.Rrset` is shared from the parent DNS package;
- SOA serial uses RFC 1982 arithmetic;
- `Zone` is an immutable validated snapshot;
- `ZoneBuilder` owns mutable construction;
- normal zone build requires one apex SOA and apex NS data;
- master-file parsing follows RFC 1035;
- `$TTL` follows RFC 2308;
- unknown RR presentation follows RFC 3597;
- `dns.Name` remains absolute;
- relative master-file names are resolved during parsing;
- standardized `$ORIGIN`, `$TTL`, and `$INCLUDE` are supported;
- non-standard directives are rejected by default;
- `$INCLUDE` never implies unrestricted filesystem access;
- include resolution is caller-controlled and bounded;
- semantic zone round-trip is required, syntax/comment preservation is not;
- unknown RR data must survive parsing/writing;
- loading DNSSEC records does not constitute cryptographic validation;
- future full-zone DNSSEC signing is orchestrated by zone but uses DNSSEC
  primitives without creating a dnssec->zone dependency;
- base zone has no active network/TLS/QUIC dependency.

---

**Document revision:** 0.1  
**Last updated:** 2026-09-07
