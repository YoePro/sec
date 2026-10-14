package main

import (
	"crypto/sha256"
	"encoding/hex"
)

// sourceContentFingerprint binds deferred navigation/presentation to immutable
// source content, independently of timestamps and open/closed editor state.
// Rules: rules/tooling/lsp.md — "Document snapshots", "Workspace symbols",
// "Interface conformance code lens".
func sourceContentFingerprint(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}
