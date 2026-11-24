package layout

import (
	"errors"
	"strings"
	"testing"
)

// TestAtomicContextProofMatrix validates every declared mechanism against both
// context requirements and all known/unknown proof combinations. Neither a
// native mechanism nor a lock-free claim bypasses the required context proof.
// Rules: rules/concurrency/atomics.md — §§38(3–7),39(1–5).
func TestAtomicContextProofMatrix(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, kind := range []AtomicImplementationKind{AtomicNativeInstruction, AtomicCompilerIntrinsic, AtomicPlatformPrimitive, AtomicEmulated} {
			for _, noBlocking := range []bool{false, true} {
				for _, interruptSafe := range []bool{false, true} {
					for _, blocking := range []AtomicProperty{AtomicPropertyUnknown, AtomicPropertyNo, AtomicPropertyYes} {
						for _, safety := range []AtomicProperty{AtomicPropertyUnknown, AtomicPropertyNo, AtomicPropertyYes} {
							request := atomicTestRequirement()
							request.Key.NoBlocking, request.Key.InterruptSafe = noBlocking, interruptSafe
							implementation := AtomicImplementation{Kind: kind, Identity: "test-platform-implementation", LockFree: AtomicPropertyYes, Blocking: blocking, InterruptSafe: safety}
							fact := AtomicCapability{Key: request.Key, Availability: AtomicSupported, Provenance: "test-proof", Implementation: implementation}
							base := atomicTestPlan()
							base.PointerWidthBits = width
							plan, err := base.WithAtomicCapabilities([]AtomicCapability{fact})
							wantNonblocking := (noBlocking || interruptSafe) && blocking != AtomicPropertyNo
							wantSafety := interruptSafe && safety != AtomicPropertyYes
							if (err != nil) != (wantNonblocking || wantSafety) {
								t.Fatalf("%d %+v %+v: %v", width, request, implementation, err)
							}
							if err == nil {
								if got, err := plan.AtomicCapability(request); err != nil || got != fact {
									t.Fatal(got, err)
								}
								continue
							}
							var violation AtomicContextViolation
							if !errors.As(err, &violation) || violation.Key != request.Key || violation.Implementation != implementation {
								t.Fatal("lost exact context evidence", err)
							}
							for _, text := range []string{"test-os/test-arch", "hosted", request.Key.Operation, string(request.Key.Context), implementation.Identity} {
								if !strings.Contains(err.Error(), text) {
									t.Fatal("missing context in diagnostic", text, err)
								}
							}
							if strings.Contains(err.Error(), "nonblocking") != wantNonblocking || strings.Contains(err.Error(), "interrupt") != wantSafety {
								t.Fatal("failed guarantees lost", err)
							}
							if (wantNonblocking && blocking == AtomicPropertyUnknown || wantSafety && safety == AtomicPropertyUnknown) && !strings.Contains(err.Error(), "no proof") {
								t.Fatal("unknown proof treated as known denial", err)
							}
							// A failed replacement must not change the original plan.
							if got, err := base.AtomicCapability(request); err != nil || got.Availability != AtomicUnknown {
								t.Fatal(got, err)
							}
						}
					}
				}
			}
		}
	}
}

// TestAtomicContextChecksEveryOperation applies the same context proof to all
// atomic operations, including a fence without a fabricated storage object.
// Rules: rules/concurrency/atomics.md — §§26,38–40.
func TestAtomicContextChecksEveryOperation(t *testing.T) {
	for _, operation := range []string{"Load", "Store", "Swap", "CompareExchange", "FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor", "Fence"} {
		request := atomicTestRequirement()
		request.Key.Operation, request.Key.Order, request.Key.NoBlocking = operation, "SeqCst", true
		if operation != "CompareExchange" {
			request.Key.FailureOrder = ""
		}
		if operation == "Fence" {
			request.SemanticTypeID, request.Key.CarrierTypeID, request.Key.Representation, request.Key.SizeBytes, request.Key.AlignmentBytes = "", "", StorageRepresentation{}, 0, 0
		}
		fact := AtomicCapability{Key: request.Key, Availability: AtomicSupported, Provenance: "test-proof", Implementation: AtomicImplementation{Kind: AtomicEmulated, Identity: "test-blocking-fallback", Blocking: AtomicPropertyYes}}
		if _, err := atomicTestPlan().WithAtomicCapabilities([]AtomicCapability{fact}); err == nil {
			t.Fatal("blocking fallback accepted", operation)
		}
	}
}

// TestAtomicContextDoesNotInventAvailability preserves missing/negative target
// evidence in restricted contexts (atomics.md § 37A(11)).
// Rules: rules/concurrency/atomics.md — §§37–39.
func TestAtomicContextDoesNotInventAvailability(t *testing.T) {
	request := atomicTestRequirement()
	request.Key.NoBlocking, request.Key.InterruptSafe = true, true
	for _, availability := range []AtomicAvailability{AtomicUnknown, AtomicUnsupported} {
		fact := AtomicCapability{Key: request.Key, Availability: availability, Provenance: "test-target"}
		plan, err := atomicTestPlan().WithAtomicCapabilities([]AtomicCapability{fact})
		if err != nil {
			t.Fatal(err)
		}
		if got, err := plan.AtomicCapability(request); err != nil || got != fact {
			t.Fatal(got, err)
		}
	}
	if got, err := atomicTestPlan().AtomicCapability(request); err != nil || got.Availability != AtomicUnknown || got.Implementation != (AtomicImplementation{}) {
		t.Fatal(got, err)
	}
}
