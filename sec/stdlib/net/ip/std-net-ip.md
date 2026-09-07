# Sec Standard Library — IP Networking

- **Status:** Draft
- **Created:** 2026-09-07
- **Last updated:** 2026-09-07
- **Document revision:** 0.1
- **Sec language version:** 0.1
- **Canonical stdlib path:** `stdlib/net/ip/std-net-ip.md`
- **Repository path:** `sec/stdlib/net/ip/std-net-ip.md`
- **Parent specification:** `stdlib/net/std-net.md`
- **Implementation-status fragment:** `implementation-status-std-net-ip.yaml`
- **Repository baseline reviewed:** `afb0bed`

---

## 1. Purpose

`net/ip` is Sec's foundational IP networking package.

It owns the common public model for:

- IPv4 and IPv6 addresses;
- IP prefixes and networks;
- IP endpoints and ports;
- IP protocol-number values;
- IPv4 and IPv6 base wire headers;
- allocation-free IP packet parsing;
- TCP streams and listeners;
- UDP sockets and datagrams;
- ICMP protocol values and wire data;
- IP multicast membership;
- network-interface identity and enumeration;
- the common socket/network failure model used by IP transports.

The package is imported as:

```sec
import "net/ip"
```

and used through the final package component:

```sec
let address := try ip.Address.Parse("192.0.2.10")
let endpoint := new ip.Ipv4Endpoint(try ip.Ipv4Address.Parse("192.0.2.10"), ip.Port(443))
let stream := try ip.TcpStream.Connect(ip.Endpoint.V4(endpoint))
```

The full import path identifies the package. Source code uses `ip`, not
`net.ip`.

This document defines the package boundary and the intended public API.
Private target adapters and lowering details may differ between platforms but
must preserve the semantics defined here.

---

## 2. Design goals

`net/ip` follows the general Sec stdlib rules and additionally requires:

1. **Typed addresses and endpoints.** Public TCP/UDP APIs do not use raw
   strings, raw `uint32` addresses, or `"tcp"` / `"udp"` protocol strings.
2. **No DNS hidden inside IP primitives.** An IP endpoint contains an IP
   address, not a host name. Name resolution belongs to `net/dns` and
   higher-level protocol packages.
3. **No public generic magic-number socket API in the initial surface.**
   Ordinary users use `TcpStream`, `TcpListener`, and `UdpSocket`.
4. **IPv4 and IPv6 are first-class.** IPv6 must not be modeled as an optional
   afterthought or squeezed into IPv4-shaped APIs.
5. **IPv6 scope is preserved.** Scoped IPv6 endpoints can carry an interface
   index as required for link-local and other scoped communication.
6. **Owned network resources are move-only.** TCP streams, listeners, UDP
   sockets, and platform enumeration handles are `@noCopy`.
7. **No hidden allocation for packet parsing.** Raw IP packet parsing borrows
   caller-provided storage.
8. **Allocation is visible in API naming where practical.** APIs that
   intentionally materialize an owning dynamic collection use names such as
   `ToArray`.
9. **Cancellation and deadlines reuse Sec's execution/context model.**
   `net/ip` does not invent a second cancellation-token system.
10. **Platform differences are abstracted, not leaked as magic constants.**
    Public APIs use Sec enums, named types, unions, structs, and existing shared
    types.
11. **Capability absence is explicit.** A target without an IP stack/socket
    facility may still use pure address and packet types, but active socket
    operations must not pretend to exist successfully.
12. **No native socket descriptor is public.** Platform descriptors, handles,
    pointers, and errno values remain implementation details unless a later
    explicitly unsafe interoperability API is specified.

---

## 3. Standards and registries

### 3.1 Standards policy

The RFCs and registries below are the primary baseline for this package.

When an RFC is updated, corrected, or partially obsoleted by a later standards
document, the current applicable standards state takes precedence. Source files
must not deliberately preserve behavior known to have been superseded merely
because an older base RFC is listed here.

Relevant IETF errata must be considered during implementation.

### 3.2 Primary references

| Area | Standard / registry | URL |
|---|---|---|
| IPv4 | RFC 791 — Internet Protocol | https://www.rfc-editor.org/rfc/rfc791 |
| Internet host requirements | RFC 1122 | https://www.rfc-editor.org/rfc/rfc1122 |
| IPv4 CIDR | RFC 4632 | https://www.rfc-editor.org/rfc/rfc4632 |
| IPv6 | RFC 8200 — Internet Protocol, Version 6 | https://www.rfc-editor.org/rfc/rfc8200 |
| IPv6 addressing | RFC 4291 | https://www.rfc-editor.org/rfc/rfc4291 |
| IPv6 text form | RFC 5952 | https://www.rfc-editor.org/rfc/rfc5952 |
| IPv6 scoped addresses | RFC 4007 | https://www.rfc-editor.org/rfc/rfc4007 |
| TCP | RFC 9293 | https://www.rfc-editor.org/rfc/rfc9293 |
| UDP | RFC 768 | https://www.rfc-editor.org/rfc/rfc768 |
| ICMPv4 | RFC 792 | https://www.rfc-editor.org/rfc/rfc792 |
| ICMPv6 | RFC 4443 | https://www.rfc-editor.org/rfc/rfc4443 |
| IPv4 multicast | RFC 1112 | https://www.rfc-editor.org/rfc/rfc1112 |
| ECN | RFC 3168 | https://www.rfc-editor.org/rfc/rfc3168 |
| Service names and ports | RFC 6335 | https://www.rfc-editor.org/rfc/rfc6335 |
| Special-purpose addresses | RFC 6890 | https://www.rfc-editor.org/rfc/rfc6890 |
| IP protocol numbers | IANA Assigned Internet Protocol Numbers | https://www.iana.org/assignments/protocol-numbers |
| Service names and port numbers | IANA Service Name and Transport Protocol Port Number Registry | https://www.iana.org/assignments/service-names-port-numbers |
| IPv4 special-purpose registry | IANA IPv4 Special-Purpose Address Space | https://www.iana.org/assignments/iana-ipv4-special-registry |
| IPv6 special-purpose registry | IANA IPv6 Special-Purpose Address Space | https://www.iana.org/assignments/iana-ipv6-special-registry |
| ICMPv4 parameters | IANA ICMP Parameters | https://www.iana.org/assignments/icmp-parameters |
| ICMPv6 parameters | IANA ICMPv6 Parameters | https://www.iana.org/assignments/icmpv6-parameters |

Additional standards may be cited by individual source files when they implement
a narrower feature.

---

## 4. Source-file standards headers

Every source file in `stdlib/net/ip` must identify the standards or registries
that directly govern its protocol semantics.

A file header should use this general shape:

```sec
// Sec Standard Library: net/ip
// File: tcp.sec
//
// Standards:
//   RFC 9293 - Transmission Control Protocol
//   https://www.rfc-editor.org/rfc/rfc9293
//
// Related:
//   RFC 1122 - Requirements for Internet Hosts
//   https://www.rfc-editor.org/rfc/rfc1122

module ip
```

The comments are implementation traceability, not a replacement for this
normative package book.

A source file that is primarily a platform abstraction rather than a wire
protocol must identify the relevant package book and any platform standard it
directly implements.

---

## 5. Package and file manifest

The intended initial package layout is:

```text
stdlib/net/ip/
├── std-net-ip.md
├── error.sec
├── protocol.sec
├── address.sec
├── prefix.sec
├── endpoint.sec
├── ipv4.sec
├── ipv6.sec
├── packet.sec
├── tcp.sec
├── udp.sec
├── icmp.sec
├── multicast.sec
├── interface.sec
└── socket.<target>.sec        # private platform adapters as required
```

Examples of target-specific adapter names are:

```text
socket.linux.amd64.sec
socket.linux.arm64.sec
socket.linux.arm32.sec
socket.macos.amd64.sec
socket.macos.arm64.sec
socket.freebsd.amd64.sec
```

Those files are implementation files, not additional public packages.

All ordinary files in this directory use:

```sec
module ip
```

The user-facing package remains one package:

```sec
import "net/ip"
```

There are no initial user-facing packages named:

```text
net/ip/tcp
net/ip/udp
net/ip/ipv4
net/ip/ipv6
```

TCP, UDP, IPv4, and IPv6 are cohesive parts of the `ip` package.

If one of these areas later grows enough to justify a deeper package boundary,
that decision requires an explicit update to this book.

---

# Part I — Common model

## 6. `error.sec`

### 6.1 Responsibility

`error.sec` owns the common failure vocabulary used by active IP networking
operations and the parsing failure types used by address and raw-packet APIs.

It must not expose platform errno values as the public error contract.

### 6.2 Public error types

The initial canonical error surface is:

```sec
enum NetworkError error {
    PermissionDenied,
    AddressInUse,
    AddressNotAvailable,
    ConnectionRefused,
    ConnectionReset,
    ConnectionAborted,
    NotConnected,
    AlreadyConnected,
    NetworkUnreachable,
    HostUnreachable,
    TimedOut,
    BrokenPipe,
    MessageTooLarge,
    ResourceExhausted,
    OutOfMemory,
    InvalidArgument,
    WriteZero,
    Closed,
    Unsupported,
    Unknown,
}

enum AddressError error {
    Empty,
    InvalidFormat,
    InvalidIpv4,
    InvalidIpv6,
    InvalidPrefixLength,
}

enum PacketError error {
    BufferTooSmall,
    UnsupportedIpVersion,
    InvalidHeaderLength,
    InvalidTotalLength,
    InvalidPayloadLength,
    InvalidChecksum,
    MalformedPacket,
}
```

### 6.3 Error rules

- `NetworkError` describes a networking operation failure, not a DNS or
  application-protocol failure.
- `AddressError` describes textual or constrained IP-address/network parsing.
- `PacketError` describes malformed or insufficient raw packet data.
- Higher-level packages such as `net/http`, `net/mqtt`, or `net/dns` may expose
  their own precise errors and carry/wrap IP-layer failures according to their
  own books.
- Native errors must be translated at the platform adapter boundary.
- Unsupported target capabilities should be rejected at compile time when the
  absence is statically known. `NetworkError.Unsupported` is for a valid API on
  a generally capable target where a particular runtime operation is not
  available.
- Cancellation is not represented by inventing `NetworkError.Cancelled`.
  Blocking network waits participate in Sec's normal cancellation semantics.
- `TimedOut` remains distinct from task/thread cancellation. It represents a
  network operation that ends because the network/platform operation itself
  times out.
- A local explicit `Close()` followed by another operation produces `Closed`.
  It should not leak a platform-specific "bad file descriptor" spelling.

### 6.4 Current migration

The current root `net/ip.sec` declares:

```sec
enum NetError error {
    ConnectionRefused,
    Timeout,
    InvalidPacketStructure,
    NetworkUnreachable,
    BufferTooSmall,
    UnsupportedIpVersion,
}
```

That declaration is not retained as the final contract.

Migration:

- socket/network failures move into `NetworkError`;
- address parsing failures move into `AddressError`;
- raw packet failures move into `PacketError`;
- `Timeout` is normalized to `TimedOut`;
- `InvalidPacketStructure`, `BufferTooSmall`, and `UnsupportedIpVersion` move to
  `PacketError`;
- the old `NetError` declaration is removed after callers are migrated.

---

## 7. `protocol.sec`

### 7.1 Responsibility

`protocol.sec` owns common protocol-level scalar/enumeration values that are
shared by IPv4 and IPv6 packet representation.

### 7.2 `IpVersion`

```sec
enum IpVersion {
    V4,
    V6,
}
```

This is a closed semantic choice because the public package currently models
the two standardized IP versions supported by Sec.

It is not the raw 4-bit wire field.

### 7.3 `IpProtocol`

The IPv4 `Protocol` field and IPv6 `Next Header` field use the IANA Assigned
Internet Protocol Numbers registry.

The declaration remains an open bit-backed enum:

```sec
enum IpProtocol bit[8] {
    HOPOPT = 0,
    ICMP = 1,
    IGMP = 2,
    IPv4 = 4,
    TCP = 6,
    UDP = 17,
    IPv6 = 41,
    IPv6_ROUTE = 43,
    IPv6_FRAG = 44,
    GRE = 47,
    ESP = 50,
    AH = 51,
    IPv6_ICMP = 58,
    IPv6_NONXT = 59,
    IPv6_OPTS = 60,
    SCTP = 132,
    // ...all currently assigned IANA keywords represented where practical...
}
```

Normative source registry:

https://www.iana.org/assignments/protocol-numbers

Rules:

- the complete implementation file should track current assigned IANA keyword
  values where a stable readable Sec identifier can be provided;
- normalized identifiers may use underscores where the registry keyword cannot
  be a Sec identifier;
- numeric values must exactly match IANA;
- deprecated assigned values may remain named because packet decoders must
  preserve real wire values;
- unassigned, experimental, reserved, and future values remain representable
  because `bit[8]` enums are open across their storage width;
- the implementation must not convert unknown protocol numbers to
  `NetworkError` merely because Sec does not have a symbolic member for them.

`IpProtocol` is **not** a duplicate of the existing shared
`TransportProtocol`.

`TransportProtocol` must be reused where an API asks the narrower semantic
question represented by that existing shared type. `IpProtocol` exists because
the IP wire field is the IANA 8-bit protocol/next-header namespace and contains
far more values than TCP/UDP-style transport selection.

### 7.4 `IpEcn`

```sec
enum IpEcn bit[2] {
    NotEct = 0b00,
    Ect1 = 0b01,
    Ect0 = 0b10,
    Ce = 0b11,
}
```

Primary reference:

https://www.rfc-editor.org/rfc/rfc3168

The exact bit representation is part of the wire contract.

---

# Part II — Addresses, prefixes, and endpoints

## 8. `ipv4.sec`

### 8.1 Responsibility

`ipv4.sec` owns IPv4-specific value representation and the fixed IPv4 base wire
header.

Primary references:

- RFC 791: https://www.rfc-editor.org/rfc/rfc791
- RFC 1122: https://www.rfc-editor.org/rfc/rfc1122
- RFC 6890: https://www.rfc-editor.org/rfc/rfc6890
- IANA IPv4 special-purpose registry:
  https://www.iana.org/assignments/iana-ipv4-special-registry

### 8.2 `Ipv4Address`

The current fixed-width representation is retained:

```sec
type Ipv4Address register[32] msb-first big-endian {
    A: bit[8],
    B: bit[8],
    C: bit[8],
    D: bit[8],
}
```

This is a nominal 32-bit IPv4 address value. The use of `register[32]` describes
a fixed-width bit layout; it does not imply MMIO or hardware-register
ownership.

Canonical associated behavior:

```sec
impl Ipv4Address {
    init(a: byte, b: byte, c: byte, d: byte)

    static fn Parse(text: string) Result[Ipv4Address, AddressError]

    fn ToString() string
    fn Bytes() byte[4]

    property IsUnspecified: bool { get { ... } }
    property IsLoopback: bool { get { ... } }
    property IsMulticast: bool { get { ... } }
    property IsLinkLocal: bool { get { ... } }
    property IsPrivate: bool { get { ... } }

    let Unspecified: Ipv4Address := ...
    let Loopback: Ipv4Address := ...
}
```

In the declaration sketch above and later sketches, `{ ... }` replaces only the
implementation body. The member name, member kind, parameter list, result type,
static/instance category, and property type are normative.

Semantics:

- `Parse` accepts standard dotted-decimal IPv4 text.
- Each component must be decimal `0..255`.
- Parsing must reject trailing garbage, missing components, extra components,
  signed components, and values outside the octet range.
- `ToString()` returns canonical dotted-decimal text without unnecessary
  leading zeroes.
- `Bytes()` returns network byte order.
- `IsPrivate` refers to the IPv4 private-use address ranges, not to an
  authorization or security decision.
- `IsLinkLocal` covers the IPv4 link-local address block.
- `IsMulticast` uses the standardized IPv4 multicast range.
- No method named `IsGlobal` is required in revision 0.1 because special-purpose
  global-reachability classification is registry-sensitive and evolves.

### 8.3 `Ipv4Flags`

```sec
type Ipv4Flags register[3] msb-first {
    Reserved: bit,
    DontFragment: bit,
    MoreFragments: bit,
}
```

The reserved bit must be validated according to the applicable IPv4 standard
when parsing a packet in a mode that performs protocol validation.

### 8.4 `Ipv4WireHeader`

```sec
type Ipv4WireHeader register[160] msb-first big-endian {
    Version: bit[4],
    Ihl: bit[4],
    Dscp: bit[6],
    Ecn: IpEcn,
    TotalLength: bit[16],
    Identification: bit[16],
    Flags: Ipv4Flags,
    FragmentOffset: bit[13],
    TimeToLive: bit[8],
    Protocol: IpProtocol,
    HeaderChecksum: bit[16],
    SourceIp: Ipv4Address,
    DestinationIp: Ipv4Address,
}
```

This is the fixed 20-byte IPv4 base header only.

IPv4 options are not embedded into this fixed-width type. They are represented
by the borrowed `Options` slice in `Ipv4Packet`.

`Ihl` determines the actual complete header size. A parser must reject values
smaller than the minimum legal IPv4 header length and lengths inconsistent with
the supplied buffer or total length.

---

## 9. `ipv6.sec`

### 9.1 Responsibility

`ipv6.sec` owns IPv6-specific address representation and the fixed IPv6 base
header.

Primary references:

- RFC 8200: https://www.rfc-editor.org/rfc/rfc8200
- RFC 4291: https://www.rfc-editor.org/rfc/rfc4291
- RFC 5952: https://www.rfc-editor.org/rfc/rfc5952
- RFC 4007: https://www.rfc-editor.org/rfc/rfc4007
- IANA IPv6 special-purpose registry:
  https://www.iana.org/assignments/iana-ipv6-special-registry

### 9.2 `Ipv6Address`

The fixed-width representation is:

```sec
type Ipv6Address register[128] msb-first big-endian {
    Segment0: bit[16],
    Segment1: bit[16],
    Segment2: bit[16],
    Segment3: bit[16],
    Segment4: bit[16],
    Segment5: bit[16],
    Segment6: bit[16],
    Segment7: bit[16],
}
```

Canonical associated behavior:

```sec
impl Ipv6Address {
    init(
        segment0: uint16,
        segment1: uint16,
        segment2: uint16,
        segment3: uint16,
        segment4: uint16,
        segment5: uint16,
        segment6: uint16,
        segment7: uint16
    )

    static fn Parse(text: string) Result[Ipv6Address, AddressError]

    fn ToString() string
    fn Bytes() byte[16]

    property IsUnspecified: bool { get { ... } }
    property IsLoopback: bool { get { ... } }
    property IsMulticast: bool { get { ... } }
    property IsLinkLocal: bool { get { ... } }
    property IsUniqueLocal: bool { get { ... } }

    let Unspecified: Ipv6Address := ...
    let Loopback: Ipv6Address := ...
}
```

Parsing rules follow RFC 4291-compatible input forms.

Formatting rules follow RFC 5952 canonical recommendations, including:

- lowercase hexadecimal output;
- suppression of leading zeroes within 16-bit fields;
- canonical zero compression;
- no ambiguous or non-canonical alternative chosen merely because input used
  that spelling.

The address value does **not** contain a textual zone identifier. Scope/zone is
endpoint/interface context and is modeled separately.

### 9.3 `Ipv6WireHeader`

```sec
type Ipv6WireHeader register[320] msb-first big-endian {
    Version: bit[4],
    Dscp: bit[6],
    Ecn: IpEcn,
    FlowLabel: bit[20],
    PayloadLength: bit[16],
    NextHeader: IpProtocol,
    HopLimit: bit[8],
    SourceIp: Ipv6Address,
    DestinationIp: Ipv6Address,
}
```

This is exactly the fixed 40-byte IPv6 base header.

IPv6 extension headers are not flattened into this register. `NextHeader`
identifies the next item in the header chain according to RFC 8200.

Revision 0.1 packet parsing exposes the bytes following the base header without
pretending that all extension-header chains have already been semantically
decoded.

---

## 10. `address.sec`

### 10.1 Responsibility

`address.sec` owns the family-neutral IP address union and cross-family
operations.

### 10.2 `Address`

```sec
type Address union {
    V4(Ipv4Address),
    V6(Ipv6Address),
}
```

Associated behavior:

```sec
impl Address {
    static fn Parse(text: string) Result[Address, AddressError]

    fn ToString() string

    property Version: IpVersion { get { ... } }
    property IsUnspecified: bool { get { ... } }
    property IsLoopback: bool { get { ... } }
    property IsMulticast: bool { get { ... } }
    property IsLinkLocal: bool { get { ... } }
}
```

Rules:

- `Address.Parse` attempts to parse an IP literal only.
- It does not perform DNS resolution.
- A host name such as `"example.com"` is not an `Address`.
- `ToString()` delegates to the canonical family-specific representation.
- Family-specific classification remains available through `Ipv4Address` or
  `Ipv6Address` when the concepts are not semantically identical.

No implicit IPv4-to-IPv6 or IPv6-to-IPv4 conversion is performed.

IPv4-mapped IPv6 addresses remain IPv6 address values. Any later helper that
recognizes or converts mapped addresses must be explicit.

---

## 11. `prefix.sec`

### 11.1 Responsibility

`prefix.sec` owns typed prefix lengths and canonical IPv4/IPv6 network-prefix
values.

Primary references:

- RFC 4632: https://www.rfc-editor.org/rfc/rfc4632
- RFC 4291: https://www.rfc-editor.org/rfc/rfc4291

### 11.2 Prefix-length types

```sec
type Ipv4PrefixLength uint8 range 0..32
type Ipv6PrefixLength uint8 range 0..128
```

These are distinct nominal types.

A runtime integer requires ordinary Sec checked conversion:

```sec
let prefix := try Ipv4PrefixLength(value)
```

A compile-time literal may be accepted when its value is provably within the
contract according to the named-type rules.

### 11.3 `Ipv4Network`

```sec
type Ipv4Network struct {
    _address: Ipv4Address,
    _prefixLength: Ipv4PrefixLength,
}
```

Associated behavior:

```sec
impl Ipv4Network {
    init(address: Ipv4Address, prefixLength: Ipv4PrefixLength)

    static fn Parse(text: string) Result[Ipv4Network, AddressError]

    property Address: Ipv4Address { get { ... } }
    property PrefixLength: Ipv4PrefixLength { get { ... } }

    fn Contains(address: Ipv4Address) bool
    fn ToString() string
}
```

The stored `Address` is always the canonical network address with host bits
cleared.

Construction from a host address plus prefix length is infallible and
normalizes the host bits rather than rejecting them.

`Parse("192.0.2.27/24")` therefore yields the canonical network
`192.0.2.0/24`.

### 11.4 `Ipv6Network`

```sec
type Ipv6Network struct {
    _address: Ipv6Address,
    _prefixLength: Ipv6PrefixLength,
}
```

Associated behavior:

```sec
impl Ipv6Network {
    init(address: Ipv6Address, prefixLength: Ipv6PrefixLength)

    static fn Parse(text: string) Result[Ipv6Network, AddressError]

    property Address: Ipv6Address { get { ... } }
    property PrefixLength: Ipv6PrefixLength { get { ... } }

    fn Contains(address: Ipv6Address) bool
    fn ToString() string
}
```

The same canonical-host-bit clearing rule applies.

### 11.5 `Network`

```sec
type Network union {
    V4(Ipv4Network),
    V6(Ipv6Network),
}
```

Associated behavior:

```sec
impl Network {
    static fn Parse(text: string) Result[Network, AddressError]

    fn Contains(address: Address) bool
    fn ToString() string

    property Version: IpVersion { get { ... } }
}
```

`Contains` returns `false` when the address family does not match the network
family.

---

## 12. `endpoint.sec`

### 12.1 Responsibility

`endpoint.sec` owns transport port identity and concrete IP socket endpoints.

Port-number registry reference:

- RFC 6335: https://www.rfc-editor.org/rfc/rfc6335
- IANA registry:
  https://www.iana.org/assignments/service-names-port-numbers

IPv6 scope reference:

- RFC 4007: https://www.rfc-editor.org/rfc/rfc4007

### 12.2 `Port`

```sec
type Port uint16
```

`Port` is nominally distinct from an arbitrary `uint16`.

The value `0` is representable.

This is intentional: port zero has legitimate API-level meaning in contexts
such as requesting an automatically selected local port during binding. The
meaning of zero is determined by the operation using the port; the `Port` type
must not globally reject it.

IANA service ranges are registry classifications, not type-level range
restrictions.

### 12.3 Interface index dependency

`InterfaceIndex` is defined in `interface.sec` and is available throughout the
same `ip` module.

An IPv6 scoped endpoint uses an optional `InterfaceIndex`, not an untyped string
zone.

### 12.4 Family-specific endpoints

```sec
type Ipv4Endpoint struct {
    Address: Ipv4Address,
    Port: Port,
}

type Ipv6Endpoint struct {
    Address: Ipv6Address,
    Port: Port,
    Zone: Option[InterfaceIndex],
}
```

Constructors:

```sec
impl Ipv4Endpoint {
    init(address: Ipv4Address, port: Port)
    fn ToString() string
}

impl Ipv6Endpoint {
    init(
        address: Ipv6Address,
        port: Port,
        zone: Option[InterfaceIndex]
    )

    fn ToString() string
}
```

The IPv6 `Zone` is retained because an address+port pair alone is insufficient
to identify some scoped destinations.

### 12.5 Family-neutral endpoint

```sec
type Endpoint union {
    V4(Ipv4Endpoint),
    V6(Ipv6Endpoint),
}
```

Associated behavior:

```sec
impl Endpoint {
    property Version: IpVersion { get { ... } }
    property Port: Port { get { ... } }
    property Address: Address { get { ... } }

    fn ToString() string
}
```

Endpoint text parsing is intentionally **not** part of revision 0.1.

The combination of IPv6 bracket syntax, zone syntax, URI authority syntax, and
platform interface names must not be silently conflated into one parser before
that contract is explicitly designed.

Higher-level packages may parse their own standardized endpoint/authority
syntax and then construct typed IP endpoints.

---

# Part III — Raw IP packets

## 13. `packet.sec`

### 13.1 Responsibility

`packet.sec` owns allocation-free structural parsing of raw IPv4 and IPv6
packets.

Primary references:

- RFC 791: https://www.rfc-editor.org/rfc/rfc791
- RFC 8200: https://www.rfc-editor.org/rfc/rfc8200

### 13.2 Borrowed packet types

```sec
type Ipv4Packet struct {
    Header: Ipv4WireHeader,
    Options: ref byte[],
    Payload: ref byte[],
}

type Ipv6Packet struct {
    Header: Ipv6WireHeader,
    Payload: ref byte[],
}

type IpPacket union {
    V4(Ipv4Packet),
    V6(Ipv6Packet),
}
```

The borrowed slices refer to the caller's original packet buffer.

No payload copy or owning allocation is required merely to parse a packet.

The compiler's ordinary granting-borrow/lifetime analysis must ensure that the
returned packet cannot outlive the borrowed source buffer.

### 13.3 Parsing functions

```sec
fn ParseIpv4Packet(buffer: ref byte[]) Result[Ipv4Packet, PacketError]
fn ParseIpv6Packet(buffer: ref byte[]) Result[Ipv6Packet, PacketError]
fn ParseIpPacket(buffer: ref byte[]) Result[IpPacket, PacketError]
```

Required IPv4 structural checks include at least:

- enough bytes exist to inspect the base header;
- version is 4;
- IHL is legal;
- complete header bytes are present;
- total length is not smaller than the header;
- total length does not claim unavailable bytes;
- option slice boundaries are valid;
- payload slice boundaries are valid.

Required IPv6 structural checks include at least:

- at least the fixed 40-byte base header is present;
- version is 6;
- payload length is consistent with available bytes where the ordinary
  non-jumbogram length field applies;
- payload boundaries are valid.

Revision 0.1 does not claim to fully decode every IPv6 extension-header chain.

### 13.4 Checksum policy

IPv4 header-checksum validation must be available.

The initial implementation may expose it as a separate explicit operation if
doing so avoids forcing checksum work on callers that only need structural
inspection.

The exact final helper name is an observation item until the checksum/packet
encoding surface is implemented.

TCP/UDP checksums are not recomputed by the ordinary socket API; the selected
network stack normally owns them.

### 13.5 Packet encoding

Packet/header encoding is in scope for `net/ip` but is **planned**, not locked
as a public revision-0.1 method surface in this draft.

The current compiler lacks a completed general endian-aware
register-to-byte/register-from-byte lowering path used by the existing
prototype. Implementations must not invent an unrelated runtime-only binary API
to bypass Sec's fixed-layout semantics.

---

# Part IV — TCP

## 14. `tcp.sec`

### 14.1 Responsibility

`tcp.sec` owns Sec's typed TCP stream and listener abstractions.

Primary reference:

- RFC 9293: https://www.rfc-editor.org/rfc/rfc9293

Related host requirements:

- RFC 1122: https://www.rfc-editor.org/rfc/rfc1122

### 14.2 Public resource types

```sec
@noCopy
type TcpStream struct {
    // Module-private target representation.
    // Private layout is not part of the public API contract.
}

@noCopy
type TcpListener struct {
    // Module-private target representation.
    // Private layout is not part of the public API contract.
}
```

The comments above are normative about visibility, not literal required empty
storage layouts. Concrete target files provide the private resource
representation.

Both types:

- own a live networking resource while open;
- are move-only;
- close their resource during deterministic destruction;
- do not expose a native descriptor as a public field;
- may provide explicit `Close()` so close failure can be observed.

### 14.3 `TcpStream`

Canonical public member surface:

```sec
impl TcpStream {
    static fn Connect(endpoint: Endpoint) Result[TcpStream, NetworkError]

    property LocalEndpoint: Endpoint { get { ... } }
    property RemoteEndpoint: Endpoint { get { ... } }

    fn Read(buffer: ref mut byte[]) Result[uint, NetworkError]
    fn ReadExact(buffer: ref mut byte[]) Result[uint, NetworkError]

    fn Write(data: ref byte[]) Result[uint, NetworkError]
    fn WriteAll(data: ref byte[]) Result[uint, NetworkError]

    fn Shutdown(direction: TcpShutdown) Result[void, NetworkError]

    fn SetNoDelay(enabled: bool) Result[void, NetworkError]

    fn Close() Result[void, NetworkError]
}
```

### 14.4 `TcpShutdown`

```sec
enum TcpShutdown {
    Read,
    Write,
    Both,
}
```

This is a closed semantic operation and must not be represented by raw integers
or strings.

### 14.5 Connection semantics

`TcpStream.Connect`:

- accepts one concrete typed `Endpoint`;
- does not perform DNS resolution;
- creates and connects an appropriate IPv4 or IPv6 transport;
- returns the owning `TcpStream` only after connection establishment commits;
- does not expose a half-created stream to the caller on failure;
- may be a cancellation point according to Sec's execution rules;
- returns network failure through `NetworkError` when the operation fails
  normally.

A higher-level "connect to host name" facility belongs in DNS or a later
composition API, not in `TcpStream.Connect`.

### 14.6 Read semantics

`Read` follows byte-stream semantics:

- TCP preserves byte order but not application message boundaries;
- a successful read may return fewer bytes than the destination buffer length;
- for a non-empty buffer, `Ok(0)` represents orderly peer EOF after all
  previously received bytes have been consumed;
- `Read` does not fabricate packets or messages.

`ReadExact` loops until the requested buffer is full or the stream can no
longer satisfy the request.

The precise error returned when orderly EOF occurs before `ReadExact` completes
remains an observation item. It should align with the common Sec I/O model
rather than inventing unnecessary TCP-only vocabulary.

### 14.7 Write semantics

`Write` may report a partial successful write.

`WriteAll` continues until:

- all bytes are written; or
- a normal network error occurs.

A successful zero-byte write while data remains is mapped to
`NetworkError.WriteZero` to prevent an infinite retry loop.

### 14.8 Shutdown and close

`Shutdown` performs the TCP half-close/full-shutdown operation selected by
`TcpShutdown`.

It does not destroy ownership of the `TcpStream`.

`Close()`:

- is explicit and fallible;
- is idempotent;
- marks the resource locally closed after successful local close commit;
- leaves later ordinary operations returning `NetworkError.Closed`;
- does not transfer ownership.

Deterministic destruction must still release an unclosed stream.

Destruction cannot surface an ordinary `Result`, so callers that need to
observe close errors use `Close()` explicitly.

### 14.9 `TcpListener`

Canonical public member surface:

```sec
impl TcpListener {
    static fn Bind(endpoint: Endpoint) Result[TcpListener, NetworkError]

    property LocalEndpoint: Endpoint { get { ... } }

    fn Accept() Result[TcpStream, NetworkError]

    fn Close() Result[void, NetworkError]
}
```

`Bind` performs the platform operations required to create a listening TCP
endpoint.

Binding to `Port(0)` requests a platform-selected local port. The resulting
actual endpoint is available through `LocalEndpoint`.

`Accept`:

- is a blocking/cancellation-aware wait according to Sec's execution model;
- returns a new owning `TcpStream`;
- does not transfer ownership of the listener;
- records the accepted stream's local and remote endpoint so those properties
  remain available without exposing native descriptors.

### 14.10 Options deliberately not generalized

Revision 0.1 does not expose a generic API such as:

```sec
socket.SetOption(level, option, value)
```

nor:

```sec
socket.SetOption("TCP_NODELAY", "1")
```

Protocol-specific options receive typed methods or typed option objects as they
are accepted into the public API.

`SetNoDelay(bool)` is the initial example.

Listen backlog, keepalive tuning, linger, reusable-address behavior, and
split-read/write halves remain observation items for a later revision.

---

# Part V — UDP

## 15. `udp.sec`

### 15.1 Responsibility

`udp.sec` owns Sec's typed UDP datagram socket abstraction.

Primary reference:

- RFC 768: https://www.rfc-editor.org/rfc/rfc768

### 15.2 Resource type

```sec
@noCopy
type UdpSocket struct {
    // Module-private target representation.
    // Private layout is not part of the public API contract.
}
```

### 15.3 Receive result

Datagram boundaries must not be hidden.

```sec
type UdpReceiveInfo struct {
    Count: uint,
    Source: Endpoint,
    Truncated: bool,
}
```

`Truncated` is necessary because a datagram larger than the caller's buffer may
be truncated by the underlying socket operation. Sec must not silently make
that indistinguishable from receiving a naturally shorter complete datagram.

### 15.4 Public member surface

```sec
impl UdpSocket {
    static fn Bind(endpoint: Endpoint) Result[UdpSocket, NetworkError]

    property LocalEndpoint: Endpoint { get { ... } }

    fn SendTo(
        data: ref byte[],
        destination: Endpoint
    ) Result[uint, NetworkError]

    fn ReceiveFrom(
        buffer: ref mut byte[]
    ) Result[UdpReceiveInfo, NetworkError]

    fn SetBroadcast(enabled: bool) Result[void, NetworkError]

    fn Close() Result[void, NetworkError]
}
```

### 15.5 Datagram semantics

`SendTo` sends one UDP datagram.

A successful return must not imply TCP-like partial-message continuation.
The implementation either commits the datagram according to platform semantics
or reports a failure. A payload too large for the selected transport is
`NetworkError.MessageTooLarge`.

`ReceiveFrom` receives at most one datagram and reports:

- number of bytes materialized in the caller buffer;
- source endpoint;
- whether the original datagram was larger than the caller buffer.

Zero-length UDP datagrams are valid. Therefore `Count == 0` is **not** UDP EOF.

UDP has no stream EOF state.

### 15.6 Connected UDP

Connected UDP is useful, especially for high-performance clients and protocols
such as QUIC, but its public state model is not locked in revision 0.1.

The final design should avoid a confusing socket that sometimes requires a
destination and sometimes rejects one based on hidden mutable state.

Possible future designs include a distinct `ConnectedUdpSocket` resource or an
explicit typed state transition.

No design is locked by this draft.

---

# Part VI — ICMP and multicast

## 16. `icmp.sec`

### 16.1 Responsibility

`icmp.sec` owns ICMP protocol values and foundational wire-format support.

Primary references:

- ICMPv4 — RFC 792: https://www.rfc-editor.org/rfc/rfc792
- ICMPv6 — RFC 4443: https://www.rfc-editor.org/rfc/rfc4443
- IANA ICMP parameters:
  https://www.iana.org/assignments/icmp-parameters
- IANA ICMPv6 parameters:
  https://www.iana.org/assignments/icmpv6-parameters

### 16.2 Open type registries

ICMP type/code registries can evolve. Protocol wire values therefore must not
be represented as closed string lists.

The intended model is open fixed-width enums:

```sec
enum Icmpv4Type bit[8] {
    EchoReply = 0,
    DestinationUnreachable = 3,
    Redirect = 5,
    EchoRequest = 8,
    TimeExceeded = 11,
    ParameterProblem = 12,
    // ...current IANA assignments...
}

enum Icmpv6Type bit[8] {
    DestinationUnreachable = 1,
    PacketTooBig = 2,
    TimeExceeded = 3,
    ParameterProblem = 4,
    EchoRequest = 128,
    EchoReply = 129,
    // ...current IANA assignments...
}
```

The implementation file must track the relevant IANA registries while
preserving unknown/future 8-bit values.

Code values whose meaning depends on the active ICMP type should not be
collapsed into one misleading closed `IcmpCode` enum.

### 16.3 Raw ICMP I/O

A generic public raw `IcmpSocket` is **not locked** in revision 0.1.

Reasons include:

- raw socket privileges differ by platform;
- some platforms expose restricted datagram-style ICMP facilities;
- bare-metal stacks may provide different capabilities;
- safe packet construction and checksum semantics must be specified first.

The foundational type/codec support belongs here even before a portable send
API is accepted.

A later `Ping` convenience API, if added, must also define timing,
cancellation, identifier/sequence handling, and privilege behavior rather than
being a thin undocumented raw-socket wrapper.

---

## 17. `multicast.sec`

### 17.1 Responsibility

`multicast.sec` owns IP multicast membership operations that extend the UDP
socket API.

Primary references:

- IPv4 host extensions for multicasting — RFC 1112:
  https://www.rfc-editor.org/rfc/rfc1112
- IPv6 multicast addressing — RFC 4291:
  https://www.rfc-editor.org/rfc/rfc4291

### 17.2 UDP multicast extensions

The intended public surface is contributed through the same `ip` module:

```sec
impl extends UdpSocket {
    fn JoinMulticast(
        group: Address,
        interface: Option[InterfaceIndex]
    ) Result[void, NetworkError]

    fn LeaveMulticast(
        group: Address,
        interface: Option[InterfaceIndex]
    ) Result[void, NetworkError]

    fn SetMulticastLoopback(enabled: bool) Result[void, NetworkError]

    fn SetMulticastHopLimit(hopLimit: uint8) Result[void, NetworkError]
}
```

Rules:

- `group` must be a multicast address of the socket-compatible family;
- a non-multicast group returns `NetworkError.InvalidArgument`;
- `None` requests the platform/default interface policy where that operation is
  defined;
- an explicit `InterfaceIndex` selects a concrete interface;
- IPv4 and IPv6 platform-specific membership structures are implementation
  details;
- one source-level method family is preferred where its semantics are genuinely
  common.

The library must not expose platform option integers such as `IP_ADD_MEMBERSHIP`
or `IPV6_JOIN_GROUP` in the ordinary safe API.

---

# Part VII — Interfaces and platform boundary

## 18. `interface.sec`

### 18.1 Responsibility

`interface.sec` owns stable interface identity and the minimum portable
network-interface query model required by IP networking and IPv6 scopes.

Relevant scoped-address reference:

- RFC 4007: https://www.rfc-editor.org/rfc/rfc4007

Interface discovery itself is platform-facing and therefore also follows the
selected target/platform contract.

### 18.2 `InterfaceIndex`

```sec
type InterfaceIndex uint32
```

`InterfaceIndex` is nominally distinct from an arbitrary integer.

Value zero is representable because some socket/platform APIs use zero as
"unspecified/default interface". A concrete enumerated interface returned by
Sec should normally have a nonzero index where the target platform has the
standard index concept.

### 18.3 Interface address

```sec
type InterfaceAddress union {
    V4 {
        Address: Ipv4Address,
        PrefixLength: Ipv4PrefixLength,
    }

    V6 {
        Address: Ipv6Address,
        PrefixLength: Ipv6PrefixLength,
    }
}
```

An interface address is a host/interface address plus prefix length. Unlike
`Ipv4Network` and `Ipv6Network`, it does **not** clear host bits.

### 18.4 `NetworkInterface`

Initial public shape:

```sec
type NetworkInterface struct {
    Index: InterfaceIndex,
    Name: string,
    Mtu: uint,
}
```

Associated behavior:

```sec
impl NetworkInterface {
    fn AddressesToArray() Result[InterfaceAddress[], NetworkError]
}
```

`AddressesToArray` explicitly materializes an owning array and may allocate.

A later zero-allocation address iterator may be added if ordinary platform
adapters can support it coherently.

### 18.5 Interface enumeration

```sec
@noCopy
type InterfaceIterator struct {
    // Module-private target representation.
}

impl InterfaceIterator {
    fn Next() Result[Option[NetworkInterface], NetworkError]
    fn Close() Result[void, NetworkError]
}

fn Interfaces() Result[InterfaceIterator, NetworkError]

fn InterfaceByIndex(
    index: InterfaceIndex
) Result[Option[NetworkInterface], NetworkError]

fn InterfaceByName(
    name: string
) Result[Option[NetworkInterface], NetworkError]
```

`None` from `InterfaceByIndex` or `InterfaceByName` means no matching interface
was found. It is not itself an operating-system error.

`InterfaceIterator.Next()` returns `None` after normal exhaustion.

`InterfaceIterator` is an owned resource where the platform enumeration
mechanism requires one. `Close()` is idempotent.

### 18.6 Deferred interface metadata

Revision 0.1 does not yet lock:

- MAC/hardware-address representation;
- interface flags;
- operational state;
- link speed;
- route table access;
- gateway/default-route queries;
- DNS server configuration;
- per-interface statistics.

Those areas should be designed according to responsibility and may require
separate packages rather than expanding `NetworkInterface` without bound.

---

## 19. `socket.<target>.sec`

### 19.1 Responsibility

Target-specific socket files implement the private adapter between Sec's
semantic networking API and the selected platform/stack.

They do not create a user-facing package named `socket`.

Typical responsibilities include:

- creating native IPv4/IPv6 sockets;
- binding;
- listening;
- accepting;
- connecting;
- reading/writing;
- UDP send/receive;
- socket shutdown;
- explicit close;
- endpoint conversion;
- interface lookup;
- typed option translation;
- native error translation to `NetworkError`;
- cancellation-aware wait integration where supported.

### 19.2 Native constants

Native address-family numbers, socket kinds, protocol numbers, `setsockopt`
constants, errno values, Windows socket error values, and equivalent platform
tokens may exist inside a platform adapter.

They are not the public Sec API.

Where a closed public choice is needed, the public layer uses a Sec enum or
other strong type.

### 19.3 No libc requirement

The semantic API does not require that every target route networking through a
C library.

Possible adapters include:

- direct hosted OS syscalls;
- libc/WinSock where deliberately selected;
- RTOS networking stacks;
- embedded IP stacks;
- target-defined native networking facilities.

The selected target must preserve the same public Sec semantics or reject an
unsupported operation.

---

# Part VIII — Resource, cancellation, and concurrency semantics

## 20. Ownership and lifecycle

`TcpStream`, `TcpListener`, `UdpSocket`, and `InterfaceIterator` are owning,
`@noCopy` resources.

They follow Sec's ordinary ownership rules:

- assignment/calls may move ownership where the selected operation consumes;
- borrowing does not clone the native resource;
- returning an owned resource transfers it through the normal return boundary;
- deterministic destruction releases still-owned live resources;
- native descriptor identity is not an excuse to make the Sec value freely
  copyable.

Fresh constructors/factories do not require an artificial move marker merely
because they produce a new resource.

### 20.1 Explicit close

Public `Close()` exists because:

- callers may want deterministic early release;
- callers may need to observe a close failure;
- destruction cannot return an ordinary `Result`.

`Close()` is idempotent.

The package must not require every successful program to call `Close()` solely
to prevent leaks; deterministic destruction remains the ownership backstop.

---

## 21. Cancellation and blocking operations

Potentially blocking operations include at least:

- `TcpStream.Connect`;
- `TcpStream.Read`;
- `TcpStream.ReadExact`;
- `TcpStream.Write` when the platform must wait;
- `TcpStream.WriteAll`;
- `TcpListener.Accept`;
- `UdpSocket.ReceiveFrom`;
- platform interface enumeration where it can wait.

These operations participate in the canonical Sec cancellation model.

The networking package must not introduce:

```text
CancellationToken
NetworkCancellationToken
SocketCancelToken
```

unless a future general Sec rule explicitly establishes such a shared core
abstraction.

Cancellation must respect operation commit boundaries.

Examples:

- cancellation before an accepted connection commits must not fabricate or lose
  ownership of a `TcpStream`;
- cancellation racing a UDP receive must resolve to exactly one committed
  outcome;
- cancellation must not report a datagram as received if the receive did not
  commit;
- cancellation after an operation has committed must preserve the committed
  result/resource.

The exact compiler effect and current-context mechanics are owned by the
canonical cancellation/context rulebooks, not redefined here.

---

## 22. Deadlines and timeouts

`net/ip` does not create an independent deadline framework.

Where Sec's current execution `Context` supplies cancellation/deadline
semantics, network waits integrate with that model.

A later network-specific timeout option may exist only when it represents a
genuine socket/transport operation setting distinct from the general execution
deadline.

`NetworkError.TimedOut` remains available for ordinary platform/network
timeout outcomes.

---

## 23. Concurrency

`@noCopy` prevents accidental copying; it does not automatically define safe
concurrent access.

Revision 0.1 makes no blanket guarantee that one `TcpStream` or `UdpSocket`
value can be concurrently operated on through unsynchronized aliases.

The compiler's borrowing/resource-effect rules and the package's method
receiver requirements govern access.

A future typed TCP split API may introduce separately owned or lifetime-bounded
read/write halves if required.

Such an API must define:

- who owns the underlying socket;
- whether halves may outlive each other;
- close/shutdown interactions;
- concurrent write ordering;
- cancellation and destruction behavior.

It is not implied by this revision.

---

# Part IX — Capability and target rules

## 24. Pure IP facilities versus active networking

The package contains two broad capability classes.

### 24.1 Pure facilities

Examples:

- IPv4/IPv6 address values;
- address parsing/formatting;
- prefix/network arithmetic;
- endpoint values;
- IP protocol-number representation;
- raw packet parsing.

These can exist on targets without a hosted process socket API, provided the
required language/runtime primitives exist.

### 24.2 Active networking facilities

Examples:

- TCP connect/listen/accept;
- UDP bind/send/receive;
- multicast membership;
- interface enumeration.

These require a selected platform or configured IP stack that provides the
necessary capability.

A schedulerless or bare-metal target must not silently provide fake successful
socket behavior.

When the selected compilation plan proves that active networking is
unavailable, using the unavailable operation must produce a compile-time
capability diagnostic.

An embedded target may legitimately support active `net/ip` through a target
network stack without being a hosted OS.

---

## 25. Platform behavior must preserve semantic distinctions

Platform adapters must preserve at least:

- IPv4 versus IPv6 identity;
- IPv6 scope/interface identity;
- TCP byte-stream semantics;
- UDP datagram boundaries;
- zero-length UDP datagrams;
- partial TCP reads/writes;
- orderly TCP EOF;
- typed network errors;
- explicit local close state;
- ownership and exactly-once resource release;
- cancellation commit semantics;
- actual bound port when port zero was requested.

An adapter that cannot preserve a required semantic distinction must reject the
operation rather than approximate it silently.

---

# Part X — Explicit exclusions and package boundaries

## 26. DNS

`net/ip` does not own:

- host names;
- DNS queries;
- resolver configuration;
- search domains;
- DNS records.

Those belong to `net/dns`.

A future higher-level connection helper may compose DNS resolution with
`ip.TcpStream.Connect`, but `TcpStream.Connect` itself remains concrete-address
based.

---

## 27. URLs and URI authorities

`net/ip` does not own URL parsing, URI schemes, userinfo, URL host syntax, or
percent encoding.

Those belong to `net/url`.

A URL parser may produce a host literal that is then parsed as `ip.Address`.

---

## 28. TLS

TLS ownership remains intentionally open at the `net` root design level.

`TcpStream` is a plain TCP byte stream.

A TLS package may wrap/consume/borrow a TCP stream according to its own
specification, but `net/ip` does not embed TLS behavior in `TcpStream`.

---

## 29. DHCP and address configuration

DHCP remains an open placement decision from `std-net.md`.

`net/ip` may provide types useful to DHCP implementations, but this revision
does not place the DHCP protocol itself in this package.

Likewise, interface address assignment/configuration is distinct from simply
enumerating current interface state and requires a separate explicit design.

---

## 30. Link-layer protocols

ARP, Ethernet framing, IEEE 802.15.4, Wi-Fi management, radio PHY control, and
similar lower-layer concerns are not automatically part of `net/ip`.

Their placement must follow responsibility:

- protocol/network semantics under the appropriate `net` family;
- hardware/controller/PHY access under `hw`;
- link-layer packet formats in an explicitly designed networking package where
  appropriate.

---

## 31. Raw sockets

A public generic raw socket API is not part of revision 0.1.

If added later it must define:

- safety boundary;
- privilege requirements;
- address-family typing;
- protocol typing;
- packet ownership;
- checksum responsibilities;
- platform capability behavior;
- interaction with firewall/kernel policy;
- target-specific unsupported cases.

It must not be introduced merely as:

```sec
Socket(2, 3, 255)
```

or:

```sec
Socket("raw", "icmp")
```

---

# Part XI — Repository migration

## 32. Current repository state reviewed

At repository baseline `afb0bed`, `sec/stdlib/net/` contains IP-related
prototype material directly in the parent directory:

```text
sec/stdlib/net/
├── interfaces.sec
├── ip.sec
├── sockets.sec
├── email.sec
├── uri.sec
├── dns/
└── http/
```

Relevant current facts:

- `ip.sec` is a substantial prototype containing errors, IANA protocol numbers,
  IPv4/IPv6 wire layouts, raw packet parsing, and an early `TcpStream`;
- `interfaces.sec` is empty;
- `sockets.sec` contains only a stub module declaration;
- `email.sec` and `uri.sec` belong to later mail/URL migrations, not this IP
  package migration.

---

## 33. Required migration map

### 33.1 `sec/stdlib/net/ip.sec`

Do **not** move the file unchanged.

Split and redesign it:

| Existing content | Destination |
|---|---|
| `NetError` | redesigned into `net/ip/error.sec` |
| `IpProtocol` | `net/ip/protocol.sec` |
| `IpEcn` | `net/ip/protocol.sec` |
| `Ipv4Flags` | `net/ip/ipv4.sec` |
| `Ipv4Address` | `net/ip/ipv4.sec` |
| `Ipv4WireHeader` | `net/ip/ipv4.sec` |
| `Ipv6Address` | `net/ip/ipv6.sec` |
| `Ipv6WireHeader` | `net/ip/ipv6.sec` |
| `Ipv4Packet` / `Ipv6Packet` / `IpPacket` | `net/ip/packet.sec` |
| `ParseIpPacket` | `net/ip/packet.sec`, expanded into family-specific parsers |
| prototype `TcpStream` | replaced by the `net/ip/tcp.sec` contract |
| `TerminateConnection` example helper | remove from stdlib API |
| `NetworkTask` usage example | move to tests/docs if useful; not stdlib API |

After migration and caller updates, the root `sec/stdlib/net/ip.sec` must be
removed to avoid competing package definitions.

### 33.2 `sec/stdlib/net/interfaces.sec`

Move/replace as:

```text
sec/stdlib/net/ip/interface.sec
```

The current empty file provides no public API that must be preserved.

### 33.3 `sec/stdlib/net/sockets.sec`

Do not preserve the current one-line stub as a public `net_socket` package.

Replace its intended responsibility with:

- public typed APIs in `tcp.sec` and `udp.sec`;
- private target adapter files named `socket.<target>.sec`.

Remove the root stub after migration.

### 33.4 Other root files

The following are explicitly outside this migration:

```text
sec/stdlib/net/email.sec
sec/stdlib/net/uri.sec
```

They are expected to move during the later `net/mail` and `net/url` work.

---

# Part XII — Tests and verification

## 34. Test placement

Package tests follow the Sec stdlib/source testing rules and should live with
the package using `*_test.sec` naming.

Initial expected test families include:

```text
address_test.sec
prefix_test.sec
endpoint_test.sec
packet_test.sec
tcp_test.sec
udp_test.sec
icmp_test.sec
multicast_test.sec
interface_test.sec
```

Target-specific integration tests may be separated where necessary.

---

## 35. Required address tests

At minimum:

- valid IPv4 dotted-decimal parse;
- malformed IPv4 forms;
- IPv4 canonical formatting;
- IPv6 full notation;
- IPv6 compressed notation;
- IPv6 canonical RFC 5952 formatting;
- unspecified/loopback classification;
- multicast classification;
- link-local classification;
- IPv4 private-use classification;
- IPv6 unique-local classification;
- union `Address.Parse`;
- host names rejected by `Address.Parse`;
- fixed byte-order round trips.

---

## 36. Required prefix/network tests

At minimum:

- `/0`, full-width prefix, and intermediate prefixes;
- out-of-range prefix conversion rejection;
- constructor clears host bits;
- CIDR parser canonicalizes host bits;
- same-family `Contains`;
- cross-family `Network.Contains` returns false;
- canonical `ToString()`.

---

## 37. Required packet tests

At minimum:

- IPv4 minimum header;
- IPv4 options;
- invalid IPv4 IHL;
- truncated IPv4 header;
- inconsistent total length;
- IPv6 minimum header;
- truncated IPv6 header;
- invalid version;
- payload-length consistency;
- borrowed payload refers to source storage;
- parser performs no owning allocation;
- malformed packet errors remain `PacketError`.

---

## 38. Required TCP tests

Where the target has active networking capability:

- loopback connect/listen/accept;
- IPv4;
- IPv6;
- port-zero bind and actual `LocalEndpoint`;
- partial read handling;
- `ReadExact`;
- partial write handling;
- `WriteAll`;
- orderly EOF returns zero from `Read`;
- half shutdown;
- `SetNoDelay`;
- idempotent `Close`;
- operations after close return `Closed`;
- move-only ownership;
- deterministic cleanup;
- cancellation race around connect/accept/read;
- native errors mapped to semantic `NetworkError`.

---

## 39. Required UDP tests

Where supported:

- IPv4 loopback datagram;
- IPv6 loopback datagram;
- zero-length datagram;
- source endpoint preserved;
- datagram boundary preserved;
- receive truncation reported;
- oversize send mapped correctly;
- broadcast enable/disable;
- idempotent close;
- operations after close;
- cancellation race around receive.

---

## 40. Required multicast/interface tests

Where supported:

- IPv4 group membership;
- IPv6 group membership;
- explicit interface index;
- default interface selection;
- non-multicast group rejection;
- interface enumeration exhaustion;
- index/name lookup;
- interface addresses preserve host bits;
- `AddressesToArray` allocation behavior is documented and tested.

---

# Part XIII — Scaffolding and implementation instructions

## 41. AI/repository task requirements

An AI or repository automation implementing this book should:

1. create `sec/stdlib/net/ip/` if missing;
2. create `std-net-ip.md` in that directory;
3. create the declared source files that are missing;
4. inspect existing `sec/stdlib/net/ip.sec`, `interfaces.sec`, and
   `sockets.sec`;
5. migrate useful existing implementation rather than discarding it blindly;
6. update existing code where the new API differs;
7. preserve the current IANA `IpProtocol` work and verify numeric values against
   the current IANA registry;
8. update source-file standards comments with authoritative URLs;
9. remove old root IP/socket/interface files only after their useful content is
   migrated and references are updated;
10. avoid implementing `TcpStream` with a mock handle;
11. avoid public raw native socket descriptors;
12. avoid DNS resolution inside `TcpStream.Connect`;
13. avoid magic string/integer socket option APIs;
14. add/update tests for every implemented public behavior;
15. update `implementation-status-std-net-ip.yaml`;
16. reconcile that status fragment into the repository's canonical
    `implementation-status.yaml` without duplicating stronger/newer evidence;
17. report compiler/platform gaps instead of fabricating runtime behavior.

---

## 42. Implementation order

Recommended order:

1. create package/module structure;
2. migrate `protocol.sec`;
3. migrate and complete IPv4/IPv6 address types;
4. add `Address`, prefix/network, port, endpoint, and interface-index values;
5. split raw packet representation/parsing;
6. define common `NetworkError`;
7. implement the first hosted target socket adapter;
8. implement `TcpStream` / `TcpListener`;
9. implement `UdpSocket`;
10. implement interface queries;
11. implement multicast;
12. complete ICMP foundational codecs;
13. expand targets;
14. complete packet encoding/checksum helpers once required compiler lowering is
    available.

Pure value types should not be blocked on socket backend work.

---

# Part XIV — Open observations

## 43. Items intentionally not locked in revision 0.1

The following require later decisions or dogfooding:

- exact `ReadExact` early-EOF error alignment with the common `io` model;
- TCP listen backlog API;
- TCP keepalive configuration;
- linger semantics;
- reusable-address/reusable-port behavior;
- connected UDP state model;
- TCP read/write split halves;
- zero-allocation network-interface address iteration;
- interface flags and operational state;
- MAC/hardware-address ownership;
- route table and gateway APIs;
- raw socket API;
- high-level ICMP echo/ping API;
- transport wire-header codecs for TCP and UDP;
- full IPv6 extension-header parser family;
- IPv4/IPv6 packet encoding helpers;
- checksum helper public naming;
- DHCP ownership;
- exact interaction between context deadlines and platform socket timeout
  options;
- whether a later shared `io.Reader` / `io.Writer` interface should be
  implemented by `TcpStream` once the common I/O interfaces are finalized.

These observations are not permission for implementations to invent
incompatible public APIs.

---

## 44. Decisions locked by this revision

Revision 0.1 establishes the following package-level direction:

- canonical package path is `net/ip`;
- users import `net/ip` and use `ip.X`;
- IPv4, IPv6, TCP, UDP, ICMP, multicast, and IP interface primitives are in the
  same `ip` package initially;
- `Address` is a typed IPv4/IPv6 union;
- endpoints are family-aware and preserve IPv6 zone/interface information;
- `Port` is a nominal `uint16` and permits zero;
- `IpProtocol` is the open IANA 8-bit protocol-number namespace;
- existing shared `TransportProtocol` is reused rather than duplicated;
- TCP uses `TcpStream` and `TcpListener`;
- UDP uses `UdpSocket` and preserves datagram boundaries/truncation;
- owning socket resources are `@noCopy`;
- DNS names are not accepted as IP addresses;
- ordinary active networking does not expose a generic raw native socket API;
- platform socket constants and handles remain private;
- network waits use Sec's common cancellation/context model;
- raw IP parsing remains allocation-free and borrow-based;
- existing root `ip.sec` is split, not moved wholesale;
- existing root `interfaces.sec` and `sockets.sec` are absorbed into the new
  package and removed after migration.

---

**Document revision:** 0.1  
**Last updated:** 2026-09-07
