package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TryRemainingPanic is one panic path inside a try's protected expression
// that the try does not convert: try removes only the language-defined
// failures it handles and never catches panic from called functions or other
// checks (errorhandling.md § 31; effect_analysis.md "try").
type TryRemainingPanic struct {
	Source lexer.Token
	// Kind is the direct panic effect, or empty for a call whose target may
	// panic.
	Kind EffectKind
	// Callee names the called function of a panicking call.
	Callee         string
	PanicReasonIDs []diagnostics.PanicReasonID
}

// tryProtection records the direct effects and call sites a try's protected
// expression introduced, so the remaining panic paths can be read from the
// final call graph after the try removed the failures it converts.
type tryProtection struct {
	callable CallableID
	effects  []EffectSite
	sites    []CallSiteID
}

type tryProtectionStart struct {
	effects map[sourceTokenKey]map[EffectKind]bool
	sites   int
}

func (a *Analyzer) beginTryProtection() (tryProtectionStart, bool) {
	if a.summaryPass || a.callGraph == nil || a.currentCallable == "" {
		return tryProtectionStart{}, false
	}
	start := tryProtectionStart{effects: map[sourceTokenKey]map[EffectKind]bool{}, sites: len(a.callGraph.sites)}
	for _, effect := range a.callGraph.effects[a.currentCallable] {
		key := sourceTokenLocation(effect.Source)
		if start.effects[key] == nil {
			start.effects[key] = map[EffectKind]bool{}
		}
		start.effects[key][effect.Kind] = true
	}
	return start, true
}

func (a *Analyzer) endTryProtection(expr *ast.TryExpression, start tryProtectionStart) {
	record := tryProtection{callable: a.currentCallable}
	for _, effect := range a.callGraph.effects[a.currentCallable] {
		if !start.effects[sourceTokenLocation(effect.Source)][effect.Kind] && isPanicEffectKind(effect.Kind) {
			record.effects = append(record.effects, effect)
		}
	}
	for _, site := range a.callGraph.sites[start.sites:] {
		if site.Caller == a.currentCallable {
			record.sites = append(record.sites, site.ID)
		}
	}
	if a.tryProtections == nil {
		a.tryProtections = map[*ast.TryExpression]tryProtection{}
	}
	a.tryProtections[expr] = record
}

// ResolvedTryRemainingPanicsOf returns the panic paths of a resolved try's
// protected expression that remain after the try: direct checks it does not
// convert and calls whose targets may panic, in source order. The bool
// reports whether the try was analyzed on a live path.
func (a *Analyzer) ResolvedTryRemainingPanicsOf(expr *ast.TryExpression) ([]TryRemainingPanic, bool) {
	if a == nil || expr == nil {
		return nil, false
	}
	record, ok := a.tryProtections[expr]
	if !ok {
		return nil, false
	}
	remaining := []TryRemainingPanic{}
	current := map[sourceTokenKey]map[EffectKind]bool{}
	for _, effect := range a.callGraph.effects[record.callable] {
		key := sourceTokenLocation(effect.Source)
		if current[key] == nil {
			current[key] = map[EffectKind]bool{}
		}
		current[key][effect.Kind] = true
	}
	for _, effect := range record.effects {
		if current[sourceTokenLocation(effect.Source)][effect.Kind] {
			remaining = append(remaining, TryRemainingPanic{Source: effect.Source, Kind: effect.Kind, PanicReasonIDs: append([]diagnostics.PanicReasonID(nil), effect.PanicReasonIDs...)})
		}
	}
	wanted := map[CallSiteID]bool{}
	for _, id := range record.sites {
		wanted[id] = true
	}
	for _, site := range a.callGraph.sites {
		if !wanted[site.ID] {
			continue
		}
		for _, target := range site.Targets {
			summary := a.callGraph.EffectSummary(target)
			if !summary.MayPanic {
				continue
			}
			entry := TryRemainingPanic{Source: site.Source}
			if node, found := a.callGraph.Node(target); found {
				entry.Callee = node.Name
			}
			seen := map[diagnostics.PanicReasonID]bool{}
			for _, cause := range summary.PanicCauses {
				for _, reason := range cause.Effect.PanicReasonIDs {
					if !seen[reason] {
						seen[reason] = true
						entry.PanicReasonIDs = append(entry.PanicReasonIDs, reason)
					}
				}
			}
			sort.Slice(entry.PanicReasonIDs, func(i, j int) bool { return entry.PanicReasonIDs[i] < entry.PanicReasonIDs[j] })
			remaining = append(remaining, entry)
		}
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		left, right := remaining[i].Source, remaining[j].Source
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Column < right.Column
	})
	return remaining, true
}
