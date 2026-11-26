package semantic

import (
	"fmt"
	"io"
	"strings"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
)

// AtomicCapabilityEvidence is one MD-095 capability requirement of an atomic
// operation under one execution context, as Sema resolved it: the semantic
// type, carrier, representation, access extent and guaranteed alignment
// (empty for a fence or an outstanding layout), operation and orders, the
// canonical context with its additional requirements, and either supported
// evidence (implementation identity and kind) or an open obligation with
// its missing fact. A proven Unsupported request is a Sema error and never
// reaches Semantic IR.
// Rules: rules/compiler/semantic_ir.md — § 123B; rules/concurrency/atomics.md — §§ 37A(9)-(13), 46(7).
type AtomicCapabilityEvidence struct {
	SemanticType       string `json:"semantic_type,omitempty"`
	Carrier            string `json:"carrier,omitempty"`
	Representation     string `json:"representation,omitempty"`
	SizeBytes          uint64 `json:"size_bytes,omitempty"`
	AlignBytes         uint64 `json:"align_bytes,omitempty"`
	Operation          string `json:"operation"`
	Order              string `json:"order"`
	FailureOrder       string `json:"failure_order,omitempty"`
	Context            string `json:"context,omitempty"`
	NoBlocking         bool   `json:"no_blocking,omitempty"`
	InterruptSafe      bool   `json:"interrupt_safe,omitempty"`
	Availability       string `json:"availability"`
	Implementation     string `json:"implementation,omitempty"`
	ImplementationKind string `json:"implementation_kind,omitempty"`
	Missing            string `json:"missing,omitempty"`
	Provenance         string `json:"provenance"`
}

// atomicCallSite is the callee token Sema keys atomic facts by.
// Rules: rules/compiler/semantic_ir.md — § 123B(2).
func atomicCallSite(call *ast.CallExpression) lexer.Token {
	if member, ok := call.Callee.(*ast.MemberExpression); ok && member.Property != nil {
		return member.Property.Token
	}
	if identifier, ok := call.Callee.(*ast.Identifier); ok {
		return identifier.Token
	}
	return call.Token
}

// atomicEvidence copies Sema's capability results for one source operation
// into Semantic IR. Construction never invents evidence: an operation Sema
// published no result for fails closed.
// Rules: rules/compiler/semantic_ir.md — § 123B(2)-(3).
func (fb *functionBuilder) atomicEvidence(source lexer.Token) ([]AtomicCapabilityEvidence, error) {
	var evidence []AtomicCapabilityEvidence
	for _, result := range fb.owner.analyzer.AtomicCapabilities() {
		if result.Source.File != source.File || result.Source.Line != source.Line || result.Source.Column != source.Column {
			continue
		}
		key := result.Requirement.Key
		item := AtomicCapabilityEvidence{
			SemanticType: result.Requirement.SemanticTypeID, Carrier: key.CarrierTypeID, SizeBytes: key.SizeBytes, AlignBytes: key.AlignmentBytes,
			Operation: key.Operation, Order: key.Order, FailureOrder: key.FailureOrder, Context: string(key.Context),
			NoBlocking: key.NoBlocking, InterruptSafe: key.InterruptSafe, Availability: string(result.Availability),
			Missing: result.Missing, Provenance: result.Requirement.Provenance,
		}
		if key.Representation != (layout.StorageRepresentation{}) {
			item.Representation = key.Representation.String()
		}
		if result.Availability == layout.AtomicSupported {
			item.Implementation, item.ImplementationKind = result.Capability.Implementation.Identity, string(result.Capability.Implementation.Kind)
		}
		evidence = append(evidence, item)
	}
	if len(evidence) == 0 {
		return nil, fb.unsupported("atomic operation without resolved capability evidence (MD-095)", source)
	}
	return evidence, nil
}

// verifyAtomicEvidence enforces semantic_ir.md § 123B(2)-(3): every atomic
// object operation and fence carries at least one entry; no other operation
// carries one; each entry matches the op's operation and orders, names a
// canonical context unless reachability itself is open, is supported with
// an implementation or unknown with its missing fact, never unsupported; a
// fence has no carrier or layout; an object entry without carrier, layout or
// context stays an open obligation.
// Rules: rules/compiler/semantic_ir.md — § 123B; rules/concurrency/atomics.md — §§ 37A(7)-(12).
func verifyAtomicEvidence(op Operation) error {
	atomic := op.Kind == OpAtomicFence || isAtomicObjectOperation(op.Kind)
	if !atomic {
		if len(op.AtomicEvidence) != 0 {
			return fmt.Errorf("%s carries atomic capability evidence", op.Kind)
		}
		return nil
	}
	if len(op.AtomicEvidence) == 0 {
		return fmt.Errorf("%s requires capability evidence or an open obligation (MD-095)", op.Kind)
	}
	operation, order, failure := "Fence", op.AtomicOrder, ""
	if op.Kind != OpAtomicFence && op.Atomic != nil {
		operation, order, failure = op.Atomic.Operation, op.Atomic.Order, op.Atomic.FailureOrder
	}
	for _, e := range op.AtomicEvidence {
		if e.Operation != operation || e.Order != order || e.FailureOrder != failure || e.Provenance == "" {
			return fmt.Errorf("%s capability evidence does not match its operation and orders", op.Kind)
		}
		canonical := false
		switch layout.AtomicContext(e.Context) {
		case layout.AtomicContextOrdinary, layout.AtomicContextTask, layout.AtomicContextThread, layout.AtomicContextInterrupt, layout.AtomicContextStartup, layout.AtomicContextShutdown:
			canonical = true
		}
		switch e.Availability {
		case string(layout.AtomicSupported):
			if e.Implementation == "" || e.ImplementationKind == "" || e.Missing != "" || !canonical {
				return fmt.Errorf("%s supported capability needs its implementation, a canonical context and no open fact", op.Kind)
			}
		case string(layout.AtomicUnknown):
			if e.Missing == "" || e.Implementation != "" || e.Context != "" && !canonical {
				return fmt.Errorf("%s open capability obligation needs its missing fact and no implementation", op.Kind)
			}
		default:
			return fmt.Errorf("%s capability availability %q is not supported or unknown", op.Kind, e.Availability)
		}
		if op.Kind == OpAtomicFence {
			if e.Carrier != "" || e.SemanticType != "" || e.Representation != "" || e.SizeBytes != 0 || e.AlignBytes != 0 {
				return fmt.Errorf("atomic.fence capability evidence has no carrier or layout")
			}
		} else if e.Availability == string(layout.AtomicSupported) && (e.Carrier == "" || e.Representation == "" || e.SizeBytes == 0 || e.AlignBytes == 0) {
			return fmt.Errorf("%s supported capability needs its carrier, representation and atomic layout", op.Kind)
		}
	}
	return nil
}

// AtomicObligation is the first open MD-095 capability obligation found at
// executable emission: the operation, function, context, source provenance
// and missing fact. LayoutIncomplete separates an incomplete atomic layout
// from other missing evidence so tooling can report the two categories.
// Rules: rules/concurrency/atomics.md — §§ 37A(12), 48(4); rules/tooling/diagnostics.md — § 24(4l).
type AtomicObligation struct {
	Subject          string
	Function         FunctionID
	Context          string
	Provenance       string
	Missing          string
	LayoutIncomplete bool
}

// Error names the operation, context and missing fact (atomics.md § 48(4)).
func (o *AtomicObligation) Error() string {
	return fmt.Sprintf("%s in %s cannot be verified for %s at %s: %s", o.Subject, o.Function, o.Context, o.Provenance, o.Missing)
}

// VerifyAtomicEmissionEvidence is the MD-095 executable-emission gate: every
// atomic operation must have supported evidence in every context; the first
// open obligation is reported, naming the operation, context and missing
// fact. Analysis-only Semantic IR keeps obligations; this gate never turns
// one into a verified operation or a proven-unsupported claim.
// Rules: rules/concurrency/atomics.md — §§ 37A(12), 47(9); rules/compiler/semantic_ir.md — § 123B(3).
func VerifyAtomicEmissionEvidence(module *Module) error {
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				for _, e := range op.AtomicEvidence {
					if e.Availability == string(layout.AtomicSupported) {
						continue
					}
					subject := "atomic.Fence"
					if e.SemanticType != "" {
						subject = "Atomic[" + e.SemanticType + "]." + e.Operation
					}
					context := e.Context
					if context == "" {
						context = "an unresolved execution context"
					}
					layoutIncomplete := strings.HasPrefix(e.Missing, "incomplete atomic layout") || e.Missing == "the selected target has no verified data layout"
					return &AtomicObligation{Subject: subject, Function: fn.ID, Context: context, Provenance: e.Provenance, Missing: e.Missing, LayoutIncomplete: layoutIncomplete}
				}
			}
		}
	}
	return nil
}

// formatAtomicEvidence prints each context's availability deterministically.
// Rules: rules/compiler/semantic_ir.md — § 123B(2).
func formatAtomicEvidence(out io.Writer, evidence []AtomicCapabilityEvidence) {
	for _, e := range evidence {
		parts := []string{e.Context, e.Availability}
		if e.NoBlocking {
			parts = append(parts, "no-blocking")
		}
		if e.InterruptSafe {
			parts = append(parts, "interrupt-safe")
		}
		if e.Implementation != "" {
			parts = append(parts, "impl="+e.Implementation)
		}
		if e.Missing != "" {
			parts = append(parts, fmt.Sprintf("missing=%q", e.Missing))
		}
		fmt.Fprintf(out, " capability{%s}", strings.Join(parts, " "))
	}
}
