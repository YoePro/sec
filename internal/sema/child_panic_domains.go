package sema

import (
	"fmt"
	"strings"
)

// ChildPanicDomain retains a spawn boundary reached on the parent's stack and
// the panic causes reachable on the child's own stack. It asserts no target
// containment/cleanup policy and never feeds child causes into parent noPanic.
// Rules: rules/errors/panic.md §§6–9,21,28(7); rules/analysis/call_graph.md —
// "Same-stack effect transfer", "Spawn effect transfer".
type ChildPanicDomain struct {
	Boundary   CallSite
	ParentPath []CallSite
	MayPanic   bool
	Unknown    bool
	Causes     []EffectDomainCause
}

// ChildPanicDomains returns detached, deterministic boundary/cause witnesses.
// Parent traversal follows same-stack calls only; each child summary similarly
// stops at further spawn boundaries. Open targets remain explicitly unknown.
// Rules: rules/errors/panic.md §§6–9,21(4),28(7);
// rules/concurrency/tasks.md §14; rules/concurrency/threads.md §§31,56.
func (g *CallGraph) ChildPanicDomains(root CallableID) []ChildPanicDomain {
	if g == nil {
		return nil
	}
	type visit struct {
		id   CallableID
		path []CallSite
	}
	queue := []visit{{id: root}}
	seen := map[CallableID]bool{}
	var domains []ChildPanicDomain
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if seen[current.id] {
			continue
		}
		seen[current.id] = true
		for _, site := range g.Outgoing(current.id) {
			if sameStackExecution(site.Execution) {
				for _, target := range site.Targets {
					path := append(cloneDomainPath(current.path), cloneCallSite(site))
					queue = append(queue, visit{target, path})
				}
				continue
			}
			switch site.Execution {
			case CallExecutionSpawnTask, CallExecutionSpawnThread, CallExecutionSpawnProcess:
			default:
				continue
			}
			domain := ChildPanicDomain{Boundary: cloneCallSite(site), ParentPath: cloneDomainPath(current.path)}
			targets := site.TargetSet
			domain.Unknown = !targets.IsClosed || targets.HasOpenContract || targets.OpenContract != "" || len(site.Targets) == 0 || len(site.Targets) != len(targets.KnownTargets)
			domain.MayPanic = domain.Unknown
			for _, target := range site.Targets {
				if _, exists := g.nodes[target]; !exists {
					domain.Unknown = true
					domain.MayPanic = true
					continue
				}
				summary := g.DomainSummary(target)
				for _, cause := range summary.Causes {
					if cause.Domain != SummaryMayPanic {
						continue
					}
					domain.MayPanic = true
					domain.Unknown = domain.Unknown || cause.Unknown
					domain.Causes = append(domain.Causes, cloneDomainCause(cause))
				}
			}
			domains = append(domains, domain)
		}
	}
	return domains
}

// ChildPanicDescriptions shares user-facing domain provenance between LSP and
// sec analyse without giving either consumer another effect/containment model.
// Rules: rules/errors/panic.md §28(7); rules/compiler/semantic_ir.md §109(1–4).
func (g *CallGraph) ChildPanicDescriptions(root CallableID) []string {
	var lines []string
	for _, domain := range g.ChildPanicDomains(root) {
		if !domain.MayPanic && !domain.Unknown {
			continue
		}
		var names []string
		for _, target := range domain.Boundary.Targets {
			node, _ := g.Node(target)
			names = append(names, node.Name)
		}
		if len(names) == 0 {
			names = []string{"open callable target"}
		}
		state := "may panic"
		if domain.Unknown {
			state = "panic behavior unproven"
		}
		at := domain.Boundary.Source
		line := fmt.Sprintf("Child panic domain %s -> %s at %s:%d:%d: %s; containment follows target policy", domain.Boundary.Execution, strings.Join(names, ", "), at.File, at.Line, at.Column, state)
		for _, cause := range domain.Causes {
			node, _ := g.Node(cause.Callable)
			line += fmt.Sprintf("; cause in %s at %s:%d:%d: %s", node.Name, cause.Source.File, cause.Source.Line, cause.Source.Column, cause.Fact)
			if cause.Effect != nil && cause.Effect.HasMessage {
				line += fmt.Sprintf(" (%q)", cause.Effect.Message)
			}
		}
		lines = append(lines, line)
	}
	return lines
}
