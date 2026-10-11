package sema

import (
	"fmt"
	"sort"
)

// Canonical compiler-owned identities for Places, references and their
// validity dependencies. Every identity is a small integer allocated per
// analysis in first-use order from a stable semantic key, so equal programs
// receive equal identities. Zero is the absent/unknown identity of every kind.
// None of them is a numeric address, a source name or an epoch value.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 21-23 (storage, invalidation domain, epoch dependency), § 29 (address space), § 30 (borrow/lifetime)
//   - rules/memory/storage.md — § 3.6 "Storage domain", § 16 "MemorySpaceKind"
//   - rules/compiler/semantic_ir.md — §§ 10(9)-10(10), 45, 46(6), 108(5)

// StorageIdentityID identifies one storage domain (storage.md § 3.6): one
// storage lifetime with its governing properties, independent of address.
type StorageIdentityID uint32

// InvalidationDomainID identifies a domain whose live incarnation a reference
// may depend on (an Arena allocation epoch domain, an object incarnation,
// collection backing storage, ...). A generation belongs to the domain, not to
// an address.
type InvalidationDomainID uint32

// EpochDependencyID identifies one dependency on an invalidation domain's
// expected live incarnation. It does not require runtime epoch bits.
type EpochDependencyID uint32

// AddressSpaceID identifies one storage.md § 16 memory space. The canonical
// kinds have fixed identities; target-defined spaces are allocated after them.
type AddressSpaceID uint32

const (
	AddressSpaceNone          AddressSpaceID = 0
	AddressSpaceOrdinary      AddressSpaceID = 1
	AddressSpaceMMIO          AddressSpaceID = 2
	firstTargetDefinedAddress AddressSpaceID = 3
)

// DynamicIndexID identifies one evaluated runtime index expression of a Place
// projection (package15.md § 14). Two different identities are unrelated
// runtime indexes; an identity never compares expression text.
type DynamicIndexID uint32

func (id DynamicIndexID) String() string { return identityString("index", uint32(id)) }

// BorrowID identifies one direct borrow creation; copies, moves and reborrows
// retain or derive borrow relationships from it.
type BorrowID uint32

// LifetimeID identifies the resolved lifetime of a borrow.
type LifetimeID uint32

func (id StorageIdentityID) String() string    { return identityString("storage", uint32(id)) }
func (id InvalidationDomainID) String() string { return identityString("domain", uint32(id)) }
func (id EpochDependencyID) String() string    { return identityString("epoch-dep", uint32(id)) }
func (id BorrowID) String() string             { return identityString("borrow", uint32(id)) }
func (id LifetimeID) String() string           { return identityString("lifetime", uint32(id)) }

func (id AddressSpaceID) String() string {
	switch id {
	case AddressSpaceNone:
		return "space:none"
	case AddressSpaceOrdinary:
		return "space:ordinary"
	case AddressSpaceMMIO:
		return "space:mmio"
	default:
		return fmt.Sprintf("space:target#%d", uint32(id-firstTargetDefinedAddress)+1)
	}
}

func identityString(kind string, id uint32) string {
	if id == 0 {
		return kind + ":none"
	}
	return fmt.Sprintf("%s#%d", kind, id)
}

// placeIdentityAllocator owns the per-analysis identity tables. Keys are
// stable semantic keys (declaration locations, Arena domain identities,
// borrow creation sites), never source spellings alone.
type placeIdentityAllocator struct {
	storage        map[string]StorageIdentityID
	domains        map[string]InvalidationDomainID
	epochs         map[string]EpochDependencyID
	targetSpaces   map[string]AddressSpaceID
	dynamicIndexes map[string]DynamicIndexID
	borrows        map[string]BorrowID
	lifetimes      map[string]LifetimeID
}

func allocateIdentity[T ~uint32](table *map[string]T, key string, offset T) T {
	if key == "" {
		return 0
	}
	if *table == nil {
		*table = map[string]T{}
	}
	if id, ok := (*table)[key]; ok {
		return id
	}
	id := T(len(*table)) + offset
	(*table)[key] = id
	return id
}

func (a *placeIdentityAllocator) storageIdentity(key string) StorageIdentityID {
	return allocateIdentity(&a.storage, key, 1)
}

func (a *placeIdentityAllocator) invalidationDomain(key string) InvalidationDomainID {
	return allocateIdentity(&a.domains, key, 1)
}

func (a *placeIdentityAllocator) epochDependency(key string) EpochDependencyID {
	return allocateIdentity(&a.epochs, key, 1)
}

func (a *placeIdentityAllocator) targetAddressSpace(name string) AddressSpaceID {
	return allocateIdentity(&a.targetSpaces, name, firstTargetDefinedAddress)
}

func (a *placeIdentityAllocator) dynamicIndex(key string) DynamicIndexID {
	return allocateIdentity(&a.dynamicIndexes, key, 1)
}

func (a *placeIdentityAllocator) borrow(key string) BorrowID {
	return allocateIdentity(&a.borrows, key, 1)
}

func (a *placeIdentityAllocator) lifetime(key string) LifetimeID {
	return allocateIdentity(&a.lifetimes, key, 1)
}

// clone detaches every table so an analyzer snapshot cannot be mutated
// through a later allocation.
func (a placeIdentityAllocator) clone() placeIdentityAllocator {
	return placeIdentityAllocator{
		storage:        cloneIdentityTable(a.storage),
		domains:        cloneIdentityTable(a.domains),
		epochs:         cloneIdentityTable(a.epochs),
		targetSpaces:   cloneIdentityTable(a.targetSpaces),
		dynamicIndexes: cloneIdentityTable(a.dynamicIndexes),
		borrows:        cloneIdentityTable(a.borrows),
		lifetimes:      cloneIdentityTable(a.lifetimes),
	}
}

func cloneIdentityTable[T ~uint32](table map[string]T) map[string]T {
	if table == nil {
		return nil
	}
	out := make(map[string]T, len(table))
	for key, id := range table {
		out[key] = id
	}
	return out
}

// identityTableKeys returns the keys of a table in identity order, for
// deterministic printing and tests.
func identityTableKeys[T ~uint32](table map[string]T) []string {
	keys := make([]string, 0, len(table))
	for key := range table {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return table[keys[i]] < table[keys[j]] })
	return keys
}

// rootStorageIdentity assigns the storage-domain identity of a root binding:
// an Arena-backed reference root shares its Arena domain's storage, another
// reference root has an unknown referent storage, and every other root
// binding owns the storage declared at its declaration.
func (a *Analyzer) rootStorageIdentity(symbol Symbol) StorageIdentityID {
	if typeCarriesReferenceOrigin(symbol.Type) {
		// A reference or view root denotes its referent, whose storage domain
		// is known here only for Arena-backed storage; anything else stays
		// unknown rather than borrowing the holder binding's own storage.
		if symbol.Type.ReferenceOriginStorage == StorageOriginArena && symbol.Type.ReferenceOriginName != "" {
			return a.placeIdentities.storageIdentity("arena:" + symbol.Type.ReferenceOriginName)
		}
		return 0
	}
	if symbol.Token.Line == 0 {
		return 0
	}
	location := sourceTokenLocation(symbol.Token)
	return a.placeIdentities.storageIdentity(fmt.Sprintf("binding:%s:%d:%d", location.File, location.Line, location.Column))
}

// rootAddressSpace classifies the memory space of a root binding: register
// fields are memory-mapped hardware storage, every other binding is ordinary.
func rootAddressSpace(symbol Symbol) AddressSpaceID {
	if symbol.RegisterAccess != "" {
		return AddressSpaceMMIO
	}
	return AddressSpaceOrdinary
}
