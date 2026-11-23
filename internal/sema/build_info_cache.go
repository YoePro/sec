package sema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// CacheIdentity fingerprints every observable build-metadata value, including
// optional presence. It is independent of pointer addresses and manifest text;
// the versioned encoding is cache infrastructure, never a Sec identity or ABI.
// Rules: rules/compiler/compiler_known_members.md — §113B(4);
// rules/projects/projects.md — §§33–34;
// rules/compiler/incremental_compilation.md — §§5,7–8.
func (snapshot BuildInfoSnapshot) CacheIdentity() string {
	// This closed struct contains only strings, uint64s and optional scalar
	// pointers; JSON encoding cannot fail and preserves their exact values.
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		panic("BuildInfo snapshot encoding failed: " + err.Error())
	}
	sum := sha256.Sum256(append([]byte("sec-build-info-v1\n"), encoded...))
	return hex.EncodeToString(sum[:])
}
