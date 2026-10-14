package debuginfo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// SourceContentIdentity fingerprints exact compiled bytes, including comments,
// BOMs and line endings. SHA-256 is this toolchain's internal metadata policy,
// not a Sec language rule. Unknown metadata is distinct from an empty file.
// Rules: rules/compiler/debug_information.md — §55(1)-(6);
// rules/compiler/incremental_compilation.md — §59(1).
type SourceContentIdentity struct {
	digest [sha256.Size]byte
	known  bool
}

// SourceContent associates a content identity with the source locator carried
// by lexer token provenance. File is not a canonical semantic file identity;
// changing/remapping it does not alter the compiled-content fingerprint.
// Rules: rules/compiler/debug_information.md — §§53(2),54(2),55(1).
type SourceContent struct {
	File     string
	Identity SourceContentIdentity
}

// IdentifySourceContent fingerprints an in-memory compilation input without
// reading a filesystem path, retaining source text or normalizing its bytes.
// Rules: rules/compiler/debug_information.md — §55(1)-(2),(5).
func IdentifySourceContent(source []byte) SourceContentIdentity {
	return SourceContentIdentity{digest: sha256.Sum256(source), known: true}
}

// Matches checks debugger-presented bytes against compiled bytes. Missing
// metadata is an error rather than proof of matching source. The caller owns
// debugger warning/breakpoint policy after a mismatch.
// Rules: rules/compiler/debug_information.md — §55(1),(3)-(4).
func (identity SourceContentIdentity) Matches(source []byte) (bool, error) {
	if !identity.known {
		return false, fmt.Errorf("compiled source-content identity is unavailable")
	}
	return identity.digest == sha256.Sum256(source), nil
}

// MarshalText encodes an algorithm-tagged internal identity without embedding
// source text; unknown metadata cannot be serialized as a valid fingerprint.
// Rules: rules/compiler/debug_information.md — §55(2),(5).
func (identity SourceContentIdentity) MarshalText() ([]byte, error) {
	if !identity.known {
		return nil, fmt.Errorf("compiled source-content identity is unavailable")
	}
	return []byte("sha256:" + hex.EncodeToString(identity.digest[:])), nil
}

// UnmarshalText validates the internal digest algorithm and length before
// replacing existing metadata. Unknown algorithms never masquerade as matches.
// Rules: rules/compiler/debug_information.md — §55(1)-(3).
func (identity *SourceContentIdentity) UnmarshalText(text []byte) error {
	name := string(text)
	if !strings.HasPrefix(name, "sha256:") || len(name) != len("sha256:")+sha256.Size*2 {
		return fmt.Errorf("invalid source-content identity: expected sha256 and a 256-bit digest")
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(name, "sha256:"))
	if err != nil {
		return fmt.Errorf("invalid source-content digest: %w", err)
	}
	var parsed SourceContentIdentity
	copy(parsed.digest[:], decoded)
	parsed.known = true
	*identity = parsed
	return nil
}
