# Sec Standard Library — Networking

**Status:** Draft  
**Created:** 2026-09-07  
**Updated:** 2026-09-07  
**Revision:** 0.1  
**Sec version:** 0.1  
**Document:** `std-net.md`

---

## 1. Purpose

The `net` standard-library area defines Sec's networking facilities, network protocols, network-facing data types, and related protocol implementations.

This document defines the overall package structure and common design principles for `stdlib/net`.

Detailed APIs are specified in dedicated documents such as:

- `std-net-ip.md`
- `std-net-http.md`
- `std-net-mqtt.md`
- `std-net-mail.md`
- `std-net-iot.md`
- `std-net-industrial.md`

Additional documents may be added as the networking standard library grows.

This document is expected to evolve as individual protocol families are designed.

---

## 2. General design principles

### 2.1 Package imports

A package is imported by its canonical path:

```sec
import "net/http"
import "net/ip"
import "net/mqtt"
```

The final package component is used as the normal package qualifier:

```sec
let cookie := new http.Cookie(...)
let client := new mqtt.Client(...)
```

Importing `net/http` does not require references such as:

```sec
net.http.Cookie
```

The full path identifies the package; the imported package name identifies it in source code.

---

### 2.2 Typed protocol values

Strings are valid protocol values where the value domain is genuinely textual, open, extensible, or externally defined.

Where a protocol defines a closed namespace of named alternatives, the standard library should normally represent those alternatives using a Sec `enum` rather than magic strings.

Other Sec types such as structs, unions, named types, or existing shared types should be used where they model the protocol more accurately.

Existing common types must be reused rather than duplicated by individual networking packages.

For example, the existing `TransportProtocol` type must be reused where applicable.

---

### 2.3 Package placement follows responsibility

Packages are classified primarily by what they do rather than by the physical medium over which they communicate.

A protocol does not become hardware merely because it ultimately uses a radio, Ethernet controller, serial interface, or another physical transport.

Hardware-facing access belongs under `hw` when the API represents hardware devices, controllers, radios, registers, buses, PHY access, or equivalent low-level facilities.

Protocol and network-stack semantics belong under `net`.

For example:

```text
hw/
    ... radio / PHY / controller access ...

net/iot/thread
net/iot/zigbee
net/iot/matter
```

The exact boundary is specified by the relevant package documents.

---

## 3. Core IP networking

IP networking is grouped under:

```text
net/ip
```

rather than placing TCP, UDP, IP addresses, and similar facilities directly in the `net` root.

The `net/ip` family is expected to cover facilities including:

- IPv4
- IPv6
- IP addresses
- network prefixes
- endpoints
- ports
- TCP
- UDP
- ICMP
- multicast
- IP-related socket semantics
- related common network primitives

The detailed structure is defined in `std-net-ip.md`.

---

## 4. Package taxonomy

The following taxonomy is the current first-pass structure for Sec networking.

Not every listed protocol is necessarily required for Sec 0.1. Some branches represent longer-term standard-library candidates.

### 4.1 Core Internet networking

```text
net/ip
net/dns
net/url
net/quic
```

Areas include:

- IPv4 and IPv6
- TCP
- UDP
- ICMP
- DNS
- DHCP where appropriate
- URL and URI handling
- QUIC

The exact placement of DHCP remains to be defined during the IP/DNS design.

---

### 4.2 Web protocols

```text
net/http
net/websocket
net/webdav
```

`net/http` owns the HTTP protocol family and common HTTP abstractions.

HTTP/1.1, HTTP/2, and HTTP/3 should normally be treated as protocol versions within the HTTP package rather than independent top-level package families.

HTTP/3 may depend on `net/quic`.

---

### 4.3 Messaging and RPC

```text
net/mqtt
net/amqp
net/rpc
```

This group covers message-oriented and remote-call communication.

Expected areas include:

- MQTT
- AMQP
- generic or Sec-specific RPC facilities

The precise meaning and scope of `net/rpc` must be specified before its public API is fixed.

---

## 5. Mail

Common mail abstractions belong under:

```text
net/mail
```

Protocol-specific facilities may be structured beneath it:

```text
net/mail/smtp
net/mail/imap
net/mail/pop3
```

The `mail` package should own common message concepts where those concepts are not specific to one transport protocol.

MIME placement remains to be decided. It may belong outside `net` if treated primarily as a general data format.

---

## 6. Remote access and file transfer

Current candidates include:

```text
net/ssh
net/sftp
net/ftp
net/tftp
```

SSH and SFTP are strong standard-library candidates.

FTP and TFTP are older protocols but remain in active use and are therefore retained in the candidate set.

Legacy status alone is not sufficient reason to treat an actively used protocol as dead.

---

## 7. Discovery

Current discovery-related candidates include:

```text
net/mdns
net/dnssd
net/ssdp
```

This covers areas such as:

- multicast DNS
- DNS Service Discovery
- local network discovery
- SSDP

Their final package grouping may be refined if a common discovery abstraction proves useful.

---

## 8. Network management

Current candidates include:

```text
net/snmp
net/lldp
net/netconf
net/restconf
```

These protocols are primarily concerned with discovering, monitoring, configuring, or managing networked systems.

---

## 9. Authentication, authorization, and directory protocols

Current candidates include:

```text
net/ldap
net/radius
net/tacacs
```

These remain separate from general cryptographic primitives and authentication algorithms.

The package owns protocol communication, not the underlying cryptographic implementations.

---

## 10. Realtime communication and media

Current candidates include:

```text
net/sip
net/rtp
net/rtsp
net/stun
net/turn
net/ice
net/webrtc
```

This is a broad candidate family covering:

- session establishment
- realtime media transport
- streaming
- NAT traversal
- peer connectivity

Some of these implementations may ultimately be considered too large or specialized for the core standard library.

They remain part of the networking candidate inventory until that decision is made explicitly.

---

## 11. IoT networking

IoT-oriented networking is grouped under:

```text
net/iot
```

Current candidates include:

```text
net/iot/matter
net/iot/thread
net/iot/zigbee
net/iot/zwave
net/iot/lorawan
net/iot/coap
```

This group contains networking and application-protocol stacks commonly used by connected devices.

The existence of an underlying radio or PHY does not by itself move these protocols into `hw`.

For example, low-level radio or IEEE 802.15.4 controller access may belong under `hw`, while the Thread or Zigbee network stack belongs under `net/iot`.

---

## 12. Industrial networking

Industrial protocols are grouped under:

```text
net/industrial
```

rather than under an `iiot` namespace.

Current candidates include:

```text
net/industrial/modbus
net/industrial/opcua
net/industrial/bacnet
net/industrial/knx
```

`industrial` describes the technical and application domain more consistently than `iiot`, since these protocols are not limited to systems marketed as Industrial IoT.

Protocols that have both serial and network transports, such as Modbus, require explicit separation between protocol semantics and transport-specific bindings.

---

## 13. Cellular networking

Current candidates are grouped provisionally under:

```text
net/cellular
```

including:

```text
net/cellular/nbiot
net/cellular/ltem
```

This area requires further design because cellular networking spans several layers.

A likely responsibility boundary is:

```text
hw/
    modem / radio / SIM-facing hardware facilities

net/cellular/
    network registration
    sessions
    bearers
    cellular network semantics
```

This placement remains under observation.

---

## 14. Routing protocols

Current candidates include:

```text
net/bgp
net/ospf
```

These are active and important network protocols but are specialized.

Whether complete routing-protocol implementations belong in the Sec standard library or in external packages remains open.

---

## 15. Network infrastructure services

Current candidates include:

```text
net/syslog
net/proxy
net/socks
```

NTP is also part of the networking inventory but its final ownership remains open.

Possible placements include:

```text
net/ntp
```

or:

```text
time/ntp
```

The decision should follow package responsibility rather than transport mechanism.

---

## 16. Network filesystems

Protocols such as:

```text
SMB
NFS
```

are active network protocols but primarily expose filesystem semantics.

They should not automatically be placed under `net`.

Possible future ownership includes filesystem-oriented standard-library packages or external libraries.

They remain part of the candidate inventory until their ownership is decided.

---

## 17. TLS placement

TLS is required by several networking packages, including HTTP, MQTT, mail protocols, and other secure transports.

Its canonical ownership is intentionally not yet fixed.

Current candidates include:

```text
net/tls
```

and a suitable cryptography-oriented location.

The design must distinguish between:

- TLS protocol and transport semantics
- certificates
- X.509 handling
- cryptographic algorithms
- keys and trust stores

TLS placement remains on the networking observation list until these responsibilities are defined.

---

## 18. Initial package structure

The current high-level structure is:

```text
stdlib/
└── net/
    ├── ip/
    ├── dns/
    ├── url/
    ├── http/
    ├── websocket/
    ├── webdav/
    ├── quic/
    │
    ├── mqtt/
    ├── amqp/
    ├── rpc/
    │
    ├── mail/
    │   ├── smtp/
    │   ├── imap/
    │   └── pop3/
    │
    ├── ssh/
    ├── sftp/
    ├── ftp/
    ├── tftp/
    │
    ├── mdns/
    ├── dnssd/
    ├── ssdp/
    │
    ├── snmp/
    ├── lldp/
    ├── netconf/
    ├── restconf/
    │
    ├── ldap/
    ├── radius/
    ├── tacacs/
    │
    ├── sip/
    ├── rtp/
    ├── rtsp/
    ├── stun/
    ├── turn/
    ├── ice/
    ├── webrtc/
    │
    ├── iot/
    │   ├── matter/
    │   ├── thread/
    │   ├── zigbee/
    │   ├── zwave/
    │   ├── lorawan/
    │   └── coap/
    │
    ├── industrial/
    │   ├── modbus/
    │   ├── opcua/
    │   ├── bacnet/
    │   └── knx/
    │
    ├── cellular/
    │   ├── nbiot/
    │   └── ltem/
    │
    ├── bgp/
    ├── ospf/
    │
    ├── syslog/
    ├── proxy/
    └── socks/
```

This is a first-pass structural map.

Individual branches may be moved, merged, renamed, rejected from stdlib, or delegated to another standard-library area as their detailed designs are developed.

---

## 19. Package-specific documents

Each substantial package family should receive its own specification document where necessary.

Examples:

```text
std-net-ip.md
std-net-dns.md
std-net-http.md
std-net-websocket.md
std-net-quic.md
std-net-mqtt.md
std-net-rpc.md
std-net-mail.md
std-net-ssh.md
std-net-iot.md
std-net-industrial.md
std-net-cellular.md
```

A package-specific document owns the detailed public API and file manifest for that package.

For example, `std-net-http.md` may define files such as:

```text
stdlib/net/http/
    client.sec
    server.sec
    request.sec
    response.sec
    header.sec
    cookie.sec
    set_cookie.sec
    cookie_jar.sec
    ...
```

The package-specific specification defines which files are normative.

---

## 20. Repository scaffolding

This document and its package-specific child documents are intended to be usable by implementation tooling and AI-assisted repository tasks.

A scaffolding task must:

1. Read `std-net.md`.
2. Create declared networking directories that are missing.
3. Read each available `std-net-*.md` package specification.
4. Create files explicitly required by the corresponding package specification when those files do not already exist.
5. Preserve all existing files.
6. Never overwrite an existing implementation merely to conform to scaffolding.
7. Never invent additional package files not described by the specification.
8. Report discrepancies between the repository and the documented package manifest.
9. Leave API implementation to the relevant implementation task unless the task explicitly requests implementation.

Package documents are therefore the canonical source for the intended stdlib networking layout.

---

## 21. Implementation status

This document has a corresponding implementation-status patch:

```text
implementation-status-std-net.yaml
```

Each later package-specific specification should receive its corresponding implementation-status file, for example:

```text
implementation-status-std-net-http.yaml
implementation-status-std-net-ip.yaml
implementation-status-std-net-mqtt.yaml
```

Implementation status must distinguish between at least:

- package structure defined
- files scaffolded
- public API defined
- implementation started
- implementation complete
- tests present
- platform support
- known gaps or blocked functionality

The exact status schema follows the common Sec implementation-status governance.

---

## 22. Open observations

The following issues are intentionally left open in this revision:

- canonical TLS ownership
- NTP ownership
- final cellular/hardware boundary
- exact DHCP placement
- MIME ownership
- whether routing protocols belong in stdlib
- whether large realtime stacks such as WebRTC belong in stdlib
- whether SMB/NFS belong elsewhere in stdlib or externally
- whether common abstractions should be introduced for discovery protocols
- whether some candidate protocol families should remain external packages
- final file manifests for individual protocol packages

These questions should be resolved while the corresponding package specifications are developed.

---

**Revision:** 0.1  
**Updated:** 2026-09-07