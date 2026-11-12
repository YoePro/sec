package layout

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// PlanFingerprints are the domain-specific semantic fingerprints of one
// resolved plan and their composite (platform_model.md § 23). Each is
// computed from canonical resolved meaning only, never from display text,
// registry order or the provenance that produced the value, and each covers
// every fact its domain's results can depend on.
type PlanFingerprints struct {
	Architecture string
	Layout       string
	Memory       string
	Runtime      string
	Execution    string
	ABI          string
	Link         string
	Plan         string
}

// Fingerprints computes the plan's domain fingerprints.
// Rules: rules/platform/platform_model.md — §§ 20(13), 23, 27.
func (p ResolvedScalarPlan) Fingerprints() PlanFingerprints {
	digest := func(domain string, facts ...any) string {
		encoded, _ := json.Marshal(append([]any{"sec.plan-fingerprint/1", domain}, facts...))
		sum := sha256.Sum256(encoded)
		return hex.EncodeToString(sum[:])
	}
	fingerprints := PlanFingerprints{
		Architecture: digest("architecture", p.TargetArch, p.PointerWidthBits, p.Endianness, p.Atomics),
		Layout:       digest("layout", p.PointerWidthBits, p.Endianness, p.CABI, p.References),
		Memory:       digest("memory", p.Profile, p.AddressDomains, p.References, p.PointerWidthBits),
		Runtime:      digest("runtime", p.TargetOS, p.Profile),
		Execution:    digest("execution", p.TargetOS, p.TargetArch, p.Profile),
		ABI:          digest("abi", p.TargetOS, p.TargetArch, p.ABI, p.CABI),
		Link:         digest("link", p.TargetOS, p.TargetArch, p.LLVMTriple, p.ABI),
	}
	fingerprints.Plan = digest("plan", fingerprints.Architecture, fingerprints.Layout, fingerprints.Memory,
		fingerprints.Runtime, fingerprints.Execution, fingerprints.ABI, fingerprints.Link)
	return fingerprints
}
