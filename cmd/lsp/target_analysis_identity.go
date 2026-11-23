package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"sec/internal/sema"
)

// targetAnalysisIdentity binds prepared target-analysis inputs to the exact
// observable metadata configured on its analyzer. Raw manifest text alone
// cannot represent explicit metadata inputs such as source revision or an
// invocation build number. Unavailable metadata is a distinct state.
// Rules: rules/compiler/compiler_known_members.md — §113B(4);
// rules/projects/projects.md — §§33–34;
// rules/compiler/incremental_compilation.md — §§4–8.
func targetAnalysisIdentity(analyzer *sema.Analyzer, inputs []any) (string, error) {
	metadata := "unavailable"
	if snapshot, available := analyzer.BuildInfo(); available {
		metadata = snapshot.CacheIdentity()
	}
	hash := sha256.New()
	if err := json.NewEncoder(hash).Encode([]any{"target-analysis-v2", inputs, metadata}); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
