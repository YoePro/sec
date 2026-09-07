# Sec Standard Library — DNSSEC

- **Status:** Draft
- **Created:** 2026-09-07
- **Last updated:** 2026-09-07
- **Document revision:** 0.1
- **Sec language version:** 0.1
- **Canonical path:** `stdlib/net/dns/dnssec/std-net-dns-dnssec.md`
- **Repository path:** `sec/stdlib/net/dns/dnssec/std-net-dns-dnssec.md`
- **Parent specification:** `stdlib/net/dns/std-net-dns.md`
- **Implementation-status fragment:** `implementation-status-std-net-dns-dnssec.yaml`
- **Latest verified repository main:** `0f5027d`
- **Standards state rechecked:** 2026-09-07

---

# 1. Purpose

`net/dns/dnssec` implements local DNS Security Extensions (DNSSEC) validation
semantics above the DNS wire types defined by `net/dns`.

The package owns:

- DNSSEC validation status;
- trust-anchor representation and storage;
- configured trust anchors;
- RFC 5011 trust-anchor maintenance;
- DNSSEC canonical name/RR/RRset encoding;
- DNSKEY key-tag computation;
- DS-to-DNSKEY matching;
- RRSIG verification orchestration;
- chain-of-trust validation;
- secure/insecure delegation handling;
- NSEC authenticated denial of existence;
- NSEC3 authenticated denial of existence;
- wildcard proof validation;
- signature inception/expiration checks;
- algorithm-policy enforcement using the live IANA DNSSEC registries;
- DNSSEC validation caching;
- validation diagnostics;
- integration requirements for the common DNS resolver.

The package is imported as:

```sec
import "net/dns/dnssec"
```

Typical direct use is conceptually:

```sec
let anchors := try dnssec.TrustAnchorStore.System()
let validator := try dnssec.Validator.New(<-anchors, dnssec.ValidationPolicy.Default())

let result := try validator.Validate(question, response, transport)
```

The package does not duplicate DNSSEC resource-record wire structures already
owned by `net/dns`.

It reuses:

```sec
dns.DnskeyRecord
dns.DsRecord
dns.RrsigRecord
dns.NsecRecord
dns.Nsec3Record
dns.Nsec3ParamRecord
dns.DnssecAlgorithm
dns.DsDigestType
```

---

# 2. Package boundary

## 2.1 Base DNS owns wire data

`net/dns` owns:

- DNSKEY;
- DS;
- RRSIG;
- NSEC;
- NSEC3;
- NSEC3PARAM;
- DNSSEC algorithm/digest registry values;
- DNS message encoding/decoding;
- AD/CD flags;
- EDNS DO signaling.

`net/dns/dnssec` owns the meaning of that data during local validation.

## 2.2 DNSSEC is not a DNS transport

The validator may validate data obtained through:

```text
classic DNS
DoT
DoH
DoQ
```

It does not care which transport carried the DNS messages once the required
DNS data is available.

DNSSEC authenticity and transport confidentiality/authentication are separate
properties.

An authenticated DoH/DoT/DoQ server does not make unsigned DNS data DNSSEC
Secure.

Conversely, DNSSEC can validate signed DNS data delivered over an
unauthenticated classic DNS transport.

## 2.3 Zone signing boundary

Revision 0.1 focuses on **validation**.

The DNSSEC package also owns the canonical cryptographic rules required by
future signing, but a full zone-signing workflow is not part of this revision.

High-level zone signing requires:

- complete zone traversal;
- NSEC/NSEC3 chain construction;
- RRSIG insertion/removal;
- SOA serial policy;
- key lifecycle;
- private-key storage;
- resigning schedules.

Those operations belong to a future integration between:

```text
net/dns/dnssec
net/dns/zone
```

The packages must not form a dependency cycle.

DNSSEC signing primitives, when added, operate on shared `dns.Rrset` values
and do not import `net/dns/zone`.

## 2.4 Crypto ownership

This package does not implement:

- SHA;
- RSA;
- ECDSA;
- EdDSA;
- private-key storage;
- X.509.

It uses the canonical Sec cryptography APIs.

DNSSEC algorithm identifiers remain DNS protocol values; cryptographic
implementations remain in `crypto`.

---

# 3. Standards and registries

## 3.1 Core DNSSEC

Primary DNSSEC specifications:

- **RFC 4033 — DNS Security Introduction and Requirements**  
  https://www.rfc-editor.org/rfc/rfc4033
- **RFC 4034 — Resource Records for the DNS Security Extensions**  
  https://www.rfc-editor.org/rfc/rfc4034
- **RFC 4035 — Protocol Modifications for the DNS Security Extensions**  
  https://www.rfc-editor.org/rfc/rfc4035
- **RFC 6840 — Clarifications and Implementation Notes for DNSSEC**  
  https://www.rfc-editor.org/rfc/rfc6840

RFC 6840 updates RFCs 4033, 4034, 4035, and 5155.

## 3.2 NSEC3

- **RFC 5155 — DNSSEC Hashed Authenticated Denial of Existence**  
  https://www.rfc-editor.org/rfc/rfc5155
- **RFC 9276 / BCP 236 — Guidance for NSEC3 Parameter Settings**  
  https://www.rfc-editor.org/rfc/rfc9276

RFC 9276 updates NSEC3 operational parameter guidance.

A validating resolver must implement NSEC3 validation according to the
applicable protocol rules and bound the amount of expensive hashing work it
will perform.

## 3.3 Aggressive validated negative cache

- **RFC 8198 — Aggressive Use of DNSSEC-Validated Cache**  
  https://www.rfc-editor.org/rfc/rfc8198

A validating resolver can synthesize validated negative answers from cached
NSEC/NSEC3 proof material when the RFC requirements are met.

## 3.4 Trust anchors

- **RFC 5011 / STD 74 — Automated Updates of DNSSEC Trust Anchors**  
  https://www.rfc-editor.org/rfc/rfc5011

RFC 5011 is an Internet Standard.

The package must support manually configured trust anchors first and provides
the state model required for RFC 5011 automated maintenance.

## 3.5 Algorithm policy

- **RFC 9904 — DNSSEC Cryptographic Algorithm Recommendation Update Process**  
  https://www.rfc-editor.org/rfc/rfc9904

RFC 9904 obsoletes RFC 8624 and moves the canonical implementation/use
recommendations into IANA registries.

Therefore the package must **not** hard-code RFC 8624 as permanent policy.

Current authoritative registries include:

- DNSSEC Algorithm Numbers  
  https://www.iana.org/assignments/dns-sec-alg-numbers
- Delegation Signer Digest Algorithms  
  https://www.iana.org/assignments/ds-rr-types
- DNSKEY Flags  
  https://www.iana.org/assignments/dnskey-flags

The DNSSEC Algorithm Numbers registry was rechecked on 2026-09-07; IANA
reported it updated on 2026-08-10.

## 3.6 Delegation automation

Related standards:

- RFC 7344 — Automating DNSSEC Delegation Trust Maintenance  
  https://www.rfc-editor.org/rfc/rfc7344
- RFC 8078 — Managing DS Records from the Parent via CDS/CDNSKEY  
  https://www.rfc-editor.org/rfc/rfc8078
- RFC 9615 — Automatic DNSSEC Bootstrapping Using Authenticated Signals  
  https://www.rfc-editor.org/rfc/rfc9615

RFC 9615 updates RFCs 7344 and 8078.

These are primarily authoritative/parental operational mechanisms. Their wire
records are represented by `net/dns`; automated parent/child provisioning is
not part of the revision-0.1 validating stub API.

## 3.7 Operational guidance

- RFC 6781 — DNSSEC Operational Practices, Version 2  
  https://www.rfc-editor.org/rfc/rfc6781
- RFC 8901 — Multi-Signer DNSSEC Models  
  https://www.rfc-editor.org/rfc/rfc8901

These inform signing/key-management work but do not change ordinary resolver
validation semantics.

---

# 4. Source-file standards headers

Every source file must state the standards directly governing it.

Example:

```sec
// Sec Standard Library: net/dns/dnssec
// File: validator.sec
//
// Standards:
//   RFC 4033 - DNSSEC Introduction and Requirements
//   https://www.rfc-editor.org/rfc/rfc4033
//
//   RFC 4035 - DNSSEC Protocol Modifications
//   https://www.rfc-editor.org/rfc/rfc4035
//
// Updates/clarifications:
//   RFC 6840
//   https://www.rfc-editor.org/rfc/rfc6840
//
// Algorithm policy:
//   RFC 9904
//   https://www.rfc-editor.org/rfc/rfc9904
//   https://www.iana.org/assignments/dns-sec-alg-numbers

module dnssec
```

Files implementing IANA-backed policy must record the registry URL and
synchronization date.

---

# 5. Package and file manifest

Initial package layout:

```text
stdlib/net/dns/dnssec/
├── std-net-dns-dnssec.md
├── error.sec
├── status.sec
├── policy.sec
├── canonical.sec
├── keytag.sec
├── trust_anchor.sec
├── anchor_update.sec
├── algorithm.sec
├── signature.sec
├── delegation.sec
├── denial_nsec.sec
├── denial_nsec3.sec
├── cache.sec
├── validator.sec
└── integration.sec
```

All source files use:

```sec
module dnssec
```

Expected tests:

```text
canonical_test.sec
keytag_test.sec
trust_anchor_test.sec
anchor_update_test.sec
algorithm_test.sec
signature_test.sec
delegation_test.sec
denial_nsec_test.sec
denial_nsec3_test.sec
cache_test.sec
validator_test.sec
integration_test.sec
```

---

# Part I — Parent DNS correction

# 6. Shared RRset type

DNSSEC signs and validates **RRsets**, not arbitrary independent records.

`std-net-dns.md` revision 0.1 currently exposes `dns.Record` but does not yet
define a canonical RRset value.

That parent book must be updated.

The required type belongs in:

```text
stdlib/net/dns/record.sec
```

not in this child package.

Required semantic shape:

```sec
type Rrset struct {
    Name: Name,
    Class: DnsClass,
    Ttl: Ttl,
    Data: RecordData[],
}
```

Construction must enforce:

- `Data` is non-empty;
- every `RecordData` item represents the same `RecordType`;
- owner name/class/TTL are shared by the RRset;
- duplicate RDATA values are rejected according to DNS RRset semantics.

Canonical behavior:

```sec
impl Rrset {
    static fn New(
        name: Name,
        class: DnsClass,
        ttl: Ttl,
        data: RecordData[]
    ) Result[Rrset, RrsetError]

    property Type: RecordType { get { ... } }
    property Count: uint { get { ... } }

    fn ToRecords() Record[]
}
```

The exact allocation-aware naming of `ToRecords()` may be revised to
`ToRecordsArray()` if the common collection naming rule requires it.

DNSSEC and zone code must share this one type.

---

# Part II — Errors and security status

# 7. `error.sec`

## 7.1 `ValidationFailure`

```sec
enum ValidationFailure error {
    NoTrustAnchor,
    MissingDnskey,
    MissingDs,
    MissingRrsig,
    NoMatchingKey,
    NoSupportedAlgorithm,
    NoSupportedDigest,
    SignatureInvalid,
    SignatureNotYetValid,
    SignatureExpired,
    KeyTagMismatch,
    DsDigestMismatch,
    InvalidDnskey,
    InvalidDs,
    InvalidRrsig,
    InvalidNsecProof,
    InvalidNsec3Proof,
    Nsec3IterationLimitExceeded,
    WildcardProofMissing,
    DelegationProofInvalid,
    ChainLoop,
    ChainTooDeep,
    ClockUnavailable,
    ResourceLimitExceeded,
}
```

## 7.2 `TrustAnchorError`

```sec
enum TrustAnchorError error {
    InvalidAnchor,
    DuplicateAnchor,
    NotFound,
    PersistentStorageUnavailable,
    InvalidState,
    UpdateRejected,
}
```

## 7.3 `DnssecError`

```sec
union DnssecError error {
    Validation(ValidationFailure),
    TrustAnchor(TrustAnchorError),
    Dns(dns.MessageError),
    Transport(error),
    Crypto(error),
    Unsupported,
}
```

`Transport(error)` preserves the concrete DNS transport failure.

`Crypto(error)` preserves the concrete canonical crypto failure.

A DNSSEC **Bogus** result is not silently converted into generic
`dns.ResolveError.NoData`.

---

# 8. `status.sec`

## 8.1 Security status

RFC 4035 defines the validation states:

```sec
enum SecurityStatus {
    Secure,
    Insecure,
    Bogus,
    Indeterminate,
}
```

Semantics:

- `Secure`: the RRset has a validated chain of trust from a configured trust
  anchor;
- `Insecure`: the validator has established that no chain of signed DNSKEY/DS
  records from a trusted starting point exists for this data;
- `Bogus`: a chain should be valid but validation failed or required signed
  material is missing/invalid;
- `Indeterminate`: the validator cannot determine whether the data should be
  signed because required information could not be obtained/evaluated.

These are protocol security states, not subjective warning levels.

## 8.2 `ValidationResult`

```sec
type ValidationResult struct {
    Status: SecurityStatus,
    Failure: Option[ValidationFailure],
}
```

Rules:

- `Secure` has `Failure = None`;
- `Insecure` normally has `Failure = None`;
- `Bogus` must include a useful failure reason;
- `Indeterminate` should include a reason when known.

## 8.3 Whole-response security

Different RRsets in one DNS response can have different validation relevance.

The validator must not blindly assign one status to every record merely
because one answer RRset validated.

A whole-response result is derived from the RRsets needed to answer the
question and the denial/delegation proofs required by DNSSEC.

---

# Part III — Validation policy

# 9. `policy.sec`

## 9.1 Responsibility

`policy.sec` owns bounded validation policy, not cryptographic algorithm
registries themselves.

## 9.2 `ValidationPolicy`

```sec
type ValidationPolicy struct {
    MaxChainDepth: uint8,
    MaxNsec3Iterations: uint16,
    AllowInsecureDelegation: bool,
    RequireSecure: bool,
}
```

Canonical defaults:

```sec
impl ValidationPolicy {
    static fn Default() ValidationPolicy
}
```

Revision-0.1 defaults conceptually:

```text
MaxChainDepth           = bounded implementation value
MaxNsec3Iterations      = current BCP-compatible validator limit
AllowInsecureDelegation = true
RequireSecure           = false
```

The exact NSEC3 iteration ceiling must follow current operational/security
guidance and may be lowered as recommended by RFC 9276.

It must not be an effectively unbounded attacker-controlled workload.

## 9.3 Strict validation use

When:

```sec
RequireSecure = true
```

`Insecure` is not accepted as a successful application result.

This is useful for applications that require DNSSEC-authenticated data.

The validator still distinguishes `Insecure` from `Bogus` internally and in
diagnostics.

---

# Part IV — Canonical DNSSEC encoding

# 10. `canonical.sec`

## 10.1 Responsibility

`canonical.sec` implements RFC 4034 canonical DNS name, RR, and RRset forms
used for signatures and authenticated denial.

Primary reference:

https://www.rfc-editor.org/rfc/rfc4034

## 10.2 Canonical name order

```sec
fn CompareCanonicalNames(
    left: ref dns.Name,
    right: ref dns.Name
) Ordering
```

Ordering follows RFC 4034 section 6.1.

It is:

- label-oriented;
- compared from the most significant/rightmost label;
- ASCII case-insensitive according to DNSSEC canonical rules;
- byte-defined, not locale-aware.

## 10.3 Canonical RR encoding

```sec
fn EncodeCanonicalRecord(
    record: ref dns.Record,
    originalTtl: dns.Ttl,
    output: ref mut byte[]
) Result[uint, DnssecError]
```

Required canonicalization includes:

- no DNS name compression;
- canonical owner name;
- canonical embedded DNS names for RR types whose canonical RDATA rules
  require it;
- original TTL used for signature input;
- correct wildcard owner treatment.

## 10.4 Canonical RRset ordering

```sec
fn EncodeCanonicalRrset(
    rrset: ref dns.Rrset,
    originalTtl: dns.Ttl,
    output: ref mut byte[]
) Result[uint, DnssecError]
```

RRs are sorted according to canonical RDATA ordering.

Caller-owned output storage is used to avoid mandatory hidden heap allocation.

---

# 11. `keytag.sec`

## 11.1 Responsibility

`keytag.sec` implements DNSKEY key-tag calculation and matching.

Canonical API:

```sec
fn KeyTag(
    key: ref dns.DnskeyRecord
) uint16
```

The calculation follows RFC 4034.

The implementation must handle algorithm-specific historical exceptions
required by the standard rather than applying one naive checksum formula to
every key.

---

# Part V — Trust anchors

# 12. `trust_anchor.sec`

## 12.1 Trust anchor representation

```sec
type TrustAnchor union {
    Dnskey {
        Name: dns.Name,
        Key: dns.DnskeyRecord,
    }

    Ds {
        Name: dns.Name,
        Ds: dns.DsRecord,
    }
}
```

A trust anchor binds a DNS name to trusted DNSSEC key material.

It is not merely a bare public key.

## 12.2 `TrustAnchorStore`

```sec
@noCopy
type TrustAnchorStore struct {
    // Private anchor set and optional persistent RFC 5011 state.
}
```

Canonical API:

```sec
impl TrustAnchorStore {
    static fn Empty() TrustAnchorStore

    static fn System()
        Result[TrustAnchorStore, TrustAnchorError]

    fn Add(
        anchor: TrustAnchor
    ) Result[void, TrustAnchorError]

    fn Remove(
        name: ref dns.Name,
        keyTag: uint16
    ) Result[bool, TrustAnchorError]

    fn AnchorsFor(
        name: ref dns.Name
    ) Result[TrustAnchor[], TrustAnchorError]

    fn Close() Result[void, TrustAnchorError]
}
```

`System()` loads platform/application configured anchors according to the
selected target integration.

Revision 0.1 does not assume one hard-coded root trust anchor forever.

## 12.3 Persistent state

RFC 5011 automated updates require persistent state across process restarts.

A platform that cannot persist RFC 5011 state may still use manually
configured trust anchors.

It must not claim RFC 5011 automation while discarding hold-down state on
every restart.

---

# 13. `anchor_update.sec`

## 13.1 Responsibility

`anchor_update.sec` implements the RFC 5011 trust-anchor state machine.

## 13.2 `AnchorState`

```sec
enum AnchorState {
    Valid,
    AddPending,
    Revoked,
    Removed,
}
```

The exact internal transition timestamps are private implementation state.

## 13.3 `AnchorUpdater`

```sec
@noCopy
type AnchorUpdater struct {
    // Private RFC 5011 state bound to a TrustAnchorStore.
}
```

Canonical semantic API:

```sec
impl AnchorUpdater {
    static fn New(
        store: <- TrustAnchorStore
    ) Result[AnchorUpdater, TrustAnchorError]

    fn ObserveValidatedDnskeyRrset(
        name: ref dns.Name,
        rrset: ref dns.Rrset
    ) Result[void, TrustAnchorError]

    fn Commit()
        Result[void, TrustAnchorError]

    fn Close()
        Result[void, TrustAnchorError]
}
```

Rules:

- only properly validated DNSKEY observations can drive automatic anchor
  state;
- hold-down periods follow RFC 5011;
- wall-clock jumps must not trivially bypass security hold-down periods;
- persistent update state must be committed atomically enough to avoid
  promoting a partially written anchor state after crash.

---

# Part VI — Algorithm policy

# 14. `algorithm.sec`

## 14.1 Responsibility

`algorithm.sec` maps DNSSEC protocol algorithm identifiers to current IANA
implementation/use policy and available Sec crypto capabilities.

It does not redefine `dns.DnssecAlgorithm`.

## 14.2 Policy values

```sec
enum AlgorithmRequirement {
    Must,
    Recommended,
    May,
    MustNot,
    Unspecified,
}
```

A policy record:

```sec
type AlgorithmPolicy struct {
    SigningUse: AlgorithmRequirement,
    ValidationUse: AlgorithmRequirement,
    SigningImplementation: AlgorithmRequirement,
    ValidationImplementation: AlgorithmRequirement,
}
```

Canonical API:

```sec
fn PolicyFor(
    algorithm: dns.DnssecAlgorithm
) AlgorithmPolicy
```

The source of truth is the live IANA DNSSEC Algorithm Numbers registry as
governed by RFC 9904.

## 14.3 Unsupported algorithms

A DNSSEC wire value can be valid and representable even when the local crypto
backend cannot implement it.

The validator must distinguish:

- unknown/unsupported algorithm capability;
- cryptographically invalid signature;
- IANA policy marking an algorithm `MUST NOT`.

These are not the same failure.

---

# Part VII — Signature validation

# 15. `signature.sec`

## 15.1 Responsibility

`signature.sec` validates RRSIG records over canonical RRsets.

## 15.2 Signature time

DNSSEC RRSIG inception/expiration fields use the DNSSEC 32-bit time model.

Validation must apply the RFC-defined serial/time arithmetic and must handle
wraparound correctly.

It must not compare the 32-bit values as naive permanently increasing Unix
timestamps.

## 15.3 `ValidateSignature`

Canonical semantic API:

```sec
fn ValidateSignature(
    rrset: ref dns.Rrset,
    signature: ref dns.RrsigRecord,
    key: ref dns.DnskeyRecord,
    now: Instant
) Result[void, ValidationFailure]
```

The exact common time type may be updated to the canonical `time` stdlib type
when that book is locked.

Required checks include:

- RRSIG type covered;
- signer name relationship;
- labels/wildcard semantics;
- key tag;
- algorithm match;
- inception;
- expiration;
- DNSKEY protocol/flags semantics where applicable;
- canonical RRset encoding;
- cryptographic signature verification.

The function does not establish a chain of trust by itself.

---

# 16. `delegation.sec`

## 16.1 Responsibility

`delegation.sec` validates DNSSEC delegation transitions.

This includes:

- parent DS RRset;
- child DNSKEY RRset;
- DS digest computation;
- secure delegation;
- proven insecure delegation.

## 16.2 DS matching

Canonical API:

```sec
fn MatchesDs(
    owner: ref dns.Name,
    key: ref dns.DnskeyRecord,
    ds: ref dns.DsRecord
) Result[bool, ValidationFailure]
```

Digest computation uses the DNSKEY owner name and canonical DNSKEY material
defined by DNSSEC.

## 16.3 `DelegationStatus`

```sec
enum DelegationStatus {
    Secure,
    Insecure,
}
```

`Secure` means at least one acceptable DS/DNSKEY chain establishes trust.

`Insecure` must be established through correct authenticated denial or other
standards-defined delegation semantics; absence of a DS packet in an
unvalidated response is not enough.

---

# Part VIII — Authenticated denial

# 17. `denial_nsec.sec`

## 17.1 Responsibility

`denial_nsec.sec` validates authenticated denial using NSEC.

Required cases include:

- NXDOMAIN;
- NODATA;
- wildcard nonexistence;
- wildcard answer proof;
- insecure delegation proof where applicable.

Canonical semantic helpers include:

```sec
fn ValidateNsecNameError(
    question: ref dns.Question,
    proofs: ref dns.Rrset[]
) Result[void, ValidationFailure]

fn ValidateNsecNoData(
    question: ref dns.Question,
    proofs: ref dns.Rrset[]
) Result[void, ValidationFailure]
```

Proof RRsets must themselves already be cryptographically validated before
they establish authenticated denial.

---

# 18. `denial_nsec3.sec`

## 18.1 Responsibility

`denial_nsec3.sec` validates NSEC3 proofs.

Primary references:

- RFC 5155
- RFC 6840
- RFC 9276

## 18.2 Work bounding

NSEC3 hashing is attacker-influenced work.

The validator must enforce:

```sec
ValidationPolicy.MaxNsec3Iterations
```

and other resource limits.

If a proof requires work beyond policy:

```sec
ValidationFailure.Nsec3IterationLimitExceeded
```

is returned.

## 18.3 Opt-Out

NSEC3 Opt-Out must be interpreted according to RFC 5155.

It can prove insecurity for eligible unsigned delegations under the specified
conditions.

The validator must not treat every covered name under an Opt-Out span as a
secure proof of nonexistence.

## 18.4 Canonical helpers

Conceptual API:

```sec
fn ValidateNsec3NameError(
    question: ref dns.Question,
    proofs: ref dns.Rrset[],
    policy: ref ValidationPolicy
) Result[void, ValidationFailure]

fn ValidateNsec3NoData(
    question: ref dns.Question,
    proofs: ref dns.Rrset[],
    policy: ref ValidationPolicy
) Result[void, ValidationFailure]
```

---

# Part IX — Validated cache

# 19. `cache.sec`

## 19.1 Responsibility

`cache.sec` owns DNSSEC-specific validation cache state.

It does not duplicate the ordinary DNS TTL cache.

It may cache:

- validated DNSKEY RRsets;
- validated DS RRsets;
- validated NSEC/NSEC3 proof material;
- derived security status;
- expensive validation results within DNS TTL bounds.

## 19.2 TTL discipline

DNSSEC validation cache entries must not outlive the DNS data/signatures on
which their security conclusion depends.

Effective expiration is bounded by all relevant:

- RRset TTLs;
- RRSIG expiration;
- negative-cache lifetime;
- trust-anchor state.

## 19.3 Aggressive negative caching

RFC 8198 support belongs here.

A validator may synthesize negative answers from cached **validated**
NSEC/NSEC3 data only when the RFC conditions are satisfied.

Unvalidated denial records must never be used for aggressive negative
synthesis.

---

# Part X — Validator

# 20. `validator.sec`

## 20.1 Responsibility

`validator.sec` orchestrates complete DNSSEC validation.

## 20.2 `Validator`

```sec
@noCopy
type Validator struct {
    // Private TrustAnchorStore, ValidationPolicy,
    // validation cache, crypto/backend state.
}
```

Canonical API:

```sec
impl Validator {
    static fn New(
        anchors: <- TrustAnchorStore,
        policy: ValidationPolicy
    ) Result[Validator, DnssecError]

    fn Validate(
        question: ref dns.Question,
        response: ref dns.Message,
        transport: ref dns.QueryTransport
    ) Result[ValidationResult, DnssecError]

    fn ClearCache()

    fn Close() Result[void, DnssecError]
}
```

The exact parent `dns.QueryTransport` syntax remains subject to the transport
correction already required by the DoH/DoT/DoQ books.

## 20.3 Validation fetches

The validator may need additional DNS queries for:

- DS;
- DNSKEY;
- NSEC;
- NSEC3;
- missing signature/proof material.

Those queries use the supplied underlying `dns.QueryTransport`.

This avoids binding DNSSEC to one transport.

## 20.4 CD flag

When the validator performs local DNSSEC validation through an upstream
recursive resolver, it must use DNS protocol signaling appropriate to local
validation, including CD behavior where required by RFC 4035/6840.

The exact base `dns.MessageFlags` builder helpers should be updated so the
validator can request the required data without magic bit operations.

## 20.5 DO flag

Queries requiring DNSSEC records use EDNS DO appropriately.

The validator does not infer local validation from an upstream AD bit.

## 20.6 AD bit

An upstream AD bit may be recorded as transport/resolver metadata, but local:

```sec
SecurityStatus.Secure
```

requires the local validator's own trust-chain result.

## 20.7 Validation process

Conceptually:

1. identify the answer/proof RRsets relevant to the question;
2. determine the applicable trust anchor;
3. obtain/validate DS delegation chain;
4. obtain/validate DNSKEY RRsets;
5. validate relevant RRSIGs;
6. validate wildcard semantics;
7. validate authenticated denial where required;
8. produce `Secure`, `Insecure`, `Bogus`, or `Indeterminate`;
9. cache only conclusions backed by valid bounded-lifetime material.

## 20.8 Fail closed for Bogus

The normal validating resolver integration must not return Bogus data as a
successful authenticated answer.

Applications may still inspect raw DNS messages through lower-level DNS
clients.

---

# Part XI — Resolver integration

# 21. `integration.sec`

## 21.1 Required parent DNS extension

The common resolver must support local validation without importing this child
package.

The recommended architecture is a parent-owned validator interface.

Semantic shape:

```sec
interface ResponseValidator {
    fn Validate(
        question: ref Question,
        response: ref Message,
        transport: ref QueryTransport
    ) Result[DnsSecurityStatus, error]
}
```

The exact parent status name must be reconciled with this book's
`SecurityStatus`.

One acceptable parent design is to move the shared four-state enum to
`net/dns`:

```sec
enum SecurityStatus {
    Secure,
    Insecure,
    Bogus,
    Indeterminate,
}
```

and have this child use `dns.SecurityStatus`.

That move is preferred if resolver results expose DNSSEC security status.

## 21.2 No duplicate resolver

Revision 0.1 does not introduce:

```sec
dnssec.Resolver
```

The existing common resolver should optionally apply a validator.

This avoids duplicating:

- search behavior;
- A/AAAA lookup;
- CNAME following;
- caching;
- transport selection;
- reverse lookup.

## 21.3 Resolver result metadata

If the common resolver exposes validation status, it should do so through a
typed result rather than a boolean such as:

```text
isSecure
```

because `Insecure`, `Bogus`, and `Indeterminate` are semantically distinct.

The exact parent resolver result wrapper remains a required follow-up
correction.

---

# Part XII — Trust and security

# 22. Trust-anchor bootstrap

DNSSEC cannot create trust from an arbitrary received DNSKEY.

At least one trust anchor must be configured/established through a trusted
mechanism.

`TrustAnchorStore.Empty()` therefore enables validation only for names beneath
anchors later explicitly added.

It must not silently trust the first DNSKEY observed.

---

# 23. Algorithm agility

Wire algorithm values are open.

Local implementation/use policy comes from IANA under RFC 9904.

The validator must:

- reject algorithms marked MUST NOT for validation;
- support algorithms required by current IANA implementation policy;
- distinguish unsupported capability from invalid signature;
- preserve unknown wire values;
- be updateable without redesigning `dns.DnssecAlgorithm`.

---

# 24. Time

RRSIG validation depends on trustworthy time.

A target without a sufficiently reliable clock cannot claim normal DNSSEC
validation for signature time windows.

It may return:

```sec
SecurityStatus.Indeterminate
ValidationFailure.ClockUnavailable
```

according to caller policy.

The package must not silently skip signature-time checks on clockless targets.

---

# 25. Resource-exhaustion resistance

Validation is performed on attacker-controlled DNS data.

The implementation must bound:

- delegation-chain depth;
- CNAME/DNAME/wildcard processing as inherited from DNS;
- NSEC3 iterations;
- number of candidate signatures/keys attempted;
- cryptographic work;
- fetched dependency queries;
- cached proof material.

---

# 26. Crypto side channels

Cryptographic primitive constant-time requirements belong to the canonical
crypto package.

DNSSEC must use appropriate safe crypto APIs and must not reimplement
non-constant-time private or verification primitives casually in DNS code.

---

# Part XIII — Signing boundary

# 27. Future RRset signing

Future DNSSEC signing primitives may include semantic operations such as:

```sec
SignRrset(...)
CreateDs(...)
BuildNsecChain(...)
BuildNsec3Chain(...)
```

They are not locked in revision 0.1 because canonical private-key and signing
key types are not yet fixed.

When introduced:

- RRset signing lives in `net/dns/dnssec`;
- full-zone orchestration lives in `net/dns/zone`;
- crypto key operations live in `crypto`;
- the DNSSEC package must not import the zone package.

## 27.1 NSEC3 generation guidance

Any future NSEC3 signing implementation follows RFC 9276 current guidance.

It must not encourage high iteration counts or unnecessary salt merely because
older configurations historically used them.

---

# Part XIV — Repository implementation

# 28. Required source structure

Create:

```text
sec/stdlib/net/dns/dnssec/
├── std-net-dns-dnssec.md
├── error.sec
├── status.sec
├── policy.sec
├── canonical.sec
├── keytag.sec
├── trust_anchor.sec
├── anchor_update.sec
├── algorithm.sec
├── signature.sec
├── delegation.sec
├── denial_nsec.sec
├── denial_nsec3.sec
├── cache.sec
├── validator.sec
└── integration.sec
```

Existing newer files must be inspected and reconciled rather than blindly
overwritten.

---

# 29. Required parent corrections

Before DNSSEC integration is complete, update `std-net-dns.md` to:

1. define shared `dns.Rrset`;
2. expose the resolver/query transport abstraction required by validators;
3. expose or accommodate the four-state DNSSEC security result;
4. allow local validators to request DO/CD behavior without raw flag hacking;
5. ensure resolver caching can carry validated-security metadata;
6. preserve the distinction between upstream AD signaling and local
   validation.

---

# Part XV — Tests

# 30. Canonical-form tests

At minimum:

- canonical DNS name ordering;
- ASCII case folding only;
- canonical owner encoding;
- canonical embedded DNS names;
- no compression;
- wildcard owner treatment;
- canonical RR ordering;
- duplicate RRset member rejection;
- original TTL handling.

Use RFC test vectors where available.

---

# 31. Key/DS tests

At minimum:

- DNSKEY key tag;
- DS digest match;
- DS digest mismatch;
- unsupported digest;
- multiple DS records;
- multiple DNSKEY algorithms;
- key-tag collision does not bypass cryptographic verification.

---

# 32. RRSIG tests

At minimum:

- valid signature;
- invalid signature;
- wrong key;
- wrong algorithm;
- wrong type covered;
- not-yet-valid signature;
- expired signature;
- wraparound time cases;
- wildcard signature labels;
- multiple signatures where one valid supported signature is sufficient under
  the governing rules.

---

# 33. Delegation tests

At minimum:

- secure parent->child DS/DNSKEY chain;
- unsigned/insecure delegation;
- missing DS without authenticated denial not treated as Insecure;
- bogus DS;
- child DNSKEY mismatch;
- chain loop;
- chain depth limit.

---

# 34. NSEC tests

At minimum:

- secure NXDOMAIN;
- secure NODATA;
- wildcard proof;
- closest-encloser behavior;
- invalid NSEC interval;
- unvalidated NSEC cannot establish denial;
- aggressive validated negative-cache synthesis.

---

# 35. NSEC3 tests

At minimum:

- valid NSEC3 NXDOMAIN;
- valid NSEC3 NODATA;
- wildcard proof;
- Opt-Out insecure delegation;
- invalid Opt-Out interpretation rejected;
- iteration limit;
- malformed salt/hash;
- high-cost hostile proof bounded;
- RFC 9276 current operational constraints.

---

# 36. Trust-anchor tests

At minimum:

- manual DNSKEY anchor;
- manual DS anchor;
- duplicate anchor;
- remove anchor;
- no trust anchor -> correct status/error;
- persistent store failure;
- RFC 5011 AddPending;
- RFC 5011 hold-down;
- RFC 5011 revoke;
- crash/restart persistence does not bypass hold-down.

---

# 37. Algorithm-policy tests

At minimum:

- current IANA required validation algorithms reflected;
- current IANA MUST NOT algorithms rejected;
- unknown algorithm remains representable;
- unsupported local crypto capability distinguished from invalid signature;
- registry synchronization test data carries date/source.

---

# 38. Resolver integration tests

At minimum:

- Secure A/AAAA result;
- Insecure unsigned delegation;
- Bogus signed result rejected by validating resolver;
- Indeterminate transport/dependency failure;
- local validation ignores untrusted AD as proof;
- DO/CD queries generated correctly;
- classic DNS transport;
- DoT transport;
- DoH transport;
- DoQ transport;
- same resolver engine used for all transports;
- cancellation during validation fetch does not cache partial security result.

---

# Part XVI — AI/repository implementation instructions

# 39. Requirements

An AI/repository automation implementing this book must:

1. treat this book as the normative DNSSEC validation specification;
2. reuse all DNSSEC wire RR types from `net/dns`;
3. add shared `dns.Rrset` to the parent DNS book/API rather than creating
   `dnssec.Rrset`;
4. use RFC 4033/4034/4035 plus RFC 6840 current clarifications;
5. implement NSEC and NSEC3 denial correctly;
6. apply RFC 9276 current NSEC3 operational guidance;
7. implement RFC 8198 aggressive negative cache only from validated proof
   material;
8. implement trust anchors;
9. implement RFC 5011 state persistently before claiming automated anchor
   updates;
10. follow RFC 9904 and current IANA algorithm registries;
11. not treat obsolete RFC 8624 as the canonical live algorithm-policy table;
12. preserve unknown algorithm values;
13. distinguish Secure/Insecure/Bogus/Indeterminate;
14. not treat AD bit as local validation;
15. not trust first-seen DNSKEY material automatically;
16. bound validation work;
17. reuse canonical crypto algorithms;
18. not implement private crypto primitives in DNSSEC files;
19. integrate through the common DNS resolver rather than create
    `dnssec.Resolver`;
20. keep future zone signing free of a dnssec->zone dependency;
21. add standards URLs to source files;
22. add required tests;
23. update `implementation-status-std-net-dns-dnssec.yaml`;
24. reconcile canonical implementation status without overwriting stronger
    evidence.

---

# 40. Recommended implementation order

1. parent `dns.Rrset`;
2. errors/status/policy;
3. canonical DNSSEC encoding;
4. key-tag;
5. trust-anchor store;
6. live algorithm policy mapping;
7. DS matching;
8. RRSIG verification;
9. secure delegation chain;
10. NSEC proofs;
11. NSEC3 proofs;
12. validation cache;
13. complete `Validator`;
14. parent resolver integration;
15. RFC 8198 aggressive cache;
16. RFC 5011 automated anchor updates;
17. future signing primitives only after crypto/private-key API is locked.

---

# Part XVII — Open observations

# 41. Open items

- final canonical `time` type used for RRSIG validation;
- final canonical crypto key/hash/signature APIs;
- whether `SecurityStatus` moves to parent `net/dns`;
- exact resolver result wrapper carrying validation metadata;
- exact `dns.QueryTransport` interface after parent corrections;
- system trust-anchor provisioning per platform;
- root trust-anchor distribution policy;
- RFC 5011 persistence backend abstraction;
- DANE integration;
- aggressive cache default enablement;
- validation work budgets;
- future zone signing API;
- CDS/CDNSKEY automation ownership;
- RFC 9615 parental-agent functionality;
- multi-signer authoritative tooling.

These observations do not permit incompatible public APIs.

---

# 42. Decisions established by revision 0.1

Revision 0.1 establishes:

- canonical package path `net/dns/dnssec`;
- base `net/dns` owns DNSSEC wire RRs;
- this child owns local DNSSEC validation semantics;
- shared `dns.Rrset` must be added to the parent package;
- validation status is Secure/Insecure/Bogus/Indeterminate;
- RFC 4033/4034/4035 plus RFC 6840 are the core validation baseline;
- NSEC3 follows RFC 5155 and current RFC 9276 guidance;
- validated negative cache may implement RFC 8198;
- trust anchors are explicit typed values;
- RFC 5011 automated update requires persistent state;
- algorithm policy comes from live IANA registries under RFC 9904;
- unknown algorithm values remain representable;
- AD is not equivalent to local validation;
- DNSSEC is transport-independent;
- validation may fetch dependencies using the common DNS transport interface;
- DNSSEC does not duplicate the resolver;
- no `dnssec.Resolver` is introduced;
- DNSSEC does not implement its own cryptographic primitives;
- full zone signing is deferred, but future RRset signing belongs to DNSSEC
  while full-zone orchestration belongs to `net/dns/zone`;
- DNSSEC must not import `net/dns/zone`;
- validation work is bounded against resource-exhaustion attacks.

---

**Document revision:** 0.1  
**Last updated:** 2026-09-07
